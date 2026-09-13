// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

type TrafficRoute struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Description       string                      `json:"description,omitempty"` // .{0,128}
	Domains           []TrafficRouteDomains       `json:"domains,omitempty"`
	Enabled           bool                        `json:"enabled"`
	IPAddresses       []TrafficRouteIPAddresses   `json:"ip_addresses,omitempty"`
	IPRanges          []TrafficRouteIPRanges      `json:"ip_ranges,omitempty"`
	KillSwitchEnabled bool                        `json:"kill_switch_enabled"`
	MatchingTarget    string                      `json:"matching_target"` // DOMAIN|IP|INTERNET|REGION
	NetworkID         string                      `json:"network_id"`
	NextHop           string                      `json:"next_hop,omitempty"`
	Regions           []string                    `json:"regions,omitempty"`
	TargetDevices     []TrafficRouteTargetDevices `json:"target_devices,omitempty"`
}

func (dst *TrafficRoute) UnmarshalJSON(b []byte) error {
	type Alias TrafficRoute
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

type TrafficRouteDomains struct {
	Domain     string                   `json:"domain,omitempty"` // .{1,256}
	PortRanges []TrafficRoutePortRanges `json:"port_ranges,omitempty"`
	Ports      []int64                  `json:"ports,omitempty"` // [1-9][0-9]{0,4}
}

func (dst *TrafficRouteDomains) UnmarshalJSON(b []byte) error {
	type Alias TrafficRouteDomains
	aux := &struct {
		Ports []types.Number `json:"ports"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	dst.Ports = make([]int64, len(aux.Ports))
	for i, v := range aux.Ports {
		if val, err := v.Int64(); err == nil {
			dst.Ports[i] = val
		}
	}

	return nil
}

type TrafficRouteIPAddresses struct {
	Address    string                   `json:"ip_or_subnet,omitempty"`
	PortRanges []TrafficRoutePortRanges `json:"port_ranges,omitempty"`
	Ports      []int64                  `json:"ports,omitempty"`      // [1-9][0-9]{0,4}
	Version    string                   `json:"ip_version,omitempty"` // v4|v6
}

func (dst *TrafficRouteIPAddresses) UnmarshalJSON(b []byte) error {
	type Alias TrafficRouteIPAddresses
	aux := &struct {
		Ports []types.Number `json:"ports"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	dst.Ports = make([]int64, len(aux.Ports))
	for i, v := range aux.Ports {
		if val, err := v.Int64(); err == nil {
			dst.Ports[i] = val
		}
	}

	return nil
}

type TrafficRouteIPRanges struct {
	Start   string `json:"ip_start,omitempty"`
	Stop    string `json:"ip_stop,omitempty"`
	Version string `json:"ip_version,omitempty"` // v4|v6
}

func (dst *TrafficRouteIPRanges) UnmarshalJSON(b []byte) error {
	type Alias TrafficRouteIPRanges
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

type TrafficRoutePortRanges struct {
	Start *int64 `json:"port_start,omitempty"` // [1-9][0-9]{0,4}
	Stop  *int64 `json:"port_stop,omitempty"`  // [1-9][0-9]{0,4}
}

func (dst *TrafficRoutePortRanges) UnmarshalJSON(b []byte) error {
	type Alias TrafficRoutePortRanges
	aux := &struct {
		Start *types.Number `json:"port_start"`
		Stop  *types.Number `json:"port_stop"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.Start != nil {
		if val, err := aux.Start.Int64(); err == nil {
			dst.Start = &val
		} else if string(*aux.Start) == "" {
			var zero int64
			dst.Start = &zero
		}
	}
	if aux.Stop != nil {
		if val, err := aux.Stop.Int64(); err == nil {
			dst.Stop = &val
		} else if string(*aux.Stop) == "" {
			var zero int64
			dst.Stop = &zero
		}
	}

	return nil
}

type TrafficRouteTargetDevices struct {
	ClientMAC string `json:"client_mac,omitempty"`
	NetworkID string `json:"network_id"`
	Type      string `json:"type,omitempty"` // ALL_CLIENTS|CLIENT|NETWORK
}

func (dst *TrafficRouteTargetDevices) UnmarshalJSON(b []byte) error {
	type Alias TrafficRouteTargetDevices
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

func (c *ApiClient) ListTrafficRoute(ctx context.Context, site string) ([]TrafficRoute, error) {
	return bareList[TrafficRoute](ctx, c, fmt.Sprintf("v2/api/site/%s/trafficroutes", site))
}

func (c *ApiClient) GetTrafficRoute(ctx context.Context, site string, id string) (*TrafficRoute, error) {
	stored, err := c.ListTrafficRoute(ctx, site)
	if err != nil {
		return nil, err
	}
	return findByID(stored, id, func(v *TrafficRoute) string { return v.ID })
}

func (c *ApiClient) DeleteTrafficRoute(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("v2/api/site/%s/trafficroutes/%s", site, id))
}

func (c *ApiClient) CreateTrafficRoute(ctx context.Context, site string, d *TrafficRoute) (*TrafficRoute, error) {
	return bareOne[TrafficRoute](ctx, c, http.MethodPost, fmt.Sprintf("v2/api/site/%s/trafficroutes", site), d)
}

// UpdateTrafficRouteFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateTrafficRouteFields(ctx context.Context, site string, d *TrafficRoute, fields ...string) (*TrafficRoute, error) {
	return bareMasked(ctx, c, fmt.Sprintf("v2/api/site/%s/trafficroutes/%s", site, d.ID), d, fields)
}

func (c *ApiClient) UpdateTrafficRoute(ctx context.Context, site string, d *TrafficRoute) (*TrafficRoute, error) {
	return bareOne[TrafficRoute](ctx, c, http.MethodPut, fmt.Sprintf("v2/api/site/%s/trafficroutes/%s", site, d.ID), d)
}
