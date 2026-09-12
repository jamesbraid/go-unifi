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

type HotspotOp struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Name     string `json:"name,omitempty"` // .{1,256}
	Note     string `json:"note,omitempty"`
	Password string `json:"x_password,omitempty"` // .{1,256}
}

func (dst *HotspotOp) UnmarshalJSON(b []byte) error {
	type Alias HotspotOp
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

func (c *ApiClient) ListHotspotOp(ctx context.Context, site string) ([]HotspotOp, error) {
	return envelopeList[HotspotOp](ctx, c, fmt.Sprintf("api/s/%s/rest/hotspotop", site))
}

func (c *ApiClient) GetHotspotOp(ctx context.Context, site string, id string) (*HotspotOp, error) {
	return envelopeOne[HotspotOp](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/hotspotop/%s", site, id), nil)
}

func (c *ApiClient) DeleteHotspotOp(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/hotspotop/%s", site, id))
}

func (c *ApiClient) CreateHotspotOp(ctx context.Context, site string, d *HotspotOp) (*HotspotOp, error) {
	return envelopeOne[HotspotOp](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/hotspotop", site), d)
}

// UpdateHotspotOpFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateHotspotOpFields(ctx context.Context, site string, d *HotspotOp, fields ...string) (*HotspotOp, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/hotspotop/%s", site, d.ID), d, fields, func() (*HotspotOp, error) { return c.GetHotspotOp(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateHotspotOp(ctx context.Context, site string, d *HotspotOp) (*HotspotOp, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/hotspotop/%s", site, d.ID), d, func() (*HotspotOp, error) { return c.GetHotspotOp(ctx, site, d.ID) })
}
