// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package settings

import (
	"encoding/json"
	"fmt"
)

type EtherLighting struct {
	BaseSetting

	NetworkOverrides []SettingEtherLightingNetworkOverrides `json:"network_overrides,omitempty"`
	SpeedOverrides   []SettingEtherLightingSpeedOverrides   `json:"speed_overrides,omitempty"`
}

func (dst *EtherLighting) UnmarshalJSON(b []byte) error {
	type Alias EtherLighting
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	// First unmarshal base setting
	if err := json.Unmarshal(b, &dst.BaseSetting); err != nil {
		return fmt.Errorf("unable to unmarshal base setting: %w", err)
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}

	return nil
}

type SettingEtherLightingNetworkOverrides struct {
	Key         string `json:"key,omitempty"`
	RawColorHex string `json:"raw_color_hex,omitempty"` // [0-9A-Fa-f]{6}
}

func (dst *SettingEtherLightingNetworkOverrides) UnmarshalJSON(b []byte) error {
	type Alias SettingEtherLightingNetworkOverrides
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

type SettingEtherLightingSpeedOverrides struct {
	Key         string `json:"key,omitempty"`           // FE|GbE|2.5GbE|5GbE|10GbE|25GbE|40GbE|100GbE
	RawColorHex string `json:"raw_color_hex,omitempty"` // [0-9A-Fa-f]{6}
}

func (dst *SettingEtherLightingSpeedOverrides) UnmarshalJSON(b []byte) error {
	type Alias SettingEtherLightingSpeedOverrides
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
