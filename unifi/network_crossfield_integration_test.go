//go:build integration

// unifi/network_crossfield_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// crossFieldCase is one "*_enabled flag turned on without the value it
// switches on" payload, and what the controller was measured to do with it.
type crossFieldCase struct {
	flag    string         // the _enabled wire name under test
	purpose string         // purpose to create it on
	partner string         // the field the flag switches on, deliberately omitted
	extra   map[string]any // anything else the payload needs to be valid
	want    string         // measured outcome: an api.err.* code, "accepted", or "disabled"
}

// networkCrossFieldRules records what the controller does when a *_enabled
// flag is set without the field it enables. The encoder performs no
// validation, so these are the errors a caller sees -- and several name a
// sibling field rather than the flag, which makes them hard to act on. Two of
// them were found by accident while fixing unrelated bugs.
//
//	<api.err.*>  the create is rejected with that code
//	accepted     the controller stores the flag on regardless
//	disabled     the create succeeds but the flag comes back off
//
// "disabled" is the dangerous outcome: rc: ok and a lie, so the caller finds
// out from the next read or not at all. There is a second, larger source of
// it that has nothing to do with missing partners -- a field can also come
// back off because an auto|manual mode field owns it, and the controller
// stored its own value. That is measured separately, per mode and per owned
// field, by TestIntegrationPreferenceOwnership, with the answers recorded in
// overrides/fields.toml. Rows are not duplicated here: a payload-driven
// differential sees fields a flag-by-flag sweep structurally cannot (it found
// twelve owned fields for setting_preference, four of them not toggles at
// all), so a partial copy in this table would only go stale.
var networkCrossFieldRules = []crossFieldCase{
	// Rejected, and the error names something other than the field you
	// actually forgot. These are the expensive ones to debug.
	{flag: "dhcpd_boot_enabled", purpose: PurposeCorporate, partner: "dhcpd_boot_server", want: "api.err.BootFilenameInvalid"},
	{flag: "dhcpd_gateway_enabled", purpose: PurposeCorporate, partner: "dhcpd_gateway", want: "api.err.GatewayIpNotInSubnet"},
	{flag: "vlan_enabled", purpose: PurposeVLANOnly, partner: "vlan", want: "api.err.VlanUsed"},

	// Rejected, error names the right thing.
	{flag: "dhcpguard_enabled", purpose: PurposeCorporate, partner: "dhcpd_ip_1", want: "api.err.MissingIPAddress"},
	{flag: "dhcpd_ntp_enabled", purpose: PurposeCorporate, partner: "dhcpd_ntp_1", want: "api.err.NtpAddressInvalid"},
	{flag: "wan_pppoe_username_enabled", purpose: PurposeWAN, partner: "wan_username", extra: map[string]any{"wan_type": "pppoe"}, want: "api.err.InvalidWanPppoeCredentials"},

	// Not a missing-partner rule at all: dhcp_relay_enabled and
	// dhcpd_enabled are mutually exclusive, and the base payload here runs a
	// DHCP server. The error is about the conflict, not the empty server list.
	{flag: "dhcp_relay_enabled", purpose: PurposeCorporate, partner: "dhcp_relay_servers", want: "api.err.DhcpServerAndRelayCannotCoexist"},

	// Not a pairing rule: network_isolation_enabled is corporate-only.
	{flag: "network_isolation_enabled", purpose: PurposeVLANOnly, partner: "", want: "api.err.NetworkIsolationAppliedOnNonCorporateNetwork"},

	// Accepted, then silently switched back off. No error, so a caller
	// believes DHCPv6 is on until something downstream disagrees.
	{flag: "dhcpdv6_enabled", purpose: PurposeCorporate, partner: "dhcpdv6_start", want: "disabled"},

	// No pairing rule: the flag stores on with the partner absent. Whether
	// that is useful is the controller's business, but it will not reject it.
	{flag: "dhcpd_dns_enabled", purpose: PurposeCorporate, partner: "dhcpd_dns_1", want: "accepted"},
	{flag: "dhcpd_wins_enabled", purpose: PurposeCorporate, partner: "dhcpd_wins_1", want: "accepted"},
	{flag: "dhcpd_time_offset_enabled", purpose: PurposeCorporate, partner: "dhcpd_time_offset", want: "accepted"},
	{flag: "mac_override_enabled", purpose: PurposeCorporate, partner: "mac_override", want: "accepted"},
	{flag: "ipv6_ra_enabled", purpose: PurposeCorporate, partner: "ipv6_subnet", want: "accepted"},
	{flag: "auto_scale_enabled", purpose: PurposeCorporate, partner: "", want: "accepted"},
	{flag: "wan_vlan_enabled", purpose: PurposeWAN, partner: "wan_vlan", extra: map[string]any{"wan_type": "dhcp"}, want: "accepted"},
	{flag: "interface_mtu_enabled", purpose: PurposeWAN, partner: "interface_mtu", extra: map[string]any{"wan_type": "dhcp"}, want: "accepted"},
	{flag: "wan_smartq_enabled", purpose: PurposeWAN, partner: "wan_smartq_up_rate", extra: map[string]any{"wan_type": "dhcp"}, want: "accepted"},
}

// TestIntegrationNetworkCrossFieldRules pins the measured behaviour of every
// *_enabled flag set without the value it enables. The SDK does no validation
// of these pairings, so this is where they are written down; a change in the
// table means the controller changed, which is worth knowing.
func TestIntegrationNetworkCrossFieldRules(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	for i, tc := range networkCrossFieldRules {
		t.Run(tc.flag, func(t *testing.T) {
			n := 700 + i
			payload := map[string]any{
				"name":    fmt.Sprintf("xf-%d", n),
				"purpose": tc.purpose,
				"enabled": true,
			}
			switch tc.purpose {
			case PurposeCorporate, PurposeGuest:
				payload["ip_subnet"] = fmt.Sprintf("10.90.%d.1/24", n%250)
				payload["vlan_enabled"] = true
				payload["vlan"] = n
				payload["dhcpd_enabled"] = true
			case PurposeVLANOnly:
				payload["networkgroup"] = "LAN"
				payload["vlan_enabled"] = true
				payload["vlan"] = n
			case PurposeWAN:
				payload["wan_networkgroup"] = "WAN2"
			}
			for k, v := range tc.extra {
				payload[k] = v
			}
			payload[tc.flag] = true
			delete(payload, tc.partner)

			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", payload)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}

			got := "accepted"
			if status != 200 {
				got = errCode(body)
			} else {
				created := firstData(t, body)
				if id, _ := created["_id"].(string); id != "" {
					defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck
				}
				if created[tc.flag] != true {
					got = "disabled"
				}
			}

			if tc.want == "" {
				t.Logf("UNPINNED %s (without %s) -> %s", tc.flag, tc.partner, got)
				return
			}
			if got != tc.want {
				t.Errorf("%s without %s: got %q, want %q", tc.flag, tc.partner, got, tc.want)
			}
		})
	}
}

// errCode pulls meta.msg out of a controller error envelope.
func errCode(body any) string {
	m, ok := body.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", body)
	}
	meta, ok := m["meta"].(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", body)
	}
	msg, _ := meta["msg"].(string)
	if msg == "" {
		return fmt.Sprintf("%v", body)
	}
	return msg
}
