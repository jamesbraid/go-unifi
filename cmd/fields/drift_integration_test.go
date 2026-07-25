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
	// collection is seeded. static-dns is the one collection a gateway-less sim
	// accepts a POST into, so seed one DNS record to exercise DnsRecord.json (a
	// failed seed fails the test rather than silently skipping). ApGroups is
	// pre-seeded by the controller, so it compares too.
	//
	// The other seven are not seeded here. TestIntegrationGatewayFeatureGate
	// re-probed all of them 2026-07-24, once with an emulated UXGENT gateway
	// adopted and once with no gateway at all; the two runs answer identically,
	// so adopted gateway hardware is not what any of this turned on:
	//   BgpConfig            404 api.err.BgpUnsupportedDevice, gateway or not
	//   FirewallZone         404 api.err.CouldNotFindHotspotFirewallZone, gateway or not
	//   FirewallPolicy       needs source/destination zone ids (blocked on FirewallZone)
	//   Nat, TrafficRoute    seedable -- they need a WAN networkconf, which the
	//                        demo site lacks and adoption does not create
	//   OSPFRouter           seedable -- needs a non-empty areas list and a
	//                        lowercase area_type ("normal", not "NORMAL")
	//   NetworkMembersGroup  405 -- the v2 collection is not POST-writable here
	// Promoting the three seedable schemas into this gate is a follow-up; until
	// then the drift gate covers static-dns and apgroups.
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
