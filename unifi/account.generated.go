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

type Account struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	FilterIDs        []string `json:"filter_ids,omitempty"`
	IP               string   `json:"ip,omitempty"`   // ^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$
	Name             string   `json:"name,omitempty"` // ^[^"' ]+$
	NetworkID        string   `json:"networkconf_id,omitempty"`
	Password         string   `json:"x_password,omitempty"`
	TunnelConfigType string   `json:"tunnel_config_type,omitempty"` // vpn|802.1x|custom
	TunnelMediumType *int64   `json:"tunnel_medium_type,omitempty"` // [1-9]|1[0-5]|^$
	TunnelType       *int64   `json:"tunnel_type,omitempty"`        // [1-9]|1[0-3]|^$
	UlpUserID        string   `json:"ulp_user_id,omitempty"`
	VLAN             *int64   `json:"vlan,omitempty"` // [2-9]|[1-9][0-9]{1,2}|[1-3][0-9]{3}|400[0-9]|^$
}

func (dst *Account) UnmarshalJSON(b []byte) error {
	type Alias Account
	aux := &struct {
		TunnelMediumType *types.Number `json:"tunnel_medium_type"`
		TunnelType       *types.Number `json:"tunnel_type"`
		VLAN             *types.Number `json:"vlan"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.TunnelMediumType != nil {
		if val, err := aux.TunnelMediumType.Int64(); err == nil {
			dst.TunnelMediumType = &val
		} else if string(*aux.TunnelMediumType) == "" {
			var zero int64
			dst.TunnelMediumType = &zero
		}
	}
	if aux.TunnelType != nil {
		if val, err := aux.TunnelType.Int64(); err == nil {
			dst.TunnelType = &val
		} else if string(*aux.TunnelType) == "" {
			var zero int64
			dst.TunnelType = &zero
		}
	}
	if aux.VLAN != nil {
		if val, err := aux.VLAN.Int64(); err == nil {
			dst.VLAN = &val
		} else if string(*aux.VLAN) == "" {
			var zero int64
			dst.VLAN = &zero
		}
	}

	return nil
}

func (c *ApiClient) ListAccount(ctx context.Context, site string) ([]Account, error) {
	return envelopeList[Account](ctx, c, fmt.Sprintf("api/s/%s/rest/account", site))
}

func (c *ApiClient) GetAccount(ctx context.Context, site string, id string) (*Account, error) {
	return envelopeOne[Account](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/account/%s", site, id), nil)
}

func (c *ApiClient) DeleteAccount(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/account/%s", site, id))
}

func (c *ApiClient) CreateAccount(ctx context.Context, site string, d *Account) (*Account, error) {
	return envelopeOne[Account](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/account", site), d)
}

// UpdateAccountFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateAccountFields(ctx context.Context, site string, d *Account, fields ...string) (*Account, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/account/%s", site, d.ID), d, fields, func() (*Account, error) { return c.GetAccount(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateAccount(ctx context.Context, site string, d *Account) (*Account, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/account/%s", site, d.ID), d, func() (*Account, error) { return c.GetAccount(ctx, site, d.ID) })
}
