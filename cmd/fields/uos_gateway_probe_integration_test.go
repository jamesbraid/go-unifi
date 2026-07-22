//go:build integration

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationUOSGatewayProbe boots a UniFi OS Server simulation and
// re-runs the gateway-dependent v2 operations the standalone controller
// rejects (BgpConfig -> BgpUnsupportedDevice, FirewallZone ->
// CouldNotFindHotspotFirewallZone), to establish whether UOS is the harness
// that unblocks the drift-seeding and route-based-IPsec work the standalone
// sim cannot host.
//
// Gated behind UNIFI_UOS_TEST: UOS is a heavy multi-minute systemd boot, so
// it stays out of the default integration gate. It asserts only that the UOS
// harness itself works (boot + login); the gateway-operation outcomes are
// logged for now — once confirmed unblocked they can become assertions and
// move the corresponding encoder/drift work off the allowlist.
//
// First run (2026-07-22, UOS 5.1.21-sim) confirmed the thesis: BGP returned a
// 400 payload-validation error (uploadedFileName required) instead of the
// standalone sim's 404 api.err.BgpUnsupportedDevice — the feature is present
// on UOS, it just needs a valid payload. Firewall-zone POST still returns
// CouldNotFindHotspotFirewallZone (GET works, returns an empty list), so it
// needs a further prerequisite. UOS is the right harness for the
// gateway-dependent work; wiring each field is follow-up.
func TestIntegrationUOSGatewayProbe(t *testing.T) {
	if os.Getenv("UNIFI_UOS_TEST") == "" {
		t.Skip("set UNIFI_UOS_TEST=1 to run the heavy UOS gateway probe")
	}
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	// StartUOS + NewSession both fail the test on error, so reaching here at
	// all proves the UOS harness boots and its bundled Network API answers a
	// login — the "does the harness work" assertion.
	c := controllertest.StartUOS(ctx, t)
	s := c.NewSession(ctx, t)
	t.Logf("UOS harness up at %s", c.BaseURL)

	// BGP: the standalone sim returns 404 api.err.BgpUnsupportedDevice.
	bgp := map[string]any{"enabled": true, "description": "uos-probe", "frr_bgpd_config": "router bgp 65000\n"}
	body, status, err := s.PostJSON(ctx, "/v2/api/site/"+c.Site+"/bgp/config", bgp)
	t.Logf("bgp/config POST: status=%d err=%v body=%v", status, err, body)

	// Firewall zone: the standalone sim returns 404 CouldNotFindHotspotFirewallZone.
	zbody, zstatus, _ := s.GetJSON(ctx, "/v2/api/site/"+c.Site+"/firewall/zone")
	t.Logf("firewall/zone GET (seeded zones?): status=%d body=%v", zstatus, zbody)
	zone := map[string]any{"name": "uos-probe-zone", "network_ids": []string{}}
	body, status, err = s.PostJSON(ctx, "/v2/api/site/"+c.Site+"/firewall/zone", zone)
	t.Logf("firewall/zone POST: status=%d err=%v body=%v", status, err, body)
}
