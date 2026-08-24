package settings

import (
	"encoding/json"
	"testing"
)

// TestGlobalNetworkRoundTrip checks the site-level global_network setting
// (un)marshals correctly, using a payload shaped like a real UniFi 9.x+
// controller response (neutral fixture IDs).
func TestGlobalNetworkRoundTrip(t *testing.T) {
	raw := `{
		"_id": "aaaa0000000000000000a001",
		"site_id": "bbbb0000000000000000b001",
		"key": "global_network",
		"default_security_posture": "ALLOW_ALL"
	}`

	var s GlobalNetwork
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.GetKey() != "global_network" {
		t.Errorf("GetKey() = %q, want global_network", s.GetKey())
	}
	if s.DefaultSecurityPosture != "ALLOW_ALL" {
		t.Errorf("DefaultSecurityPosture = %q, want ALLOW_ALL", s.DefaultSecurityPosture)
	}

	// GetSettingKey must resolve the type to the correct endpoint key.
	if k, err := GetSettingKey(&s); err != nil || k != "global_network" {
		t.Errorf("GetSettingKey = (%q, %v), want (global_network, nil)", k, err)
	}

	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if back["key"] != "global_network" || back["default_security_posture"] != "ALLOW_ALL" {
		t.Errorf("round-trip lost fields: %v", back)
	}
}
