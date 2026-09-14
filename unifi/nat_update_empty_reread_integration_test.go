//go:build integration

// unifi/nat_update_empty_reread_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationNatUpdateEmptyVsAbsentReread re-measures the three nat
// fields TestIntegrationNatUpdateEmptyVsAbsent
// (unifi/behavior_probe_integration_test.go) already records. That test
// still takes its EMPTY/OMIT verdict from the PUT's own response rather
// than a GET of the stored document -- the exact mistake that put
// OMIT-CLEARS in schemas/behavior.json for 39 fields across hotspotpackage,
// networkconf, portconf and wlanconf before 8b4eeae fixed it there and
// introduced storedEmptySemantics to do the reading correctly. nat was not
// swept by that fix.
//
// This lives in its own file and calls storedEmptySemantics rather than
// editing TestIntegrationNatUpdateEmptyVsAbsent in place, because another
// change is landing concurrently in behavior_probe_integration_test.go and
// editing it here would collide with that work. storedEmptySemantics is
// unexported but package-scoped, so this reaches it without touching the
// file it is declared in. TestIntegrationNatUpdateEmptyVsAbsent itself is
// unchanged and still measures the old, unsound way; reconciling the two
// (most likely by deleting the old one) is left for whoever lands that
// other change, once it is no longer moving.
//
// Two things came out of measuring this correctly instead of assuming the
// old verdicts were wrong:
//
//  1. description and in_interface, both on a MASQUERADE rule -- the same
//     rule shape the original probe uses -- re-measure IDENTICALLY off a
//     GET: EMPTY-CLEARS/OMIT-CLEARS for description, EMPTY-REJECTED/
//     OMIT-CLEARS for in_interface. The old verdicts were not wrong here.
//     Unlike the v1 rest PUT the earlier bug was measured against, a v2 nat
//     PUT that changes nothing still answers with the full stored document,
//     not an empty data array -- confirmed directly against this
//     controller (a no-op PUT and a PUT with an already-absent key both
//     answered with the complete object) -- so the failure mode that made
//     40 verdicts wrong elsewhere had nothing to bite on here. Same
//     methodology, and it happens to be a distinction without a difference
//     on this particular collection.
//
//  2. ip_address is a different story, and not the one the artifact
//     records. MASQUERADE refuses ip_address outright -- measured directly,
//     api.err.NatRuleInvalidParameters, "MASQUERADE may not have an IP
//     address translation" -- for any value, seed included. The original
//     probe's own seed loop hits exactly this rejection, logs it, and falls
//     back to "measuring against the bare rule": a baseline where the field
//     was never present to begin with, which makes EMPTY-CLEARS and
//     EMPTY-IGNORED (and OMIT-CLEARS and OMIT-KEEPS) indistinguishable --
//     storedEmptySemantics refuses to run in that shape at all, for exactly
//     this reason. So the pinned ip_address entry does not describe a value
//     the field can hold; it describes a field that was never seeded. NAT's
//     "type" is a discriminator (DNAT|SNAT|MASQUERADE) and ip_address is
//     SNAT's whole purpose -- an SNAT rule takes a real ip_address (measured:
//     201, stored verbatim) -- so this measures it there instead, which is a
//     different branch from what the artifact's entry was ever really
//     about, not a correction of the same measurement.
func TestIntegrationNatUpdateEmptyVsAbsentReread(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	wanID := ensureWANNetwork(ctx, t, s, c.Site)
	if wanID == "" {
		t.Fatal("no WAN network; every NAT write would fail for the wrong reason")
	}

	path := "/v2/api/site/" + c.Site + "/nat"
	filter := func() map[string]any {
		return map[string]any{
			"filter_type": "NONE", "firewall_group_ids": []string{},
			"invert_address": false, "invert_port": false,
		}
	}

	// putFor and readFor close over one NAT rule's id. Every verdict here
	// comes from readFor -- a GET of api/s/{site}/nat/{id} -- never from
	// what putFor's own response happened to say, which is the entire
	// point of re-measuring this.
	putFor := func(id string) func(map[string]any) int {
		return func(doc map[string]any) int {
			_, status, err := s.PutJSON(ctx, path+"/"+id, doc)
			mustTransport(t, err)
			return status
		}
	}
	readFor := func(id string) func() map[string]any {
		return func() map[string]any {
			body, status, err := s.GetJSON(ctx, path+"/"+id)
			mustTransport(t, err)
			doc := firstData(t, body)
			if status != 200 || doc == nil {
				t.Fatalf("re-reading %s/%s answered HTTP %d with %v; a verdict cannot be taken "+
					"from a document that did not come back", path, id, status, body)
			}
			return doc
		}
	}

	measured := map[string]behavior.EmptySemantics{}

	t.Run("masquerade", func(t *testing.T) {
		base := map[string]any{
			"enabled": true, "type": "MASQUERADE", "ip_version": "IPV4",
			"protocol": "all", "out_interface": wanID,
			"source_filter": filter(), "destination_filter": filter(),
		}
		body, status, err := s.PostJSON(ctx, path, base)
		mustTransport(t, err)
		if status/100 != 2 {
			t.Fatalf("the known-good MASQUERADE body did not create (HTTP %d): %v.\n\n"+
				"A probe with nothing to update measures nothing.", status, body)
		}
		stored := firstData(t, body)
		id := objectID(stored)
		if id == "" {
			t.Fatalf("no id in create response: %v", stored)
		}
		defer s.DeleteJSON(context.WithoutCancel(ctx), path+"/"+id) //nolint:errcheck
		put, read := putFor(id), readFor(id)

		for _, tc := range []struct{ field, seed string }{
			{"in_interface", wanID},
			{"description", "clear-probe"},
		} {
			seedDoc := clone(stored)
			seedDoc[tc.field] = tc.seed
			if st := put(seedDoc); st/100 != 2 {
				t.Fatalf("seeding %s=%q on a MASQUERADE rule was rejected (HTTP %d); this field "+
					"was expected to hold a value here", tc.field, tc.seed, st)
			}
			seeded := read()
			measured[tc.field] = storedEmptySemantics(t, tc.field, "", nil, seeded, put, read)
			put(clone(stored)) // reset for the next field
		}
	})

	t.Run("snat", func(t *testing.T) {
		base := map[string]any{
			"enabled": true, "type": "SNAT", "ip_version": "IPV4",
			"protocol": "all", "out_interface": wanID, "ip_address": "192.0.2.55",
			"source_filter": filter(), "destination_filter": filter(),
		}
		body, status, err := s.PostJSON(ctx, path, base)
		mustTransport(t, err)
		if status/100 != 2 {
			t.Fatalf("the known-good SNAT body did not create (HTTP %d): %v.\n\n"+
				"A probe with nothing to update measures nothing.", status, body)
		}
		stored := firstData(t, body)
		id := objectID(stored)
		if id == "" {
			t.Fatalf("no id in create response: %v", stored)
		}
		defer s.DeleteJSON(context.WithoutCancel(ctx), path+"/"+id) //nolint:errcheck
		put, read := putFor(id), readFor(id)

		measured["ip_address"] = storedEmptySemantics(t, "ip_address", "", nil, stored, put, read)
	})

	var summary []string
	for _, f := range slices.Sorted(maps.Keys(measured)) {
		summary = append(summary, fmt.Sprintf("%-16s %-16s %s", f, measured[f].Empty, measured[f].Omit))
	}
	t.Logf("nat empty-vs-absent semantics, re-measured off a GET rather than the PUT response:\n  %s",
		strings.Join(summary, "\n  "))

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Empty == nil {
				a.Empty = map[string]map[string]behavior.EmptySemantics{}
			}
			if a.Empty["nat"] == nil {
				a.Empty["nat"] = map[string]behavior.EmptySemantics{}
			}
			for f, sem := range measured {
				a.Empty["nat"][f] = sem
			}
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	pinned := art.Empty["nat"]
	if !ok || pinned == nil {
		t.Logf("no pinned empty semantics for nat in %s; run with BEHAVIOR_WRITE=1 to record them", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would "+
			"file a version difference as drift", art.ControllerVersion, running)
	}
	for f, got := range measured {
		want, has := pinned[f]
		if !has {
			t.Errorf("nat.%s: measured empty=%s omit=%s but the artifact pins nothing; "+
				"re-measure with BEHAVIOR_WRITE=1", f, got.Empty, got.Omit)
			continue
		}
		if got != want {
			t.Errorf("nat.%s: re-measured off a GET as empty=%s omit=%s, but the artifact pins "+
				"empty=%s omit=%s. Re-measure with BEHAVIOR_WRITE=1 once this is understood.",
				f, got.Empty, got.Omit, want.Empty, want.Omit)
		}
	}
}
