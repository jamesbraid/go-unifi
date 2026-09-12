// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package settings

import (
	"encoding/json"
	"fmt"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

type Country struct {
	BaseSetting

	Code *int64 `json:"code,omitempty"`
}

func (dst *Country) UnmarshalJSON(b []byte) error {
	type Alias Country
	aux := &struct {
		Code *types.Number `json:"code"`

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
	if aux.Code != nil {
		if val, err := aux.Code.Int64(); err == nil {
			dst.Code = &val
		} else if string(*aux.Code) == "" {
			var zero int64
			dst.Code = &zero
		}
	}

	return nil
}
