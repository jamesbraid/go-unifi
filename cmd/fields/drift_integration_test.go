//go:build integration

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// v2Probes maps each hand-written schema in overrides/resources/ to the
// live endpoint that serves it.
var v2Probes = []struct {
	schemaFile string
	path       string
}{
	{"FirewallZone.json", "/v2/api/site/%s/firewall/zone"},
	{"FirewallPolicy.json", "/v2/api/site/%s/firewall-policies"},
	{"TrafficRoute.json", "/v2/api/site/%s/trafficroutes"},
	{"Nat.json", "/v2/api/site/%s/nat"},
	{"DnsRecord.json", "/v2/api/site/%s/static-dns"},
	{"OSPFRouter.json", "/v2/api/site/%s/ospf/router"},
	{"BgpConfig.json", "/v2/api/site/%s/bgp/config"},
	{"ApGroups.json", "/v2/api/site/%s/apgroups"},
	{"NetworkMembersGroup.json", "/v2/api/site/%s/network-members-groups"},
}

// TestIntegrationV2Drift compares the hand-written v2 schemas against what a
// live controller actually serves. LiveOnly fields fail: they are upstream
// drift our definitions are missing. SchemaOnly fields only log: absent
// wire fields are normal for unset options.
func TestIntegrationV2Drift(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	// A fresh simulation controller starts with every v2 collection empty, so
	// the drift subtests below skip on "no live objects to compare" unless the
	// collection is seeded. ApGroups is pre-seeded by the controller, so it
	// compares as-is; static-dns and firewall/zone are seeded here. A failed
	// seed fails the test rather than silently skipping — a silent skip is
	// indistinguishable from a pass and is exactly how the zone write below
	// stayed hidden for so long.
	//
	// The remaining six are not seeded. TestIntegrationGatewayFeatureGate
	// re-probed them 2026-07-24, once with an emulated UXGENT gateway adopted
	// and once with no gateway at all; the two runs answer identically, so
	// adopted gateway hardware is not what any of this turned on:
	//   BgpConfig            needs an adopted gateway that claims the BGP
	//                        capability bit, which no bare harness has --
	//                        covered by TestIntegrationSeededUOSBgpConfig
	//   FirewallPolicy       needs source/destination zone ids. Running the
	//                        zone migration below is NOT enough on its own:
	//                        measured 2026-08-01, firewall-policies is still
	//                        [] afterwards, because the rule-conversion step
	//                        has no firewall or traffic rules to convert on a
	//                        fresh site.
	//   Nat, TrafficRoute    seedable -- they need a WAN networkconf, which the
	//                        demo site lacks and adoption does not create
	//   OSPFRouter           seedable -- needs a non-empty areas list and a
	//                        lowercase area_type ("normal", not "NORMAL")
	//   NetworkMembersGroup  405 -- the v2 collection is not POST-writable here
	// Promoting the three seedable schemas into this gate is a follow-up.
	migrateZoneBasedFirewall(ctx, t, c, s)
	seedFirewallZone(ctx, t, c, s)

	seed := map[string]any{
		"enabled":     true,
		"key":         "probe.example.com",
		"record_type": "A",
		"value":       "192.0.2.1",
		"ttl":         3600,
	}
	seedBody, seedStatus, seedErr := s.PostJSON(ctx, fmt.Sprintf("/v2/api/site/%s/static-dns", c.Site), seed)
	if seedErr != nil {
		t.Fatalf("seed DNS record: status=%d err=%v", seedStatus, seedErr)
	}
	if seedStatus >= 300 {
		t.Fatalf("seed DNS record: status=%d body=%v", seedStatus, seedBody)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	resourcesDir := filepath.Join(findModuleRoot(wd), "overrides", "resources")

	for _, probe := range v2Probes {
		t.Run(probe.schemaFile, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(resourcesDir, probe.schemaFile))
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			var schema map[string]any
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatalf("schema parse: %v", err)
			}

			body, status, err := s.GetJSON(ctx, fmt.Sprintf(probe.path, c.Site))
			if status == 404 {
				t.Skipf("endpoint absent on this controller version (404)")
			}
			if errors.Is(err, controllertest.ErrNotJSON) {
				t.Fatalf("probe returned HTTP %d with a non-JSON body — controller not serving the API?", status)
			}
			if err != nil {
				t.Fatalf("probe: %v", err)
			}
			if status != 200 {
				t.Fatalf("probe status = %d", status)
			}
			// body == nil here is a legal JSON `null` response (an empty
			// v2 collection some controllers serve that way); it falls
			// through to the observed-count check below and is skipped
			// there like any other empty collection.

			var observed []map[string]any
			switch v := body.(type) {
			case []any:
				for _, item := range v {
					if m, ok := item.(map[string]any); ok {
						observed = append(observed, m)
					}
				}
			case map[string]any:
				observed = append(observed, v)
			}
			if len(observed) == 0 {
				t.Skipf("no live objects to compare (empty collection)")
			}

			r := driftCompare(observed, schema)
			if len(r.SchemaOnly) > 0 {
				t.Logf("schema-only fields (unset live, informational): %v", r.SchemaOnly)
			}
			if len(r.LiveOnly) > 0 {
				t.Errorf("live controller emits fields missing from %s: %v — update overrides/resources/%s",
					probe.schemaFile, r.LiveOnly, probe.schemaFile)
			}
		})
	}
}

// migrateZoneBasedFirewall switches the site to zone-based firewalling, which
// is the only thing that creates the default firewall zones — and, as a
// consequence, the only thing that makes the zone collection writable at all.
//
// Until a site is migrated, every zone write fails on a lookup of the site's
// "hotspot" zone, which does not exist yet:
// com/ubnt/service/firewall/policy/c/c/YPBkdOLFaXsxg resolves it through
// zoneDao.findOneBySiteIdAndZoneKey(siteId, HOTSPOT) and throws
// api.err.CouldNotFindHotspotFirewallZone when it comes back empty. The call
// ordering inside that class is what produced the behaviour this helper used to
// depend on: CREATE saves the document at pc 5 and only looks the hotspot zone
// up at pc 28, so a rejected POST left the zone behind, while UPDATE and DELETE
// look it up first and therefore wrote nothing at all.
//
// Seeding through that ordering quirk worked but was miserable: it depended on
// the POST being the site's first-ever zone call (a plain GET beforehand stopped
// the write landing), it could only ever create — never update — and it hung the
// whole FirewallZone drift check off a controller bug that any patch release
// could fix, in which case the check would go back to skipping and a skip reads
// exactly like a pass.
//
// POST /v2/api/site/{site}/firewall/migrate is the supported route to the same
// place. It runs the ZONE_BASED_FIREWALL site-feature migration, whose first
// step creates the six default zones from a hardcoded key list; the same
// migration then converts any existing firewall and traffic rules into firewall
// policies. Measured against 10.4.57-sim, gateway-less, 2026-08-01:
//
//   - the POST answers 204 with an empty body, so Session.do reports
//     ErrNotJSON — check the status, not the error
//   - the zone collection goes from [] to exactly six zones: Internal,
//     External, Gateway, Vpn, Hotspot, Dmz (that capitalisation is literally
//     StringUtils.capitalize over the zone keys), all default_zone=true, with
//     attr_no_edit=true on External, Gateway and Vpn
//   - Internal comes back already holding the site's LAN in network_ids
//   - a subsequent zone POST answers 201 instead of 404, and a PUT answers 200
//     and the change is there on read-back
//   - it is idempotent: a second call is a no-op, still 204, no duplicate zones
//   - reading the collection first does NOT break any of this, unlike the
//     ordering quirk it replaces
//
// No adopted gateway is needed. A gateway only changes anything if one is
// present AND has SSL inspection enabled, in which case the migration is
// deferred to an async task instead of running inline; gateway-less always
// takes the synchronous branch, which is why this can live in the base gate.
//
// One way this can look like it worked without working: the migration service is
// conditional on the controller's DB mode, and a postgresql-mode controller
// wires a no-op implementation that still answers 204. The zone read below is
// what actually settles it.
func migrateZoneBasedFirewall(ctx context.Context, t *testing.T, c *controllertest.Controller, s *controllertest.Session) {
	t.Helper()

	_, status, err := s.PostJSON(ctx, fmt.Sprintf("/v2/api/site/%s/firewall/migrate", c.Site), nil)
	// A 204 carries no body, which Session.do reports as ErrNotJSON. That is
	// the success shape here, so only a genuine transport failure (status 0)
	// or a non-2xx counts as an error.
	if status == 0 {
		t.Fatalf("zone-based firewall migration: transport error: %v", err)
	}
	if status >= 300 {
		t.Fatalf("zone-based firewall migration: POST returned HTTP %d (want 204)", status)
	}
	t.Logf("zone-based firewall migration: POST answered HTTP %d", status)

	// The status is not the evidence — read the collection back.
	zones, gstatus, gerr := s.GetJSON(ctx, fmt.Sprintf("/v2/api/site/%s/firewall/zone", c.Site))
	if gerr != nil || gstatus != 200 {
		t.Fatalf("zone-based firewall migration: list zones: status=%d err=%v", gstatus, gerr)
	}
	list, ok := zones.([]any)
	if !ok {
		t.Fatalf("zone-based firewall migration: collection is not a list: %v", zones)
	}

	// Assert on zone_key, not on name: the keys are what the migration is
	// defined in terms of, and they survive a user renaming a zone.
	want := map[string]bool{"internal": false, "external": false, "gateway": false, "vpn": false, "hotspot": false, "dmz": false}
	for _, item := range list {
		z, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if k, _ := z["zone_key"].(string); k != "" {
			if _, expected := want[k]; expected {
				want[k] = true
			}
		}
	}
	var missing []string
	for k, seen := range want {
		if !seen {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		// Loud on purpose: a silent skip here is indistinguishable from a pass.
		t.Fatalf("zone-based firewall migration: POST answered HTTP %d but the site has no %v zone(s). "+
			"Either the migration no longer creates the defaults, or this controller wired the no-op "+
			"migration service (non-mongo DB mode), in which case it answers 204 and does nothing. "+
			"Collection: %v", status, missing, zones)
	}
	t.Logf("zone-based firewall migration: site has all six default zones")
}

// seedFirewallZone lands one non-default firewall zone so the FirewallZone
// drift comparison sees a user-created object alongside the six defaults the
// migration creates.
//
// This must run after migrateZoneBasedFirewall: on an unmigrated site the POST
// answers 404 api.err.CouldNotFindHotspotFirewallZone (see that function for
// why). On a migrated site it answers a clean 201.
func seedFirewallZone(ctx context.Context, t *testing.T, c *controllertest.Controller, s *controllertest.Session) {
	t.Helper()

	const zoneName = "drift-probe-zone"
	path := fmt.Sprintf("/v2/api/site/%s/firewall/zone", c.Site)

	body, status, err := s.PostJSON(ctx, path, map[string]any{
		"name":        zoneName,
		"network_ids": []string{},
	})
	if err != nil {
		t.Fatalf("seed firewall zone: transport error: %v", err)
	}
	if status >= 300 {
		t.Fatalf("seed firewall zone: POST returned HTTP %d (want 201): %v — "+
			"a 404 CouldNotFindHotspotFirewallZone here means the site was not migrated first", status, body)
	}

	// The status is not the evidence — read the collection back.
	zones, status, err := s.GetJSON(ctx, path)
	if err != nil || status != 200 {
		t.Fatalf("seed firewall zone: list back: status=%d err=%v", status, err)
	}
	list, ok := zones.([]any)
	if !ok {
		t.Fatalf("seed firewall zone: collection is not a list: %v", zones)
	}
	for _, item := range list {
		z, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if n, _ := z["name"].(string); n == zoneName {
			return
		}
	}
	t.Fatalf("seed firewall zone: %q absent after a successful POST — the write did not persist. Collection: %v",
		zoneName, zones)
}
