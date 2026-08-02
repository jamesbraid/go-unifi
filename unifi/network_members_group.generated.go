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

type NetworkMembersGroup struct {
	ID string `json:"id,omitempty"`

	Members []string `json:"members"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
}

func (dst *NetworkMembersGroup) UnmarshalJSON(b []byte) error {
	type Alias NetworkMembersGroup
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

func (c *ApiClient) listNetworkMembersGroup(
	ctx context.Context,
	site string,
	query ...map[string]string,
) ([]NetworkMembersGroup, error) {
	var respBody []NetworkMembersGroup

	err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("v2/api/site/%s/network-members-groups", site),
		nil,
		&respBody,
		query...,
	)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

func (c *ApiClient) getNetworkMembersGroup(
	ctx context.Context,
	site string,
	id string,
) (*NetworkMembersGroup, error) {
	respBody, err := c.listNetworkMembersGroup(ctx, site)
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

func (c *ApiClient) deleteNetworkMembersGroup(
	ctx context.Context,
	site string,
	id string,
) error {
	err := c.do(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("v2/api/site/%s/network-members-groups/%s", site, id),
		struct{}{},
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *ApiClient) createNetworkMembersGroup(
	ctx context.Context,
	site string,
	d *NetworkMembersGroup,
) (*NetworkMembersGroup, error) {
	var respBody NetworkMembersGroup

	err := c.do(
		ctx,
		http.MethodPost,
		fmt.Sprintf("v2/api/site/%s/network-members-groups", site),
		d,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

// UpdateNetworkMembersGroupFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
func (c *ApiClient) UpdateNetworkMembersGroupFields(ctx context.Context, site string, d *NetworkMembersGroup, fields ...string) (*NetworkMembersGroup, error) {
	return c.updateNetworkMembersGroupFields(ctx, site, d, fields)
}

// updateNetworkMembersGroupFields writes only the named wire fields, leaving
// every other field on the stored object alone. See maskedBody.
func (c *ApiClient) updateNetworkMembersGroupFields(
	ctx context.Context,
	site string,
	d *NetworkMembersGroup,
	fields []string,
) (*NetworkMembersGroup, error) {
	body, err := maskedBody(d, fields)
	if err != nil {
		return nil, err
	}
	var respBody NetworkMembersGroup
	if err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("v2/api/site/%s/network-members-groups/%s", site, d.ID),
		body,
		&respBody,
	); err != nil {
		return nil, err
	}

	return &respBody, nil
}

func (c *ApiClient) updateNetworkMembersGroup(
	ctx context.Context,
	site string,
	d *NetworkMembersGroup,
) (*NetworkMembersGroup, error) {
	var respBody NetworkMembersGroup
	err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("v2/api/site/%s/network-members-groups/%s", site, d.ID),
		d,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}
