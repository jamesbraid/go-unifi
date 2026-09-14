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
		if len(w.RequiredOnCreateWhen) == 0 {
			w.RequiredOnCreateWhen = nil
		} else {
			for k := range w.RequiredOnCreateWhen {
				sort.Strings(w.RequiredOnCreateWhen[k])
			}
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
//
// controllerDefault is what the controller stores for the field when nobody
// asks, or nil where that is not known. A write that lands on it is recorded
// as REPLACED-default rather than by value, because several such defaults are
// per-site object ids: recording the id would read as drift on the next run
// against a fresh container.
func storedEmptySemantics(
	t *testing.T,
	field string,
	blank any,
	controllerDefault any,
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
		case controllerDefault != nil && jsonEqual(got, controllerDefault):
			return prefix + "-REPLACED-default"
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
// absent, JSON null, the empty string, the empty list, false, or zero. The
// controller spells a cleared field every one of those ways depending on the
// collection -- firewallgroup drops the key, trafficroutes stores null for a
// string and [] for a list, static-dns binds its numbers and its boolean to a
// fresh object and stores 0 and false -- and they all mean the same thing to
// a caller.
//
// The cost of covering the last two is that a field already sitting at zero
// cannot be told from one that was cleared, which is why
// storedEmptySemantics refuses a seed that does not carry a value.
func blankValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case bool:
		return !t
	case float64:
		return t == 0
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
		measured[f.wire] = storedEmptySemantics(t, f.wire, f.blank, nil, stored, put, read)
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
// sweep that finds it matches on IP, which on its own says nothing about the
// other three selectors, so trafficRouteBranchRequiredFields re-runs the
// fields it records against all four. The destination filters are conditional
// and are asserted rather than recorded:
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
	trafficRouteBranchRequiredFields(ctx, t, s, path, wanID, lanID)
	trafficRouteEmptyTargetKeys(ctx, t, s, path, wanID, lanID)

	// The empty/omit half, for the two fields whose verdict does not depend on
	// which filter matching_target selects. The filter lists are conditional --
	// see trafficRouteMatchingBranches -- so recording an emptied one would
	// pin a verdict that only holds for the branch this seed happens to be in.
	measured := map[string]behavior.EmptySemantics{}
	for _, field := range []string{"description", "next_hop"} {
		measured[field] = storedEmptySemantics(t, field, "", nil, stored, put, func() map[string]any {
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

// trafficRouteBranchRequiredFields measures whether the flat
// required_on_create entry is really flat.
//
// The sweep above deletes one field at a time from a body that always matches
// on IP, so on its own it can only say a field is required for an IP route.
// A consumer reading the artifact compiles the entry into "this attribute is
// required" for every route it writes, including the DOMAIN, REGION and
// INTERNET ones the sweep never sent -- so a field required only on the IP
// branch would be recorded as required everywhere and break configurations the
// controller accepts today.
//
// Measured on 10.6.101: network_id and target_devices are refused by all four
// matching_target values, which is what makes the flat entry the right shape.
// The controller's own error carries the attribution independent of this
// function's sweep discipline: Spring bean validation names the offending
// field in camelCase, so a refusal reading "networkId NotEmpty" is the removal
// being answered rather than the branch's filter list.
func trafficRouteBranchRequiredFields(
	ctx context.Context,
	t *testing.T,
	s *controllertest.Session,
	path, wanID, lanID string,
) {
	t.Helper()

	// The smallest body each branch accepts: the common half, plus the filter
	// list matching_target names. INTERNET names none.
	base := func(target, description string) map[string]any {
		doc := map[string]any{
			"description":     description,
			"enabled":         true,
			"network_id":      wanID,
			"matching_target": target,
			"target_devices":  []any{map[string]any{"type": "NETWORK", "network_id": lanID}},
		}
		switch target {
		case "IP":
			doc["ip_addresses"] = []any{map[string]any{"ip_or_subnet": "192.0.2.0/24", "ip_version": "v4"}}
		case "DOMAIN":
			doc["domains"] = []any{map[string]any{"domain": "example.com"}}
		case "REGION":
			doc["regions"] = []any{"US"}
		}
		return doc
	}

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

	for _, target := range []string{"IP", "DOMAIN", "REGION", "INTERNET"} {
		// The branch's known-good body first. Nothing removed from a body
		// that does not create measures anything.
		if status, body := post(base(target, "traffic-route-required-"+target)); status/100 != 2 {
			t.Errorf("the smallest %s route was refused (HTTP %d, %s); the two removals below "+
				"would then measure the body rather than the field", target, status, v2Rejection(body))
			continue
		}

		for _, field := range []struct{ wire, named string }{
			{"network_id", "networkId NotEmpty"},
			{"target_devices", "targetDevices NotEmpty"},
		} {
			doc := base(target, "traffic-route-required-"+target+"-no-"+field.wire)
			delete(doc, field.wire)
			status, body := post(doc)
			if status/100 == 2 {
				t.Errorf("a route matching %s created without %s (HTTP %d); required_on_create "+
					"records it as required for every branch, so the flat entry now over-requires "+
					"and has to become conditional", target, field.wire, status)
				continue
			}
			if got := v2Rejection(body); got != field.named {
				t.Errorf("a route matching %s without %s was refused with %q, not %q; the field is "+
					"still required but something else is answering for it",
					target, field.wire, got, field.named)
				continue
			}
			t.Logf("matching %-8s without %-14s -> %s", target, field.wire, field.named)
		}
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
		measured[f.wire] = storedEmptySemantics(t, f.wire, f.blank, nil, stored, put, read)
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

// TestIntegrationDNSRecordWriteContract measures the static DNS collection,
// which no section of the artifact described: the SDK has shipped a client
// for it since the schema was captured and nothing had ever written one.
//
// It owns three artifact entries outright -- the write contract, the discard
// list and the clearing semantics -- because a second probe replacing any of
// them would erase what this one measured.
//
// The collection is v2, so an accepted body was taken as sent and the
// verdicts below can be read off the status where a v1 probe would have to
// re-read. The create path is measured rather than assumed: content
// filtering serves its create from a /create sub-path and answers 405 on the
// collection, so "POST the collection" is a convention, not a rule, and the
// probe checks both spellings. There is no by-id read at all -- GET on the
// {id} path is 405 -- which is why the generated Get filters the list.
//
// What needed care is the record type. The controller validates a record
// against the type it declares and refuses the numeric parameters that type
// has no use for, so a requirement measured on one type is not a requirement
// of the resource. The sweep therefore runs on four types -- A, SRV, MX and
// TXT, which admit four different parameter sets -- and the set is recorded
// only because all four agreed on it. Which parameters each type refuses is
// measured too, and logged rather than recorded: required_on_create can say
// a key must be present, never that a value must be absent.
//
// The refusal is on the value, not the key: zero is accepted in a slot a
// type refuses and 7 is not. That is what makes the recorded set safe to
// feed the generator, which drops omitempty from a required field: the
// parameters a type refuses stay optional, so their zero values keep
// vanishing from the request rather than turning every write into a
// rejection.
func TestIntegrationDNSRecordWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/v2/api/site/" + c.Site + "/static-dns"

	// The list has to be served before anything else here means what it
	// looks like, and it is also the only read this resource has.
	if body, status, err := s.GetJSON(ctx, path); status != 200 {
		t.Fatalf("GET %s answered HTTP %d (%v %v); the collection is not served, so no verb below "+
			"measures the write contract", path, status, body, err)
	}

	post := func(doc map[string]any) (int, any, string) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		// A missing record_type draws a non-JSON HTTP 500 from this
		// endpoint; the status is the measurement, so ErrNotJSON is not
		// fatal here.
		if err != nil {
			t.Logf("POST %s -> HTTP %d (%v)", path, status, err)
		}
		id := ""
		if status/100 == 2 {
			id = objectID(firstData(t, body))
		}
		return status, body, id
	}
	stored := func(id string) map[string]any {
		body, status, err := s.GetJSON(ctx, path)
		if status != 200 {
			t.Fatalf("GET %s answered HTTP %d (%v)", path, status, err)
		}
		for _, entry := range asSlice(body) {
			if m, _ := entry.(map[string]any); m != nil && objectID(m) == id {
				return m
			}
		}
		return nil
	}

	// One body per record type, each carrying every parameter that type
	// admits. SRV is the richest -- it is the only type that admits all four
	// of port, priority, weight and ttl -- so it is what the round trip and
	// the clearing sweep run against. Its key has to name a service and a
	// protocol: the controller refuses one whose first label does not start
	// with an underscore.
	branches := []dnsRecordBranch{
		{"A", func(suffix string) map[string]any {
			return map[string]any{
				"enabled": true, "record_type": "A", "ttl": 3600,
				"key": "a" + suffix + ".dns-probe.example.com", "value": "192.0.2.10",
			}
		}},
		{"SRV", func(suffix string) map[string]any {
			return map[string]any{
				"enabled": true, "record_type": "SRV", "ttl": 3600,
				"key": "_sip._tcp.s" + suffix + ".dns-probe.example.com", "value": "srv.example.com",
				"port": 5060, "priority": 10, "weight": 20,
			}
		}},
		{"MX", func(suffix string) map[string]any {
			return map[string]any{
				"enabled": true, "record_type": "MX",
				"key": "mx" + suffix + ".dns-probe.example.com", "value": "mail.example.com",
				"priority": 10,
			}
		}},
		{"TXT", func(suffix string) map[string]any {
			return map[string]any{
				"enabled": true, "record_type": "TXT",
				"key": "txt" + suffix + ".dns-probe.example.com", "value": "probe",
			}
		}},
	}
	srv := branches[1].body

	asked := srv("-roundtrip")
	status, body, id := post(asked)
	if status/100 != 2 || id == "" {
		t.Fatalf("the known-good SRV record was rejected (HTTP %d, %s): %v\n\nNothing removed from a "+
			"body that does not create can measure anything.", status, v2Rejection(body), body)
	}
	t.Logf("POST %s -> HTTP %d; that is the create", path, status)

	dropped := []string{}
	for _, r := range probe.Classify(asked, stored(id)) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("DROPPED %-16s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Errorf("CHANGED %-16s (%s) -- a rewrite here is a fact this probe records nowhere",
				r.Wire, r.Detail)
		}
	}
	sort.Strings(dropped)
	t.Logf("static DNS round trip: %d asked, %d dropped", len(asked), len(dropped))

	// Settles what a 2xx from the sweeps below means: on v2 the controller
	// binds the body strictly, so an accepted create took the body as sent.
	unknownKeyRejected(ctx, t, s, path, srv("-unknown-key"))

	// The trap content filtering set: a collection that serves GET alone and
	// creates from a sub-path answers 405 to a collection POST. The
	// collection POST above created, so the sub-path must not also exist --
	// two create routes would leave the generated client on an arbitrary one.
	for _, sub := range []string{"/create", "/add"} {
		body, subStatus, err := s.PostJSON(ctx, path+sub, srv("-subpath"))
		if subStatus/100 == 2 {
			if subID := objectID(firstData(t, body)); subID != "" {
				s.DeleteJSON(ctx, path+"/"+subID) //nolint:errcheck
			}
			t.Errorf("POST %s%s also creates (HTTP %d); the controller grew a second create route and "+
				"the generated client writes to the collection", path, sub, subStatus)
			continue
		}
		t.Logf("POST %s%s -> HTTP %d (%v) -- the collection is the only create", path, sub, subStatus, err)
	}

	// There is no by-id read; the generated Get filters the list because of
	// it. A controller that grows one would leave that detour unnoticed.
	if body, getStatus, _ := s.GetJSON(ctx, path+"/"+id); getStatus != 405 {
		t.Errorf("GET %s/{id} answered HTTP %d (%v); the controller grew a by-id read the list-backed "+
			"get routes around", path, getStatus, body)
	}

	updateVerb, updatePath := "", ""
	edited := clone(stored(id))
	edited["value"] = "srv2.example.com"
	after, putStatus, err := s.PutJSON(ctx, path+"/"+id, edited)
	if putStatus/100 != 2 {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v); the generated update writes to that path",
			path, id, putStatus, after, err)
	} else {
		updateVerb, updatePath = "PUT", "v2/api/site/{site}/static-dns/{id}"
		t.Logf("PUT %s/{id} -> HTTP %d; that is the update", path, putStatus)
	}
	s.PutJSON(ctx, path+"/"+id, clone(stored(id))) //nolint:errcheck

	required := dnsRecordRequiredOnCreate(ctx, t, s, path, branches, post)
	dnsRecordTypeParameters(ctx, t, s, path, branches, post)

	measured := dnsRecordClearing(ctx, t, s, path, id, stored(id), func() map[string]any { return stored(id) })

	if body, delStatus, err := s.DeleteJSON(ctx, path+"/"+id); delStatus/100 != 2 {
		t.Errorf("delete %s/%s answered HTTP %d (%v %v)", path, id, delStatus, body, err)
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "v2/api/site/{site}/static-dns",
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
			a.Writes["DNSRecord"] = contract
			a.Discarded["DNSRecord"] = dropped
			a.Empty["static-dns"] = measured
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok {
		t.Logf("no artifact at %s; run with BEHAVIOR_WRITE=1 to record the static DNS record", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	compareWriteContract(t, "DNSRecord", art.Writes, contract)
	if pinned, has := art.Discarded["DNSRecord"]; has && !reflect.DeepEqual(pinned, dropped) {
		t.Errorf("DNSRecord discard list drifted:\n  artifact: %v\n  measured: %v\n\n"+
			"re-measure with BEHAVIOR_WRITE=1 once the change is understood", pinned, dropped)
	}
	compareEmptySemantics(t, "static-dns", art.Empty["static-dns"], measured)
}

// dnsRecordRequiredOnCreate sweeps each record type's own known-good body and
// returns the requirement they all share.
//
// Running it once would measure one type's rules and publish them as the
// resource's. Running it on three types that admit three different parameter
// sets, and refusing to record anything the three disagree about, is what
// makes the recorded set a fact about static DNS rather than about A records.
func dnsRecordRequiredOnCreate(
	ctx context.Context, t *testing.T, s *controllertest.Session, path string,
	branches []dnsRecordBranch,
	post func(map[string]any) (int, any, string),
) []string {
	t.Helper()

	perType := map[string][]string{}
	for _, branch := range branches {
		fields := make([]string, 0, len(branch.body("")))
		for f := range branch.body("") {
			fields = append(fields, f)
		}
		sort.Strings(fields)

		var required []string
		for i, field := range fields {
			doc := branch.body(fmt.Sprintf("-req-%s-%d", strings.ToLower(branch.recordType), i))
			delete(doc, field)
			status, body, id := post(doc)
			if status/100 == 2 {
				if id != "" {
					s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
				}
				t.Logf("%-3s create without %-12s accepted (HTTP %d) -- not required",
					branch.recordType, field, status)
				continue
			}
			t.Logf("%-3s create without %-12s rejected (HTTP %d, %s) -- required on create",
				branch.recordType, field, status, v2Rejection(body))
			required = append(required, field)
			// A 500 is a crash, not a considered rejection, and a crash can
			// still have written. Nothing else here would notice.
			if status == 500 {
				if key, _ := doc["key"].(string); key != "" && dnsRecordByKey(ctx, t, s, path, key) != "" {
					t.Errorf("%s create without %s answered HTTP 500 and stored the record anyway; a "+
						"caller that retries the create would file a second copy",
						branch.recordType, field)
				} else {
					t.Logf("    the HTTP 500 stored nothing, so the create is a refusal and not a " +
						"half-completed write")
				}
			}
		}
		perType[branch.recordType] = required
	}

	first := branches[0].recordType
	shared := perType[first]
	for _, branch := range branches[1:] {
		if reflect.DeepEqual(perType[branch.recordType], shared) {
			continue
		}
		t.Errorf("the required-on-create set depends on the record type: %s needs %v and %s needs %v.\n\n"+
			"Only what every type needs can be recorded as the resource's requirement -- a set "+
			"measured on one type would be published as a rule the others do not obey. Record "+
			"nothing until the condition is understood.",
			first, shared, branch.recordType, perType[branch.recordType])
		return nil
	}
	t.Logf("required on create, agreed by %v: %v", dnsRecordTypeNames(branches), shared)
	return shared
}

// dnsRecordBranch is one record type and the fullest body that type admits.
type dnsRecordBranch struct {
	recordType string
	body       func(suffix string) map[string]any
}

// dnsRecordTypeNames lists the record types a sweep covered, for the log.
func dnsRecordTypeNames(branches []dnsRecordBranch) []string {
	out := make([]string, 0, len(branches))
	for _, b := range branches {
		out = append(out, b.recordType)
	}
	return out
}

// dnsRecordByKey returns the id of the record with the given key, or "".
func dnsRecordByKey(ctx context.Context, t *testing.T, s *controllertest.Session, path, key string) string {
	t.Helper()
	body, status, err := s.GetJSON(ctx, path)
	if status != 200 {
		t.Fatalf("GET %s answered HTTP %d (%v)", path, status, err)
	}
	for _, entry := range asSlice(body) {
		if m, _ := entry.(map[string]any); m != nil {
			if k, _ := m["key"].(string); k == key {
				return objectID(m)
			}
		}
	}
	return ""
}

// dnsRecordTypeParameters measures which of the four numeric parameters each
// record type refuses, by adding to that type's known-good body every one it
// does not already carry.
//
// The refused sets are what scopes the recorded requirement: they are the
// reason the required-on-create sweep runs on four types rather than one, and
// they are logged rather than recorded because required_on_create can only
// say a key must be present, never that a value must be absent.
//
// The invariant the recorded contract rests on is measured alongside them: a
// parameter refused with a value is accepted with zero. The generator drops
// omitempty from a field the artifact records as required, so a zero reaches
// the wire; if zero were refused in these slots, a caller writing an A record
// through the generated client would be rejected by the controller.
func dnsRecordTypeParameters(
	ctx context.Context, t *testing.T, s *controllertest.Session, path string,
	branches []dnsRecordBranch,
	post func(map[string]any) (int, any, string),
) {
	t.Helper()

	create := func(doc map[string]any) (int, any) {
		status, body, id := post(doc)
		if id != "" {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, body
	}

	refusals := 0
	for _, branch := range branches {
		var refused []string
		for i, param := range []string{"port", "priority", "ttl", "weight"} {
			if _, carried := branch.body("")[param]; carried {
				continue
			}
			suffix := fmt.Sprintf("-param-%s-%d", strings.ToLower(branch.recordType), i)
			valued := branch.body(suffix)
			valued[param] = 7
			status, body := create(valued)
			if status/100 == 2 {
				continue
			}
			refused = append(refused, param)
			refusals++
			t.Logf("%-3s carrying %-8s = 7 rejected (HTTP %d, %s)",
				branch.recordType, param, status, v2Rejection(body))

			zeroed := branch.body(suffix + "-zero")
			zeroed[param] = 0
			if status, body := create(zeroed); status/100 != 2 {
				t.Errorf("a %s record carrying %s=0 was rejected (HTTP %d, %s); the refusal is on the "+
					"key rather than the value, so a required field losing omitempty would send a zero "+
					"into a slot the type refuses", branch.recordType, param, status, v2Rejection(body))
			}
		}
		t.Logf("%-3s refuses a value in: %v", branch.recordType, refused)
	}

	if refusals == 0 {
		t.Errorf("no record type refused any of port, priority, ttl or weight; the per-type parameter "+
			"rule the recorded contract is scoped by no longer holds, and the sweep across %v is "+
			"measuring the same thing four times", dnsRecordTypeNames(branches))
	}
}

// dnsRecordClearing measures, for every field the seeded record stored a
// value in, what an emptied field and an absent key do on update.
//
// Every field is swept, not the strings alone, because this is where the
// full-replace question is answered and it is answered per field: the
// controller binds the body to a fresh object, so a key the payload leaves
// out arrives at the Java field's default rather than at what was stored. The
// blank written is "" for all of them, numeric fields included -- that is
// what the clearing vocabulary means by empty, and the controller coerces it
// during binding rather than refusing it.
func dnsRecordClearing(
	ctx context.Context, t *testing.T, s *controllertest.Session, path, id string,
	stored map[string]any, read func() map[string]any,
) map[string]behavior.EmptySemantics {
	t.Helper()

	var fields []string
	for k, v := range stored {
		if k == "_id" || k == "site_id" || blankValue(v) {
			continue
		}
		fields = append(fields, k)
	}
	sort.Strings(fields)

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

	measured := map[string]behavior.EmptySemantics{}
	for _, field := range fields {
		measured[field] = storedEmptySemantics(t, field, "", nil, stored, put, read)
	}
	var summary []string
	for _, f := range slices.Sorted(maps.Keys(measured)) {
		summary = append(summary, fmt.Sprintf("%-12s %-16s %s", f, measured[f].Empty, measured[f].Omit))
	}
	t.Logf("static DNS clearing semantics (%d fields):\n  %s", len(summary), strings.Join(summary, "\n  "))

	return measured
}

// createSweep measures a v1 rest collection's create contract by reduction:
// it starts from a body the controller accepts, removes every field the
// controller does not need, and names what is left.
//
// Reduction rather than the one-at-a-time sweep the other probes here use,
// because a network's rich body is full of pairs. dhcpd_start is refused
// only while dhcpd_enabled is set; dhcpd_ip_1 only while dhcpguard_enabled
// is. A sweep that removes one key from a body still carrying its partner
// records the partner as required on create, which it is not -- it is
// required by a flag the caller chose to set, and those cross-field rules
// are measured on their own by TestIntegrationNetworkCrossField. Here every
// removable field is gone before anything is named, so a field that survives
// is one no create of that shape may omit whatever else it carries.
//
// The passes run to a fixpoint rather than once, because removability itself
// depends on what is still in the body: the first pass cannot remove
// dhcpd_start while dhcpd_enabled is present, and the second can once it is
// not. A pass that removes nothing is the answer.
type createSweep struct {
	s      *controllertest.Session
	path   string // the collection, e.g. /api/s/default/rest/networkconf
	prefix string // name prefix; every attempt gets its own
	n      int
}

// maxCreateSweepPasses bounds the reduction. A body still shrinking after
// this many passes means the controller is answering the same request
// differently run to run, and every verdict below it would be noise.
const maxCreateSweepPasses = 8

// post creates one document, re-reads it, and deletes it again. The re-read
// is the point: this is a v1 collection, which drops a key it does not
// recognise and carries on, so a 200 says the controller stored something
// and not that it stored what was asked. The delete is confirmed rather than
// fired and forgotten -- a document left behind makes the next attempt
// collide on a unique value, and a collision rejection read as a field
// verdict is exactly the kind of lie this probe exists to avoid.
func (c *createSweep) post(ctx context.Context, t *testing.T, doc map[string]any) (int, string, map[string]any) {
	t.Helper()
	body, status, err := c.s.PostJSON(ctx, c.path, doc)
	if status == 0 {
		t.Fatalf("transport to %s: %v", c.path, err)
	}
	if status/100 != 2 {
		return status, v1ErrCode(body), nil
	}
	id := objectID(firstData(t, body))
	if id == "" {
		t.Fatalf("POST %s answered HTTP %d with no id, so nothing can be re-read or cleaned up: %v",
			c.path, status, body)
	}
	stored := v1Read(ctx, t, c.s, c.path, id)
	if len(stored) == 0 {
		t.Fatalf("GET %s/%s after a %d create came back empty; the stored document is what classifies "+
			"the write", c.path, id, status)
	}
	c.remove(ctx, t, id)
	return status, "", stored
}

// v1Read fetches one document by id, or nil when the collection holds none.
// A v1 rest read of an id that is not there answers HTTP 200 with an empty
// data array rather than a 404, so presence is the envelope's doing and not
// the status line's.
func v1Read(ctx context.Context, t *testing.T, s *controllertest.Session, path, id string) map[string]any {
	t.Helper()
	body, status, err := s.GetJSON(ctx, path+"/"+id)
	if status != 200 {
		t.Fatalf("GET %s/%s answered HTTP %d (%v %v)", path, id, status, body, err)
	}
	return firstData(t, body)
}

// remove deletes one document and waits for the collection to stop serving
// it.
func (c *createSweep) remove(ctx context.Context, t *testing.T, id string) {
	t.Helper()
	if _, status, err := c.s.DeleteJSON(ctx, c.path+"/"+id); status/100 != 2 {
		t.Fatalf("DELETE %s/%s answered HTTP %d (%v); the objects this sweep creates have to go away "+
			"between attempts or the next one collides", c.path, id, status, err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if len(v1Read(ctx, t, c.s, c.path, id)) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s/%s is still served after a successful DELETE; every later attempt would be "+
				"measured against a site that still holds it", c.path, id)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// sweepVerdict is one attempted create. The two failing halves are kept
// apart on purpose: only a refusal is a requirement, and a create the
// controller took but stored off-branch is a different fact that
// required_on_create has no vocabulary for.
type sweepVerdict struct {
	accepted bool   // 2xx, and the stored document is in the branch
	refused  bool   // the controller answered other than 2xx
	why      string // what it said, or what it stored instead
}

// attempt sends one candidate body and classifies the answer. The branch
// check on the stored document is what stops a 200 being read as "the field
// was optional": a create that omits purpose is accepted and stores a
// network with no purpose at all, which is not the branch anything here is
// measuring.
func (c *createSweep) attempt(ctx context.Context, t *testing.T, doc map[string]any, selector map[string]string) sweepVerdict {
	t.Helper()
	c.n++
	body := clone(doc)
	// Only when the body has one: name is a field under test, and stamping
	// it on every attempt would make it un-removable and so read as
	// required.
	if _, named := body["name"]; named {
		body["name"] = fmt.Sprintf("%s-%d", c.prefix, c.n)
	}
	status, reason, stored := c.post(ctx, t, body)
	if status/100 != 2 {
		return sweepVerdict{refused: true, why: fmt.Sprintf("HTTP %d %s", status, reason)}
	}
	for _, wire := range sortedWireNames(selector) {
		if got, _ := stored[wire].(string); got != selector[wire] {
			return sweepVerdict{why: fmt.Sprintf("accepted (HTTP %d) and stored %s=%v", status, wire, stored[wire])}
		}
	}
	return sweepVerdict{accepted: true}
}

// reduce runs the sweep and returns the smallest body the controller
// accepted along with the fields it REFUSED to create without. Fields named
// in selector are held out of the reduction -- removing one leaves the
// branch under measurement -- and tested last.
//
// Only a refusal makes the recorded list. A removal the controller accepts
// but stores off-branch is loud in the log and absent from the artifact:
// required_on_create means "the create was rejected", and stretching it to
// cover "accepted, but you got something else" would publish two different
// measurements as one.
func (c *createSweep) reduce(ctx context.Context, t *testing.T, seed map[string]any, selector map[string]string) (map[string]any, []string) {
	t.Helper()
	branch := createBranchKey(selector)
	if v := c.attempt(ctx, t, seed, selector); !v.accepted {
		t.Fatalf("the seed for %s was not accepted (%s).\n\nNothing removed from a body that does "+
			"not create can measure anything.", branch, v.why)
	}

	body := clone(seed)
	var blocked map[string]sweepVerdict
	for pass := 1; ; pass++ {
		removed := 0
		blocked = map[string]sweepVerdict{}
		for _, wire := range sortedWireNames(body) {
			if _, held := selector[wire]; held {
				continue
			}
			if _, still := body[wire]; !still {
				continue // removed earlier in this pass
			}
			trial := clone(body)
			delete(trial, wire)
			v := c.attempt(ctx, t, trial, selector)
			if v.accepted {
				body = trial
				removed++
				continue
			}
			blocked[wire] = v
		}
		if removed == 0 {
			break
		}
		if pass == maxCreateSweepPasses {
			t.Fatalf("the %s create body was still shrinking after %d passes; the controller is not "+
				"answering the same request the same way and no verdict here would mean anything",
				branch, pass)
		}
	}

	for _, wire := range sortedWireNames(selector) {
		trial := clone(body)
		delete(trial, wire)
		if v := c.attempt(ctx, t, trial, selector); !v.accepted {
			blocked[wire] = v
		}
	}

	t.Logf("%s minimal accepted body: %v", branch, sortedWireNames(body))
	required := []string{}
	for _, wire := range sortedWireNames(blocked) {
		v := blocked[wire]
		if v.refused {
			required = append(required, wire)
			t.Logf("%s requires %-32s removing it: %s", branch, wire, v.why)
			continue
		}
		t.Logf("LOUD: %s is NOT refused without %-24s it %s. Recorded nowhere: the create succeeded, "+
			"and what came back is a different shape rather than an error the caller can act on.",
			branch, wire, v.why)
	}
	return body, required
}

// createBranch is one shape of a resource's create: the field values that
// select it, and a body the controller accepts in it.
type createBranch struct {
	selector map[string]string
	seed     map[string]any
}

// createBranchKey renders a selector as the artifact's condition key --
// field=value pairs, sorted by field name, comma-joined.
func createBranchKey(selector map[string]string) string {
	parts := make([]string, 0, len(selector))
	for _, wire := range sortedWireNames(selector) {
		parts = append(parts, wire+"="+selector[wire])
	}
	return strings.Join(parts, ",")
}

// requiredInEveryBranch is the intersection: the fields no create of any
// measured branch may omit, which is the only thing the artifact may publish
// as an unconditional rule.
func requiredInEveryBranch(byBranch map[string][]string) []string {
	if len(byBranch) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, fields := range byBranch {
		for _, f := range fields {
			counts[f]++
		}
	}
	out := []string{}
	for f, n := range counts {
		if n == len(byBranch) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// sortedWireNames is the deterministic iteration order every sweep here
// needs: a reduction whose order varies finds a different minimal body run
// to run, and the artifact would never settle.
func sortedWireNames[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// v1UpdateContract measures the update half: which verb and path the
// collection takes an edit on, and which of the stored document's own keys a
// PUT may not omit. It seeds one object, re-reads it, and puts it back a key
// at a time.
//
// The collection PUT is measured too, and it is what says the update path is
// the {id} one rather than the collection: an update body carries its own
// _id, so it could plausibly go either way, and only the controller settles
// which.
func v1UpdateContract(ctx context.Context, t *testing.T, s *controllertest.Session, path, relPath string, seed map[string]any) (verb, updatePath string, required []string) {
	t.Helper()

	body, status, err := s.PostJSON(ctx, path, seed)
	if status/100 != 2 {
		t.Fatalf("seeding an object to update failed (HTTP %d): %v %v", status, body, err)
	}
	id := objectID(firstData(t, body))
	if id == "" {
		t.Fatalf("the seeded object carries no id, so the update path cannot be measured: %v", body)
	}
	defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck

	stored := v1Read(ctx, t, s, path, id)
	if len(stored) == 0 {
		t.Fatalf("GET %s/%s came back empty right after the create; the generated read uses that path", path, id)
	}

	if after, putStatus, err := s.PutJSON(ctx, path+"/"+id, stored); putStatus/100 == 2 {
		verb, updatePath = "PUT", relPath
		t.Logf("PUT %s/{id} -> HTTP %d; that is the update", path, putStatus)
	} else {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v); the generated update writes to that path",
			path, id, putStatus, after, err)
	}

	// The same document, same verb, one path segment shorter. A 2xx here
	// would mean the {id} path above is one update route of two.
	if collBody, collStatus, _ := s.PutJSON(ctx, path, stored); collStatus/100 == 2 {
		t.Errorf("PUT %s (the collection) also accepted an update (HTTP %d); the update path is then "+
			"not the {id} one alone and the recorded contract is too narrow", path, collStatus)
	} else {
		t.Logf("PUT %s (the collection) is refused (HTTP %d, %s) -- the update needs the id in the path",
			path, collStatus, v1ErrCode(collBody))
	}

	required = []string{}
	for _, wire := range sortedWireNames(stored) {
		trial := clone(stored)
		delete(trial, wire)
		after, putStatus, _ := s.PutJSON(ctx, path+"/"+id, trial)
		if putStatus/100 == 2 {
			continue
		}
		t.Logf("update without %-32s rejected (HTTP %d, %s) -- required on update",
			wire, putStatus, v1ErrCode(after))
		required = append(required, wire)
	}
	sort.Strings(required)
	return verb, updatePath, required
}

// listMinItems measures the smallest length a present list field may carry:
// it writes the field as an empty array into a body the controller otherwise
// accepts. 0 means an empty list is stored, 1 means it is refused. Distinct
// from required-on-create, which says whether the key may be absent at all --
// remote_vpn_subnets is measured absent-is-refused and empty-is-stored, and
// only the two together describe it.
func listMinItems(ctx context.Context, t *testing.T, sweep *createSweep, seed map[string]any, selector map[string]string, wire string) int {
	t.Helper()
	doc := clone(seed)
	doc[wire] = []string{}
	if v := sweep.attempt(ctx, t, doc, selector); !v.accepted {
		t.Logf("%s written as an empty list is refused (%s) -- a present list needs a member", wire, v.why)
		return 1
	}
	t.Logf("%s written as an empty list is stored -- the key is required, its contents are not", wire)
	return 0
}

// TestIntegrationNetworkWriteContract measures the write contract of the
// biggest resource the SDK ships. Network already has entries in the
// artifact's discarded, ownership, rejected_creates, replays and uos_pins
// sections; the plainest thing about it had never been recorded -- which
// verb and path create one, and which fields a create cannot omit.
//
// It owns the artifact's writes["Network"] entry outright.
//
// The required set is not one set. A network's create rules follow its
// purpose, and for the three VPN purposes they follow vpn_type inside that,
// so the answer is recorded per branch under required_on_create_when. A flat
// list would say a WAN create has to carry an IPsec pre-shared key. Only the
// intersection -- what every branch requires -- may be published flat, and
// here that intersection is empty: no field is required on every network
// create.
//
// Two verdicts are worth reading before the numbers:
//
//   - purpose is NOT required. A create that names none is accepted and
//     stores a network with no purpose at all -- an object the SDK's own
//     Network.MarshalJSON cannot encode, since it dispatches on Purpose and
//     errors on a value it does not know. That is asserted here rather than
//     recorded, because required_on_create has no way to say "accepted, but
//     not the thing you asked for".
//   - name is NOT required either, on any purpose. A nameless network
//     creates and is stored nameless.
//
// Three of the thirteen vpn_type values the controller declares have an
// accepted body here; the other ten are swept by networkVPNBranchesUnmeasured
// and left out of the artifact, loudly, rather than guessed at.
//
// The corporate, guest and vlan-only branches require vlan and vlan_enabled,
// and the reason is measured rather than assumed: the refusal is
// api.err.VlanUsed, the same code a second network on an already-used VLAN
// draws, because a create naming no VLAN lands untagged and the site's
// default LAN is already there. networkUntaggedSlotIsTaken measures that
// whole chain, including that the default LAN can be neither moved onto a
// VLAN nor deleted -- which is what makes the requirement hold on every site
// rather than only on this one.
func TestIntegrationNetworkWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 60*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	const (
		createRel = "api/s/{site}/rest/networkconf"
		updateRel = "api/s/{site}/rest/networkconf/{id}"
	)
	path := "/api/s/" + c.Site + "/rest/networkconf"

	// The list has to be served before anything else here means what it
	// looks like: a 404 collection would make every refusal below "the route
	// is absent" wearing another status code's clothes.
	if body, status, err := s.GetJSON(ctx, path); status != 200 {
		t.Fatalf("GET %s answered HTTP %d (%v %v); the collection is not served, so nothing below "+
			"measures the write contract", path, status, body, err)
	}

	// Why every sweep below re-reads instead of trusting its 200.
	unknownKeyStripped(ctx, t, s, path, map[string]any{
		"name": "write-contract-unknown-key", "purpose": PurposeCorporate,
		"vlan_enabled": true, "vlan": 3901,
	})

	// remote-user-vpn authenticates against the built-in RADIUS server,
	// which has to be running before the controller will accept the network.
	setSiteRadiusEnabled(ctx, t, s, c.Site, true)
	radiusProfile := builtinRadiusProfileID(ctx, t, s, c.Site)

	sweep := &createSweep{s: s, path: path, prefix: "write-contract"}
	byBranch := map[string][]string{}
	var siteVPN createBranch
	for _, branch := range networkCreateBranches(radiusProfile) {
		key := createBranchKey(branch.selector)
		if branch.selector["purpose"] == PurposeSiteVPN {
			siteVPN = branch
		}
		_, required := sweep.reduce(ctx, t, branch.seed, branch.selector)
		byBranch[key] = required
	}

	// A list field named in a required set says the key must be there; it
	// does not say the list must hold anything. The site-vpn branch is the
	// only measured branch that requires one.
	minItems := map[string]int{}
	if n := listMinItems(ctx, t, sweep, siteVPN.seed, siteVPN.selector, "remote_vpn_subnets"); n > 0 {
		minItems["remote_vpn_subnets"] = n
	}

	networkPurposeless(ctx, t, s, path)
	networkUntaggedSlotIsTaken(ctx, t, s, c.Site)
	networkVPNBranchesUnmeasured(ctx, t, s, c.Site, radiusProfile)

	updateVerb, updatePath, requiredOnUpdate := v1UpdateContract(ctx, t, s, path, updateRel, map[string]any{
		"name": "write-contract-update", "purpose": PurposeCorporate,
		"vlan_enabled": true, "vlan": 3902,
	})

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: createRel,
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate:     requiredInEveryBranch(byBranch),
		RequiredOnCreateWhen: byBranch,
		RequiredOnUpdate:     requiredOnUpdate,
		MinItems:             minItems,
	}

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Writes == nil {
				a.Writes = map[string]behavior.WriteContract{}
			}
			a.Writes["Network"] = contract
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok || art.Writes == nil {
		t.Logf("no pinned write contracts in %s; run with BEHAVIOR_WRITE=1 to record the network one",
			behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	compareWriteContract(t, "Network", art.Writes, contract)
}

// networkCreateBranches is one accepted body per shape of network create.
//
// Each seed only has to create and to carry every field the branch is asked
// about: a field the seed does not name is proven unnecessary by the seed's
// own success, and one it does name is tested by removal. Which is why they
// are lean rather than the full per-purpose payloads
// TestIntegrationNetworkRoundTrip seeds -- the extra hundred fields there
// buy nothing here and cost an attempt each.
func networkCreateBranches(radiusProfile string) []createBranch {
	return []createBranch{
		{
			selector: map[string]string{"purpose": PurposeCorporate},
			seed: map[string]any{
				"name": "x", "purpose": PurposeCorporate, "enabled": true,
				"ip_subnet": "10.94.10.1/24", "vlan_enabled": true, "vlan": 810,
				"networkgroup": "LAN", "setting_preference": "manual",
				"igmp_snooping": true, "dhcpguard_enabled": true, "dhcpd_ip_1": "10.94.10.2",
				"dhcpd_enabled": true, "dhcpd_start": "10.94.10.6", "dhcpd_stop": "10.94.10.254",
			},
		},
		{
			selector: map[string]string{"purpose": PurposeGuest},
			seed: map[string]any{
				"name": "x", "purpose": PurposeGuest, "enabled": true,
				"ip_subnet": "10.94.20.1/24", "vlan_enabled": true, "vlan": 820,
				"networkgroup": "LAN", "setting_preference": "manual",
				"igmp_snooping": true, "dhcpguard_enabled": true, "dhcpd_ip_1": "10.94.20.2",
				"dhcpd_enabled": true, "dhcpd_start": "10.94.20.6", "dhcpd_stop": "10.94.20.254",
			},
		},
		{
			selector: map[string]string{"purpose": PurposeVLANOnly},
			seed: map[string]any{
				"name": "x", "purpose": PurposeVLANOnly, "enabled": false,
				"networkgroup": "LAN", "vlan_enabled": true, "vlan": 830,
				"igmp_snooping": true, "dhcpguard_enabled": true, "dhcpd_ip_1": "10.94.30.2",
			},
		},
		{
			selector: map[string]string{"purpose": PurposeWAN},
			seed: map[string]any{
				"name": "x", "purpose": PurposeWAN, "enabled": true,
				"wan_networkgroup": "WAN2", "wan_type": "dhcp", "wan_type_v6": "disabled",
				"wan_dns_preference": "manual", "wan_dns1": "10.94.40.53",
				"wan_load_balance_type": "failover-only", "wan_failover_priority": 2,
			},
		},
		// The three VPN purposes pin vpn_type as well. Their requirements
		// belong to the tunnel, not to the purpose: an IPsec site-to-site
		// network needs a peer and a pre-shared key, and recording that
		// under purpose=site-vpn alone would hand the same rules to the
		// other tunnel types the enum admits.
		{
			selector: map[string]string{"purpose": PurposeSiteVPN, "vpn_type": "ipsec-vpn"},
			seed: map[string]any{
				"name": "x", "purpose": PurposeSiteVPN, "enabled": true,
				"vpn_type": "ipsec-vpn", "ipsec_interface": "wan",
				"ipsec_peer_ip": "203.0.113.9", "ipsec_key_exchange": "ikev2",
				"x_ipsec_pre_shared_key": "s3cret-psk",
				"remote_vpn_subnets":     []string{"192.0.2.0/24"},
			},
		},
		{
			selector: map[string]string{"purpose": PurposeVPNClient, "vpn_type": "wireguard-client"},
			seed: map[string]any{
				"name": "x", "purpose": PurposeVPNClient, "enabled": true,
				"vpn_type": "wireguard-client", "wireguard_client_mode": "manual",
				"ip_subnet":                "10.198.0.1/24",
				"wireguard_client_peer_ip": "203.0.113.20", "wireguard_client_peer_port": 51820,
				"wireguard_client_peer_public_key": "yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=",
				"x_wireguard_private_key":          "6KpcbNfK7kFzOlKjnDbSaYbmDbAZBOKwFqjOWMkSCFU=",
				"vpn_client_pull_dns":              false,
				"dhcpd_dns_enabled":                true, "dhcpd_dns_1": "1.1.1.1",
			},
		},
		{
			selector: map[string]string{"purpose": PurposeUserVPN, "vpn_type": "openvpn-server"},
			seed: map[string]any{
				"name": "x", "purpose": PurposeUserVPN, "enabled": true,
				"vpn_type": "openvpn-server", "openvpn_mode": "server",
				"openvpn_encryption_cipher": "AES_256_CBC",
				"ip_subnet":                 "10.199.0.1/24", "local_port": 1195,
				"radiusprofile_id":  radiusProfile,
				"dhcpd_dns_enabled": true, "dhcpd_dns_1": "1.1.1.1",
			},
		},
	}
}

// networkPurposeless measures what a create that names no purpose does. It
// is accepted, and the network it stores has no purpose at all -- not a
// default, not corporate, absent.
//
// This is asserted rather than recorded because required_on_create can only
// say "the create was refused", and this create was not. It matters anyway:
// the SDK's Network.MarshalJSON dispatches on Purpose and refuses a value it
// does not know, so an object created this way -- or by any other client --
// can be read by the SDK and never written back.
func networkPurposeless(ctx context.Context, t *testing.T, s *controllertest.Session, path string) {
	t.Helper()
	body, status, err := s.PostJSON(ctx, path, map[string]any{
		"name": "write-contract-no-purpose", "vlan_enabled": true, "vlan": 3903,
	})
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	id := objectID(firstData(t, body))
	if id != "" {
		defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	}
	if status/100 != 2 {
		t.Errorf("a create naming no purpose was refused (HTTP %d, %s); it was measured being "+
			"accepted, and the branch sweeps read their own purpose-removal results against that",
			status, v1ErrCode(body))
		return
	}
	stored := v1Read(ctx, t, s, path, id)
	if purpose, present := stored["purpose"]; present {
		t.Errorf("a create naming no purpose stored purpose=%v; it was measured storing none at all, "+
			"and a controller that now fills one in changes what every purpose branch means", purpose)
		return
	}
	t.Logf("LOUD: a create naming no purpose is accepted (HTTP %d) and stores a network with no "+
		"purpose key: %v. Network.MarshalJSON cannot encode that object, so the SDK can read it and "+
		"never write it back.", status, sortedWireNames(stored))
}

// networkUntaggedSlotIsTaken measures why a corporate, guest or vlan-only
// create that names no VLAN is refused, so the vlan and vlan_enabled entries
// in those branches' required sets are read as what they are.
//
// Three legs, in order:
//
//   - the matrix: of the four ways to write the two VLAN keys, only the pair
//     -- the flag true and an id -- creates. The other three draw
//     api.err.VlanUsed, and so does naming neither.
//   - the control: a second network on an id the first already holds draws
//     the SAME code. So the refusals above are a collision, not a missing
//     field: a create with no usable VLAN lands untagged and something is
//     already there.
//   - the occupant: the site's default LAN is untagged, refuses to be moved
//     onto a VLAN, and refuses to be deleted. Which is what turns "this
//     site's untagged slot is taken" into a rule that holds on every site.
func networkUntaggedSlotIsTaken(ctx context.Context, t *testing.T, s *controllertest.Session, site string) {
	t.Helper()
	path := "/api/s/" + site + "/rest/networkconf"
	sweep := &createSweep{s: s, path: path, prefix: "vlan-matrix"}
	corporate := map[string]string{"purpose": PurposeCorporate}

	for _, tc := range []struct {
		what   string
		keys   map[string]any
		accept bool
	}{
		{"neither key", map[string]any{}, false},
		{"vlan_enabled alone", map[string]any{"vlan_enabled": true}, false},
		{"an id alone", map[string]any{"vlan": 3904}, false},
		{"an id with the flag false", map[string]any{"vlan_enabled": false, "vlan": 3905}, false},
		{"the flag true and an id", map[string]any{"vlan_enabled": true, "vlan": 3906}, true},
	} {
		doc := map[string]any{"name": "vlan-matrix", "purpose": PurposeCorporate}
		for wire, v := range tc.keys {
			doc[wire] = v
		}
		v := sweep.attempt(ctx, t, doc, corporate)
		switch {
		case v.accepted && !tc.accept:
			t.Errorf("a corporate create carrying %s was accepted; the vlan entries in the recorded "+
				"required set rest on it being refused", tc.what)
		case !v.accepted && tc.accept:
			t.Fatalf("a corporate create carrying %s was refused (%s); that is the one combination "+
				"the whole matrix reads the others against", tc.what, v.why)
		case !v.accepted && !strings.Contains(v.why, "api.err.VlanUsed"):
			t.Errorf("a corporate create carrying %s was refused with %s, not api.err.VlanUsed; the "+
				"refusal is about something other than the VLAN and the reading below does not hold",
				tc.what, v.why)
		default:
			t.Logf("corporate create carrying %-26s accepted=%v %s", tc.what, v.accepted, v.why)
		}
	}

	// The control. Without it, api.err.VlanUsed above is just a code that
	// happens to mention VLANs.
	held := map[string]any{
		"name": "vlan-collision-a", "purpose": PurposeCorporate,
		"vlan_enabled": true, "vlan": 3907,
	}
	body, status, err := s.PostJSON(ctx, path, held)
	if status/100 != 2 {
		t.Fatalf("seeding a network on VLAN 3907 failed (HTTP %d): %v %v", status, body, err)
	}
	if id := objectID(firstData(t, body)); id != "" {
		defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	}
	second := clone(held)
	second["name"] = "vlan-collision-b"
	dup, dupStatus, _ := s.PostJSON(ctx, path, second)
	if id := objectID(firstData(t, dup)); id != "" && dupStatus/100 == 2 {
		s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	}
	if code := v1ErrCode(dup); dupStatus/100 == 2 || code != "api.err.VlanUsed" {
		t.Errorf("a second network on VLAN 3907 answered HTTP %d %q; api.err.VlanUsed was measured "+
			"as the collision code, and the matrix above is read as collisions because of it",
			dupStatus, code)
	} else {
		t.Logf("a second network on a held VLAN draws %s -- the matrix above is collisions, not "+
			"missing fields", code)
	}

	// The occupant.
	lanID := defaultLANNetworkID(ctx, t, s, site)
	if lanID == "" {
		t.Fatal("the site has no corporate network, so nothing here says what holds the untagged slot")
	}
	lan := v1Read(ctx, t, s, path, lanID)
	if lan["vlan_enabled"] == true {
		t.Fatalf("the site's default network is on VLAN %v, so it does not hold the untagged slot and "+
			"the refusals above have some other cause", lan["vlan"])
	}
	moved := clone(lan)
	moved["vlan_enabled"] = true
	moved["vlan"] = 3908
	if after, moveStatus, _ := s.PutJSON(ctx, path+"/"+lanID, moved); moveStatus/100 == 2 {
		t.Errorf("the default network moved onto a VLAN (HTTP %d); the untagged slot can then be "+
			"freed and vlan is not required on a site whose owner does so", moveStatus)
		s.PutJSON(ctx, path+"/"+lanID, lan) //nolint:errcheck
	} else {
		t.Logf("the default network refuses to move onto a VLAN (HTTP %d, %s)", moveStatus, v1ErrCode(after))
	}
	deleted, deleteStatus, _ := s.DeleteJSON(ctx, path+"/"+lanID)
	stillThere := len(v1Read(ctx, t, s, path, lanID)) > 0
	if deleteStatus/100 == 2 || !stillThere {
		t.Errorf("the default network was deleted (HTTP %d, still served: %v); the untagged slot can "+
			"then be freed and the vlan requirement does not hold on every site",
			deleteStatus, stillThere)
	} else {
		t.Logf("the default network refuses to be deleted (HTTP %d, %s) and is still served -- the "+
			"untagged slot is taken on every site", deleteStatus, v1ErrCode(deleted))
	}
}

// TestIntegrationWLANWriteContract measures the other resource the artifact
// described everywhere except where it is written. WLAN already has entries
// under ownership, replays and uos_pins; the verb, the path and the
// required-on-create set had never been recorded.
//
// It owns the artifact's writes["WLAN"] entry outright.
//
// One field is required on every create -- ap_group_ids, and a present list
// must name a group -- and one more is required on two of the five security
// modes, so the answer is again per branch. What is NOT required is the
// surprise: a WPA-PSK SSID with no passphrase creates, and so does an SSID
// with no security and no name.
func TestIntegrationWLANWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	const (
		createRel = "api/s/{site}/rest/wlanconf"
		updateRel = "api/s/{site}/rest/wlanconf/{id}"
	)
	path := "/api/s/" + c.Site + "/rest/wlanconf"

	if body, status, err := s.GetJSON(ctx, path); status != 200 {
		t.Fatalf("GET %s answered HTTP %d (%v %v); the collection is not served, so nothing below "+
			"measures the write contract", path, status, body, err)
	}

	apGroup := firstAPGroupID(ctx, t, s, c.Site)
	if apGroup == "" {
		t.Fatal("the site offers no AP group; every WLAN create would fail for the wrong reason")
	}
	// The enterprise modes authenticate against the site's own RADIUS
	// server, and the controller refuses the SSID outright
	// (api.err.RadiusServerNotEnabled) while it is off -- before it gets as
	// far as saying anything about the fields. A prerequisite, not a
	// requirement: it has to be satisfied for the wpaeap and osen sweeps
	// below to be measuring their own bodies.
	setSiteRadiusEnabled(ctx, t, s, c.Site, true)
	radiusProfile := builtinRadiusProfileID(ctx, t, s, c.Site)

	unknownKeyStripped(ctx, t, s, path, map[string]any{
		"name": "write-contract-unknown-key", "security": "open",
		"ap_group_ids": []string{apGroup},
	})

	sweep := &createSweep{s: s, path: path, prefix: "wlan-contract"}
	byBranch := map[string][]string{}
	var psk createBranch
	for _, branch := range wlanCreateBranches(apGroup, radiusProfile) {
		if branch.selector["security"] == "wpapsk" {
			psk = branch
		}
		_, required := sweep.reduce(ctx, t, branch.seed, branch.selector)
		byBranch[createBranchKey(branch.selector)] = required
	}

	// ap_group_ids is required in every branch, but "required" only says the
	// key has to be there. Whether it may be empty is its own measurement.
	minItems := map[string]int{}
	if n := listMinItems(ctx, t, sweep, psk.seed, psk.selector, "ap_group_ids"); n > 0 {
		minItems["ap_group_ids"] = n
	}

	wlanPassphraseless(ctx, t, s, path, apGroup)

	updateVerb, updatePath, requiredOnUpdate := v1UpdateContract(ctx, t, s, path, updateRel, map[string]any{
		"name": "write-contract-update", "security": "open",
		"ap_group_ids": []string{apGroup},
	})

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: createRel,
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate:     requiredInEveryBranch(byBranch),
		RequiredOnCreateWhen: byBranch,
		RequiredOnUpdate:     requiredOnUpdate,
		MinItems:             minItems,
	}

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Writes == nil {
				a.Writes = map[string]behavior.WriteContract{}
			}
			a.Writes["WLAN"] = contract
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok || art.Writes == nil {
		t.Logf("no pinned write contracts in %s; run with BEHAVIOR_WRITE=1 to record the WLAN one",
			behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	compareWriteContract(t, "WLAN", art.Writes, contract)
}

// wlanCreateBranches is one accepted body per security mode the collection
// takes. All five are measured, not the two a caller is likeliest to write:
// wpaeap and osen turn out to want a RADIUS profile that the other three do
// not, and a set measured on wpapsk alone would have missed it.
func wlanCreateBranches(apGroup, radiusProfile string) []createBranch {
	base := func(security string) map[string]any {
		return map[string]any{
			"name": "x", "security": security, "enabled": true,
			"ap_group_ids": []string{apGroup}, "ap_group_mode": "all",
			"wlan_band": "both", "hide_ssid": false, "is_guest": false,
			"vlan_enabled": false, "setting_preference": "manual",
		}
	}
	psk := base("wpapsk")
	psk["wpa_mode"] = "wpa2"
	psk["wpa_enc"] = "ccmp"
	psk["x_passphrase"] = "write-contract-probe"

	eap := base("wpaeap")
	eap["wpa_mode"] = "wpa2"
	eap["wpa_enc"] = "ccmp"
	eap["radiusprofile_id"] = radiusProfile

	osen := base("osen")
	osen["radiusprofile_id"] = radiusProfile

	return []createBranch{
		{selector: map[string]string{"security": "open"}, seed: base("open")},
		{selector: map[string]string{"security": "wpapsk"}, seed: psk},
		{selector: map[string]string{"security": "wep"}, seed: base("wep")},
		{selector: map[string]string{"security": "wpaeap"}, seed: eap},
		{selector: map[string]string{"security": "osen"}, seed: osen},
	}
}

// wlanPassphraseless measures the create the reduction says is legal and
// nobody would guess: security wpapsk with no x_passphrase, no wpa_mode and
// no wpa_enc.
//
// It is accepted, and the SSID is stored with all three absent. Asserted
// rather than recorded, for the same reason as the purposeless network: the
// artifact's required_on_create can say a create was refused and nothing
// else, and this one was not.
func wlanPassphraseless(ctx context.Context, t *testing.T, s *controllertest.Session, path, apGroup string) {
	t.Helper()
	body, status, err := s.PostJSON(ctx, path, map[string]any{
		"name": "write-contract-no-passphrase", "security": "wpapsk",
		"ap_group_ids": []string{apGroup},
	})
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	id := objectID(firstData(t, body))
	if id != "" {
		defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	}
	if status/100 != 2 {
		t.Errorf("a wpapsk SSID with no passphrase was refused (HTTP %d, %s); it was measured being "+
			"accepted, and the wpapsk branch's required set reads x_passphrase as optional because "+
			"of it", status, v1ErrCode(body))
		return
	}
	got, readStatus, err := s.GetJSON(ctx, path+"/"+id)
	if readStatus != 200 {
		t.Fatalf("GET %s/%s answered HTTP %d (%v)", path, id, readStatus, err)
	}
	stored := firstData(t, got)
	for _, wire := range []string{"x_passphrase", "wpa_mode", "wpa_enc"} {
		if v, present := stored[wire]; present {
			t.Errorf("the passphrase-less wpapsk SSID stored %s=%v; it was measured storing none, and "+
				"a controller that now fills one in has changed what the branch requires", wire, v)
		}
	}
	t.Logf("LOUD: security wpapsk with no x_passphrase, wpa_mode or wpa_enc is accepted (HTTP %d) "+
		"and stored with none of them: %v", status, sortedWireNames(stored))
}

// networkVPNBranchesUnmeasured walks every vpn_type the controller's own
// field definitions declare against each VPN purpose, and records nothing.
//
// The three vpn_type branches above are the ones this probe has an accepted
// body for. NetworkVPNTypeValues names thirteen, and the artifact says
// nothing about the other ten -- which is correct and which is also
// invisible unless something says so. So this sweep sends the smallest body
// that names the branch and reports what came back:
//
//	accepted             the branch needs nothing more; it could be measured
//	api.err.InvalidValue the purpose does not admit that vpn_type at all
//	anything else        the purpose admits it and wants fields nothing here
//	                     supplies, so its required set stays unknown
//
// Reading the list off the controller's definitions rather than naming the
// types here is the point: a controller that grows a tunnel type puts it in
// this log on the next run instead of waiting for someone to remember.
func networkVPNBranchesUnmeasured(ctx context.Context, t *testing.T, s *controllertest.Session, site, radiusProfile string) {
	t.Helper()
	path := "/api/s/" + site + "/rest/networkconf"
	sweep := &createSweep{s: s, path: path, prefix: "vpn-type"}

	measured := map[string]bool{
		PurposeSiteVPN + "/ipsec-vpn":          true,
		PurposeVPNClient + "/wireguard-client": true,
		PurposeUserVPN + "/openvpn-server":     true,
	}
	for _, purpose := range []string{PurposeSiteVPN, PurposeVPNClient, PurposeUserVPN} {
		var reachable, refused, notInEnum []string
		for _, vpnType := range NetworkVPNTypeValues {
			if measured[purpose+"/"+vpnType] {
				continue
			}
			doc := map[string]any{"name": "vpn-type", "purpose": purpose, "vpn_type": vpnType}
			if purpose == PurposeUserVPN {
				// Without it every remote-user-vpn body is refused for the
				// site's RADIUS profile before the tunnel type is reached,
				// and each branch would read as unreachable for the wrong
				// reason.
				doc["radiusprofile_id"] = radiusProfile
			}
			v := sweep.attempt(ctx, t, doc, map[string]string{"purpose": purpose, "vpn_type": vpnType})
			switch {
			case v.accepted:
				reachable = append(reachable, vpnType)
			case strings.Contains(v.why, "api.err.InvalidValue"):
				notInEnum = append(notInEnum, vpnType)
			default:
				refused = append(refused, vpnType+" ("+v.why+")")
			}
		}
		t.Logf("LOUD: the artifact says nothing about %s over %v. Each is refused, none of the "+
			"refusals was narrowed to the fields it wants, and a required set nothing observed the "+
			"controller accept is not one to publish.", purpose, refused)
		if len(notInEnum) > 0 {
			t.Logf("%s refuses %v as tunnel types outright, though the controller's own field "+
				"definitions declare them", purpose, notInEnum)
		}
		if len(reachable) > 0 {
			t.Logf("%s creates from purpose and vpn_type alone over %v -- those branches require "+
				"nothing, which is all an entry each could say", purpose, reachable)
		}
	}
}
