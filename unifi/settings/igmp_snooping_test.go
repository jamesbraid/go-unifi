package settings

import (
	"encoding/json"
	"testing"
)

// TestIgmpSnoopingLegacyQuerierAddresses covers the pre-10.x wire shape:
// querier_addresses as plain address strings (also seen in databases
// upgraded from old controllers). The tolerant decode maps each string to
// an entry with only QuerierAddress set.
func TestIgmpSnoopingLegacyQuerierAddresses(t *testing.T) {
	raw := `{
		"key": "igmp_snooping",
		"enabled": true,
		"querier_addresses": ["10.0.0.1", "10.0.0.2"]
	}`

	var s IgmpSnooping
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal legacy shape: %v", err)
	}
	if len(s.QuerierAddresses) != 2 {
		t.Fatalf("QuerierAddresses = %v, want 2 entries", s.QuerierAddresses)
	}
	if s.QuerierAddresses[0].QuerierAddress != "10.0.0.1" || s.QuerierAddresses[1].QuerierAddress != "10.0.0.2" {
		t.Errorf("addresses = %v", s.QuerierAddresses)
	}
	if s.QuerierAddresses[0].MAC != "" {
		t.Errorf("legacy entries should carry only the address, got %+v", s.QuerierAddresses[0])
	}

	// null elements are skipped, not turned into empty entries; legacy
	// strings re-marshal as objects (normalization is one-way by design).
	var nulls IgmpSnooping
	if err := json.Unmarshal([]byte(`{"key":"igmp_snooping","querier_addresses":[null,"10.0.0.3"]}`), &nulls); err != nil {
		t.Fatalf("unmarshal with null: %v", err)
	}
	if len(nulls.QuerierAddresses) != 1 || nulls.QuerierAddresses[0].QuerierAddress != "10.0.0.3" {
		t.Errorf("null handling: %v", nulls.QuerierAddresses)
	}
}

// TestIgmpSnoopingRoundTrip checks the site-level igmp_snooping setting
// (un)marshals correctly, using a payload shaped like a real UniFi 10.x
// controller response. Guards ubiquiti-community/terraform-provider-unifi#164.
func TestIgmpSnoopingRoundTrip(t *testing.T) {
	raw := `{
		"_id": "000000000000000000000a01",
		"site_id": "000000000000000000000b01",
		"key": "igmp_snooping",
		"enabled": true,
		"flood_known_protocols": true,
		"forward_unknown_mcast_router_ports": false,
		"subscription_mode": "ALL",
		"querier_mode": "CUSTOM",
		"querier_subscription_mode": "ALL",
		"querier_switches": ["02:00:00:00:00:01"],
		"querier_addresses": [
			{
				"mac": "02:00:00:00:00:01",
				"network_id": "000000000000000000000c01",
				"querier_address": "192.168.1.2",
				"query_interval": "125"
			},
			{
				"mac": "02:00:00:00:00:02",
				"network_id": "000000000000000000000c02",
				"querier_address": "192.168.10.2",
				"query_interval": 60
			}
		],
		"network_ids": ["000000000000000000000c01", "000000000000000000000c02"]
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
	if len(s.NetworkIDs) != 2 || s.NetworkIDs[0] != "000000000000000000000c01" {
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
	if qa.MAC != "02:00:00:00:00:01" || qa.NetworkID != "000000000000000000000c01" || qa.QuerierAddress != "192.168.1.2" {
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
