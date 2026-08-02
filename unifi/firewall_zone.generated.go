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

type FirewallZone struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	CloudTemplate string   `json:"cloud_template,omitempty"`
	DefaultZone   *bool    `json:"default_zone,omitempty"`
	ExternalID    string   `json:"external_id,omitempty"`
	Name          string   `json:"name,omitempty"`
	NetworkIDs    []string `json:"network_ids"`
	ZoneKey       string   `json:"zone_key,omitempty"`
}

// MarshalJSON omits the fields the controller reports but rejects on a write,
// so a value read from the controller can be modified and written straight
// back. Without it an update after a read fails on the server-assigned fields
// the read filled in.
func (src FirewallZone) MarshalJSON() ([]byte, error) {
	type Alias FirewallZone
	return json.Marshal(&struct {
		SiteID        *struct{} `json:"site_id,omitempty"`
		Hidden        *struct{} `json:"attr_hidden,omitempty"`
		HiddenID      *struct{} `json:"attr_hidden_id,omitempty"`
		NoDelete      *struct{} `json:"attr_no_delete,omitempty"`
		NoEdit        *struct{} `json:"attr_no_edit,omitempty"`
		CloudTemplate *struct{} `json:"cloud_template,omitempty"`
		DefaultZone   *struct{} `json:"default_zone,omitempty"`
		ExternalID    *struct{} `json:"external_id,omitempty"`
		ZoneKey       *struct{} `json:"zone_key,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(&src),
	})
}

func (dst *FirewallZone) UnmarshalJSON(b []byte) error {
	type Alias FirewallZone
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

func (c *ApiClient) listFirewallZone(
	ctx context.Context,
	site string,
	query ...map[string]string,
) ([]FirewallZone, error) {
	var respBody []FirewallZone

	err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("v2/api/site/%s/firewall/zone", site),
		nil,
		&respBody,
		query...,
	)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

func (c *ApiClient) getFirewallZone(
	ctx context.Context,
	site string,
	id string,
) (*FirewallZone, error) {
	respBody, err := c.listFirewallZone(ctx, site)
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

func (c *ApiClient) deleteFirewallZone(
	ctx context.Context,
	site string,
	id string,
) error {
	err := c.do(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("v2/api/site/%s/firewall/zone/%s", site, id),
		struct{}{},
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *ApiClient) createFirewallZone(
	ctx context.Context,
	site string,
	d *FirewallZone,
) (*FirewallZone, error) {
	var respBody FirewallZone

	err := c.do(
		ctx,
		http.MethodPost,
		fmt.Sprintf("v2/api/site/%s/firewall/zone", site),
		d,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

// UpdateFirewallZoneFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
func (c *ApiClient) UpdateFirewallZoneFields(ctx context.Context, site string, d *FirewallZone, fields ...string) (*FirewallZone, error) {
	return c.updateFirewallZoneFields(ctx, site, d, fields)
}

// updateFirewallZoneFields writes only the named wire fields, leaving
// every other field on the stored object alone. See maskedBody.
func (c *ApiClient) updateFirewallZoneFields(
	ctx context.Context,
	site string,
	d *FirewallZone,
	fields []string,
) (*FirewallZone, error) {
	body, err := maskedBody(d, fields)
	if err != nil {
		return nil, err
	}
	var respBody FirewallZone
	if err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("v2/api/site/%s/firewall/zone/%s", site, d.ID),
		body,
		&respBody,
	); err != nil {
		return nil, err
	}

	return &respBody, nil
}

func (c *ApiClient) updateFirewallZone(
	ctx context.Context,
	site string,
	d *FirewallZone,
) (*FirewallZone, error) {
	var respBody FirewallZone
	err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("v2/api/site/%s/firewall/zone/%s", site, d.ID),
		d,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}
