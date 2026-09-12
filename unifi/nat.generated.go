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

type Nat struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Description           string                `json:"description,omitempty"`
	DestinationFilter     *NatDestinationFilter `json:"destination_filter,omitempty"`
	Enabled               bool                  `json:"enabled"`
	Exclude               bool                  `json:"exclude"`
	IPAddress             string                `json:"ip_address,omitempty"`
	InInterface           string                `json:"in_interface,omitempty"`
	IsPredefined          bool                  `json:"is_predefined"`
	Logging               bool                  `json:"logging"`
	OutInterface          string                `json:"out_interface,omitempty"`
	Port                  *int64                `json:"port,omitempty"` // [1-9][0-9]{0,4}
	PppoeUseBaseInterface bool                  `json:"pppoe_use_base_interface"`
	Protocol              string                `json:"protocol"` // all|tcp|udp|tcp_udp
	RuleIndex             *int64                `json:"rule_index,omitempty"`
	SettingPreference     string                `json:"setting_preference,omitempty"` // auto|manual
	SourceFilter          *NatSourceFilter      `json:"source_filter,omitempty"`
	Type                  string                `json:"type,omitempty"`       // DNAT|SNAT|MASQUERADE
	Version               string                `json:"ip_version,omitempty"` // IPV4|IPV6
}

func (dst *Nat) UnmarshalJSON(b []byte) error {
	type Alias Nat
	aux := &struct {
		Port      *types.Number `json:"port"`
		RuleIndex *types.Number `json:"rule_index"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.Port != nil {
		if val, err := aux.Port.Int64(); err == nil {
			dst.Port = &val
		} else if string(*aux.Port) == "" {
			var zero int64
			dst.Port = &zero
		}
	}
	if aux.RuleIndex != nil {
		if val, err := aux.RuleIndex.Int64(); err == nil {
			dst.RuleIndex = &val
		} else if string(*aux.RuleIndex) == "" {
			var zero int64
			dst.RuleIndex = &zero
		}
	}

	return nil
}

type NatDestinationFilter struct {
	Address          string   `json:"address,omitempty"`
	FilterType       string   `json:"filter_type,omitempty"` // NONE|ADDRESS_AND_PORT|FIREWALL_GROUPS|NETWORK_CONF|IID_AND_PORT
	FirewallGroupIDs []string `json:"firewall_group_ids,omitempty"`
	Iid              string   `json:"iid,omitempty"`
	InvertAddress    bool     `json:"invert_address"`
	InvertPort       bool     `json:"invert_port"`
	NetworkConfID    string   `json:"network_conf_id,omitempty"`
	Port             *int64   `json:"port,omitempty"` // [1-9][0-9]{0,4}
}

func (dst *NatDestinationFilter) UnmarshalJSON(b []byte) error {
	type Alias NatDestinationFilter
	aux := &struct {
		Port *types.Number `json:"port"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.Port != nil {
		if val, err := aux.Port.Int64(); err == nil {
			dst.Port = &val
		} else if string(*aux.Port) == "" {
			var zero int64
			dst.Port = &zero
		}
	}

	return nil
}

type NatSourceFilter struct {
	Address          string   `json:"address,omitempty"`
	FilterType       string   `json:"filter_type,omitempty"` // NONE|ADDRESS_AND_PORT|FIREWALL_GROUPS|NETWORK_CONF|IID_AND_PORT
	FirewallGroupIDs []string `json:"firewall_group_ids,omitempty"`
	Iid              string   `json:"iid,omitempty"`
	InvertAddress    bool     `json:"invert_address"`
	InvertPort       bool     `json:"invert_port"`
	NetworkConfID    string   `json:"network_conf_id,omitempty"`
	Port             *int64   `json:"port,omitempty"` // [1-9][0-9]{0,4}
}

func (dst *NatSourceFilter) UnmarshalJSON(b []byte) error {
	type Alias NatSourceFilter
	aux := &struct {
		Port *types.Number `json:"port"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.Port != nil {
		if val, err := aux.Port.Int64(); err == nil {
			dst.Port = &val
		} else if string(*aux.Port) == "" {
			var zero int64
			dst.Port = &zero
		}
	}

	return nil
}

func (c *ApiClient) listNat(ctx context.Context, site string, query ...map[string]string) ([]Nat, error) {
	return bareList[Nat](ctx, c, fmt.Sprintf("v2/api/site/%s/nat", site), query...)
}

func (c *ApiClient) GetNat(ctx context.Context, site string, id string) (*Nat, error) {
	stored, err := c.listNat(ctx, site)
	if err != nil {
		return nil, err
	}
	return findByID(stored, id, func(v *Nat) string { return v.ID })
}

func (c *ApiClient) DeleteNat(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("v2/api/site/%s/nat/%s", site, id))
}

func (c *ApiClient) CreateNat(ctx context.Context, site string, d *Nat) (*Nat, error) {
	return bareOne[Nat](ctx, c, http.MethodPost, fmt.Sprintf("v2/api/site/%s/nat", site), d)
}

// UpdateNatFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateNatFields(ctx context.Context, site string, d *Nat, fields ...string) (*Nat, error) {
	return bareMasked(ctx, c, fmt.Sprintf("v2/api/site/%s/nat/%s", site, d.ID), d, fields)
}

func (c *ApiClient) UpdateNat(ctx context.Context, site string, d *Nat) (*Nat, error) {
	return bareOne[Nat](ctx, c, http.MethodPut, fmt.Sprintf("v2/api/site/%s/nat/%s", site, d.ID), d)
}
