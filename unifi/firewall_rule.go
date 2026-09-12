package unifi

import (
	"context"
)

type FirewallRuleIndexUpdate struct {
	Id        string `json:"_id"`
	RuleIndex int64  `json:"rule_index,string"`
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
