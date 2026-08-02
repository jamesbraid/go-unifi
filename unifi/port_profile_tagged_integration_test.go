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
// If the tagged variants ever FAIL here, the controller has started
// honoring the field again: restore the [PortProfile.field.tagged_networkconf_ids]
// pin in overrides/fields.toml (git history of this file has the stanza) and
// flip those assertions. If the control variant fails, the exclusion
// mechanism itself changed and PortProfile's model needs re-measuring.
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

	tagged := []string{vlan51}
	excluded := []string{vlan52}

	variants := []struct {
		name    string
		payload map[string]any
		// wantTaggedStripped: the controller must accept the write and store
		// the document without tagged_networkconf_ids.
		wantTaggedStripped bool
	}{
		{"legacy", map[string]any{
			"forward":                "customize",
			"native_networkconf_id":  lanID,
			"tagged_networkconf_ids": tagged,
		}, true},
		{"legacy_opmode", map[string]any{
			"op_mode":                "switch",
			"forward":                "customize",
			"native_networkconf_id":  lanID,
			"tagged_networkconf_ids": tagged,
		}, true},
		{"mgmt_custom_plus_tagged", map[string]any{
			"forward":                "customize",
			"native_networkconf_id":  lanID,
			"tagged_vlan_mgmt":       "custom",
			"tagged_networkconf_ids": tagged,
		}, true},
		{"mgmt_auto_plus_tagged", map[string]any{
			"native_networkconf_id":  lanID,
			"tagged_vlan_mgmt":       "auto",
			"tagged_networkconf_ids": tagged,
		}, true},
		// Control: the shape the 10.x UI writes. tagged_networkconf_ids is
		// absent by design; excluded_networkconf_ids must persist or the
		// exclusion mechanism the SDK models has changed under us.
		{"modern_excluded_control", map[string]any{
			"forward":                  "customize",
			"native_networkconf_id":    lanID,
			"tagged_vlan_mgmt":         "custom",
			"excluded_networkconf_ids": excluded,
		}, false},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			payload := map[string]any{"name": "probe-tagged-" + v.name}
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
			assertTaggedFate(t, "CREATE", stored, tagged, v.wantTaggedStripped)
			if !v.wantTaggedStripped {
				if got, ok := stored["excluded_networkconf_ids"]; !ok || !jsonEqual(got, excluded) {
					t.Errorf("CREATE lost excluded_networkconf_ids: want %v got %v (ok=%v)", excluded, got, ok)
				}
				if fwd := stored["forward"]; fwd != "customize" {
					t.Errorf("CREATE rewrote forward under tagged_vlan_mgmt=custom: got %v, want customize", fwd)
				}
			}

			// The update path answers separately from create; a PUT of the
			// same shape must behave identically.
			payload["_id"] = id
			respBody, status, err := s.PutJSON(ctx, "/api/s/"+c.Site+"/rest/portconf/"+id, payload)
			if err != nil {
				t.Fatalf("update portconf: transport error: %v", err)
			}
			if status != 200 {
				t.Fatalf("controller now REJECTS this shape on PUT (HTTP %d): %v — re-measure and update this guard", status, respBody)
			}
			after := fetchPortConf(ctx, t, s, c.Site, id)
			assertTaggedFate(t, "PUT", after, tagged, v.wantTaggedStripped)
			if !v.wantTaggedStripped {
				if got, ok := after["excluded_networkconf_ids"]; !ok || !jsonEqual(got, excluded) {
					t.Errorf("PUT lost excluded_networkconf_ids: want %v got %v (ok=%v)", excluded, got, ok)
				}
			}
		})
	}
}

// assertTaggedFate enforces the measured behavior: tagged_networkconf_ids
// must be absent from the stored document (wantStripped), or present and
// intact where a variant expects otherwise.
func assertTaggedFate(t *testing.T, phase string, stored map[string]any, sent []string, wantStripped bool) {
	t.Helper()
	got, ok := stored["tagged_networkconf_ids"]
	if wantStripped {
		if ok {
			t.Errorf("%s: controller now PERSISTS tagged_networkconf_ids (= %v) — restore the "+
				"[PortProfile.field.tagged_networkconf_ids] pin in overrides/fields.toml and flip this guard", phase, got)
			return
		}
		t.Logf("%s STRIPPED tagged_networkconf_ids as measured (forward=%v tagged_vlan_mgmt=%v)",
			phase, stored["forward"], stored["tagged_vlan_mgmt"])
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
