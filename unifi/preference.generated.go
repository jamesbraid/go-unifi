// Generated code. DO NOT EDIT.

package unifi

import "slices"

// Preference is one auto|manual mode field and the fields it governs.
type Preference struct {
	// Container is the dotted wire path to the sub-object holding the mode,
	// empty when the mode sits on the resource itself. An array container
	// holds one mode per element, each governing that element.
	Container string

	// Mode is the mode field's wire name, relative to Container.
	Mode string

	// Owns lists the wire names the controller takes over while Mode is
	// "auto", relative to Container -- a mode governs its own object. An
	// empty list is a measured result, not a gap: that mode was probed and
	// owns nothing.
	//
	// This is the standalone UniFi Network answer. Inside UniFi OS, subtract
	// UOSExcludes -- or call OwnsOn, which does it for you.
	Owns []string

	// UOSExcludes lists entries of Owns that do NOT hold inside UniFi OS,
	// because the console owns the field outright and neither mode reaches
	// it. Always a subset of Owns.
	//
	// The Network version does not separate the two products: UniFi OS
	// bundles the same build and reports it, while pinning some fields the
	// standalone controller leaves to manual mode. A consumer that assumes
	// Owns holds everywhere will describe UniFi OS wrongly, so the
	// difference is published rather than folded away.
	UOSExcludes []string
}

// OwnsOn returns the wire names this mode owns on one product.
//
// uos selects the UniFi OS answer, which is Owns minus the fields the console
// pins. Everything else gets Owns unchanged.
func (p Preference) OwnsOn(uos bool) []string {
	if !uos || len(p.UOSExcludes) == 0 {
		return p.Owns
	}
	out := make([]string, 0, len(p.Owns))
	for _, wire := range p.Owns {
		if !slices.Contains(p.UOSExcludes, wire) {
			out = append(out, wire)
		}
	}
	return out
}

// PreferenceOwnedFields records what each auto|manual mode field owns.
//
// A UniFi resource can carry a mode field -- setting_preference and its
// siblings -- that decides whether a block of its own fields is the caller's
// to set or the controller's. While the mode is "auto" the controller stores
// its own values over whatever the payload asked for, answers rc: ok, and
// reports nothing, so a caller learns from the next read or not at all.
//
// The key is the resource's schema name; settings keep their "Setting"
// prefix, so the site NTP document is "SettingNtp".
//
// Measured against a live controller by TestIntegrationPreferenceOwnership
// and recorded in overrides/fields.toml; the build of each set is in the
// "measured" key beside it there.
var PreferenceOwnedFields = map[string][]Preference{
	"Device": {
		{Container: "port_overrides", Mode: "setting_preference", Owns: []string{}},
	},
	"FirewallRule": {
		{Mode: "setting_preference", Owns: []string{
			"state_established",
			"state_related",
		}},
	},
	"Nat": {
		{Mode: "setting_preference", Owns: []string{}},
	},
	"Network": {
		{Mode: "ipv6_setting_preference", Owns: []string{
			"dhcpdv6_dns_auto",
			"dhcpdv6_leasetime",
			"ipv6_ra_preferred_lifetime",
		}},
		{Mode: "setting_preference", Owns: []string{
			"dhcpd_dns_enabled",
			"dhcpd_gateway_enabled",
			"dhcpd_leasetime",
			"dhcpd_ntp_enabled",
			"dhcpd_tftp_server",
			"dhcpd_time_offset_enabled",
			"dhcpd_unifi_controller",
			"dhcpd_wins_enabled",
			"domain_name",
			"igmp_snooping",
			"upnp_lan_enabled",
		}},
		{Mode: "wan_dns_preference", Owns: []string{
			"wan_dns1",
			"wan_dns2",
		}},
		{Mode: "wan_ipv6_dns_preference", Owns: []string{}},
	},
	"PortProfile": {
		{Mode: "setting_preference", Owns: []string{
			"egress_rate_limit_kbps_enabled",
			"isolation",
			"lldpmed_enabled",
			"lldpmed_notify_enabled",
			"port_keepalive_enabled",
			"stormctrl_bcast_enabled",
			"stormctrl_mcast_enabled",
			"stormctrl_ucast_enabled",
			"stp_port_mode",
		}},
	},
	"SettingDashboard": {
		{Mode: "layout_preference", Owns: []string{
			"widgets",
		}},
	},
	"SettingNtp": {
		{Mode: "setting_preference", Owns: []string{
			"ntp_server_1",
			"ntp_server_2",
			"ntp_server_3",
			"ntp_server_4",
		}},
	},
	"SettingRadioAi": {
		{Mode: "setting_preference", Owns: []string{}},
	},
	"SettingSuperMgmt": {
		{Mode: "data_retention_setting_preference", Owns: []string{
			"data_retention_time_in_hours_for_5minutes_scale",
			"data_retention_time_in_hours_for_hourly_scale",
			"data_retention_time_in_hours_for_others",
		}, UOSExcludes: []string{
			"data_retention_time_in_hours_for_5minutes_scale",
			"data_retention_time_in_hours_for_hourly_scale",
		}},
	},
	"SettingUsg": {
		{Container: "dns_verification", Mode: "setting_preference", Owns: []string{
			"domain",
			"primary_dns_server",
			"secondary_dns_server",
		}},
		{Mode: "timeout_setting_preference", Owns: []string{
			"icmp_timeout",
			"tcp_close_timeout",
			"tcp_close_wait_timeout",
			"tcp_established_timeout",
			"tcp_fin_wait_timeout",
			"tcp_last_ack_timeout",
			"tcp_syn_recv_timeout",
			"tcp_syn_sent_timeout",
			"tcp_time_wait_timeout",
			"udp_other_timeout",
			"udp_stream_timeout",
		}},
	},
	"WLAN": {
		{Mode: "minrate_setting_preference", Owns: []string{
			"minrate_na_data_rate_kbps",
			"minrate_na_enabled",
			"minrate_ng_data_rate_kbps",
		}},
		{Mode: "setting_preference", Owns: []string{
			"bc_filter_enabled",
			"bss_transition",
			"dtim_mode",
			"dtim_ng",
			"fast_roaming_enabled",
			"group_rekey",
			"l2_isolation",
			"mcastenhance_enabled",
			"proxy_arp",
			"uapsd_enabled",
		}},
	},
}
