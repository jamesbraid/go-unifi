//go:build integration

// unifi/behavior_probe_shared_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/internal/probe"
)

// This file holds the machinery the five batch probe files
// (behavior_probe_batch1..5_integration_test.go) share. It exists because
// roughly a third of the 30 resources those files measure are the same
// shape the existing FirewallGroup probe already established -- a flat v1
// REST collection with no branch discriminator -- and copying that
// function's body ten times would make the real, resource-specific findings
// harder to find in the diff, not easier.
//
// Nothing here replaces the judgment calls the individual probes make: a
// resource with a branch discriminator, a nested required field, or a
// non-generic path (v2, a command, a singleton, a batch endpoint) is written
// by hand in its own batch file instead of being forced through this shape.

// requiredFieldSweep removes one field at a time (via attempt) and reports
// which removals the controller refused. attempt owns building the doc and
// posting it; this only classifies the answer and logs it, so branchy
// resources (Account, Routing, RADIUSProfile's nested lists) can reuse it
// alongside the flat ones.
func requiredFieldSweep(t *testing.T, fields []string, attempt func(field string) (status int, body any)) []string {
	t.Helper()
	var required []string
	for _, field := range fields {
		status, body := attempt(field)
		if status/100 == 2 {
			t.Logf("create without %-28s accepted (HTTP %d) -- not required", field, status)
			continue
		}
		t.Logf("create without %-28s rejected (HTTP %d, %s) -- required on create",
			field, status, v1ErrCode(body))
		required = append(required, field)
	}
	sort.Strings(required)
	return required
}

// unknownKeyObserve probes an unrecognised key without asserting a
// direction. unknownKeyStripped/unknownKeyRejected exist for resources whose
// behaviour a prior probe already pinned; every resource this file measures
// is new to the artifact, and assuming the general v1-strips/v2-rejects
// split before measuring it is exactly the inference the task warns against
// -- APGroup and NetworkMembersGroup both turned out to be exceptions to it.
// This logs what actually happened and returns a short tag for the test's
// own log line; it asserts nothing, because nothing has been measured yet to
// assert against.
func unknownKeyObserve(ctx context.Context, t *testing.T, s *controllertest.Session, path string, doc map[string]any) string {
	t.Helper()
	payload := clone(doc)
	payload[probeUnknownKey] = "x"
	body, status, err := s.PostJSON(ctx, path, payload)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		reason := v1ErrCode(body)
		if reason == "no reason given" {
			reason = v2Rejection(body)
		}
		t.Logf("unknown-key probe: %s refused an unrecognised key (HTTP %d, %s)", path, status, reason)
		return fmt.Sprintf("REJECTED-%d", status)
	}
	stored := firstData(t, body)
	if id := objectID(stored); id != "" {
		s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	}
	if _, present := stored[probeUnknownKey]; present {
		t.Logf("unknown-key probe: %s stored an unrecognised key verbatim (HTTP %d)", path, status)
		return "STORED"
	}
	t.Logf("unknown-key probe: %s accepted an unrecognised key and stripped it (HTTP %d)", path, status)
	return "STRIPPED"
}

// simpleV1Field names one field the empty-vs-omit sweep exercises: the wire
// name and the blank form to write ("" for a string, []any{} for a list).
type simpleV1Field struct {
	wire  string
	blank any
}

// simpleV1Spec describes a flat v1 REST collection with a single create
// branch -- no discriminator field that changes which other fields are
// required. seed must build a full known-good body with every field the
// probe cares about already at a value the controller has no default for
// (the seeding trap): it is called once per unique name to create the probe
// object, once per required-on-create sweep iteration, and once for the
// unknown-key check.
type simpleV1Spec struct {
	resource   string // artifact key, e.g. "DpiGroup"
	collection string // v1 collection segment, e.g. "dpigroup"
	seed       func(unique string) map[string]any
	rename     string // a field safe to flip to prove the update path; "" picks a bare round-trip PUT instead
	empties    []simpleV1Field
}

// measureSimpleV1Contract runs the create/re-read/round-trip/required/
// empty-omit sequence the FirewallGroup probe established, generalised so
// every plain v1 collection in this task's findings does not need its own
// copy of it. It returns exactly what that probe returned: the write
// contract, the round-trip discard list, and the empty/omit map -- callers
// decide how to file them, since some resources (Client, FirewallRule) layer
// extra branch-specific measurement on top before recording anything.
func measureSimpleV1Contract(
	ctx context.Context, t *testing.T, s *controllertest.Session, site string, spec simpleV1Spec,
) (behavior.WriteContract, []string, map[string]behavior.EmptySemantics) {
	t.Helper()
	path := "/api/s/" + site + "/rest/" + spec.collection

	post := func(doc map[string]any) (int, any, map[string]any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		created := firstData(t, body)
		if id := objectID(created); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, body, created
	}

	asked := spec.seed(spec.resource + "-probe")
	body, status, err := s.PostJSON(ctx, path, asked)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		t.Fatalf("the known-good %s body was rejected (HTTP %d, %s): %v\n\nNothing removed from a "+
			"body that does not create can measure anything.",
			spec.resource, status, v1ErrCode(body), firstData(t, body))
	}
	id := objectID(firstData(t, body))
	if id == "" {
		t.Fatalf("the created %s carries no id, so nothing below can re-read it: %v", spec.resource, body)
	}
	defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck

	// Every verdict below comes from a fresh read, never the write's own
	// response -- trap 1.
	read := func() map[string]any {
		got, status, err := s.GetJSON(ctx, path+"/"+id)
		if err != nil || status != 200 {
			t.Fatalf("GET %s/%s answered HTTP %d (%v); the generated read uses that path",
				path, id, status, err)
		}
		return firstData(t, got)
	}
	stored := read()

	dropped := []string{}
	for _, r := range probe.Classify(asked, stored) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("%s: DROPPED %-24s (%s)", spec.resource, r.Wire, r.Detail)
		case probe.Changed:
			t.Logf("%s: CHANGED %-24s (%s) -- stored, not discarded", spec.resource, r.Wire, r.Detail)
		}
	}
	sort.Strings(dropped)
	t.Logf("%s round trip: %d asked, %d dropped", spec.resource, len(asked), len(dropped))

	unknownKeyObserve(ctx, t, s, path, spec.seed(spec.resource+"-unknown-key"))

	if _, listStatus, err := s.GetJSON(ctx, path); listStatus != 200 {
		t.Errorf("GET %s answered HTTP %d (%v); the generated list reads that path", path, listStatus, err)
	}

	updateVerb, updatePath := "", ""
	if spec.rename != "" {
		renamed := clone(stored)
		renamed[spec.rename] = fmt.Sprintf("%v-renamed", stored[spec.rename])
		after, putStatus, err := s.PutJSON(ctx, path+"/"+id, renamed)
		switch {
		case putStatus/100 != 2:
			t.Errorf("PUT %s/%s answered HTTP %d (%v %v); the generated update writes to that path",
				path, id, putStatus, after, err)
		case !jsonEqual(read()[spec.rename], renamed[spec.rename]):
			t.Errorf("PUT %s/{id} answered HTTP %d but the collection still reports the old %s; "+
				"the write was accepted and not stored", path, putStatus, spec.rename)
		default:
			updateVerb, updatePath = "PUT", "api/s/{site}/rest/"+spec.collection+"/{id}"
		}
		s.PutJSON(ctx, path+"/"+id, clone(stored)) //nolint:errcheck // restore
	} else {
		after, putStatus, err := s.PutJSON(ctx, path+"/"+id, clone(stored))
		if putStatus/100 == 2 {
			updateVerb, updatePath = "PUT", "api/s/{site}/rest/"+spec.collection+"/{id}"
		} else {
			t.Errorf("PUT %s/%s answered HTTP %d (%v %v)", path, id, putStatus, after, err)
		}
	}

	fields := sortedWireNames(asked)
	required := requiredFieldSweep(t, fields, func(field string) (int, any) {
		i := sort.SearchStrings(fields, field)
		doc := spec.seed(fmt.Sprintf("%s-req-%d", spec.resource, i))
		delete(doc, field)
		status, body, _ := post(doc)
		return status, body
	})

	put := func(doc map[string]any) int {
		body, status, err := s.PutJSON(ctx, path+"/"+id, doc)
		if status == 0 {
			t.Fatalf("transport to %s/%s: %v", path, id, err)
		}
		if status/100 != 2 {
			t.Logf("PUT %s/%s -> HTTP %d: %s", path, id, status, v1ErrCode(body))
		}
		return status
	}
	measured := map[string]behavior.EmptySemantics{}
	for _, f := range spec.empties {
		measured[f.wire] = storedEmptySemantics(t, f.wire, f.blank, nil, stored, put, read)
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "api/s/{site}/rest/" + spec.collection,
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: required,
	}
	return contract, dropped, measured
}

// recordWrite is the BEHAVIOR_WRITE=1 half every batch probe ends with:
// merge one resource's contract, discard list and empty map into the
// artifact. Replace, not merge, within each map -- a field that stopped
// being dropped, or stopped being measured, has to leave the artifact
// rather than linger as a fact nothing re-measures.
func recordWrite(
	t *testing.T, root, captured, resource, emptySection string,
	contract behavior.WriteContract, dropped []string, empties map[string]behavior.EmptySemantics,
) {
	t.Helper()
	mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
		if a.Writes == nil {
			a.Writes = map[string]behavior.WriteContract{}
		}
		a.Writes[resource] = contract
		if dropped != nil {
			if a.Discarded == nil {
				a.Discarded = map[string][]string{}
			}
			a.Discarded[resource] = dropped
		}
		if len(empties) > 0 {
			if a.Empty == nil {
				a.Empty = map[string]map[string]behavior.EmptySemantics{}
			}
			a.Empty[emptySection] = empties
		}
	})
}

// compareRecorded is the bare-run half: compare a fresh measurement against
// what the artifact pins, resource by resource. Skips (rather than fails)
// when there is no artifact yet or it was measured on a different
// controller -- the same gate behaviorGate already applies to writing.
func compareRecorded(
	t *testing.T, root, running, resource, emptySection string,
	contract behavior.WriteContract, dropped []string, empties map[string]behavior.EmptySemantics,
) {
	t.Helper()
	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok {
		t.Logf("no artifact at %s; run with BEHAVIOR_WRITE=1 to record %s", behavior.Path, resource)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	compareWriteContract(t, resource, art.Writes, contract)
	// dropped == nil means this probe never swept the resource's discard
	// list at all -- distinct from a non-nil empty slice, which means it
	// swept every field it sent and found nothing dropped. recordWrite
	// already draws this line (its own "if dropped != nil" guard), so a
	// caller documents "I don't own this resource's discard list" by
	// passing nil, same as it always has. compareRecorded used to ignore
	// that distinction and diff a nil measurement against whatever another
	// probe had pinned for the same resource: two DevicePortOverrides
	// probes assert this artifact key, one sweeping tagged_networkconf_ids
	// and one not, and the one that skips it was reading its own silence
	// as "swept, found nothing" and reporting the other probe's finding as
	// drift. Skipping the comparison on a nil measurement makes that
	// probe's silence read as what it is -- and the resource is still
	// covered, by the probe that does pass a real list.
	if dropped != nil {
		if pinned, has := art.Discarded[resource]; has {
			sort.Strings(pinned)
			sortedDropped := append([]string(nil), dropped...)
			sort.Strings(sortedDropped)
			if !stringSlicesEqual(pinned, sortedDropped) {
				t.Errorf("%s discard list drifted:\n  artifact: %v\n  measured: %v\n\n"+
					"re-measure with BEHAVIOR_WRITE=1 once the change is understood", resource, pinned, sortedDropped)
			}
		} else if len(dropped) > 0 {
			t.Errorf("%s measured %d dropped field(s) but the artifact pins none; "+
				"re-measure with BEHAVIOR_WRITE=1", resource, len(dropped))
		}
	}
	if len(empties) > 0 {
		compareEmptySemantics(t, emptySection, art.Empty[emptySection], empties)
	}
}

// recordWriteMergeEmpty is recordWrite's counterpart for a resource whose
// Empty section a prior probe already partly populated (PortProfile's
// "portconf", Device's "device"): it overlays the newly measured fields onto
// whatever is already pinned there instead of replacing the section
// outright, so a field this probe does not re-measure is not silently
// dropped from the artifact. Writes and Discarded still replace, as
// recordWrite's do -- only Empty additively merges, and only for this
// resource's own section.
func recordWriteMergeEmpty(
	t *testing.T, root, captured, resource, emptySection string,
	contract behavior.WriteContract, dropped []string, newEmpties map[string]behavior.EmptySemantics,
) {
	t.Helper()
	mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
		if a.Writes == nil {
			a.Writes = map[string]behavior.WriteContract{}
		}
		a.Writes[resource] = contract
		if dropped != nil {
			if a.Discarded == nil {
				a.Discarded = map[string][]string{}
			}
			a.Discarded[resource] = dropped
		}
		if a.Empty == nil {
			a.Empty = map[string]map[string]behavior.EmptySemantics{}
		}
		existing := a.Empty[emptySection]
		if existing == nil {
			existing = map[string]behavior.EmptySemantics{}
		}
		for field, sem := range newEmpties {
			existing[field] = sem
		}
		a.Empty[emptySection] = existing
	})
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
