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
	resource string    // struct name, keying into overrides/fields.toml
	mode     string    // wire name of the auto|manual field, relative to container
	path     string    // endpoint, relative to /api/s/<site>/
	kind     probeKind // how to write it

	// container is the dotted wire path to the sub-object holding the mode,
	// empty when the mode sits on the resource itself. An array container is
	// compared element by element, so a probe payload carries exactly one.
	container string

	// build returns a complete payload for one arm. n varies the addressing
	// so the two arms do not collide.
	build func(name string, n int, mode string, deps probeDeps) map[string]any
}

type probeKind int

const (
	// probeCollection creates a throwaway object per arm and deletes it.
	probeCollection probeKind = iota

	// probeSetting writes a site-level singleton with PUT. There is only
	// one document, so the arms run over it in sequence and nothing is
	// cleaned up: each test gets its own disposable controller.
	probeSetting

	// probeV2Collection is a collection on the internal v2 API, which lives
	// under a different base, answers 201 on create, and returns the bare
	// object rather than a meta/data envelope.
	probeV2Collection
)

// endpoint builds the request path for one arm.
func (p preferenceProbe) endpoint(site string) string {
	if p.kind == probeV2Collection {
		return "/v2/api/site/" + site + "/" + p.path
	}
	return "/api/s/" + site + "/" + p.path
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

	{
		resource: "WLAN",
		mode:     "setting_preference",
		path:     "rest/wlanconf",
		build:    wlanPreferencePayload,
	},
	{
		resource: "WLAN",
		mode:     "minrate_setting_preference",
		path:     "rest/wlanconf",
		build:    wlanMinratePreferencePayload,
	},
	{
		resource: "PortProfile",
		mode:     "setting_preference",
		path:     "rest/portconf",
		build:    portProfilePreferencePayload,
	},
	{
		resource: "FirewallRule",
		mode:     "setting_preference",
		path:     "rest/firewallrule",
		build:    firewallRulePreferencePayload,
	},

	{
		resource: "Nat",
		mode:     "setting_preference",
		path:     "nat",
		kind:     probeV2Collection,
		build:    natPreferencePayload,
	},

	{
		resource: "SettingNtp",
		mode:     "setting_preference",
		path:     "set/setting/ntp",
		kind:     probeSetting,
		build:    ntpPreferencePayload,
	},
	{
		resource: "SettingUsg",
		mode:     "timeout_setting_preference",
		path:     "set/setting/usg",
		kind:     probeSetting,
		build:    usgTimeoutPreferencePayload,
	},
	{
		resource: "SettingSuperMgmt",
		mode:     "data_retention_setting_preference",
		path:     "set/setting/super_mgmt",
		kind:     probeSetting,
		build:    superMgmtPreferencePayload,
	},
	{
		resource:  "SettingUsg",
		mode:      "setting_preference",
		container: "dns_verification",
		path:      "set/setting/usg",
		kind:      probeSetting,
		build:     usgDNSVerificationPayload,
	},
	{
		resource: "SettingRadioAi",
		mode:     "setting_preference",
		path:     "set/setting/radio_ai",
		kind:     probeSetting,
		build:    radioAiPreferencePayload,
	},
	{
		resource: "SettingDashboard",
		mode:     "layout_preference",
		path:     "set/setting/dashboard",
		kind:     probeSetting,
		build:    dashboardPreferencePayload,
	},
}

// Two of the sixteen auto|manual fields in the generated client are not
// probed here, and neither is an oversight:
//
//	Device.setting_preference        nested in port_overrides
//	SettingUsg.setting_preference    nested in dns_verification
//
// Both sit inside a sub-object rather than on the resource itself, so a
// [Resource.preference.<wire>] key cannot name them and the generator's
// validation -- which resolves against top-level fields -- would reject the
// entry. Addressing nested modes needs a path syntax in the table, which is
// a schema change worth making deliberately rather than smuggling in with a
// measurement. Device additionally needs an adopted device to write against.

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

	deps := probeDeps{
		apGroupID:    firstAPGroupID(ctx, t, s, c.Site),
		wanNetworkID: ensureWANNetwork(ctx, t, s, c.Site),
	}

	for i, probe := range networkPreferenceProbes {
		t.Run(probe.resource+"."+probe.key(), func(t *testing.T) {
			manual := probe.measure(ctx, t, s, c.Site, 2*i, "manual", deps)
			auto := probe.measure(ctx, t, s, c.Site, 2*i+1, "auto", deps)

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
				if _, alsoAuto := auto[wire]; alsoAuto {
					t.Logf("refused under both modes, not this mode's doing: %s (%s)", wire, detail)
					continue
				}
				// Refused under manual but kept under auto is the mode
				// running backwards, which no probe here expects. Worth
				// saying out loud rather than filing under "both".
				t.Logf("refused under MANUAL only, which is the mode inverted: %s (%s)", wire, detail)
			}

			entry, ok := recorded[probe.resource][probe.key()]
			if !ok {
				t.Errorf("no ownership recorded for %s.%s. Measured %d field(s) on %s; add to "+
					"overrides/fields.toml:\n\n%s\nRun the other harness before trusting this as the "+
					"whole answer: UniFi OS pins fields the standalone controller leaves to manual "+
					"mode, and anything it pins belongs in uos_excludes.",
					probe.resource, probe.key(), len(owned), harnessName(),
					preferenceTOML(probe, owned, controllerVersion(ctx, t, s)))
				return
			}

			// Ownership is a property of the product, not just the Network
			// build: UniFi OS pins some fields the standalone controller
			// leaves to manual mode, and reports the same version while
			// doing it. Compare against the answer for the harness actually
			// under test.
			want := entry.OwnsOn(onUOSHarness())
			if diff := comparePreference(want, owned, manual); diff != "" {
				t.Errorf("%s.%s ownership no longer matches overrides/fields.toml (harness: %s):\n%s\n\n"+
					"The controller's behaviour moved or the table was wrong. Re-measure before editing it.",
					probe.resource, probe.key(), harnessName(), diff)
			}
		})
	}
}

// measure writes one arm and returns what the controller refused to store.
func (p preferenceProbe) measure(ctx context.Context, t *testing.T, s *controllertest.Session, site string, n int, mode string, deps probeDeps) map[string]string {
	t.Helper()

	name := fmt.Sprintf("pref-%d-%s", n, mode)
	payload := p.build(name, n, mode, deps)

	var (
		body   any
		status int
		err    error
	)
	path := p.endpoint(site)
	switch p.kind {
	case probeSetting:
		body, status, err = s.PutJSON(ctx, path, payload)
	default:
		body, status, err = s.PostJSON(ctx, path, payload)
	}
	if err != nil {
		// The status is the useful half of a non-JSON response: it separates
		// "this endpoint is not here" from "this endpoint is angry".
		t.Fatalf("transport to %s (HTTP %d): %v", path, status, err)
	}
	// v2 answers 201 on create, v1 answers 200 throughout.
	if status != 200 && status != 201 {
		t.Fatalf("%s arm rejected (HTTP %d): %v\n\nThe payload must be valid under both modes or the "+
			"comparison measures the rejection, not the mode.", mode, status, body)
	}
	stored := firstData(t, body)
	if p.kind != probeSetting {
		id, _ := stored["_id"].(string)
		if id == "" {
			id, _ = stored["id"].(string) // true-v2 objects
		}
		if id != "" {
			defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
	}

	askedScope, storedScope := payload, stored
	if p.container != "" {
		askedScope = descend(t, payload, p.container, "the payload")
		storedScope = descend(t, stored, p.container, "the stored object")
		if askedScope == nil || storedScope == nil {
			t.Fatalf("%s did not survive the write, so nothing inside it can be measured", p.container)
		}
	}

	if got, _ := storedScope[p.mode].(string); got != mode {
		t.Fatalf("asked for %s = %q, controller stored %q. The arm did not run under the mode "+
			"it was supposed to, so anything measured from it is meaningless.", p.mode, mode, got)
	}

	return discardedFields(askedScope, storedScope)
}

// descend follows a dotted wire path into an object, taking the first element
// of an array on the way: a probe payload carries one element, and comparing
// by position is only meaningful because of that.
func descend(t *testing.T, obj map[string]any, path, label string) map[string]any {
	t.Helper()

	current := any(obj)
	for segment := range strings.SplitSeq(path, ".") {
		asMap, ok := current.(map[string]any)
		if !ok {
			t.Logf("%s: %q is not an object in %s", path, segment, label)
			return nil
		}
		next, ok := asMap[segment]
		if !ok {
			t.Logf("%s: %s has no %q", path, label, segment)
			return nil
		}
		if list, isList := next.([]any); isList {
			if len(list) == 0 {
				t.Logf("%s: %q is empty in %s", path, segment, label)
				return nil
			}
			next = list[0]
		}
		current = next
	}

	out, ok := current.(map[string]any)
	if !ok {
		t.Logf("%s does not resolve to an object in %s", path, label)
		return nil
	}
	return out
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
// refusedUnderManual is what the manual arm did not store, so a field missing
// from the measured set can be reported for the right reason. "Not owned by
// the mode" has two causes that want opposite responses: the controller stored
// what was asked, or it refused the value whatever the mode said. Reporting
// the second as the first sends the reader looking for a mode change that did
// not happen -- and the run log directly above the failure contradicts it.
func comparePreference(recorded, measured []string, refusedUnderManual map[string]string) string {
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
		if has(measured, wire) {
			continue
		}
		if detail, refused := refusedUnderManual[wire]; refused {
			fmt.Fprintf(&b, "  - %s (the table says the mode owns this, but the controller refused it "+
				"under BOTH modes: %s. Not the mode's doing -- some other rule reaches this field "+
				"now, so it belongs in uos_excludes or out of owns)\n", wire, detail)
			continue
		}
		fmt.Fprintf(&b, "  - %s (the table says the controller owns this; it stored what was asked)\n", wire)
	}
	return b.String()
}

// onUOSHarness reports whether this run targets the UniFi OS harness, which
// answers differently from the standalone controller for some fields.
func onUOSHarness() bool {
	return strings.EqualFold(os.Getenv("UNIFI_TEST_HARNESS"), "uos")
}

// harnessName labels a failure with the product it was measured against, so
// a table mismatch does not read as a controller regression when it is a
// difference between the two harnesses.
func harnessName() string {
	if onUOSHarness() {
		return "UniFi OS"
	}
	return "standalone UniFi Network"
}

// key is the preference key this probe measures: the mode's wire name, or a
// dotted path when the mode sits inside a sub-object.
func (p preferenceProbe) key() string {
	if p.container == "" {
		return p.mode
	}
	return p.container + "." + p.mode
}

// quoteKeyIfNested quotes a dotted key. An unquoted one is read by TOML as
// nested tables and decodes into an entry that owns nothing.
func quoteKeyIfNested(key string) string {
	if strings.Contains(key, ".") {
		return fmt.Sprintf("%q", key)
	}
	return key
}

// preferenceTOML renders a measured set as the overrides/fields.toml entry to
// paste, so recording a result is copying rather than transcribing.
func preferenceTOML(p preferenceProbe, owned []string, version string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s.preference.%s]\n", p.resource, quoteKeyIfNested(p.key()))
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
func corporatePreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
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
func corporateIPV6PreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
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
func wanDNSPreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
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
func wanIPV6DNSPreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
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

// wlanPreferencePayload exercises the WLAN advanced block.
func wlanPreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
	p := wlanBase(name, deps)
	p["setting_preference"] = mode
	p["bss_transition"] = false
	p["uapsd_enabled"] = true
	p["fast_roaming_enabled"] = true
	p["dtim_mode"] = "custom"
	p["dtim_ng"] = 3
	p["dtim_na"] = 3
	p["group_rekey"] = 7200
	p["mcastenhance_enabled"] = true
	p["proxy_arp"] = true
	p["l2_isolation"] = true
	p["bc_filter_enabled"] = true
	return p
}

// wlanMinratePreferencePayload exercises the minimum-data-rate block, which
// has its own mode field independent of the WLAN's general one.
func wlanMinratePreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
	p := wlanBase(name, deps)
	p["minrate_setting_preference"] = mode
	p["minrate_ng_enabled"] = true
	p["minrate_ng_data_rate_kbps"] = 5500
	p["minrate_na_enabled"] = true
	p["minrate_na_data_rate_kbps"] = 12000
	p["minrate_ng_advertising_rates"] = true
	p["minrate_na_advertising_rates"] = true
	return p
}

// portProfilePreferencePayload exercises the switch-port advanced block.
func portProfilePreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
	return map[string]any{
		"name": name, "forward": "all", "op_mode": "switch",
		"setting_preference": mode,

		"autoneg":                        false,
		"speed":                          1000,
		"full_duplex":                    true,
		"isolation":                      true,
		"lldpmed_enabled":                false,
		"lldpmed_notify_enabled":         true,
		"stp_port_mode":                  false,
		"egress_rate_limit_kbps_enabled": true,
		"egress_rate_limit_kbps":         64000,
		"stormctrl_type":                 "level",
		"stormctrl_bcast_enabled":        true,
		"stormctrl_bcast_level":          50,
		"stormctrl_mcast_enabled":        true,
		"stormctrl_mcast_level":          50,
		"stormctrl_ucast_enabled":        true,
		"stormctrl_ucast_level":          50,
		"port_keepalive_enabled":         true,
		"eee_enabled":                    true,
		"flow_control_enabled":           true,
	}
}

// firewallRulePreferencePayload exercises the legacy firewall rule block.
func firewallRulePreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
	return map[string]any{
		"name": name, "enabled": true,
		"ruleset": "LAN_IN", "rule_index": 2000 + n,
		"action": "accept", "protocol": "all",
		"setting_preference": mode,

		"logging":                 true,
		"src_address":             "10.94.0.0/24",
		"dst_address":             "10.94.1.0/24",
		"protocol_match_excepted": false,
		"ipsec":                   "",
		"state_established":       true,
		"state_related":           true,
	}
}

// natPreferencePayload exercises the v2 NAT rule block. The filter objects
// and out_interface are not optional: without them the controller answers a
// non-JSON HTTP 500 instead of a rejection.
func natPreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
	return map[string]any{
		"description": name, "enabled": true,
		"type": "MASQUERADE", "ip_version": "IPV4",
		"setting_preference": mode,

		"protocol":                 "all",
		"rule_index":               fmt.Sprintf("%d", 3000+n),
		"logging":                  true,
		"exclude":                  false,
		"is_predefined":            false,
		"pppoe_use_base_interface": false,
		"out_interface":            deps.wanNetworkID,
		"source_filter":            map[string]any{"filter_type": "NONE", "firewall_group_ids": []string{}},
		"destination_filter":       map[string]any{"filter_type": "NONE", "firewall_group_ids": []string{}},
	}
}

// ntpPreferencePayload exercises the site NTP servers.
func ntpPreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
	return map[string]any{
		"key":                "ntp",
		"setting_preference": mode,
		"ntp_server_1":       "10.95.0.1",
		"ntp_server_2":       "10.95.0.2",
		"ntp_server_3":       "10.95.0.3",
		"ntp_server_4":       "10.95.0.4",
	}
}

// usgTimeoutPreferencePayload exercises the connection-tracking timeouts,
// which have their own mode field separate from the gateway setting's.
func usgTimeoutPreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
	return map[string]any{
		"key":                        "usg",
		"timeout_setting_preference": mode,
		"tcp_established_timeout":    7200,
		"tcp_close_timeout":          20,
		"tcp_close_wait_timeout":     40,
		"tcp_fin_wait_timeout":       60,
		"tcp_last_ack_timeout":       20,
		"tcp_syn_recv_timeout":       30,
		"tcp_syn_sent_timeout":       60,
		"tcp_time_wait_timeout":      60,
		"udp_other_timeout":          60,
		"udp_stream_timeout":         120,
		"icmp_timeout":               60,
		"other_timeout":              600,
	}
}

// usgDNSVerificationPayload exercises the mode nested in the gateway's DNS
// verification block -- one of the two modes that do not sit on their
// resource.
func usgDNSVerificationPayload(name string, n int, mode string, deps probeDeps) map[string]any {
	return map[string]any{
		"key": "usg",
		"dns_verification": map[string]any{
			"setting_preference":   mode,
			"domain":               "verify.example",
			"primary_dns_server":   "10.98.0.53",
			"secondary_dns_server": "10.98.0.54",
		},
	}
}

// superMgmtPreferencePayload exercises the data-retention windows.
func superMgmtPreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
	return map[string]any{
		"key":                               "super_mgmt",
		"data_retention_setting_preference": mode,
		"data_retention_time_in_hours_for_5minutes_scale": 48,
		"data_retention_time_in_hours_for_hourly_scale":   720,
		"data_retention_time_in_hours_for_daily_scale":    2160,
		"data_retention_time_in_hours_for_monthly_scale":  8760,
		"data_retention_time_in_hours_for_others":         720,
	}
}

// radioAiPreferencePayload exercises the AI radio optimisation block.
func radioAiPreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
	return map[string]any{
		"key":                "radio_ai",
		"setting_preference": mode,
		"enabled":            true,
		"cron_expr":          "0 2 * * 0",
		"optimize":           []string{"channel", "power"},
		"radios":             []string{"na", "ng"},
		"channels_ng":        []int{1, 6, 11},
		"channels_na":        []int{36, 40, 44},
		"ht_modes_ng":        []int{20},
		"ht_modes_na":        []int{40},
		"exclude_devices":    []string{},
	}
}

// dashboardPreferencePayload exercises layout_preference. It carries the
// auto|manual enum like the rest, and whether it behaves like one is the
// question -- a UI layout choice would own nothing.
func dashboardPreferencePayload(name string, n int, mode string, deps probeDeps) map[string]any {
	return map[string]any{
		"key":               "dashboard",
		"layout_preference": mode,
		"widgets": []map[string]any{
			{"name": "wifi_channels", "enabled": true},
			{"name": "wan_activity", "enabled": false},
		},
	}
}
