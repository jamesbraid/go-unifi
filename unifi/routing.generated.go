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

type Routing struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Enabled              bool   `json:"enabled"`
	GatewayDevice        string `json:"gateway_device,omitempty"`        // ^([0-9A-Fa-f]{2}[:]){5}([0-9A-Fa-f]{2})$
	GatewayType          string `json:"gateway_type,omitempty"`          // default|switch
	Name                 string `json:"name,omitempty"`                  // .{1,128}
	StaticRouteDistance  *int64 `json:"static-route_distance,omitempty"` // ^[1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]$|^$
	StaticRouteInterface string `json:"static-route_interface"`          // WAN[1-9]?|[\d\w-]+|^$
	StaticRouteNetwork   string `json:"static-route_network,omitempty"`  // ^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\/([1-9]|[1-2][0-9]|3[0-2])$|^([a-fA-F0-9:]+\/(([1-9]|[1-8][0-9]|9[0-9]|1[01][0-9]|12[0-8])))$
	StaticRouteNexthop   string `json:"static-route_nexthop"`            // ^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^([a-fA-F0-9:]+)$|^$
	StaticRouteType      string `json:"static-route_type,omitempty"`     // nexthop-route|interface-route|blackhole
	Type                 string `json:"type,omitempty"`                  // static-route
}

func (dst *Routing) UnmarshalJSON(b []byte) error {
	type Alias Routing
	aux := &struct {
		StaticRouteDistance *types.Number `json:"static-route_distance"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.StaticRouteDistance != nil {
		if val, err := aux.StaticRouteDistance.Int64(); err == nil {
			dst.StaticRouteDistance = &val
		} else if string(*aux.StaticRouteDistance) == "" {
			var zero int64
			dst.StaticRouteDistance = &zero
		}
	}

	return nil
}

func (c *ApiClient) ListRouting(ctx context.Context, site string) ([]Routing, error) {
	return envelopeList[Routing](ctx, c, fmt.Sprintf("api/s/%s/rest/routing", site))
}

func (c *ApiClient) GetRouting(ctx context.Context, site string, id string) (*Routing, error) {
	return envelopeOne[Routing](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/routing/%s", site, id), nil)
}

func (c *ApiClient) DeleteRouting(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/routing/%s", site, id))
}

func (c *ApiClient) CreateRouting(ctx context.Context, site string, d *Routing) (*Routing, error) {
	return envelopeOne[Routing](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/routing", site), d)
}

// UpdateRoutingFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateRoutingFields(ctx context.Context, site string, d *Routing, fields ...string) (*Routing, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/routing/%s", site, d.ID), d, fields, func() (*Routing, error) { return c.GetRouting(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateRouting(ctx context.Context, site string, d *Routing) (*Routing, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/routing/%s", site, d.ID), d, func() (*Routing, error) { return c.GetRouting(ctx, site, d.ID) })
}
