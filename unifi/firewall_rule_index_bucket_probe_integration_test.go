//go:build integration

// unifi/firewall_rule_index_bucket_probe_integration_test.go
package unifi

import (
	"context"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationFirewallRuleIndexBuckets asks whether the controller
// actually enforces the fixed rule_index ranges terraform-provider-unifi
// hardcodes (2000-2999 LAN, 3000-3999 WAN, 4000-4999 GUEST, plus their
// 10x/20x high-range equivalents), or whether that convention lives only in
// the provider's own validator.
//
// rule_index's own extracted pattern (schemas/fields/FirewallRule.json,
// "2[0-9]{3,4}|4[0-9]{3,4}") admits 2000-29999 and 4000-49999 -- there is no
// "3[0-9]{3,4}" branch at all, so a 3000-3999 WAN bucket cannot be a
// controller-enforced range: the pattern refuses every value in it, for
// every ruleset. This probe confirms that refusal is real (not a stale
// extraction) and checks whether the *2000s vs 4000s* split the pattern
// does allow is actually tied to which ruleset a rule names, or is just a
// numeric range with no relationship to WAN_IN/LAN_IN/GUEST_IN at all.
func TestIntegrationFirewallRuleIndexBuckets(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 15*time.Minute)
	site := c.Site

	cases := []struct {
		name     string
		ruleset  string
		index    int64
		accepted bool
	}{
		// The claimed "3000-3999 is WAN" bucket: refused for WAN_IN itself,
		// at both digit lengths -- there is no controller-side WAN bucket.
		{"wan_in-3xxx", "WAN_IN", 3050, false},
		{"wan_in-3xxxx-high", "WAN_IN", 30001, false},

		// The 2xxx/4xxx split the pattern DOES allow is not tied to
		// ruleset: every ruleset accepts both.
		{"wan_in-2xxx", "WAN_IN", 2050, true},
		{"wan_in-4xxx", "WAN_IN", 4050, true},
		{"lan_in-2xxx", "LAN_IN", 2051, true},
		{"lan_in-4xxx", "LAN_IN", 4051, true},
		{"guest_in-4xxx", "GUEST_IN", 4052, true},
		{"guest_in-2xxx", "GUEST_IN", 2052, true},
		{"wan_in-2xxxx-high", "WAN_IN", 20001, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := createFirewallRuleRaw(ctx, s, site, tc.ruleset, tc.index)
			accepted := status/100 == 2
			if accepted {
				id := objectID(firstData(t, body))
				t.Cleanup(func() { s.DeleteJSON(ctx, "/api/s/"+site+"/rest/firewallrule/"+id) }) //nolint:errcheck
			}
			if accepted != tc.accepted {
				t.Fatalf("ruleset=%s rule_index=%d: got HTTP %d (%s), want accepted=%v -- "+
					"the rule_index/ruleset relationship has changed and this fact needs re-measuring",
					tc.ruleset, tc.index, status, v1ErrCode(body), tc.accepted)
			}
			t.Logf("MEASURED: ruleset=%s rule_index=%d -> accepted=%v as expected (HTTP %d)", tc.ruleset, tc.index, accepted, status)
		})
	}
}

func createFirewallRuleRaw(ctx context.Context, s *controllertest.Session, site, ruleset string, index int64) (int, any) {
	doc := map[string]any{
		"name": "probe-rule-index", "ruleset": ruleset,
		"rule_index": index, "action": "accept", "protocol": "all", "enabled": true,
		"src_firewallgroup_ids": []string{}, "dst_firewallgroup_ids": []string{},
	}
	body, status, err := s.PostJSON(ctx, "/api/s/"+site+"/rest/firewallrule", doc)
	if err != nil {
		return 0, map[string]any{"transport_error": err.Error()}
	}
	return status, body
}
