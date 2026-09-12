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

// MarshalJSON fixes up the write shape of this type.
//
// Read-only fields are dropped: the controller reports them and rejects them
// on a write, so without this an update after a read fails on the
// server-assigned fields the read filled in.
//
// Slices marked nil-as-empty are sent as [] rather than null. They serialize
// unconditionally by design -- an empty list has to reach the wire to clear
// the value -- but a caller that never touched the field holds nil, and the
// controller rejects null where it expects an array.
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
		NetworkIDs    []string  `json:"network_ids"`
		*Alias
	}{
		NetworkIDs: emptyIfNil(src.NetworkIDs),
		Alias:      (*Alias)(&src),
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

func (c *ApiClient) ListFirewallZone(ctx context.Context, site string) ([]FirewallZone, error) {
	return bareList[FirewallZone](ctx, c, fmt.Sprintf("v2/api/site/%s/firewall/zone", site))
}

func (c *ApiClient) GetFirewallZone(ctx context.Context, site string, id string) (*FirewallZone, error) {
	stored, err := c.ListFirewallZone(ctx, site)
	if err != nil {
		return nil, err
	}
	return findByID(stored, id, func(v *FirewallZone) string { return v.ID })
}

func (c *ApiClient) DeleteFirewallZone(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("v2/api/site/%s/firewall/zone/%s", site, id))
}

func (c *ApiClient) CreateFirewallZone(ctx context.Context, site string, d *FirewallZone) (*FirewallZone, error) {
	return bareOne[FirewallZone](ctx, c, http.MethodPost, fmt.Sprintf("v2/api/site/%s/firewall/zone", site), d)
}

// UpdateFirewallZoneFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateFirewallZoneFields(ctx context.Context, site string, d *FirewallZone, fields ...string) (*FirewallZone, error) {
	return bareMasked(ctx, c, fmt.Sprintf("v2/api/site/%s/firewall/zone/%s", site, d.ID), d, fields)
}

func (c *ApiClient) UpdateFirewallZone(ctx context.Context, site string, d *FirewallZone) (*FirewallZone, error) {
	return bareOne[FirewallZone](ctx, c, http.MethodPut, fmt.Sprintf("v2/api/site/%s/firewall/zone/%s", site, d.ID), d)
}
