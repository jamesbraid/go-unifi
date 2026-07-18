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
			{"mac": "d8:b3:70:11:a9:5c", "network_id": "681268c001e36a7836e21559", "querier_address": "192.0.2.1", "query_interval": 60},
			{"mac": "d8:b3:70:11:a9:5d", "network_id": "6813e64a4ee8cb0f1f486ac8", "querier_address": "192.0.2.2", "query_interval": "90"}
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
	if len(s.QuerierAddresses) != 2 || s.QuerierAddresses[0] != "192.0.2.1" || s.QuerierAddresses[1] != "192.0.2.2" {
		t.Errorf("QuerierAddresses = %#v", s.QuerierAddresses)
	}
	if len(s.QuerierAddressDetails) != 2 || s.QuerierAddressDetails[0].MAC != "d8:b3:70:11:a9:5c" || s.QuerierAddressDetails[0].NetworkID != "681268c001e36a7836e21559" || s.QuerierAddressDetails[0].QueryInterval == nil || *s.QuerierAddressDetails[0].QueryInterval != 60 || s.QuerierAddressDetails[1].QueryInterval == nil || *s.QuerierAddressDetails[1].QueryInterval != 90 {
		t.Errorf("QuerierAddressDetails = %#v", s.QuerierAddressDetails)
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
	addresses, ok := back["querier_addresses"].([]any)
	if !ok || len(addresses) != 2 {
		t.Errorf("round-trip querier_addresses = %#v", back["querier_addresses"])
	} else {
		first, firstOK := addresses[0].(map[string]any)
		second, secondOK := addresses[1].(map[string]any)
		if !firstOK || !secondOK || first["mac"] != "d8:b3:70:11:a9:5c" || first["network_id"] != "681268c001e36a7836e21559" || first["querier_address"] != "192.0.2.1" || first["query_interval"] != float64(60) || second["querier_address"] != "192.0.2.2" || second["query_interval"] != float64(90) {
			t.Errorf("round-trip querier_addresses = %#v", back["querier_addresses"])
		}
	}
}

func TestIgmpSnoopingLegacyQuerierAddressesRemainStrings(t *testing.T) {
	raw := `{"key":"igmp_snooping","querier_addresses":["192.0.2.1"]}`
	var setting IgmpSnooping
	if err := json.Unmarshal([]byte(raw), &setting); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(setting.QuerierAddressDetails) != 0 || len(setting.QuerierAddresses) != 1 || setting.QuerierAddresses[0] != "192.0.2.1" {
		t.Fatalf("decoded legacy addresses = %#v details=%#v", setting.QuerierAddresses, setting.QuerierAddressDetails)
	}
	body, err := json.Marshal(&setting)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	addresses, ok := back["querier_addresses"].([]any)
	if !ok || len(addresses) != 1 || addresses[0] != "192.0.2.1" {
		t.Fatalf("legacy round-trip addresses = %#v", back["querier_addresses"])
	}
}

func TestIgmpSnoopingOmitsUnsetQuerierAddresses(t *testing.T) {
	body, err := json.Marshal(&IgmpSnooping{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, exists := decoded["querier_addresses"]; exists {
		t.Fatalf("unset querier_addresses unexpectedly encoded: %s", body)
	}
}

func TestIgmpSnoopingLegacyAddressEditUpdatesStructuredWireShape(t *testing.T) {
	raw := `{"key":"igmp_snooping","querier_addresses":[{"mac":"d8:b3:70:11:a9:5c","network_id":"network-1","querier_address":"192.0.2.1","query_interval":60}]}`
	var setting IgmpSnooping
	if err := json.Unmarshal([]byte(raw), &setting); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	setting.QuerierAddresses[0] = "192.0.2.99"

	body, err := json.Marshal(&setting)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back struct {
		Addresses []IgmpSnoopingQuerierAddress `json:"querier_addresses"`
	}
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if len(back.Addresses) != 1 || back.Addresses[0].QuerierAddress != "192.0.2.99" || back.Addresses[0].MAC != "d8:b3:70:11:a9:5c" || back.Addresses[0].NetworkID != "network-1" || back.Addresses[0].QueryInterval == nil || *back.Addresses[0].QueryInterval != 60 {
		t.Fatalf("edited structured addresses = %#v", back.Addresses)
	}
}

func TestIgmpSnoopingLegacyAddressClearOmitsStaleStructuredWireShape(t *testing.T) {
	raw := `{"key":"igmp_snooping","querier_addresses":[{"mac":"d8:b3:70:11:a9:5c","querier_address":"192.0.2.1","query_interval":60}]}`
	var setting IgmpSnooping
	if err := json.Unmarshal([]byte(raw), &setting); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	setting.QuerierAddresses = nil

	body, err := json.Marshal(&setting)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if _, exists := back["querier_addresses"]; exists {
		t.Fatalf("cleared querier_addresses emitted stale details: %s", body)
	}
}
