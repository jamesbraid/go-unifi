package unifi

import (
	"context"
)

type FirewallRuleIndexUpdate struct {
	Id        string `json:"_id"`
	RuleIndex int64  `json:"rule_index,string"`
}

func (c *ApiClient) ListFirewallRule(ctx context.Context, site string) ([]FirewallRule, error) {
	return c.listFirewallRule(ctx, site)
}

func (c *ApiClient) GetFirewallRule(ctx context.Context, site, id string) (*FirewallRule, error) {
	return c.getFirewallRule(ctx, site, id)
}

func (c *ApiClient) DeleteFirewallRule(ctx context.Context, site, id string) error {
	return c.deleteFirewallRule(ctx, site, id)
}

func (c *ApiClient) CreateFirewallRule(
	ctx context.Context,
	site string,
	d *FirewallRule,
) (*FirewallRule, error) {
	return c.createFirewallRule(ctx, site, d)
}

func (c *ApiClient) UpdateFirewallRule(
	ctx context.Context,
	site string,
	d *FirewallRule,
) (*FirewallRule, error) {
	return c.updateFirewallRule(ctx, site, d)
}

func (c *ApiClient) ReorderFirewallRules(
	ctx context.Context,
	site, ruleset string,
	reorder []FirewallRuleIndexUpdate,
) error {
	reqBody := struct {
		Cmd     string                    `json:"cmd"`
		Ruleset string                    `json:"ruleset"`
		Rules   []FirewallRuleIndexUpdate `json:"rules"`
	}{
		Cmd:     "reorder",
		Ruleset: ruleset,
		Rules:   reorder,
	}
	var respBody commandResult
	if err := c.siteCommand(ctx, site, "firewall", reqBody, &respBody); err != nil {
		return err
	}

	return respBody.acted("reorder")
}
