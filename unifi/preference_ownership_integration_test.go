//go:build integration

// unifi/preference_ownership_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

// preferenceProbe is one auto|manual mode field and a payload rich enough to
// show what it takes over.
//
// The payload is written twice, once under each mode, and each write is
// compared against what it asked for rather than against the other write.
// That matters: the two arms cannot share a subnet or a VLAN id, so half the
// values legitimately differ between them, and diffing the two stored objects
// directly would report the addressing as owned. Comparing each arm to its
// own request removes the addressing and leaves the mode.
type preferenceProbe struct {
	resource string // struct name, keying into overrides/fields.toml
	mode     string // wire name of the auto|manual field
	path     string // REST collection, relative to /api/s/<site>/

	// build returns a complete payload for one arm. n varies the addressing
	// so the two arms do not collide.
	build func(name string, n int, mode string) map[string]any
}

// networkPreferenceProbes covers the four auto|manual fields on a network.
//
// Ownership is measured, never assumed. The one prior record of it -- the
// commit that stopped the encoder sending "auto" -- worked from the encoder's
// *_enabled toggles and so counted six fields; the same mode also replaces
// dhcpd_leasetime and blanks domain_name, which are not toggles and were
// never in view. A sweep that starts from a payload rather than from a field
// list does not have that blind spot.
var networkPreferenceProbes = []preferenceProbe{
	{
		resource: "Network",
		mode:     "setting_preference",
		path:     "rest/networkconf",
		build:    corporatePreferencePayload,
	},
	{
		resource: "Network",
		mode:     "ipv6_setting_preference",
		path:     "rest/networkconf",
		build:    corporateIPV6PreferencePayload,
	},
	{
		resource: "Network",
		mode:     "wan_dns_preference",
		path:     "rest/networkconf",
		build:    wanDNSPreferencePayload,
	},
	{
		resource: "Network",
		mode:     "wan_ipv6_dns_preference",
		path:     "rest/networkconf",
		build:    wanIPV6DNSPreferencePayload,
	},
}

// TestIntegrationPreferenceOwnership measures what each auto|manual mode
// field takes over, and checks the answer against overrides/fields.toml.
//
// A mode field with no entry there is reported with the TOML to paste, so
// adding a resource is: write a payload, run this, paste the result. A mode
// field whose entry disagrees with the controller fails -- either the
// controller changed or the table was wrong, and both are worth stopping for.
func TestIntegrationPreferenceOwnership(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	recorded, err := fields.LoadPreferences()
	if err != nil {
		t.Fatalf("load overrides/fields.toml: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	for i, probe := range networkPreferenceProbes {
		t.Run(probe.resource+"."+probe.mode, func(t *testing.T) {
			manual := probe.measure(ctx, t, s, c.Site, 2*i, "manual")
			auto := probe.measure(ctx, t, s, c.Site, 2*i+1, "auto")

			// What auto refused and manual kept. Anything both arms refuse
			// is some other rule -- a missing prerequisite, a purpose
			// restriction -- and is not this mode's doing.
			owned := []string{}
			for wire, detail := range auto {
				if _, alsoManual := manual[wire]; alsoManual {
					continue
				}
				if wire == probe.mode {
					continue // the mode field itself, not a field it owns
				}
				owned = append(owned, wire)
				t.Logf("OWNED %s (%s)", wire, detail)
			}
			sort.Strings(owned)

			for wire, detail := range manual {
				if wire == probe.mode {
					continue
				}
				t.Logf("refused under both modes, not this mode's doing: %s (%s)", wire, detail)
			}

			entry, ok := recorded[probe.resource][probe.mode]
			if !ok {
				t.Errorf("no ownership recorded for %s.%s. Measured %d field(s); add to overrides/fields.toml:\n\n%s",
					probe.resource, probe.mode, len(owned), preferenceTOML(probe, owned, controllerVersion(ctx, t, s)))
				return
			}

			if diff := comparePreference(entry.Owns, owned); diff != "" {
				t.Errorf("%s.%s ownership no longer matches overrides/fields.toml:\n%s\n\n"+
					"The controller's behaviour moved or the table was wrong. Re-measure before editing it.",
					probe.resource, probe.mode, diff)
			}
		})
	}
}

// measure writes one arm and returns what the controller refused to store.
func (p preferenceProbe) measure(ctx context.Context, t *testing.T, s *controllertest.Session, site string, n int, mode string) map[string]string {
	t.Helper()

	name := fmt.Sprintf("pref-%d-%s", n, mode)
	payload := p.build(name, n, mode)

	body, status, err := s.PostJSON(ctx, "/api/s/"+site+"/"+p.path, payload)
	if err != nil {
		t.Fatalf("transport: %v", err)
	}
	if status != 200 {
		t.Fatalf("%s arm rejected (HTTP %d): %v\n\nThe payload must be valid under both modes or the "+
			"comparison measures the rejection, not the mode.", mode, status, body)
	}
	stored := firstData(t, body)
	if id, _ := stored["_id"].(string); id != "" {
		defer s.DeleteJSON(ctx, "/api/s/"+site+"/"+p.path+"/"+id) //nolint:errcheck
	}

	if got, _ := stored[p.mode].(string); got != mode {
		t.Fatalf("asked for %s = %q, controller stored %q. The arm did not run under the mode "+
			"it was supposed to, so anything measured from it is meaningless.", p.mode, mode, got)
	}

	return discardedFields(payload, stored)
}

// controllerVersion reads the running controller's build off /status, so a
// recorded measurement names the build it came from rather than whatever the
// schema cache happens to hold.
func controllerVersion(ctx context.Context, t *testing.T, s *controllertest.Session) string {
	t.Helper()

	body, status, err := s.GetJSON(ctx, "/status")
	if err != nil || status != 200 {
		t.Logf("unable to read /status (HTTP %d, %v); recording an unknown build", status, err)
		return "unknown"
	}
	envelope, ok := body.(map[string]any)
	if !ok {
		return "unknown"
	}
	meta, ok := envelope["meta"].(map[string]any)
	if !ok {
		return "unknown"
	}
	version, _ := meta["server_version"].(string)
	if version == "" {
		return "unknown"
	}
	return version
}

// comparePreference renders the difference between the recorded and measured
// ownership sets, or "" when they agree.
func comparePreference(recorded, measured []string) string {
	has := func(list []string, wire string) bool {
		for _, w := range list {
			if w == wire {
				return true
			}
		}
		return false
	}

	var b strings.Builder
	for _, wire := range measured {
		if !has(recorded, wire) {
			fmt.Fprintf(&b, "  + %s (the controller owns this now; the table does not say so)\n", wire)
		}
	}
	sorted := append([]string(nil), recorded...)
	sort.Strings(sorted)
	for _, wire := range sorted {
		if !has(measured, wire) {
			fmt.Fprintf(&b, "  - %s (the table says the controller owns this; it stored what was asked)\n", wire)
		}
	}
	return b.String()
}

// preferenceTOML renders a measured set as the overrides/fields.toml entry to
// paste, so recording a result is copying rather than transcribing.
func preferenceTOML(p preferenceProbe, owned []string, version string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s.preference.%s]\n", p.resource, p.mode)
	if len(owned) == 0 {
		b.WriteString("owns = []\n")
	} else {
		b.WriteString("owns = [\n")
		for _, wire := range owned {
			fmt.Fprintf(&b, "  %q,\n", wire)
		}
		b.WriteString("]\n")
	}
	fmt.Fprintf(&b, "measured = %q\n", version)
	return b.String()
}

// corporatePreferencePayload is the corporate advanced block, wider than the
// round-trip seed: every field reachable without a gateway that a mode might
// plausibly own, because a field the payload does not set cannot be observed
// being taken away.
func corporatePreferencePayload(name string, n int, mode string) map[string]any {
	net := fmt.Sprintf("10.92.%d", n)
	return map[string]any{
		"name": name, "purpose": PurposeCorporate, "enabled": true,
		"ip_subnet": net + ".1/24", "vlan_enabled": true, "vlan": 900 + n,
		"setting_preference": mode, "networkgroup": "LAN",

		"domain_name":               "pref.example",
		"igmp_snooping":             true,
		"igmp_fastleave":            true,
		"mdns_enabled":              true,
		"upnp_lan_enabled":          true,
		"network_isolation_enabled": true,
		"auto_scale_enabled":        false,

		"dhcpguard_enabled": true,
		"dhcpd_ip_1":        net + ".2",
		"dhcpd_mac_1":       "00:11:22:33:44:55",

		"dhcpd_enabled":             true,
		"dhcpd_start":               net + ".6",
		"dhcpd_stop":                net + ".254",
		"dhcpd_leasetime":           7200,
		"dhcpd_conflict_checking":   true,
		"dhcpd_dns_enabled":         true,
		"dhcpd_dns_1":               net + ".53",
		"dhcpd_gateway_enabled":     true,
		"dhcpd_gateway":             net + ".1",
		"dhcpd_ntp_enabled":         true,
		"dhcpd_ntp_1":               net + ".123",
		"dhcpd_time_offset_enabled": true,
		"dhcpd_time_offset":         3600,
		"dhcpd_wins_enabled":        true,
		"dhcpd_wins_1":              net + ".44",
		"dhcpd_tftp_server":         net + ".69",
		"dhcpd_unifi_controller":    net + ".8",
	}
}

// corporateIPV6PreferencePayload exercises ipv6_setting_preference. The
// interface type stays "none" deliberately: "static" needs a deployable
// prefix and this harness has no gateway, so the controller answers
// api.err.NotDeployableIPv6Subnet and there is no arm to compare.
func corporateIPV6PreferencePayload(name string, n int, mode string) map[string]any {
	net := fmt.Sprintf("10.92.%d", n)
	return map[string]any{
		"name": name, "purpose": PurposeCorporate, "enabled": true,
		"ip_subnet": net + ".1/24", "vlan_enabled": true, "vlan": 900 + n,
		"networkgroup": "LAN",

		"ipv6_setting_preference":        mode,
		"ipv6_interface_type":            "none",
		"ipv6_ra_enabled":                true,
		"ipv6_ra_priority":               "high",
		"ipv6_ra_valid_lifetime":         7200,
		"ipv6_ra_preferred_lifetime":     3600,
		"ipv6_client_address_assignment": "slaac",
		"dhcpdv6_dns_auto":               false,
		"dhcpdv6_dns_1":                  "2001:db8::53",
		"dhcpdv6_leasetime":              7200,
	}
}

// wanDNSPreferencePayload exercises wan_dns_preference. The same trap as
// setting_preference on the WAN path: a caller sets wan_dns1 while the mode
// says auto, and the controller keeps its own resolvers.
func wanDNSPreferencePayload(name string, n int, mode string) map[string]any {
	return map[string]any{
		"name": name, "purpose": PurposeWAN, "enabled": true,
		"wan_networkgroup": wanGroupFor(n), "wan_type": "dhcp", "wan_type_v6": "disabled",

		"wan_dns_preference":    mode,
		"wan_dns1":              "10.93.0.53",
		"wan_dns2":              "10.93.0.54",
		"wan_dns3":              "10.93.0.55",
		"wan_dns4":              "10.93.0.56",
		"wan_load_balance_type": "failover-only",
		"wan_failover_priority": 2,
		"report_wan_event":      true,
	}
}

// wanIPV6DNSPreferencePayload exercises wan_ipv6_dns_preference.
func wanIPV6DNSPreferencePayload(name string, n int, mode string) map[string]any {
	return map[string]any{
		"name": name, "purpose": PurposeWAN, "enabled": true,
		"wan_networkgroup": wanGroupFor(n), "wan_type": "dhcp", "wan_type_v6": "dhcpv6",

		"wan_ipv6_dns_preference": mode,
		"wan_ipv6_dns1":           "2001:db8::53",
		"wan_ipv6_dns2":           "2001:db8::54",
		"wan_load_balance_type":   "failover-only",
		"wan_failover_priority":   2,
	}
}

// wanGroupFor spreads WAN arms across the controller's WAN slots: two
// networks cannot share one, and both arms of a probe exist at once.
func wanGroupFor(n int) string {
	return fmt.Sprintf("WAN%d", 2+n)
}
