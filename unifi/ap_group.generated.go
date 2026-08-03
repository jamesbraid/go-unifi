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

func (c *ApiClient) listAPGroup(
	ctx context.Context,
	site string,
	query ...map[string]string,
) ([]APGroup, error) {
	var respBody []APGroup

	err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("v2/api/site/%s/apgroups", site),
		nil,
		&respBody,
		query...,
	)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

func (c *ApiClient) getAPGroup(
	ctx context.Context,
	site string,
	id string,
) (*APGroup, error) {
	respBody, err := c.listAPGroup(ctx, site)
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

func (c *ApiClient) deleteAPGroup(
	ctx context.Context,
	site string,
	id string,
) error {
	err := c.do(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("v2/api/site/%s/apgroups/%s", site, id),
		struct{}{},
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *ApiClient) createAPGroup(
	ctx context.Context,
	site string,
	d *APGroup,
) (*APGroup, error) {
	var respBody APGroup

	err := c.do(
		ctx,
		http.MethodPost,
		fmt.Sprintf("v2/api/site/%s/apgroups", site),
		d,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

// UpdateAPGroupFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
func (c *ApiClient) UpdateAPGroupFields(ctx context.Context, site string, d *APGroup, fields ...string) (*APGroup, error) {
	return c.updateAPGroupFields(ctx, site, d, fields)
}

// updateAPGroupFields writes only the named wire fields, leaving
// every other field on the stored object alone. See maskedBody.
func (c *ApiClient) updateAPGroupFields(
	ctx context.Context,
	site string,
	d *APGroup,
	fields []string,
) (*APGroup, error) {
	body, err := maskedBody(d, fields)
	if err != nil {
		return nil, err
	}
	var respBody APGroup
	if err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("v2/api/site/%s/apgroups/%s", site, d.ID),
		body,
		&respBody,
	); err != nil {
		return nil, err
	}

	return &respBody, nil
}

func (c *ApiClient) updateAPGroup(
	ctx context.Context,
	site string,
	d *APGroup,
) (*APGroup, error) {
	var respBody APGroup
	err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("v2/api/site/%s/apgroups/%s", site, d.ID),
		d,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}
