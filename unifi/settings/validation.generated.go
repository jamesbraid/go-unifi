// Code generated from ace.jar fields *.json files
// DO NOT EDIT.

package settings

// FieldValidationPatterns holds the controller's own validation regex for
// every generated field that has one, keyed by Go type name and then by wire
// (JSON) name.
//
// This is the raw form, useful for building a regex-matching validator. The
// TypeFieldValues variables below are the subset of these patterns that is a
// plain enumeration, already split into values.
var FieldValidationPatterns = map[string]map[string]string{
	"SettingBroadcast": {
		"sound_after_type":  "sample|media",
		"sound_before_type": "sample|media",
	},
	"SettingDashboard": {
		"layout_preference": "auto|manual",
	},
	"SettingDashboardWidgets": {
		"name": "critical_traffic_prioritization|cybersecure|traffic_identification|wifi_technology|wifi_channels|wifi_client_experience|wifi_tx_retries|most_active_apps_aps_clients|most_active_apps_clients|most_active_aps_clients|most_active_apps_aps|most_active_apps|v2_most_active_aps|v2_most_active_clients|wifi_connectivity|ap_radio_density|wifi_channel_preset_configuration|most_common_client_fingerprints|wan_activity",
	},
	"SettingDeviceSupervision": {
		"heartbeat_interval_seconds": "^([6-9][0-9]|[1-2][0-9][0-9]|300)$",
		"power_off_duration_seconds": "^([6-9][0-9]|[1-9][0-9]{2}|[1-8][0-9]{3}|9000)$",
		"silence_threshold_seconds":  "^(300|[3-9][0-9][0-9]|[1-8][0-9]{3}|9000)$",
	},
	"SettingDoh": {
		"state": "off|auto|manual|custom",
	},
	"SettingEtherLightingNetworkOverrides": {
		"raw_color_hex": "[0-9A-Fa-f]{6}",
	},
	"SettingEtherLightingSpeedOverrides": {
		"key":           "FE|GbE|2.5GbE|5GbE|10GbE|25GbE|40GbE|100GbE",
		"raw_color_hex": "[0-9A-Fa-f]{6}",
	},
	"SettingGlobalAp": {
		"6e_channel_size":  "20|40|80|160",
		"6e_tx_power":      "[0-9]|[1-4][0-9]",
		"6e_tx_power_mode": "auto|medium|high|low|custom",
		"ap_exclusions":    "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
		"na_channel_size":  "20|40|80|160",
		"na_tx_power":      "[0-9]|[1-4][0-9]",
		"na_tx_power_mode": "auto|medium|high|low|custom",
		"ng_channel_size":  "20|40",
		"ng_tx_power":      "[0-9]|[1-4][0-9]",
		"ng_tx_power_mode": "auto|medium|high|low|custom",
	},
	"SettingGlobalNat": {
		"mode": "auto|custom|off",
	},
	"SettingGlobalSwitch": {
		"dot1x_fallback_networkconf_id": "[\\d\\w-]+|",
		"link_debounce":                 "0|[1-9]00|[1-4][0-9]00|5000",
		"poe_staging_delay_msec":        "0|200|400|600|800|1000|1200|1400|1600|1800|2000",
		"stp_version":                   "stp|rstp|disabled",
		"switch_exclusions":             "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
	},
	"SettingGuestAccess": {
		"auth":                                    "none|hotspot|custom",
		"custom_ip":                               "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"expire":                                  "[\\d]+|custom",
		"expire_number":                           "^[1-9][0-9]{0,5}|1000000$",
		"expire_unit":                             "1|60|1440",
		"gateway":                                 "paypal|stripe|authorize|quickpay|merchantwarrior|ippay",
		"portal_customized_bg_color":              "^#[a-zA-Z0-9]{6}$|^#[a-zA-Z0-9]{3}$|^$",
		"portal_customized_bg_type":               "color|image|gallery",
		"portal_customized_box_color":             "^#[a-zA-Z0-9]{6}$|^#[a-zA-Z0-9]{3}$|^$",
		"portal_customized_box_link_color":        "^#[a-zA-Z0-9]{6}$|^#[a-zA-Z0-9]{3}$|^$",
		"portal_customized_box_opacity":           "^[1-9][0-9]?$|^100$|^$",
		"portal_customized_box_radius":            "[0-9]|[1-4][0-9]|50",
		"portal_customized_box_text_color":        "^#[a-zA-Z0-9]{6}$|^#[a-zA-Z0-9]{3}$|^$",
		"portal_customized_button_color":          "^#[a-zA-Z0-9]{6}$|^#[a-zA-Z0-9]{3}$|^$",
		"portal_customized_button_text_color":     "^#[a-zA-Z0-9]{6}$|^#[a-zA-Z0-9]{3}$|^$",
		"portal_customized_languages":             "^[a-z]{2}([_-][a-zA-Z]{2,4})*$",
		"portal_customized_link_color":            "^#[a-zA-Z0-9]{6}$|^#[a-zA-Z0-9]{3}$|^$",
		"portal_customized_logo_position":         "left|center|right",
		"portal_customized_logo_size":             "6[4-9]|[7-9][0-9]|1[0-8][0-9]|19[0-2]",
		"portal_customized_text_color":            "^#[a-zA-Z0-9]{6}$|^#[a-zA-Z0-9]{3}$|^$",
		"portal_customized_welcome_text_position": "under_logo|above_boxes",
		"portal_hostname":                         "^[a-zA-Z0-9.-]+$|^$",
		"radius_auth_type":                        "chap|mschapv2",
		"radius_disconnect_port":                  "[1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5]",
		"restricted_dns_servers":                  "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
	},
	"SettingIgmpSnooping": {
		"querier_mode":              "PRIMARY_AND_FAILOVER|CUSTOM|OFF",
		"querier_subscription_mode": "ALL|CUSTOM",
		"subscription_mode":         "ALL|CUSTOM",
	},
	"SettingIgmpSnoopingQuerierAddresses": {
		"query_interval": "[3-9][0-9]|1[0-7][0-9]|180",
	},
	"SettingIps": {
		"advanced_filtering_preference": "|manual|disabled",
		"enabled_categories":            "emerging-activex|emerging-attackresponse|botcc|emerging-chat|ciarmy|compromised|emerging-dns|emerging-dos|dshield|emerging-exploit|emerging-ftp|emerging-games|emerging-icmp|emerging-icmpinfo|emerging-imap|emerging-inappropriate|emerging-info|emerging-malware|emerging-misc|emerging-mobile|emerging-netbios|emerging-p2p|emerging-policy|emerging-pop3|emerging-rpc|emerging-scada|emerging-scan|emerging-shellcode|emerging-smtp|emerging-snmp|emerging-sql|emerging-telnet|emerging-tftp|tor|emerging-useragent|emerging-voip|emerging-webapps|emerging-webclient|emerging-webserver|emerging-worm|exploit-kit|adware-pup|botcc-portgrouped|phishing|threatview-cs-c2|3coresec|chat|coinminer|current-events|drop|hunting|icmp-info|inappropriate|info|ja3|policy|scada|dark-web-blocker-list|malicious-hosts|dyn_dns|file_sharing|remote_access|ta_abused_services",
		"ips_mode":                      "ids|ips|ipsInline|disabled",
	},
	"SettingIpsHoneypot": {
		"version": "v4|v6",
	},
	"SettingIpsSuppressionAlerts": {
		"type": "all|track",
	},
	"SettingIpsSuppressionTracking": {
		"direction": "both|src|dest",
		"mode":      "ip|subnet|network",
	},
	"SettingIpsSuppressionWhitelist": {
		"direction": "both|src|dest",
		"mode":      "ip|subnet|network",
	},
	"SettingLcm": {
		"brightness":   "[1-9]|[1-9][0-9]|100",
		"idle_timeout": "[1-9][0-9]|[1-9][0-9][0-9]|[1-2][0-9][0-9][0-9]|3[0-5][0-9][0-9]|3600",
	},
	"SettingMdns": {
		"mode": "all|auto|custom",
	},
	"SettingMdnsCustomServices": {
		"address": "^_[a-zA-Z0-9._-]+\\._(tcp|udp)(\\.local)?$",
	},
	"SettingMdnsPredefinedServices": {
		"code": "amazon_devices|android_tv_remote|apple_airDrop|apple_airPlay|apple_file_sharing|apple_iChat|apple_iTunes|aqara|bose|dns_service_discovery|ftp_servers|google_chromecast|homeKit|matter_network|philips_hue|printers|roku|scanners|shelly|sonos|spotify_connect|ssh_servers|time_capsule|web_servers|windows_file_sharing_samba",
	},
	"SettingMgmt": {
		"auto_upgrade_hour": "[0-9]|1[0-9]|2[0-3]|^$",
		"x_mgmt_key":        "[0-9a-f]{32}",
		"x_ssh_password":    ".{1,128}",
		"x_ssh_username":    "^[_A-Za-z0-9][-_.A-Za-z0-9]{0,29}$",
	},
	"SettingNetflow": {
		"engine_id":     "^$|[1-9][0-9]*",
		"port":          "102[4-9]|10[3-9][0-9]|1[1-9][0-9]{2}|[2-9][0-9]{3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5]",
		"sampling_mode": "off|hash|random|deterministic",
		"sampling_rate": "[2-9]|[1-9][0-9]{1,3}|1[0-5][0-9]{3}|16[0-2][0-9]{2}|163[0-7][0-9]|1638[0-3]|^$",
		"server":        ".{0,252}[^\\.]$",
		"version":       "5|9|10",
	},
	"SettingNtp": {
		"setting_preference": "auto|manual",
	},
	"SettingRadioAi": {
		"auto_channel_presets_type": "maximum_speed|conservative|custom",
		"channels_6e":               "[1-9]|[1-2][0-9]|3[3-9]|[4-5][0-9]|6[0-1]|6[5-9]|[7-8][0-9]|9[0-3]|9[7-9]|1[0-1][0-9]|12[0-5]|129|1[3-4][0-9]|15[0-7]|16[1-9]|1[7-8][0-9]|19[3-9]|2[0-1][0-9]|22[0-1]|22[5-9]|233",
		"channels_na":               "34|36|38|40|42|44|46|48|52|56|60|64|100|104|108|112|116|120|124|128|132|136|140|144|149|153|157|161|165|169",
		"channels_ng":               "1|2|3|4|5|6|7|8|9|10|11|12|13|14",
		"exclude_devices":           "([0-9a-z]{2}:){5}[0-9a-z]{2}",
		"high_priority_devices":     "([0-9a-z]{2}:){5}[0-9a-z]{2}",
		"ht_modes_na":               "^(20|40|80|160)$",
		"ht_modes_ng":               "^(20|40)$",
		"optimize":                  "channel|power",
		"radios":                    "na|ng|6e",
		"setting_preference":        "auto|manual",
	},
	"SettingRadioAiChannelsBlacklist": {
		"channel":       "[1-9]|[1-9][0-9]|1[0-9][0-9]|2[0-9]|2[0-1][0-9]|22[0-1]|22[5-9]|233",
		"channel_width": "20|40|80|160|240|320",
		"radio":         "na|ng|6e",
	},
	"SettingRadioAiRadiosConfiguration": {
		"channel_width": "20|40|80|160|320",
		"radio":         "na|ng|6e",
	},
	"SettingRadius": {
		"acct_port":               "[1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5]",
		"auth_port":               "[1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5]",
		"interim_update_interval": "^([6-9][0-9]|[1-9][0-9]{2,3}|[1-7][0-9]{4}|8[0-5][0-9]{3}|86[0-3][0-9][0-9]|86400)$",
		"x_secret":                "^[^\\\\\"' ]{1,48}$",
	},
	"SettingRsyslogd": {
		"contents":        "device|client|firewall_default_policy|triggers|updates|admin_activity|critical|security_detections|vpn|gateway|access_points|switches",
		"netconsole_port": "[1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5]",
		"port":            "[1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5]",
	},
	"SettingSnmp": {
		"community":  ".{1,256}",
		"username":   "[a-zA-Z0-9_-]{1,30}",
		"x_password": "[^'\"]{8,32}",
	},
	"SettingSslInspection": {
		"state": "off|simple|advanced",
	},
	"SettingSuperFwupdate": {
		"controller_channel": "internal|alpha|beta|release-candidate|release",
		"firmware_channel":   "internal|alpha|beta|release-candidate|release",
	},
	"SettingSuperMail": {
		"provider": "smtp|cloud|disabled",
	},
	"SettingSuperMgmt": {
		"autobackup_post_actions":                 "copy_local|copy_cloud",
		"data_retention_setting_preference":       "auto|manual",
		"default_site_device_auth_password_alert": "false",
		"live_chat":     "disabled|super-only|everyone",
		"store_enabled": "disabled|super-only|everyone",
	},
	"SettingSuperSmtp": {
		"port": "[1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5]|^$",
	},
	"SettingTeleport": {
		"subnet_cidr": "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([8-9]|[1-2][0-9]|3[0-2])$|^$",
	},
	"SettingTestAndCommit": {
		"mode": "auto|custom",
	},
	"SettingUsg": {
		"arp_cache_base_reachable":   "^$|^[1-9]{1}[0-9]{0,4}$",
		"arp_cache_timeout":          "normal|min-dhcp-lease|custom",
		"dhcp_relay_agents_packets":  "append|discard|forward|replace|^$",
		"dhcp_relay_hop_count":       "([1-9]|[1-8][0-9]|9[0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])|^$",
		"dhcp_relay_max_size":        "(6[4-9]|[7-9][0-9]|[1-8][0-9]{2}|9[0-8][0-9]|99[0-9]|1[0-3][0-9]{2}|1400)|^$",
		"dhcp_relay_port":            "[1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5]|^$",
		"echo_server":                "[^\\\"\\' ]{1,255}",
		"mss_clamp":                  "auto|custom|disabled",
		"mss_clamp_mss":              "[1-9][0-9]{2,3}",
		"timeout_setting_preference": "auto|reduced|manual",
		"upnp_wan_interface":         "WAN[2-9]?",
	},
	"SettingUsgDNSVerification": {
		"setting_preference": "auto|manual",
	},
	"SettingUsgGeoIPFiltering": {
		"action":            "block|allow",
		"countries":         "^([A-Z]{2})?(,[A-Z]{2}){0,149}$",
		"traffic_direction": "^(both|ingress|egress)$",
	},
}

// SettingBroadcastSoundAfterTypeValues are the values the controller accepts for SettingBroadcast.sound_after_type.
var SettingBroadcastSoundAfterTypeValues = []string{"sample", "media"}

// SettingBroadcastSoundBeforeTypeValues are the values the controller accepts for SettingBroadcast.sound_before_type.
var SettingBroadcastSoundBeforeTypeValues = []string{"sample", "media"}

// SettingDashboardLayoutPreferenceValues are the values the controller accepts for SettingDashboard.layout_preference.
var SettingDashboardLayoutPreferenceValues = []string{"auto", "manual"}

// SettingDashboardWidgetsNameValues are the values the controller accepts for SettingDashboardWidgets.name.
var SettingDashboardWidgetsNameValues = []string{"critical_traffic_prioritization", "cybersecure", "traffic_identification", "wifi_technology", "wifi_channels", "wifi_client_experience", "wifi_tx_retries", "most_active_apps_aps_clients", "most_active_apps_clients", "most_active_aps_clients", "most_active_apps_aps", "most_active_apps", "v2_most_active_aps", "v2_most_active_clients", "wifi_connectivity", "ap_radio_density", "wifi_channel_preset_configuration", "most_common_client_fingerprints", "wan_activity"}

// SettingDeviceSupervisionHeartbeatIntervalSecondsMin and SettingDeviceSupervisionHeartbeatIntervalSecondsMax are the inclusive bounds the controller accepts for SettingDeviceSupervision.heartbeat_interval_seconds.
const (
	SettingDeviceSupervisionHeartbeatIntervalSecondsMin int64 = 60
	SettingDeviceSupervisionHeartbeatIntervalSecondsMax int64 = 300
)

// SettingDeviceSupervisionPowerOffDurationSecondsMin and SettingDeviceSupervisionPowerOffDurationSecondsMax are the inclusive bounds the controller accepts for SettingDeviceSupervision.power_off_duration_seconds.
const (
	SettingDeviceSupervisionPowerOffDurationSecondsMin int64 = 60
	SettingDeviceSupervisionPowerOffDurationSecondsMax int64 = 9000
)

// SettingDeviceSupervisionSilenceThresholdSecondsMin and SettingDeviceSupervisionSilenceThresholdSecondsMax are the inclusive bounds the controller accepts for SettingDeviceSupervision.silence_threshold_seconds.
const (
	SettingDeviceSupervisionSilenceThresholdSecondsMin int64 = 300
	SettingDeviceSupervisionSilenceThresholdSecondsMax int64 = 9000
)

// SettingDohStateValues are the values the controller accepts for SettingDoh.state.
var SettingDohStateValues = []string{"off", "auto", "manual", "custom"}

// SettingGlobalApSixEChannelSizeValues are the values the controller accepts for SettingGlobalAp.6e_channel_size.
var SettingGlobalApSixEChannelSizeValues = []int64{20, 40, 80, 160}

// SettingGlobalApSixETxPowerMin and SettingGlobalApSixETxPowerMax are the inclusive bounds the controller accepts for SettingGlobalAp.6e_tx_power.
const (
	SettingGlobalApSixETxPowerMin int64 = 0
	SettingGlobalApSixETxPowerMax int64 = 49
)

// SettingGlobalApSixETxPowerModeValues are the values the controller accepts for SettingGlobalAp.6e_tx_power_mode.
var SettingGlobalApSixETxPowerModeValues = []string{"auto", "medium", "high", "low", "custom"}

// SettingGlobalApNaChannelSizeValues are the values the controller accepts for SettingGlobalAp.na_channel_size.
var SettingGlobalApNaChannelSizeValues = []int64{20, 40, 80, 160}

// SettingGlobalApNaTxPowerMin and SettingGlobalApNaTxPowerMax are the inclusive bounds the controller accepts for SettingGlobalAp.na_tx_power.
const (
	SettingGlobalApNaTxPowerMin int64 = 0
	SettingGlobalApNaTxPowerMax int64 = 49
)

// SettingGlobalApNaTxPowerModeValues are the values the controller accepts for SettingGlobalAp.na_tx_power_mode.
var SettingGlobalApNaTxPowerModeValues = []string{"auto", "medium", "high", "low", "custom"}

// SettingGlobalApNgChannelSizeValues are the values the controller accepts for SettingGlobalAp.ng_channel_size.
var SettingGlobalApNgChannelSizeValues = []int64{20, 40}

// SettingGlobalApNgTxPowerMin and SettingGlobalApNgTxPowerMax are the inclusive bounds the controller accepts for SettingGlobalAp.ng_tx_power.
const (
	SettingGlobalApNgTxPowerMin int64 = 0
	SettingGlobalApNgTxPowerMax int64 = 49
)

// SettingGlobalApNgTxPowerModeValues are the values the controller accepts for SettingGlobalAp.ng_tx_power_mode.
var SettingGlobalApNgTxPowerModeValues = []string{"auto", "medium", "high", "low", "custom"}

// SettingGlobalNatModeValues are the values the controller accepts for SettingGlobalNat.mode.
var SettingGlobalNatModeValues = []string{"auto", "custom", "off"}

// SettingGlobalSwitchPoeStagingDelayMsecValues are the values the controller accepts for SettingGlobalSwitch.poe_staging_delay_msec.
var SettingGlobalSwitchPoeStagingDelayMsecValues = []int64{0, 200, 400, 600, 800, 1000, 1200, 1400, 1600, 1800, 2000}

// SettingGlobalSwitchStpVersionValues are the values the controller accepts for SettingGlobalSwitch.stp_version.
var SettingGlobalSwitchStpVersionValues = []string{"stp", "rstp", "disabled"}

// SettingGuestAccessAuthValues are the values the controller accepts for SettingGuestAccess.auth.
var SettingGuestAccessAuthValues = []string{"none", "hotspot", "custom"}

// SettingGuestAccessExpireUnitValues are the values the controller accepts for SettingGuestAccess.expire_unit.
var SettingGuestAccessExpireUnitValues = []int64{1, 60, 1440}

// SettingGuestAccessGatewayValues are the values the controller accepts for SettingGuestAccess.gateway.
var SettingGuestAccessGatewayValues = []string{"paypal", "stripe", "authorize", "quickpay", "merchantwarrior", "ippay"}

// SettingGuestAccessPortalCustomizedBgTypeValues are the values the controller accepts for SettingGuestAccess.portal_customized_bg_type.
var SettingGuestAccessPortalCustomizedBgTypeValues = []string{"color", "image", "gallery"}

// SettingGuestAccessPortalCustomizedBoxOpacityMin and SettingGuestAccessPortalCustomizedBoxOpacityMax are the inclusive bounds the controller accepts for SettingGuestAccess.portal_customized_box_opacity.
const (
	SettingGuestAccessPortalCustomizedBoxOpacityMin int64 = 1
	SettingGuestAccessPortalCustomizedBoxOpacityMax int64 = 100
)

// SettingGuestAccessPortalCustomizedBoxRADIUSMin and SettingGuestAccessPortalCustomizedBoxRADIUSMax are the inclusive bounds the controller accepts for SettingGuestAccess.portal_customized_box_radius.
const (
	SettingGuestAccessPortalCustomizedBoxRADIUSMin int64 = 0
	SettingGuestAccessPortalCustomizedBoxRADIUSMax int64 = 50
)

// SettingGuestAccessPortalCustomizedLogoPositionValues are the values the controller accepts for SettingGuestAccess.portal_customized_logo_position.
var SettingGuestAccessPortalCustomizedLogoPositionValues = []string{"left", "center", "right"}

// SettingGuestAccessPortalCustomizedLogoSizeMin and SettingGuestAccessPortalCustomizedLogoSizeMax are the inclusive bounds the controller accepts for SettingGuestAccess.portal_customized_logo_size.
const (
	SettingGuestAccessPortalCustomizedLogoSizeMin int64 = 64
	SettingGuestAccessPortalCustomizedLogoSizeMax int64 = 192
)

// SettingGuestAccessPortalCustomizedWelcomeTextPositionValues are the values the controller accepts for SettingGuestAccess.portal_customized_welcome_text_position.
var SettingGuestAccessPortalCustomizedWelcomeTextPositionValues = []string{"under_logo", "above_boxes"}

// SettingGuestAccessRADIUSAuthTypeValues are the values the controller accepts for SettingGuestAccess.radius_auth_type.
var SettingGuestAccessRADIUSAuthTypeValues = []string{"chap", "mschapv2"}

// SettingGuestAccessRADIUSDisconnectPortMin and SettingGuestAccessRADIUSDisconnectPortMax are the inclusive bounds the controller accepts for SettingGuestAccess.radius_disconnect_port.
const (
	SettingGuestAccessRADIUSDisconnectPortMin int64 = 1
	SettingGuestAccessRADIUSDisconnectPortMax int64 = 65535
)

// SettingIgmpSnoopingQuerierModeValues are the values the controller accepts for SettingIgmpSnooping.querier_mode.
var SettingIgmpSnoopingQuerierModeValues = []string{"PRIMARY_AND_FAILOVER", "CUSTOM", "OFF"}

// SettingIgmpSnoopingQuerierSubscriptionModeValues are the values the controller accepts for SettingIgmpSnooping.querier_subscription_mode.
var SettingIgmpSnoopingQuerierSubscriptionModeValues = []string{"ALL", "CUSTOM"}

// SettingIgmpSnoopingSubscriptionModeValues are the values the controller accepts for SettingIgmpSnooping.subscription_mode.
var SettingIgmpSnoopingSubscriptionModeValues = []string{"ALL", "CUSTOM"}

// SettingIgmpSnoopingQuerierAddressesQueryIntervalMin and SettingIgmpSnoopingQuerierAddressesQueryIntervalMax are the inclusive bounds the controller accepts for SettingIgmpSnoopingQuerierAddresses.query_interval.
const (
	SettingIgmpSnoopingQuerierAddressesQueryIntervalMin int64 = 30
	SettingIgmpSnoopingQuerierAddressesQueryIntervalMax int64 = 180
)

// SettingIpsEnabledCategoriesValues are the values the controller accepts for SettingIps.enabled_categories.
var SettingIpsEnabledCategoriesValues = []string{"emerging-activex", "emerging-attackresponse", "botcc", "emerging-chat", "ciarmy", "compromised", "emerging-dns", "emerging-dos", "dshield", "emerging-exploit", "emerging-ftp", "emerging-games", "emerging-icmp", "emerging-icmpinfo", "emerging-imap", "emerging-inappropriate", "emerging-info", "emerging-malware", "emerging-misc", "emerging-mobile", "emerging-netbios", "emerging-p2p", "emerging-policy", "emerging-pop3", "emerging-rpc", "emerging-scada", "emerging-scan", "emerging-shellcode", "emerging-smtp", "emerging-snmp", "emerging-sql", "emerging-telnet", "emerging-tftp", "tor", "emerging-useragent", "emerging-voip", "emerging-webapps", "emerging-webclient", "emerging-webserver", "emerging-worm", "exploit-kit", "adware-pup", "botcc-portgrouped", "phishing", "threatview-cs-c2", "3coresec", "chat", "coinminer", "current-events", "drop", "hunting", "icmp-info", "inappropriate", "info", "ja3", "policy", "scada", "dark-web-blocker-list", "malicious-hosts", "dyn_dns", "file_sharing", "remote_access", "ta_abused_services"}

// SettingIpsIPsModeValues are the values the controller accepts for SettingIps.ips_mode.
var SettingIpsIPsModeValues = []string{"ids", "ips", "ipsInline", "disabled"}

// SettingIpsHoneypotVersionValues are the values the controller accepts for SettingIpsHoneypot.version.
var SettingIpsHoneypotVersionValues = []string{"v4", "v6"}

// SettingIpsSuppressionAlertsTypeValues are the values the controller accepts for SettingIpsSuppressionAlerts.type.
var SettingIpsSuppressionAlertsTypeValues = []string{"all", "track"}

// SettingIpsSuppressionTrackingDirectionValues are the values the controller accepts for SettingIpsSuppressionTracking.direction.
var SettingIpsSuppressionTrackingDirectionValues = []string{"both", "src", "dest"}

// SettingIpsSuppressionTrackingModeValues are the values the controller accepts for SettingIpsSuppressionTracking.mode.
var SettingIpsSuppressionTrackingModeValues = []string{"ip", "subnet", "network"}

// SettingIpsSuppressionWhitelistDirectionValues are the values the controller accepts for SettingIpsSuppressionWhitelist.direction.
var SettingIpsSuppressionWhitelistDirectionValues = []string{"both", "src", "dest"}

// SettingIpsSuppressionWhitelistModeValues are the values the controller accepts for SettingIpsSuppressionWhitelist.mode.
var SettingIpsSuppressionWhitelistModeValues = []string{"ip", "subnet", "network"}

// SettingLcmBrightnessMin and SettingLcmBrightnessMax are the inclusive bounds the controller accepts for SettingLcm.brightness.
const (
	SettingLcmBrightnessMin int64 = 1
	SettingLcmBrightnessMax int64 = 100
)

// SettingLcmIDleTimeoutMin and SettingLcmIDleTimeoutMax are the inclusive bounds the controller accepts for SettingLcm.idle_timeout.
const (
	SettingLcmIDleTimeoutMin int64 = 10
	SettingLcmIDleTimeoutMax int64 = 3600
)

// SettingMdnsModeValues are the values the controller accepts for SettingMdns.mode.
var SettingMdnsModeValues = []string{"all", "auto", "custom"}

// SettingMdnsPredefinedServicesCodeValues are the values the controller accepts for SettingMdnsPredefinedServices.code.
var SettingMdnsPredefinedServicesCodeValues = []string{"amazon_devices", "android_tv_remote", "apple_airDrop", "apple_airPlay", "apple_file_sharing", "apple_iChat", "apple_iTunes", "aqara", "bose", "dns_service_discovery", "ftp_servers", "google_chromecast", "homeKit", "matter_network", "philips_hue", "printers", "roku", "scanners", "shelly", "sonos", "spotify_connect", "ssh_servers", "time_capsule", "web_servers", "windows_file_sharing_samba"}

// SettingMgmtAutoUpgradeHourMin and SettingMgmtAutoUpgradeHourMax are the inclusive bounds the controller accepts for SettingMgmt.auto_upgrade_hour.
const (
	SettingMgmtAutoUpgradeHourMin int64 = 0
	SettingMgmtAutoUpgradeHourMax int64 = 23
)

// SettingMgmtSSHPasswordMinLength and SettingMgmtSSHPasswordMaxLength are the character-count bounds the controller accepts for SettingMgmt.x_ssh_password.
const (
	SettingMgmtSSHPasswordMinLength int64 = 1
	SettingMgmtSSHPasswordMaxLength int64 = 128
)

// SettingNetflowPortMin and SettingNetflowPortMax are the inclusive bounds the controller accepts for SettingNetflow.port.
const (
	SettingNetflowPortMin int64 = 1024
	SettingNetflowPortMax int64 = 65535
)

// SettingNetflowSamplingModeValues are the values the controller accepts for SettingNetflow.sampling_mode.
var SettingNetflowSamplingModeValues = []string{"off", "hash", "random", "deterministic"}

// SettingNetflowSamplingRateMin and SettingNetflowSamplingRateMax are the inclusive bounds the controller accepts for SettingNetflow.sampling_rate.
const (
	SettingNetflowSamplingRateMin int64 = 2
	SettingNetflowSamplingRateMax int64 = 16383
)

// SettingNetflowVersionValues are the values the controller accepts for SettingNetflow.version.
var SettingNetflowVersionValues = []int64{5, 9, 10}

// SettingNtpSettingPreferenceValues are the values the controller accepts for SettingNtp.setting_preference.
var SettingNtpSettingPreferenceValues = []string{"auto", "manual"}

// SettingRadioAiAutoChannelPresetsTypeValues are the values the controller accepts for SettingRadioAi.auto_channel_presets_type.
var SettingRadioAiAutoChannelPresetsTypeValues = []string{"maximum_speed", "conservative", "custom"}

// SettingRadioAiChannelsNaValues are the values the controller accepts for SettingRadioAi.channels_na.
var SettingRadioAiChannelsNaValues = []int64{34, 36, 38, 40, 42, 44, 46, 48, 52, 56, 60, 64, 100, 104, 108, 112, 116, 120, 124, 128, 132, 136, 140, 144, 149, 153, 157, 161, 165, 169}

// SettingRadioAiChannelsNgValues are the values the controller accepts for SettingRadioAi.channels_ng.
var SettingRadioAiChannelsNgValues = []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}

// SettingRadioAiHtModesNaValues are the values the controller accepts for SettingRadioAi.ht_modes_na.
var SettingRadioAiHtModesNaValues = []int64{20, 40, 80, 160}

// SettingRadioAiHtModesNgValues are the values the controller accepts for SettingRadioAi.ht_modes_ng.
var SettingRadioAiHtModesNgValues = []int64{20, 40}

// SettingRadioAiOptimizeValues are the values the controller accepts for SettingRadioAi.optimize.
var SettingRadioAiOptimizeValues = []string{"channel", "power"}

// SettingRadioAiRadiosValues are the values the controller accepts for SettingRadioAi.radios.
var SettingRadioAiRadiosValues = []string{"na", "ng", "6e"}

// SettingRadioAiSettingPreferenceValues are the values the controller accepts for SettingRadioAi.setting_preference.
var SettingRadioAiSettingPreferenceValues = []string{"auto", "manual"}

// SettingRadioAiChannelsBlacklistChannelWidthValues are the values the controller accepts for SettingRadioAiChannelsBlacklist.channel_width.
var SettingRadioAiChannelsBlacklistChannelWidthValues = []int64{20, 40, 80, 160, 240, 320}

// SettingRadioAiChannelsBlacklistRadioValues are the values the controller accepts for SettingRadioAiChannelsBlacklist.radio.
var SettingRadioAiChannelsBlacklistRadioValues = []string{"na", "ng", "6e"}

// SettingRadioAiRadiosConfigurationChannelWidthValues are the values the controller accepts for SettingRadioAiRadiosConfiguration.channel_width.
var SettingRadioAiRadiosConfigurationChannelWidthValues = []int64{20, 40, 80, 160, 320}

// SettingRadioAiRadiosConfigurationRadioValues are the values the controller accepts for SettingRadioAiRadiosConfiguration.radio.
var SettingRadioAiRadiosConfigurationRadioValues = []string{"na", "ng", "6e"}

// SettingRadiusAcctPortMin and SettingRadiusAcctPortMax are the inclusive bounds the controller accepts for SettingRadius.acct_port.
const (
	SettingRadiusAcctPortMin int64 = 1
	SettingRadiusAcctPortMax int64 = 65535
)

// SettingRadiusAuthPortMin and SettingRadiusAuthPortMax are the inclusive bounds the controller accepts for SettingRadius.auth_port.
const (
	SettingRadiusAuthPortMin int64 = 1
	SettingRadiusAuthPortMax int64 = 65535
)

// SettingRadiusInterimUpdateIntervalMin and SettingRadiusInterimUpdateIntervalMax are the inclusive bounds the controller accepts for SettingRadius.interim_update_interval.
const (
	SettingRadiusInterimUpdateIntervalMin int64 = 60
	SettingRadiusInterimUpdateIntervalMax int64 = 86400
)

// SettingRsyslogdContentsValues are the values the controller accepts for SettingRsyslogd.contents.
var SettingRsyslogdContentsValues = []string{"device", "client", "firewall_default_policy", "triggers", "updates", "admin_activity", "critical", "security_detections", "vpn", "gateway", "access_points", "switches"}

// SettingRsyslogdNetconsolePortMin and SettingRsyslogdNetconsolePortMax are the inclusive bounds the controller accepts for SettingRsyslogd.netconsole_port.
const (
	SettingRsyslogdNetconsolePortMin int64 = 1
	SettingRsyslogdNetconsolePortMax int64 = 65535
)

// SettingRsyslogdPortMin and SettingRsyslogdPortMax are the inclusive bounds the controller accepts for SettingRsyslogd.port.
const (
	SettingRsyslogdPortMin int64 = 1
	SettingRsyslogdPortMax int64 = 65535
)

// SettingSnmpCommunityMinLength and SettingSnmpCommunityMaxLength are the character-count bounds the controller accepts for SettingSnmp.community.
const (
	SettingSnmpCommunityMinLength int64 = 1
	SettingSnmpCommunityMaxLength int64 = 256
)

// SettingSslInspectionStateValues are the values the controller accepts for SettingSslInspection.state.
var SettingSslInspectionStateValues = []string{"off", "simple", "advanced"}

// SettingSuperFwupdateControllerChannelValues are the values the controller accepts for SettingSuperFwupdate.controller_channel.
var SettingSuperFwupdateControllerChannelValues = []string{"internal", "alpha", "beta", "release-candidate", "release"}

// SettingSuperFwupdateFirmwareChannelValues are the values the controller accepts for SettingSuperFwupdate.firmware_channel.
var SettingSuperFwupdateFirmwareChannelValues = []string{"internal", "alpha", "beta", "release-candidate", "release"}

// SettingSuperMailProviderValues are the values the controller accepts for SettingSuperMail.provider.
var SettingSuperMailProviderValues = []string{"smtp", "cloud", "disabled"}

// SettingSuperMgmtAutobackupPostActionsValues are the values the controller accepts for SettingSuperMgmt.autobackup_post_actions.
var SettingSuperMgmtAutobackupPostActionsValues = []string{"copy_local", "copy_cloud"}

// SettingSuperMgmtDataRetentionSettingPreferenceValues are the values the controller accepts for SettingSuperMgmt.data_retention_setting_preference.
var SettingSuperMgmtDataRetentionSettingPreferenceValues = []string{"auto", "manual"}

// SettingSuperMgmtLiveChatValues are the values the controller accepts for SettingSuperMgmt.live_chat.
var SettingSuperMgmtLiveChatValues = []string{"disabled", "super-only", "everyone"}

// SettingSuperMgmtStoreEnabledValues are the values the controller accepts for SettingSuperMgmt.store_enabled.
var SettingSuperMgmtStoreEnabledValues = []string{"disabled", "super-only", "everyone"}

// SettingSuperSmtpPortMin and SettingSuperSmtpPortMax are the inclusive bounds the controller accepts for SettingSuperSmtp.port.
const (
	SettingSuperSmtpPortMin int64 = 1
	SettingSuperSmtpPortMax int64 = 65535
)

// SettingTestAndCommitModeValues are the values the controller accepts for SettingTestAndCommit.mode.
var SettingTestAndCommitModeValues = []string{"auto", "custom"}

// SettingUsgArpCacheBaseReachableMin and SettingUsgArpCacheBaseReachableMax are the inclusive bounds the controller accepts for SettingUsg.arp_cache_base_reachable.
const (
	SettingUsgArpCacheBaseReachableMin int64 = 1
	SettingUsgArpCacheBaseReachableMax int64 = 99999
)

// SettingUsgArpCacheTimeoutValues are the values the controller accepts for SettingUsg.arp_cache_timeout.
var SettingUsgArpCacheTimeoutValues = []string{"normal", "min-dhcp-lease", "custom"}

// SettingUsgDHCPRelayHopCountMin and SettingUsgDHCPRelayHopCountMax are the inclusive bounds the controller accepts for SettingUsg.dhcp_relay_hop_count.
const (
	SettingUsgDHCPRelayHopCountMin int64 = 1
	SettingUsgDHCPRelayHopCountMax int64 = 255
)

// SettingUsgDHCPRelayMaxSizeMin and SettingUsgDHCPRelayMaxSizeMax are the inclusive bounds the controller accepts for SettingUsg.dhcp_relay_max_size.
const (
	SettingUsgDHCPRelayMaxSizeMin int64 = 64
	SettingUsgDHCPRelayMaxSizeMax int64 = 1400
)

// SettingUsgDHCPRelayPortMin and SettingUsgDHCPRelayPortMax are the inclusive bounds the controller accepts for SettingUsg.dhcp_relay_port.
const (
	SettingUsgDHCPRelayPortMin int64 = 1
	SettingUsgDHCPRelayPortMax int64 = 65535
)

// SettingUsgMssClampValues are the values the controller accepts for SettingUsg.mss_clamp.
var SettingUsgMssClampValues = []string{"auto", "custom", "disabled"}

// SettingUsgMssClampMssMin and SettingUsgMssClampMssMax are the inclusive bounds the controller accepts for SettingUsg.mss_clamp_mss.
const (
	SettingUsgMssClampMssMin int64 = 100
	SettingUsgMssClampMssMax int64 = 9999
)

// SettingUsgTimeoutSettingPreferenceValues are the values the controller accepts for SettingUsg.timeout_setting_preference.
var SettingUsgTimeoutSettingPreferenceValues = []string{"auto", "reduced", "manual"}

// SettingUsgDNSVerificationSettingPreferenceValues are the values the controller accepts for SettingUsgDNSVerification.setting_preference.
var SettingUsgDNSVerificationSettingPreferenceValues = []string{"auto", "manual"}

// SettingUsgGeoIPFilteringActionValues are the values the controller accepts for SettingUsgGeoIPFiltering.action.
var SettingUsgGeoIPFilteringActionValues = []string{"block", "allow"}

// SettingUsgGeoIPFilteringTrafficDirectionValues are the values the controller accepts for SettingUsgGeoIPFiltering.traffic_direction.
var SettingUsgGeoIPFilteringTrafficDirectionValues = []string{"both", "ingress", "egress"}
