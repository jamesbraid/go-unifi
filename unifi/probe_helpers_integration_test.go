//go:build integration

// unifi/probe_helpers_integration_test.go
package unifi

import (
	"context"
	"testing"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/internal/probe"
)

// The probe primitives live in internal/probe so the capture-time behaviour
// stage and these tests share one implementation. These file-local wrappers
// keep the ~30 existing call sites unchanged while the definitions collapse
// to one place.

func firstData(_ *testing.T, body any) map[string]any { return probe.FirstData(body) }

func jsonEqual(a, b any) bool { return probe.JSONEqual(a, b) }

// discardedFields reports, per asked field, what the controller did not store
// as asked -- Changed or Dropped, keyed by wire name with a before/after.
func discardedFields(asked, stored map[string]any) map[string]string {
	out := map[string]string{}
	for _, r := range probe.Classify(asked, stored) {
		out[r.Wire] = r.Detail
	}
	return out
}

// probeDeps carries ids resolved from the live controller that a payload has
// to reference by value. A well-formed but nonexistent id is rejected, and a
// rejected write measures the rejection rather than the thing under test.
type probeDeps struct {
	apGroupID    string
	wanNetworkID string
}

// firstAPGroupID resolves the site's default AP group. Every site ships one,
// and a WLAN cannot be created without naming a real group.
func firstAPGroupID(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/v2/api/site/"+site+"/apgroups")
	if err != nil || status != 200 {
		t.Logf("no AP groups available (status %d, %v); WLAN probes will be rejected", status, err)
		return ""
	}
	items, ok := body.([]any)
	if !ok || len(items) == 0 {
		t.Logf("AP group list is empty; WLAN probes will be rejected")
		return ""
	}
	m, _ := items[0].(map[string]any)
	return objectID(m)
}

// ensureWANNetwork gives the site a WAN networkconf and returns its id.
//
// A demo site ships without one, and adoption does not create it either. The
// v2 collections reference it, and a request that names nothing is answered
// with a non-JSON HTTP 500 rather than a rejection.
func ensureWANNetwork(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()

	body, status, err := s.PostJSON(ctx, "/api/s/"+site+"/rest/networkconf", map[string]any{
		"name": "probe-wan", "purpose": PurposeWAN, "enabled": true,
		"wan_networkgroup": "WAN", "wan_type": "dhcp", "wan_type_v6": "disabled",
	})
	if err != nil || (status != 200 && status != 201) {
		t.Logf("unable to seed a WAN network (status %d, %v); v2 probes will fail", status, err)
		return ""
	}
	id, _ := firstData(t, body)["_id"].(string)
	if id != "" {
		t.Cleanup(func() {
			s.DeleteJSON(context.WithoutCancel(ctx), "/api/s/"+site+"/rest/networkconf/"+id) //nolint:errcheck
		})
	}
	return id
}

// firewallZonePair seeds two probe zones and returns ids to address a
// policy's source and destination with. Empty ids mean the controller
// offered no zones at all; each caller decides whether that is a skip or a
// failure. Three tests used to carry this block each.
func firewallZonePair(ctx context.Context, t *testing.T, s *controllertest.Session, site string) (src, dst string) {
	t.Helper()
	for _, name := range []string{"probe-zone-src", "probe-zone-dst"} {
		s.PostJSON(ctx, "/v2/api/site/"+site+"/firewall/zone", //nolint:errcheck
			map[string]any{"name": name, "network_ids": []string{}})
	}
	zones, status, err := s.GetJSON(ctx, "/v2/api/site/"+site+"/firewall/zone")
	if err != nil || status != 200 {
		t.Fatalf("list zones (HTTP %d): %v", status, err)
	}
	var ids []string
	for _, z := range asSlice(zones) {
		if m, _ := z.(map[string]any); m != nil {
			if id, _ := m["_id"].(string); id != "" {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return "", ""
	}
	return ids[0], ids[len(ids)-1]
}

// firewallPolicyProbeBase is the smallest policy body the controller
// accepts, addressed at the given zones. index must be unique among the
// policies one probe creates: the controller refuses a duplicate index.
func firewallPolicyProbeBase(name string, index int, src, dst string) map[string]any {
	return map[string]any{
		"name": name, "enabled": true,
		"action": "ALLOW", "predefined": false, "index": index,
		"protocol": "all", "ip_version": "BOTH",
		"connection_state_type": "ALL", "connection_states": []string{},
		"source":      map[string]any{"zone_id": src, "matching_target": "ANY"},
		"destination": map[string]any{"zone_id": dst, "matching_target": "ANY"},
		"logging":     false, "create_allow_respond": true,
		"schedule": map[string]any{"mode": "ALWAYS", "time_all_day": true, "repeat_on_days": []string{}},
	}
}

// wlanBase is the smallest WLAN a bare controller accepts. Without
// ap_group_ids the create is rejected with api.err.ApGroupMissing.
func wlanBase(name string, deps probeDeps) map[string]any {
	return map[string]any{
		"name":          name,
		"enabled":       true,
		"security":      "wpapsk",
		"wpa_mode":      "wpa2",
		"wpa_enc":       "ccmp",
		"x_passphrase":  "preference-probe",
		"ap_group_ids":  []string{deps.apGroupID},
		"ap_group_mode": "all",
	}
}
