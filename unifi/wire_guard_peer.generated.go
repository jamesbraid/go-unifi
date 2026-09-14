// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"encoding/json"
	"fmt"
)

type WireGuardPeer struct {
	ID string `json:"_id,omitempty"`

	AllowedIPs  []string `json:"allowed_ips"`
	InterfaceIP string   `json:"interface_ip"`
	Name        string   `json:"name"`
	NetworkID   string   `json:"network_id,omitempty"`
	PublicKey   string   `json:"public_key"`
}

func (dst *WireGuardPeer) UnmarshalJSON(b []byte) error {
	type Alias WireGuardPeer
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
