//go:build integration

// unifi/behavior_probe_integration_test.go
package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
	"github.com/ubiquiti-community/go-unifi/internal/probe"
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
//
// The two API generations disagree about what an accepted write proves, and
// several verdicts below are only readable once that is settled:
//
//   - v1 (api/s/{site}/rest/...) validates each key against the field
//     document the controller ships, and a key the document does not name
//     is dropped from the payload and the write carries on. Rejecting it
//     instead is behind a webapi.strict system property that defaults off.
//     So a v1 create answering 200 says the controller stored SOMETHING, not
//     that it stored what was asked -- which is why every v1 probe here
//     re-reads and classifies rather than trusting the status.
//   - v2 (v2/api/site/{site}/...) binds the body to a Jackson DTO with no
//     unknown-property tolerance, so an unrecognised key is a 400 naming the
//     key. Not every v2 collection does: trafficroutes accepts one and
//     stores the document without it, exactly as v1 does (measured by
//     TestIntegrationTrafficRouteWriteContract), so the generation alone
//     does not settle what a 2xx proves.
//
// Both halves are asserted where they are relied on -- unknownKeyStripped
// for v1, unknownKeyRejected for v2, and the measurement in the traffic
// route probe for the collection that splits the difference -- so the
// distinction stays a measurement rather than a remembered claim.

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

// behaviorGate resolves what every artifact probe needs before it measures
// anything: the module root, the controller version the capture pinned, and
// the version the booted controller reports. Recording against a controller
// the capture does not name files the measurement under the wrong build, so
// that is refused in one place rather than restated in each probe.
func behaviorGate(ctx context.Context, t *testing.T, s *controllertest.Session, site string) (root, captured, running string) {
	t.Helper()
	root, captured = capturedBehaviorVersion(t)
	running = runningControllerVersion(ctx, t, s, site)
	if behaviorWriteRequested() && running != captured {
		t.Fatalf("BEHAVIOR_WRITE=1 but the booted controller reports %s while schemas/VERSION says %s; "+
			"recording would file the measurement against the wrong controller", running, captured)
	}
	return root, captured, running
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
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

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

// TestIntegrationWriteContract measures the write contracts the codegen
// cannot guess from resource shape, into the artifact's Writes section:
//
//   - ContentFiltering: the collection path answers 405 to POST because it
//     is mapped for GET alone; creates go to a /create sub-path. The probe
//     measures which verb and path actually create, so the generated client
//     follows the controller rather than the collection-POST convention the
//     other v2 resources happen to use.
//   - Nat: create works as generated, but three fields the struct marks
//     optional must be present or the controller refuses the create. The
//     probe records the measured required-on-create set.
//   - FirewallPolicy: the jar marks source.zone_id and destination.zone_id
//     @NotNull on the write DTO; the probe measures whether a create
//     without each actually fails.
//
// OSPFRouter is measured by TestIntegrationOSPFRouterWriteContract, which
// owns the artifact entry outright: two probes replacing the same key would
// each erase the other's measured facts.
func TestIntegrationWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	cf := measureContentFilteringContract(ctx, t, s, c.Site)
	nat := measureNatContract(ctx, t, s, c.Site)
	fwp := measureFirewallPolicyContract(ctx, t, s, c.Site)

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Writes == nil {
				a.Writes = map[string]behavior.WriteContract{}
			}
			a.Writes["ContentFiltering"] = cf
			a.Writes["Nat"] = nat
			a.Writes["FirewallPolicy"] = fwp
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
	compareWriteContract(t, "FirewallPolicy", art.Writes, fwp)
}

// measureContentFilteringContract measures the verb and path that create a
// content-filtering rule, and which fields the create cannot omit.
//
// The 405 this endpoint answers to a collection POST was read once as the
// product feature-gating creates. It is not: the controller maps
// /content-filtering for GET alone and puts the create on a /create
// sub-path, so a POST to the collection matches the path, finds no handler
// for the verb, and draws Spring's 405. The jar's own route table for
// 10.6.101 is
//
//	GET    /api/site/{siteName}/content-filtering
//	POST   /api/site/{siteName}/content-filtering/create
//	PUT    /api/site/{siteName}/content-filtering/{id}
//	DELETE /api/site/{siteName}/content-filtering/{id}
//	GET    /api/site/{siteName}/content-filtering/categories
//
// (com.ubnt.net.l.k, served under the /v2 context path). The probe measures
// that rather than trusting it, and asserts the collection POST is still
// 405 so a controller that grows one is noticed instead of silently
// leaving the generated client on the longer path.
func measureContentFilteringContract(ctx context.Context, t *testing.T, s *controllertest.Session, site string) behavior.WriteContract {
	t.Helper()

	const (
		createRel = "v2/api/site/{site}/content-filtering/create"
		updateRel = "v2/api/site/{site}/content-filtering/{id}"
	)
	base := "/v2/api/site/" + site + "/content-filtering"

	// The list has to be served before anything else here means what it
	// looks like: a 404 collection would make every verdict below "the
	// route is absent" wearing another status code's clothes.
	if body, status, err := s.GetJSON(ctx, base); status != 200 {
		t.Fatalf("GET %s answered HTTP %d (%v %v); the collection is not served, so no verb below "+
			"measures the write contract", base, status, body, err)
	}

	post := func(path string, doc map[string]any) (int, map[string]any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		stored := firstData(t, body)
		if id := objectID(stored); id != "" {
			s.DeleteJSON(ctx, base+"/"+id) //nolint:errcheck
		}
		return status, stored
	}

	minimal := map[string]any{"name": "probe", "enabled": false}
	if status, _ := post(base, minimal); status != 405 {
		t.Errorf("POST %s answered HTTP %d, not the 405 a GET-only path gives; the controller's "+
			"route table changed and the create path below may no longer be the right one", base, status)
	}

	lanID := defaultLANNetworkID(ctx, t, s, site)
	if lanID == "" {
		t.Fatal("no corporate network on the site; a rule naming no network would fail for the wrong reason")
	}
	// Smallest body the DTO's own constraints admit: name and categories are
	// @NotEmpty, schedule is @NotNull, and the service refuses a rule that
	// addresses neither a network nor a client.
	good := func(name string) map[string]any {
		return map[string]any{
			"name": name, "enabled": true,
			"categories":  []string{"ADULT"},
			"network_ids": []string{lanID},
			"client_macs": []string{},
			"allow_list":  []string{},
			"block_list":  []string{},
			"safe_search": []string{},
			"schedule":    map[string]any{"mode": "ALWAYS"},
		}
	}

	status, created := post(base+"/create", good("contract-probe"))
	if status/100 != 2 {
		t.Fatalf("the known-good body was rejected at %s/create (HTTP %d): %v\n\nNothing removed from "+
			"a body that does not create can measure anything.", base, status, created)
	}
	t.Logf("POST %s/create -> HTTP %d; that is the create", base, status)

	// Settles what a 2xx from the sweep below means: on v2 the controller
	// binds the body strictly, so an accepted create took the body as sent.
	unknownKeyRejected(ctx, t, s, base+"/create", good("contract-probe-unknown-key"))

	// The update path the generated client already uses, confirmed rather
	// than assumed: the artifact recorded no update contract at all while
	// the create was believed impossible.
	updateVerb, updatePath := "", ""
	body, updStatus, err := s.PostJSON(ctx, base+"/create", good("contract-probe-update"))
	if updStatus/100 != 2 {
		t.Fatalf("seeding a rule to update failed (HTTP %d): %v %v", updStatus, body, err)
	}
	seeded := firstData(t, body)
	if id := objectID(seeded); id != "" {
		defer s.DeleteJSON(ctx, base+"/"+id) //nolint:errcheck
		edited := clone(seeded)
		edited["name"] = "contract-probe-updated"
		after, putStatus, err := s.PutJSON(ctx, base+"/"+id, edited)
		if putStatus/100 == 2 {
			updateVerb, updatePath = "PUT", updateRel
			t.Logf("PUT %s/{id} -> HTTP %d; that is the update", base, putStatus)
		} else {
			t.Errorf("PUT %s/%s answered HTTP %d (%v %v); the generated update writes to that path",
				base, id, putStatus, after, err)
		}
	} else {
		t.Errorf("the seeded rule carries no id, so the update path cannot be measured: %v", seeded)
	}

	fields := make([]string, 0, len(good("")))
	for f := range good("") {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	var required []string
	for i, field := range fields {
		doc := good(fmt.Sprintf("contract-probe-%d", i))
		delete(doc, field)
		status, stored := post(base+"/create", doc)
		if status/100 == 2 {
			t.Logf("content-filtering create without %-12s accepted (HTTP %d) -- not required", field, status)
			continue
		}
		t.Logf("content-filtering create without %-12s rejected (HTTP %d, %s) -- required on create",
			field, status, v2Rejection(stored))
		required = append(required, field)
	}

	// network_ids and client_macs are one requirement, not two: the service
	// refuses a rule that addresses neither. The sweep above empties
	// client_macs, so removing network_ids leaves nothing addressed and
	// reads as "network_ids is required" -- which would be published as a
	// rule a caller addressing clients does not have to obey. Measure the
	// other half and drop the entry when clients satisfy it.
	byClient := good("contract-probe-macs")
	delete(byClient, "network_ids")
	byClient["client_macs"] = []string{"00:11:22:33:44:55"}
	if status, stored := post(base+"/create", byClient); status/100 == 2 {
		required = slices.DeleteFunc(required, func(f string) bool { return f == "network_ids" })
		t.Logf("LOUD: a rule naming client_macs and no network_ids creates (HTTP %d); network_ids is "+
			"not required on its own, so it is not recorded as such", status)
	} else {
		t.Logf("a rule naming client_macs and no network_ids is rejected too (HTTP %d, %s)",
			status, v2Rejection(stored))
	}
	sort.Strings(required)

	return behavior.WriteContract{
		CreateVerb: "POST", CreatePath: createRel,
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: required,
	}
}

// probeUnknownKey is the wire name no controller schema claims, sent to find
// out what the endpoint does with a key it does not recognise.
const probeUnknownKey = "go_unifi_unknown_probe_key"

// unknownKeyRejected asserts that a v2 endpoint refuses a body carrying an
// unrecognised key, and refuses it by name. Without that, a v2 create
// answering 200 would prove no more than a v1 one does, and the
// required-on-create sweeps that read a 200 as "the field is optional" would
// be reading a stripped payload instead of an accepted one.
func unknownKeyRejected(ctx context.Context, t *testing.T, s *controllertest.Session, path string, doc map[string]any) {
	t.Helper()
	payload := clone(doc)
	payload[probeUnknownKey] = "x"
	body, status, err := s.PostJSON(ctx, path, payload)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 == 2 {
		if id := objectID(firstData(t, body)); id != "" {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		t.Errorf("POST %s accepted an unrecognised key (HTTP %d); v2 was measured rejecting one, and "+
			"the sweeps here read a 2xx as the controller having taken the body as sent", path, status)
		return
	}
	reason := v2Rejection(body)
	if !strings.Contains(reason, probeUnknownKey) {
		t.Errorf("POST %s refused an unrecognised key with %q, which does not name it; the rejection "+
			"may be about something else entirely", path, reason)
		return
	}
	t.Logf("v2 %s refuses an unrecognised key by name (HTTP %d) -- an accepted v2 body was taken as sent",
		path, status)
}

// unknownKeyStripped asserts the v1 counterpart: the collection accepts a
// body carrying an unrecognised key and stores the document without it. That
// is why a v1 probe cannot read a 200 as "the controller took what I sent"
// and has to compare the re-read against what it asked for.
func unknownKeyStripped(ctx context.Context, t *testing.T, s *controllertest.Session, path string, doc map[string]any) {
	t.Helper()
	payload := clone(doc)
	payload[probeUnknownKey] = "x"
	body, status, err := s.PostJSON(ctx, path, payload)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	stored := firstData(t, body)
	if id := objectID(stored); id != "" && status/100 == 2 {
		defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	}
	if status/100 != 2 {
		t.Errorf("POST %s refused an unrecognised key (HTTP %d, %s); v1 was measured stripping one, "+
			"and a controller that now rejects makes every v1 write stricter than the SDK assumes",
			path, status, v1ErrCode(body))
		return
	}
	if _, present := stored[probeUnknownKey]; present {
		t.Errorf("POST %s stored the unrecognised key %q; v1 was measured dropping it",
			path, probeUnknownKey)
		return
	}
	t.Logf("v1 %s accepts an unrecognised key and stores the document without it (HTTP %d) -- "+
		"a 2xx here does not mean the body was taken as sent", path, status)
}

// v2Rejection names why a v2 write was refused, in one line.
//
// Bean validation answers with a Spring paragraph under "message" that
// repeats the obfuscated handler's signature and then every failing
// constraint; v2ErrCode has no case for it and falls back to printing the
// whole envelope, which buries the one fact a reader wants. Pull the failing
// fields and their constraints out of it, and leave every other shape to
// v2ErrCode.
func v2Rejection(body any) string {
	if m, ok := body.(map[string]any); ok {
		if msg, _ := m["message"].(string); msg != "" {
			if fields := beanValidationFailures(msg); fields != "" {
				return fields
			}
		}
	}
	return v2ErrCode(body)
}

// beanValidationFailureRe matches one field error inside Spring's
// bean-validation message: the field's path and the constraint that fired.
var beanValidationFailureRe = regexp.MustCompile(`on field '([^']+)': [^;]*; codes \[([A-Za-z]+)\.`)

// beanValidationFailures renders the field errors as "field NotEmpty" pairs,
// or "" when the message is not a bean-validation one.
func beanValidationFailures(msg string) string {
	var out []string
	for _, m := range beanValidationFailureRe.FindAllStringSubmatch(msg, -1) {
		out = append(out, m[1]+" "+m[2])
	}
	return strings.Join(out, ", ")
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

// measureFirewallPolicyContract measures which fields a firewall-policy
// create cannot omit. The jar marks source.zone_id and destination.zone_id
// @NotNull on the write DTO; only what the controller actually rejects is
// recorded, spelled by dotted path.
func measureFirewallPolicyContract(ctx context.Context, t *testing.T, s *controllertest.Session, site string) behavior.WriteContract {
	t.Helper()

	src, dst := firewallZonePair(ctx, t, s, site)
	if src == "" {
		t.Fatal("no firewall zones; every policy create would fail for the wrong reason")
	}
	path := "/v2/api/site/" + site + "/firewall-policies"

	// post creates a policy without the named side's zone_id (or the full
	// body for ""), deleting whatever the controller stored.
	n := 0
	post := func(strip string) (int, map[string]any) {
		n++
		doc := firewallPolicyProbeBase(fmt.Sprintf("contract-probe-%d", n), 22400+n, src, dst)
		if strip != "" {
			delete(doc[strip].(map[string]any), "zone_id")
		}
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		m, _ := body.(map[string]any)
		if id, _ := m["_id"].(string); id != "" {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, m
	}

	if status, body := post(""); status/100 != 2 {
		t.Fatalf("the known-good policy body was rejected (HTTP %d): %v\n\nNothing removed from a "+
			"body that does not create can measure anything.", status, body)
	}

	var required []string
	for _, side := range []string{"source", "destination"} {
		status, m := post(side)
		if status/100 == 2 {
			t.Logf("policy create without %s.zone_id accepted (HTTP %d) -- not required", side, status)
			continue
		}
		t.Logf("policy create without %s.zone_id rejected (HTTP %d, %s) -- required on create",
			side, status, v2ErrCode(m))
		required = append(required, side+".zone_id")
	}

	return behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "v2/api/site/{site}/firewall-policies",
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
		if len(w.MinItems) == 0 {
			w.MinItems = nil
		}
		return w
	}
	want, got = normalize(want), normalize(got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s write contract drifted:\n  artifact: %+v\n  measured: %+v\n\nEither the controller "+
			"changed or the artifact is stale; re-measure with BEHAVIOR_WRITE=1.", resource, want, got)
	}
}

// TestIntegrationNatUpdateEmptyVsAbsent measures, on the NAT update path
// (PUT v2/api/site/{site}/nat/{id}), whether the controller treats an empty
// string differently from an absent key for three optional fields a caller
// has to decide how to encode: ip_address, in_interface and description.
// The verdicts use the clearing probe's vocabulary and land in the
// artifact's Empty["nat"] section.
//
// The provider measured in_interface: "" rejected on update
// (NatRuleInvalidNetworkConf); this probe confirms or refutes that rather
// than assuming it.
//
// The rule under test is the known-good MASQUERADE create from
// measureNatContract. That shape constrains what can be measured: a field
// this rule type cannot hold a value for (the seed write below is how that
// is found out) has nothing stored to clear, so EMPTY-CLEARS versus
// EMPTY-IGNORED cannot be distinguished for it and the verdict describes
// what "" does on a rule where the field is empty -- which is still the
// case an encoder actually faces.
func TestIntegrationNatUpdateEmptyVsAbsent(t *testing.T) {
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
	base := map[string]any{
		"enabled": true, "type": "MASQUERADE", "ip_version": "IPV4",
		"protocol": "all", "out_interface": wanID,
		"source_filter": filter(), "destination_filter": filter(),
	}

	body, status, err := s.PostJSON(ctx, path, base)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	stored := firstData(t, body)
	id := objectID(stored)
	if status/100 != 2 || id == "" {
		t.Fatalf("the known-good NAT body did not create (HTTP %d): %v\n\nAn update probe "+
			"with nothing to update measures nothing.", status, body)
	}
	defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck

	put := func(doc map[string]any) (map[string]any, int) {
		body, status, err := s.PutJSON(ctx, path+"/"+id, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		if status != 200 {
			// The rejection is the measurement; its body is the reason.
			t.Logf("PUT %s/%s -> HTTP %d: %v (%v)", path, id, status, body, err)
		}
		return firstData(t, body), status
	}

	probes := []struct{ field, seed string }{
		// A MASQUERADE rule translates to the outbound interface's own
		// address, so ip_address may refuse any value here; the seed write
		// measures that instead of assuming it.
		{"ip_address", "192.0.2.10"},
		{"in_interface", wanID},
		{"description", "clear-probe"},
	}

	measured := map[string]behavior.EmptySemantics{}
	var summary []string
	for _, p := range probes {
		// Seed a real value first: without one stored, EMPTY-CLEARS and
		// EMPTY-IGNORED are the same observation. The clearing probe solves
		// this by only probing fields the seed populated; here the fields
		// are fixed, so the limitation is logged instead.
		baseline, original := stored, ""
		seedDoc := clone(stored)
		seedDoc[p.field] = p.seed
		if after, st := put(seedDoc); st == 200 {
			if got, _ := after[p.field].(string); got != "" {
				baseline, original = after, got
			} else {
				t.Logf("%s: seed %q accepted but not stored; measuring against the bare rule", p.field, p.seed)
			}
		} else {
			t.Logf("%s: this rule cannot hold %q (HTTP %d); measuring against the bare rule", p.field, p.seed, st)
			put(clone(stored)) // in case the rejected write left partial state
		}

		// Empty string.
		doc := clone(baseline)
		doc[p.field] = ""
		after, st := put(doc)
		empty := "EMPTY-REJECTED"
		if st == 200 {
			if got, _ := after[p.field].(string); got == "" {
				empty = "EMPTY-CLEARS"
			} else if got == original {
				empty = "EMPTY-IGNORED"
			} else {
				empty = "EMPTY-REPLACED-" + got
			}
		}

		put(clone(baseline)) // reset

		// Absent key.
		doc = clone(baseline)
		delete(doc, p.field)
		after, st = put(doc)
		omit := "OMIT-REJECTED"
		if st == 200 {
			if got, _ := after[p.field].(string); got == "" {
				omit = "OMIT-CLEARS"
			} else if got == original {
				omit = "OMIT-KEEPS"
			} else {
				omit = "OMIT-REPLACED-" + got
			}
		}

		put(clone(stored)) // back to the bare rule for the next field

		measured[p.field] = behavior.EmptySemantics{Empty: empty, Omit: omit}
		summary = append(summary, fmt.Sprintf("%-16s %-16s %s", p.field, empty, omit))
	}
	t.Logf("NAT update empty-vs-absent semantics:\n  %s", strings.Join(summary, "\n  "))

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Empty == nil {
				a.Empty = map[string]map[string]behavior.EmptySemantics{}
			}
			if a.Empty["nat"] == nil {
				a.Empty["nat"] = map[string]behavior.EmptySemantics{}
			}
			// Per-field upsert, like the clearing probe: this run measures
			// only its three fields and must not erase others.
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
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	for f, got := range measured {
		want, has := pinned[f]
		if !has {
			t.Errorf("nat.%s: measured empty=%s omit=%s but the artifact pins nothing; "+
				"re-measure with BEHAVIOR_WRITE=1", f, got.Empty, got.Omit)
			continue
		}
		if want != got {
			t.Errorf("nat.%s: artifact pins empty=%s omit=%s, measured empty=%s omit=%s; "+
				"re-measure with BEHAVIOR_WRITE=1 once the change is understood",
				f, want.Empty, want.Omit, got.Empty, got.Omit)
		}
	}
}

// TestIntegrationHotspotPackageWriteContract measures the hotspot package,
// which no section of the artifact described: the SDK has shipped a client
// for it since the schema was captured and nothing had ever written one.
//
// It owns three artifact entries outright -- the write contract, the
// discard list and the coercion map -- because a second probe replacing any
// of them would erase what this one measured.
//
// The resource is a v1 rest collection, so the verbs and paths are the
// generic ones; what needed measuring is which fields a create cannot omit.
// The controller's own sanitizer (com.ubnt.data.isqI) puts an exclusive-or
// over the two duration fields, keyed on amount:
//
//	amount == 0  ->  trial_duration_minutes required, hours refused
//	amount != 0  ->  hours required, trial_duration_minutes refused
//
// so "required on create" is not one set -- it depends on which branch the
// body is in. The probe measures the free-trial branch, whose known-good
// body this harness accepts, and asserts both halves of the exclusive-or
// directly. The paid branch is measured and NOT recorded: see the loud log
// below and the comment on paidHotspotPackage.
func TestIntegrationHotspotPackageWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/api/s/" + c.Site + "/rest/hotspotpackage"

	// A free-trial package (amount absent, so amount == 0) carrying one
	// value of every kind the schema has: the duration pair, the rate and
	// quota limits, and the payment-field booleans.
	base := func(name string) map[string]any {
		return map[string]any{
			"name":                          name,
			"trial_duration_minutes":        60,
			"trial_reset":                   24,
			"limit_overwrite":               true,
			"limit_up":                      1024,
			"limit_down":                    2048,
			"limit_quota":                   500,
			"index":                         90,
			"custom_payment_fields_enabled": true,
			"payment_fields_email_enabled":  true,
			"payment_fields_email_required": true,
		}
	}

	post := func(doc map[string]any) (int, any, map[string]any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		stored := firstData(t, body)
		// A v1 rejection echoes the offending document under data rather
		// than a stored one, so only a 2xx carrying an id created anything.
		if id := objectID(stored); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, body, stored
	}

	asked := base("hotspot-package-probe")
	status, body, created := post(asked)
	if status/100 != 2 {
		t.Fatalf("the known-good hotspot package was rejected (HTTP %d, %s): %v\n\nNothing removed "+
			"from a body that does not create can measure anything.", status, v1ErrCode(body), created)
	}

	// What the controller kept, changed and dropped, in the round-trip
	// probe's vocabulary. A CHANGED field is a coercion -- the controller
	// stored something other than what was written -- and a DROPPED one was
	// accepted and never persisted.
	dropped := []string{}
	coercions := map[string]behavior.Coercion{}
	for _, r := range probe.Classify(asked, created) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("DROPPED %-32s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			coercions[r.Wire] = behavior.Coercion{
				Wrote:  renderStoredValue(asked[r.Wire]),
				Stored: renderStoredValue(created[r.Wire]),
			}
			t.Logf("CHANGED %-32s (%s)", r.Wire, r.Detail)
		}
	}
	sort.Strings(dropped)
	t.Logf("hotspot package round trip: %d asked, %d dropped, %d coerced",
		len(asked), len(dropped), len(coercions))

	// Why the classification above is the measurement and the 200 is not:
	// this collection accepts a key it does not recognise and stores the
	// document without it, so a create answering 200 says nothing on its
	// own about what reached the database.
	unknownKeyStripped(ctx, t, s, path, base("hotspot-package-unknown-key"))

	// The generic v1 verbs, confirmed rather than assumed: the generated
	// client lists the collection, reads by id, creates on the collection
	// and updates by id, and none of that had ever been exercised.
	if listBody, listStatus, err := s.GetJSON(ctx, path); listStatus != 200 {
		t.Errorf("GET %s answered HTTP %d (%v %v); the generated list reads that path",
			path, listStatus, listBody, err)
	}
	updateVerb, updatePath := "", ""
	seedBody, seedStatus, err := s.PostJSON(ctx, path, base("hotspot-package-update-probe"))
	if seedStatus/100 != 2 {
		t.Fatalf("seeding a package to update failed (HTTP %d): %v %v", seedStatus, seedBody, err)
	}
	seeded := firstData(t, seedBody)
	if id := objectID(seeded); id != "" {
		defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		if got, getStatus, err := s.GetJSON(ctx, path+"/"+id); getStatus != 200 {
			t.Errorf("GET %s/%s answered HTTP %d (%v %v); the generated read uses that path",
				path, id, getStatus, got, err)
		}
		edited := clone(seeded)
		edited["name"] = "hotspot-package-updated"
		after, putStatus, err := s.PutJSON(ctx, path+"/"+id, edited)
		if putStatus/100 == 2 {
			updateVerb, updatePath = "PUT", "api/s/{site}/rest/hotspotpackage/{id}"
			t.Logf("PUT %s/{id} -> HTTP %d; that is the update", path, putStatus)
		} else {
			t.Errorf("PUT %s/%s answered HTTP %d (%v %v); the generated update writes to that path",
				path, id, putStatus, after, err)
		}
	} else {
		t.Errorf("the seeded package carries no id, so the update path cannot be measured: %v", seeded)
	}

	fields := make([]string, 0, len(asked))
	for f := range asked {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	var required []string
	for i, field := range fields {
		doc := base(fmt.Sprintf("hotspot-package-probe-%d", i))
		delete(doc, field)
		status, body, _ := post(doc)
		if status/100 == 2 {
			t.Logf("hotspot package create without %-32s accepted (HTTP %d) -- not required", field, status)
			continue
		}
		t.Logf("hotspot package create without %-32s rejected (HTTP %d, %s) -- required on create",
			field, status, v1ErrCode(body))
		required = append(required, field)
	}
	sort.Strings(required)

	// The other half of the sanitizer's exclusive-or: a free-trial package
	// may not also carry hours. That is a refusal to accept a field, which
	// required_on_create has no way to say, so it is asserted here and
	// named in the doc comment rather than recorded as a value.
	withHours := base("hotspot-package-both-durations")
	withHours["hours"] = 2
	if status, body, _ := post(withHours); status/100 == 2 {
		t.Errorf("a free-trial package carrying hours was accepted (HTTP %d); the sanitizer's "+
			"exclusive-or over the duration fields no longer holds", status)
	} else {
		t.Logf("free trial carrying hours rejected (HTTP %d, %s) -- the duration fields are exclusive",
			status, v1ErrCode(body))
	}

	paidHotspotPackage(ctx, t, s, path)

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "api/s/{site}/rest/hotspotpackage",
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: required,
	}

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Writes == nil {
				a.Writes = map[string]behavior.WriteContract{}
			}
			a.Writes["HotspotPackage"] = contract
			if a.Discarded == nil {
				a.Discarded = map[string][]string{}
			}
			if a.Coercions == nil {
				a.Coercions = map[string]map[string]behavior.Coercion{}
			}
			// Replace, not merge: the probe writes the same body every run,
			// so a field that stopped being dropped or coerced has to leave
			// the artifact rather than linger as a fact nothing re-measures.
			a.Discarded["HotspotPackage"] = dropped
			a.Coercions["HotspotPackage"] = coercions
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok {
		t.Logf("no artifact at %s; run with BEHAVIOR_WRITE=1 to record the hotspot package", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	compareWriteContract(t, "HotspotPackage", art.Writes, contract)
	if pinned, has := art.Discarded["HotspotPackage"]; has && !reflect.DeepEqual(pinned, dropped) {
		t.Errorf("HotspotPackage discard list drifted:\n  artifact: %v\n  measured: %v\n\n"+
			"re-measure with BEHAVIOR_WRITE=1 once the change is understood", pinned, dropped)
	}
	if pinned, has := art.Coercions["HotspotPackage"]; has && !reflect.DeepEqual(pinned, coercions) {
		t.Errorf("HotspotPackage coercions drifted:\n  artifact: %v\n  measured: %v\n\n"+
			"re-measure with BEHAVIOR_WRITE=1 once the change is understood", pinned, coercions)
	}
}

// paidHotspotPackage measures the sanitizer's other branch -- amount != 0,
// priced by the hour -- and deliberately records nothing.
//
// The branch is reachable in principle: the sanitizer wants hours present
// and trial_duration_minutes absent, which the body below satisfies, and
// every field matches its own validator pattern. On this harness the create
// is refused anyway, with a bare api.err.Invalid carrying no detail, and it
// stays refused with the site's guest-access payment gateway switched on --
// so the refusal was never narrowed to a missing prerequisite and no
// required-on-create set for the paid branch could be established. Writing
// one anyway would publish a shape nothing here observed the controller
// accept, so the probe logs what it saw and the artifact stays silent about
// paid packages.
func paidHotspotPackage(ctx context.Context, t *testing.T, s *controllertest.Session, path string) {
	t.Helper()
	paid := map[string]any{
		"name": "hotspot-package-paid-probe", "amount": 5.0,
		"currency": "USD", "charged_as": "hour", "hours": 24,
	}
	body, status, err := s.PostJSON(ctx, path, paid)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if id := objectID(firstData(t, body)); id != "" && status/100 == 2 {
		s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	}
	if status/100 == 2 {
		t.Logf("LOUD: a paid hotspot package now creates (HTTP %d). The artifact records nothing for "+
			"the paid branch because nothing could measure it; it can be measured now.", status)
		return
	}
	t.Logf("LOUD: the paid branch stays unmeasured. A priced package (%v) is refused here with HTTP %d "+
		"%q and no further detail, so its required-on-create set is unknown and the artifact "+
		"records only the free-trial branch.", paid, status, v1ErrCode(body))
}

// v1ErrCode names why a v1 rest write was refused. A validation failure
// echoes the offending document under data[0] with its own msg, which says
// which rule fired; the envelope's meta.msg is the generic code and is all
// there is when the controller sent no document. Prefer the specific one.
func v1ErrCode(body any) string {
	if data := probe.FirstData(body); data != nil {
		if msg, _ := data["msg"].(string); msg != "" {
			return msg
		}
	}
	if envelope, ok := body.(map[string]any); ok {
		if meta, ok := envelope["meta"].(map[string]any); ok {
			if msg, _ := meta["msg"].(string); msg != "" {
				return msg
			}
		}
	}
	return "no reason given"
}

// TestIntegrationOSPFRouterWriteContract measures the OSPF router write
// contract into the artifact's Writes section. The generated CRUD says POST
// v2/api/site/{site}/ospf/router creates and PUT .../{id} updates; the
// create is verified live, and every top-level field of the known-good body
// is removed one at a time to find which the controller refuses to create
// without. The struct marks them all optional, which is the codegen's
// guess-from-shape this section exists to replace.
//
// The known-good body comes from the gateway feature sweep (cmd/fields):
// ospf/router rejects an empty areas list and takes area_type as a
// lowercase enum, and the areas have to name a real network.
//
// Below the top level, the jar puts @Size(min=1) on an area's network_ids.
// The empty list and the absent key are measured separately, because
// bean-style size validation skips a null and an absent list could slip
// past @Size -- on 10.6.101 they converge, because the DTO materializes
// the missing list as [] before the check runs. The length floor lands as
// min_items, a distinct fact from required_on_create, which can only say
// whether the key may be omitted at all.
func TestIntegrationOSPFRouterWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	lanID := defaultLANNetworkID(ctx, t, s, c.Site)
	if lanID == "" {
		t.Fatal("no corporate network on the site; an OSPF area naming nothing would fail for the wrong reason")
	}

	path := "/v2/api/site/" + c.Site + "/ospf/router"
	base := map[string]any{
		"enabled":                       true,
		"router_id":                     "0.0.0.1",
		"announce_default_route":        false,
		"redistribute_bgp_routes":       false,
		"redistribute_connected_routes": false,
		"redistribute_static_routes":    false,
		"interfaces":                    []any{},
		"areas": []any{map[string]any{
			"area_id":     "0.0.0.0",
			"area_type":   "normal",
			"name":        "probe-area",
			"network_ids": []string{lanID},
		}},
	}

	post := func(doc map[string]any) (int, string, any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		if err != nil {
			t.Logf("POST %s -> HTTP %d (%v)", path, status, err)
		}
		return status, objectID(firstData(t, body)), body
	}
	// A leftover router would make the next create measure "one already
	// exists" instead of the removed field, so a failed delete is fatal. The
	// delete answers 204 with an empty body, so only the status can be
	// judged; the decode error a bodiless response draws is not a failure.
	deleteRouter := func(id string) {
		if body, status, err := s.DeleteJSON(ctx, path+"/"+id); status/100 != 2 {
			t.Fatalf("delete ospf/router/%s (HTTP %d): %v %v\n\nEvery later verdict would be "+
				"measured against a site that already has a router.", id, status, body, err)
		}
	}

	status, id, body := post(base)
	if status/100 != 2 {
		t.Fatalf("the known-good OSPF body was rejected (HTTP %d): %v\n\nNothing removed from a body "+
			"that does not create can measure anything.", status, body)
	}
	if id == "" {
		t.Fatalf("created OSPF router carries no id; it cannot be deleted and would poison the sweep: %v", body)
	}
	deleteRouter(id)

	// The router has no by-id read: GET on the {id} path, which PUT and
	// DELETE do serve, answers 405 at the route level, which is why the
	// generated get filters the list instead.
	if body, status, _ := s.GetJSON(ctx, path+"/000000000000000000000000"); status != 405 {
		t.Errorf("GET %s/{id} answered HTTP %d (%v); the controller grew a by-id read the "+
			"list-backed get routes around", path, status, body)
	}

	fields := make([]string, 0, len(base))
	for f := range base {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	var required []string
	for _, field := range fields {
		doc := clone(base)
		delete(doc, field)
		status, id, body := post(doc)
		if status/100 == 2 {
			if id != "" {
				deleteRouter(id)
			}
			t.Logf("OSPF create without %-32s accepted (HTTP %d) -- not required", field, status)
			continue
		}
		t.Logf("OSPF create without %-32s rejected (HTTP %d): %v -- required on create", field, status, body)
		required = append(required, field)
	}

	// The sweep above removes whole top-level keys and cannot see inside an
	// area. An area with its network_ids emptied, then removed, measures
	// the @Size floor and the absent key as the separate facts they are.
	areaVariant := func(mutate func(area map[string]any)) map[string]any {
		area := map[string]any{
			"area_id":     "0.0.0.0",
			"area_type":   "normal",
			"name":        "probe-area",
			"network_ids": []string{lanID},
		}
		mutate(area)
		doc := clone(base)
		doc["areas"] = []any{area}
		return doc
	}

	var minItems map[string]int
	if status, id, body := post(areaVariant(func(area map[string]any) {
		area["network_ids"] = []string{}
	})); status/100 == 2 {
		if id != "" {
			deleteRouter(id)
		}
		t.Logf("OSPF create with empty areas[].network_ids accepted (HTTP %d) -- no minimum", status)
	} else {
		t.Logf("OSPF create with empty areas[].network_ids rejected (HTTP %d, %s) -- one entry is "+
			"the measured minimum", status, v2ErrCode(body))
		minItems = map[string]int{"areas[].network_ids": 1}
	}

	if status, id, body := post(areaVariant(func(area map[string]any) {
		delete(area, "network_ids")
	})); status/100 == 2 {
		if id != "" {
			deleteRouter(id)
		}
		t.Logf("OSPF create with absent areas[].network_ids accepted (HTTP %d) -- not required", status)
	} else {
		t.Logf("OSPF create with absent areas[].network_ids rejected (HTTP %d, %s) -- required on "+
			"create", status, v2ErrCode(body))
		required = append(required, "areas[].network_ids")
	}
	sort.Strings(required)

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "v2/api/site/{site}/ospf/router",
		UpdateVerb: "PUT", UpdatePath: "v2/api/site/{site}/ospf/router/{id}",
		RequiredOnCreate: required,
		MinItems:         minItems,
	}

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Writes == nil {
				a.Writes = map[string]behavior.WriteContract{}
			}
			a.Writes["OSPFRouter"] = contract
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
	compareWriteContract(t, "OSPFRouter", art.Writes, contract)
}

// defaultLANNetworkID returns the site's stock corporate network, which is
// what an OSPF area can safely name.
func defaultLANNetworkID(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	nets, err := listNetworks(ctx, s, site)
	if err != nil {
		t.Fatalf("list networkconf: %v", err)
	}
	for _, obj := range nets {
		if purpose, _ := obj["purpose"].(string); purpose == PurposeCorporate {
			if id, _ := obj["_id"].(string); id != "" {
				return id
			}
		}
	}
	return ""
}

// TestIntegrationDevicePortOverridesDiscard writes one port override carrying
// a rich set of per-port members to an adopted switch and classifies, member
// by member, what the controller stored versus dropped. The dropped set goes
// into the artifact's Discarded["DevicePortOverrides"] section: fields the
// controller accepts with rc ok and does not persist, which a provider would
// otherwise report as permanent diffs.
//
// Unlike the Network discard list, CHANGED members are logged but not
// recorded: a stored-with-a-different-value member is a coercion, and filing
// it as discarded would tell the encoder to stop sending a field the
// controller does keep.
func TestIntegrationDevicePortOverridesDiscard(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	emulated := controllertest.StartDevices(ctx, t, c, controllertest.DeviceRequest{Model: "USM8P"})
	if len(emulated) != 1 {
		t.Skip("no emulated switch available for this controller target")
	}
	adopted := c.AdoptDevice(ctx, t, s, emulated[0].MAC)

	id := deviceIDForMAC(ctx, t, s, c.Site, adopted.MAC)
	if id == "" {
		t.Skipf("adopted %s but the controller lists no device with that MAC", adopted.MAC)
	}

	// One entry, many members: name/port_idx/poe_mode plus a spread of the
	// generated struct's member kinds -- mode strings, plain bools, and a
	// stormctrl level with its enable pair so the value is not dead config.
	asked := map[string]any{
		"port_idx":                1,
		"name":                    "discard-probe",
		"poe_mode":                "off",
		"op_mode":                 "switch",
		"setting_preference":      "manual",
		"autoneg":                 true,
		"isolation":               true,
		"eee_enabled":             true,
		"stp_port_mode":           true,
		"stormctrl_type":          "level",
		"stormctrl_bcast_enabled": true,
		"stormctrl_bcast_level":   42,
	}
	if body, status, err := s.PutJSON(ctx, "/api/s/"+c.Site+"/rest/device/"+id,
		map[string]any{"port_overrides": []any{asked}}); err != nil || status != 200 {
		t.Fatalf("writing the probe override (HTTP %d): %v %v", status, body, err)
	}
	defer func() {
		if _, status, err := s.PutJSON(context.WithoutCancel(ctx), "/api/s/"+c.Site+"/rest/device/"+id,
			map[string]any{"port_overrides": []any{}}); err != nil || status != 200 {
			t.Logf("clearing the probe override failed (HTTP %d): %v", status, err)
		}
	}()

	// stat/device lags a write by a second or two; wait for this write's own
	// entry (identified by its name) rather than any stored override.
	var entry map[string]any
	deadline := time.Now().Add(30 * time.Second)
	for {
		entry = storedPortOverride(ctx, t, s, c.Site, adopted.MAC, 1)
		if name, _ := entry["name"].(string); name == "discard-probe" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the probe override never appeared on stat/device; last read: %v", entry)
		}
		time.Sleep(2 * time.Second)
	}

	// Non-nil so an all-kept measurement records as [] rather than null:
	// "measured, nothing dropped" and "never measured" must not render alike.
	dropped := []string{}
	for _, r := range probe.Classify(asked, entry) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("DROPPED %-26s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Logf("CHANGED %-26s (%s) -- stored, not discarded", r.Wire, r.Detail)
		}
	}
	sort.Strings(dropped)
	t.Logf("port override members: %d asked, %d dropped", len(asked), len(dropped))

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Discarded == nil {
				a.Discarded = map[string][]string{}
			}
			// Replace, not union: the probe sees the whole set every run,
			// and a union could never drop a field the controller stopped
			// discarding (recordDiscarded's reasoning).
			a.Discarded["DevicePortOverrides"] = dropped
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	pinned, has := art.Discarded["DevicePortOverrides"]
	if !ok || !has {
		t.Logf("no pinned discard list for DevicePortOverrides in %s; run with BEHAVIOR_WRITE=1 to record it", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	want := map[string]bool{}
	for _, wire := range pinned {
		want[wire] = true
	}
	got := map[string]bool{}
	for _, wire := range dropped {
		got[wire] = true
		if !want[wire] {
			t.Errorf("%s is now dropped from port overrides but the artifact does not list it; "+
				"re-measure with BEHAVIOR_WRITE=1", wire)
		}
	}
	for _, wire := range pinned {
		if !got[wire] {
			t.Errorf("%s is no longer dropped: the controller stored what was asked.\n"+
				"The controller's behaviour changed -- re-measure with BEHAVIOR_WRITE=1 once that is understood", wire)
		}
	}
}

// storedPortOverride reads the stored override entry for one port off
// stat/device, or nil while none is stored yet.
func storedPortOverride(ctx context.Context, t *testing.T, s *controllertest.Session, site, mac string, portIdx int) map[string]any {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/stat/device/"+mac)
	if err != nil || status != 200 {
		t.Fatalf("read stat/device/%s (HTTP %d): %v", mac, status, err)
	}
	device := firstData(t, body)
	overrides, _ := device["port_overrides"].([]any)
	for _, o := range overrides {
		entry, ok := o.(map[string]any)
		if !ok {
			continue
		}
		if jsonEqual(entry["port_idx"], portIdx) {
			return entry
		}
	}
	return nil
}

// TestIntegrationDeviceRadioTableWrites measures how the controller takes a
// write to a device's radio table -- the other array of objects in the device
// document, and the one no probe had touched.
//
// It owns four artifact entries, because nothing else writes them:
//
//	writes["DeviceRadioTable"]        the verb, the path, and the member an
//	                                  entry cannot leave out
//	empty["DeviceRadioTable"]         what "" and an absent key do to a member
//	empty["device"]["radio_table"]    what an empty array and an unnamed entry
//	                                  do to the array
//	discarded["DeviceRadioTable"]     members every radio takes and drops
//
// The omit verdicts are the half that matters. port_overrides, written by
// the same PUT to the same document, replaces on both levels: an entry the
// payload omits is dropped and a member an entry omits is dropped from that
// entry, which is why UpdateDevicePortOverrides reads the stored array and
// resends it. If radio_table behaved the same way, UpdateDeviceRadioTable
// would have to do the same, and these two verdicts are what says whether it
// does.
func TestIntegrationDeviceRadioTableWrites(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	emulated := controllertest.StartDevices(ctx, t, c, controllertest.DeviceRequest{Model: "U7PRO"})
	if len(emulated) != 1 {
		t.Skip("no emulated access point available for this controller target")
	}
	adopted := c.AdoptDevice(ctx, t, s, emulated[0].MAC)
	id := deviceIDForMAC(ctx, t, s, c.Site, adopted.MAC)
	if id == "" {
		t.Skipf("adopted %s but the controller lists no device with that MAC", adopted.MAC)
	}
	writePath := "/api/s/" + c.Site + "/rest/device/" + id

	// A slice, never a variadic: an empty radio_table has to be written as
	// [], and a nil slice marshals as null, which the controller rejects
	// outright (api.err.InvalidPayload) as it does for port_overrides.
	put := func(entries []map[string]any) int {
		body, status, err := s.PutJSON(ctx, writePath, map[string]any{"radio_table": entries})
		if status == 0 {
			t.Fatalf("transport to %s: %v", writePath, err)
		}
		if status != 200 {
			// The rejection is the measurement; its body is the reason.
			t.Logf("PUT radio_table %v -> HTTP %d: %v", entries, status, body)
		}
		return status
	}

	radios := func() map[string]map[string]any {
		body, status, err := s.GetJSON(ctx, "/api/s/"+c.Site+"/stat/device/"+adopted.MAC)
		if err != nil || status != 200 {
			t.Fatalf("read stat/device/%s (HTTP %d): %v", adopted.MAC, status, err)
		}
		raw, _ := json.Marshal(firstData(t, body)["radio_table"])
		var entries []map[string]any
		if err := json.Unmarshal(raw, &entries); err != nil {
			t.Fatalf("radio_table is not an array of objects: %v", err)
		}
		out := make(map[string]map[string]any, len(entries))
		for _, entry := range entries {
			name, _ := entry["name"].(string)
			out[name] = entry
		}
		return out
	}

	// Every verdict here is read from a settled table, and settling takes two
	// steps rather than one.
	//
	// stat/device lags a write by a second or two, so a read taken straight
	// after a PUT can still show the table as it was. And once the write does
	// appear, it is not yet what survives: the AP informs every few seconds,
	// the controller answers by provisioning the radios, and the AP then
	// reports the configuration it was given -- which replaces radio_table in
	// the document. A member the controller accepted but does not provision
	// therefore sits in the document for one inform and then vanishes.
	// Measured: dfs and a chosen antenna_id both do exactly that, so a probe
	// that read straight after the write would record them as kept and a
	// caller would find them gone.
	//
	// So: wait for the write's own value where there is one, then wait for
	// the entry to stop moving.
	settle := func(radio string) map[string]map[string]any {
		t.Helper()
		const quiet = 4 // consecutive equal reads, three seconds apart
		deadline := time.Now().Add(2 * time.Minute)
		var last map[string]any
		same := 0
		for {
			table := radios()
			if last != nil && reflect.DeepEqual(table[radio], last) {
				if same++; same == quiet {
					return table
				}
			} else {
				same = 0
			}
			last = table[radio]
			if time.Now().After(deadline) {
				t.Fatalf("%s never stopped changing, so nothing read from it is what survives; last: %v",
					radio, last)
			}
			time.Sleep(3 * time.Second)
		}
	}
	settled := func(radio, wire string, want any) map[string]map[string]any {
		t.Helper()
		deadline := time.Now().Add(60 * time.Second)
		for !jsonEqual(radios()[radio][wire], want) {
			if time.Now().After(deadline) {
				t.Fatalf("%s never reported %s = %v; last read: %v", radio, wire, want, radios()[radio])
			}
			time.Sleep(2 * time.Second)
		}
		return settle(radio)
	}

	var table map[string]map[string]any
	deadline := time.Now().Add(3 * time.Minute)
	for {
		table = radios()
		if len(table) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the adopted AP never reported a radio_table, so nothing here can be measured")
		}
		time.Sleep(3 * time.Second)
	}
	names := make([]string, 0, len(table))
	for name := range table {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) < 2 || names[0] == "" {
		t.Skipf("the AP reports %v; two named radios are needed to tell an unnamed entry being "+
			"kept from one being dropped", names)
	}
	target, other := names[0], names[1]
	t.Logf("radios: %v (merge arms on %s and %s, discard sweep on all of them, last)", names, target, other)

	// An entry with no name, against the same entry with one: the pair is
	// what makes the rejection about the name rather than about the body.
	band := table[target]["radio"]
	if put([]map[string]any{{"name": target, "radio": band, "maxsta": 33}}) != 200 {
		t.Fatal("a named entry was refused; nothing below can attribute a rejection to the missing name")
	}
	requiredOnUpdate := []string{}
	if put([]map[string]any{{"radio": band, "maxsta": 34}}) != 200 {
		requiredOnUpdate = append(requiredOnUpdate, "name")
	}

	// Seed both radios with values the AP never reports on its own, so a
	// preserved setting cannot be confused with a re-reported one.
	if put([]map[string]any{{"name": target, "tx_power_mode": "custom", "tx_power": "17"}}) != 200 {
		t.Fatal("the seed write was refused, so the omit verdicts below have nothing to preserve")
	}
	settled(target, "tx_power_mode", "custom")
	if put([]map[string]any{{"name": other, "maxsta": 55}}) != 200 {
		t.Fatal("the second seed write was refused, so an unnamed entry's fate cannot be measured")
	}
	settled(other, "maxsta", 55)

	// A member written as "".
	memberEmpty := "EMPTY-REJECTED"
	if put([]map[string]any{{"name": target, "tx_power_mode": ""}}) == 200 {
		switch got, _ := settle(target)[target]["tx_power_mode"].(string); got {
		case "":
			memberEmpty = "EMPTY-CLEARS"
		case "custom":
			memberEmpty = "EMPTY-IGNORED"
		default:
			memberEmpty = "EMPTY-REPLACED-" + got
		}
	}

	// One write, two omissions: it names a member the seeded entry does not
	// carry, and names no entry at all for the other radio.
	memberOmit, entryOmit := "OMIT-REJECTED", "OMIT-REJECTED"
	if put([]map[string]any{{"name": target, "maxsta": 44}}) == 200 {
		after := settled(target, "maxsta", 44)
		switch got, _ := after[target]["tx_power_mode"].(string); got {
		case "custom":
			memberOmit = "OMIT-KEEPS"
		case "":
			memberOmit = "OMIT-CLEARS"
		default:
			memberOmit = "OMIT-REPLACED-" + got
		}
		switch entry, present := after[other]; {
		case !present:
			entryOmit = "OMIT-CLEARS"
		case jsonEqual(entry["maxsta"], 55):
			entryOmit = "OMIT-KEEPS"
		default:
			entryOmit = fmt.Sprintf("OMIT-REPLACED-%v", entry["maxsta"])
		}
	}

	// The array written as [].
	arrayEmpty := "EMPTY-REJECTED"
	if put([]map[string]any{}) == 200 {
		switch after := settle(target); {
		case len(after) == 0:
			arrayEmpty = "EMPTY-CLEARS"
		case jsonEqual(after[target]["maxsta"], 44) && jsonEqual(after[other]["maxsta"], 55):
			arrayEmpty = "EMPTY-IGNORED"
		default:
			arrayEmpty = fmt.Sprintf("EMPTY-REPLACED-%d-radios", len(after))
		}
	}

	t.Logf("radio_table semantics: member %s / %s, array %s / %s",
		memberEmpty, memberOmit, arrayEmpty, entryOmit)

	// One entry carrying a value of every member kind the generated struct
	// has: the channel pair, the power pair, the thresholds and their enable
	// bools, and an antenna choice. channel auto and a 20 MHz width, so the
	// sweep measures what the controller keeps rather than what the band it
	// lands on allows.
	sweepEntry := func(radio string) map[string]any {
		return map[string]any{
			"name": radio, "radio": table[radio]["radio"],
			"channel": "auto", "ht": 20,
			"tx_power_mode": "custom", "tx_power": "17",
			"antenna_gain": 6, "antenna_id": 4,
			"dfs": true, "hard_noise_floor_enabled": true,
			"loadbalance_enabled": true, "maxsta": 100,
			"min_rssi_enabled": true, "min_rssi": -80,
			"sens_level_enabled": true, "sens_level": -80,
			"vwire_enabled": false,
		}
	}

	// Every radio, not one: the verdict does vary by radio, because the
	// controller answers out of the hardware's own capabilities. antenna_gain
	// is coerced to each radio's built-in gain, so on the radio whose gain
	// happens to match what was asked it reads as kept and on the others as
	// changed. Only what every radio dropped is recorded, so the list holds
	// whichever radio a caller writes to.
	//
	// CHANGED members are logged but not recorded, as in the port-override
	// sweep: a member stored with a different value is a coercion, and filing
	// it as discarded would tell a caller to stop sending a member the
	// controller does keep.
	drops := map[string]int{}
	for _, radio := range names {
		asked := sweepEntry(radio)
		if put([]map[string]any{asked}) != 200 {
			t.Fatalf("the sweep write for %s was refused outright, so no member of it can be classified", radio)
		}
		stored := settled(radio, "maxsta", 100)[radio]
		for _, r := range probe.Classify(asked, stored) {
			switch r.Verdict {
			case probe.Dropped:
				drops[r.Wire]++
				t.Logf("DROPPED %-26s on %-8s (%s)", r.Wire, radio, r.Detail)
			case probe.Changed:
				t.Logf("CHANGED %-26s on %-8s (%s) -- stored, not discarded", r.Wire, radio, r.Detail)
			}
		}
	}

	// Non-nil so an all-kept measurement records as [] rather than null:
	// "measured, nothing dropped" and "never measured" must not render alike.
	dropped := []string{}
	for wire, bands := range drops {
		if bands == len(names) {
			dropped = append(dropped, wire)
			continue
		}
		t.Logf("%s was dropped by %d of this AP's %d radios, so it is a band's answer rather than "+
			"the type's: logged, not recorded", wire, bands, len(names))
	}
	sort.Strings(dropped)
	t.Logf("radio members: %d asked of each radio, %d dropped by all of them", len(sweepEntry(target)), len(dropped))

	contract := behavior.WriteContract{
		UpdateVerb:       "PUT",
		UpdatePath:       "api/s/{site}/rest/device/{id}",
		RequiredOnUpdate: requiredOnUpdate,
	}
	member := behavior.EmptySemantics{Empty: memberEmpty, Omit: memberOmit}
	array := behavior.EmptySemantics{Empty: arrayEmpty, Omit: entryOmit}

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Writes == nil {
				a.Writes = map[string]behavior.WriteContract{}
			}
			if a.Empty == nil {
				a.Empty = map[string]map[string]behavior.EmptySemantics{}
			}
			if a.Empty["DeviceRadioTable"] == nil {
				a.Empty["DeviceRadioTable"] = map[string]behavior.EmptySemantics{}
			}
			if a.Empty["device"] == nil {
				a.Empty["device"] = map[string]behavior.EmptySemantics{}
			}
			if a.Discarded == nil {
				a.Discarded = map[string][]string{}
			}
			// Replace, not merge: the probe writes the same bodies every
			// run, so a member that stopped being dropped has to leave the
			// artifact rather than linger as a fact nothing re-measures.
			a.Writes["DeviceRadioTable"] = contract
			a.Empty["DeviceRadioTable"]["tx_power_mode"] = member
			a.Empty["device"]["radio_table"] = array
			a.Discarded["DeviceRadioTable"] = dropped
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok {
		t.Logf("no artifact at %s; run with BEHAVIOR_WRITE=1 to record the radio table", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	compareWriteContract(t, "DeviceRadioTable", art.Writes, contract)
	for _, pin := range []struct {
		section, field string
		got            behavior.EmptySemantics
	}{
		{"DeviceRadioTable", "tx_power_mode", member},
		{"device", "radio_table", array},
	} {
		want, has := art.Empty[pin.section][pin.field]
		if !has {
			t.Logf("no pinned semantics for %s.%s; run with BEHAVIOR_WRITE=1 to record them",
				pin.section, pin.field)
			continue
		}
		if want != pin.got {
			t.Errorf("%s.%s: artifact pins empty=%s omit=%s, measured empty=%s omit=%s.\n\n"+
				"A change away from OMIT-KEEPS is the one that matters: UpdateDeviceRadioTable "+
				"writes only what the caller named because the controller keeps the rest. "+
				"Re-measure with BEHAVIOR_WRITE=1 once the change is understood.",
				pin.section, pin.field, want.Empty, want.Omit, pin.got.Empty, pin.got.Omit)
		}
	}
	if pinned, has := art.Discarded["DeviceRadioTable"]; has && !reflect.DeepEqual(pinned, dropped) {
		t.Errorf("DeviceRadioTable discard list drifted:\n  artifact: %v\n  measured: %v\n\n"+
			"re-measure with BEHAVIOR_WRITE=1 once the change is understood", pinned, dropped)
	}
}

// storedEmptySemantics measures what writing a field as blank and leaving its
// key out do to one field of one stored object, in the clearing probe's
// vocabulary.
//
// Every verdict is read off a re-read of the stored document, never off the
// write's own response, and that is the whole point of the helper. The v1 rest
// PUT answers a write that changed nothing with an empty data array (measured
// on firewallgroup, portconf and networkconf), so a probe that reads a field
// out of the response sees "" for a field the controller did not touch and
// files it as cleared.
//
// blank is the field's empty form as it goes on the wire -- "" for a string,
// []any{} for a list. seed is the stored document to measure against and to
// restore between the two halves; it must carry a value for field, or CLEARS
// and KEEPS are the same observation.
func storedEmptySemantics(
	t *testing.T,
	field string,
	blank any,
	seed map[string]any,
	put func(map[string]any) int,
	read func() map[string]any,
) behavior.EmptySemantics {
	t.Helper()

	original := seed[field]
	if blankValue(original) {
		t.Errorf("%s is empty in the seed document, so clearing it measures nothing", field)
	}

	classify := func(prefix string, status int) string {
		if status/100 != 2 {
			return prefix + "-REJECTED"
		}
		switch got := read()[field]; {
		case blankValue(got):
			return prefix + "-CLEARS"
		case jsonEqual(got, original):
			if prefix == "EMPTY" {
				return "EMPTY-IGNORED"
			}
			return "OMIT-KEEPS"
		default:
			return fmt.Sprintf("%s-REPLACED-%v", prefix, got)
		}
	}

	doc := clone(seed)
	doc[field] = blank
	empty := classify("EMPTY", put(doc))
	put(clone(seed)) // reset

	doc = clone(seed)
	delete(doc, field)
	omit := classify("OMIT", put(doc))
	put(clone(seed)) // reset

	return behavior.EmptySemantics{Empty: empty, Omit: omit}
}

// blankValue reports whether a stored value is the empty form of its field:
// absent, JSON null, the empty string or the empty list. The controller
// spells a cleared field all four ways depending on the collection --
// firewallgroup drops the key, trafficroutes stores null for a string and []
// for a list -- and they all mean the same thing to a caller.
func blankValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	}
	return false
}

// TestIntegrationFirewallGroupWriteContract measures the firewall group, which
// no section of the artifact described: the SDK has shipped a client for the
// collection since the schema was captured and no probe had ever written one.
//
// It owns three artifact entries outright -- the write contract, the discard
// list and the empty/omit semantics -- because a second probe replacing any of
// them would erase what this one measured.
//
// The resource is a v1 rest collection, so the verbs and paths are the generic
// ones; they are confirmed here rather than assumed, and the rest is measured:
//
//   - which fields a create cannot omit. The struct marks all seven optional
//     and the controller refuses a body without name or group_type, each with
//     its own error code.
//   - the two branches the collection keeps apart, keyed on source. A static
//     group -- source "static", or absent, which is how the controller stores
//     a group that never named one -- is refused if it carries url or
//     update_interval_seconds. Those two fields belong to the dynamic branch,
//     which this controller refuses outright for every group type, so nothing
//     about them is recorded; see the loud log below.
//   - what "" and an absent key do. This is where the collection parts company
//     with the rest of the tree: its PUT merges, so leaving a key out preserves
//     the stored value rather than clearing it.
func TestIntegrationFirewallGroupWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/api/s/" + c.Site + "/rest/firewallgroup"

	// A static address group carrying one value of every kind the branch
	// admits: the name and type the create needs, two members, the free-text
	// description and the source selector.
	base := func(name string) map[string]any {
		return map[string]any{
			"name":          name,
			"group_type":    "address-group",
			"group_members": []any{"192.0.2.10", "198.51.100.0/24"},
			"description":   "firewall-group probe",
			"source":        "static",
		}
	}

	// post creates and deletes again, for the sweeps that only care whether
	// the controller took the body at all.
	post := func(doc map[string]any) (int, any, map[string]any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		// A v1 rejection echoes the offending document under data rather
		// than a stored one, so only a 2xx carrying an id created anything.
		created := firstData(t, body)
		if id := objectID(created); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, body, created
	}

	asked := base("firewall-group-probe")
	body, status, err := s.PostJSON(ctx, path, asked)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		t.Fatalf("the known-good firewall group was rejected (HTTP %d, %s): %v\n\nNothing removed "+
			"from a body that does not create can measure anything.", status, v1ErrCode(body), firstData(t, body))
	}
	id := objectID(firstData(t, body))
	if id == "" {
		t.Fatalf("the created group carries no id, so nothing below can re-read it: %v", body)
	}
	defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck

	// The stored document, read back from the collection. Every verdict below
	// comes from here rather than from a write's response: the create's
	// response happens to echo the document, the update's does not, and a
	// discard is only a discard if the collection is missing the field.
	read := func() map[string]any {
		got, status, err := s.GetJSON(ctx, path+"/"+id)
		if err != nil || status != 200 {
			t.Fatalf("GET %s/%s answered HTTP %d (%v); the generated read uses that path",
				path, id, status, err)
		}
		return firstData(t, got)
	}
	stored := read()

	// What the controller kept, changed and dropped. A CHANGED field is a
	// coercion -- the controller stored something other than what was written
	// -- and a DROPPED one was accepted and never persisted.
	dropped := []string{}
	for _, r := range probe.Classify(asked, stored) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("DROPPED %-16s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Logf("CHANGED %-16s (%s) -- stored, not discarded", r.Wire, r.Detail)
		}
	}
	sort.Strings(dropped)
	t.Logf("firewall group round trip: %d asked, %d dropped", len(asked), len(dropped))

	// Why the classification above is the measurement and the 200 is not:
	// this collection accepts a key it does not recognise and stores the
	// document without it.
	unknownKeyStripped(ctx, t, s, path, base("firewall-group-unknown-key"))

	// The generic v1 verbs, confirmed rather than assumed. read() has already
	// exercised the by-id GET; this is the collection the generated list reads.
	if listBody, listStatus, err := s.GetJSON(ctx, path); listStatus != 200 {
		t.Errorf("GET %s answered HTTP %d (%v %v); the generated list reads that path",
			path, listStatus, listBody, err)
	}
	updateVerb, updatePath := "", ""
	renamed := clone(stored)
	renamed["name"] = "firewall-group-probe-renamed"
	after, putStatus, err := s.PutJSON(ctx, path+"/"+id, renamed)
	if putStatus/100 != 2 {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v); the generated update writes to that path",
			path, id, putStatus, after, err)
	} else if got, _ := read()["name"].(string); got != "firewall-group-probe-renamed" {
		t.Errorf("PUT %s/{id} answered HTTP %d but the collection still reports name %q; the write "+
			"was accepted and not stored", path, putStatus, got)
	} else {
		updateVerb, updatePath = "PUT", "api/s/{site}/rest/firewallgroup/{id}"
		t.Logf("PUT %s/{id} -> HTTP %d; that is the update", path, putStatus)
	}
	s.PutJSON(ctx, path+"/"+id, clone(stored)) //nolint:errcheck // back to the seeded name

	fields := make([]string, 0, len(asked))
	for f := range asked {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	var required []string
	for i, field := range fields {
		doc := base(fmt.Sprintf("firewall-group-probe-%d", i))
		delete(doc, field)
		status, body, _ := post(doc)
		if status/100 == 2 {
			t.Logf("firewall group create without %-14s accepted (HTTP %d) -- not required", field, status)
			continue
		}
		t.Logf("firewall group create without %-14s rejected (HTTP %d, %s) -- required on create",
			field, status, v1ErrCode(body))
		required = append(required, field)
	}
	sort.Strings(required)

	firewallGroupSourceBranches(ctx, t, s, path, base)

	// The empty/omit half. The seed is the stored document, so each field has
	// a value to lose, and the helper restores it between the two halves.
	put := func(doc map[string]any) int {
		body, status, err := s.PutJSON(ctx, path+"/"+id, doc)
		if status == 0 {
			t.Fatalf("transport to %s/%s: %v", path, id, err)
		}
		if status/100 != 2 {
			// The rejection is the measurement; its body is the reason.
			t.Logf("PUT %s/%s -> HTTP %d: %s", path, id, status, v1ErrCode(body))
		}
		return status
	}
	measured := map[string]behavior.EmptySemantics{}
	for _, f := range []struct {
		wire  string
		blank any
	}{
		{"description", ""},
		{"group_members", []any{}},
		{"group_type", ""},
		{"name", ""},
		{"source", ""},
	} {
		measured[f.wire] = storedEmptySemantics(t, f.wire, f.blank, stored, put, read)
	}
	var summary []string
	for _, f := range slices.Sorted(maps.Keys(measured)) {
		summary = append(summary, fmt.Sprintf("%-16s %-16s %s", f, measured[f].Empty, measured[f].Omit))
	}
	t.Logf("firewall group empty-vs-absent semantics:\n  %s", strings.Join(summary, "\n  "))

	// The finding that makes this collection worth its own entry: the v1 PUT
	// merges, so a partial write leaves the rest of the group alone. The
	// artifact records the opposite for every other v1 collection, which is
	// worth knowing before reading these verdicts as the odd ones out: those
	// were read off the PUT's own response, and this collection was measured
	// answering a no-op write with an empty data array. See
	// storedEmptySemantics.
	for _, f := range slices.Sorted(maps.Keys(measured)) {
		if measured[f].Omit != "OMIT-KEEPS" {
			t.Logf("LOUD: %s now answers an omitted %s with %s rather than OMIT-KEEPS; the merge this "+
				"collection was measured doing no longer holds for every field", path, f, measured[f].Omit)
		}
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "api/s/{site}/rest/firewallgroup",
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: required,
	}

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Writes == nil {
				a.Writes = map[string]behavior.WriteContract{}
			}
			if a.Discarded == nil {
				a.Discarded = map[string][]string{}
			}
			if a.Empty == nil {
				a.Empty = map[string]map[string]behavior.EmptySemantics{}
			}
			// Replace, not merge: the probe writes the same bodies every run,
			// so a field that stopped being dropped -- or stopped being
			// measured at all -- has to leave the artifact rather than linger
			// as a fact nothing re-measures.
			a.Writes["FirewallGroup"] = contract
			a.Discarded["FirewallGroup"] = dropped
			a.Empty["firewallgroup"] = measured
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok {
		t.Logf("no artifact at %s; run with BEHAVIOR_WRITE=1 to record the firewall group", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	compareWriteContract(t, "FirewallGroup", art.Writes, contract)
	if pinned, has := art.Discarded["FirewallGroup"]; has && !reflect.DeepEqual(pinned, dropped) {
		t.Errorf("FirewallGroup discard list drifted:\n  artifact: %v\n  measured: %v\n\n"+
			"re-measure with BEHAVIOR_WRITE=1 once the change is understood", pinned, dropped)
	}
	compareEmptySemantics(t, "firewallgroup", art.Empty["firewallgroup"], measured)
}

// firewallGroupSourceBranches measures the sanitizer's static/dynamic split and
// deliberately records nothing about it.
//
// url and update_interval_seconds are the dynamic branch's fields: a static
// group carrying either is refused by name, and required_on_create has no way
// to say "refused when present". The dynamic branch itself is refused outright
// on this controller for every group type the schema names, with a bare
// api.err.FirewallGroupDynamicSourceUnsupported and no further detail, so no
// required-on-create set for it could be established. Writing one anyway would
// publish a shape nothing here observed the controller accept.
func firewallGroupSourceBranches(
	ctx context.Context,
	t *testing.T,
	s *controllertest.Session,
	path string,
	base func(string) map[string]any,
) {
	t.Helper()

	post := func(doc map[string]any) (int, any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		if id := objectID(firstData(t, body)); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, body
	}

	for _, f := range []struct{ wire, value string }{
		{"url", "https://example.com/list.txt"},
		{"update_interval_seconds", "3600"},
	} {
		doc := base("firewall-group-static-" + f.wire)
		doc[f.wire] = f.value
		if status, body := post(doc); status/100 == 2 {
			t.Errorf("a static firewall group carrying %s was accepted (HTTP %d); the field was "+
				"measured belonging to the dynamic branch alone", f.wire, status)
		} else {
			t.Logf("static group carrying %-24s rejected (HTTP %d, %s)", f.wire, status, v1ErrCode(body))
		}
	}

	for _, groupType := range []string{"address-group", "port-group", "ipv6-address-group", "domain-group"} {
		doc := base("firewall-group-dynamic-" + groupType)
		doc["group_type"] = groupType
		doc["group_members"] = []any{}
		doc["source"] = "dynamic"
		doc["url"] = "https://example.com/list.txt"
		doc["update_interval_seconds"] = "3600"
		status, body := post(doc)
		if status/100 == 2 {
			t.Logf("LOUD: a dynamic %s now creates (HTTP %d). The artifact records nothing for the "+
				"dynamic branch because nothing could measure it; it can be measured now.", groupType, status)
			continue
		}
		t.Logf("LOUD: the dynamic branch stays unmeasured for %-18s -- HTTP %d %q, no further detail, "+
			"so url and update_interval_seconds have no measured contract",
			groupType, status, v1ErrCode(body))
	}
}

// compareEmptySemantics checks one measured empty/omit map against the
// artifact, field by field. A field the artifact has never seen is an error
// rather than a log: this probe measures the same fixed set every run, so a
// missing pin means the artifact was written by an older probe and the new
// verdict is going unchecked.
func compareEmptySemantics(t *testing.T, section string, pinned, got map[string]behavior.EmptySemantics) {
	t.Helper()
	if pinned == nil {
		t.Logf("no pinned empty semantics for %s; run with BEHAVIOR_WRITE=1 to record them", section)
		return
	}
	for _, f := range slices.Sorted(maps.Keys(got)) {
		want, has := pinned[f]
		if !has {
			t.Errorf("%s.%s: measured empty=%s omit=%s but the artifact pins nothing; "+
				"re-measure with BEHAVIOR_WRITE=1", section, f, got[f].Empty, got[f].Omit)
			continue
		}
		if want != got[f] {
			t.Errorf("%s.%s: artifact pins empty=%s omit=%s, measured empty=%s omit=%s; "+
				"re-measure with BEHAVIOR_WRITE=1 once the change is understood",
				section, f, want.Empty, want.Omit, got[f].Empty, got[f].Omit)
		}
	}
}

// TestIntegrationTrafficRouteWriteContract measures the traffic route, the
// other resource the artifact said nothing about, and the one whose API
// generation had to be established before any of it could be read.
//
// It owns three artifact entries outright -- the write contract, the discard
// list and the empty/omit semantics -- because a second probe replacing any of
// them would erase what this one measured.
//
// The collection is v2 (v2/api/site/{site}/trafficroutes), and two of its
// answers are not what the generation predicts:
//
//   - it tolerates an unrecognised key. The v2 collections this file relies on
//     bind the body to a Jackson DTO that refuses one by name, which is what
//     lets their sweeps read a 2xx as "the controller took the body as sent".
//     This one answers 201 and stores the document without the key, exactly as
//     a v1 collection does, so every verdict here re-reads the collection.
//   - there is no by-id GET: the path answers 405, which is why the generated
//     GetTrafficRoute lists and filters instead.
//
// What the update does is the half that matters to a caller. The PUT binds the
// whole document: every field a payload leaves out comes back at its type's
// zero value -- null for a string, false for a bool, [] for a list -- and the
// four the DTO validates are refused outright. It is the opposite of the
// firewall group's merging PUT, and neither could be guessed from the resource
// shape.
//
// required_on_create is measured as the set no branch can do without. The
// destination filters are conditional and are asserted rather than recorded:
// matching_target selects which filter list has to be non-empty, and a flat
// list naming any of them would tell a caller addressing IPs to send domains.
func TestIntegrationTrafficRouteWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	// A route sends one network's traffic out of another, so both have to
	// exist: the demo site ships a LAN and no WAN.
	wanID := seedTrafficRouteWAN(ctx, t, s, c.Site)
	lanID := firstNetworkIDForPurpose(ctx, t, s, c.Site, PurposeCorporate)
	path := "/v2/api/site/" + c.Site + "/trafficroutes"

	// One route carrying a value of every kind the struct has: the three
	// destination filter lists and the region list, the ports and port ranges
	// inside a filter, the two free-text strings and both bools. It matches on
	// IP so that no single filter list is the one matching_target requires,
	// which is what lets the omission sweeps below say something about each of
	// them.
	base := func(description string) map[string]any {
		return map[string]any{
			"description":         description,
			"enabled":             true,
			"network_id":          wanID,
			"kill_switch_enabled": true,
			"matching_target":     "IP",
			"next_hop":            "192.0.2.1",
			"domains": []any{map[string]any{
				"domain":      "example.com",
				"ports":       []any{8080},
				"port_ranges": []any{map[string]any{"port_start": 100, "port_stop": 200}},
			}},
			"ip_addresses": []any{map[string]any{
				"ip_or_subnet": "192.0.2.0/24", "ip_version": "v4", "ports": []any{443},
			}},
			"ip_ranges": []any{map[string]any{
				"ip_start": "198.51.100.10", "ip_stop": "198.51.100.20", "ip_version": "v4",
			}},
			"regions":        []any{"US"},
			"target_devices": []any{map[string]any{"type": "NETWORK", "network_id": lanID}},
		}
	}

	// storedRoute reads one route back out of the collection. There is no
	// by-id GET (measured below), so the listing is the only stored document
	// there is.
	storedRoute := func(id string) map[string]any {
		body, status, err := s.GetJSON(ctx, path)
		if err != nil || status != 200 {
			t.Fatalf("GET %s answered HTTP %d (%v); the generated list reads that path", path, status, err)
		}
		for _, r := range asSlice(body) {
			if m, _ := r.(map[string]any); m != nil && objectID(m) == id {
				return m
			}
		}
		return nil
	}

	// post creates, reads the document back out of the collection and deletes
	// it again. The read-back is the whole discipline of this probe in one
	// place: the collection tolerates keys it does not know, so the response
	// to a create is not evidence that what was sent is what was stored.
	post := func(doc map[string]any) (int, any, map[string]any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		id := objectID(firstData(t, body))
		if id == "" || status/100 != 2 {
			return status, body, nil
		}
		created := storedRoute(id)
		s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		return status, body, created
	}

	asked := base("traffic-route-probe")
	body, status, err := s.PostJSON(ctx, path, asked)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		t.Fatalf("the known-good traffic route was rejected (HTTP %d, %s)\n\nNothing removed from a "+
			"body that does not create can measure anything.", status, v2Rejection(body))
	}
	id := objectID(firstData(t, body))
	if id == "" {
		t.Fatalf("the created route carries no id, so nothing below can re-read it: %v", body)
	}
	defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	t.Logf("POST %s -> HTTP %d; that is the create", path, status)

	stored := storedRoute(id)
	if stored == nil {
		t.Fatalf("the route created at %s is not in the collection; nothing below can be measured", path)
	}

	dropped := []string{}
	for _, r := range probe.Classify(asked, stored) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("DROPPED %-20s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Logf("CHANGED %-20s (%s) -- stored, not discarded", r.Wire, r.Detail)
		}
	}
	sort.Strings(dropped)
	t.Logf("traffic route round trip: %d asked, %d dropped", len(asked), len(dropped))

	// The generation's own assumption, measured rather than carried over: this
	// v2 collection does NOT reject an unrecognised key, so a 2xx from the
	// sweeps below says the controller stored something, not that it stored
	// what was asked -- which is why each of them re-reads.
	unknown := base("traffic-route-unknown-key")
	unknown[probeUnknownKey] = "x"
	unknownStatus, unknownBody, unknownStored := post(unknown)
	switch {
	case unknownStatus/100 != 2:
		t.Errorf("POST %s refused an unrecognised key (HTTP %d, %s); it was measured accepting one, "+
			"and a collection that now rejects makes every write stricter than the SDK assumes",
			path, unknownStatus, v2Rejection(unknownBody))
	case unknownStored == nil:
		t.Errorf("POST %s accepted an unrecognised key and the route is not in the collection: %v",
			path, unknownBody)
	default:
		if echoed, ok := unknownStored[probeUnknownKey]; ok {
			t.Errorf("POST %s stored the unrecognised key %q as %v; it was measured dropping it",
				path, probeUnknownKey, echoed)
		}
		t.Logf("LOUD: v2 %s accepts an unrecognised key and stores the document without it (HTTP %d). "+
			"The other v2 collections here refuse one by name; this one behaves like a v1 rest "+
			"collection, so a 2xx does not mean the body was taken as sent.", path, unknownStatus)
	}

	// No by-id GET. The generated client lists and filters because of this;
	// a controller that grows the route should be noticed rather than leave
	// the client on the longer path forever.
	if got, getStatus, _ := s.GetJSON(ctx, path+"/"+id); getStatus != 405 {
		t.Errorf("GET %s/{id} answered HTTP %d (%v), not the 405 it was measured giving; "+
			"GetTrafficRoute lists and filters because that route does not exist", path, getStatus, got)
	}

	// The update, confirmed rather than assumed, and confirmed against the
	// stored document: a rename that the collection does not report is not an
	// update.
	updateVerb, updatePath := "", ""
	renamed := clone(stored)
	renamed["description"] = "traffic-route-probe-renamed"
	after, putStatus, err := s.PutJSON(ctx, path+"/"+id, renamed)
	if putStatus/100 != 2 {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v); the generated update writes to that path",
			path, id, putStatus, after, err)
	} else if got, _ := storedRoute(id)["description"].(string); got != "traffic-route-probe-renamed" {
		t.Errorf("PUT %s/{id} answered HTTP %d but the collection still reports description %q; "+
			"the write was accepted and not stored", path, putStatus, got)
	} else {
		updateVerb, updatePath = "PUT", "v2/api/site/{site}/trafficroutes/{id}"
		t.Logf("PUT %s/{id} -> HTTP %d; that is the update", path, putStatus)
	}
	s.PutJSON(ctx, path+"/"+id, clone(stored)) //nolint:errcheck // back to the seeded description

	fields := make([]string, 0, len(asked))
	for f := range asked {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	var requiredOnCreate []string
	for i, field := range fields {
		doc := base(fmt.Sprintf("traffic-route-probe-%d", i))
		delete(doc, field)
		status, body, created := post(doc)
		if status/100 != 2 {
			t.Logf("traffic route create without %-20s rejected (HTTP %d, %s) -- required on create",
				field, status, v2Rejection(body))
			requiredOnCreate = append(requiredOnCreate, field)
			continue
		}
		// The DTO filled its own default in; naming what the collection then
		// holds says what a caller who omits the field actually gets.
		t.Logf("traffic route create without %-20s accepted (HTTP %d), stored %v -- not required",
			field, status, created[field])
	}
	sort.Strings(requiredOnCreate)

	put := func(doc map[string]any) int {
		body, status, err := s.PutJSON(ctx, path+"/"+id, doc)
		if status == 0 {
			t.Fatalf("transport to %s/%s: %v", path, id, err)
		}
		if status/100 != 2 {
			// The rejection is the measurement; its body is the reason.
			t.Logf("PUT %s/%s -> HTTP %d: %s", path, id, status, v2Rejection(body))
		}
		return status
	}
	var requiredOnUpdate []string
	for _, field := range fields {
		doc := clone(stored)
		delete(doc, field)
		if status := put(doc); status/100 != 2 {
			requiredOnUpdate = append(requiredOnUpdate, field)
			t.Logf("traffic route update without %-20s rejected (HTTP %d) -- required on update", field, status)
		} else {
			t.Logf("traffic route update without %-20s accepted, stored %v",
				field, storedRoute(id)[field])
		}
		put(clone(stored)) // reset
	}
	sort.Strings(requiredOnUpdate)

	trafficRouteMatchingBranches(ctx, t, s, path, wanID, lanID)
	trafficRouteEmptyTargetKeys(ctx, t, s, path, wanID, lanID)

	// The empty/omit half, for the two fields whose verdict does not depend on
	// which filter matching_target selects. The filter lists are conditional --
	// see trafficRouteMatchingBranches -- so recording an emptied one would
	// pin a verdict that only holds for the branch this seed happens to be in.
	measured := map[string]behavior.EmptySemantics{}
	for _, field := range []string{"description", "next_hop"} {
		measured[field] = storedEmptySemantics(t, field, "", stored, put, func() map[string]any {
			return storedRoute(id)
		})
	}
	var summary []string
	for _, f := range slices.Sorted(maps.Keys(measured)) {
		summary = append(summary, fmt.Sprintf("%-16s %-16s %s", f, measured[f].Empty, measured[f].Omit))
	}
	t.Logf("traffic route empty-vs-absent semantics:\n  %s", strings.Join(summary, "\n  "))

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "v2/api/site/{site}/trafficroutes",
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: requiredOnCreate,
		RequiredOnUpdate: requiredOnUpdate,
	}

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Writes == nil {
				a.Writes = map[string]behavior.WriteContract{}
			}
			if a.Discarded == nil {
				a.Discarded = map[string][]string{}
			}
			if a.Empty == nil {
				a.Empty = map[string]map[string]behavior.EmptySemantics{}
			}
			// Replace, not merge: the probe writes the same bodies every run,
			// so a field that stopped being dropped -- or stopped being
			// measured at all -- has to leave the artifact rather than linger
			// as a fact nothing re-measures.
			a.Writes["TrafficRoute"] = contract
			a.Discarded["TrafficRoute"] = dropped
			a.Empty["trafficroutes"] = measured
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok {
		t.Logf("no artifact at %s; run with BEHAVIOR_WRITE=1 to record the traffic route", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	compareWriteContract(t, "TrafficRoute", art.Writes, contract)
	if pinned, has := art.Discarded["TrafficRoute"]; has && !reflect.DeepEqual(pinned, dropped) {
		t.Errorf("TrafficRoute discard list drifted:\n  artifact: %v\n  measured: %v\n\n"+
			"re-measure with BEHAVIOR_WRITE=1 once the change is understood", pinned, dropped)
	}
	compareEmptySemantics(t, "trafficroutes", art.Empty["trafficroutes"], measured)
}

// trafficRouteMatchingBranches measures the requirement required_on_create
// cannot hold: which destination filter a route must carry depends on
// matching_target, so the filter lists are individually optional and
// collectively not.
//
// Each row is a route addressed at one target with that target's own filter
// list left out, and the error the controller answered. The INTERNET row is
// the control: it names no filter list and creates, which is what says the
// other three refusals are about the selector rather than about a route
// needing a filter at all.
//
// The sibling probe in traffic_route_matching_integration_test.go measures the
// other direction -- that a route may carry filter lists the selector does not
// name -- so nothing here repeats it.
func trafficRouteMatchingBranches(
	ctx context.Context,
	t *testing.T,
	s *controllertest.Session,
	path, wanID, lanID string,
) {
	t.Helper()

	for _, tc := range []struct {
		target, filter, want string
	}{
		{"DOMAIN", "domains", "api.err.MissingDomain"},
		{"IP", "ip_addresses", "api.err.MissingIpAddressesOrIpRanges"},
		{"REGION", "regions", "api.err.MissingRegion"},
		{"INTERNET", "", ""},
	} {
		doc := map[string]any{
			"description":     "traffic-route-branch-" + tc.target,
			"enabled":         true,
			"network_id":      wanID,
			"matching_target": tc.target,
			"target_devices":  []any{map[string]any{"type": "NETWORK", "network_id": lanID}},
		}
		switch tc.filter {
		case "domains":
			doc["regions"] = []any{"US"}
		case "ip_addresses":
			doc["domains"] = []any{map[string]any{"domain": "example.com"}}
		case "regions":
			doc["domains"] = []any{map[string]any{"domain": "example.com"}}
		}
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		if id := objectID(firstData(t, body)); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		if tc.want == "" {
			if status/100 != 2 {
				t.Errorf("a route matching %s and naming no filter list was refused (HTTP %d, %s); "+
					"the filter requirements below are then not about the selector",
					tc.target, status, v2Rejection(body))
			} else {
				t.Logf("matching %-8s with no filter list at all creates (HTTP %d)", tc.target, status)
			}
			continue
		}
		got := v2Rejection(body)
		if status/100 == 2 {
			t.Errorf("a route matching %s without %s was accepted (HTTP %d); the filter list the "+
				"selector names was measured being required", tc.target, tc.filter, status)
			continue
		}
		if got != tc.want {
			t.Errorf("a route matching %s without %s was refused with %q, not %q; the requirement "+
				"is still there but its reason changed", tc.target, tc.filter, got, tc.want)
			continue
		}
		t.Logf("matching %-8s without %-12s -> %s", tc.target, tc.filter, got)
	}
}

// trafficRouteEmptyTargetKeys measures what an empty string does inside a
// target device, because recording network_id as required on create reaches
// further than the field it names.
//
// The generator drops omitempty from every field whose wire name a resource's
// required-on-create set holds, matched on the leaf (see withRequiredOnCreate),
// so recording the route's own network_id also flips
// target_devices[].network_id: the shipped client now sends "network_id": ""
// inside a target device that names a client, or all of them. That is only
// safe if the controller takes it.
//
// It does, and the neighbouring key is why the question is worth a
// measurement rather than an assumption: an empty client_mac in the same
// object is refused ("must be valid mac Address"), so the same flip on that
// field would break every route the SDK writes. The two are asserted together
// so a controller that starts validating network_id the same way is caught
// here rather than in a caller.
func trafficRouteEmptyTargetKeys(
	ctx context.Context,
	t *testing.T,
	s *controllertest.Session,
	path, wanID, lanID string,
) {
	t.Helper()

	route := func(description string, target map[string]any) map[string]any {
		return map[string]any{
			"description": description, "enabled": true,
			"network_id": wanID, "kill_switch_enabled": false,
			"matching_target": "INTERNET",
			"target_devices":  []any{target},
		}
	}

	for _, tc := range []struct {
		what   string
		target map[string]any
		accept bool
	}{
		{"all clients, empty network_id", map[string]any{
			"type": "ALL_CLIENTS", "network_id": "",
		}, true},
		{"one client, empty network_id", map[string]any{
			"type": "CLIENT", "client_mac": "00:11:22:33:44:55", "network_id": "",
		}, true},
		{"a network, empty client_mac", map[string]any{
			"type": "NETWORK", "network_id": lanID, "client_mac": "",
		}, false},
	} {
		body, status, err := s.PostJSON(ctx, path, route("traffic-route-target-probe", tc.target))
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		if id := objectID(firstData(t, body)); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		switch {
		case tc.accept && status/100 != 2:
			t.Errorf("a target device with %s was refused (HTTP %d, %s); the generated client sends "+
				"that shape for every target device since network_id was recorded required on create",
				tc.what, status, v2Rejection(body))
		case !tc.accept && status/100 == 2:
			t.Errorf("a target device with %s was accepted (HTTP %d); it was measured being refused, "+
				"and that refusal is why an empty member is not safe to send blindly", tc.what, status)
		case tc.accept:
			t.Logf("target device with %-30s accepted (HTTP %d)", tc.what, status)
		default:
			t.Logf("target device with %-30s refused (HTTP %d, %s)", tc.what, status, v2Rejection(body))
		}
	}
}

// TestIntegrationPortForwardWriteContract measures the port-forward
// collection, which no section of the artifact described: the SDK has
// shipped a client for it since the schema was captured and nothing had ever
// written one.
//
// It owns three artifact entries outright -- the write contract, the discard
// list and the clearing semantics -- because a second probe replacing any of
// them would erase what this one measured.
//
// Three findings are worth reading before the code.
//
// Nothing is required on create. A POST carrying {} is answered 200 with a
// document holding an id and nothing else. The field-by-field sweep and the
// empty body are both here because they are different claims: the sweep says
// no single field is load-bearing, the empty body says there is no
// at-least-one-of rule the sweep would miss.
//
// Two fields do draw a refusal in that sweep, and neither is recorded,
// because both refusals are about another field in the same body:
// src_limiting_type is refused as missing only while src_limiting_enabled is
// true, and src_firewall_group_id only while the type is "firewall_group".
// required_on_create cannot say "when", so filing them flat would publish
// them as rules a caller not limiting its source has to obey -- and would
// tell the generator to drop omitempty from src_limiting_type, putting ""
// into every create for the field's own pattern to refuse. Both conditions
// are measured below by relaxing the other field and watching the create
// succeed, which is what earns the drop.
//
// And omitting a key on update PRESERVES the stored value, as it does on
// firewallgroup. TestIntegrationClearingSemantics still asserts OMIT-CLEARS
// for networkconf, portconf and wlanconf and calls a merge the thing that
// would break the encoder's empty-string rule; those verdicts were read off
// the write's own response, which is what storedEmptySemantics exists to
// avoid. The verdict here is taken twice from different write shapes -- a
// full document with one key removed, and a body naming one field and
// nothing else -- so it does not rest on one request's quirk.
//
// One rewrite is measured and deliberately NOT recorded. Asking for
// src_limiting_type "firewall_group" stores "firewall_group" when src is
// "any" or absent, and stores "ip" when src carries an address -- the
// address wins. The coercion map is keyed by field alone and has no way to
// say "when src is set", so an entry there would read as "the controller
// always rewrites this", which is false and would tell a caller to stop
// sending a value the controller does honour. It is asserted below instead.
func TestIntegrationPortForwardWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/api/s/" + c.Site + "/rest/portforward"

	// The list has to be served before anything else here means what it
	// looks like: a 404 collection would make every verdict below "the route
	// is absent" wearing another status code's clothes.
	if body, status, err := s.GetJSON(ctx, path); status != 200 {
		t.Fatalf("GET %s answered HTTP %d (%v %v); the collection is not served, so no verb below "+
			"measures the write contract", path, status, body, err)
	}

	groupID := portForwardSourceGroup(ctx, t, s, c.Site)

	// A rule carrying one value of every kind the struct has: the address
	// and port pair on each side, the interface selection in both its
	// spellings, the two booleans, and source limiting by firewall group.
	// src stays "any" because an address in it overrides the asked
	// src_limiting_type, which would measure the override rather than the
	// round trip; the override is measured on its own below.
	base := func(name, dstPort string) map[string]any {
		return map[string]any{
			"name": name, "enabled": true, "log": true,
			"dst_port": dstPort, "fwd": "192.168.1.51", "fwd_port": "81",
			"proto": "tcp", "pfwd_interface": "wan", "destination_ip": "any",
			"destination_ips":       []any{map[string]any{"destination_ip": "any", "interface": "wan"}},
			"src":                   "any",
			"src_limiting_enabled":  true,
			"src_limiting_type":     "firewall_group",
			"src_firewall_group_id": groupID,
		}
	}

	post := func(doc map[string]any) (int, any, string) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		stored := firstData(t, body)
		// A v1 rejection echoes the offending document under data rather
		// than a stored one, so only a 2xx carrying an id created anything.
		id := ""
		if status/100 == 2 {
			id = objectID(stored)
		}
		return status, body, id
	}
	// read returns the STORED document, not the create response: a v1
	// collection answering 200 says it stored something, not that it stored
	// what was asked.
	read := func(id string) map[string]any {
		body, status, err := s.GetJSON(ctx, path+"/"+id)
		if status != 200 {
			t.Fatalf("GET %s/%s answered HTTP %d (%v %v); the generated read uses that path",
				path, id, status, body, err)
		}
		return firstData(t, body)
	}

	asked := base("port-forward-probe", "7001")
	status, body, id := post(asked)
	if status/100 != 2 || id == "" {
		t.Fatalf("the known-good port forward was rejected (HTTP %d, %s): %v\n\nNothing removed from "+
			"a body that does not create can measure anything.", status, v1ErrCode(body), body)
	}
	stored := read(id)

	// What the controller kept, changed and dropped, read from the stored
	// document. A CHANGED field is a rewrite and a DROPPED one was accepted
	// and never persisted; only the drops are recorded, because telling a
	// caller to stop sending a field the controller does keep is worse than
	// saying nothing.
	dropped := []string{}
	for _, r := range probe.Classify(asked, stored) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("DROPPED %-24s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Errorf("CHANGED %-24s (%s) -- the round-trip body was chosen so nothing in it is "+
				"rewritten; a rewrite here is a fact this probe does not record anywhere", r.Wire, r.Detail)
		}
	}
	sort.Strings(dropped)
	t.Logf("port forward round trip: %d asked, %d dropped", len(asked), len(dropped))

	// Why the classification above is the measurement and the 200 is not:
	// this collection accepts a key it does not recognise and stores the
	// document without it, so a create answering 200 says nothing on its own
	// about what reached the database.
	unknownKeyStripped(ctx, t, s, path, base("port-forward-unknown-key", "7002"))

	updateVerb, updatePath := measurePortForwardUpdate(ctx, t, s, path, id, stored)

	required := portForwardRequiredOnCreate(ctx, t, s, path, base, post, read)
	portForwardSourceLimiting(ctx, t, s, path, base, post, read)

	measured := portForwardClearing(ctx, t, s, path, id, stored, func() map[string]any { return read(id) })

	if _, delStatus, err := s.DeleteJSON(ctx, path+"/"+id); delStatus/100 != 2 {
		t.Errorf("delete %s/%s answered HTTP %d (%v)", path, id, delStatus, err)
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "api/s/{site}/rest/portforward",
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: required,
	}

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Writes == nil {
				a.Writes = map[string]behavior.WriteContract{}
			}
			if a.Discarded == nil {
				a.Discarded = map[string][]string{}
			}
			if a.Empty == nil {
				a.Empty = map[string]map[string]behavior.EmptySemantics{}
			}
			// Replace, not merge: the probe writes the same body every run,
			// so a field that stopped being dropped, or stopped being
			// swept, has to leave the artifact rather than linger as a fact
			// nothing re-measures. Nothing else writes these keys.
			a.Writes["PortForward"] = contract
			a.Discarded["PortForward"] = dropped
			a.Empty["portforward"] = measured
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok {
		t.Logf("no artifact at %s; run with BEHAVIOR_WRITE=1 to record the port forward", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	compareWriteContract(t, "PortForward", art.Writes, contract)
	if pinned, has := art.Discarded["PortForward"]; has && !reflect.DeepEqual(pinned, dropped) {
		t.Errorf("PortForward discard list drifted:\n  artifact: %v\n  measured: %v\n\n"+
			"re-measure with BEHAVIOR_WRITE=1 once the change is understood", pinned, dropped)
	}
	compareEmptySemantics(t, "portforward", art.Empty["portforward"], measured)
}

// portForwardSourceGroup seeds an address group for a rule to limit its
// source by, and returns its id. Source limiting by firewall group is the
// only way to get a value into src_firewall_group_id, and a rule naming a
// well-formed but nonexistent id measures the rejection rather than the
// field.
func portForwardSourceGroup(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	groups := "/api/s/" + site + "/rest/firewallgroup"
	body, status, err := s.PostJSON(ctx, groups, map[string]any{
		"name": "port-forward-probe-src", "group_type": "address-group",
		"group_members": []string{"192.0.2.0/24"},
	})
	id := objectID(firstData(t, body))
	if status/100 != 2 || id == "" {
		t.Fatalf("seeding a source address group failed (HTTP %d, %v %v); every rule below would "+
			"limit its source by an id the site does not have", status, body, err)
	}
	t.Cleanup(func() {
		s.DeleteJSON(context.WithoutCancel(ctx), groups+"/"+id) //nolint:errcheck
	})
	return id
}

// measurePortForwardUpdate confirms the generic v1 by-id verbs on a rule the
// caller already created: the read the generated client uses, and the write
// it uses. Neither had ever been exercised.
func measurePortForwardUpdate(
	ctx context.Context, t *testing.T, s *controllertest.Session, path, id string, stored map[string]any,
) (verb, rel string) {
	t.Helper()
	edited := clone(stored)
	edited["name"] = "port-forward-probe-updated"
	after, status, err := s.PutJSON(ctx, path+"/"+id, edited)
	if status/100 != 2 {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v); the generated update writes to that path",
			path, id, status, after, err)
		return "", ""
	}
	if got, _ := firstData(t, after)["name"].(string); got != "port-forward-probe-updated" {
		t.Errorf("PUT %s/%s answered 200 but the name reads back %q; the write did not take, so the "+
			"clearing verdicts below would be measuring a no-op", path, id, got)
	}
	t.Logf("PUT %s/{id} -> HTTP %d; that is the update", path, status)
	// Put the seeded name back so the clearing sweep starts from the
	// document the round trip classified.
	s.PutJSON(ctx, path+"/"+id, stored) //nolint:errcheck
	return "PUT", "api/s/{site}/rest/portforward/{id}"
}

// portForwardRequiredOnCreate removes each field of the known-good body in
// turn, and then sends nothing at all. The empty body is not redundant: the
// one-at-a-time sweep cannot see an at-least-one-of rule, because every
// other field is still there to satisfy it.
func portForwardRequiredOnCreate(
	ctx context.Context, t *testing.T, s *controllertest.Session, path string,
	base func(string, string) map[string]any,
	post func(map[string]any) (int, any, string),
	read func(string) map[string]any,
) []string {
	t.Helper()

	fields := make([]string, 0, len(base("", "")))
	for f := range base("", "") {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	var required []string
	for i, field := range fields {
		doc := base(fmt.Sprintf("port-forward-probe-%d", i), strconv.Itoa(7100+i))
		delete(doc, field)
		status, body, id := post(doc)
		if status/100 == 2 {
			if id != "" {
				s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
			}
			t.Logf("port forward create without %-24s accepted (HTTP %d) -- not required", field, status)
			continue
		}
		t.Logf("port forward create without %-24s rejected (HTTP %d, %s) -- required on create",
			field, status, v1ErrCode(body))
		required = append(required, field)
	}
	sort.Strings(required)

	// Both refusals the sweep finds are about a second field in the same
	// body, so each is re-measured with that field relaxed. A create that
	// then succeeds says the requirement was the condition's, not the
	// resource's, and the entry is dropped -- the same treatment content
	// filtering's network_ids gets, and for the same reason: a conditional
	// requirement filed flat is published as a rule callers outside the
	// condition do not have to obey.
	for i, cond := range []struct {
		field, when string
		relax       func(map[string]any)
	}{
		{
			field: "src_limiting_type",
			when:  "src_limiting_enabled is true",
			relax: func(d map[string]any) {
				delete(d, "src_limiting_enabled")
				delete(d, "src_limiting_type")
				delete(d, "src_firewall_group_id")
			},
		},
		{
			field: "src_firewall_group_id",
			when:  `src_limiting_type is "firewall_group"`,
			relax: func(d map[string]any) {
				d["src"] = "192.0.2.0/24"
				d["src_limiting_type"] = "ip"
				delete(d, "src_firewall_group_id")
			},
		},
	} {
		if !slices.Contains(required, cond.field) {
			continue
		}
		doc := base("port-forward-probe-cond-"+cond.field, strconv.Itoa(7300+i))
		cond.relax(doc)
		status, body, id := post(doc)
		if status/100 != 2 {
			t.Logf("%s stays required: a create relaxing %s is refused too (HTTP %d, %s)",
				cond.field, cond.when, status, v1ErrCode(body))
			continue
		}
		if id != "" {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		required = slices.DeleteFunc(required, func(f string) bool { return f == cond.field })
		t.Logf("LOUD: %s is refused as missing only while %s; a create outside that condition is "+
			"accepted (HTTP %d), so it is not recorded as required on create", cond.field, cond.when, status)
	}

	status, body, id := post(map[string]any{})
	if status/100 != 2 || id == "" {
		t.Logf("a port forward create carrying {} is refused (HTTP %d, %s); something in the body is "+
			"load-bearing after all, and the sweep above did not find it", status, v1ErrCode(body))
		return required
	}
	empty := read(id)
	s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	delete(empty, "_id")
	delete(empty, "site_id")
	if len(empty) != 0 {
		t.Errorf("a port forward create carrying {} stored %v besides its id; the controller is "+
			"filling in defaults this probe has not measured", empty)
	}
	t.Logf("LOUD: a port forward create carrying {} is accepted (HTTP %d) and stores a document with "+
		"an id and nothing else. Nothing is required on create.", status)
	return required
}

// portForwardSourceLimiting measures the two cross-field rules the source
// limiting fields obey, neither of which required_on_create nor the coercion
// map can express: a rule limiting by address must carry one, and an address
// in src overrides the limiting type the caller asked for.
func portForwardSourceLimiting(
	ctx context.Context, t *testing.T, s *controllertest.Session, path string,
	base func(string, string) map[string]any,
	post func(map[string]any) (int, any, string),
	read func(string) map[string]any,
) {
	t.Helper()

	byAddress := base("port-forward-probe-src-ip", "7201")
	byAddress["src_limiting_type"] = "ip"
	delete(byAddress, "src_firewall_group_id")
	status, body, id := post(byAddress)
	if status/100 == 2 {
		if id != "" {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		t.Errorf("a rule limiting its source by address while src is %q was accepted (HTTP %d); the "+
			"controller was measured refusing it, and the known-good body above relies on that",
			byAddress["src"], status)
	} else {
		t.Logf("source limiting by address with src=%q rejected (HTTP %d, %s) -- the address is not "+
			"optional once the type asks for one", byAddress["src"], status, v1ErrCode(body))
	}

	override := base("port-forward-probe-src-override", "7202")
	override["src"] = "192.0.2.0/24"
	status, body, id = post(override)
	if status/100 != 2 || id == "" {
		t.Fatalf("a rule carrying an address in src was rejected (HTTP %d, %s): %v", status, v1ErrCode(body), body)
	}
	got, _ := read(id)["src_limiting_type"].(string)
	s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	if got == override["src_limiting_type"] {
		t.Logf("LOUD: src_limiting_type now survives as %q alongside an address in src. The rewrite "+
			"this probe declines to record because it is conditional is gone; it can be recorded flat.", got)
		return
	}
	if got != "ip" {
		t.Errorf("asked src_limiting_type %q with an address in src and the controller stored %q; it "+
			"was measured storing \"ip\", and neither value is what was asked",
			override["src_limiting_type"], got)
		return
	}
	t.Logf("an address in src overrides src_limiting_type %q to %q -- a rewrite conditional on another "+
		"field, which is why it is asserted here and not recorded", override["src_limiting_type"], got)
}

// portForwardClearing measures, for every field the seeded rule stored a
// value in, what an emptied field and an absent key do on update. The empty
// form is the field's own: "" for a string, [] for a list.
//
// storedEmptySemantics does the reading, and it matters more here than
// anywhere else it is used: this collection answers a write that changed
// nothing with an empty data array, so a verdict taken from the write's own
// response scores every preserved field as cleared. That answer is measured
// below rather than carried over from the collections it was first seen on,
// so a controller that starts echoing the document is noticed.
func portForwardClearing(
	ctx context.Context, t *testing.T, s *controllertest.Session, path, id string,
	stored map[string]any, read func() map[string]any,
) map[string]behavior.EmptySemantics {
	t.Helper()

	// Only fields the seed populated: with nothing stored, clearing and
	// ignoring are the same observation.
	var fields []struct {
		wire  string
		blank any
	}
	for k, v := range stored {
		if k == "_id" || k == "site_id" || blankValue(v) {
			continue
		}
		switch v.(type) {
		case string:
			fields = append(fields, struct {
				wire  string
				blank any
			}{k, ""})
		case []any:
			fields = append(fields, struct {
				wire  string
				blank any
			}{k, []any{}})
		}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].wire < fields[j].wire })

	put := func(doc map[string]any) int {
		body, status, err := s.PutJSON(ctx, path+"/"+id, doc)
		if status == 0 {
			t.Fatalf("transport to %s/%s: %v", path, id, err)
		}
		if status/100 != 2 {
			// The rejection is the measurement; its body is the reason.
			t.Logf("PUT %s/%s -> HTTP %d: %s", path, id, status, v1ErrCode(body))
		}
		return status
	}

	if body, status, _ := s.PutJSON(ctx, path+"/"+id, clone(stored)); status/100 != 2 {
		t.Fatalf("re-writing the stored document was refused (HTTP %d); the sweep below has no way "+
			"to reset between arms", status)
	} else if firstData(t, body) == nil {
		t.Logf("a PUT that changes nothing answers HTTP 200 carrying no document, as the other v1 "+
			"collections do; every verdict below is read back from %s/%s", path, id)
	} else {
		t.Logf("a PUT that changes nothing now echoes the document; the verdicts below still read it "+
			"back from %s/%s", path, id)
	}

	measured := map[string]behavior.EmptySemantics{}
	for _, f := range fields {
		measured[f.wire] = storedEmptySemantics(t, f.wire, f.blank, stored, put, read)
	}
	var summary []string
	for _, f := range slices.Sorted(maps.Keys(measured)) {
		summary = append(summary, fmt.Sprintf("%-24s %-16s %s", f, measured[f].Empty, measured[f].Omit))
	}
	t.Logf("port forward clearing semantics (%d fields):\n  %s", len(summary), strings.Join(summary, "\n  "))

	// The merge, taken a second time from a write shape the sweep never
	// sends: a body naming one field and nothing else. If this collection
	// replaced, everything but the name would be gone.
	renamed := "port-forward-probe-merge"
	if put(map[string]any{"name": renamed}) == 200 {
		after := read()
		if got, _ := after["name"].(string); got != renamed {
			t.Errorf("a PUT naming only the name answered 200 and the name reads back %q; the write "+
				"did not take, so it proves nothing about the keys it left out", got)
		}
		for _, f := range fields {
			if f.wire == "name" {
				continue
			}
			if !jsonEqual(after[f.wire], stored[f.wire]) {
				t.Errorf("a PUT naming only the name left %s as %v, not the stored %v; this "+
					"collection replaces after all and the OMIT-KEEPS verdicts above are wrong",
					f.wire, after[f.wire], stored[f.wire])
			}
		}
		t.Logf("LOUD: a PUT naming only name kept all %d other stored fields, so this collection "+
			"merges like firewallgroup rather than replacing.", len(fields)-1)
	} else {
		t.Error("a PUT naming only the name was refused; the OMIT-KEEPS verdicts above then rest " +
			"on one write shape alone")
	}
	put(clone(stored)) // reset

	return measured
}
