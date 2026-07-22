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
// probes the gateway-dependent v2 operations the standalone controller
// rejects (BgpConfig -> BgpUnsupportedDevice, FirewallZone ->
// CouldNotFindHotspotFirewallZone). It exists to answer whether UOS's full
// stack makes those features available without an adopted gateway device.
//
// It does not. Re-probed 2026-07-22 (UOS 5.1.21-sim) with complete payloads:
// BGP returns the same 404 api.err.BgpUnsupportedDevice as the standalone
// sim, and firewall-zone POST the same CouldNotFindHotspotFirewallZone. (An
// earlier partial-payload run returned 400 "uploadedFileName required" and
// was misread as the feature being present -- that 400 is validation firing
// on the missing field first; supply it and the device-support 404 shows
// through.) So the gateway features require an adopted gateway device, not
// just the full UOS stack, and wiring them waits on a device-emulation
// harness.
//
// Gated behind UNIFI_UOS_TEST: UOS is a heavy multi-minute systemd boot, so
// it stays out of the default integration gate. It asserts only that the UOS
// harness itself works (boot + login) -- a valid full-stack smoke test -- and
// logs the (still device-gated) gateway-operation outcomes.
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

	// BGP: both the standalone sim and UOS return 404 BgpUnsupportedDevice.
	// Send a complete payload (uploaded_file_name included) so the response is
	// the device-support 404, not a 400 for the missing field.
	bgp := map[string]any{
		"enabled": true, "description": "uos-probe",
		"frr_bgpd_config": "router bgp 65000\n", "uploaded_file_name": "bgp.conf",
	}
	body, status, err := s.PostJSON(ctx, "/v2/api/site/"+c.Site+"/bgp/config", bgp)
	t.Logf("bgp/config POST: status=%d err=%v body=%v", status, err, body)

	// Firewall zone: both the standalone sim and UOS return 404
	// CouldNotFindHotspotFirewallZone.
	zbody, zstatus, _ := s.GetJSON(ctx, "/v2/api/site/"+c.Site+"/firewall/zone")
	t.Logf("firewall/zone GET (seeded zones?): status=%d body=%v", zstatus, zbody)
	zone := map[string]any{"name": "uos-probe-zone", "network_ids": []string{}}
	body, status, err = s.PostJSON(ctx, "/v2/api/site/"+c.Site+"/firewall/zone", zone)
	t.Logf("firewall/zone POST: status=%d err=%v body=%v", status, err, body)
}
