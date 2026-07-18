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

// siteVPNBasePrereq is the minimum realistic IPsec site-vpn payload the
// simulated 10.x controller accepts; a bare vpn_type + one advanced field
// gets api.err.Invalid with no further detail. Peer addressing, a
// "customized" profile with explicit phase 1/2 parameters, a PSK, and a
// remote subnet round out a config the controller will actually persist.
// ipsec_local_ip / ipsec_dynamic_routing are deliberately not here: the
// controller only accepts a real (non-"any") ipsec_local_ip when
// ipsec_dynamic_routing is true, and rejects it as api.err.UnrecognizedLocalIp
// unless that address is bound to an actually-adopted gateway's WAN
// interface -- unsatisfiable in this container-only, no-hardware simulation.
// mergePolicyBasedPrereq/mergeRouteBasedPrereq pick the mode-appropriate
// pairing for each candidate; layer each candidate's own gating fields
// (enable flags, mode selectors) on top via extra.
var siteVPNBasePrereq = map[string]any{
	"vpn_type":               "ipsec-vpn",
	"ipsec_interface":        "wan",
	"ipsec_peer_ip":          "203.0.113.9",
	"ipsec_key_exchange":     "ikev2",
	"x_ipsec_pre_shared_key": "s3cret-psk",
	"ipsec_profile":          "customized",
	"ipsec_encryption":       "aes256",
	"ipsec_hash":             "sha256",
	"ipsec_dh_group":         14,
	"ipsec_esp_encryption":   "aes256",
	"ipsec_esp_hash":         "sha256",
	"ipsec_esp_dh_group":     14,
	"remote_vpn_subnets":     []string{"192.0.2.0/24"},
}

// mergePrereq layers extra on top of a copy of base so each site-vpn
// candidate can add its own gating field without repeating the base config
// or mutating the shared map.
func mergePrereq(base map[string]any, extra map[string]any) map[string]any {
	m := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		m[k] = v
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

// mergePolicyBasedPrereq is siteVPNBasePrereq in policy-based mode
// (ipsec_dynamic_routing false), which accepts ipsec_local_ip "any". Use for
// candidates that are policy-based concepts (per-peer IKE identifiers,
// per-child-SA IKEv2 networks).
func mergePolicyBasedPrereq(extra map[string]any) map[string]any {
	return mergePrereq(siteVPNBasePrereq, mergePrereq(map[string]any{
		"ipsec_local_ip":        "any",
		"ipsec_dynamic_routing": false,
	}, extra))
}

// mergeRouteBasedPrereq is siteVPNBasePrereq in route-based / dynamic-routing
// mode, which requires a real ipsec_local_ip -- the controller further
// requires that address be recognized as bound to an adopted gateway's WAN
// interface (api.err.UnrecognizedLocalIp), which this bare simulation cannot
// provide. Use for the route-based candidates (tunnel IP, dynamic subnets)
// even though they stay REJECTED here as a result.
func mergeRouteBasedPrereq(extra map[string]any) map[string]any {
	return mergePrereq(siteVPNBasePrereq, mergePrereq(map[string]any{
		"ipsec_local_ip":        "198.51.100.5",
		"ipsec_dynamic_routing": true,
	}, extra))
}

// networkFieldCandidates lists every networkEncoderAllowlist entry in
// network_encode_coverage_test.go marked "TODO: possibly a real gap" -- one
// fieldCandidate per wire name, enumerated in the same order and grouping as
// that file. Values follow the generated struct's type and validation
// comment for each wire name (see network.generated.go); Prereq carries the
// sibling fields (mode selectors, enable flags) the controller needs before
// it will accept the candidate value at all.
//
// The vpn_client_configuration_remote_ip_override_enabled, vpn_protocol,
// require_mschapv2, wan_pppoe_username_enabled, wan_pppoe_password_enabled,
// and all seven ipsec_*/remote_vpn_dynamic_subnets_enabled candidates were
// initially rejected by the live controller (api.err.MissingLocalPort,
// api.err.L2tpPskRequired, api.err.InvalidWanPppoeCredentials, api.err.Invalid
// respectively) because their Prereq payloads were missing sibling fields the
// controller requires before it will accept the base config at all -- not
// because the candidate field itself was rejected. Prereqs below were
// enriched with those siblings (a local_port, an L2TP PSK, both PPPoE
// credentials, a real radiusprofile_id, or a fuller site-vpn base config) and
// the probe re-run; see field-probe.log for the resulting classification.
var networkFieldCandidates = []fieldCandidate{
	// vpn_client_configuration_remote_ip_override is already emitted for
	// remote-user-vpn (grouped under WireGuard Server Configuration in
	// marshalUserVPN); its enable flag is not. The controller rejects a
	// wireguard-server network with api.err.MissingLocalPort and
	// api.err.WireguardMissingPrivateKey unless local_port and
	// x_wireguard_private_key are present.
	{Wire: "vpn_client_configuration_remote_ip_override_enabled", Purpose: PurposeUserVPN, Value: true, Prereq: map[string]any{"vpn_type": "wireguard-server", "vpn_client_configuration_remote_ip_override": "192.0.2.55", "local_port": 51820, "x_wireguard_private_key": "wGwQ7hjIRMxERjBS+iaGXfDcnTAtoQAaHXqIisVWWXg="}},

	// igmp_proxy_downstream_networkconf_ids: shape of a referenced
	// networkconf id; a real id must be substituted to test acceptance live.
	// The rest of this group (fast leave, querier, flood control,
	// suppression) is wired -- see marshalCorporate.
	{Wire: "igmp_proxy_downstream_networkconf_ids", Purpose: PurposeCorporate, Value: []string{"000000000000000000000000"}, Prereq: nil},

	// WAN MTU. The coverage comment attributes this to marshalWAN, not
	// corporate -- Purpose is PurposeWAN here.
	{Wire: "interface_mtu", Purpose: PurposeWAN, Value: 1400, Prereq: map[string]any{"interface_mtu_enabled": true}},
	{Wire: "interface_mtu_enabled", Purpose: PurposeWAN, Value: true, Prereq: nil},

	// Advanced site-vpn IPsec options: IKE identifiers, tunnel IP, separate
	// IKEv2 networks. All require vpn_type "ipsec-vpn" (the only IPsec value
	// in the vpn_type enum) to be meaningful.
	{Wire: "ipsec_local_identifier", Purpose: PurposeSiteVPN, Value: "site-a.vpn.example.com", Prereq: mergePolicyBasedPrereq(map[string]any{"ipsec_local_identifier_enabled": true})},
	{Wire: "ipsec_local_identifier_enabled", Purpose: PurposeSiteVPN, Value: true, Prereq: mergePolicyBasedPrereq(map[string]any{"ipsec_local_identifier": "site-a.vpn.example.com"})},
	{Wire: "ipsec_remote_identifier", Purpose: PurposeSiteVPN, Value: "site-b.vpn.example.com", Prereq: mergePolicyBasedPrereq(map[string]any{"ipsec_remote_identifier_enabled": true})},
	{Wire: "ipsec_remote_identifier_enabled", Purpose: PurposeSiteVPN, Value: true, Prereq: mergePolicyBasedPrereq(map[string]any{"ipsec_remote_identifier": "site-b.vpn.example.com"})},
	{Wire: "ipsec_separate_ikev2_networks", Purpose: PurposeSiteVPN, Value: true, Prereq: mergePolicyBasedPrereq(nil)}, // "separate IKEv2 networks" is a per-child-SA policy-based concept; the controller rejects it in route-based (dynamic routing) mode with api.err.IpsecSeparateIkeV2NetworkRequiresPolicyBased
	{Wire: "ipsec_tunnel_ip", Purpose: PurposeSiteVPN, Value: "192.0.2.4/30", Prereq: mergeRouteBasedPrereq(map[string]any{"ipsec_tunnel_ip_enabled": true})},
	{Wire: "ipsec_tunnel_ip_enabled", Purpose: PurposeSiteVPN, Value: true, Prereq: mergeRouteBasedPrereq(map[string]any{"ipsec_tunnel_ip": "192.0.2.4/30"})},

	// site-vpn emits remote_vpn_subnets but not the dynamic-subnets toggle;
	// pairs naturally with the ipsec_dynamic_routing flag the encoder
	// already sends. Requires a tunnel IP (api.err.IpsecDynamicSubnetsRequireTunnelIp).
	{
		Wire:    "remote_vpn_dynamic_subnets_enabled",
		Purpose: PurposeSiteVPN,
		Value:   true,
		Prereq:  mergeRouteBasedPrereq(map[string]any{"ipsec_tunnel_ip_enabled": true, "ipsec_tunnel_ip": "192.0.2.4/30"}),
	},

	// L2TP remote-user-vpn RADIUS option; the other l2tp_* fields are
	// already emitted. The controller rejects an l2tp-server network with
	// api.err.L2tpPskRequired unless the PSK is present, then with
	// api.err.RadiusProfileRequired unless radiusprofile_id references a
	// real profile ("@radiusprofile" resolves live to the site's default).
	{Wire: "require_mschapv2", Purpose: PurposeUserVPN, Value: true, Prereq: map[string]any{"vpn_type": "l2tp-server", "l2tp_interface": "wan", "x_ipsec_pre_shared_key": "l2tp-s3cret-psk", "radiusprofile_id": "@radiusprofile"}},

	// OpenVPN server protocol for remote-user-vpn; openvpn_mode is already
	// emitted. The controller rejects an openvpn-server network with
	// api.err.MissingLocalPort unless a server local_port is present, then
	// with api.err.RadiusProfileRequired unless radiusprofile_id references
	// a real profile.
	{Wire: "vpn_protocol", Purpose: PurposeUserVPN, Value: "UDP", Prereq: map[string]any{"vpn_type": "openvpn-server", "openvpn_mode": "server", "local_port": 1194, "radiusprofile_id": "@radiusprofile"}},

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
	// The controller rejects a pppoe WAN with api.err.InvalidWanPppoeCredentials
	// unless BOTH wan_username and x_wan_password are present, even when only
	// probing one enable flag.
	{Wire: "wan_pppoe_username_enabled", Purpose: PurposeWAN, Value: true, Prereq: map[string]any{"wan_type": "pppoe", "wan_username": "pppoe-user", "x_wan_password": "pppoe-pass"}},
	{Wire: "wan_pppoe_password_enabled", Purpose: PurposeWAN, Value: true, Prereq: map[string]any{"wan_type": "pppoe", "wan_username": "pppoe-user", "x_wan_password": "pppoe-pass"}},
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
