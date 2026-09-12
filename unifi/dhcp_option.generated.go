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

type DHCPOption struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Code   string `json:"code,omitempty"` // ^(?!(?:15|42|43|44|51|66|67|252)$)([7-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-4])$
	Name   string `json:"name,omitempty"` // ^[A-Za-z0-9-_]{1,25}$
	Signed bool   `json:"signed"`
	Type   string `json:"type,omitempty"`  // ^(boolean|hexarray|integer|ipaddress|macaddress|text)$
	Width  *int64 `json:"width,omitempty"` // ^(8|16|32)$
}

func (dst *DHCPOption) UnmarshalJSON(b []byte) error {
	type Alias DHCPOption
	aux := &struct {
		Width *types.Number `json:"width"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.Width != nil {
		if val, err := aux.Width.Int64(); err == nil {
			dst.Width = &val
		} else if string(*aux.Width) == "" {
			var zero int64
			dst.Width = &zero
		}
	}

	return nil
}

func (c *ApiClient) ListDHCPOption(ctx context.Context, site string) ([]DHCPOption, error) {
	return envelopeList[DHCPOption](ctx, c, fmt.Sprintf("api/s/%s/rest/dhcpoption", site))
}

func (c *ApiClient) GetDHCPOption(ctx context.Context, site string, id string) (*DHCPOption, error) {
	return envelopeOne[DHCPOption](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/dhcpoption/%s", site, id), nil)
}

func (c *ApiClient) DeleteDHCPOption(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/dhcpoption/%s", site, id))
}

func (c *ApiClient) CreateDHCPOption(ctx context.Context, site string, d *DHCPOption) (*DHCPOption, error) {
	return envelopeOne[DHCPOption](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/dhcpoption", site), d)
}

// UpdateDHCPOptionFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateDHCPOptionFields(ctx context.Context, site string, d *DHCPOption, fields ...string) (*DHCPOption, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/dhcpoption/%s", site, d.ID), d, fields, func() (*DHCPOption, error) { return c.GetDHCPOption(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateDHCPOption(ctx context.Context, site string, d *DHCPOption) (*DHCPOption, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/dhcpoption/%s", site, d.ID), d, func() (*DHCPOption, error) { return c.GetDHCPOption(ctx, site, d.ID) })
}
