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

type DpiApp struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Apps           []int64 `json:"apps,omitempty"`
	Blocked        bool    `json:"blocked"`
	Cats           []int64 `json:"cats,omitempty"`
	Enabled        bool    `json:"enabled"`
	Log            bool    `json:"log"`
	Name           string  `json:"name,omitempty"`              // .{1,128}
	QOSRateMaxDown *int64  `json:"qos_rate_max_down,omitempty"` // -1|[2-9]|[1-9][0-9]{1,4}|100000|10[0-1][0-9]{3}|102[0-3][0-9]{2}|102400
	QOSRateMaxUp   *int64  `json:"qos_rate_max_up,omitempty"`   // -1|[2-9]|[1-9][0-9]{1,4}|100000|10[0-1][0-9]{3}|102[0-3][0-9]{2}|102400
}

func (dst *DpiApp) UnmarshalJSON(b []byte) error {
	type Alias DpiApp
	aux := &struct {
		Apps           []types.Number `json:"apps"`
		Cats           []types.Number `json:"cats"`
		QOSRateMaxDown *types.Number  `json:"qos_rate_max_down"`
		QOSRateMaxUp   *types.Number  `json:"qos_rate_max_up"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	dst.Apps = make([]int64, len(aux.Apps))
	for i, v := range aux.Apps {
		if val, err := v.Int64(); err == nil {
			dst.Apps[i] = val
		}
	}
	dst.Cats = make([]int64, len(aux.Cats))
	for i, v := range aux.Cats {
		if val, err := v.Int64(); err == nil {
			dst.Cats[i] = val
		}
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

func (c *ApiClient) ListDpiApp(ctx context.Context, site string) ([]DpiApp, error) {
	return envelopeList[DpiApp](ctx, c, fmt.Sprintf("api/s/%s/rest/dpiapp", site))
}

func (c *ApiClient) GetDpiApp(ctx context.Context, site string, id string) (*DpiApp, error) {
	return envelopeOne[DpiApp](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/dpiapp/%s", site, id), nil)
}

func (c *ApiClient) DeleteDpiApp(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/dpiapp/%s", site, id))
}

func (c *ApiClient) CreateDpiApp(ctx context.Context, site string, d *DpiApp) (*DpiApp, error) {
	return envelopeOne[DpiApp](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/dpiapp", site), d)
}

// UpdateDpiAppFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateDpiAppFields(ctx context.Context, site string, d *DpiApp, fields ...string) (*DpiApp, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/dpiapp/%s", site, d.ID), d, fields, func() (*DpiApp, error) { return c.GetDpiApp(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateDpiApp(ctx context.Context, site string, d *DpiApp) (*DpiApp, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/dpiapp/%s", site, d.ID), d, func() (*DpiApp, error) { return c.GetDpiApp(ctx, site, d.ID) })
}
