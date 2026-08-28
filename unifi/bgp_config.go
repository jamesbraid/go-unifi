// Code generated from ace.jar fields *.json files
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// just to fix compile issues with the import.
var (
	_ context.Context
	_ fmt.Formatter
	_ json.Marshaler
)

func (c *ApiClient) GetBGPConfig(ctx context.Context, site string) (*BGPConfig, error) {
	return c.getBGPConfig(ctx, site)
}

func (c *ApiClient) CreateBGPConfig(
	ctx context.Context,
	site string,
	d *BGPConfig,
) (*BGPConfig, error) {
	return c.createBGPConfig(ctx, site, d)
}

func (c *ApiClient) UpdateBGPConfig(
	ctx context.Context,
	site string,
	d *BGPConfig,
) (*BGPConfig, error) {
	return c.createBGPConfig(ctx, site, d)
}

func (c *ApiClient) DeleteBGPConfig(ctx context.Context, site string) error {
	return c.deleteBGPConfig(ctx, site)
}

// UpdateBGPConfigFields writes only the named wire fields of the site's BGP
// configuration and leaves the rest of the stored object untouched.
//
// The endpoint's only write is a POST of the whole object, which is why the
// generator emits no masked update for this type. So the mask is applied to
// the configuration as stored and the result posted back; see overlayMasked.
func (c *ApiClient) UpdateBGPConfigFields(
	ctx context.Context,
	site string,
	d *BGPConfig,
	fields ...string,
) (*BGPConfig, error) {
	masked, err := maskedBody(d, fields)
	if err != nil {
		return nil, err
	}

	var stored []json.RawMessage
	err = c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("v2/api/site/%s/bgp/config", site),
		nil,
		&stored,
	)
	if err != nil {
		return nil, err
	}
	if len(stored) != 1 {
		return nil, &NotFoundError{}
	}

	merged, err := overlayMasked(stored[0], masked)
	if err != nil {
		return nil, err
	}

	var respBody BGPConfig
	err = c.do(
		ctx,
		http.MethodPost,
		fmt.Sprintf("v2/api/site/%s/bgp/config", site),
		merged,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}
