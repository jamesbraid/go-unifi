// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package settings

import (
	"encoding/json"
	"fmt"
)

type MagicSiteToSiteVpn struct {
	BaseSetting

	Enabled bool `json:"enabled"`
}

func (dst *MagicSiteToSiteVpn) UnmarshalJSON(b []byte) error {
	type Alias MagicSiteToSiteVpn
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
