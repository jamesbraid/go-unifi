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

type BroadcastGroup struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	MemberTable []string `json:"member_table,omitempty"`
	Name        string   `json:"name,omitempty"`
}

func (dst *BroadcastGroup) UnmarshalJSON(b []byte) error {
	type Alias BroadcastGroup
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

func (c *ApiClient) ListBroadcastGroup(
	ctx context.Context,
	site string,
) ([]BroadcastGroup, error) {
	var respBody struct {
		Meta meta             `json:"meta"`
		Data []BroadcastGroup `json:"data"`
	}

	err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("api/s/%s/rest/broadcastgroup", site),
		nil,
		&respBody,
	)
	if err != nil {
		return nil, err
	}
	return respBody.Data, nil
}

func (c *ApiClient) GetBroadcastGroup(
	ctx context.Context,
	site string,
	id string,
) (*BroadcastGroup, error) {
	var respBody struct {
		Meta meta             `json:"meta"`
		Data []BroadcastGroup `json:"data"`
	}
	err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("api/s/%s/rest/broadcastgroup/%s", site, id),
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

func (c *ApiClient) DeleteBroadcastGroup(
	ctx context.Context,
	site string,
	id string,
) error {
	err := c.do(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("api/s/%s/rest/broadcastgroup/%s", site, id),
		struct{}{},
		nil,
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *ApiClient) CreateBroadcastGroup(
	ctx context.Context,
	site string,
	d *BroadcastGroup,
) (*BroadcastGroup, error) {
	var respBody struct {
		Meta meta             `json:"meta"`
		Data []BroadcastGroup `json:"data"`
	}

	err := c.do(
		ctx,
		http.MethodPost,
		fmt.Sprintf("api/s/%s/rest/broadcastgroup", site),
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

// UpdateBroadcastGroupFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateBroadcastGroupFields(
	ctx context.Context,
	site string,
	d *BroadcastGroup,
	fields ...string,
) (*BroadcastGroup, error) {
	body, err := maskedBody(d, fields)
	if err != nil {
		return nil, err
	}
	var respBody struct {
		Meta meta             `json:"meta"`
		Data []BroadcastGroup `json:"data"`
	}
	if err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("api/s/%s/rest/broadcastgroup/%s", site, d.ID),
		body,
		&respBody,
	); err != nil {
		return nil, err
	}

	if len(respBody.Data) == 0 {
		return c.GetBroadcastGroup(ctx, site, d.ID)
	}
	if len(respBody.Data) != 1 {
		return nil, &NotFoundError{}
	}
	res := respBody.Data[0]
	return &res, nil
}

func (c *ApiClient) UpdateBroadcastGroup(
	ctx context.Context,
	site string,
	d *BroadcastGroup,
) (*BroadcastGroup, error) {
	var respBody struct {
		Meta meta             `json:"meta"`
		Data []BroadcastGroup `json:"data"`
	}
	err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("api/s/%s/rest/broadcastgroup/%s", site, d.ID),
		d,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	// UDM SE API returns empty data array on successful PUT.
	// In that case, fetch the updated resource via GET.
	if len(respBody.Data) == 0 {
		return c.GetBroadcastGroup(ctx, site, d.ID)
	}

	if len(respBody.Data) != 1 {
		return nil, &NotFoundError{}
	}

	res := respBody.Data[0]

	return &res, nil
}
