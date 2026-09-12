//go:build integration

package unifi

import (
	"fmt"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationFirewallProtocolStrictness measures how strict each firewall
// surface is about a protocol name, because the two published patterns differ
// and only one of them is ours.
//
// FirewallPolicy's pattern is assembled in overrides/resources: its names were
// measured against the controller, and joining ax.25 unescaped let the dot
// match anything, so the published pattern accepted a value the controller
// refuses. v1.111.0 escaped it.
//
// FirewallRule, DeviceQOSMatching and PortProfileQOSMatching carry the same
// name with the dot still unescaped, and that is NOT the same defect: those
// patterns come from the controller's own field definitions, and this v1
// surface really is that permissive. Escaping them would make the SDK stricter
// than the controller -- inventing a rule, which is the thing this project
// refuses to do everywhere else.
//
// So the asymmetry is the controller's, not a half-finished fix, and a
// consumer deriving both validators inherits two different behaviours from one
// nominal constraint. This pins that, so nobody "finishes the job" later and
// silently starts rejecting what v1 accepts.
func TestIntegrationFirewallProtocolStrictness(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 15*time.Minute)

	// v2 policies: strict. A name that only matches because a dot is a
	// wildcard is refused by name.
	policyPath := "/v2/api/site/" + c.Site + "/firewall-policies"
	src, dst := firewallZonePair(ctx, t, s, c.Site)
	if src == "" {
		t.Fatal("no firewall zones; the policy half cannot be measured and a skip here would " +
			"hide the strictness this test exists to pin")
	}

	n := 0
	postPolicy := func(protocol string) (int, map[string]any) {
		n++
		payload := firewallPolicyProbeBase(fmt.Sprintf("proto-strict-%d", n), 23200+n, src, dst)
		payload["protocol"] = protocol
		body, status, err := s.PostJSON(ctx, policyPath, payload)
		mustTransport(t, err)
		m, _ := body.(map[string]any)
		if status == 200 || status == 201 {
			if id, _ := m["_id"].(string); id != "" {
				s.DeleteJSON(ctx, policyPath+"/"+id) //nolint:errcheck
			}
		}
		return status, m
	}

	for _, wildcarded := range []string{"axX25", "ax:25"} {
		status, body := postPolicy(wildcarded)
		code, _ := body["code"].(string)
		if status == 200 || status == 201 {
			t.Errorf("policy accepted protocol %q; the escaped pattern in overrides is stricter "+
				"than the controller, which means v1.111.0 invented a rule", wildcarded)
			continue
		}
		if code != "api.err.InvalidFirewallPolicyProtocolName" {
			t.Errorf("policy refused %q with %q, wanted api.err.InvalidFirewallPolicyProtocolName; "+
				"a different refusal may be about something other than the name", wildcarded, code)
		}
	}
	if status, _ := postPolicy("tcp"); status != 200 && status != 201 {
		t.Errorf("policy refused tcp (HTTP %d); the control failed, so the refusals above "+
			"say nothing about protocol names", status)
	}

	// v1 rules: permissive. The unescaped dot in the controller's own pattern
	// is its real behaviour here, so the SDK publishing it unescaped is
	// correct and must stay that way.
	rulePath := "/api/s/" + c.Site + "/rest/firewallrule"
	for i, wildcarded := range []string{"axX25", "ax:25"} {
		body, status, err := s.PostJSON(ctx, rulePath, map[string]any{
			"name": fmt.Sprintf("proto-loose-%d", i), "ruleset": "LAN_IN",
			"rule_index": 2200 + i, "action": "accept", "protocol": wildcarded,
			"enabled": true, "src_firewallgroup_ids": []string{}, "dst_firewallgroup_ids": []string{},
		})
		mustTransport(t, err)
		if status != 200 {
			t.Errorf("rule refused protocol %q (HTTP %d). The controller got stricter: its "+
				"published pattern still has the dot unescaped, so the SDK now publishes a rule "+
				"looser than the controller. Re-measure before escaping it -- the fix is the "+
				"controller's definition, not ours.", wildcarded, status)
			continue
		}
		if d := firstData(t, body); d != nil {
			if id, _ := d["_id"].(string); id != "" {
				s.DeleteJSON(ctx, rulePath+"/"+id) //nolint:errcheck
			}
		}
	}
}
