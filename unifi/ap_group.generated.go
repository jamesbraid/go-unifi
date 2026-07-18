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
