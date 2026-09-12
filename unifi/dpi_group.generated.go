// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type DpiGroup struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	DPIappIDs []string `json:"dpiapp_ids,omitempty"` // [\d\w-]+
	Enabled   bool     `json:"enabled"`
	Name      string   `json:"name,omitempty"` // .{1,128}
}

func (dst *DpiGroup) UnmarshalJSON(b []byte) error {
	type Alias DpiGroup
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

func (c *ApiClient) ListDpiGroup(ctx context.Context, site string) ([]DpiGroup, error) {
	return envelopeList[DpiGroup](ctx, c, fmt.Sprintf("api/s/%s/rest/dpigroup", site))
}

func (c *ApiClient) GetDpiGroup(ctx context.Context, site string, id string) (*DpiGroup, error) {
	return envelopeOne[DpiGroup](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/dpigroup/%s", site, id), nil)
}

func (c *ApiClient) DeleteDpiGroup(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/dpigroup/%s", site, id))
}

func (c *ApiClient) CreateDpiGroup(ctx context.Context, site string, d *DpiGroup) (*DpiGroup, error) {
	return envelopeOne[DpiGroup](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/dpigroup", site), d)
}

// UpdateDpiGroupFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateDpiGroupFields(ctx context.Context, site string, d *DpiGroup, fields ...string) (*DpiGroup, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/dpigroup/%s", site, d.ID), d, fields, func() (*DpiGroup, error) { return c.GetDpiGroup(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateDpiGroup(ctx context.Context, site string, d *DpiGroup) (*DpiGroup, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/dpigroup/%s", site, d.ID), d, func() (*DpiGroup, error) { return c.GetDpiGroup(ctx, site, d.ID) })
}
