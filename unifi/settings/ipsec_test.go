package settings

import (
	"encoding/json"
	"testing"
)

// TestIpsecRoundTrip checks the site-level ipsec setting (un)marshals
// correctly, using a payload shaped like a real UniFi 9.x+ controller
// response (neutral fixture IDs).
func TestIpsecRoundTrip(t *testing.T) {
	raw := `{
		"_id": "aaaa0000000000000000a002",
		"site_id": "bbbb0000000000000000b001",
		"key": "ipsec",
		"ikev2_reauthentication_method": "make-before-break"
	}`

	var s Ipsec
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.GetKey() != "ipsec" {
		t.Errorf("GetKey() = %q, want ipsec", s.GetKey())
	}
	if s.Ikev2ReauthenticationMethod != "make-before-break" {
		t.Errorf("Ikev2ReauthenticationMethod = %q", s.Ikev2ReauthenticationMethod)
	}

	if k, err := GetSettingKey(&s); err != nil || k != "ipsec" {
		t.Errorf("GetSettingKey = (%q, %v), want (ipsec, nil)", k, err)
	}

	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if back["key"] != "ipsec" || back["ikev2_reauthentication_method"] != "make-before-break" {
		t.Errorf("round-trip lost fields: %v", back)
	}
}
