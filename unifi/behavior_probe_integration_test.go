//go:build integration

// unifi/behavior_probe_integration_test.go
package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

// These probes feed schemas/behavior.json, the measured-behaviour artifact.
// Run with BEHAVIOR_WRITE=1 at capture time to record what the controller
// did; run bare (CI) to compare a fresh measurement against what the
// artifact pins and fail on drift.
//
// A measurement is a claim about one controller generation, so both modes
// check the booted controller's own reported version: recording refuses to
// file a measurement against a version the capture did not pin, and the
// comparison skips rather than report drift between two different
// controllers (the UOS harness bundles an older Network app than the lock).

// behaviorWriteRequested reports whether this run records into the artifact
// rather than comparing against it.
func behaviorWriteRequested() bool {
	return os.Getenv("BEHAVIOR_WRITE") == "1"
}

// capturedBehaviorVersion returns the module root and the pinned controller
// version from schemas/VERSION -- the version the artifact claims to
// describe.
func capturedBehaviorVersion(t *testing.T) (root, version string) {
	t.Helper()
	root = fields.ModuleRoot()
	if root == "" {
		t.Fatal("unable to locate the module root (go.mod)")
	}
	raw, err := os.ReadFile(filepath.Join(root, "schemas", "VERSION"))
	if err != nil {
		t.Fatalf("read schemas/VERSION: %v", err)
	}
	version = strings.TrimSpace(string(raw))
	if version == "" {
		t.Fatal("schemas/VERSION is empty")
	}
	return root, version
}

// runningControllerVersion asks the booted controller what build it actually
// is, because the artifact must never be written from -- or compared against
// -- a controller other than the one it names.
func runningControllerVersion(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/stat/sysinfo")
	if err != nil || status != 200 {
		t.Fatalf("sysinfo: status=%d err=%v", status, err)
	}
	v, _ := firstData(t, body)["version"].(string)
	if v == "" {
		t.Fatalf("sysinfo carries no version: %#v", body)
	}
	return v
}

// mergeBehaviorArtifact load-modify-writes the artifact so each probe owns
// only its own section and a partial re-measure never erases the rest.
func mergeBehaviorArtifact(t *testing.T, root, version string, mutate func(*behavior.Artifact)) {
	t.Helper()
	art, _, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	art.ControllerVersion = version
	mutate(&art)
	if err := behavior.Write(root, art); err != nil {
		t.Fatalf("write %s: %v", behavior.Path, err)
	}
	t.Logf("recorded into %s (controller %s)", behavior.Path, version)
}

// TestIntegrationCoercionFloors measures the SettingUsg connection-tracking
// timeout floors: the controller accepts a below-range value and silently
// stores a per-field minimum instead. The floors were previously hand-carried
// in the terraform provider; this records them where a controller bump
// re-measures them into a reviewable diff.
//
// The probe writes 1 -- below every plausible floor -- to each timeout field
// under timeout_setting_preference=manual (under auto the controller owns the
// fields outright and would overwrite them regardless, measuring ownership
// rather than coercion), re-reads, and records every field the controller
// refused to store verbatim.
func TestIntegrationCoercionFloors(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	root, captured := capturedBehaviorVersion(t)
	running := runningControllerVersion(ctx, t, s, c.Site)
	if behaviorWriteRequested() && running != captured {
		t.Fatalf("BEHAVIOR_WRITE=1 but the booted controller reports %s while schemas/VERSION says %s; "+
			"recording would file the measurement against the wrong controller", running, captured)
	}

	body, status, err := s.GetJSON(ctx, "/api/s/"+c.Site+"/get/setting/usg")
	if err != nil || status != 200 {
		t.Fatalf("get/setting/usg (HTTP %d): %v", status, err)
	}
	current := firstData(t, body)
	if current == nil {
		t.Fatalf("get/setting/usg returned no document: %#v", body)
	}

	timeouts := usgTimeoutFields(t, root, current)
	if len(timeouts) == 0 {
		t.Fatal("no timeout fields to probe; neither the schema cache nor the live document named any")
	}
	t.Logf("probing %d timeout fields: %s", len(timeouts), strings.Join(timeouts, ", "))

	// One merge-write sets every field to 1 at once; set/setting merges a
	// partial body (see masked_update_integration_test.go), and the floors
	// are per-field, so one write measures all of them.
	write := map[string]any{"key": "usg", "timeout_setting_preference": "manual"}
	for _, f := range timeouts {
		write[f] = 1
	}

	// Restore what was stored before the probe. Fields the controller never
	// reported a value for cannot be restored to anything; the mode going
	// back to its original value covers them when that value is auto, since
	// auto re-owns every timeout.
	defer func() {
		restore := map[string]any{"key": "usg"}
		for _, f := range append([]string{"timeout_setting_preference"}, timeouts...) {
			if v, ok := current[f]; ok {
				restore[f] = v
			}
		}
		if len(restore) == 1 {
			t.Log("nothing to restore: the original document carried none of the probed fields")
			return
		}
		if _, status, err := s.PutJSON(ctx, "/api/s/"+c.Site+"/set/setting/usg", restore); err != nil || status != 200 {
			t.Logf("restore write failed (HTTP %d): %v", status, err)
		}
	}()

	if body, status, err := s.PutJSON(ctx, "/api/s/"+c.Site+"/set/setting/usg", write); err != nil || status != 200 {
		t.Fatalf("probe write rejected (HTTP %d): %v %v\n\nA rejection measures validation, not coercion; "+
			"nothing below is meaningful.", status, body, err)
	}

	body, status, err = s.GetJSON(ctx, "/api/s/"+c.Site+"/get/setting/usg")
	if err != nil || status != 200 {
		t.Fatalf("re-read failed (HTTP %d): %v", status, err)
	}
	after := firstData(t, body)

	measured := map[string]behavior.Coercion{}
	var summary []string
	for _, f := range timeouts {
		stored, present := after[f]
		if !present {
			// A written field the controller did not store at all is a
			// discard, which is the round-trip probe's finding, not a floor.
			t.Errorf("%s: written as 1 and absent from the re-read; a discarded field cannot carry a floor", f)
			continue
		}
		if jsonEqual(stored, 1) {
			summary = append(summary, fmt.Sprintf("%-28s kept 1", f))
			continue
		}
		got := renderStoredValue(stored)
		measured[f] = behavior.Coercion{Wrote: "1", Stored: got}
		summary = append(summary, fmt.Sprintf("%-28s floored to %s", f, got))
	}
	t.Logf("coercion floors for SettingUsg (wrote 1 everywhere):\n  %s", strings.Join(summary, "\n  "))

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Coercions == nil {
				a.Coercions = map[string]map[string]behavior.Coercion{}
			}
			a.Coercions["SettingUsg"] = measured
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	pinned := art.Coercions["SettingUsg"]
	if !ok || pinned == nil {
		t.Logf("no pinned coercions for SettingUsg in %s; run with BEHAVIOR_WRITE=1 to record them", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	probed := map[string]bool{}
	for _, f := range timeouts {
		probed[f] = true
		want, pinnedHas := pinned[f]
		got, measuredHas := measured[f]
		switch {
		case pinnedHas && !measuredHas:
			t.Errorf("%s: artifact pins a floor (wrote %s, stored %s) but the controller now stores the "+
				"probe value verbatim; re-measure with BEHAVIOR_WRITE=1", f, want.Wrote, want.Stored)
		case !pinnedHas && measuredHas:
			t.Errorf("%s: the controller now floors (wrote %s, stored %s) but the artifact pins nothing; "+
				"re-measure with BEHAVIOR_WRITE=1", f, got.Wrote, got.Stored)
		case pinnedHas && got != want:
			t.Errorf("%s: artifact pins wrote %s -> stored %s, measured wrote %s -> stored %s",
				f, want.Wrote, want.Stored, got.Wrote, got.Stored)
		}
	}
	for f := range pinned {
		if !probed[f] {
			t.Logf("%s is pinned in the artifact but was not probed this run (field list source differs)", f)
		}
	}
}

// usgTimeoutFields returns the connection-tracking timeout wire names to
// probe. The primary source is the extracted field definitions: every
// SettingUsg field ending _timeout whose declared pattern accepts the probe
// value. That excludes arp_cache_timeout by measurement rather than by name
// -- its pattern (normal|min-dhcp-lease|custom) is a mode enum, not a
// duration, and refuses "1".
//
// The cache is deliberately not committed (see .gitignore), so a plain
// checkout -- CI's integration run -- has no schemas/fields. There the live
// document is the fallback: every _timeout key the controller reports with a
// numeric value.
func usgTimeoutFields(t *testing.T, root string, current map[string]any) []string {
	t.Helper()

	var out []string
	path := filepath.Join(root, "schemas", "fields", "SettingUsg.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Logf("%s absent (%v); deriving the field list from the live setting document instead", path, err)
		for k, v := range current {
			if strings.HasSuffix(k, "_timeout") && isWireNumber(v) {
				out = append(out, k)
			}
		}
		sort.Strings(out)
		return out
	}

	var def map[string]any
	if err := json.Unmarshal(raw, &def); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for k, v := range def {
		if !strings.HasSuffix(k, "_timeout") {
			continue
		}
		pattern, ok := v.(string)
		if !ok {
			continue
		}
		if pattern != "" {
			// The controller validates with java.util.regex matches(), i.e.
			// a full match; anchor the same way.
			re, err := regexp.Compile("^(?:" + pattern + ")$")
			if err != nil || !re.MatchString("1") {
				t.Logf("skipping %s: its pattern %q refuses the probe value", k, pattern)
				continue
			}
		}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// isWireNumber reports whether a decoded JSON value is a number, tolerating
// the controller's habit of returning numerics as digit strings.
func isWireNumber(v any) bool {
	switch n := v.(type) {
	case float64:
		return true
	case string:
		_, err := strconv.Atoi(n)
		return err == nil
	}
	return false
}

// renderStoredValue renders a stored wire value for the artifact. Integral
// floats print as integers so a floor reads "7440", not "7440e+03"-adjacent
// noise; everything else prints as-is.
func renderStoredValue(v any) string {
	if f, ok := v.(float64); ok && f == math.Trunc(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return fmt.Sprintf("%v", v)
}

// TestIntegrationWriteContract measures two write contracts the codegen
// cannot guess from resource shape, into the artifact's Writes section:
//
//   - ContentFiltering: the generated create POSTs to the v2 collection and
//     the controller answers 405 -- route exists, verb wrong (the ROADMAP
//     finding, measured on 10.6.101). The probe records the verb+path that
//     actually creates, or the rejection itself as an honest unknown.
//   - Nat: create works as generated, but three fields the struct marks
//     optional must be present or the controller refuses the create. The
//     probe records the measured required-on-create set.
func TestIntegrationWriteContract(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	root, captured := capturedBehaviorVersion(t)
	running := runningControllerVersion(ctx, t, s, c.Site)
	if behaviorWriteRequested() && running != captured {
		t.Fatalf("BEHAVIOR_WRITE=1 but the booted controller reports %s while schemas/VERSION says %s; "+
			"recording would file the measurement against the wrong controller", running, captured)
	}

	cf := measureContentFilteringContract(ctx, t, s, c.Site)
	nat := measureNatContract(ctx, t, s, c.Site)

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Writes == nil {
				a.Writes = map[string]behavior.WriteContract{}
			}
			a.Writes["ContentFiltering"] = cf
			a.Writes["Nat"] = nat
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok || art.Writes == nil {
		t.Logf("no pinned write contracts in %s; run with BEHAVIOR_WRITE=1 to record them", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	compareWriteContract(t, "ContentFiltering", art.Writes, cf)
	compareWriteContract(t, "Nat", art.Writes, nat)
}

// measureContentFilteringContract finds the verb+path that creates a
// content-filtering object. The generated POST is expected to answer 405;
// the probe then tries PUT with the same minimal body, then PUT with a shape
// discovered from the collection, and records whatever succeeds. Nothing
// succeeding is itself the finding, recorded as the rejection.
func measureContentFilteringContract(ctx context.Context, t *testing.T, s *controllertest.Session, site string) behavior.WriteContract {
	t.Helper()

	const relPath = "v2/api/site/{site}/content-filtering"
	path := "/v2/api/site/" + site + "/content-filtering"
	minimal := map[string]any{"name": "probe", "enabled": false}

	deleteCreated := func(body any) {
		if id := objectID(firstData(t, body)); id != "" {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
	}
	try := func(verb string, doc map[string]any) (int, any) {
		var (
			body   any
			status int
			err    error
		)
		switch verb {
		case "POST":
			body, status, err = s.PostJSON(ctx, path, doc)
		case "PUT":
			body, status, err = s.PutJSON(ctx, path, doc)
		}
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		t.Logf("%s %s -> HTTP %d: %v", verb, path, status, body)
		return status, body
	}

	postStatus, postBody := try("POST", minimal)
	if postStatus/100 == 2 {
		// The 405 belief is stale; the measured truth wins.
		deleteCreated(postBody)
		return behavior.WriteContract{CreateVerb: "POST", CreatePath: relPath}
	}

	putStatus, putBody := try("PUT", minimal)
	if putStatus/100 == 2 {
		deleteCreated(putBody)
		return behavior.WriteContract{CreateVerb: "PUT", CreatePath: relPath}
	}

	// The minimal body may be what PUT refuses, not the verb: discover the
	// object shape from the collection and retry with it.
	listBody, listStatus, err := s.GetJSON(ctx, path)
	if err != nil || listStatus != 200 {
		t.Logf("GET %s -> HTTP %d: %v (no shape to discover)", path, listStatus, err)
	} else if template := firstData(t, listBody); template != nil {
		doc := clone(template)
		delete(doc, "_id")
		delete(doc, "site_id")
		doc["name"] = "probe"
		doc["enabled"] = false
		shapedStatus, shapedBody := try("PUT", doc)
		if shapedStatus/100 == 2 {
			deleteCreated(shapedBody)
			return behavior.WriteContract{CreateVerb: "PUT", CreatePath: relPath}
		}
	} else {
		t.Logf("GET %s returned an empty collection; no shape to discover", path)
	}

	// The honest unknown: no verb this probe knows creates the object. The
	// artifact records the measured rejection so the codegen stops assuming
	// POST works, without inventing a verb nothing measured.
	verdict := fmt.Sprintf("POST-REJECTED-%d", postStatus)
	t.Logf("LOUD: no verb creates content-filtering here (POST %d, PUT %d); recording %q as the contract",
		postStatus, putStatus, verdict)
	return behavior.WriteContract{CreateVerb: verdict, CreatePath: relPath}
}

// measureNatContract verifies the known-good NAT create and measures which
// of the fields the struct marks optional are actually load-bearing: the
// controller answers a create missing them with an error (an unhandled 500,
// measured), so they belong in RequiredOnCreate.
func measureNatContract(ctx context.Context, t *testing.T, s *controllertest.Session, site string) behavior.WriteContract {
	t.Helper()

	wanID := ensureWANNetwork(ctx, t, s, site)
	if wanID == "" {
		t.Fatal("no WAN network; every NAT create would fail for the wrong reason")
	}

	path := "/v2/api/site/" + site + "/nat"
	filter := func() map[string]any {
		return map[string]any{
			"filter_type": "NONE", "firewall_group_ids": []string{},
			"invert_address": false, "invert_port": false,
		}
	}
	base := map[string]any{
		"enabled": true, "type": "MASQUERADE", "ip_version": "IPV4",
		"protocol": "all", "out_interface": wanID,
		"source_filter": filter(), "destination_filter": filter(),
	}

	post := func(doc map[string]any) (int, string, any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		// A missing field draws a non-JSON HTTP 500 from this endpoint;
		// the status is the measurement, so ErrNotJSON is not fatal here.
		if err != nil {
			t.Logf("POST %s -> HTTP %d (%v)", path, status, err)
		}
		return status, objectID(firstData(t, body)), body
	}

	status, id, body := post(base)
	if status/100 != 2 {
		t.Fatalf("the known-good NAT body was rejected (HTTP %d): %v\n\nNothing removed from a body "+
			"that does not create can measure anything.", status, body)
	}
	if id == "" {
		t.Errorf("created NAT rule carries no id; it cannot be deleted: %v", body)
	} else {
		s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	}

	var required []string
	for _, field := range []string{"protocol", "source_filter", "destination_filter"} {
		doc := clone(base)
		delete(doc, field)
		status, id, _ := post(doc)
		if status/100 == 2 {
			if id != "" {
				s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
			}
			t.Logf("NAT create without %-18s accepted (HTTP %d) -- not required", field, status)
			continue
		}
		t.Logf("NAT create without %-18s rejected (HTTP %d) -- required on create", field, status)
		required = append(required, field)
	}
	sort.Strings(required)

	return behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "v2/api/site/{site}/nat",
		UpdateVerb: "PUT", UpdatePath: "v2/api/site/{site}/nat/{id}",
		RequiredOnCreate: required,
	}
}

// compareWriteContract checks one measured contract against the artifact.
// Empty and nil required sets mean the same thing on the wire (omitempty),
// so both sides normalize before comparing.
func compareWriteContract(t *testing.T, resource string, pinned map[string]behavior.WriteContract, got behavior.WriteContract) {
	t.Helper()
	want, ok := pinned[resource]
	if !ok {
		t.Logf("no pinned write contract for %s; run with BEHAVIOR_WRITE=1 to record it", resource)
		return
	}
	normalize := func(w behavior.WriteContract) behavior.WriteContract {
		if len(w.RequiredOnCreate) == 0 {
			w.RequiredOnCreate = nil
		} else {
			sort.Strings(w.RequiredOnCreate)
		}
		if len(w.RequiredOnUpdate) == 0 {
			w.RequiredOnUpdate = nil
		} else {
			sort.Strings(w.RequiredOnUpdate)
		}
		return w
	}
	want, got = normalize(want), normalize(got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s write contract drifted:\n  artifact: %+v\n  measured: %+v\n\nEither the controller "+
			"changed or the artifact is stale; re-measure with BEHAVIOR_WRITE=1.", resource, want, got)
	}
}
