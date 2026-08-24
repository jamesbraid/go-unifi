// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

// just to fix compile issues with the import.
var (
	_ context.Context
	_ fmt.Formatter
	_ json.Marshaler
	_ types.Number
	_ strconv.NumError
	_ strings.Builder
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

func (c *ApiClient) listMediaFile(
	ctx context.Context,
	site string,
	query ...map[string]string,
) ([]MediaFile, error) {
	var respBody struct {
		Meta meta        `json:"meta"`
		Data []MediaFile `json:"data"`
	}

	err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("api/s/%s/rest/mediafile", site),
		nil,
		&respBody,
		query...,
	)
	if err != nil {
		return nil, err
	}
	return respBody.Data, nil
}

func (c *ApiClient) getMediaFile(
	ctx context.Context,
	site string,
	id string,
) (*MediaFile, error) {
	var respBody struct {
		Meta meta        `json:"meta"`
		Data []MediaFile `json:"data"`
	}
	err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("api/s/%s/rest/mediafile/%s", site, id),
		nil,
		&respBody,
	)
	if err != nil {
		return nil, err
	}
	if len(respBody.Data) != 1 {
		return nil, &NotFoundError{}
	}

	d := respBody.Data[0]
	return &d, nil
}

func (c *ApiClient) deleteMediaFile(
	ctx context.Context,
	site string,
	id string,
) error {
	err := c.do(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("api/s/%s/rest/mediafile/%s", site, id),
		struct{}{},
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *ApiClient) createMediaFile(
	ctx context.Context,
	site string,
	d *MediaFile,
) (*MediaFile, error) {
	var respBody struct {
		Meta meta        `json:"meta"`
		Data []MediaFile `json:"data"`
	}

	err := c.do(
		ctx,
		http.MethodPost,
		fmt.Sprintf("api/s/%s/rest/mediafile", site),
		d,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	if len(respBody.Data) != 1 {
		return nil, &NotFoundError{}
	}

	res := respBody.Data[0]

	return &res, nil
}

// UpdateMediaFileFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
func (c *ApiClient) UpdateMediaFileFields(ctx context.Context, site string, d *MediaFile, fields ...string) (*MediaFile, error) {
	return c.updateMediaFileFields(ctx, site, d, fields)
}

// updateMediaFileFields writes only the named wire fields, leaving
// every other field on the stored object alone. See maskedBody.
func (c *ApiClient) updateMediaFileFields(
	ctx context.Context,
	site string,
	d *MediaFile,
	fields []string,
) (*MediaFile, error) {
	body, err := maskedBody(d, fields)
	if err != nil {
		return nil, err
	}
	var respBody struct {
		Meta meta        `json:"meta"`
		Data []MediaFile `json:"data"`
	}
	if err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("api/s/%s/rest/mediafile/%s", site, d.ID),
		body,
		&respBody,
	); err != nil {
		return nil, err
	}

	if len(respBody.Data) == 0 {
		return c.getMediaFile(ctx, site, d.ID)
	}
	if len(respBody.Data) != 1 {
		return nil, &NotFoundError{}
	}
	res := respBody.Data[0]
	return &res, nil
}

func (c *ApiClient) updateMediaFile(
	ctx context.Context,
	site string,
	d *MediaFile,
) (*MediaFile, error) {
	var respBody struct {
		Meta meta        `json:"meta"`
		Data []MediaFile `json:"data"`
	}
	err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("api/s/%s/rest/mediafile/%s", site, d.ID),
		d,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	// UDM SE API returns empty data array on successful PUT.
	// In that case, fetch the updated resource via GET.
	if len(respBody.Data) == 0 {
		return c.getMediaFile(ctx, site, d.ID)
	}

	if len(respBody.Data) != 1 {
		return nil, &NotFoundError{}
	}

	res := respBody.Data[0]

	return &res, nil
}
