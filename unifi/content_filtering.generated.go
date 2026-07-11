// Code generated from ace.jar fields *.json files
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

func (c *ApiClient) listContentFiltering(
	ctx context.Context,
	site string,
	query ...map[string]string,
) ([]ContentFiltering, error) {
	var respBody []ContentFiltering

	err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("v2/api/site/%s/content-filtering", site),
		nil,
		&respBody,
		query...,
	)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

func (c *ApiClient) getContentFiltering(
	ctx context.Context,
	site string,
	id string,
) (*ContentFiltering, error) {
	respBody, err := c.listContentFiltering(ctx, site)
	if err != nil {
		return nil, err
	}

	if len(respBody) == 0 {
		return nil, &NotFoundError{}
	}

	for _, val := range respBody {
		if val.ID == id {
			return &val, nil
		}
	}

	return nil, &NotFoundError{}
}

func (c *ApiClient) deleteContentFiltering(
	ctx context.Context,
	site string,
	id string,
) error {
	err := c.do(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("v2/api/site/%s/content-filtering/%s", site, id),
		struct{}{},
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *ApiClient) createContentFiltering(
	ctx context.Context,
	site string,
	d *ContentFiltering,
) (*ContentFiltering, error) {
	var respBody ContentFiltering

	err := c.do(
		ctx,
		http.MethodPost,
		fmt.Sprintf("v2/api/site/%s/content-filtering", site),
		d,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

func (c *ApiClient) updateContentFiltering(
	ctx context.Context,
	site string,
	d *ContentFiltering,
) (*ContentFiltering, error) {
	var respBody ContentFiltering
	err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("v2/api/site/%s/content-filtering/%s", site, d.ID),
		d,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}
