//go:build integration

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
// An adopted gateway is NOT required, settled 2026-07-25 after this test was
// written: four sites on one gateway-less standalone -sim boot separated the
// two factors, and post-first landed a zone (2/2) while pre-read-then-post
// landed none (1/1). The gateway this test adopts is incidental to the write;
// it stays because this test is also the seeded-UOS-with-hardware case, and
// the gateway-free path is now covered by the base drift gate
// (seedFirewallZone in drift_integration_test.go).
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

// TestIntegrationSeededUOSBgpConfig writes a BGP configuration against an
// adopted gateway and drift-checks what the controller serves back.
//
// bgp/config refused every write until unifi-emu v0.4.2. The controller gates
// it on a device capability rather than on site state: ace.jar's BGP service
// (com/ubnt/service/bgp/oPvLNutRmdeIccrKkF) throws api.err.BgpUnsupportedDevice
// unless the site's gateway reports supportsUdapiRoutesBgp, which is
// hasUdapiCapability(1<<22) against the udapi_caps bitmap on the device
// document (com/ubnt/data/HiimPm; the switch path reads switch_caps bit 1<<25
// and is not exercised here).
//
// Reporting udapi_caps alone was not enough, and the way it failed is worth
// keeping: the capability writer (com/ubnt/service/devmgr/TtfKyjBLlbFMCr) skips
// its whole update pass for a device on firmware >= 4.1.0 that reports no
// udapi_version, storing none of fw_caps, hw_caps, switch_caps or udapi_caps.
// A bitmap sent by itself was silently dropped, which read as payload
// rejection. The emulator now reports both per model profile, and only the
// models that carry the bit claim it.
//
// So this asserts the write lands, and then compares the persisted object
// against the hand-written schema — the same drift check the base gate runs
// for the collections it can seed. It stays behind UNIFI_GATEWAY_TEST because
// it needs an adopted gateway, which the base gate has no way to produce.
func TestIntegrationSeededUOSBgpConfig(t *testing.T) {
	if os.Getenv("UNIFI_GATEWAY_TEST") == "" {
		t.Skip("set UNIFI_GATEWAY_TEST=1 to run the seeded UOS BGP test")
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
	if status >= 300 {
		t.Fatalf("bgp/config POST: HTTP %d %s — the adopted %s no longer clears the BGP capability gate; "+
			"check that the emulator still reports udapi_caps AND udapi_version",
			status, jsonString(body), controllertest.GatewayModel)
	}
	t.Logf("bgp/config accepted: HTTP %d %s", status, jsonString(body))

	// The status is not the evidence — read the collection back, the habit
	// that uncovered the firewall/zone write in the first place.
	after, gstatus, gerr := s.GetJSON(ctx, "/v2/api/site/"+c.Site+"/bgp/config")
	if gerr != nil || gstatus != 200 {
		t.Fatalf("list bgp/config: status=%d err=%v", gstatus, gerr)
	}
	list, ok := after.([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("bgp/config answered HTTP %d but stored nothing: %s", status, jsonString(after))
	}
	t.Logf("bgp/config persisted: %s", jsonString(after))

	observed := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			observed = append(observed, m)
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(findModuleRoot(wd), "overrides", "resources", "BgpConfig.json"))
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("schema parse: %v", err)
	}

	r := driftCompare(observed, schema)
	if len(r.SchemaOnly) > 0 {
		t.Logf("schema-only fields (unset live, informational): %v", r.SchemaOnly)
	}
	if len(r.LiveOnly) > 0 {
		t.Errorf("live controller emits fields missing from BgpConfig.json: %v — update overrides/resources/BgpConfig.json",
			r.LiveOnly)
	}
}
