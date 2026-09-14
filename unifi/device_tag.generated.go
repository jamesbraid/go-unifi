// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"encoding/json"
	"fmt"
)

type DeviceTag struct {
	ID string `json:"_id,omitempty"`

	MemberDeviceMacs []string `json:"member_device_macs"`
	Name             string   `json:"name"`
}

func (dst *DeviceTag) UnmarshalJSON(b []byte) error {
	type Alias DeviceTag
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
