package settings

import (
	"encoding/json"
	"fmt"
)

// Setting is the interface that all setting types must implement.
type Setting interface {
	GetKey() string
	SetKey(key string)
}

// BaseSetting contains common fields for all settings.
type BaseSetting struct {
	ID       string `json:"_id,omitempty"`
	SiteID   string `json:"site_id,omitempty"`
	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`
	Key      string `json:"key"`
}

// GetKey returns the setting key.
func (b *BaseSetting) GetKey() string {
	return b.Key
}

// SetKey sets the setting key.
func (b *BaseSetting) SetKey(key string) {
	b.Key = key
}

// RawSetting represents a generic setting when the specific type is unknown.
type RawSetting struct {
	BaseSetting
	Data map[string]any `json:"-"`
}

// UnmarshalJSON implements custom unmarshaling for RawSetting.
func (r *RawSetting) UnmarshalJSON(b []byte) error {
	// First unmarshal into BaseSetting
	if err := json.Unmarshal(b, &r.BaseSetting); err != nil {
		return err
	}

	// Then unmarshal the full data
	if err := json.Unmarshal(b, &r.Data); err != nil {
		return err
	}

	return nil
}

// MarshalJSON implements custom marshaling for RawSetting.
func (r *RawSetting) MarshalJSON() ([]byte, error) {
	// Merge base setting fields into data map
	data := make(map[string]any)

	for k, v := range r.Data {
		data[k] = v
	}

	// Override with base setting fields
	baseBytes, err := json.Marshal(r.BaseSetting)
	if err != nil {
		return nil, err
	}

	var baseData map[string]any
	if err := json.Unmarshal(baseBytes, &baseData); err != nil {
		return nil, err
	}

	for k, v := range baseData {
		data[k] = v
	}

	return json.Marshal(data)
}

// GetSettingKey returns the key name for a specific setting type
// This is used internally to determine which endpoint to call.
func GetSettingKey(setting Setting) (string, error) {
	if s, ok := setting.(*RawSetting); ok {
		// For raw settings, use the key from the data
		if s.Key != "" {
			return s.Key, nil
		}
		return "", fmt.Errorf("raw setting has no key")
	}
	// These settings were exposed by older UniFi releases and remain part of
	// the public Go API even though the current schema no longer defines them.
	switch setting.(type) {
	case *EvaluationScore:
		return "evaluation_score", nil
	case *RoamingAssistant:
		return "roaming_assistant", nil
	}
	if key, ok := generatedSettingKey(setting); ok {
		return key, nil
	}
	return "", fmt.Errorf("unknown setting type: %T", setting)
}
