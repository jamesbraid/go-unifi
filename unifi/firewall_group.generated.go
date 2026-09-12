// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

// just to fix compile issues with the import.
var (
	_ context.Context
	_ fmt.Formatter
	_ json.Marshaler
	_ types.Number
	_ strconv.NumError
	_ http.Client
)

type FirewallGroup struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Description           string   `json:"description,omitempty"`
	GroupMembers          []string `json:"group_members,omitempty"`
	GroupType             string   `json:"group_type,omitempty"` // address-group|port-group|ipv6-address-group|domain-group
	Name                  string   `json:"name,omitempty"`       // .{1,64}
	Source                string   `json:"source,omitempty"`     // static|dynamic
	UpdateIntervalSeconds string   `json:"update_interval_seconds,omitempty"`
	Url                   string   `json:"url,omitempty"`
}

func (dst *FirewallGroup) UnmarshalJSON(b []byte) error {
	type Alias FirewallGroup
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}

	return nil
}

func (c *ApiClient) ListFirewallGroup(ctx context.Context, site string) ([]FirewallGroup, error) {
	return envelopeList[FirewallGroup](ctx, c, fmt.Sprintf("api/s/%s/rest/firewallgroup", site))
}

func (c *ApiClient) GetFirewallGroup(ctx context.Context, site string, id string) (*FirewallGroup, error) {
	return envelopeOne[FirewallGroup](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/firewallgroup/%s", site, id), nil)
}

func (c *ApiClient) DeleteFirewallGroup(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/firewallgroup/%s", site, id))
}

func (c *ApiClient) CreateFirewallGroup(ctx context.Context, site string, d *FirewallGroup) (*FirewallGroup, error) {
	return envelopeOne[FirewallGroup](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/firewallgroup", site), d)
}

// UpdateFirewallGroupFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateFirewallGroupFields(ctx context.Context, site string, d *FirewallGroup, fields ...string) (*FirewallGroup, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/firewallgroup/%s", site, d.ID), d, fields, func() (*FirewallGroup, error) { return c.GetFirewallGroup(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateFirewallGroup(ctx context.Context, site string, d *FirewallGroup) (*FirewallGroup, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/firewallgroup/%s", site, d.ID), d, func() (*FirewallGroup, error) { return c.GetFirewallGroup(ctx, site, d.ID) })
}
