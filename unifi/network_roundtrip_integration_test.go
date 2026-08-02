//go:build integration

// unifi/network_roundtrip_integration_test.go
package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationNetworkRoundTrip checks that decoding a controller's own
// representation of a network and re-encoding it loses nothing.
//
// This is the guard the dhcpd_ip_1 bug slipped past.
// TestNetworkEncoderCoversGeneratedFields unions the emitted wire names
// ACROSS purposes, so marshalVLANOnly emitting dhcpd_ip_1 was enough to
// satisfy it while marshalCorporate and marshalGuest silently dropped the
// field. Presence in some purpose says nothing about the purpose you are
// actually writing. Here each purpose is checked against a real object the
// controller stored, which is the only thing that knows what a corporate
// network is supposed to carry.
//
// Two failure directions, both real bugs:
//
//	LOST     the controller holds a value and the encoder emits no such key
//	MUTATED  the encoder emits the key with a different value than was stored
//
// Networks are seeded with raw payloads, never through the encoder, so a
// field the encoder cannot express still gets set on the object and the
// round trip has something to lose.
func TestIntegrationNetworkRoundTrip(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	known := map[string]bool{}
	for _, w := range networkWireNames(t) {
		known[w] = true
	}

	// remote-user-vpn authenticates against the built-in RADIUS server, which
	// has to be running before the controller will accept the network.
	setSiteRadiusEnabled(ctx, t, s, c.Site, true)
	radiusProfile := builtinRadiusProfileID(ctx, t, s, c.Site)

	for _, tc := range roundTripSeeds() {
		t.Run(tc.purpose, func(t *testing.T) {
			for k, v := range tc.seed {
				if v == "@radiusprofile" {
					tc.seed[k] = radiusProfile
				}
			}
			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", tc.seed)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}
			if status != 200 {
				t.Fatalf("seed rejected (HTTP %d): %v", status, body)
			}
			stored := firstData(t, body)
			if id, _ := stored["_id"].(string); id != "" {
				defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck
			}

			raw, err := json.Marshal(stored)
			if err != nil {
				t.Fatalf("re-marshal stored: %v", err)
			}
			var decoded Network
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("decode stored into Network: %v", err)
			}
			out, err := json.Marshal(&decoded)
			if err != nil {
				t.Fatalf("re-encode: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("unmarshal re-encoded: %v", err)
			}

			lost, mutated := []string{}, []string{}
			for wire, want := range stored {
				if !known[wire] || networkEncoderAllowlist[wire] {
					continue // not a Network field, or intentionally never sent
				}
				have, emitted := got[wire]
				switch {
				case !emitted:
					if !isZeroJSON(want) {
						lost = append(lost, fmt.Sprintf("%s (stored %v)", wire, want))
					}
				case !jsonEqual(have, want):
					mutated = append(mutated, fmt.Sprintf("%s (stored %v, re-encoded %v)", wire, want, have))
				}
			}
			sort.Strings(lost)
			sort.Strings(mutated)

			for _, f := range lost {
				t.Errorf("LOST %s: the controller holds this value and the encoder emits no such key", f)
			}
			for _, f := range mutated {
				t.Errorf("MUTATED %s: the encoder changed a value it read back", f)
			}
		})
	}
}

type roundTripSeed struct {
	purpose string
	seed    map[string]any
}

// roundTripSeeds builds one richly-populated raw payload per purpose that a
// bare simulation controller will accept.
//
// The three VPN purposes were long omitted, on the grounds that they needed
// peer addressing, key material and a gateway-bound local IP the harness
// could not supply. Re-measured: that is not what blocks them. Each simply
// has required fields, and the controller names the one it is missing --
// remote-user-vpn wants local_port and a radiusprofile_id, vpn-client wants
// an ip_subnet and then interface DNS (api.err.WireguardMissingInterfaceDns).
// Supply those and all seven purposes round-trip on a bare controller, which
// is what closes the encoder's biggest blind spot: three of its seven
// marshalers had no round-trip coverage at all.
//
// "@radiusprofile" is resolved to the site's built-in profile before send.
func roundTripSeeds() []roundTripSeed {
	return []roundTripSeed{
		{
			purpose: PurposeCorporate,
			seed: map[string]any{
				"name": "rt-corporate", "purpose": PurposeCorporate, "enabled": true,
				"ip_subnet": "10.91.10.1/24", "vlan_enabled": true, "vlan": 810,
				"setting_preference": "manual", "networkgroup": "LAN",
				"domain_name": "rt.example", "igmp_snooping": true,
				"network_isolation_enabled": true, "mdns_enabled": true,
				"upnp_lan_enabled": true, "dhcpguard_enabled": true,
				"dhcpd_ip_1": "10.91.10.2", "dhcpd_mac_1": "00:11:22:33:44:55",
				"dhcpd_enabled": true, "dhcpd_start": "10.91.10.6", "dhcpd_stop": "10.91.10.254",
				"dhcpd_leasetime": 7200, "dhcpd_dns_enabled": true, "dhcpd_dns_1": "10.91.10.53",
				"dhcpd_gateway_enabled": true, "dhcpd_gateway": "10.91.10.1",
				"dhcpd_ntp_enabled": true, "dhcpd_ntp_1": "10.91.10.123",
				"dhcpd_conflict_checking": true,
			},
		},
		{
			purpose: PurposeGuest,
			seed: map[string]any{
				"name": "rt-guest", "purpose": PurposeGuest, "enabled": true,
				"ip_subnet": "10.91.20.1/24", "vlan_enabled": true, "vlan": 820,
				"setting_preference": "manual", "networkgroup": "LAN",
				"igmp_snooping": true, "mdns_enabled": true,
				"dhcpguard_enabled": true, "dhcpd_ip_1": "10.91.20.2",
				"dhcpd_enabled": true, "dhcpd_start": "10.91.20.6", "dhcpd_stop": "10.91.20.254",
				"dhcpd_dns_enabled": true, "dhcpd_dns_1": "10.91.20.53",
			},
		},
		{
			purpose: PurposeVLANOnly,
			seed: map[string]any{
				"name": "rt-vlanonly", "purpose": PurposeVLANOnly, "enabled": false,
				"networkgroup": "LAN", "vlan_enabled": true, "vlan": 830,
				// network_isolation_enabled is corporate-only: the controller
				// answers api.err.NetworkIsolationAppliedOnNonCorporateNetwork.
				"igmp_snooping": true, "mdns_enabled": false,
				"dhcpguard_enabled": true, "dhcpd_ip_1": "10.91.30.2",
			},
		},
		{
			purpose: PurposeWAN,
			seed: map[string]any{
				"name": "rt-wan", "purpose": PurposeWAN, "enabled": true,
				"wan_networkgroup": "WAN2", "wan_type": "dhcp", "wan_type_v6": "disabled",
				"wan_dns_preference": "manual", "wan_dns1": "10.91.40.53",
				"wan_load_balance_type": "failover-only", "wan_failover_priority": 2,
				"report_wan_event": true, "wan_smartq_enabled": false,
			},
		},
		{
			purpose: PurposeSiteVPN,
			seed: map[string]any{
				"name": "rt-site-vpn", "purpose": PurposeSiteVPN, "enabled": true,
				"vpn_type": "ipsec-vpn", "ipsec_interface": "wan",
				"ipsec_peer_ip": "203.0.113.9", "ipsec_key_exchange": "ikev2",
				"x_ipsec_pre_shared_key": "s3cret-psk", "ipsec_profile": "customized",
				"ipsec_encryption": "aes256", "ipsec_hash": "sha256", "ipsec_dh_group": 14,
				"ipsec_esp_encryption": "aes256", "ipsec_esp_hash": "sha256",
				"ipsec_esp_dh_group": 14, "remote_vpn_subnets": []string{"192.0.2.0/24"},
			},
		},
		{
			purpose: PurposeVPNClient,
			seed: map[string]any{
				"name": "rt-vpn-client", "purpose": PurposeVPNClient, "enabled": true,
				"vpn_type": "wireguard-client", "wireguard_client_mode": "manual",
				"ip_subnet":                "10.198.0.1/24",
				"wireguard_client_peer_ip": "203.0.113.20", "wireguard_client_peer_port": 51820,
				"wireguard_client_peer_public_key": "yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=",
				"x_wireguard_private_key":          "6KpcbNfK7kFzOlKjnDbSaYbmDbAZBOKwFqjOWMkSCFU=",
				"vpn_client_pull_dns":              false,
				"dhcpd_dns_enabled":                true, "dhcpd_dns_1": "1.1.1.1",
			},
		},
		{
			purpose: PurposeUserVPN,
			seed: map[string]any{
				"name": "rt-user-vpn", "purpose": PurposeUserVPN, "enabled": true,
				"vpn_type": "openvpn-server", "openvpn_mode": "server",
				"openvpn_encryption_cipher": "AES_256_CBC",
				"ip_subnet":                 "10.199.0.1/24", "local_port": 1195,
				"radiusprofile_id":          "@radiusprofile",
			},
		},
	}
}
