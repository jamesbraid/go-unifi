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

// probeTunnelIP is the route-based site-vpn tunnel address the probe uses. It
// has to miss every other subnet in play at once: the probe's own scratch
// networks (10.99.x/24), the site LAN (192.168.x), remote_vpn_subnets
// (192.0.2.0/24) and the seeded probe WAN (203.0.113.0/24). A /30 in the
// RFC1918 172.16/12 block collides with none of them.
const probeTunnelIP = "172.31.99.1/30"

// siteVPNBasePrereq is the minimum realistic IPsec site-vpn payload the
// simulated 10.x controller accepts; a bare vpn_type + one advanced field
// gets api.err.Invalid with no further detail. Peer addressing, a
// "customized" profile with explicit phase 1/2 parameters, a PSK, and a
// remote subnet round out a config the controller will actually persist.
// ipsec_local_ip / ipsec_dynamic_routing are deliberately not here; they are
// route-based-only and mergeRouteBasedPrereq layers them (plus each
// candidate's own gating fields) on top via extra.
//
// ipsec_interface is "@wanif" rather than a literal: the probe seeds its own
// static WAN and substitutes the slot it landed in (see ensureStaticWAN in
// network_field_probe_integration_test.go).
var siteVPNBasePrereq = map[string]any{
	"vpn_type":               "ipsec-vpn",
	"ipsec_interface":        "@wanif",
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
// mode, which requires a real (non-"any") ipsec_local_ip. The controller
// further requires that address be one it recognizes as local to the site
// (api.err.UnrecognizedLocalIp). An earlier pass used a literal 198.51.100.5,
// which is in no such set, and recorded the resulting rejection as the
// candidates' verdict -- a non-result, not a measurement. "@wanlocalip" is
// substituted by the probe with the wan_ip of a static WAN it seeds first
// (ensureStaticWAN); ipsec_separate_ikev2_networks is pinned false because
// dynamic routing and separate IKEv2 child SAs are mutually exclusive.
func mergeRouteBasedPrereq(extra map[string]any) map[string]any {
	return mergePrereq(siteVPNBasePrereq, mergePrereq(map[string]any{
		"ipsec_local_ip":                "@wanlocalip",
		"ipsec_dynamic_routing":         true,
		"ipsec_separate_ikev2_networks": false,
	}, extra))
}

// networkFieldCandidates lists every networkEncoderAllowlist entry in
// network_encode_coverage_test.go that is still a TODO after the probe sweep's wiring
// pass -- one fieldCandidate per wire name that came back STRIPPED or
// REJECTED from the live probe (see network_encode_coverage_test.go for the
// measured outcome of each). Values follow the generated struct's type and
// validation comment for each wire name (see network.generated.go); Prereq
// carries the sibling fields (mode selectors, enable flags) the controller
// needs before it will accept the candidate value at all.
//
// This table originally held 39 entries (one per allowlisted wire name at
// the start of the sweep). 35 came back PERSISTED -- some only after their
// Prereq was enriched with a sibling the controller required first (a
// local_port, an L2TP PSK, both PPPoE credentials, a real radiusprofile_id,
// or a fuller site-vpn base config; see the corresponding git history for
// the before/after) -- and were wired into network_encode.go and removed
// from both this table and networkEncoderPresenceAllowlistTODOs.
//
// Of the 4 left, the 3 route-based site-vpn fields came back PERSISTED in
// 2026-08 once the probe seeded a static WAN for their ipsec_local_ip. They
// stay here until the encoder emits them; the table is then their regression
// cover, since a controller that stops persisting one logs STRIPPED.
var networkFieldCandidates = []fieldCandidate{
	// igmp_proxy_downstream_networkconf_ids: shape of a referenced networkconf
	// id; a real id must be substituted to test acceptance live.
	{
		Wire:    "igmp_proxy_downstream_networkconf_ids",
		Purpose: PurposeCorporate,
		Value:   []string{"000000000000000000000000"},
		Prereq:  nil,
	},

	// Route-based (dynamic routing) site-vpn tunnel IP. Requires vpn_type
	// "ipsec-vpn" and ipsec_dynamic_routing true to be meaningful.
	//
	// probeTunnelIP is deliberately outside remote_vpn_subnets: 192.0.2.4/30
	// (used until the 2026-08 re-probe) sits inside the 192.0.2.0/24 the same
	// payload declares as the remote subnet, which is its own rejection.
	// Each candidate also gets its own ipsec_peer_ip so a create that outlives
	// its cleanup cannot collide with the next one
	// (api.err.IpsecPeerIpOverlapped keys on peer IP + interface + local IP).
	{
		Wire:    "ipsec_tunnel_ip",
		Purpose: PurposeSiteVPN,
		Value:   probeTunnelIP,
		Prereq: mergeRouteBasedPrereq(
			map[string]any{"ipsec_peer_ip": "203.0.113.11", "ipsec_tunnel_ip_enabled": true},
		),
	},
	{
		Wire:    "ipsec_tunnel_ip_enabled",
		Purpose: PurposeSiteVPN,
		Value:   true,
		Prereq: mergeRouteBasedPrereq(
			map[string]any{"ipsec_peer_ip": "203.0.113.12", "ipsec_tunnel_ip": probeTunnelIP},
		),
	},

	// site-vpn emits remote_vpn_subnets but not the dynamic-subnets toggle;
	// pairs naturally with the ipsec_dynamic_routing flag the encoder already
	// sends. Requires a tunnel IP (api.err.IpsecDynamicSubnetsRequireTunnelIp).
	{
		Wire:    "remote_vpn_dynamic_subnets_enabled",
		Purpose: PurposeSiteVPN,
		Value:   true,
		Prereq: mergeRouteBasedPrereq(
			map[string]any{
				"ipsec_peer_ip":           "203.0.113.13",
				"ipsec_tunnel_ip_enabled": true,
				"ipsec_tunnel_ip":         probeTunnelIP,
			},
		),
	},

	// Added by the 10.6.101 regeneration. Purposes below are where each
	// field's neighbours already live, which is a starting point for the
	// probe, not a claim: the verdict the probe measures is what decides
	// whether the encoder emits it.

	// WiFi uplink ("WiFi tethering") WAN: a WAN whose upstream is a wireless
	// network rather than a wired port. The six fields describe that radio
	// link, so they are probed together on one WAN and gated by the flag
	// that turns the mode on.
	{Wire: "is_wifi_tethering", Purpose: PurposeWAN, Value: true, Prereq: wifiUplinkWANPrereq(nil)},
	{
		Wire:    "uplink_ssid",
		Purpose: PurposeWAN,
		Value:   "probe-uplink",
		Prereq:  wifiUplinkWANPrereq(nil),
	},
	{
		Wire:    "uplink_band",
		Purpose: PurposeWAN,
		Value:   "ng",
		Prereq:  wifiUplinkWANPrereq(map[string]any{"uplink_ssid": "probe-uplink"}),
	},
	{
		Wire:    "uplink_security",
		Purpose: PurposeWAN,
		Value:   "WPA2-Personal",
		Prereq:  wifiUplinkWANPrereq(map[string]any{"uplink_ssid": "probe-uplink"}),
	},
	{
		Wire:    "x_uplink_password",
		Purpose: PurposeWAN,
		Value:   "probe-uplink-pass",
		Prereq: wifiUplinkWANPrereq(
			map[string]any{"uplink_ssid": "probe-uplink", "uplink_security": "WPA2-Personal"},
		),
	},
	{
		Wire:    "uplink_identity",
		Purpose: PurposeWAN,
		Value:   "probe-identity",
		Prereq: wifiUplinkWANPrereq(
			map[string]any{"uplink_ssid": "probe-uplink", "uplink_security": "WPA2-Enterprise"},
		),
	},

	// RFC 4638 is the PPPoE jumbo-MTU option, so it is probed on a PPPoE
	// WAN rather than the DHCP one the other WAN candidates use.
	{
		Wire:    "wan_pppoe_rfc4638_enabled",
		Purpose: PurposeWAN,
		Value:   true,
		Prereq: mergePrereq(probeWANBase, map[string]any{
			"wan_type": "pppoe", "wan_username": "probe-user", "x_wan_password": "probe-pass",
			"wan_pppoe_username_enabled": true, "wan_pppoe_password_enabled": true,
		}),
	},

	// SD-WAN marks a WAN as an underlay for the overlay fabric.
	{Wire: "sdwan_underlay", Purpose: PurposeWAN, Value: true, Prereq: probeWANBase},

	// Which local subnets a site-to-site tunnel exposes to the peer. The
	// mode selects between all networks, named networks, and literal
	// subnets, so each of the three is probed under the mode that uses it.
	{
		Wire:    "local_vpn_subnets_mode",
		Purpose: PurposeSiteVPN,
		Value:   "custom",
		Prereq:  mergeRouteBasedPrereq(map[string]any{"ipsec_peer_ip": "203.0.113.21"}),
	},
	{
		Wire:    "local_vpn_subnets",
		Purpose: PurposeSiteVPN,
		Value:   []string{"10.98.0.0/24"},
		Prereq: mergeRouteBasedPrereq(
			map[string]any{"ipsec_peer_ip": "203.0.113.22", "local_vpn_subnets_mode": "custom"},
		),
	},
	{
		Wire:    "local_vpn_networkconf_ids",
		Purpose: PurposeSiteVPN,
		Value:   []string{"@defaultnetwork"},
		Prereq: mergeRouteBasedPrereq(
			map[string]any{
				"ipsec_peer_ip":          "203.0.113.23",
				"local_vpn_subnets_mode": "selected_networks",
			},
		),
	},

	// OpenVPN compression is a property of the OpenVPN remote-user server,
	// so it is probed on the same shape the round trip already establishes
	// the controller accepts for that purpose.
	{
		Wire:    "openvpn_compression_disabled",
		Purpose: PurposeUserVPN,
		Value:   true,
		Prereq: map[string]any{
			"vpn_type": "openvpn-server", "openvpn_mode": "server",
			"openvpn_encryption_cipher": "AES_256_CBC",
			"ip_subnet":                 "10.197.0.1/24", "local_port": 1196,
			"radiusprofile_id": "@radiusprofile",
		},
	},
}

// probeWANBase is a WAN the controller accepts: a DHCP upstream in a slot
// the probe's own seeded static WAN does not hold. Each candidate network is
// deleted when its subtest ends, so one slot serves them all.
var probeWANBase = map[string]any{
	"wan_networkgroup":      "WAN4",
	"wan_type":              "dhcp",
	"wan_type_v6":           "disabled",
	"wan_load_balance_type": "failover-only",
	"report_wan_event":      false,
}

// wifiUplinkWANPrereq is probeWANBase with WiFi tethering turned on, plus
// whichever sibling fields the candidate under test needs to be meaningful.
// is_wifi_tethering is itself a candidate, so it appears here as a prereq
// for the others and as the value under test for itself -- the probe
// overwrites the key it is measuring.
func wifiUplinkWANPrereq(extra map[string]any) map[string]any {
	return mergePrereq(mergePrereq(probeWANBase, map[string]any{"is_wifi_tethering": true}), extra)
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
