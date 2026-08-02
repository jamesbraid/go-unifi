//go:build integration

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	// This test writes a firewall zone, a DNS record, a WAN networkconf, a NAT
	// rule, a traffic route and an OSPF router. The last four are site
	// topology, not scratch objects, so it must never touch a controller
	// somebody uses. Every other mutating probe in this package carries the
	// same guard; this one predates the writes that made it necessary. CI
	// never sets UNIFI_TEST_URL for cmd/fields (the integration workflow
	// selects an image, not a URL), so nothing gated here loses coverage.
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating drift probe only runs against the disposable container")
	}

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
	//   FirewallPolicy       needs source/destination zone ids
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
	// ORDER: seedFirewallZone runs first and must stay first. It is the one
	// seed whose write is known to depend on its POST being the site's first
	// request to that collection (see its comment), and the WAN networkconf
	// the next seed creates is exactly the kind of site-state change that
	// could lazily materialize a default zone set. Nothing has measured that
	// it would — but ordering the seeds this way costs nothing and removes
	// the question.
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

// seedFirewallZone lands one firewall zone so FirewallZone.json has a live
// object to compare against.
//
// POST /v2/api/site/{site}/firewall/zone answers 404
// api.err.CouldNotFindHotspotFirewallZone and PERSISTS THE ZONE ANYWAY: the
// handler writes the object, then throws looking for a "hotspot" zone the site
// has never had. Every probe before 2026-07-24 classified that 404 as "gated,
// nothing created" because none of them read the collection back. The status is
// therefore not a result here — only the collection read below is.
//
// CALL THIS BEFORE ANYTHING READS THE ZONE COLLECTION, and do NOT add a
// "does one already exist?" pre-read to it. A GET on the collection before
// the first POST stops that POST persisting: measured 2026-07-25 on one
// standalone -sim boot across four sites, a POST issued as the site's first
// zone call landed immediately (2/2 sites) while a POST after a pre-read
// landed nothing — an earlier run retried twenty times over five minutes
// after a pre-read and never landed one. The v2Probes loop below GETs this
// collection, so seeding after it silently stops working and FirewallZone
// goes back to skipping, which reads like success.
//
// No adopted gateway is required, which the same run settled: all four sites
// were on a plain gateway-less controllertest.Start, so the pre-read is the
// whole story and the gateway never was. That is what lets this live in the
// base gate rather than behind UNIFI_GATEWAY_TEST.
//
// Only the first POST per site lands; a second answers the same 404 and
// creates nothing. One zone per site is all a drift comparison needs.
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
	t.Logf("seed firewall zone: POST answered HTTP %d (expected 404, the write lands anyway): %v", status, body)

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
	// Loud on purpose. Skipping here would hide both a controller change and
	// the far likelier cause: something started reading this collection before
	// the seed runs.
	t.Fatalf("seed firewall zone: %q absent after the POST. Either this controller "+
		"no longer persists the rejected write, or something read %s before the seed "+
		"did — a pre-read stops the write landing. Collection: %v",
		zoneName, path, zones)
}
