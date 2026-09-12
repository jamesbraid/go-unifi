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

type ChannelPlan struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Date       string                  `json:"date"` // ^$|^(20[0-9]{2}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9])Z?$
	RadioTable []ChannelPlanRadioTable `json:"radio_table,omitempty"`
}

func (dst *ChannelPlan) UnmarshalJSON(b []byte) error {
	type Alias ChannelPlan
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

type ChannelPlanRadioTable struct {
	Channel     string `json:"channel,omitempty"`       // [0-9]|[1][0-4]|16|34|36|38|40|42|44|46|48|52|56|60|64|100|104|108|112|116|120|124|128|132|136|140|144|149|153|157|161|165|183|184|185|187|188|189|192|196|auto
	DeviceMAC   string `json:"device_mac,omitempty"`    // ^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$
	Name        string `json:"name,omitempty"`          // [a-z]*[0-9]*
	TxPower     string `json:"tx_power,omitempty"`      // [\d]+|auto
	TxPowerMode string `json:"tx_power_mode,omitempty"` // auto|medium|high|low|custom
	Width       *int64 `json:"width,omitempty"`         // 20|40|80|160
}

func (dst *ChannelPlanRadioTable) UnmarshalJSON(b []byte) error {
	type Alias ChannelPlanRadioTable
	aux := &struct {
		Channel types.Number  `json:"channel"`
		TxPower types.Number  `json:"tx_power"`
		Width   *types.Number `json:"width"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	dst.Channel = aux.Channel.String()
	dst.TxPower = aux.TxPower.String()
	if aux.Width != nil {
		if val, err := aux.Width.Int64(); err == nil {
			dst.Width = &val
		} else if string(*aux.Width) == "" {
			var zero int64
			dst.Width = &zero
		}
	}

	return nil
}

func (c *ApiClient) ListChannelPlan(ctx context.Context, site string) ([]ChannelPlan, error) {
	return envelopeList[ChannelPlan](ctx, c, fmt.Sprintf("api/s/%s/rest/channelplan", site))
}

func (c *ApiClient) GetChannelPlan(ctx context.Context, site string, id string) (*ChannelPlan, error) {
	return envelopeOne[ChannelPlan](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/channelplan/%s", site, id), nil)
}

func (c *ApiClient) DeleteChannelPlan(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/channelplan/%s", site, id))
}

func (c *ApiClient) CreateChannelPlan(ctx context.Context, site string, d *ChannelPlan) (*ChannelPlan, error) {
	return envelopeOne[ChannelPlan](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/channelplan", site), d)
}

// UpdateChannelPlanFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateChannelPlanFields(ctx context.Context, site string, d *ChannelPlan, fields ...string) (*ChannelPlan, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/channelplan/%s", site, d.ID), d, fields, func() (*ChannelPlan, error) { return c.GetChannelPlan(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateChannelPlan(ctx context.Context, site string, d *ChannelPlan) (*ChannelPlan, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/channelplan/%s", site, d.ID), d, func() (*ChannelPlan, error) { return c.GetChannelPlan(ctx, site, d.ID) })
}
