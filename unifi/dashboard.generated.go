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

type Dashboard struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	ControllerVersion string             `json:"controller_version,omitempty"`
	Desc              string             `json:"desc,omitempty"`
	IsPublic          bool               `json:"is_public"`
	Modules           []DashboardModules `json:"modules,omitempty"`
	Name              string             `json:"name,omitempty"`
}

func (dst *Dashboard) UnmarshalJSON(b []byte) error {
	type Alias Dashboard
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

type DashboardModules struct {
	Config       string `json:"config,omitempty"`
	ID           string `json:"id,omitempty"`
	ModuleID     string `json:"module_id,omitempty"`
	Restrictions string `json:"restrictions,omitempty"`
}

func (dst *DashboardModules) UnmarshalJSON(b []byte) error {
	type Alias DashboardModules
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

func (c *ApiClient) ListDashboard(ctx context.Context, site string) ([]Dashboard, error) {
	return envelopeList[Dashboard](ctx, c, fmt.Sprintf("api/s/%s/rest/dashboard", site))
}

func (c *ApiClient) GetDashboard(ctx context.Context, site string, id string) (*Dashboard, error) {
	return envelopeOne[Dashboard](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/dashboard/%s", site, id), nil)
}

func (c *ApiClient) DeleteDashboard(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/dashboard/%s", site, id))
}

func (c *ApiClient) CreateDashboard(ctx context.Context, site string, d *Dashboard) (*Dashboard, error) {
	return envelopeOne[Dashboard](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/dashboard", site), d)
}

// UpdateDashboardFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateDashboardFields(ctx context.Context, site string, d *Dashboard, fields ...string) (*Dashboard, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/dashboard/%s", site, d.ID), d, fields, func() (*Dashboard, error) { return c.GetDashboard(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateDashboard(ctx context.Context, site string, d *Dashboard) (*Dashboard, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/dashboard/%s", site, d.ID), d, func() (*Dashboard, error) { return c.GetDashboard(ctx, site, d.ID) })
}
