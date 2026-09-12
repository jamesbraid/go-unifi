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

type APGroup struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	DeviceMacs  []string `json:"device_macs"`
	ForWLANconf bool     `json:"for_wlanconf"`
	Name        string   `json:"name"`
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
func (src APGroup) MarshalJSON() ([]byte, error) {
	type Alias APGroup
	return json.Marshal(&struct {
		HiddenID   *struct{} `json:"attr_hidden_id,omitempty"`
		NoDelete   *struct{} `json:"attr_no_delete,omitempty"`
		DeviceMacs []string  `json:"device_macs"`
		*Alias
	}{
		DeviceMacs: emptyIfNil(src.DeviceMacs),
		Alias:      (*Alias)(&src),
	})
}

func (dst *APGroup) UnmarshalJSON(b []byte) error {
	type Alias APGroup
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

func (c *ApiClient) ListAPGroup(ctx context.Context, site string) ([]APGroup, error) {
	return bareList[APGroup](ctx, c, fmt.Sprintf("v2/api/site/%s/apgroups", site))
}

func (c *ApiClient) GetAPGroup(ctx context.Context, site string, id string) (*APGroup, error) {
	stored, err := c.ListAPGroup(ctx, site)
	if err != nil {
		return nil, err
	}
	return findByID(stored, id, func(v *APGroup) string { return v.ID })
}

func (c *ApiClient) DeleteAPGroup(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("v2/api/site/%s/apgroups/%s", site, id))
}

func (c *ApiClient) CreateAPGroup(ctx context.Context, site string, d *APGroup) (*APGroup, error) {
	return bareOne[APGroup](ctx, c, http.MethodPost, fmt.Sprintf("v2/api/site/%s/apgroups", site), d)
}

// UpdateAPGroupFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateAPGroupFields(ctx context.Context, site string, d *APGroup, fields ...string) (*APGroup, error) {
	return bareMasked(ctx, c, fmt.Sprintf("v2/api/site/%s/apgroups/%s", site, d.ID), d, fields)
}

func (c *ApiClient) UpdateAPGroup(ctx context.Context, site string, d *APGroup) (*APGroup, error) {
	return bareOne[APGroup](ctx, c, http.MethodPut, fmt.Sprintf("v2/api/site/%s/apgroups/%s", site, d.ID), d)
}
