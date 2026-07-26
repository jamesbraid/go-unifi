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
	//   BgpConfig            404 api.err.BgpUnsupportedDevice, gateway or not
	//   FirewallPolicy       needs source/destination zone ids
	//   Nat, TrafficRoute    seedable -- they need a WAN networkconf, which the
	//                        demo site lacks and adoption does not create
	//   OSPFRouter           seedable -- needs a non-empty areas list and a
	//                        lowercase area_type ("normal", not "NORMAL")
	//   NetworkMembersGroup  405 -- the v2 collection is not POST-writable here
	// Promoting the three seedable schemas into this gate is a follow-up.
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
