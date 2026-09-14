// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"encoding/json"
	"fmt"
)

type Site struct {
	ID string `json:"_id,omitempty"`

	Description string `json:"desc"`
	Name        string `json:"name"`
}

func (dst *Site) UnmarshalJSON(b []byte) error {
	type Alias Site
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
