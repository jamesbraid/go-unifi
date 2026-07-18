package unifi

import "testing"

// fieldCandidate is one encoder-allowlist wire field to verify against a
// live controller: create a network of Purpose with Value (plus Prereq
// siblings), read back, and see whether the controller kept it.
type fieldCandidate struct {
	Wire    string
	Purpose string
	Value   any
	Prereq  map[string]any
}

// networkFieldCandidates lists every networkEncoderAllowlist entry in
// network_encode_coverage_test.go marked "TODO: possibly a real gap" -- one
// fieldCandidate per wire name, enumerated in the same order and grouping as
// that file. Values follow the generated struct's type and validation
// comment for each wire name (see network.generated.go); Prereq carries the
// sibling fields (mode selectors, enable flags) the controller needs before
// it will accept the candidate value at all.
var networkFieldCandidates = []fieldCandidate{
	// dhcpd_time_offset_enabled is already emitted for corporate/guest; the
	// offset value itself is not.
	{Wire: "dhcpd_time_offset", Purpose: PurposeCorporate, Value: 3600, Prereq: map[string]any{"dhcpd_time_offset_enabled": true}},

	// mac_override is already emitted for corporate/guest; its enable flag
	// is not. The candidate IS the enable flag, so Value is the flag itself
	// and Prereq supplies the value field it gates.
	{Wire: "mac_override_enabled", Purpose: PurposeCorporate, Value: true, Prereq: map[string]any{"mac_override": "02:00:00:00:00:01"}},

	// vpn_client_configuration_remote_ip_override is already emitted for
	// remote-user-vpn (grouped under WireGuard Server Configuration in
	// marshalUserVPN); its enable flag is not.
	{Wire: "vpn_client_configuration_remote_ip_override_enabled", Purpose: PurposeUserVPN, Value: true, Prereq: map[string]any{"vpn_type": "wireguard-server", "vpn_client_configuration_remote_ip_override": "192.0.2.55"}},

	// Zone-based firewall (controller 9.0+) network-to-zone assignment.
	{Wire: "firewall_zone_id", Purpose: PurposeCorporate, Value: "@zone", Prereq: nil}, // "@zone": resolved live to an existing zone id (Task 3)

	// Advanced multicast settings: fast leave, querier, proxy downstream
	// networks, flood control. Grouped with the igmp_snooping toggle the
	// encoder already sends for corporate/guest/vlan-only.
	{Wire: "igmp_fastleave", Purpose: PurposeCorporate, Value: true, Prereq: nil},
	{Wire: "igmp_flood_unknown_multicast", Purpose: PurposeCorporate, Value: true, Prereq: nil},
	{Wire: "igmp_groupmembership", Purpose: PurposeCorporate, Value: 260, Prereq: nil},                                                   // seconds; validation allows 2-3600, 260 is IGMP's own default interval
	{Wire: "igmp_maxresponse", Purpose: PurposeCorporate, Value: 10, Prereq: nil},                                                        // seconds; validation caps at 25
	{Wire: "igmp_mcrtrexpiretime", Purpose: PurposeCorporate, Value: 300, Prereq: nil},                                                   // seconds; validation allows 0-3600
	{Wire: "igmp_proxy_downstream_networkconf_ids", Purpose: PurposeCorporate, Value: []string{"000000000000000000000000"}, Prereq: nil}, // shape of a referenced networkconf id; a real id must be substituted to test acceptance live
	{Wire: "igmp_querier_switches", Purpose: PurposeCorporate, Value: []NetworkIGMPQuerierSwitches{{QuerierAddress: "10.0.0.254"}}, Prereq: nil},
	{Wire: "igmp_supression", Purpose: PurposeCorporate, Value: true, Prereq: nil},

	// WAN MTU. The coverage comment attributes this to marshalWAN, not
	// corporate -- Purpose is PurposeWAN here.
	{Wire: "interface_mtu", Purpose: PurposeWAN, Value: 1400, Prereq: map[string]any{"interface_mtu_enabled": true}},
	{Wire: "interface_mtu_enabled", Purpose: PurposeWAN, Value: true, Prereq: nil},

	// Advanced site-vpn IPsec options: IKE identifiers, tunnel IP, separate
	// IKEv2 networks. All require vpn_type "ipsec-vpn" (the only IPsec value
	// in the vpn_type enum) to be meaningful.
	{Wire: "ipsec_local_identifier", Purpose: PurposeSiteVPN, Value: "site-a.vpn.example.com", Prereq: map[string]any{"vpn_type": "ipsec-vpn", "ipsec_local_identifier_enabled": true}},
	{Wire: "ipsec_local_identifier_enabled", Purpose: PurposeSiteVPN, Value: true, Prereq: map[string]any{"vpn_type": "ipsec-vpn"}},
	{Wire: "ipsec_remote_identifier", Purpose: PurposeSiteVPN, Value: "site-b.vpn.example.com", Prereq: map[string]any{"vpn_type": "ipsec-vpn", "ipsec_remote_identifier_enabled": true}},
	{Wire: "ipsec_remote_identifier_enabled", Purpose: PurposeSiteVPN, Value: true, Prereq: map[string]any{"vpn_type": "ipsec-vpn"}},
	{Wire: "ipsec_separate_ikev2_networks", Purpose: PurposeSiteVPN, Value: true, Prereq: map[string]any{"vpn_type": "ipsec-vpn", "ipsec_key_exchange": "ikev2"}}, // "separate IKEv2 networks" only makes sense in ikev2 mode
	{Wire: "ipsec_tunnel_ip", Purpose: PurposeSiteVPN, Value: "192.0.2.4/30", Prereq: map[string]any{"vpn_type": "ipsec-vpn", "ipsec_tunnel_ip_enabled": true}},
	{Wire: "ipsec_tunnel_ip_enabled", Purpose: PurposeSiteVPN, Value: true, Prereq: map[string]any{"vpn_type": "ipsec-vpn"}},

	// ipv6_interface_type "single_network" mode and its companion
	// interface/LAN selection fields.
	{Wire: "ipv6_single_network_interface", Purpose: PurposeCorporate, Value: "wan", Prereq: map[string]any{"ipv6_interface_type": "single_network"}},           // no validation comment on the generated field; "wan" mirrors the ipsec_interface/l2tp_interface "wan[2-9]?" convention
	{Wire: "single_network_lan", Purpose: PurposeCorporate, Value: "000000000000000000000000", Prereq: map[string]any{"ipv6_interface_type": "single_network"}}, // shape of a referenced LAN network id; a real id must be substituted to test acceptance live

	// site-vpn emits remote_vpn_subnets but not the dynamic-subnets toggle;
	// pairs naturally with the ipsec_dynamic_routing flag the encoder
	// already sends.
	{Wire: "remote_vpn_dynamic_subnets_enabled", Purpose: PurposeSiteVPN, Value: true, Prereq: map[string]any{"vpn_type": "ipsec-vpn", "ipsec_dynamic_routing": true}},

	// L2TP remote-user-vpn RADIUS option; the other l2tp_* fields are
	// already emitted.
	{Wire: "require_mschapv2", Purpose: PurposeUserVPN, Value: true, Prereq: map[string]any{"vpn_type": "l2tp-server", "l2tp_interface": "wan"}},

	// Per-LAN UPnP toggle (distinct from the WAN-side upnp_* fields
	// marshalWAN already sends).
	{Wire: "upnp_lan_enabled", Purpose: PurposeCorporate, Value: true, Prereq: nil},

	// OpenVPN server protocol for remote-user-vpn; openvpn_mode is already
	// emitted.
	{Wire: "vpn_protocol", Purpose: PurposeUserVPN, Value: "UDP", Prereq: map[string]any{"vpn_type": "openvpn-server", "openvpn_mode": "server"}},

	// marshalWAN sends wan_type but not the fields its "static", "pppoe", or
	// "dslite" modes need, so only DHCP-style WANs round-trip fully.
	{Wire: "wan_ip", Purpose: PurposeWAN, Value: "192.0.2.10", Prereq: map[string]any{"wan_type": "static", "wan_netmask": "255.255.255.0", "wan_gateway": "192.0.2.1"}},
	{Wire: "wan_netmask", Purpose: PurposeWAN, Value: "255.255.255.0", Prereq: map[string]any{"wan_type": "static", "wan_ip": "192.0.2.10", "wan_gateway": "192.0.2.1"}},
	{Wire: "wan_gateway", Purpose: PurposeWAN, Value: "192.0.2.1", Prereq: map[string]any{"wan_type": "static", "wan_ip": "192.0.2.10", "wan_netmask": "255.255.255.0"}},
	{Wire: "wan_ipv6", Purpose: PurposeWAN, Value: "2001:db8::10", Prereq: map[string]any{"wan_type_v6": "static", "wan_gateway_v6": "2001:db8::1", "wan_prefixlen": 64}},
	{Wire: "wan_gateway_v6", Purpose: PurposeWAN, Value: "2001:db8::1", Prereq: map[string]any{"wan_type_v6": "static", "wan_ipv6": "2001:db8::10", "wan_prefixlen": 64}},
	{Wire: "wan_prefixlen", Purpose: PurposeWAN, Value: 64, Prereq: map[string]any{"wan_type_v6": "static", "wan_ipv6": "2001:db8::10", "wan_gateway_v6": "2001:db8::1"}},
	{Wire: "wan_username", Purpose: PurposeWAN, Value: "pppoe-user", Prereq: map[string]any{"wan_type": "pppoe", "x_wan_password": "pppoe-pass", "wan_pppoe_username_enabled": true}},
	{Wire: "x_wan_password", Purpose: PurposeWAN, Value: "pppoe-pass", Prereq: map[string]any{"wan_type": "pppoe", "wan_username": "pppoe-user", "wan_pppoe_password_enabled": true}},
	{Wire: "wan_pppoe_username_enabled", Purpose: PurposeWAN, Value: true, Prereq: map[string]any{"wan_type": "pppoe", "wan_username": "pppoe-user"}},
	{Wire: "wan_pppoe_password_enabled", Purpose: PurposeWAN, Value: true, Prereq: map[string]any{"wan_type": "pppoe", "x_wan_password": "pppoe-pass"}},
	{Wire: "wan_dslite_remote_host", Purpose: PurposeWAN, Value: "aftr.example.net", Prereq: map[string]any{"wan_type": "dslite", "wan_dslite_remote_host_auto": false}}, // AFTR hostname; no validation comment on the generated field
	{Wire: "wan_dslite_remote_host_auto", Purpose: PurposeWAN, Value: true, Prereq: map[string]any{"wan_type": "dslite"}},
}

// TestFieldCandidatesCoverAllTODOs keeps networkFieldCandidates and
// networkEncoderPresenceAllowlistTODOs (network_encode_coverage_test.go) in
// lockstep: every TODO wire name must have exactly one candidate, and every
// candidate must correspond to a TODO wire name (not an already-wired or
// stale entry).
func TestFieldCandidatesCoverAllTODOs(t *testing.T) {
	want := map[string]bool{}
	for _, w := range networkEncoderPresenceAllowlistTODOs {
		want[w] = true
	}
	got := map[string]bool{}
	for _, c := range networkFieldCandidates {
		if got[c.Wire] {
			t.Errorf("duplicate candidate %q", c.Wire)
		}
		got[c.Wire] = true
	}
	for w := range want {
		if !got[w] {
			t.Errorf("allowlist TODO %q has no candidate entry", w)
		}
	}
	for w := range got {
		if !want[w] {
			t.Errorf("candidate %q is not an allowlist TODO (already wired or stale?)", w)
		}
	}
}
