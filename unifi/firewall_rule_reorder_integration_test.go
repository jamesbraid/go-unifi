//go:build integration

package unifi

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationReorderFirewallRules drives the reorder through the public
// method against a live controller, and pins the two facts that decide the
// method's shape.
//
// The first is that the command works and is reached at api/s/<site>/cmd/
// firewall. The path used until 2026-08 answered 404, so this is also the
// first proof the call reaches the controller at all.
//
// The second is that the command is not redundant. Writing rule_index
// through the ordinary REST collection cannot express a reorder: the
// controller rejects the first write whose index another rule still holds
// (api.err.FirewallRuleIndexExisted), and any permutation has such a step.
// That is why the client keeps a command for this rather than a masked
// update per rule.
func TestIntegrationReorderFirewallRules(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)
	client := harnessClient(ctx, t, c)
	base := "/api/s/" + c.Site + "/rest/firewallrule"

	// Three rules in one ruleset, in a known order.
	var ids []string
	for i, index := range []int64{2001, 2002, 2003} {
		body, status, err := s.PostJSON(ctx, base, map[string]any{
			"name": fmt.Sprintf("reorder-probe-%d", i), "ruleset": "LAN_IN",
			"rule_index": index, "action": "accept", "protocol": "all", "enabled": true,
			"src_firewallgroup_ids": []string{}, "dst_firewallgroup_ids": []string{},
		})
		if err != nil || status != 200 {
			t.Fatalf("seeding rule %d failed (HTTP %d): %v", i, status, body)
		}
		id, _ := firstData(t, body)["_id"].(string)
		if id == "" {
			t.Fatalf("seeded rule %d has no id", i)
		}
		ids = append(ids, id)
		defer s.DeleteJSON(ctx, base+"/"+id) //nolint:errcheck
	}

	storedIndexes := func() map[string]int64 {
		rules, err := client.ListFirewallRule(ctx, c.Site)
		if err != nil {
			t.Fatalf("list firewall rules: %v", err)
		}
		out := map[string]int64{}
		for _, rule := range rules {
			if rule.RuleIndex != nil && len(rule.Name) > 13 && rule.Name[:13] == "reorder-probe" {
				out[rule.Name] = *rule.RuleIndex
			}
		}
		return out
	}

	before := storedIndexes()
	if len(before) != 3 {
		t.Fatalf("expected three seeded rules, read back %v", before)
	}

	// Reverse them. Every rule moves, so nothing here passes by accident.
	want := []int64{2003, 2002, 2001}
	update := make([]FirewallRuleIndexUpdate, len(ids))
	for i, id := range ids {
		update[i] = FirewallRuleIndexUpdate{Id: id, RuleIndex: want[i]}
	}
	if err := client.ReorderFirewallRules(ctx, c.Site, "LAN_IN", update); err != nil {
		t.Fatalf("ReorderFirewallRules: %v", err)
	}

	after := storedIndexes()
	for i, index := range want {
		name := fmt.Sprintf("reorder-probe-%d", i)
		if after[name] != index {
			t.Errorf("%s has rule_index %d after the reorder, want %d (before: %d)",
				name, after[name], index, before[name])
		}
	}

	// The REST collection cannot do this. Writing the first rule's new index
	// while another rule still holds it is refused, which is the whole
	// reason the command exists.
	_, status, err := s.PutJSON(ctx, base+"/"+ids[0], map[string]any{
		"_id": ids[0], "name": "reorder-probe-0", "ruleset": "LAN_IN",
		"rule_index": after["reorder-probe-1"],
		"action":     "accept", "protocol": "all", "enabled": true,
	})
	if err != nil {
		t.Fatalf("transport: %v", err)
	}
	if status == 200 {
		t.Log("note: this controller accepted a REST write of an index another rule holds; " +
			"a reorder could become a sequence of masked updates if that holds up")
	}
}

// TestIntegrationFirewallCommandAcceptsAnythingItDoesNotKnow pins the reason
// ReorderFirewallRules inspects the response body rather than the status
// code. The command manager answers a name it has never heard of with
// HTTP 200 and rc ok, so a caller that discards the body cannot tell a
// performed command from an ignored one.
func TestIntegrationFirewallCommandAcceptsAnythingItDoesNotKnow(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/cmd/firewall",
		map[string]any{"cmd": "definitely-not-a-command", "ruleset": "LAN_IN"})
	if err != nil {
		t.Fatalf("transport: %v", err)
	}
	if status != 200 {
		t.Logf("this controller now rejects an unknown firewall command with HTTP %d (%v); "+
			"the response check in ReorderFirewallRules is no longer the only thing "+
			"separating a performed command from an ignored one", status, body)
		return
	}

	data, _ := body.(map[string]any)["data"].([]any)
	if len(data) != 0 {
		t.Errorf("an unknown command answered with a result (%v); the check for an empty "+
			"data array no longer distinguishes it from a real one", data)
	}
}
