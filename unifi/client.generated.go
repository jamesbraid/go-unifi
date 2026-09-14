// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

type Client struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	DisplayName string `json:"display_name,omitempty"`

	Blocked        *bool  `json:"blocked,omitempty"`
	FixedApEnabled bool   `json:"fixed_ap_enabled"`
	FixedApMAC     string `json:"fixed_ap_mac,omitempty"` // ^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$
	FixedIP        string `json:"fixed_ip,omitempty"`
	Hostname       string `json:"hostname,omitempty"`
	// The client's most recent IP, reported by the controller on /rest/user. Read-only: the controller publishes no schema for it, so it carries no validation.
	LastIP                        string   `json:"last_ip,omitempty"`
	LastSeen                      *int64   `json:"last_seen,omitempty"`
	LocalDNSRecord                string   `json:"local_dns_record,omitempty"`
	LocalDNSRecordEnabled         bool     `json:"local_dns_record_enabled"`
	MAC                           string   `json:"mac"` // ^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$
	Name                          string   `json:"name,omitempty"`
	NetworkID                     string   `json:"network_id,omitempty"`
	NetworkMembersGroupIDs        []string `json:"network_members_group_ids,omitempty"`
	Note                          string   `json:"note,omitempty"`
	UseFixedIP                    bool     `json:"use_fixedip"`
	UserGroupID                   string   `json:"usergroup_id,omitempty"`
	VirtualNetworkOverrideEnabled *bool    `json:"virtual_network_override_enabled,omitempty"`
	VirtualNetworkOverrideID      string   `json:"virtual_network_override_id,omitempty"`
}

func (dst *Client) UnmarshalJSON(b []byte) error {
	type Alias Client
	aux := &struct {
		Blocked *types.Bool `json:"blocked"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	dst.Blocked = boolPtrValue(aux.Blocked)

	return nil
}

func (c *ApiClient) listClient(ctx context.Context, site string, query ...map[string]string) ([]Client, error) {
	return envelopeList[Client](ctx, c, fmt.Sprintf("api/s/%s/rest/user", site), query...)
}

func (c *ApiClient) GetClient(ctx context.Context, site string, id string) (*Client, error) {
	return envelopeOne[Client](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/user/%s", site, id), nil)
}

func (c *ApiClient) CreateClient(ctx context.Context, site string, d *Client) (*Client, error) {
	return envelopeOne[Client](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/user", site), d)
}

// UpdateClientFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateClientFields(ctx context.Context, site string, d *Client, fields ...string) (*Client, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/user/%s", site, d.ID), d, fields, func() (*Client, error) { return c.GetClient(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateClient(ctx context.Context, site string, d *Client) (*Client, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/user/%s", site, d.ID), d, func() (*Client, error) { return c.GetClient(ctx, site, d.ID) })
}
