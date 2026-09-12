// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type MediaFile struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Name string `json:"name,omitempty"`
}

func (dst *MediaFile) UnmarshalJSON(b []byte) error {
	type Alias MediaFile
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

func (c *ApiClient) ListMediaFile(ctx context.Context, site string) ([]MediaFile, error) {
	return envelopeList[MediaFile](ctx, c, fmt.Sprintf("api/s/%s/rest/mediafile", site))
}

func (c *ApiClient) GetMediaFile(ctx context.Context, site string, id string) (*MediaFile, error) {
	return envelopeOne[MediaFile](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/mediafile/%s", site, id), nil)
}

func (c *ApiClient) DeleteMediaFile(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/mediafile/%s", site, id))
}

func (c *ApiClient) CreateMediaFile(ctx context.Context, site string, d *MediaFile) (*MediaFile, error) {
	return envelopeOne[MediaFile](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/mediafile", site), d)
}

// UpdateMediaFileFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateMediaFileFields(ctx context.Context, site string, d *MediaFile, fields ...string) (*MediaFile, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/mediafile/%s", site, d.ID), d, fields, func() (*MediaFile, error) { return c.GetMediaFile(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateMediaFile(ctx context.Context, site string, d *MediaFile) (*MediaFile, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/mediafile/%s", site, d.ID), d, func() (*MediaFile, error) { return c.GetMediaFile(ctx, site, d.ID) })
}
