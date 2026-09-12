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

type WLANGroup struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Name string `json:"name,omitempty"` // .{1,128}
}

func (dst *WLANGroup) UnmarshalJSON(b []byte) error {
	type Alias WLANGroup
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

func (c *ApiClient) ListWLANGroup(ctx context.Context, site string) ([]WLANGroup, error) {
	return envelopeList[WLANGroup](ctx, c, fmt.Sprintf("api/s/%s/rest/wlangroup", site))
}

func (c *ApiClient) GetWLANGroup(ctx context.Context, site string, id string) (*WLANGroup, error) {
	return envelopeOne[WLANGroup](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/wlangroup/%s", site, id), nil)
}

func (c *ApiClient) DeleteWLANGroup(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/wlangroup/%s", site, id))
}

func (c *ApiClient) CreateWLANGroup(ctx context.Context, site string, d *WLANGroup) (*WLANGroup, error) {
	return envelopeOne[WLANGroup](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/wlangroup", site), d)
}

// UpdateWLANGroupFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateWLANGroupFields(ctx context.Context, site string, d *WLANGroup, fields ...string) (*WLANGroup, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/wlangroup/%s", site, d.ID), d, fields, func() (*WLANGroup, error) { return c.GetWLANGroup(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateWLANGroup(ctx context.Context, site string, d *WLANGroup) (*WLANGroup, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/wlangroup/%s", site, d.ID), d, func() (*WLANGroup, error) { return c.GetWLANGroup(ctx, site, d.ID) })
}
