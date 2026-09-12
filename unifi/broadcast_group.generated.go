// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type BroadcastGroup struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	MemberTable []string `json:"member_table,omitempty"`
	Name        string   `json:"name,omitempty"`
}

func (dst *BroadcastGroup) UnmarshalJSON(b []byte) error {
	type Alias BroadcastGroup
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

func (c *ApiClient) ListBroadcastGroup(ctx context.Context, site string) ([]BroadcastGroup, error) {
	return envelopeList[BroadcastGroup](ctx, c, fmt.Sprintf("api/s/%s/rest/broadcastgroup", site))
}

func (c *ApiClient) GetBroadcastGroup(ctx context.Context, site string, id string) (*BroadcastGroup, error) {
	return envelopeOne[BroadcastGroup](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/broadcastgroup/%s", site, id), nil)
}

func (c *ApiClient) DeleteBroadcastGroup(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/broadcastgroup/%s", site, id))
}

func (c *ApiClient) CreateBroadcastGroup(ctx context.Context, site string, d *BroadcastGroup) (*BroadcastGroup, error) {
	return envelopeOne[BroadcastGroup](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/broadcastgroup", site), d)
}

// UpdateBroadcastGroupFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateBroadcastGroupFields(ctx context.Context, site string, d *BroadcastGroup, fields ...string) (*BroadcastGroup, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/broadcastgroup/%s", site, d.ID), d, fields, func() (*BroadcastGroup, error) { return c.GetBroadcastGroup(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateBroadcastGroup(ctx context.Context, site string, d *BroadcastGroup) (*BroadcastGroup, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/broadcastgroup/%s", site, d.ID), d, func() (*BroadcastGroup, error) { return c.GetBroadcastGroup(ctx, site, d.ID) })
}
