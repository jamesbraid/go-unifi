package unifi

import (
	"encoding/json"
	"testing"
)

// Helper function to parse JSON and check for expected/unexpected fields.
func checkJSONFields(t *testing.T, data []byte, expectedFields []string, unexpectedFields []string) {
	t.Helper()

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check expected fields are present
	for _, field := range expectedFields {
		if _, ok := result[field]; !ok {
			t.Errorf("Expected field %q not found in JSON", field)
		}
	}

	// Check unexpected fields are absent
	for _, field := range unexpectedFields {
		if _, ok := result[field]; ok {
			t.Errorf("Unexpected field %q found in JSON", field)
		}
	}
}

func TestMarshalNetworkCorporate(t *testing.T) {
	// Create a corporate network with common fields
	vlan := int64(10)
	leasetime := int64(86400)
	dhcpGateway := "192.168.1.1"
	dhcpStart := "192.168.1.100"
	dhcpStop := "192.168.1.200"

	network := &Network{
		ID:                    "507f1f77bcf86cd799439011",
		SiteID:                "default",
		Name:                  strPtr("Corporate LAN"),
		Purpose:               PurposeCorporate,
		Enabled:               true,
		AutoScaleEnabled:      false,
		NetworkGroup:          strPtr("LAN"),
		IPSubnet:              strPtr("192.168.1.0/24"),
		VLAN:                  &vlan,
		VLANEnabled:           true,
		DomainName:            strPtr("example.local"),
		GatewayType:           strPtr("default"),
		DHCPDGateway:          &dhcpGateway,
		DHCPDGatewayEnabled:   true,
		InternetAccessEnabled: true,
		MdnsEnabled:           true,
		IGMPSnooping:          false,
		DHCPDEnabled:          true,
		DHCPDStart:            &dhcpStart,
		DHCPDStop:             &dhcpStop,
		DHCPDLeaseTime:        &leasetime,
		DHCPDDNS1:             "8.8.8.8",
		DHCPDDNS2:             "8.8.4.4",
		DHCPDDNSEnabled:       true,
		IPAliases:             []string{},
	}

	// Marshal to JSON
	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("Failed to marshal network: %v", err)
	}

	// Expected fields for corporate network
	expectedFields := []string{
		"_id",
		"site_id",
		"name",
		"purpose",
		"enabled",
		"networkgroup",
		"ip_subnet",
		"vlan",
		"vlan_enabled",
		"domain_name",
		"gateway_type",
		"dhcpd_gateway",
		"dhcpd_gateway_enabled",
		"internet_access_enabled",
		"mdns_enabled",
		"igmp_snooping",
		"dhcpd_enabled",
		"dhcpd_start",
		"dhcpd_stop",
		"dhcpd_leasetime",
		"dhcpd_dns_1",
		"dhcpd_dns_2",
		"dhcpd_dns_enabled",
		"ip_aliases",
		"auto_scale_enabled",
		"setting_preference",
	}

	// Unexpected fields (WAN-specific)
	unexpectedFields := []string{
		"wan_type",
		"wan_ip",
		"wan_networkgroup",
		"ipsec_key_exchange",
		"wireguard_interface",
	}

	checkJSONFields(t, data, expectedFields, unexpectedFields)

	// Verify purpose is correct
	var result map[string]any
	json.Unmarshal(data, &result)
	if result["purpose"] != string(PurposeCorporate) {
		t.Errorf("Expected purpose %q, got %q", PurposeCorporate, result["purpose"])
	}

	// Verify default values are applied
	if result["networkgroup"] != "LAN" {
		t.Errorf("Expected networkgroup 'LAN', got %q", result["networkgroup"])
	}
	if result["gateway_type"] != "default" {
		t.Errorf("Expected gateway_type 'default', got %q", result["gateway_type"])
	}
	if result["setting_preference"] != "auto" {
		t.Errorf("Expected setting_preference 'auto', got %q", result["setting_preference"])
	}
}

func TestMarshalNetworkCorporateDefaults(t *testing.T) {
	// Create a minimal corporate network to test defaults
	network := &Network{
		ID:      "507f1f77bcf86cd799439011",
		Purpose: PurposeCorporate,
		Enabled: true,
	}

	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("Failed to marshal network: %v", err)
	}

	var result map[string]any
	json.Unmarshal(data, &result)

	// Verify defaults are applied
	if result["networkgroup"] != "LAN" {
		t.Errorf("Expected default networkgroup 'LAN', got %v", result["networkgroup"])
	}
	if result["gateway_type"] != "default" {
		t.Errorf("Expected default gateway_type 'default', got %v", result["gateway_type"])
	}
	if result["setting_preference"] != "auto" {
		t.Errorf("Expected default setting_preference 'auto', got %v", result["setting_preference"])
	}
	if result["ip_subnet"] != "" {
		t.Errorf("Expected empty ip_subnet, got %v", result["ip_subnet"])
	}

	// Verify empty arrays are empty, not nil
	if aliases, ok := result["ip_aliases"].([]any); !ok || len(aliases) != 0 {
		t.Errorf("Expected empty array for ip_aliases, got %v", result["ip_aliases"])
	}
}

func TestMarshalNetworkWAN(t *testing.T) {
	vlan := int64(20)
	failoverPriority := int64(1)
	loadBalanceWeight := int64(50)
	dhcpv6PDSize := int64(56)

	network := &Network{
		ID:                    "507f1f77bcf86cd799439012",
		SiteID:                "default",
		Name:                  strPtr("WAN"),
		Purpose:               PurposeWAN,
		Enabled:               true,
		WANType:               strPtr("dhcp"),
		WANTypeV6:             strPtr("dhcpv6"),
		WANNetworkGroup:       strPtr("WAN"),
		WANVLANEnabled:        true,
		WANVLAN:               &vlan,
		WANDNSPreference:      strPtr("auto"),
		WANIPV6DNSPreference:  strPtr("auto"),
		WANDHCPv6PDSize:       &dhcpv6PDSize,
		WANDHCPv6PDSizeAuto:   true,
		IPV6WANDelegationType: strPtr("pd"),
		WANLoadBalanceType:    strPtr("failover-only"),
		WANLoadBalanceWeight:  &loadBalanceWeight,
		WANFailoverPriority:   &failoverPriority,
		IGMPProxyFor:          strPtr("none"),
		IGMPProxyUpstream:     false,
		ReportWANEvent:        true,
		WANIPAliases:          []string{},
		WANDHCPOptions:        []NetworkWANDHCPOptions{},
	}

	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("Failed to marshal network: %v", err)
	}

	expectedFields := []string{
		"_id",
		"site_id",
		"name",
		"purpose",
		"enabled",
		"wan_type",
		"wan_type_v6",
		"wan_networkgroup",
		"wan_vlan_enabled",
		"wan_vlan",
		"wan_dns_preference",
		"wan_ipv6_dns_preference",
		"wan_dhcpv6_pd_size",
		"wan_dhcpv6_pd_size_auto",
		"ipv6_wan_delegation_type",
		"wan_load_balance_type",
		"wan_load_balance_weight",
		"wan_failover_priority",
		"igmp_proxy_for",
		"igmp_proxy_upstream",
		"report_wan_event",
		"wan_ip_aliases",
		"wan_dhcp_options",
		"ipv6_enabled",
	}

	unexpectedFields := []string{
		"networkgroup",
		"ip_subnet",
		"vlan",
		"dhcpd_enabled",
		"ipsec_interface",
		"wireguard_interface",
	}

	checkJSONFields(t, data, expectedFields, unexpectedFields)

	var result map[string]any
	json.Unmarshal(data, &result)

	// Verify WAN-specific values
	if result["purpose"] != string(PurposeWAN) {
		t.Errorf("Expected purpose %q, got %q", PurposeWAN, result["purpose"])
	}
	if result["ipv6_enabled"] != true {
		t.Errorf("Expected ipv6_enabled true, got %v", result["ipv6_enabled"])
	}

	// Verify empty arrays
	if aliases, ok := result["wan_ip_aliases"].([]any); !ok || len(aliases) != 0 {
		t.Errorf("Expected empty array for wan_ip_aliases, got %v", result["wan_ip_aliases"])
	}
}

func TestMarshalNetworkUnknownPurpose(t *testing.T) {
	network := &Network{
		ID:      "507f1f77bcf86cd799439016",
		Purpose: "unknown-purpose",
		Enabled: true,
	}

	_, err := json.Marshal(network)
	if err == nil {
		t.Error("Expected error for unknown purpose, got nil")
	}
}

func TestMarshalNetworkVLANOnly(t *testing.T) {
	vlan := int64(92)

	network := &Network{
		ID:      "507f1f77bcf86cd799439017",
		SiteID:  "default",
		Name:    strPtr("VLAN_92"),
		Purpose: PurposeVLANOnly,
		VLAN:    &vlan,
	}

	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("Failed to marshal vlan-only network: %v", err)
	}

	expectedFields := []string{
		"_id",
		"site_id",
		"name",
		"purpose",
		"enabled",
		"networkgroup",
		"vlan",
		"vlan_enabled",
	}

	unexpectedFields := []string{
		"ip_subnet",
		"dhcpd_enabled",
		"wan_type",
		"wireguard_interface",
	}

	checkJSONFields(t, data, expectedFields, unexpectedFields)

	var result map[string]any
	json.Unmarshal(data, &result)

	if result["purpose"] != "vlan-only" {
		t.Errorf("Expected purpose 'vlan-only', got %q", result["purpose"])
	}
	if result["enabled"] != true {
		t.Errorf("Expected enabled true (default), got %v", result["enabled"])
	}
	if result["vlan_enabled"] != true {
		t.Errorf("Expected vlan_enabled true (auto-set from VLAN ID), got %v", result["vlan_enabled"])
	}
	if result["networkgroup"] != "LAN" {
		t.Errorf("Expected networkgroup 'LAN', got %v", result["networkgroup"])
	}
}

func TestMarshalNetworkVLANOnlyMinimal(t *testing.T) {
	network := &Network{
		ID:      "507f1f77bcf86cd799439018",
		Purpose: PurposeVLANOnly,
	}

	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("Failed to marshal minimal vlan-only network: %v", err)
	}

	var result map[string]any
	json.Unmarshal(data, &result)

	if result["purpose"] != "vlan-only" {
		t.Errorf("Expected purpose 'vlan-only', got %q", result["purpose"])
	}
	if result["enabled"] != true {
		t.Errorf("Expected enabled true (default), got %v", result["enabled"])
	}
	if result["vlan_enabled"] != false {
		t.Errorf("Expected vlan_enabled false (no VLAN ID), got %v", result["vlan_enabled"])
	}
}

func TestMarshalNetworkGuest(t *testing.T) {
	vlan := int64(100)
	leasetime := int64(86400)
	dhcpStart := "192.168.100.100"
	dhcpStop := "192.168.100.200"

	network := &Network{
		ID:                    "507f1f77bcf86cd799439019",
		SiteID:                "default",
		Name:                  strPtr("Guest Network"),
		Purpose:               PurposeGuest,
		Enabled:               true,
		NetworkGroup:          strPtr("LAN"),
		IPSubnet:              strPtr("192.168.100.0/24"),
		VLAN:                  &vlan,
		VLANEnabled:           true,
		InternetAccessEnabled: true,
		DHCPDEnabled:          true,
		DHCPDStart:            &dhcpStart,
		DHCPDStop:             &dhcpStop,
		DHCPDLeaseTime:        &leasetime,
		DHCPDDNSEnabled:       true,
		DHCPDDNS1:             "8.8.8.8",
	}

	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("Failed to marshal guest network: %v", err)
	}

	expectedFields := []string{
		"_id",
		"site_id",
		"name",
		"purpose",
		"enabled",
		"networkgroup",
		"ip_subnet",
		"vlan",
		"vlan_enabled",
		"internet_access_enabled",
		"dhcpd_enabled",
		"dhcpd_start",
		"dhcpd_stop",
		"dhcpd_leasetime",
		"dhcpd_dns_enabled",
		"dhcpd_dns_1",
		"ip_aliases",
		"setting_preference",
	}

	unexpectedFields := []string{
		"wan_type",
		"wan_networkgroup",
		"wireguard_interface",
	}

	checkJSONFields(t, data, expectedFields, unexpectedFields)

	var result map[string]any
	json.Unmarshal(data, &result)

	if result["purpose"] != "guest" {
		t.Errorf("Expected purpose 'guest', got %q", result["purpose"])
	}
	if result["networkgroup"] != "LAN" {
		t.Errorf("Expected networkgroup 'LAN', got %v", result["networkgroup"])
	}
	if result["setting_preference"] != "auto" {
		t.Errorf("Expected setting_preference 'auto', got %v", result["setting_preference"])
	}
}

// TestMarshalNetworkIPv6ClientAddressAssignment guards that the corporate and
// guest marshalers emit ipv6_client_address_assignment when set, and omit it
// when nil. The field lives on the generated Network struct but the marshalers
// only serialize a curated subset, so it would otherwise be silently dropped on
// write (ubiquiti-community/terraform-provider-unifi#232).
func TestMarshalNetworkIPv6ClientAddressAssignment(t *testing.T) {
	for _, purpose := range []string{PurposeCorporate, PurposeGuest} {
		t.Run(purpose, func(t *testing.T) {
			// Set => emitted with the configured value.
			network := &Network{
				ID:                          "507f1f77bcf86cd799439011",
				Purpose:                     purpose,
				Enabled:                     true,
				IPV6InterfaceType:           strPtr("static"),
				IPV6ClientAddressAssignment: strPtr("slaac-dhcpv6"),
			}
			data, err := json.Marshal(network)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := result["ipv6_client_address_assignment"]; got != "slaac-dhcpv6" {
				t.Errorf("ipv6_client_address_assignment = %v, want slaac-dhcpv6", got)
			}

			// Unset => omitted (omitempty), no perpetual diff against the API.
			data, err = json.Marshal(&Network{ID: "x", Purpose: purpose, Enabled: true})
			if err != nil {
				t.Fatalf("marshal (unset): %v", err)
			}
			result = map[string]any{}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal (unset): %v", err)
			}
			if _, ok := result["ipv6_client_address_assignment"]; ok {
				t.Errorf("ipv6_client_address_assignment serialized for nil value: %s", data)
			}
		})
	}
}

// TestMarshalNetworkSiteVPN guards that the site-to-site IPsec VPN marshaler
// emits the VPN/IPsec fields (not just name/purpose/enabled). It was previously
// a stub, which silently dropped the whole tunnel configuration on write
// (ubiquiti-community/terraform-provider-unifi#78).
func TestMarshalNetworkSiteVPN(t *testing.T) {
	dh := int64(14)
	network := &Network{
		ID:                "507f1f77bcf86cd799439011",
		Name:              strPtr("HQ-to-Branch"),
		Purpose:           PurposeSiteVPN,
		Enabled:           true,
		VPNType:           strPtr("ipsec-vpn"),
		IPSecInterface:    strPtr("wan"),
		IPSecPeerIP:       strPtr("203.0.113.9"),
		IPSecKeyExchange:  strPtr("ikev2"),
		IPSecPreSharedKey: strPtr("s3cret-psk"),
		IPSecProfile:      strPtr("customized"),
		IPSecEncryption:   strPtr("aes256"),
		IPSecHash:         strPtr("sha256"),
		IPSecDhGroup:      &dh,
		IPSecPfs:          true,
		RemoteVPNSubnets:  []string{"192.0.2.0/24", "198.51.100.0/24"},
	}

	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	checkJSONFields(t, data, []string{
		"name", "purpose", "enabled", "vpn_type", "ipsec_interface",
		"ipsec_peer_ip", "ipsec_key_exchange", "x_ipsec_pre_shared_key",
		"ipsec_profile", "ipsec_encryption", "ipsec_hash", "ipsec_dh_group",
		"ipsec_pfs", "remote_vpn_subnets",
	}, []string{"ip_subnet", "dhcpd_enabled", "vlan"})

	if result["purpose"] != PurposeSiteVPN {
		t.Errorf("purpose = %v, want %q", result["purpose"], PurposeSiteVPN)
	}
	if result["vpn_type"] != "ipsec-vpn" {
		t.Errorf("vpn_type = %v, want ipsec-vpn", result["vpn_type"])
	}
	if result["x_ipsec_pre_shared_key"] != "s3cret-psk" {
		t.Errorf("x_ipsec_pre_shared_key = %v", result["x_ipsec_pre_shared_key"])
	}
	subnets, ok := result["remote_vpn_subnets"].([]any)
	if !ok || len(subnets) != 2 {
		t.Errorf("remote_vpn_subnets = %v, want 2 entries", result["remote_vpn_subnets"])
	}
}

// TestMarshalNetworkRoutedInterfaceAndIPv6Aliases guards fields added in
// controller 10.4.57: the L3 routed interface attachment
// (l3_interface_type/routed_port_idx/routed_lag_idx) and ipv6_aliases (the
// v6 analog of ip_aliases). The marshalers serialize a curated subset of the
// generated struct, so new fields are silently dropped on write unless
// explicitly added.
// TestMarshalNetworkDHCPRelayServers guards against the corporate marshaler
// sourcing dhcp_relay_servers from the wrong struct field (it used to copy
// RemoteVPNSubnets).
func TestMarshalNetworkDHCPRelayServers(t *testing.T) {
	for _, purpose := range []string{PurposeCorporate, PurposeGuest} {
		t.Run(purpose, func(t *testing.T) {
			network := &Network{
				ID:               "507f1f77bcf86cd799439021",
				Purpose:          purpose,
				Enabled:          true,
				IPSubnet:         strPtr("192.168.5.0/24"),
				DHCPRelayServers: []string{"192.168.1.5", "192.168.1.6"},
				RemoteVPNSubnets: []string{"10.99.0.0/16"},
			}

			data, err := json.Marshal(network)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			relays, ok := result["dhcp_relay_servers"].([]any)
			if !ok || len(relays) != 2 || relays[0] != "192.168.1.5" || relays[1] != "192.168.1.6" {
				t.Errorf("dhcp_relay_servers = %v, want the DHCPRelayServers values", result["dhcp_relay_servers"])
			}
		})
	}
}

func TestMarshalNetworkRoutedInterfaceAndIPv6Aliases(t *testing.T) {
	for _, purpose := range []string{PurposeCorporate, PurposeGuest} {
		t.Run(purpose, func(t *testing.T) {
			routedPort := int64(8)
			network := &Network{
				ID:              "507f1f77bcf86cd799439020",
				Purpose:         purpose,
				Enabled:         true,
				IPSubnet:        strPtr("192.168.5.0/24"),
				IPAliases:       []string{"192.168.6.1/24"},
				IPV6Aliases:     []string{"2001:db8::1/64"},
				L3InterfaceType: strPtr("port"),
				RoutedPortIDX:   &routedPort,
			}

			data, err := json.Marshal(network)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if got := result["l3_interface_type"]; got != "port" {
				t.Errorf("l3_interface_type = %v, want port", got)
			}
			if got := result["routed_port_idx"]; got != float64(8) {
				t.Errorf("routed_port_idx = %v, want 8", got)
			}
			if aliases, ok := result["ipv6_aliases"].([]any); !ok || len(aliases) != 1 || aliases[0] != "2001:db8::1/64" {
				t.Errorf("ipv6_aliases = %v, want [2001:db8::1/64]", result["ipv6_aliases"])
			}
			// RoutedLagIDX was not set, so it must be omitted.
			if _, ok := result["routed_lag_idx"]; ok {
				t.Errorf("routed_lag_idx serialized for nil value: %s", data)
			}

			// Unset => pointer fields omitted; ipv6_aliases still emitted as
			// an empty array, mirroring ip_aliases.
			data, err = json.Marshal(&Network{ID: "x", Purpose: purpose, Enabled: true})
			if err != nil {
				t.Fatalf("marshal (unset): %v", err)
			}
			result = map[string]any{}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal (unset): %v", err)
			}
			for _, field := range []string{"l3_interface_type", "routed_port_idx", "routed_lag_idx"} {
				if _, ok := result[field]; ok {
					t.Errorf("%s serialized for nil value: %s", field, data)
				}
			}
			if aliases, ok := result["ipv6_aliases"].([]any); !ok || len(aliases) != 0 {
				t.Errorf("Expected empty array for ipv6_aliases, got %v", result["ipv6_aliases"])
			}
		})
	}
}

// TestMarshalNetworkWANMssClamp guards the MSS clamping fields added in
// controller 10.4.57 (mss_clamp/mss_clamp_mss and their IPv6 variants) on
// the WAN marshaler.
func TestMarshalNetworkWANMssClamp(t *testing.T) {
	mss := int64(1452)
	mssV6 := int64(1432)
	network := &Network{
		ID:              "507f1f77bcf86cd799439021",
		Purpose:         PurposeWAN,
		Enabled:         true,
		WANType:         strPtr("dhcp"),
		MssClamp:        strPtr("custom"),
		MssClampMss:     &mss,
		MssClampIPV6:    strPtr("custom"),
		MssClampMssIPV6: &mssV6,
	}

	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := result["mss_clamp"]; got != "custom" {
		t.Errorf("mss_clamp = %v, want custom", got)
	}
	if got := result["mss_clamp_mss"]; got != float64(1452) {
		t.Errorf("mss_clamp_mss = %v, want 1452", got)
	}
	if got := result["mss_clamp_ipv6"]; got != "custom" {
		t.Errorf("mss_clamp_ipv6 = %v, want custom", got)
	}
	if got := result["mss_clamp_mss_ipv6"]; got != float64(1432) {
		t.Errorf("mss_clamp_mss_ipv6 = %v, want 1432", got)
	}

	// Unset => omitted, no perpetual diff against the API.
	data, err = json.Marshal(&Network{ID: "x", Purpose: PurposeWAN, Enabled: true})
	if err != nil {
		t.Fatalf("marshal (unset): %v", err)
	}
	result = map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal (unset): %v", err)
	}
	for _, field := range []string{"mss_clamp", "mss_clamp_mss", "mss_clamp_ipv6", "mss_clamp_mss_ipv6"} {
		if _, ok := result[field]; ok {
			t.Errorf("%s serialized for nil value: %s", field, data)
		}
	}
}

// TestMarshalNetworkUserVPNBindingMode guards vpn_binding_mode (added in
// controller 10.4.57), which selects how the remote-user VPN server binds to
// a WAN address (static|interface|any).
func TestMarshalNetworkUserVPNBindingMode(t *testing.T) {
	network := &Network{
		ID:             "507f1f77bcf86cd799439022",
		Purpose:        PurposeUserVPN,
		Enabled:        true,
		VPNType:        strPtr("wireguard-server"),
		VPNBindingMode: strPtr("interface"),
	}

	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := result["vpn_binding_mode"]; got != "interface" {
		t.Errorf("vpn_binding_mode = %v, want interface", got)
	}

	// MSS clamping is reported on remote-user-vpn networks by live 10.4.57
	// controllers; make sure the user-VPN marshaler sends it.
	clampMss := int64(1400)
	network.MssClamp = strPtr("custom")
	network.MssClampMss = &clampMss
	data, err = json.Marshal(network)
	if err != nil {
		t.Fatalf("marshal (mss clamp): %v", err)
	}
	result = map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal (mss clamp): %v", err)
	}
	if got := result["mss_clamp"]; got != "custom" {
		t.Errorf("mss_clamp = %v, want custom", got)
	}
	if got := result["mss_clamp_mss"]; got != float64(1400) {
		t.Errorf("mss_clamp_mss = %v, want 1400", got)
	}

	// Unset => omitted.
	data, err = json.Marshal(&Network{ID: "x", Purpose: PurposeUserVPN, Enabled: true})
	if err != nil {
		t.Fatalf("marshal (unset): %v", err)
	}
	result = map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal (unset): %v", err)
	}
	if _, ok := result["vpn_binding_mode"]; ok {
		t.Errorf("vpn_binding_mode serialized for nil value: %s", data)
	}
}

// TestMarshalNetworkCorporateIGMPAdvanced guards the advanced multicast
// fields added to the corporate marshaler in Task 4 (fast leave, flood
// control, querier interval/response/expire timers, querier switches,
// suppression) -- live-verified PERSISTED against a simulation-mode 10.0.162
// controller (network_field_probe_integration_test.go).
func TestMarshalNetworkCorporateIGMPAdvanced(t *testing.T) {
	groupmembership := int64(260)
	maxresponse := int64(10)
	mcrtrexpiretime := int64(300)

	network := &Network{
		ID:                        "507f1f77bcf86cd799439030",
		Purpose:                   PurposeCorporate,
		Enabled:                   true,
		IGMPFastleave:             true,
		IGMPFloodUnknownMulticast: true,
		IGMPGroupmembership:       &groupmembership,
		IGMPMaxresponse:           &maxresponse,
		IGMPMcrtrexpiretime:       &mcrtrexpiretime,
		IGMPQuerierSwitches:       []NetworkIGMPQuerierSwitches{{QuerierAddress: "10.0.0.254"}},
		IGMPSuppression:           true,
	}

	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := result["igmp_fastleave"]; got != true {
		t.Errorf("igmp_fastleave = %v, want true", got)
	}
	if got := result["igmp_flood_unknown_multicast"]; got != true {
		t.Errorf("igmp_flood_unknown_multicast = %v, want true", got)
	}
	if got := result["igmp_groupmembership"]; got != float64(260) {
		t.Errorf("igmp_groupmembership = %v, want 260", got)
	}
	if got := result["igmp_maxresponse"]; got != float64(10) {
		t.Errorf("igmp_maxresponse = %v, want 10", got)
	}
	if got := result["igmp_mcrtrexpiretime"]; got != float64(300) {
		t.Errorf("igmp_mcrtrexpiretime = %v, want 300", got)
	}
	if got := result["igmp_supression"]; got != true {
		t.Errorf("igmp_supression = %v, want true", got)
	}
	switches, ok := result["igmp_querier_switches"].([]any)
	if !ok || len(switches) != 1 {
		t.Fatalf("igmp_querier_switches = %v, want 1 entry", result["igmp_querier_switches"])
	}
	entry, ok := switches[0].(map[string]any)
	if !ok || entry["querier_address"] != "10.0.0.254" {
		t.Errorf("igmp_querier_switches[0] = %v, want querier_address 10.0.0.254", switches[0])
	}

	// Unset => bools false, pointers/slices omitted.
	data, err = json.Marshal(&Network{ID: "x", Purpose: PurposeCorporate, Enabled: true})
	if err != nil {
		t.Fatalf("marshal (unset): %v", err)
	}
	result = map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal (unset): %v", err)
	}
	if got := result["igmp_fastleave"]; got != false {
		t.Errorf("igmp_fastleave = %v, want false", got)
	}
	if got := result["igmp_supression"]; got != false {
		t.Errorf("igmp_supression = %v, want false", got)
	}
	for _, field := range []string{"igmp_groupmembership", "igmp_maxresponse", "igmp_mcrtrexpiretime", "igmp_querier_switches"} {
		if _, ok := result[field]; ok {
			t.Errorf("%s serialized for nil value: %s", field, data)
		}
	}
}

// TestMarshalNetworkCorporateMisc guards the remaining corporate fields
// wired in Task 4: DHCP time offset value, MAC override enable flag,
// zone-based firewall assignment, per-LAN UPnP, and the ipv6_interface_type
// "single_network" companion fields -- live-verified PERSISTED.
func TestMarshalNetworkCorporateMisc(t *testing.T) {
	timeOffset := int64(3600)
	zoneID := "64f0000000000000000000aa"
	lanID := "64f0000000000000000000bb"

	network := &Network{
		ID:                         "507f1f77bcf86cd799439031",
		Purpose:                    PurposeCorporate,
		Enabled:                    true,
		DHCPDTimeOffset:            &timeOffset,
		MACOverride:                "02:00:00:00:00:01",
		MACOverrideEnabled:         true,
		FirewallZoneID:             &zoneID,
		UPnPLanEnabled:             true,
		IPV6InterfaceType:          strPtr("single_network"),
		IPV6SingleNetworkInterface: strPtr("wan"),
		SingleNetworkLan:           &lanID,
	}

	data, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := result["dhcpd_time_offset"]; got != float64(3600) {
		t.Errorf("dhcpd_time_offset = %v, want 3600", got)
	}
	if got := result["mac_override_enabled"]; got != true {
		t.Errorf("mac_override_enabled = %v, want true", got)
	}
	if got := result["firewall_zone_id"]; got != zoneID {
		t.Errorf("firewall_zone_id = %v, want %s", got, zoneID)
	}
	if got := result["upnp_lan_enabled"]; got != true {
		t.Errorf("upnp_lan_enabled = %v, want true", got)
	}
	if got := result["ipv6_single_network_interface"]; got != "wan" {
		t.Errorf("ipv6_single_network_interface = %v, want wan", got)
	}
	if got := result["single_network_lan"]; got != lanID {
		t.Errorf("single_network_lan = %v, want %s", got, lanID)
	}

	// Unset => omitted/false, no perpetual diff against the API.
	data, err = json.Marshal(&Network{ID: "x", Purpose: PurposeCorporate, Enabled: true})
	if err != nil {
		t.Fatalf("marshal (unset): %v", err)
	}
	result = map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal (unset): %v", err)
	}
	if got := result["mac_override_enabled"]; got != false {
		t.Errorf("mac_override_enabled = %v, want false", got)
	}
	if got := result["upnp_lan_enabled"]; got != false {
		t.Errorf("upnp_lan_enabled = %v, want false", got)
	}
	for _, field := range []string{"dhcpd_time_offset", "firewall_zone_id", "ipv6_single_network_interface", "single_network_lan"} {
		if _, ok := result[field]; ok {
			t.Errorf("%s serialized for nil value: %s", field, data)
		}
	}
}

// Helper function to create string pointers.
func strPtr(s string) *string {
	return &s
}
