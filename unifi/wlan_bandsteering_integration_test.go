//go:build integration

package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationWLANBandsteeringModeNotStored pins what 10.4.57 does with
// the compat field bandsteering_mode: accepts it and stores nothing. The
// override in overrides/fields.toml documents that, and a consumer must not
// expect the value echoed back on this release. When a locked controller
// starts storing it, this fails and the override comment is what to update.
func TestIntegrationWLANBandsteeringModeNotStored(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/wlanconf", map[string]any{
		"name": "bandsteer-probe", "enabled": true,
		"security": "wpapsk", "x_passphrase": "probe-passphrase",
		"wpa_mode": "wpa2", "wpa_enc": "ccmp",
		"usergroup_id":      firstUserGroupID(ctx, t, s, c.Site),
		"wlangroup_id":      firstWLANGroupID(ctx, t, s, c.Site),
		"ap_group_ids":      []string{requiredAPGroupID(ctx, t, s, c.Site)},
		"bandsteering_mode": "prefer_5g",
	})
	if err != nil || status != 200 {
		t.Fatalf("wlanconf with bandsteering_mode rejected (HTTP %d): %v %v", status, body, err)
	}
	id, _ := firstData(t, body)["_id"].(string)
	defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/wlanconf/"+id) //nolint:errcheck

	body, status, err = s.GetJSON(ctx, "/api/s/"+c.Site+"/rest/wlanconf/"+id)
	if err != nil || status != 200 {
		t.Fatalf("re-read failed (HTTP %d): %v", status, err)
	}
	if v, ok := firstData(t, body)["bandsteering_mode"]; ok {
		t.Fatalf("this controller stores bandsteering_mode (%v); update the override's "+
			"comment in overrides/fields.toml, the field is no longer write-only here", v)
	}
}
