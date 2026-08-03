//go:build integration

// unifi/port_profile_tagged_integration_test.go
package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationPortProfileTaggedNetworks pins the measured fate of
// tagged_networkconf_ids on portconf.
//
// The vendor schema dropped the field in June 2023 when tagged_vlan_mgmt and
// excluded_networkconf_ids were introduced, and the generated PortProfile
// lost it in the same regeneration. Measured 2026-08-02 against 10.4.57
// before deciding whether to pin it back via overrides/fields.toml: the
// controller strips the field from the stored document on every write shape
// tried — create and update, with and without op_mode, alongside
// tagged_vlan_mgmt custom and auto — while the same request's other fields
// persist. forward is now derived from tagged_vlan_mgmt (absent or "auto"
// rewrite forward to "all"; "custom" keeps "customize"), and the only
// tagged-VLAN semantics portconf retains is the exclusion form:
// tagged_vlan_mgmt plus excluded_networkconf_ids, which the control variant
// shows persisting intact. So the pin was not added: modeling the field
// would make the SDK silently send a no-op key and callers would trunk
// nothing, which is the exact class of bug this repo hunts.
//
// The same wire name still persists on Device port_overrides (also absent
// from the vendor schema there; see the hook in cmd/fields/main.go) — the
// two write paths genuinely differ.
//
// A failing tagged variant does not on its own mean the field came back, and
// no outcome here earns the pin by itself. Read what assertTaggedFate
// reports. A stored value that differs from the submitted ids -- a
// normalized [], say -- is still the field being ignored. A value that
// round-trips exactly is persistence, which is necessary and not sufficient:
// the controller could hold the ids as an opaque blob and go on deriving
// trunking from tagged_vlan_mgmt alone. Either way the next step is a
// behavioural measurement -- does a port carrying the profile actually trunk
// the named VLANs, and stop when they are removed -- and the pin goes back
// only on that. TestPortProfileDoesNotModelTaggedNetworks holds the other
// half: the generated type must not grow the field while the controller is
// still stripping it. When the behaviour is genuinely there, restore the
// stanza in overrides/fields.toml and flip both guards:
//
//	[PortProfile.field.tagged_networkconf_ids]
//	add = true
//	name = "TaggedNetworkIDs"
//	type = "string"
//	omitempty = true
//	array = true
//
// If the control variant fails, the exclusion mechanism itself changed and
// PortProfile's model needs re-measuring.
func TestIntegrationPortProfileTaggedNetworks(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	lanID := firstCorporateNetworkID(ctx, t, s, c.Site)
	vlan51 := seedVLANNetwork(ctx, t, s, c.Site, "probe-tagged-vlan51", 51)
	vlan52 := seedVLANNetwork(ctx, t, s, c.Site, "probe-tagged-vlan52", 52)
	vlan53 := seedVLANNetwork(ctx, t, s, c.Site, "probe-tagged-vlan53", 53)

	// Two ids, not one. The pin this test gates is an array field, and a
	// controller that persisted only the first element would round-trip a
	// singleton exactly -- so assertTaggedFate would call the field honored
	// and recommend the pin, while every profile with more than one tagged
	// VLAN silently lost the rest. A list only demonstrates list support if
	// it is long enough to be truncated.
	tagged := []string{vlan51, vlan53}
	excluded := []string{vlan52}

	// wantForward carries the measured 10.4.57 result for each shape, from
	// the matrix in this test's doc comment: forward is derived from
	// tagged_vlan_mgmt, so "custom" keeps "customize" and everything else
	// is rewritten to "all". Asserting it per variant is what stops a
	// controller that kept stripping the ids but changed the derivation
	// from passing silently -- the stripping is only half the measurement.
	//
	// wantTaggedMgmt is asserted only where the variant submits the field.
	// The legacy shapes do not, and what the controller defaults it to was
	// never measured; pinning a guess would fake exactly the kind of
	// unmeasured claim this whole test exists to refuse.
	variants := []struct {
		name    string
		payload map[string]any
		// wantTaggedStripped: the controller must accept the write and store
		// the document without tagged_networkconf_ids.
		wantTaggedStripped bool
		wantForward        string
		wantTaggedMgmt     string
	}{
		{"legacy", map[string]any{
			"forward":                "customize",
			"native_networkconf_id":  lanID,
			"tagged_networkconf_ids": tagged,
		}, true, "all", ""},
		{"legacy_opmode", map[string]any{
			"op_mode":                "switch",
			"forward":                "customize",
			"native_networkconf_id":  lanID,
			"tagged_networkconf_ids": tagged,
		}, true, "all", ""},
		{"mgmt_custom_plus_tagged", map[string]any{
			"forward":                "customize",
			"native_networkconf_id":  lanID,
			"tagged_vlan_mgmt":       "custom",
			"tagged_networkconf_ids": tagged,
		}, true, "customize", "custom"},
		{"mgmt_auto_plus_tagged", map[string]any{
			"native_networkconf_id":  lanID,
			"tagged_vlan_mgmt":       "auto",
			"tagged_networkconf_ids": tagged,
		}, true, "all", "auto"},
		// Control: the shape the 10.x UI writes. tagged_networkconf_ids is
		// absent by design; excluded_networkconf_ids must persist or the
		// exclusion mechanism the SDK models has changed under us.
		{"modern_excluded_control", map[string]any{
			"forward":                  "customize",
			"native_networkconf_id":    lanID,
			"tagged_vlan_mgmt":         "custom",
			"excluded_networkconf_ids": excluded,
		}, false, "customize", "custom"},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			createName := "probe-tagged-" + v.name
			payload := map[string]any{"name": createName}
			for k, val := range v.payload {
				payload[k] = val
			}
			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/portconf", payload)
			if err != nil {
				t.Fatalf("create portconf: transport error: %v", err)
			}
			if status != 200 {
				t.Fatalf("controller now REJECTS this shape (HTTP %d): %v — re-measure and update this guard", status, body)
			}
			id, _ := firstData(t, body)["_id"].(string)
			if id == "" {
				t.Fatalf("no _id in create response: %v", body)
			}
			defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/portconf/"+id) //nolint:errcheck

			stored := fetchPortConf(ctx, t, s, c.Site, id)
			assertSurvived(t, "CREATE", stored, createName, lanID)
			assertDerivedMode(t, "CREATE", stored, v.wantForward, v.wantTaggedMgmt)
			assertTaggedFate(t, "CREATE", stored, tagged, v.wantTaggedStripped)
			if !v.wantTaggedStripped {
				if got, ok := stored["excluded_networkconf_ids"]; !ok || !jsonEqual(got, excluded) {
					t.Errorf("CREATE lost excluded_networkconf_ids: want %v got %v (ok=%v)", excluded, got, ok)
				}
			}

			// The update path answers separately from create; a PUT of the
			// same shape must behave identically.
			//
			// The name changes, and it is the only thing that does. Resending
			// the created document unchanged would leave every read-back
			// assertion below satisfied by the CREATE, so a controller that
			// accepted updates and applied none -- or returned an
			// application error inside an HTTP 200 envelope -- would look
			// exactly like one that behaved. The sentinel is what makes the
			// PUT observable at all.
			updateName := createName + "-updated"
			payload["_id"] = id
			payload["name"] = updateName
			respBody, status, err := s.PutJSON(ctx, "/api/s/"+c.Site+"/rest/portconf/"+id, payload)
			if err != nil {
				t.Fatalf("update portconf: transport error: %v", err)
			}
			if status != 200 {
				t.Fatalf("controller now REJECTS this shape on PUT (HTTP %d): %v — re-measure and update this guard", status, respBody)
			}
			after := fetchPortConf(ctx, t, s, c.Site, id)
			assertSurvived(t, "PUT", after, updateName, lanID)
			// Every assertion CREATE makes, because "the PUT behaves
			// identically" is the claim under test and a controller that
			// diverged only on updates is the interesting failure.
			assertDerivedMode(t, "PUT", after, v.wantForward, v.wantTaggedMgmt)
			assertTaggedFate(t, "PUT", after, tagged, v.wantTaggedStripped)
			if !v.wantTaggedStripped {
				if got, ok := after["excluded_networkconf_ids"]; !ok || !jsonEqual(got, excluded) {
					t.Errorf("PUT lost excluded_networkconf_ids: want %v got %v (ok=%v)", excluded, got, ok)
				}
			}
		})
	}
}

// assertDerivedMode pins what the controller decided about forwarding.
//
// Stripping tagged_networkconf_ids is only half of what was measured; the
// other half is that forward stops being a field you set and becomes one
// derived from tagged_vlan_mgmt. A controller that kept stripping the ids
// but changed that derivation would leave every other assertion here happy
// while the SDK's model of port profiles quietly went wrong, and the
// stripping result -- the thing this test exists to hold -- would still
// report green.
//
// wantMgmt is empty for the shapes that never submit tagged_vlan_mgmt: what
// the controller defaults it to on those was not measured, and asserting a
// guess would manufacture the kind of unmeasured claim this test refuses on
// principle. Logged instead, so the value is in the run output the day
// somebody does measure it.
func assertDerivedMode(t *testing.T, phase string, stored map[string]any, wantForward, wantMgmt string) {
	t.Helper()

	if got, _ := stored["forward"].(string); got != wantForward {
		t.Errorf("%s: forward is %q, want %q — the controller changed how it derives forwarding "+
			"from tagged_vlan_mgmt, so the port-profile model needs re-measuring", phase, got, wantForward)
	}

	got, _ := stored["tagged_vlan_mgmt"].(string)
	if wantMgmt == "" {
		t.Logf("%s stored tagged_vlan_mgmt=%q (unasserted: this shape does not submit it)", phase, got)
		return
	}
	if got != wantMgmt {
		t.Errorf("%s: tagged_vlan_mgmt is %q, want %q — the field the derivation reads did not "+
			"round-trip", phase, got, wantMgmt)
	}
}

// assertSurvived checks the fields every variant submits and no variant
// expects the controller to touch.
//
// Without it the tagged variants assert only an absence, and an absence is
// what a controller that discarded the whole write would also produce. A
// 200 that stored nothing, or stored a document with the submitted name and
// native network dropped, would read as "tagged_networkconf_ids was
// stripped, as measured" -- the test would report the finding it is supposed
// to be establishing. Pinning two fields that must survive is what separates
// selective stripping from wholesale rejection.
//
// name doubles as the update sentinel, so the caller passes whichever value
// that phase submitted.
func assertSurvived(t *testing.T, phase string, stored map[string]any, wantName, wantNativeID string) {
	t.Helper()
	if got, _ := stored["name"].(string); got != wantName {
		t.Errorf("%s: name is %q, want %q — the write did not land, so nothing else read back here means anything",
			phase, got, wantName)
	}
	if got, _ := stored["native_networkconf_id"].(string); got != wantNativeID {
		t.Errorf("%s: native_networkconf_id is %q, want %q — a submitted field the controller should keep went missing",
			phase, got, wantNativeID)
	}
}

// assertTaggedFate enforces the measured behavior: tagged_networkconf_ids
// must be absent from the stored document (wantStripped), or present and
// intact where a variant expects otherwise.
//
// Under wantStripped there are three outcomes, not two, and none of them
// authorizes restoring the pin on its own. The key being back is not enough:
// a controller that starts storing a normalized default there -- [] most
// plausibly -- while still discarding the submitted ids would trip a
// presence-only check and have this test recommend a pin for a field that
// remains a silent no-op, which is the exact failure the pin was dropped to
// stop faking.
//
// Comparing against what was sent separates that case out, but round-tripping
// is still only persistence. The controller could store the ids verbatim as
// an opaque blob and go on deriving trunking from tagged_vlan_mgmt alone, and
// a pin restored on that evidence would put the same silent no-op back. The
// policy wants proof the controller *uses* the ids, which is a behavioural
// measurement no read-back can stand in for -- so the exact-match branch
// escalates to "measure the semantics", not to "restore the pin".
func assertTaggedFate(t *testing.T, phase string, stored map[string]any, sent []string, wantStripped bool) {
	t.Helper()
	got, ok := stored["tagged_networkconf_ids"]
	if wantStripped {
		switch {
		case !ok:
			t.Logf("%s STRIPPED tagged_networkconf_ids as measured (forward=%v tagged_vlan_mgmt=%v)",
				phase, stored["forward"], stored["tagged_vlan_mgmt"])
		case jsonEqual(got, sent):
			t.Errorf("%s: controller now PERSISTS tagged_networkconf_ids unchanged — sent %v and "+
				"read it back identical. That is necessary for the pin and not sufficient: it "+
				"proves storage, not use, and a controller that keeps the ids as an opaque blob "+
				"while still deriving trunking from tagged_vlan_mgmt would look exactly like "+
				"this. Before restoring the [PortProfile.field.tagged_networkconf_ids] pin, "+
				"measure that a port carrying this profile actually trunks the named VLANs and "+
				"stops trunking them when they are removed. Pin on that, not on this", phase, sent)
		default:
			t.Errorf("%s: controller now stores tagged_networkconf_ids but not what was sent "+
				"(sent %v, stored %v). Do NOT restore the pin on this alone: a defaulted or "+
				"normalized value is still the field being ignored. Re-measure what the "+
				"controller does with the ids first", phase, sent, got)
		}
		return
	}
	if ok && !jsonEqual(got, sent) {
		t.Errorf("%s MUTATED tagged_networkconf_ids: sent %v stored %v", phase, sent, got)
	}
}

// seedVLANNetwork creates a VLAN-only network and returns its id.
func seedVLANNetwork(ctx context.Context, t *testing.T, s *controllertest.Session, site, name string, vlan int) string {
	t.Helper()
	body, status, err := s.PostJSON(ctx, "/api/s/"+site+"/rest/networkconf", map[string]any{
		"name":         name,
		"purpose":      PurposeVLANOnly,
		"vlan":         vlan,
		"vlan_enabled": true,
		"enabled":      true,
	})
	if err != nil || status != 200 {
		t.Fatalf("create vlan network %s: status=%d err=%v body=%v", name, status, err, body)
	}
	id, _ := firstData(t, body)["_id"].(string)
	if id == "" {
		t.Fatalf("create vlan network %s: no _id in response: %v", name, body)
	}
	t.Cleanup(func() {
		s.DeleteJSON(ctx, "/api/s/"+site+"/rest/networkconf/"+id) //nolint:errcheck
	})
	return id
}

// firstCorporateNetworkID returns the site's default LAN network id.
func firstCorporateNetworkID(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/rest/networkconf")
	if err != nil || status != 200 {
		t.Fatalf("list networks: status=%d err=%v", status, err)
	}
	envelope, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("list networks: unexpected body shape %T", body)
	}
	data, _ := envelope["data"].([]any)
	for _, item := range data {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if m["purpose"] == PurposeCorporate {
			if id, ok := m["_id"].(string); ok {
				return id
			}
		}
	}
	t.Fatal("no corporate network on the controller to use as the native network")
	return ""
}

// fetchPortConf returns the stored portconf document, matched by _id.
func fetchPortConf(ctx context.Context, t *testing.T, s *controllertest.Session, site, id string) map[string]any {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/rest/portconf/"+id)
	if err != nil || status != 200 {
		t.Fatalf("fetch portconf %s: status=%d err=%v", id, status, err)
	}
	doc := firstData(t, body)
	if doc == nil || doc["_id"] != id {
		t.Fatalf("fetch portconf %s: stored document absent or id mismatch: %v", id, body)
	}
	return doc
}
