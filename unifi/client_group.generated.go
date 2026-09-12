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

type ClientGroup struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Name           string `json:"name,omitempty"`              // .{1,128}
	QOSRateMaxDown *int64 `json:"qos_rate_max_down,omitempty"` // -1|[2-9]|[1-9][0-9]{1,4}|100000
	QOSRateMaxUp   *int64 `json:"qos_rate_max_up,omitempty"`   // -1|[2-9]|[1-9][0-9]{1,4}|100000
}

func (dst *ClientGroup) UnmarshalJSON(b []byte) error {
	type Alias ClientGroup
	aux := &struct {
		QOSRateMaxDown *types.Number `json:"qos_rate_max_down"`
		QOSRateMaxUp   *types.Number `json:"qos_rate_max_up"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.QOSRateMaxDown != nil {
		if val, err := aux.QOSRateMaxDown.Int64(); err == nil {
			dst.QOSRateMaxDown = &val
		} else if string(*aux.QOSRateMaxDown) == "" {
			var zero int64
			dst.QOSRateMaxDown = &zero
		}
	}
	if aux.QOSRateMaxUp != nil {
		if val, err := aux.QOSRateMaxUp.Int64(); err == nil {
			dst.QOSRateMaxUp = &val
		} else if string(*aux.QOSRateMaxUp) == "" {
			var zero int64
			dst.QOSRateMaxUp = &zero
		}
	}

	return nil
}

func (c *ApiClient) ListClientGroup(ctx context.Context, site string) ([]ClientGroup, error) {
	return envelopeList[ClientGroup](ctx, c, fmt.Sprintf("api/s/%s/rest/usergroup", site))
}

func (c *ApiClient) GetClientGroup(ctx context.Context, site string, id string) (*ClientGroup, error) {
	return envelopeOne[ClientGroup](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/usergroup/%s", site, id), nil)
}

func (c *ApiClient) DeleteClientGroup(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/usergroup/%s", site, id))
}

func (c *ApiClient) CreateClientGroup(ctx context.Context, site string, d *ClientGroup) (*ClientGroup, error) {
	return envelopeOne[ClientGroup](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/usergroup", site), d)
}

// UpdateClientGroupFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateClientGroupFields(ctx context.Context, site string, d *ClientGroup, fields ...string) (*ClientGroup, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/usergroup/%s", site, d.ID), d, fields, func() (*ClientGroup, error) { return c.GetClientGroup(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateClientGroup(ctx context.Context, site string, d *ClientGroup) (*ClientGroup, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/usergroup/%s", site, d.ID), d, func() (*ClientGroup, error) { return c.GetClientGroup(ctx, site, d.ID) })
}
