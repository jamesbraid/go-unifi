package settings

import (
	"encoding/json"
	"testing"
)

func TestLegacySettingsRemainAddressable(t *testing.T) {
	tests := []struct {
		setting Setting
		want    string
	}{
		{setting: &EvaluationScore{}, want: "evaluation_score"},
		{setting: &RoamingAssistant{}, want: "roaming_assistant"},
	}
	for _, test := range tests {
		got, err := GetSettingKey(test.setting)
		if err != nil || got != test.want {
			t.Errorf("GetSettingKey(%T) = (%q, %v), want (%q, nil)", test.setting, got, err, test.want)
		}
	}
}

func TestLegacyRoamingAssistantAcceptsNumericRSSI(t *testing.T) {
	var setting RoamingAssistant
	if err := json.Unmarshal([]byte(`{"key":"roaming_assistant","rssi":-70}`), &setting); err != nil {
		t.Fatalf("unmarshal numeric RSSI: %v", err)
	}
	if setting.Rssi == nil || *setting.Rssi != -70 {
		t.Fatalf("Rssi = %v, want -70", setting.Rssi)
	}
}
