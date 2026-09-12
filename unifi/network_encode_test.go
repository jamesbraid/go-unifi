package unifi

import (
	"encoding/json"
	"slices"
	"testing"
)

// What a purpose puts on the wire -- which keys, which values, sparse and
// dense, create and update -- is pinned byte-for-byte by the corpus golden
// (TestNetworkEncodeCorpus, testdata/network_encode_corpus.txt), and the
// value each key is sourced from is pinned by TestNetworkEncoderValueFlow.
// The tests here pin the rules the encoder applies on top of the generated
// declaration: the outputs the golden records would still be reproducible
// from different inputs, so each rule keeps a test that states its inputs
// and the measurement behind it.

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

// TestMarshalNetworkDHCPRangeDerivedOnlyOnCreate pins where the DHCP range
// derivation is allowed to fire. A create (no _id yet) without a range is
// rejected by the controller, so deriving one from ip_subnet there is
// load-bearing. An update carrying the same derived range asserts values the
// caller never set and the controller never stored, so with an _id the
// encoder passes the caller's values through untouched.
func TestMarshalNetworkDHCPRangeDerivedOnlyOnCreate(t *testing.T) {
	for _, purpose := range []string{PurposeCorporate, PurposeGuest} {
		t.Run(purpose, func(t *testing.T) {
			// Create: no _id, no range set. The derived defaults go out.
			got := marshalKeys(t, &Network{
				Purpose:  purpose,
				Enabled:  true,
				IPSubnet: strPtr("192.168.1.0/24"),
			})
			if got["dhcpd_start"] != "192.168.1.6" || got["dhcpd_stop"] != "192.168.1.254" {
				t.Errorf("create range = %v-%v, want the derived 192.168.1.6-192.168.1.254",
					got["dhcpd_start"], got["dhcpd_stop"])
			}

			// Update: _id set, no range set. No range may be invented.
			got = marshalKeys(t, &Network{
				ID:       "507f1f77bcf86cd799439011",
				Purpose:  purpose,
				Enabled:  true,
				IPSubnet: strPtr("192.168.1.0/24"),
			})
			for _, key := range []string{"dhcpd_start", "dhcpd_stop"} {
				if v, ok := got[key]; ok {
					t.Errorf("update invented %s = %v; the caller set no range", key, v)
				}
			}

			// Update with an explicit range: the caller's values go out.
			got = marshalKeys(t, &Network{
				ID:         "507f1f77bcf86cd799439011",
				Purpose:    purpose,
				Enabled:    true,
				IPSubnet:   strPtr("192.168.1.0/24"),
				DHCPDStart: strPtr("192.168.1.100"),
				DHCPDStop:  strPtr("192.168.1.200"),
			})
			if got["dhcpd_start"] != "192.168.1.100" || got["dhcpd_stop"] != "192.168.1.200" {
				t.Errorf("explicit range came out as %v-%v, want the caller's 192.168.1.100-192.168.1.200",
					got["dhcpd_start"], got["dhcpd_stop"])
			}
		})
	}
}

// TestMarshalNetworkVLANEnabledInference pins the vlan-only inference: a
// VLAN id with the flag left off is turned on, because the controller
// otherwise ignores the id, falls back to VLAN 1 and rejects the create with
// api.err.VlanUsed naming the Default network (measured on 10.4.57). The
// inference belongs to vlan-only alone; every other purpose passes the flag
// through as the caller set it.
func TestMarshalNetworkVLANEnabledInference(t *testing.T) {
	vlan := int64(7)
	zero := int64(0)

	cases := []struct {
		name string
		n    *Network
		want bool
	}{
		{"vlan-only id without flag", &Network{Purpose: PurposeVLANOnly, VLAN: &vlan}, true},
		{"vlan-only id with flag", &Network{Purpose: PurposeVLANOnly, VLAN: &vlan, VLANEnabled: true}, true},
		{"vlan-only no id", &Network{Purpose: PurposeVLANOnly}, false},
		{"vlan-only id zero", &Network{Purpose: PurposeVLANOnly, VLAN: &zero}, false},
		{"corporate id without flag", &Network{Purpose: PurposeCorporate, VLAN: &vlan}, false},
	}
	for _, tc := range cases {
		if got := marshalKeys(t, tc.n)["vlan_enabled"]; got != tc.want {
			t.Errorf("%s: vlan_enabled = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestMarshalNetworkWANIPv6Enabled pins the synthetic ipv6_enabled a WAN
// network sends: derived from wan_type_v6, true only for a real value. The
// key exists nowhere on the generated struct, so nothing but this derivation
// decides it -- and an unset or empty wan_type_v6 must not leave
// ipv6_enabled asserting IPv6 off a field that was not sent.
func TestMarshalNetworkWANIPv6Enabled(t *testing.T) {
	cases := []struct {
		name string
		v6   *string
		want bool
	}{
		{"unset", nil, false},
		{"empty", strPtr(""), false},
		{"disabled", strPtr("disabled"), false},
		{"dhcpv6", strPtr("dhcpv6"), true},
	}
	for _, tc := range cases {
		got := marshalKeys(t, &Network{Purpose: PurposeWAN, WANTypeV6: tc.v6})["ipv6_enabled"]
		if got != tc.want {
			t.Errorf("wan_type_v6 %s: ipv6_enabled = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestNetworkAlwaysArraysReachTheWire pins the always-array exception as a
// rule rather than as bytes: every networkAlwaysArrays entry reaches the
// wire as [] from every purpose that sends it, even though the caller left
// the slice nil and the generated declaration says omitempty. The controller
// wants an array for these; for remote_vpn_subnets a site-to-site create
// without the key is refused outright (measured on 10.6.101).
func TestNetworkAlwaysArraysReachTheWire(t *testing.T) {
	covered := map[string]bool{}
	for _, purpose := range NetworkPurposes {
		emitted := marshalKeys(t, &Network{Purpose: purpose})
		for wire := range networkAlwaysArrays {
			if !slices.Contains(networkPurposeFields[purpose], wire) {
				continue
			}
			covered[wire] = true
			if v, ok := emitted[wire].([]any); !ok || len(v) != 0 {
				t.Errorf("%s: %s = %v for a nil slice, want []", purpose, wire, emitted[wire])
			}
		}
	}
	for wire := range networkAlwaysArrays {
		if !covered[wire] {
			t.Errorf("%s is in networkAlwaysArrays but no purpose sends it; the entry is dead", wire)
		}
	}
}

// Helper function to create string pointers.
func strPtr(s string) *string {
	return &s
}
