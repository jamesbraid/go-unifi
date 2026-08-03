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

// TestIntegrationSeededUOSZoneMigration runs the zone-based-firewall migration
// on the seeded UniFi OS Server image with a gateway adopted — the highest
// fidelity target available here — and checks the two things the base drift
// gate cannot.
//
// migrateZoneBasedFirewall (drift_integration_test.go) documents the migration
// itself; that runs gateway-less on the standalone sim. This test covers what
// hardware changes:
//
//   - The migration still runs INLINE. A gateway is the one thing that can move
//     it off the synchronous path: when a gateway is present AND has SSL
//     inspection enabled, the controller defers the migration to an async task
//     instead. The adopted UXGENT does not trigger that, measured 2026-08-01 —
//     all six default zones were already there on the first read after the POST
//     returned. If a future emulator profile turns SSL inspection on, this is
//     the test that will catch the zones arriving late or not at all.
//
//   - The migration creates the predefined FIREWALL POLICIES, and that part
//     genuinely does need a gateway. Measured 2026-08-01 across three cells:
//     sim gateway-less gives 6 zones and 0 policies; sim with an adopted UXGENT
//     gives 6 zones and 10 policies; seeded UOS with an adopted UXGENT gives
//     the same 6 and 10. So the gateway is the variable, not the UOS image, and
//     FirewallPolicy can never be seeded from the base gate.
//
// The 10 policies are all predefined=true (two per IP version of "Block All
// Other Traffic", plus established/related, invalid-traffic and IPv6 neighbour
// discovery pairs). They are NOT conversions of user rules — a fresh site has
// none to convert. Their fields already match overrides/resources/
// FirewallPolicy.json exactly (zero live-only fields, 2026-08-01), so promoting
// FirewallPolicy into a gateway-gated drift comparison is a clean follow-up.
//
// This test replaced one that proved a controller bug instead: before the
// migration endpoint was found, the only way to get a zone onto a site was that
// a rejected POST persisted its document before throwing. See
// migrateZoneBasedFirewall for why that route is gone.
//
// Gated behind UNIFI_GATEWAY_TEST: the seeded UOS is a systemd boot of minutes.
func TestIntegrationSeededUOSZoneMigration(t *testing.T) {
	if os.Getenv("UNIFI_GATEWAY_TEST") == "" {
		t.Skip("set UNIFI_GATEWAY_TEST=1 to run the seeded UOS zone migration test")
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

	d := controllertest.AdoptGateway(ctx, t, c, s)
	t.Logf("gateway adopted: mac=%s model=%q type=%q state=%d", d.MAC, d.Model, d.Type, d.State)

	// Creates the six default zones and asserts they are actually there.
	migrateZoneBasedFirewall(ctx, t, c, s)

	// With the site migrated, zone writes work normally. Prove both directions
	// the pre-migration controller refused: a clean create, and an update that
	// actually applies.
	const zoneName = "probe-seeded-zone"
	created, status, err := s.PostJSON(ctx, "/v2/api/site/"+c.Site+"/firewall/zone", map[string]any{
		"name":        zoneName,
		"network_ids": []string{},
	})
	if err != nil {
		t.Fatalf("zone POST transport error: %v", err)
	}
	if status >= 300 {
		t.Fatalf("zone POST on a migrated site: HTTP %d %s (want 201)", status, jsonString(created))
	}
	zone, ok := created.(map[string]any)
	if !ok {
		t.Fatalf("zone POST returned no object: %s", jsonString(created))
	}
	zoneID, _ := zone["_id"].(string)
	if zoneID == "" {
		t.Fatalf("zone POST returned no _id: %s", jsonString(created))
	}
	t.Logf("zone created: HTTP %d %s", status, jsonString(created))

	const renamed = "probe-seeded-zone-renamed"
	upBody, upStatus, upErr := s.PutJSON(ctx, "/v2/api/site/"+c.Site+"/firewall/zone/"+zoneID, map[string]any{
		"_id":         zoneID,
		"name":        renamed,
		"network_ids": []string{},
	})
	if upErr != nil {
		t.Fatalf("zone PUT transport error: %v", upErr)
	}
	if upStatus >= 300 {
		t.Fatalf("zone PUT on a migrated site: HTTP %d %s (want 200)", upStatus, jsonString(upBody))
	}

	// The status is not the evidence — read the collection back.
	zones, gstatus, gerr := s.GetJSON(ctx, "/v2/api/site/"+c.Site+"/firewall/zone")
	if gerr != nil || gstatus != 200 {
		t.Fatalf("list zones after update: status=%d err=%v", gstatus, gerr)
	}
	list, ok := zones.([]any)
	if !ok {
		t.Fatalf("zone collection is not a list: %s", jsonString(zones))
	}
	var found map[string]any
	for _, item := range list {
		z, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := z["_id"].(string); id == zoneID {
			found = z
			break
		}
	}
	if found == nil {
		t.Fatalf("zone %s absent after a successful PUT: %s", zoneID, jsonString(zones))
	}
	if n, _ := found["name"].(string); n != renamed {
		t.Fatalf("zone PUT answered HTTP %d but the name is still %q, want %q — the update did not apply",
			upStatus, n, renamed)
	}
	t.Logf("zone update applied: %s", jsonString(found))

	// The gateway-only half: the migration should have created the predefined
	// firewall policies. This is the collection the base gate can never seed.
	policies, pstatus, perr := s.GetJSON(ctx, "/v2/api/site/"+c.Site+"/firewall-policies")
	if perr != nil || pstatus != 200 {
		t.Fatalf("list firewall-policies: status=%d err=%v", pstatus, perr)
	}
	plist, ok := policies.([]any)
	if !ok || len(plist) == 0 {
		t.Fatalf("firewall-policies is empty after migrating a site with an adopted %s. "+
			"The migration created the zones but no policies, so either the predefined policy set moved "+
			"behind another gate or the migration was deferred to the async task: %s",
			controllertest.GatewayModel, jsonString(policies))
	}
	t.Logf("migration created %d firewall policies", len(plist))
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
