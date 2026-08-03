//go:build integration

// unifi/traffic_route_matching_integration_test.go
package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// trafficRouteMatchCase is one traffic route carrying some combination of
// destination filters, and what the controller did with it.
type trafficRouteMatchCase struct {
	name    string
	target  string // matching_target: DOMAIN, IP, INTERNET, or unset
	domains bool
	ips     bool
	regions bool
	want    string // measured api.err.* code, or "accepted"
	// wantBoth asserts that a route carrying two filter kinds keeps both,
	// which is what says the controller treats matching_target as a selector
	// rather than the filters as alternatives.
	wantBoth bool
}

// networkTrafficRouteMatchingRules records what the controller does when a
// traffic route carries more than one kind of destination filter.
//
// terraform-provider-unifi declares domain, region and ip mutually exclusive
// with ConflictsWith. The wire shape suggests otherwise: matching_target is
// its own field taking DOMAIN|IP|INTERNET, which reads like a selector -- the
// same shape as stormctrl_type, where the equivalent provider-side conflict
// turned out to be stricter than the controller.
//
// Note there is no REGION in matching_target, yet regions is a first-class
// list on the object. What selects it is part of what this measures.
var networkTrafficRouteMatchingRules = []trafficRouteMatchCase{
	{name: "domain only", target: "DOMAIN", domains: true, want: "accepted"},
	{name: "ip only", target: "IP", ips: true, want: "accepted"},
	{name: "internet only", target: "INTERNET", want: "accepted"},

	// The rows that matter: two filter kinds at once, accepted either way
	// round, with both kept. matching_target selects which one is applied;
	// it does not make the others illegal.
	{name: "domain target with ip filter too", target: "DOMAIN", domains: true, ips: true, want: "accepted", wantBoth: true},
	{name: "ip target with domain filter too", target: "IP", domains: true, ips: true, want: "accepted", wantBoth: true},

	// regions rides alongside a filter rather than replacing one -- there is
	// no REGION in matching_target, and DOMAIN without domains is rejected
	// however many regions are supplied.
	{name: "regions with domain target", target: "DOMAIN", domains: true, regions: true, want: "accepted"},
	{name: "regions alone", target: "DOMAIN", regions: true, want: "api.err.MissingDomain"},
}

// TestIntegrationTrafficRouteMatchingRules measures each combination.
func TestIntegrationTrafficRouteMatchingRules(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	// A traffic route routes one network's traffic via another, so it needs
	// both to exist. The demo site ships a LAN but no WAN.
	wanID := seedTrafficRouteWAN(ctx, t, s, c.Site)
	lanID := firstNetworkIDForPurpose(ctx, t, s, c.Site, PurposeCorporate)

	for i, tc := range networkTrafficRouteMatchingRules {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{
				"description":         fmt.Sprintf("tr-match-%d", i),
				"enabled":             true,
				"network_id":          wanID,
				"kill_switch_enabled": false,
				"target_devices":      []any{map[string]any{"type": "NETWORK", "network_id": lanID}},
			}
			// The controller wants all four filter lists present, empty when
			// unused: omitting them is api.err.InvalidObject regardless of
			// matching_target.
			domains, ips, regions := []any{}, []any{}, []any{}
			if tc.domains {
				domains = []any{map[string]any{"domain": "example.com"}}
			}
			if tc.ips {
				ips = []any{map[string]any{"ip_or_subnet": "192.0.2.0/24", "ip_version": "v4"}}
			}
			if tc.regions {
				regions = []any{"US"}
			}
			payload["domains"] = domains
			payload["ip_addresses"] = ips
			payload["ip_ranges"] = []any{}
			payload["regions"] = regions
			if tc.target != "" {
				payload["matching_target"] = tc.target
			}

			body, status, err := s.PostJSON(ctx, "/v2/api/site/"+c.Site+"/trafficroutes", payload)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}

			got := "accepted"
			var created map[string]any
			// v2 answers 201 Created, not the v1 200.
			if status != 200 && status != 201 {
				got = v2ErrCode(body)
			} else {
				created, _ = body.(map[string]any)
				if id, _ := created["_id"].(string); id != "" {
					defer s.DeleteJSON(ctx, "/v2/api/site/"+c.Site+"/trafficroutes/"+id) //nolint:errcheck
				}
			}

			raw, _ := json.Marshal(created)
			t.Logf("MEASURED %-32s -> %-40s %s", tc.name, got, raw)

			if tc.want == "" {
				t.Logf("UNPINNED %s -> %s", tc.name, got)
				return
			}
			if got != tc.want {
				t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
			}
			if tc.wantBoth && created != nil {
				domains, _ := created["domains"].([]any)
				ips, _ := created["ip_addresses"].([]any)
				if len(domains) == 0 || len(ips) == 0 {
					t.Errorf("sent both filter kinds, controller stored domains=%d ip_addresses=%d; one was discarded",
						len(domains), len(ips))
				}
			}
		})
	}
}

// seedTrafficRouteWAN creates the WAN a traffic route routes via. The demo
// site has none, and adoption does not create one.
func seedTrafficRouteWAN(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()

	body, status, err := s.PostJSON(ctx, "/api/s/"+site+"/rest/networkconf", map[string]any{
		"name": "tr-probe-wan", "purpose": PurposeWAN, "enabled": true,
		"wan_networkgroup": "WAN", "wan_type": "dhcp",
		"wan_load_balance_type": "failover-only", "report_wan_event": false,
	})
	if err != nil || status != 200 {
		t.Fatalf("seed WAN: status %d, %v, body %v", status, err, body)
	}
	created := firstData(t, body)
	id, _ := created["_id"].(string)
	if id == "" {
		t.Fatalf("seeded WAN has no id")
	}
	t.Cleanup(func() {
		s.DeleteJSON(context.WithoutCancel(ctx), "/api/s/"+site+"/rest/networkconf/"+id) //nolint:errcheck
	})
	return id
}

// firstNetworkIDForPurpose returns an existing network of the given purpose.
func firstNetworkIDForPurpose(ctx context.Context, t *testing.T, s *controllertest.Session, site, purpose string) string {
	t.Helper()

	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/rest/networkconf")
	if err != nil || status != 200 {
		t.Fatalf("list networkconf: status %d, %v", status, err)
	}
	m, _ := body.(map[string]any)
	items, _ := m["data"].([]any)
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok && obj["purpose"] == purpose {
			if id, _ := obj["_id"].(string); id != "" {
				return id
			}
		}
	}
	t.Fatalf("no %s network on this site", purpose)
	return ""
}

// v2ErrCode pulls the error code out of a v2 envelope, which uses
// {"code","message"} rather than the v1 {"meta":{"msg"}}.
func v2ErrCode(body any) string {
	m, ok := body.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", body)
	}
	if code, _ := m["code"].(string); code != "" {
		return code
	}
	return errCode(body)
}
