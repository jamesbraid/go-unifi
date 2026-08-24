package unifi

import (
	"fmt"
	"sort"
	"testing"
)

// TestNetworkEncoderPerPurposeRequiredKeys pins wire keys that a SPECIFIC
// purpose must emit.
//
// TestNetworkEncoderCoversGeneratedFields cannot do this job. It unions the
// emitted keys of every purpose before comparing against the generated struct,
// so a field counts as covered when ANY ONE marshaller emits it. A field that
// marshalCorporate emits and marshalUserVPN drops is invisible to it -- which
// is precisely the shape of the defect it was written to catch.
//
// Mutation proof (run before this test existed): tagging marshalUserVPN's
// DHCPDDNS1 as `json:"-"`, so the key vanishes from remote-user-vpn payloads
// while still compiling, left the whole unifi package GREEN -- coverage guard
// ok, write-shape guard ok, `go test ./unifi/` ok. With this test present that
// mutation fails as:
//
//	purpose "remote-user-vpn": required key "dhcpd_dns_1" not emitted
//
// Scope is deliberately narrow. Each entry below names the evidence that the
// controller actually stores the field for that purpose. A marshaller may omit
// a field precisely BECAUSE the controller rejects it, so "some other purpose
// sends it" is not a reason to require it here.
func TestNetworkEncoderPerPurposeRequiredKeys(t *testing.T) {
	dnsSlots := []string{"dhcpd_dns_1", "dhcpd_dns_2", "dhcpd_dns_3", "dhcpd_dns_4"}

	cases := []struct {
		purpose  string
		required []string
		evidence string
	}{
		{
			purpose:  PurposeUserVPN,
			required: dnsSlots,
			// Measured 2026-08 against a 10.4.57 simulation controller: a
			// direct POST to /rest/networkconf with purpose remote-user-vpn
			// and four dhcpd_dns_N values stored and read back all four.
			// Before this was wired, marshalUserVPN declared only slots 1
			// and 2, so a client asking for four got two with no error at
			// any layer.
			evidence: "live controller stored and returned all four slots",
		},
		{
			purpose:  PurposeCorporate,
			required: dnsSlots,
			evidence: "already emitted; pinned so a silent removal fails here",
		},
		{
			purpose:  PurposeGuest,
			required: dnsSlots,
			evidence: "already emitted; pinned so a silent removal fails here",
		},
		{
			purpose:  PurposeVPNClient,
			required: dnsSlots,
			// Measured, and it is why this entry now exists: seeding all four
			// slots on a purpose=vpn-client networkconf and running
			// TestIntegrationNetworkRoundTrip returned
			//     LOST dhcpd_dns_3 (stored 9.9.9.9)
			//     LOST dhcpd_dns_4 (stored 149.112.112.112)
			// "the controller holds this value and the encoder emits no such
			// key". The controller stores all four; the encoder emitted two.
			// user-vpn passed the same run as a negative control, so the
			// verdict is not an artefact of seeding raw payloads.
			evidence: "live controller stored all four; the round trip reported LOST on 3 and 4",
		},
		// Still not asserted: vlan-only, wan and site-vpn. Whether the
		// controller persists DHCP DNS for those has not been measured, and
		// freezing an unmeasured claim is worse than leaving the gap visible.
		// vpn-client was in this list until it was measured.
	}

	for _, tc := range cases {
		t.Run(tc.purpose, func(t *testing.T) {
			emitted := networkEmittedKeys(t, tc.purpose)

			var missing []string
			for _, key := range tc.required {
				if !emitted[key] {
					missing = append(missing, key)
				}
			}
			sort.Strings(missing)

			for _, key := range missing {
				t.Errorf("purpose %q: required key %q not emitted", tc.purpose, key)
			}
			if len(missing) > 0 {
				t.Errorf("purpose %q drops %d of %d required key(s) from create/update "+
					"payloads with no error at any layer.\n"+
					"Evidence this purpose needs them: %s.\n"+
					"Add the field(s) to the matching struct in network_encode.go, or -- if "+
					"the controller genuinely rejects them for this purpose -- delete the "+
					"entry here and record the measurement that says so.",
					tc.purpose, len(missing), len(tc.required), tc.evidence)
			}
		})
	}
}

// TestPerPurposeRequiredKeysAreGenerated keeps the table above from drifting
// into fiction: every key it requires must exist on the generated Network
// struct. Without this, a typo'd key name would be a requirement no encoder
// could ever satisfy, and a renamed API field would leave a stale demand
// behind.
func TestPerPurposeRequiredKeysAreGenerated(t *testing.T) {
	generated := map[string]bool{}
	for _, name := range networkWireNames(t) {
		generated[name] = true
	}

	for _, key := range []string{"dhcpd_dns_1", "dhcpd_dns_2", "dhcpd_dns_3", "dhcpd_dns_4"} {
		if !generated[key] {
			t.Errorf("required key %q is not a field on the generated Network struct; "+
				"the per-purpose table in %s is stale",
				key, "network_encode_purpose_test.go")
		}
	}

	if len(generated) == 0 {
		t.Fatal(fmt.Sprint("networkWireNames returned no fields -- the helper is broken, ",
			"not the struct. Refusing to report an empty set as agreement."))
	}
}
