package settings

import (
	"encoding/json"
	"testing"
)

// TestUsgGeoRoundTrip checks the site-level usg_geo (geo IP filtering)
// setting (un)marshals correctly, including the nested ip_filtering object,
// using a payload shaped like a real controller response (neutral fixture
// IDs).
func TestUsgGeoRoundTrip(t *testing.T) {
	raw := `{
		"_id": "aaaa0000000000000000a004",
		"site_id": "bbbb0000000000000000b001",
		"key": "usg_geo",
		"ip_filtering": {
			"action": "block",
			"countries": "KP,RU",
			"enabled": true,
			"traffic_direction": "both"
		}
	}`

	var s UsgGeo
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.GetKey() != "usg_geo" {
		t.Errorf("GetKey() = %q, want usg_geo", s.GetKey())
	}
	if s.IPFiltering == nil {
		t.Fatal("IPFiltering = nil, want populated")
	}
	if s.IPFiltering.Action != "block" || !s.IPFiltering.Enabled {
		t.Errorf("ip_filtering = %+v", s.IPFiltering)
	}
	if s.IPFiltering.Countries != "KP,RU" || s.IPFiltering.TrafficDirection != "both" {
		t.Errorf("ip_filtering = %+v", s.IPFiltering)
	}

	if k, err := GetSettingKey(&s); err != nil || k != "usg_geo" {
		t.Errorf("GetSettingKey = (%q, %v), want (usg_geo, nil)", k, err)
	}

	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	ipf, ok := back["ip_filtering"].(map[string]any)
	if !ok || back["key"] != "usg_geo" || ipf["action"] != "block" || ipf["enabled"] != true {
		t.Errorf("round-trip lost fields: %v", back)
	}
}
