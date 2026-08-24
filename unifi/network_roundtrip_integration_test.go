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
// Three failure directions, all real bugs:
//
//	LOST       the controller holds a value and the encoder emits no such key
//	MUTATED    the encoder emits the key with a different value than was stored
//	DISCARDED  the seed asked for a value, the controller answered rc: ok, and
//	           stored something else
//
// The first two check the encoder against the controller. DISCARDED checks
// the controller against the caller, and it is a direction nothing here used
// to check: `stored` is the baseline for LOST and MUTATED, so a value the
// controller quietly refused at seed time is simply absent from the baseline
// and both directions agree it round-tripped fine. That is how
// setting_preference "auto" hid -- it makes the controller own six of the
// advanced toggles and store its own values for them, with no error, and a
// guard anchored on `stored` can never see it.
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
		// Named, not keyed by purpose: two seeds share PurposeCorporate --
		// one per setting_preference mode -- so the purpose alone would
		// give them the same subtest name.
		t.Run(tc.name, func(t *testing.T) {
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

			checkDiscarded(t, tc, stored)

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
	name    string
	purpose string
	seed    map[string]any

	// wantDiscarded lists the wire names the controller is known to store
	// differently from what the seed asked for, with no error. A well-formed
	// seed has none -- anything discarded is the controller refusing a write
	// quietly, which is the whole point of the direction. rt-corporate-auto
	// is the deliberate exception: it exists to hold the measured cost of
	// setting_preference "auto" and must never be emptied to make CI green.
	wantDiscarded []string
}

// discardedFields returns the keys of asked that the controller did not store
// as asked, mapped to a human-readable before/after. A missing key counts as
// discarded: the controller dropped it rather than storing it.
//
// Shared with TestIntegrationPreferenceOwnership, which subtracts one call's
// result from another's to isolate what a mode owns.
func discardedFields(asked, stored map[string]any) map[string]string {
	out := map[string]string{}
	for wire, want := range asked {
		have, ok := stored[wire]
		if ok && jsonEqual(have, want) {
			continue
		}
		out[wire] = describeDifference(want, have)
	}
	return out
}

// describeDifference renders a before/after that a reader can act on.
//
// %v alone is not enough: the controller returns some numeric fields as JSON
// strings, so a values-differ report can read "asked [1 6 11], stored
// [1 6 11]" and look like a bug in the comparison. When the rendered forms
// match, the difference is representation, and saying so is the whole
// message.
func describeDifference(want, have any) string {
	wantStr := fmt.Sprintf("%v", want)
	haveStr := fmt.Sprintf("%v", have)
	if wantStr == haveStr {
		return fmt.Sprintf("asked %s (%T), stored the same value as %T", wantStr, want, have)
	}
	return fmt.Sprintf("asked %s, stored %s", wantStr, haveStr)
}

// checkDiscarded compares what the seed asked for against what the controller
// stored, and pins the difference. Both directions are errors: a field the
// controller newly refuses is a regression to find, and a field it stopped
// refusing means the controller changed and the table is now a lie.
func checkDiscarded(t *testing.T, tc roundTripSeed, stored map[string]any) {
	t.Helper()

	want := map[string]bool{}
	for _, wire := range tc.wantDiscarded {
		want[wire] = true
	}

	detail := discardedFields(tc.seed, stored)

	got := make([]string, 0, len(detail))
	for wire := range detail {
		got = append(got, wire)
	}
	sort.Strings(got)

	for _, wire := range got {
		if !want[wire] {
			t.Errorf("DISCARDED %s (%s): the controller accepted the write with rc: ok and stored something else",
				wire, detail[wire])
		}
	}
	for _, wire := range tc.wantDiscarded {
		if _, ok := detail[wire]; !ok {
			t.Errorf("%s is no longer discarded: the controller stored what the seed asked for.\n"+
				"The controller's behaviour changed -- remove it from wantDiscarded once that is understood.", wire)
		}
	}
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
			name:    "rt-corporate",
			purpose: PurposeCorporate,
			seed:    corporateRoundTripSeed("rt-corporate", 10, 810, "manual"),
		},
		{
			// The same payload as rt-corporate with one key changed, so
			// whatever this seed loses is the mode's doing and nothing else.
			// Pinned, not fixed: the SDK sends what it is given, and a caller
			// who asks for "auto" gets the controller's advanced block. What
			// was missing was any record of the price.
			name:    "rt-corporate-auto",
			purpose: PurposeCorporate,
			seed:    corporateRoundTripSeed("rt-corporate-auto", 50, 850, "auto"),
			// Measured on 10.4.57. Nine, not the six c476b82 recorded: that
			// sweep walked the *_enabled toggles, so the two fields below
			// that are not toggles were never in view.
			wantDiscarded: []string{
				// The six from c476b82.
				"dhcpguard_enabled",
				"igmp_snooping",
				"upnp_lan_enabled",
				"dhcpd_dns_enabled",
				"dhcpd_ntp_enabled",
				"dhcpd_time_offset_enabled",
				// A seventh toggle, missed there because that measurement
				// worked from the encoder's fields and this one was not
				// among the six it compared.
				"dhcpd_gateway_enabled",
				// Not toggles, and the reason a toggle-shaped sweep could
				// never have found them: auto replaces the lease time with
				// the controller's own 86400 and blanks the search domain.
				"dhcpd_leasetime",
				"domain_name",
			},
		},
		{
			name:    "rt-guest",
			purpose: PurposeGuest,
			seed: map[string]any{
				"name": "rt-guest", "purpose": PurposeGuest, "enabled": true,
				"ip_subnet": "10.91.20.1/24", "vlan_enabled": true, "vlan": 820,
				"setting_preference": "manual", "networkgroup": "LAN",
				"igmp_snooping": true, "mdns_enabled": true,
				"dhcpguard_enabled": true, "dhcpd_ip_1": "10.91.20.2",
				"dhcpd_enabled": true, "dhcpd_start": "10.91.20.6", "dhcpd_stop": "10.91.20.254",
				"dhcpd_dns_enabled": true, "dhcpd_dns_1": "10.91.20.53",
				"dhcpd_dns_2": "10.91.20.54", "dhcpd_dns_3": "9.9.9.9", "dhcpd_dns_4": "149.112.112.112",
			},
		},
		{
			name:    "rt-vlanonly",
			purpose: PurposeVLANOnly,
			seed: map[string]any{
				"name": "rt-vlanonly", "purpose": PurposeVLANOnly, "enabled": false,
				"networkgroup": "LAN", "vlan_enabled": true, "vlan": 830,
				// network_isolation_enabled is corporate-only: the controller
				// answers api.err.NetworkIsolationAppliedOnNonCorporateNetwork.
				"igmp_snooping": true, "mdns_enabled": false,
				"dhcpguard_enabled": true, "dhcpd_ip_1": "10.91.30.2",
			},
			// Measured on 10.4.57, and not a mode effect -- this seed sets no
			// setting_preference at all. The controller stores true over an
			// explicit false, so a caller cannot turn mDNS off on a vlan-only
			// network through this API and is not told. Recorded here because
			// it is real; why it happens is not established.
			wantDiscarded: []string{"mdns_enabled"},
		},
		{
			name:    "rt-wan",
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
				"dhcpd_dns_2": "1.0.0.1", "dhcpd_dns_3": "9.9.9.9", "dhcpd_dns_4": "149.112.112.112",
			},
		},
		{
			purpose: PurposeUserVPN,
			seed: map[string]any{
				"name": "rt-user-vpn", "purpose": PurposeUserVPN, "enabled": true,
				"vpn_type": "openvpn-server", "openvpn_mode": "server",
				"openvpn_encryption_cipher": "AES_256_CBC",
				"ip_subnet":                 "10.199.0.1/24", "local_port": 1195,
				"dhcpd_dns_enabled": true, "dhcpd_dns_1": "1.1.1.1",
				"dhcpd_dns_2": "1.0.0.1", "dhcpd_dns_3": "9.9.9.9", "dhcpd_dns_4": "149.112.112.112",
				"radiusprofile_id": "@radiusprofile",
			},
		},
	}
}

// corporateRoundTripSeed builds the corporate payload both corporate seeds
// use. They differ only in name, addressing and setting_preference, and that
// is the point: two hand-written maps would drift, and a differential whose
// two arms differ in some other way measures nothing.
//
// dhcpd_time_offset_enabled is here rather than in the original rt-corporate
// map because it is one of the toggles "auto" takes over, and a differential
// cannot show a field being taken over if neither arm sets it.
func corporateRoundTripSeed(name string, octet, vlan int, preference string) map[string]any {
	net := fmt.Sprintf("10.91.%d", octet)
	return map[string]any{
		"name": name, "purpose": PurposeCorporate, "enabled": true,
		"ip_subnet": net + ".1/24", "vlan_enabled": true, "vlan": vlan,
		"setting_preference": preference, "networkgroup": "LAN",
		"domain_name": "rt.example", "igmp_snooping": true,
		"network_isolation_enabled": true, "mdns_enabled": true,
		"upnp_lan_enabled": true, "dhcpguard_enabled": true,
		"dhcpd_ip_1": net + ".2", "dhcpd_mac_1": "00:11:22:33:44:55",
		"dhcpd_enabled": true, "dhcpd_start": net + ".6", "dhcpd_stop": net + ".254",
		"dhcpd_leasetime": 7200, "dhcpd_dns_enabled": true, "dhcpd_dns_1": net + ".53",
		"dhcpd_dns_2": net + ".54", "dhcpd_dns_3": "9.9.9.9", "dhcpd_dns_4": "149.112.112.112",
		"dhcpd_gateway_enabled": true, "dhcpd_gateway": net + ".1",
		"dhcpd_ntp_enabled": true, "dhcpd_ntp_1": net + ".123",
		"dhcpd_time_offset_enabled": true, "dhcpd_time_offset": 3600,
		"dhcpd_conflict_checking": true,
	}
}
