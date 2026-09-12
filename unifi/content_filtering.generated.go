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

type ContentFiltering struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	AllowList  []string                  `json:"allow_list"`
	BlockList  []string                  `json:"block_list"`
	Categories []string                  `json:"categories"`
	ClientMACs []string                  `json:"client_macs"`
	Enabled    bool                      `json:"enabled"`
	Name       string                    `json:"name,omitempty"`
	NetworkIDs []string                  `json:"network_ids"`
	SafeSearch []string                  `json:"safe_search"`
	Schedule   *ContentFilteringSchedule `json:"schedule,omitempty"`
}

func (dst *ContentFiltering) UnmarshalJSON(b []byte) error {
	type Alias ContentFiltering
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

type ContentFilteringSchedule struct {
	Mode string `json:"mode,omitempty"`
}

func (dst *ContentFilteringSchedule) UnmarshalJSON(b []byte) error {
	type Alias ContentFilteringSchedule
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

func (c *ApiClient) listContentFiltering(ctx context.Context, site string, query ...map[string]string) ([]ContentFiltering, error) {
	return bareList[ContentFiltering](ctx, c, fmt.Sprintf("v2/api/site/%s/content-filtering", site), query...)
}

func (c *ApiClient) GetContentFiltering(ctx context.Context, site string, id string) (*ContentFiltering, error) {
	stored, err := c.listContentFiltering(ctx, site)
	if err != nil {
		return nil, err
	}
	return findByID(stored, id, func(v *ContentFiltering) string { return v.ID })
}

func (c *ApiClient) DeleteContentFiltering(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("v2/api/site/%s/content-filtering/%s", site, id))
}

func (c *ApiClient) CreateContentFiltering(ctx context.Context, site string, d *ContentFiltering) (*ContentFiltering, error) {
	return bareOne[ContentFiltering](ctx, c, http.MethodPost, fmt.Sprintf("v2/api/site/%s/content-filtering", site), d)
}

// UpdateContentFilteringFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateContentFilteringFields(ctx context.Context, site string, d *ContentFiltering, fields ...string) (*ContentFiltering, error) {
	return bareMasked(ctx, c, fmt.Sprintf("v2/api/site/%s/content-filtering/%s", site, d.ID), d, fields)
}

func (c *ApiClient) UpdateContentFiltering(ctx context.Context, site string, d *ContentFiltering) (*ContentFiltering, error) {
	return bareOne[ContentFiltering](ctx, c, http.MethodPut, fmt.Sprintf("v2/api/site/%s/content-filtering/%s", site, d.ID), d)
}
