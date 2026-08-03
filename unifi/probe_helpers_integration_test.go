//go:build integration

// unifi/probe_helpers_integration_test.go
package unifi

import (
	"context"
	"testing"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

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
