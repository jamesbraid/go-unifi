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
// mergeRouteBasedPrereq layers the route-based (dynamic routing) pairing plus
// each candidate's own gating fields on top via extra.
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
// network_encode_coverage_test.go that is still a TODO after Task 4's wiring
// pass -- one fieldCandidate per wire name that came back STRIPPED or
// REJECTED from the live probe (see network_encode_coverage_test.go for the
// measured outcome of each). Values follow the generated struct's type and
// validation comment for each wire name (see network.generated.go); Prereq
// carries the sibling fields (mode selectors, enable flags) the controller
// needs before it will accept the candidate value at all.
//
// This table originally held 39 entries (one per allowlisted wire name at
// the start of Task 4). 35 came back PERSISTED -- some only after their
// Prereq was enriched with a sibling the controller required first (a
// local_port, an L2TP PSK, both PPPoE credentials, a real radiusprofile_id,
// or a fuller site-vpn base config; see the corresponding git history for
// the before/after) -- and were wired into network_encode.go and removed
// from both this table and networkEncoderPresenceAllowlistTODOs. The 4
// remaining below stayed STRIPPED/REJECTED after that same enrichment
// treatment.
var networkFieldCandidates = []fieldCandidate{
	// igmp_proxy_downstream_networkconf_ids: shape of a referenced networkconf
	// id; a real id must be substituted to test acceptance live.
	{Wire: "igmp_proxy_downstream_networkconf_ids", Purpose: PurposeCorporate, Value: []string{"000000000000000000000000"}, Prereq: nil},

	// Route-based (dynamic routing) site-vpn tunnel IP. Requires vpn_type
	// "ipsec-vpn" and ipsec_dynamic_routing true to be meaningful.
	{Wire: "ipsec_tunnel_ip", Purpose: PurposeSiteVPN, Value: "192.0.2.4/30", Prereq: mergeRouteBasedPrereq(map[string]any{"ipsec_tunnel_ip_enabled": true})},
	{Wire: "ipsec_tunnel_ip_enabled", Purpose: PurposeSiteVPN, Value: true, Prereq: mergeRouteBasedPrereq(map[string]any{"ipsec_tunnel_ip": "192.0.2.4/30"})},

	// site-vpn emits remote_vpn_subnets but not the dynamic-subnets toggle;
	// pairs naturally with the ipsec_dynamic_routing flag the encoder already
	// sends. Requires a tunnel IP (api.err.IpsecDynamicSubnetsRequireTunnelIp).
	{
		Wire:    "remote_vpn_dynamic_subnets_enabled",
		Purpose: PurposeSiteVPN,
		Value:   true,
		Prereq:  mergeRouteBasedPrereq(map[string]any{"ipsec_tunnel_ip_enabled": true, "ipsec_tunnel_ip": "192.0.2.4/30"}),
	},
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
