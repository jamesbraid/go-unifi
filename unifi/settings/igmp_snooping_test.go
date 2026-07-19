package settings

import (
	"encoding/json"
	"testing"
)

// TestIgmpSnoopingRoundTrip checks the site-level igmp_snooping setting
// (un)marshals correctly, using a payload shaped like a real UniFi 10.x
// controller response. Guards ubiquiti-community/terraform-provider-unifi#164.
func TestIgmpSnoopingRoundTrip(t *testing.T) {
	raw := `{
		"_id": "69d1908dd5c33da485ee2ea2",
		"site_id": "681268bd01e36a7836e2153f",
		"key": "igmp_snooping",
		"enabled": true,
		"flood_known_protocols": true,
		"forward_unknown_mcast_router_ports": false,
		"subscription_mode": "ALL",
		"querier_mode": "CUSTOM",
		"querier_subscription_mode": "ALL",
		"querier_switches": ["d8:b3:70:11:a9:5c"],
		"querier_addresses": [
			{
				"mac": "d8:b3:70:11:a9:5c",
				"network_id": "681268c001e36a7836e21559",
				"querier_address": "192.168.1.2",
				"query_interval": "125"
			},
			{
				"mac": "d8:b3:70:22:b8:6d",
				"network_id": "6813e64a4ee8cb0f1f486ac8",
				"querier_address": "192.168.10.2",
				"query_interval": 60
			}
		],
		"network_ids": ["681268c001e36a7836e21559", "6813e64a4ee8cb0f1f486ac8"]
	}`

	var s IgmpSnooping
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.GetKey() != "igmp_snooping" {
		t.Errorf("GetKey() = %q, want igmp_snooping", s.GetKey())
	}
	if !s.Enabled {
		t.Error("Enabled = false, want true")
	}
	if len(s.NetworkIDs) != 2 || s.NetworkIDs[0] != "681268c001e36a7836e21559" {
		t.Errorf("NetworkIDs = %v", s.NetworkIDs)
	}
	if s.SubscriptionMode != "ALL" || s.QuerierMode != "CUSTOM" {
		t.Errorf("subscription_mode=%q querier_mode=%q", s.SubscriptionMode, s.QuerierMode)
	}

	// querier_addresses is a list of objects since controller 10.x. The shape
	// follows the 10.4.57 ace.jar schema and matches a live 10.4.57 console's
	// own frontend form schema ({mac, network_id, querier_address}; the live
	// array was empty, so element shape was confirmed from the UI model).
	// query_interval arrives as a JSON string in the first entry to exercise
	// the tolerant string-or-number decode.
	if len(s.QuerierAddresses) != 2 {
		t.Fatalf("QuerierAddresses = %v, want 2 entries", s.QuerierAddresses)
	}
	qa := s.QuerierAddresses[0]
	if qa.MAC != "d8:b3:70:11:a9:5c" || qa.NetworkID != "681268c001e36a7836e21559" || qa.QuerierAddress != "192.168.1.2" {
		t.Errorf("QuerierAddresses[0] = %+v", qa)
	}
	if qa.QueryInterval == nil || *qa.QueryInterval != 125 {
		t.Errorf("QuerierAddresses[0].QueryInterval = %v, want 125", qa.QueryInterval)
	}
	if qi := s.QuerierAddresses[1].QueryInterval; qi == nil || *qi != 60 {
		t.Errorf("QuerierAddresses[1].QueryInterval = %v, want 60", qi)
	}

	// GetSettingKey must resolve the type to the correct endpoint key.
	if k, err := GetSettingKey(&s); err != nil || k != "igmp_snooping" {
		t.Errorf("GetSettingKey = (%q, %v), want (igmp_snooping, nil)", k, err)
	}

	// Re-marshal and ensure key + enabled survive.
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if back["key"] != "igmp_snooping" || back["enabled"] != true {
		t.Errorf("round-trip lost fields: key=%v enabled=%v", back["key"], back["enabled"])
	}
	if addrs, ok := back["querier_addresses"].([]any); !ok || len(addrs) != 2 {
		t.Errorf("round-trip lost querier_addresses: %v", back["querier_addresses"])
	}
}
