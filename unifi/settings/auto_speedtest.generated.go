// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package settings

import (
	"encoding/json"
	"fmt"
)

type AutoSpeedtest struct {
	BaseSetting

	CronExpr      string   `json:"cron_expr,omitempty"`
	Enabled       bool     `json:"enabled"`
	SpeedTestMode string   `json:"speed_test_mode,omitempty"` // ALL|CUSTOM
	WANList       []string `json:"wan_list,omitempty"`        // WAN[2-9]?
}

func (dst *AutoSpeedtest) UnmarshalJSON(b []byte) error {
	type Alias AutoSpeedtest
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
