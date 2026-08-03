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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	// A fresh simulation controller starts with every v2 collection empty, so
	// the drift subtests below skip on "no live objects to compare" unless the
	// collection is seeded. ApGroups is pre-seeded by the controller, so it
	// compares as-is; the rest are seeded here. A failed seed fails the test
	// rather than silently skipping — a silent skip is indistinguishable from
	// a pass and is exactly how the zone write below stayed hidden for so
	// long.
	//
	// Three schemas stay unseeded, all for reasons that are not about the
	// payload. TestIntegrationGatewayFeatureGate re-probed the set once with
	// an emulated UXGENT gateway adopted and once with no gateway at all:
	//   BgpConfig            needs an adopted gateway that claims the BGP
	//                        capability bit, which no bare harness has --
	//                        covered by TestIntegrationSeededUOSBgpConfig
	//   FirewallPolicy       needs source/destination zone ids. Running the
	//                        zone migration below is NOT enough on its own:
	//                        measured 2026-08-01, firewall-policies is still
	//                        [] afterwards, because the rule-conversion step
	//                        has no firewall or traffic rules to convert on a
	//                        fresh site.
	//   NetworkMembersGroup  405 -- the v2 collection is not POST-writable here
	//
	// Nat, TrafficRoute and OSPFRouter used to sit in that list too. What
	// blocked them was never gateway hardware — the gateway-adopted and
	// gateway-less sweeps answer 201 on all three alike. It was site state
	// and payload shape: the demo site has no WAN networkconf and adoption
	// does not create one, ospf/router rejects an empty areas list, and its
	// area_type is a lowercase enum ("normal", not "NORMAL"). Supply those
	// and the writes land on a bare controller, which is what
	// seedGatewayV2Collections below does.
	//
	// None of it runs against a UNIFI_TEST_URL target. The comparison itself
	// only reads, so it is worth pointing at a real controller — a populated
	// site is the best drift subject there is — but the seeding is not. The
	// zone migration in particular switches the site to zone-based
	// firewalling and converts its existing firewall and traffic rules into
	// policies, permanently and site-wide. That is fine on a container we
	// throw away and unacceptable on someone's network, which is why every
	// other mutating probe in the suite skips URL targets too.
	if seedable(t) {
		migrateZoneBasedFirewall(ctx, t, c, s)
		seedFirewallZone(ctx, t, c, s)
		seedGatewayV2Collections(ctx, t, c, s)

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

			observed := observedObjects(body)
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

// seedGatewayV2Collections lands one NAT rule, one traffic route and one OSPF
// router so Nat.json, TrafficRoute.json and OSPFRouter.json have live objects
// to compare against.
//
// It reuses gatewayProbes, the same payload table TestIntegrationGatewayFeatureGate
// sweeps with, so the seed here and the probe there cannot drift apart: those
// payloads are schema-complete on purpose, and a payload that loses a required
// field draws a 400 from validation rather than creating anything. The other
// two entries in that table are skipped — firewall-zone is seeded above by a
// route of its own, and bgp/config needs a gateway advertising the BGP
// capability bit, which no bare harness has.
//
// Like seedFirewallZone, this issues each POST without first asking whether an
// object already exists. A pre-read is the one construct measured to stop a v2
// write landing on this controller; there is no reason to think these three
// share the zone's pathology (they answer a real 201 rather than writing and
// then throwing a 404), but the check would buy nothing and the hazard is
// real, so don't add one.
func seedGatewayV2Collections(ctx context.Context, t *testing.T, c *controllertest.Controller, s *controllertest.Session) {
	t.Helper()

	lanID, wanID := ensureWAN(ctx, t, s, c.Site)
	// ensureWAN fatals on a missing WAN but returns the corporate id without
	// checking it. Empty is a quietly bad payload rather than an error: the
	// OSPF area would carry network_ids [""] and the traffic route a target
	// device pointing at nothing, either of which may well be accepted and
	// mean nothing.
	if lanID == "" {
		t.Fatalf("site %s has no corporate networkconf: the OSPF area and the traffic route target both need one", c.Site)
	}
	t.Logf("seed v2 collections: network ids lan=%q wan=%q", lanID, wanID)

	// How to recognize our own object in the collection afterwards. nat and
	// trafficroutes carry the probe description; ospf/router is a singleton
	// with no description, so its router id is the marker.
	markers := map[string]struct{ key, value string }{
		"nat":           {"description", "emu-probe"},
		"traffic-route": {"description", "emu-probe"},
		"ospf":          {"router_id", "0.0.0.1"},
	}

	seeded := map[string]bool{}
	for _, p := range gatewayProbes(lanID, wanID) {
		marker, seedable := markers[p.name]
		if !seedable {
			continue
		}
		seeded[p.name] = true

		path := "/v2/api/site/" + c.Site + "/" + p.path
		body, postStatus, err := s.PostJSON(ctx, path, p.payload)
		if err != nil {
			t.Fatalf("seed %s: POST %s transport error: status=%d err=%v", p.name, path, postStatus, err)
		}
		if postStatus >= 300 {
			t.Fatalf("seed %s: POST %s answered HTTP %d: %s", p.name, path, postStatus, jsonString(body))
		}

		// The status is not the result. The firewall zone above answers 404
		// and writes anyway; nothing rules out a handler doing the reverse.
		// Only this read decides whether the seed worked. Keep the two status
		// codes in separate variables: reusing one makes a later log read
		// "POST answered HTTP 200" while reporting the GET, which is the same
		// confusion between a status and a result that this whole seed exists
		// to avoid.
		got, getStatus, err := s.GetJSON(ctx, path)
		if err != nil || getStatus != 200 {
			t.Fatalf("seed %s: list back %s: status=%d err=%v", p.name, path, getStatus, err)
		}
		observed := observedObjects(got)
		found := false
		for _, o := range observed {
			if v, _ := o[marker.key].(string); v == marker.value {
				found = true
				break
			}
		}
		if !found {
			// Loud on purpose: a seed that quietly does nothing turns the
			// subtest below into "no live objects to compare", which is a
			// skip, which reads like a pass.
			t.Fatalf("seed %s: POST %s answered HTTP %d but no object with %s=%q is in the collection afterwards. "+
				"First suspect: something read %s before this seed ran — a pre-read is what stops the firewall "+
				"zone write landing and these three have never been proven immune. Second: the controller changed "+
				"the payload it accepts. Collection: %s",
				p.name, path, postStatus, marker.key, marker.value, path, jsonString(observed))
		}
		t.Logf("seed %s: POST %s answered HTTP %d, read back HTTP %d with %d object(s)",
			p.name, path, postStatus, getStatus, len(observed))
	}

	// Every marker must have matched a probe. The loop above selects by name,
	// so renaming an entry in gatewayProbes would silently seed nothing here
	// and send the matching drift subtest back to "no live objects to
	// compare" — a skip, which reads like a pass. Catch the rename instead.
	for name := range markers {
		if !seeded[name] {
			t.Fatalf("seed: no gatewayProbes entry named %q, so its collection was never seeded. "+
				"A probe was probably renamed; the drift subtest for it would skip and look like a pass.", name)
		}
	}
}

// seedable reports whether this run may write to the controller it is
// pointed at.
//
// Only a controller the harness booted itself qualifies. UNIFI_TEST_URL
// targets an existing one, and nothing about that variable says whether the
// thing on the other end is another throwaway container or a production
// site; the suite's convention is to assume the latter and leave it alone.
func seedable(t *testing.T) bool {
	t.Helper()

	if os.Getenv("UNIFI_TEST_URL") == "" {
		return true
	}
	// Said once, up front, so the per-schema "no live objects" skips below
	// are attributable rather than mysterious.
	t.Log("UNIFI_TEST_URL is set, so this run seeds nothing and compares the site as it stands. " +
		"Schemas whose collections are empty here will skip. The CI gate runs against a disposable " +
		"container, seeds, and fails on an empty collection.")
	return false
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
