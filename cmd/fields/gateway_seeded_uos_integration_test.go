//go:build integration

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationSeededUOSFirewallZoneSeed proves the one thing that makes
// FirewallZone drift-checkable: POST /v2/api/site/{site}/firewall/zone
// PERSISTS the zone even though it answers 404
// api.err.CouldNotFindHotspotFirewallZone. The 404 is thrown after the write,
// while the handler looks for the hotspot zone the site has never had, so
// reading the collection back is the only way to see the object.
//
// Every probe before this one classified that 404 as "gated, nothing created"
// because it never re-read the collection. It creates a zone every time.
//
// Two conditions, both measured 2026-07-24 on the seeded UniFi OS Server image
// (real mode, wizard completed — Network 10.4.57, the same app the standalone
// sim runs):
//
//   - The POST must be the FIRST request to the zone collection on that site.
//     Reading the collection first poisons it: three runs that GET before
//     posting never landed a zone (one of them retried twenty times over five
//     minutes), while every run that posted first landed one immediately.
//   - Only that first POST lands. A second answers the same 404 and creates
//     nothing, so a site yields exactly one seeded zone — which is all a drift
//     comparison needs.
//
// Whether an adopted gateway is also required is NOT established: this test
// adopts one because that is the configuration the passing runs used, but the
// no-gateway runs that failed also did the poisoning pre-read, so the two
// factors were never separated.
//
// Gated behind UNIFI_GATEWAY_TEST: the seeded UOS is a systemd boot of minutes.
func TestIntegrationSeededUOSFirewallZoneSeed(t *testing.T) {
	if os.Getenv("UNIFI_GATEWAY_TEST") == "" {
		t.Skip("set UNIFI_GATEWAY_TEST=1 to run the seeded UOS zone-seed test")
	}
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	c := controllertest.StartUOSSeeded(ctx, t)
	// One login for the whole test: UniFi OS rate-limits its login endpoint.
	s := c.NewSession(ctx, t)
	t.Logf("seeded UOS up at %s (network API %s)", c.RootURL, c.BaseURL)

	// Do NOT read the collection before writing to it. A GET on the zone
	// collection before the first POST makes that POST fail to persist —
	// measured 2026-07-24, twenty retries over five minutes never landed a
	// zone after a pre-read, while the identical POST issued as the first
	// zone call on the same image lands immediately.
	//
	// Adopting a gateway first is not a prerequisite for the write (see the
	// doc comment) — it is what makes this the with-hardware case.
	d := controllertest.AdoptGateway(ctx, t, c, s)
	t.Logf("gateway adopted: mac=%s model=%q type=%q state=%d", d.MAC, d.Model, d.Type, d.State)

	const zoneName = "probe-seeded-zone"
	body, status, err := s.PostJSON(ctx, "/v2/api/site/"+c.Site+"/firewall/zone", map[string]any{
		"name":        zoneName,
		"network_ids": []string{},
	})
	if err != nil {
		t.Fatalf("zone POST transport error: %v", err)
	}
	t.Logf("zone POST answered HTTP %d: %s", status, jsonString(body))

	// The status lies about what happened; the collection is the truth.
	zones, gstatus, gerr := s.GetJSON(ctx, "/v2/api/site/"+c.Site+"/firewall/zone")
	if gerr != nil || gstatus != 200 {
		t.Fatalf("list zones after seed: status=%d err=%v", gstatus, gerr)
	}
	list, ok := zones.([]any)
	if !ok {
		t.Fatalf("zone collection is not a list: %s", jsonString(zones))
	}
	var seeded map[string]any
	for _, item := range list {
		z, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if n, _ := z["name"].(string); n == zoneName {
			seeded = z
			break
		}
	}
	if seeded == nil {
		t.Fatalf("zone %q absent after POST with a gateway adopted: the write no longer persists, so FirewallZone drift needs a new seeding route (collection: %s)",
			zoneName, jsonString(zones))
	}
	t.Logf("SEEDED FirewallZone despite HTTP %d: %s", status, jsonString(seeded))

	// The live object carries fields the hand-written schema does not, which is
	// exactly the drift the v2 gate exists to catch once this seed is wired in.
	for _, k := range []string{"_id", "attr_no_edit", "cloud_template", "external_id"} {
		if _, present := seeded[k]; !present {
			t.Logf("live zone has no %q", k)
		}
	}
}

// TestIntegrationSeededUOSBgpGate records that bgp/config stays device-gated on
// the seeded UniFi OS Server, and — unlike firewall/zone — persists nothing.
//
// Measured 2026-07-24 across five adopted gateway models (UXGENT, UXGPRO at
// stock and UniFi-OS-style firmware, UXGB, and UXGENT again), each on its own
// fresh console: every one answers 404 api.err.BgpUnsupportedDevice with the
// gateway CONNECTED and flagged neither unsupported nor incompatible, and
// bgp/config stays empty afterwards.
//
// The gate is a device capability, not site state: ace.jar's BGP service
// (com/ubnt/service/bgp/oPvLNutRmdeIccrKkF) throws that error unless the device
// reports supportsUdapiRoutesBgp or supportsSwitchBgp, whose bits are
// UNIFI_UDAPI_CAP_ROUTES_BGP = 1<<22 and UNIFI_SWITCH_CAP_BGP = 1<<25 in
// com/ubnt/data/HiimPm. Whether an emulator can set them is UNRESOLVED: adding
// capability keys to the inform payload made the controller drop the caps it
// otherwise stores (fw_caps went from 3 to absent), so those attempts never
// landed and prove nothing either way.
func TestIntegrationSeededUOSBgpGate(t *testing.T) {
	if os.Getenv("UNIFI_GATEWAY_TEST") == "" {
		t.Skip("set UNIFI_GATEWAY_TEST=1 to run the seeded UOS BGP gate test")
	}
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	c := controllertest.StartUOSSeeded(ctx, t)
	s := c.NewSession(ctx, t)

	d := controllertest.AdoptGateway(ctx, t, c, s)
	t.Logf("gateway adopted on seeded UOS: mac=%s model=%q type=%q state=%d", d.MAC, d.Model, d.Type, d.State)

	body, status, err := s.PostJSON(ctx, "/v2/api/site/"+c.Site+"/bgp/config", map[string]any{
		"enabled":            true,
		"description":        "emu-probe",
		"frr_bgpd_config":    "router bgp 65000\n",
		"uploaded_file_name": "bgp.conf",
	})
	if err != nil {
		t.Fatalf("bgp POST transport error: %v", err)
	}
	t.Logf("%s POST bgp/config: HTTP %d body=%s", classify(status, body), status, jsonString(body))

	// Unlike firewall/zone, this one really does create nothing — check rather
	// than assume, since that assumption is exactly what hid the zone write.
	after, gstatus, gerr := s.GetJSON(ctx, "/v2/api/site/"+c.Site+"/bgp/config")
	if gerr != nil || gstatus != 200 {
		t.Fatalf("list bgp/config: status=%d err=%v", gstatus, gerr)
	}
	t.Logf("bgp/config collection after POST: %s", jsonString(after))
	if list, ok := after.([]any); ok && len(list) > 0 {
		t.Errorf("bgp/config persisted an object despite HTTP %d — the gate now writes like firewall/zone does, and BgpConfig drift is seedable: %s",
			status, jsonString(after))
	}
}
