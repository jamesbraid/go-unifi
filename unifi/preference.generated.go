// Generated code. DO NOT EDIT.

package unifi

// PreferenceOwnedFields records what each auto|manual mode field owns.
//
// A UniFi resource can carry a mode field -- setting_preference and its
// siblings -- that decides whether a block of its own fields is the caller's
// to set or the controller's. While the mode is "auto" the controller stores
// its own values over whatever the payload asked for, answers rc: ok, and
// reports nothing, so a caller learns from the next read or not at all.
//
// The outer key is the resource's schema name (settings keep their "Setting"
// prefix, so the site NTP document is "SettingNtp"). The inner key is the
// mode field's wire name, and the value is the wire names it owns. An empty
// list is a measured result, not a gap: that mode was probed and owns
// nothing.
//
// Measured against a live controller by TestIntegrationPreferenceOwnership
// and recorded in overrides/fields.toml; the build of each set is in the
// "measured" key beside it there.
var PreferenceOwnedFields = map[string]map[string][]string{
	"FirewallRule": {
		"setting_preference": {
			"state_established",
			"state_related",
		},
	},
	"Nat": {
		"setting_preference": {},
	},
	"Network": {
		"ipv6_setting_preference": {
			"dhcpdv6_dns_auto",
			"dhcpdv6_leasetime",
			"ipv6_ra_preferred_lifetime",
		},
		"setting_preference": {
			"dhcpd_dns_enabled",
			"dhcpd_gateway_enabled",
			"dhcpd_leasetime",
			"dhcpd_ntp_enabled",
			"dhcpd_tftp_server",
			"dhcpd_time_offset_enabled",
			"dhcpd_unifi_controller",
			"dhcpd_wins_enabled",
			"dhcpguard_enabled",
			"domain_name",
			"igmp_snooping",
			"upnp_lan_enabled",
		},
		"wan_dns_preference": {
			"wan_dns1",
			"wan_dns2",
		},
		"wan_ipv6_dns_preference": {},
	},
	"PortProfile": {
		"setting_preference": {
			"autoneg",
			"egress_rate_limit_kbps_enabled",
			"isolation",
			"lldpmed_enabled",
			"lldpmed_notify_enabled",
			"port_keepalive_enabled",
			"stormctrl_bcast_enabled",
			"stormctrl_mcast_enabled",
			"stormctrl_ucast_enabled",
			"stp_port_mode",
		},
	},
	"SettingDashboard": {
		"layout_preference": {
			"widgets",
		},
	},
	"SettingNtp": {
		"setting_preference": {
			"ntp_server_1",
			"ntp_server_2",
			"ntp_server_3",
			"ntp_server_4",
		},
	},
	"SettingRadioAi": {
		"setting_preference": {
			"cron_expr",
		},
	},
	"SettingSuperMgmt": {
		"data_retention_setting_preference": {
			"data_retention_time_in_hours_for_5minutes_scale",
			"data_retention_time_in_hours_for_hourly_scale",
			"data_retention_time_in_hours_for_others",
		},
	},
	"SettingUsg": {
		"timeout_setting_preference": {
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
		},
	},
	"WLAN": {
		"minrate_setting_preference": {
			"minrate_na_data_rate_kbps",
			"minrate_na_enabled",
			"minrate_ng_data_rate_kbps",
		},
		"setting_preference": {
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
		},
	},
}
