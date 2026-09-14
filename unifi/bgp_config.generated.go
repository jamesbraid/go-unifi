// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type BGPConfig struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Config           string `json:"frr_bgpd_config"`
	Description      string `json:"description,omitempty"` // .{0,128}
	Enabled          bool   `json:"enabled"`
	UploadedFileName string `json:"uploaded_file_name"` // .{0,256}
}

func (dst *BGPConfig) UnmarshalJSON(b []byte) error {
	type Alias BGPConfig
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

func (c *ApiClient) GetBGPConfig(ctx context.Context, site string) (*BGPConfig, error) {
	stored, err := bareList[BGPConfig](ctx, c, fmt.Sprintf("v2/api/site/%s/bgp/config", site))
	if err != nil {
		return nil, err
	}
	return onlyElement(stored)
}

func (c *ApiClient) DeleteBGPConfig(ctx context.Context, site string) error {
	return deleteResource(ctx, c, fmt.Sprintf("v2/api/site/%s/bgp/config", site))
}

func (c *ApiClient) CreateBGPConfig(ctx context.Context, site string, d *BGPConfig) (*BGPConfig, error) {
	return bareOne[BGPConfig](ctx, c, http.MethodPost, fmt.Sprintf("v2/api/site/%s/bgp/config", site), d)
}
