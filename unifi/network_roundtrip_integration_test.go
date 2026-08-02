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

	for _, tc := range roundTripSeeds() {
		t.Run(tc.purpose, func(t *testing.T) {
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
// bare simulation controller will accept. site-vpn, vpn-client and
// remote-user-vpn are omitted: they need peer addressing, key material and a
// gateway-bound local IP that this harness cannot supply (see
// networkFieldCandidates for the measured rejections).
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
	}
}
