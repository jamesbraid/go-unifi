package settings

import (
	"encoding/json"
	"testing"
)

// TestProviderCapabilitiesRoundTrip checks the site-level
// provider_capabilities setting (un)marshals correctly, using a payload
// shaped like a real controller response (neutral fixture IDs).
func TestProviderCapabilitiesRoundTrip(t *testing.T) {
	raw := `{
		"_id": "aaaa0000000000000000a003",
		"site_id": "bbbb0000000000000000b001",
		"key": "provider_capabilities",
		"download": 1000000,
		"upload": 1000000
	}`

	var s ProviderCapabilities
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.GetKey() != "provider_capabilities" {
		t.Errorf("GetKey() = %q, want provider_capabilities", s.GetKey())
	}
	if s.Download != 1000000 || s.Upload != 1000000 {
		t.Errorf("Download/Upload = %d/%d, want 1000000/1000000", s.Download, s.Upload)
	}

	if k, err := GetSettingKey(&s); err != nil || k != "provider_capabilities" {
		t.Errorf("GetSettingKey = (%q, %v), want (provider_capabilities, nil)", k, err)
	}

	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	// JSON numbers decode to float64 in a map[string]any.
	if back["key"] != "provider_capabilities" || back["download"] != float64(1000000) {
		t.Errorf("round-trip lost fields: %v", back)
	}
}
