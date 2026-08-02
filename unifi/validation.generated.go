// Code generated from ace.jar fields *.json files
// DO NOT EDIT.

package unifi

// FieldValidationPatterns holds the controller's own validation regex for
// every generated field that has one, keyed by Go type name and then by wire
// (JSON) name.
//
// This is the raw form, useful for building a regex-matching validator. The
// TypeFieldValues variables below are the subset of these patterns that is a
// plain enumeration, already split into values.
var FieldValidationPatterns = map[string]map[string]string{
	"Account": {
		"ip":                 "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"name":               "^[^\"' ]+$",
		"tunnel_config_type": "vpn|802.1x|custom",
		"tunnel_medium_type": "[1-9]|1[0-5]|^$",
		"tunnel_type":        "[1-9]|1[0-3]|^$",
		"vlan":               "[2-9]|[1-9][0-9]{1,2}|[1-3][0-9]{3}|400[0-9]|^$",
	},
	"BGPConfig": {
		"description":        ".{0,128}",
		"uploaded_file_name": ".{0,256}",
	},
	"ChannelPlan": {
		"date": "^$|^(20[0-9]{2}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9])Z?$",
	},
	"ChannelPlanRadioTable": {
		"channel":       "[0-9]|[1][0-4]|16|34|36|38|40|42|44|46|48|52|56|60|64|100|104|108|112|116|120|124|128|132|136|140|144|149|153|157|161|165|183|184|185|187|188|189|192|196|auto",
		"device_mac":    "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
		"name":          "[a-z]*[0-9]*",
		"tx_power":      "[\\d]+|auto",
		"tx_power_mode": "auto|medium|high|low|custom",
		"width":         "20|40|80|160",
	},
	"Client": {
		"display_name": "non-generated field",
		"fixed_ap_mac": "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
		"last_ip":      "non-generated field",
		"mac":          "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
	},
	"ClientGroup": {
		"name":              ".{1,128}",
		"qos_rate_max_down": "-1|[2-9]|[1-9][0-9]{1,4}|100000",
		"qos_rate_max_up":   "-1|[2-9]|[1-9][0-9]{1,4}|100000",
	},
	"DHCPOption": {
		"code":  "^(?!(?:15|42|43|44|51|66|67|252)$)([7-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-4])$",
		"name":  "^[A-Za-z0-9-_]{1,25}$",
		"type":  "^(boolean|hexarray|integer|ipaddress|macaddress|text)$",
		"width": "^(8|16|32)$",
	},
	"DNSRecord": {
		"key":         ".{1,128}",
		"port":        "[1-9][0-9]{0,4}",
		"priority":    ".{1,128}",
		"record_type": "A|AAAA|CNAME|MX|NS|PTR|SOA|SRV|TXT",
		"value":       ".{1,256}",
	},
	"Device": {
		"bandsteering_mode":                         "off|equal|prefer_5g",
		"baresip_auth_user":                         "^\\+?[a-zA-Z0-9_.\\-!~*'()]*",
		"baresip_extension":                         "^\\+?[a-zA-Z0-9_.\\-!~*'()]*",
		"dot1x_fallback_networkconf_id":             "[\\d\\w]+|",
		"eav_bridge_role":                           "host|client",
		"fan_mode_override":                         "default|quiet",
		"gateway_vrrp_mode":                         "primary|secondary",
		"gateway_vrrp_priority":                     "[1-9][0-9]|[1-9][0-9][0-9]",
		"hostname":                                  ".{1,128}",
		"lcm_brightness":                            "[1-9]|[1-9][0-9]|100",
		"lcm_idle_timeout":                          "[1-9][0-9]|[1-9][0-9][0-9]|[1-2][0-9][0-9][0-9]|3[0-5][0-9][0-9]|3600",
		"lcm_night_mode_begins":                     "(^$)|(^(0[0-9])|(1[0-9])|(2[0-3])):([0-5][0-9]$)",
		"lcm_night_mode_ends":                       "(^$)|(^(0[0-9])|(1[0-9])|(2[0-3])):([0-5][0-9]$)",
		"lcm_orientation_override":                  "0|90|180|270",
		"lcm_tracker_seed":                          ".{0,50}",
		"led_override":                              "default|on|off",
		"led_override_color":                        "^#(?:[0-9a-fA-F]{3}){1,2}$",
		"led_override_color_brightness":             "^[0-9][0-9]?$|^100$",
		"lte_apn":                                   ".{1,128}",
		"lte_auth_type":                             "PAP|CHAP|PAP-CHAP|NONE",
		"mgmt_network_id":                           "[\\d\\w]+",
		"name":                                      ".{0,128}",
		"outdoor_mode_override":                     "default|on|off",
		"outlet_power_cycle_on_ac_recovery_seconds": "[6-9][0-9]|[1-5][0-9]{2}|600",
		"peer_to_peer_mode":                         "ap|sta",
		"poe_mode":                                  "auto|pasv24|passthrough|off",
		"power_source_ctrl":                         "auto|8023af|8023at|8023bt-type3|8023bt-type4|pasv24|poe-injector|ac|adapter|dc|rps",
		"power_source_ctrl_budget":                  "[0-9]|[1-9][0-9]|[1-9][0-9][0-9]|[1-9][0-9][0-9][0-9]|[1-9][0-9][0-9][0-9][0-9]|[1-9][0-9][0-9][0-9][0-9][0-9]",
		"ptmp_ap_mac":                               "^([0-9A-Fa-f]{2}[:]){5}([0-9A-Fa-f]{2})$",
		"ptp_ap_mac":                                "^([0-9A-Fa-f]{2}[:]){5}([0-9A-Fa-f]{2})$",
		"resetbtn_enabled":                          "on|off",
		"snmp_contact":                              ".{0,255}",
		"snmp_location":                             ".{0,255}",
		"station_mode":                              "ptp|ptmp|wifi",
		"stp_priority":                              "0|4096|8192|12288|16384|20480|24576|28672|32768|36864|40960|45056|49152|53248|57344|61440",
		"stp_version":                               "stp|rstp|disabled",
		"ubb_pair_name":                             ".{1,128}",
		"ups_shutdown_remaining_minutes":            "[1-9]|1[0-5]",
		"volume":                                    "[0-9]|[1-9][0-9]|100",
		"x_baresip_password":                        "^[a-zA-Z0-9_.\\-!~*'()]*",
	},
	"DeviceAudioInfo": {
		"channel": "[2-9]|[1-9][0-9]|[1-9][0-9][0-9]|[1-9][0-9][0-9][0-9]",
		"role":    "host|client",
	},
	"DeviceConfigNetwork": {
		"dns1":    "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$|^$",
		"dns2":    "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$|^$",
		"gateway": "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"ip":      "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"netmask": "^((128|192|224|240|248|252|254)\\.0\\.0\\.0)|(255\\.(((0|128|192|224|240|248|252|254)\\.0\\.0)|(255\\.(((0|128|192|224|240|248|252|254)\\.0)|255\\.(0|128|192|224|240|248|252|254)))))$",
		"type":    "dhcp|static",
	},
	"DeviceCurrentApn": {
		"auth_type": "PAP|CHAP|PAP-CHAP|NONE",
		"pdp_type":  "IPv4|IPv6|IPv4v6",
	},
	"DeviceEtherLighting": {
		"behavior":   "breath|steady",
		"brightness": "[1-9]|[1-9][0-9]|100",
		"led_mode":   "standard|etherlighting",
		"mode":       "speed|network",
	},
	"DeviceEthernetOverrides": {
		"ifname":       "eth[0-9]{1,2}",
		"networkgroup": "LAN[2-8]?|WAN[2-9]?|MGMT",
	},
	"DeviceHdmiPorts": {
		"state": "CLIENT_STATE_SUSPENDING|WAITING_HOST_MODE|OPERATING",
		"type":  "in|out",
	},
	"DeviceIPV6": {
		"ip":      "^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$|^$",
		"netmask": "^(0|[1-9]|[1-8][0-9]|9[0-9]|1[01][0-9]|12[0-8])$|^$",
		"type":    "slaac|dhcp|static|none",
	},
	"DeviceIPv4": {
		"ip":      "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"netmask": "^(0|[1-9]|1[0-9]|2[0-9]|3[0-2])$|^$",
		"type":    "dhcp|static",
	},
	"DeviceMbbOverrides": {
		"primary_slot": "1|2",
	},
	"DeviceOutletOverrides": {
		"name": ".{0,128}",
	},
	"DevicePortOverrides": {
		"aggregate_members":         "[1-9]|[1-4][0-9]|5[0-6]",
		"dot1x_ctrl":                "auto|force_authorized|force_unauthorized|mac_based|multi_host",
		"dot1x_idle_timeout":        "[0-9]|[1-9][0-9]{1,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5]",
		"egress_rate_limit_kbps":    "6[4-9]|[7-9][0-9]|[1-9][0-9]{2,6}",
		"fec_mode":                  "rs-fec|fc-fec|default|disabled",
		"forward":                   "all|native|customize|disabled",
		"link_debounce":             "0|[1-9]00|[1-4][0-9]00|5000",
		"mirror_port_idx":           "[1-9]|[1-4][0-9]|5[0-6]",
		"multicast_router_mode":     "ALL|CUSTOM|NONE",
		"name":                      ".{0,128}",
		"op_mode":                   "switch|mirror|aggregate|routed|routed_aggregate",
		"poe_mode":                  "auto|pasv24|passthrough|off",
		"port_idx":                  "[1-9]|[1-4][0-9]|5[0-6]",
		"port_security_mac_address": "^([0-9A-Fa-f]{2}[:]){5}([0-9A-Fa-f]{2})$",
		"portconf_id":               "[\\d\\w]+",
		"priority_queue1_level":     "[0-9]|[1-9][0-9]|100",
		"priority_queue2_level":     "[0-9]|[1-9][0-9]|100",
		"priority_queue3_level":     "[0-9]|[1-9][0-9]|100",
		"priority_queue4_level":     "[0-9]|[1-9][0-9]|100",
		"setting_preference":        "auto|manual",
		"speed":                     "10|100|1000|2500|5000|10000|20000|25000|40000|50000|100000",
		"stormctrl_bcast_level":     "[0-9]|[1-9][0-9]|100",
		"stormctrl_bcast_rate":      "[0-9]|[1-9][0-9]{1,6}|1[0-3][0-9]{6}|14[0-7][0-9]{5}|148[0-7][0-9]{4}|14880000",
		"stormctrl_mcast_level":     "[0-9]|[1-9][0-9]|100",
		"stormctrl_mcast_rate":      "[0-9]|[1-9][0-9]{1,6}|1[0-3][0-9]{6}|14[0-7][0-9]{5}|148[0-7][0-9]{4}|14880000",
		"stormctrl_type":            "level|rate",
		"stormctrl_ucast_level":     "[0-9]|[1-9][0-9]|100",
		"stormctrl_ucast_rate":      "[0-9]|[1-9][0-9]{1,6}|1[0-3][0-9]{6}|14[0-7][0-9]{5}|148[0-7][0-9]{4}|14880000",
		"stp_edge_state":            "auto|enabled|disabled",
		"tagged_vlan_mgmt":          "auto|block_all|custom",
	},
	"DevicePrecisionTimeProtocolConfig": {
		"clock_mode":                "boundary|sma|transparent",
		"custom_announce_interval":  "^(-[1-4]|[0-4])$",
		"custom_announce_timeout":   "^([2-9]|10)$",
		"custom_delay_req_interval": "^(-[1-7]|[0-4])$",
		"custom_domain":             "^([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])",
		"custom_sync_interval":      "^(-[1-7]|[0-4])$",
		"priority1":                 "^([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])",
		"priority2":                 "^([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])",
		"profile":                   "smpte|ieee1588|aes67|aes_r16|custom",
		"transport_type":            "ipv4|layer2",
	},
	"DeviceQOSMarking": {
		"cos_code":           "[0-7]",
		"dscp_code":          "0|8|16|24|32|40|48|56|10|12|14|18|20|22|26|28|30|34|36|38|44|46",
		"ip_precedence_code": "[0-7]",
		"queue":              "[0-7]",
	},
	"DeviceQOSMatching": {
		"cos_code":           "[0-7]",
		"dscp_code":          "[0-9]|[1-5][0-9]|6[0-3]",
		"dst_port":           "[0-9]|[1-9][0-9]|[1-9][0-9][0-9]|[1-9][0-9][0-9][0-9]|[1-5][0-9][0-9][0-9][0-9]|6[0-4][0-9][0-9][0-9]|65[0-4][0-9][0-9]|655[0-2][0-9]|6553[0-4]|65535",
		"ip_precedence_code": "[0-7]",
		"protocol":           "([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])|ah|ax.25|dccp|ddp|egp|eigrp|encap|esp|etherip|fc|ggp|gre|hip|hmp|icmp|idpr-cmtp|idrp|igmp|igp|ip|ipcomp|ipencap|ipip|ipv6|ipv6-frag|ipv6-icmp|ipv6-nonxt|ipv6-opts|ipv6-route|isis|iso-tp4|l2tp|manet|mobility-header|mpls-in-ip|ospf|pim|pup|rdp|rohc|rspf|rsvp|sctp|shim6|skip|st|tcp|udp|udplite|vmtp|vrrp|wesp|xns-idp|xtp",
		"src_port":           "[0-9]|[1-9][0-9]|[1-9][0-9][0-9]|[1-9][0-9][0-9][0-9]|[1-5][0-9][0-9][0-9][0-9]|6[0-4][0-9][0-9][0-9]|65[0-4][0-9][0-9]|655[0-2][0-9]|6553[0-4]|65535",
	},
	"DeviceQOSProfile": {
		"qos_profile_mode": "custom|unifi_play|aes67_audio|crestron_audio_video|dante_audio|ndi_aes67_audio|ndi_dante_audio|qsys_audio_video|qsys_video_dante_audio|sdvoe_aes67_audio|sdvoe_dante_audio|shure_audio|smpte_st2110",
	},
	"DeviceRadioTable": {
		"antenna_gain":  "^-?([0-9]|[1-9][0-9])",
		"antenna_id":    "-1|[0-9]",
		"channel":       "[0-9]|[1][0-4]|1.5|2.5|3.5|4.5|5.5|6.5|5|16|17|21|25|29|33|34|36|37|38|40|41|42|44|45|46|48|49|52|53|56|57|60|61|64|65|69|73|77|81|85|89|93|97|100|101|104|105|108|109|112|113|117|116|120|121|124|125|128|129|132|133|136|137|140|141|144|145|149|153|157|161|165|169|173|177|181|183|184|185|187|188|189|192|193|196|197|201|205|209|213|217|221|225|229|233|auto",
		"ht":            "20|40|80|160|240|320|1080|2160|4320",
		"maxsta":        "[1-9]|[1-9][0-9]|1[0-9]{2}|200|^$",
		"min_rssi":      "^-(6[7-9]|[7-8][0-9]|90)$",
		"radio":         "ng|na|ad|6e",
		"sens_level":    "^-([5-8][0-9]|90)$",
		"tx_power":      "[\\d]+|auto",
		"tx_power_mode": "auto|medium|high|low|custom|disabled",
	},
	"DeviceRpsOverride": {
		"power_management_mode": "dynamic|static",
	},
	"DeviceRpsPortTable": {
		"name":      ".{0,32}",
		"port_idx":  "[1-8]",
		"port_mode": "auto|force_active|manual|disabled",
	},
	"DeviceSim": {
		"data_soft_limit_display_unit": "MB|GB",
		"data_warning_threshold":       "[0-9]|[1-9][0-9]|100",
		"reset_date":                   "[0-9]|[1-2][0-9]|3[0-1]",
		"reset_policy":                 "day|week|month",
		"slot":                         "1|2",
	},
	"DeviceSmaPortConfig": {
		"clock_source": "gps|external",
		"display":      "ns|m|ft",
	},
	"DeviceVideoInfo": {
		"audio_mode":      "auto|pcm",
		"channel":         "[2-9]|[1-9][0-9]|[1-9][0-9][0-9]|[1-9][0-9][0-9][0-9]",
		"color_format":    "rgb|ycbcr444|ycbcr422",
		"mode":            "unicast|multicast",
		"resolution":      "auto|1080p|1440p|4k",
		"role":            "host|client",
		"tvwall_end_x":    "[2-5]",
		"tvwall_end_y":    "[2-5]",
		"tvwall_layout_x": "[1-4]",
		"tvwall_layout_y": "[1-4]",
		"tvwall_start_x":  "[2-5]",
		"tvwall_start_y":  "[2-5]",
	},
	"DpiApp": {
		"name":              ".{1,128}",
		"qos_rate_max_down": "-1|[2-9]|[1-9][0-9]{1,4}|100000|10[0-1][0-9]{3}|102[0-3][0-9]{2}|102400",
		"qos_rate_max_up":   "-1|[2-9]|[1-9][0-9]{1,4}|100000|10[0-1][0-9]{3}|102[0-3][0-9]{2}|102400",
	},
	"DpiGroup": {
		"dpiapp_ids": "[\\d\\w]+",
		"name":       ".{1,128}",
	},
	"DynamicDNS": {
		"custom_service": "^[^\"' ]+$",
		"host_name":      "^[^\"' ]+$",
		"interface":      "wan[2-9]?",
		"login":          "^[^\"' ]+$",
		"options":        "^[^\"' ]+$",
		"server":         "^[^\"' ]+$|^$",
		"service":        "afraid|changeip|cloudflare|cloudxns|ddnss|dhis|dnsexit|dnsomatic|dnspark|dnspod|dslreports|dtdns|duckdns|duiadns|dyn|dyndns|dynv6|easydns|freemyip|googledomains|loopia|namecheap|noip|nsupdate|ovh|sitelutions|spdyn|strato|tunnelbroker|zoneedit|cloudflare|custom",
		"x_password":     "^[^\"' ]+$",
	},
	"FirewallGroup": {
		"group_type": "address-group|port-group|ipv6-address-group",
		"name":       ".{1,64}",
	},
	"FirewallPolicy": {
		"action":                "ALLOW|BLOCK|REJECT",
		"connection_state_type": "ALL|RESPOND_ONLY",
		"icmp_typename":         "ANY|SPECIFIC|LIST|OBJECT",
		"icmp_v6_typename":      "ANY|SPECIFIC|LIST|OBJECT",
		"index":                 "[1-9][0-9]+",
		"ip_version":            "BOTH|IPV4|IPV6",
		"protocol":              "all|tcp|udp|tcp_udp",
	},
	"FirewallPolicyDestination": {
		"matching_target":      "ANY|DEVICE|IP|NETWORK|CLIENT|MAC|WEB",
		"matching_target_type": "ANY|SPECIFIC|LIST|OBJECT",
		"port_matching_type":   "ANY|SPECIFIC|LIST|OBJECT",
	},
	"FirewallPolicySchedule": {
		"mode":           "ALWAYS|EVERY_DAY|EVERY_WEEK|ONE_TIME_ONLY",
		"repeat_on_days": "mon|tue|wed|thu|fri|sat|sun",
	},
	"FirewallPolicySource": {
		"matching_target":      "ANY|DEVICE|IP|NETWORK|CLIENT|MAC|WEB",
		"matching_target_type": "ANY|SPECIFIC|LIST|OBJECT",
		"port_matching_type":   "ANY|SPECIFIC|LIST|OBJECT",
	},
	"FirewallRule": {
		"action":                "drop|reject|accept",
		"dst_firewallgroup_ids": "[\\d\\w]+",
		"dst_networkconf_id":    "[\\d\\w]+|^$",
		"dst_networkconf_type":  "ADDRv4|NETv4",
		"icmp_typename":         "^$|address-mask-reply|address-mask-request|any|communication-prohibited|destination-unreachable|echo-reply|echo-request|fragmentation-needed|host-precedence-violation|host-prohibited|host-redirect|host-unknown|host-unreachable|ip-header-bad|network-prohibited|network-redirect|network-unknown|network-unreachable|parameter-problem|port-unreachable|precedence-cutoff|protocol-unreachable|redirect|required-option-missing|router-advertisement|router-solicitation|source-quench|source-route-failed|time-exceeded|timestamp-reply|timestamp-request|TOS-host-redirect|TOS-host-unreachable|TOS-network-redirect|TOS-network-unreachable|ttl-zero-during-reassembly|ttl-zero-during-transit",
		"icmpv6_typename":       "^$|address-unreachable|bad-header|beyond-scope|communication-prohibited|destination-unreachable|echo-reply|echo-request|failed-policy|neighbor-advertisement|neighbor-solicitation|no-route|packet-too-big|parameter-problem|port-unreachable|redirect|reject-route|router-advertisement|router-solicitation|time-exceeded|ttl-zero-during-reassembly|ttl-zero-during-transit|unknown-header-type|unknown-option",
		"ipsec":                 "match-ipsec|match-none|^$",
		"name":                  ".{1,128}",
		"protocol":              "^$|all|([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])|tcp_udp|ah|ax.25|dccp|ddp|egp|eigrp|encap|esp|etherip|fc|ggp|gre|hip|hmp|icmp|idpr-cmtp|idrp|igmp|igp|ip|ipcomp|ipencap|ipip|ipv6|ipv6-frag|ipv6-icmp|ipv6-nonxt|ipv6-opts|ipv6-route|isis|iso-tp4|l2tp|manet|mobility-header|mpls-in-ip|ospf|pim|pup|rdp|rohc|rspf|rsvp|sctp|shim6|skip|st|tcp|udp|udplite|vmtp|vrrp|wesp|xns-idp|xtp",
		"protocol_v6":           "^$|([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])|ah|all|dccp|eigrp|esp|gre|icmpv6|ipcomp|ipv6|ipv6-frag|ipv6-icmp|ipv6-nonxt|ipv6-opts|ipv6-route|isis|l2tp|manet|mobility-header|mpls-in-ip|ospf|pim|rsvp|sctp|shim6|tcp|tcp_udp|udp|vrrp",
		"rule_index":            "2[0-9]{3,4}|4[0-9]{3,4}",
		"ruleset":               "WAN_IN|WAN_OUT|WAN_LOCAL|LAN_IN|LAN_OUT|LAN_LOCAL|GUEST_IN|GUEST_OUT|GUEST_LOCAL|WANv6_IN|WANv6_OUT|WANv6_LOCAL|LANv6_IN|LANv6_OUT|LANv6_LOCAL|GUESTv6_IN|GUESTv6_OUT|GUESTv6_LOCAL",
		"setting_preference":    "auto|manual",
		"src_firewallgroup_ids": "[\\d\\w]+",
		"src_mac_address":       "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$|^$",
		"src_networkconf_id":    "[\\d\\w]+|^$",
		"src_networkconf_type":  "ADDRv4|NETv4",
	},
	"Hotspot2Conf": {
		"anqp_domain_id":           "^0|[1-9][0-9]{0,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5]|$",
		"deauth_req_timeout":       "[1-9][0-9]|[1-9][0-9][0-9]|[1-2][0-9][0-9][0-9]|3[0-5][0-9][0-9]|3600",
		"domain_name_list":         ".{1,128}",
		"hessid":                   "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$|^$",
		"ipaddr_type_avail_v4":     "0|1|2|3|4|5|6|7",
		"ipaddr_type_avail_v6":     "0|1|2",
		"metrics_info_link_status": "up|down|test",
		"name":                     ".{1,128}",
		"network_auth_type":        "-1|0|1|2|3",
		"network_type":             "0|1|2|3|4|5|14|15",
		"t_c_filename":             ".{1,256}",
		"venue_group":              "0|1|2|3|4|5|6|7|8|9|10|11",
		"venue_type":               "0|1|2|3|4|5|6|7|8|9|10|11|12|13|14|15",
	},
	"Hotspot2ConfCapab": {
		"port":     "^(0|[1-9][0-9]{0,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])|$",
		"protocol": "icmp|tcp_udp|tcp|udp|esp",
		"status":   "closed|open|unknown",
	},
	"Hotspot2ConfCellularNetworkList": {
		"name": ".{1,128}",
	},
	"Hotspot2ConfDescription": {
		"language": "[a-z]{3}",
		"text":     ".{1,128}",
	},
	"Hotspot2ConfFriendlyName": {
		"language": "[a-z]{3}",
		"text":     ".{1,128}",
	},
	"Hotspot2ConfIcon": {
		"name": ".{1,128}",
	},
	"Hotspot2ConfIcons": {
		"filename": ".{1,256}",
		"language": "[a-z]{3}",
		"media":    ".{1,256}",
		"name":     ".{1,256}",
	},
	"Hotspot2ConfNaiRealmList": {
		"eap_method": "13|21|18|23|50",
		"encoding":   "0|1",
		"name":       ".{1,128}",
	},
	"Hotspot2ConfOsu": {
		"operating_class": "[0-9A-Fa-f]{12}",
	},
	"Hotspot2ConfQOSMapExceptions": {
		"up": "[0-7]",
	},
	"Hotspot2ConfRoamingConsortiumList": {
		"name": ".{1,128}",
		"oid":  ".{1,128}",
	},
	"Hotspot2ConfVenueName": {
		"language": "[a-z]{3}",
	},
	"HotspotOp": {
		"name":       ".{1,256}",
		"x_password": ".{1,256}",
	},
	"HotspotPackage": {
		"currency": "[A-Z]{3}",
	},
	"Nat": {
		"ip_version":         "IPV4|IPV6",
		"port":               "[1-9][0-9]{0,4}",
		"protocol":           "all|tcp|udp|tcp_udp",
		"setting_preference": "auto|manual",
		"type":               "DNAT|SNAT|MASQUERADE",
	},
	"NatDestinationFilter": {
		"filter_type": "NONE|ADDRESS_AND_PORT|FIREWALL_GROUPS|NETWORK_CONF",
		"port":        "[1-9][0-9]{0,4}",
	},
	"NatSourceFilter": {
		"filter_type": "NONE|ADDRESS_AND_PORT|FIREWALL_GROUPS|NETWORK_CONF",
		"port":        "[1-9][0-9]{0,4}",
	},
	"Network": {
		"dhcp_relay_servers":                  "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_boot_filename":                 ".{1,256}",
		"dhcpd_boot_server":                   "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$|(?=^.{3,253}$)(^((?!-)[a-zA-Z0-9-]{1,63}(?<!-)\\.)+[a-zA-Z]{2,63}$)|[a-zA-Z0-9-]{1,63}|^$",
		"dhcpd_dns_3":                         "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_dns_4":                         "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_gateway":                       "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_ip_1":                          "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_ip_2":                          "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_ip_3":                          "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_mac_1":                         "(^$|^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$)",
		"dhcpd_mac_2":                         "(^$|^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$)",
		"dhcpd_mac_3":                         "(^$|^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$)",
		"dhcpd_ntp_1":                         "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_ntp_2":                         "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_start":                         "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_stop":                          "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_time_offset":                   "^0$|^-?([1-9]([0-9]{1,3})?|[1-7][0-9]{4}|[8][0-5][0-9]{3}|86[0-3][0-9]{2}|86400)$",
		"dhcpd_unifi_controller":              "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_wins_1":                        "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dhcpd_wins_2":                        "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"domain_name":                         "(?=^.{3,253}$)(^((?!-)[a-zA-Z0-9-]{1,63}(?<!-)\\.)+[a-zA-Z]{2,63}$)|^$|[a-zA-Z0-9-]{1,63}",
		"dpigroup_id":                         "[\\d\\w]+|^$",
		"gateway_device":                      "(^$|^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$)",
		"gateway_type":                        "default|switch",
		"igmp_groupmembership":                "[2-9]|[1-9][0-9]{1,2}|[1-2][0-9]{3}|3[0-5][0-9]{2}|3600|^$",
		"igmp_maxresponse":                    "[1-9]|1[0-9]|2[0-5]|^$",
		"igmp_mcrtrexpiretime":                "[0-9]|[1-9][0-9]{1,2}|[1-2][0-9]{3}|3[0-5][0-9]{2}|3600|^$",
		"igmp_proxy_for":                      "all|some|none",
		"interface_mtu":                       "^(6[89]|[7-9][0-9]|[1-9][0-9]{2,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-6])$",
		"ip_aliases":                          "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([8-9]|[1-2][0-9]|3[0-2])$|^$",
		"ip_subnet":                           "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([1-9]|[1-2][0-9]|3[0-2])$",
		"ipsec_dh_group":                      "2|5|14|15|16|19|20|21|25|26",
		"ipsec_encryption":                    "aes128|aes192|aes256|3des",
		"ipsec_esp_dh_group":                  "1|2|5|14|15|16|17|18|19|20|21|22|23|24|25|26|27|28|29|30|31|32",
		"ipsec_esp_encryption":                "aes128|aes192|aes256|3des",
		"ipsec_esp_hash":                      "sha1|md5|sha256|sha384|sha512",
		"ipsec_esp_lifetime":                  "^(?:3[0-9]|[4-9][0-9]|[1-9][0-9]{2,3}|[1-7][0-9]{4}|8[0-5][0-9]{3}|86[0-3][0-9]{2}|86400)$",
		"ipsec_hash":                          "sha1|md5|sha256|sha384|sha512",
		"ipsec_ike_dh_group":                  "1|2|5|14|15|16|17|18|19|20|21|22|23|24|25|26|27|28|29|30|31|32",
		"ipsec_ike_encryption":                "aes128|aes192|aes256|3des",
		"ipsec_ike_hash":                      "sha1|md5|sha256|sha384|sha512",
		"ipsec_ike_lifetime":                  "^(?:3[0-9]|[4-9][0-9]|[1-9][0-9]{2,3}|[1-7][0-9]{4}|8[0-5][0-9]{3}|86[0-3][0-9]{2}|86400)$",
		"ipsec_interface":                     "wan[2-9]?",
		"ipsec_key_exchange":                  "ikev1|ikev2",
		"ipsec_local_ip":                      "^any$|^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"ipsec_profile":                       "customized|azure_dynamic|azure_static",
		"ipsec_tunnel_ip":                     "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([1-9]|[1-2][0-9]|3[0-2])$",
		"ipv6_client_address_assignment":      "slaac|dhcpv6",
		"ipv6_interface_type":                 "static|pd|single_network|none",
		"ipv6_pd_interface":                   "wan[2-9]?",
		"ipv6_pd_prefixid":                    "^$|[a-fA-F0-9]{1,4}",
		"ipv6_ra_preferred_lifetime":          "^([0-9]|[1-8][0-9]|9[0-9]|[1-8][0-9]{2}|9[0-8][0-9]|99[0-9]|[1-8][0-9]{3}|9[0-8][0-9]{2}|99[0-8][0-9]|999[0-9]|[1-8][0-9]{4}|9[0-8][0-9]{3}|99[0-8][0-9]{2}|999[0-8][0-9]|9999[0-9]|[1-8][0-9]{5}|9[0-8][0-9]{4}|99[0-8][0-9]{3}|999[0-8][0-9]{2}|9999[0-8][0-9]|99999[0-9]|[1-8][0-9]{6}|9[0-8][0-9]{5}|99[0-8][0-9]{4}|999[0-8][0-9]{3}|9999[0-8][0-9]{2}|99999[0-8][0-9]|999999[0-9]|[12][0-9]{7}|30[0-9]{6}|31[0-4][0-9]{5}|315[0-2][0-9]{4}|3153[0-5][0-9]{3}|31536000)$|^$",
		"ipv6_ra_priority":                    "high|medium|low",
		"ipv6_ra_valid_lifetime":              "^([0-9]|[1-8][0-9]|9[0-9]|[1-8][0-9]{2}|9[0-8][0-9]|99[0-9]|[1-8][0-9]{3}|9[0-8][0-9]{2}|99[0-8][0-9]|999[0-9]|[1-8][0-9]{4}|9[0-8][0-9]{3}|99[0-8][0-9]{2}|999[0-8][0-9]|9999[0-9]|[1-8][0-9]{5}|9[0-8][0-9]{4}|99[0-8][0-9]{3}|999[0-8][0-9]{2}|9999[0-8][0-9]|99999[0-9]|[1-8][0-9]{6}|9[0-8][0-9]{5}|99[0-8][0-9]{4}|999[0-8][0-9]{3}|9999[0-8][0-9]{2}|99999[0-8][0-9]|999999[0-9]|[12][0-9]{7}|30[0-9]{6}|31[0-4][0-9]{5}|315[0-2][0-9]{4}|3153[0-5][0-9]{3}|31536000)$|^$",
		"ipv6_setting_preference":             "auto|manual",
		"ipv6_wan_delegation_type":            "pd|single_network|none",
		"l2tp_interface":                      "wan[2-9]?",
		"l2tp_local_wan_ip":                   "^any$|^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"l3_interface_type":                   "vlan|port|lag",
		"local_port":                          "^([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])$",
		"mac_override":                        "(^$|^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$)",
		"mss_clamp":                           "auto|custom|disabled",
		"mss_clamp_ipv6":                      "auto|custom|disabled",
		"mss_clamp_mss":                       "^(50[0-9]|5[1-9][0-9]|[6-9][0-9]{2}|[1-7][0-9]{3}|8[0-8][0-9]{2}|89[0-5][0-9]|8960)$",
		"mss_clamp_mss_ipv6":                  "^(50[0-9]|5[1-9][0-9]|[6-9][0-9]{2}|[1-7][0-9]{3}|8[0-8][0-9]{2}|89[0-5][0-9]|8960)$",
		"name":                                ".{1,128}",
		"networkgroup":                        "LAN[2-8]?",
		"openvpn_encryption_cipher":           "AES_256_CBC|BF_CBC",
		"openvpn_interface":                   "wan[2-9]?",
		"openvpn_local_address":               "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"openvpn_local_port":                  "^([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])$",
		"openvpn_local_wan_ip":                "^any$|^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"openvpn_mode":                        "site-to-site|client|server",
		"openvpn_remote_address":              "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"openvpn_remote_host":                 "[^\\\"\\' ]+|^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"openvpn_remote_port":                 "^([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])$",
		"pptpc_route_distance":                "^[1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]$|^$",
		"pptpc_server_ip":                     "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|(?=^.{3,253}$)(^((?!-)[a-zA-Z0-9-]{1,63}(?<!-)\\.)+[a-zA-Z]{2,63}$)|^[a-zA-Z0-9-]{1,63}$",
		"pptpc_username":                      "[^\\\"\\' ]+",
		"priority":                            "[1-4]",
		"purpose":                             "corporate|guest|remote-user-vpn|site-vpn|vlan-only|vpn-client|wan",
		"remote_site_subnets":                 "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([1-9]|[1-2][0-9]|30)$|^$",
		"remote_vpn_subnets":                  "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([1-9]|[1-2][0-9]|3[0-2])$|^$",
		"route_distance":                      "^[1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]$|^$",
		"routed_lag_idx":                      "([0-9]|[1-9][0-9])",
		"routed_port_idx":                     "([0-9]|[1-9][0-9])",
		"setting_preference":                  "auto|manual",
		"uid_public_gateway_port":             "^([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])$",
		"uid_vpn_custom_routing":              "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([1-9]|[1-2][0-9]|3[0-2])$",
		"uid_vpn_max_connection_time_seconds": "^[1-9][0-9]*$",
		"uid_vpn_type":                        "openvpn|wireguard",
		"vlan":                                "[2-9]|[1-9][0-9]{1,2}|[1-3][0-9]{3}|400[0-9]|401[0-8]|^$",
		"vpn_binding_mode":                    "static|interface|any",
		"vpn_protocol":                        "TCP|UDP",
		"vpn_type":                            "auto|ipsec-vpn|openvpn-client|openvpn-server|openvpn-vpn|pptp-client|l2tp-server|pptp-server|sdwan-hub-spoke-tunnel|sdwan-mesh-tunnel|uid-server|wireguard-server|wireguard-client",
		"vrrp_ip_subnet_gw1":                  "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([1-9]|[1-2][0-9]|30)$",
		"vrrp_ip_subnet_gw2":                  "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([1-9]|[1-2][0-9]|30)$",
		"vrrp_vrid":                           "[1-9]|[1-9][0-9]",
		"wan_dhcp_cos":                        "[0-7]|^$",
		"wan_dhcpv6_cos":                      "[0-7]|^$",
		"wan_dhcpv6_pd_size":                  "^(4[89]|5[0-9]|6[0-4])$|^$",
		"wan_dns1":                            "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"wan_dns2":                            "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"wan_dns3":                            "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"wan_dns4":                            "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"wan_dns_preference":                  "auto|manual",
		"wan_egress_qos":                      "[1-7]|^$",
		"wan_failover_priority":               "[1-9]",
		"wan_gateway":                         "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"wan_gateway_v6":                      "^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$|^$",
		"wan_ip":                              "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"wan_ip_aliases":                      "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([8-9]|[1-2][0-9]|3[0-2])$|^$",
		"wan_ipv6":                            "^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$|^$",
		"wan_ipv6_dns1":                       "^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$|^$",
		"wan_ipv6_dns2":                       "^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$|^$",
		"wan_ipv6_dns_preference":             "auto|manual",
		"wan_load_balance_type":               "failover-only|weighted",
		"wan_load_balance_weight":             "^$|[1-9]|[1-9][0-9]",
		"wan_netmask":                         "^((128|192|224|240|248|252|254)\\.0\\.0\\.0)|(255\\.(((0|128|192|224|240|248|252|254)\\.0\\.0)|(255\\.(((0|128|192|224|240|248|252|254)\\.0)|255\\.(0|128|192|224|240|248|252|254)))))$",
		"wan_networkgroup":                    "WAN[2-9]?|WAN_LTE_FAILOVER",
		"wan_prefixlen":                       "^([1-9]|[1-8][0-9]|9[0-9]|1[01][0-9]|12[0-8])$|^$",
		"wan_smartq_down_rate":                "[0-9]{1,9}|1000000000",
		"wan_smartq_up_rate":                  "[0-9]{1,9}|1000000000",
		"wan_type":                            "disabled|dhcp|static|pppoe|dslite|map-e,hubspoke|map-e,jpix|map-e,ntt|dslite-over-pppoe",
		"wan_type_v6":                         "disabled|slaac|dhcpv6|static",
		"wan_username":                        "[^\"' ]+|^$",
		"wan_vlan":                            "[0-9]|[1-9][0-9]{1,2}|[1-3][0-9]{3}|40[0-8][0-9]|409[0-4]|^$",
		"wireguard_client_mode":               "file|manual",
		"wireguard_client_peer_port":          "^([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])$",
		"wireguard_interface":                 "wan[2-9]?",
		"wireguard_interface_binding_mode_ip_version": "^(v4|v6)$",
		"x_ipsec_pre_shared_key":                      "[^\\\"\\' ]+",
		"x_openvpn_shared_secret_key":                 "[0-9A-Fa-f]{512}",
		"x_pptpc_password":                            "[^\\\"\\' ]+",
		"x_wan_password":                              "[^\"' ]+|^$",
	},
	"NetworkIGMPQuerierSwitches": {
		"querier_address": "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"switch_mac":      "^([0-9A-Fa-f]{2}[:]){5}([0-9A-Fa-f]{2})$",
	},
	"NetworkNATOutboundIPAddresses": {
		"ip_address":        "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"ip_address_pool":   "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])-(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"mode":              "all|ip_address|ip_address_pool",
		"wan_network_group": "WAN[2-9]?",
	},
	"NetworkWANDHCPOptions": {
		"optionNumber": "([1-9]|[1-8][0-9]|9[0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-4])",
	},
	"NetworkWANDHCPv6Options": {
		"optionNumber": "(1|11|15|16|17)",
	},
	"NetworkWANProviderCapabilities": {
		"download_kilobits_per_second": "^[1-9][0-9]*$",
		"upload_kilobits_per_second":   "^[1-9][0-9]*$",
	},
	"PortForward": {
		"destination_ip":    "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^any$",
		"dst_port":          "(([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])|([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])-([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5]))+(,([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])|,([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])-([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])){0,14}",
		"fwd":               "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"fwd_port":          "(([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])|([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])-([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5]))+(,([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])|,([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])-([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])){0,14}",
		"name":              ".{1,128}",
		"pfwd_interface":    "wan[2-9]?|both|all",
		"proto":             "tcp_udp|tcp|udp",
		"src":               "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])-(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])/([0-9]|[1-2][0-9]|3[0-2])$|^!(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^!(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])-(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^!(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])/([0-9]|[1-2][0-9]|3[0-2])$|^any$",
		"src_limiting_type": "ip|firewall_group",
	},
	"PortForwardDestinationIPs": {
		"destination_ip": "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^any$",
		"interface":      "wan[2-9]?",
	},
	"PortProfile": {
		"dot1x_ctrl":                "auto|force_authorized|force_unauthorized|mac_based|multi_host",
		"dot1x_idle_timeout":        "[0-9]|[1-9][0-9]{1,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5]",
		"egress_rate_limit_kbps":    "6[4-9]|[7-9][0-9]|[1-9][0-9]{2,6}",
		"fec_mode":                  "rs-fec|fc-fec|default|disabled",
		"forward":                   "all|native|customize|disabled",
		"link_debounce":             "0|[1-9]00|[1-4][0-9]00|5000",
		"multicast_router_mode":     "ALL|CUSTOM|NONE",
		"op_mode":                   "switch",
		"poe_mode":                  "auto|off",
		"port_security_mac_address": "^([0-9A-Fa-f]{2}[:]){5}([0-9A-Fa-f]{2})$",
		"priority_queue1_level":     "[0-9]|[1-9][0-9]|100",
		"priority_queue2_level":     "[0-9]|[1-9][0-9]|100",
		"priority_queue3_level":     "[0-9]|[1-9][0-9]|100",
		"priority_queue4_level":     "[0-9]|[1-9][0-9]|100",
		"setting_preference":        "auto|manual",
		"speed":                     "10|100|1000|2500|5000|10000|20000|25000|40000|50000|100000",
		"stormctrl_bcast_level":     "[0-9]|[1-9][0-9]|100",
		"stormctrl_bcast_rate":      "[0-9]|[1-9][0-9]{1,6}|1[0-3][0-9]{6}|14[0-7][0-9]{5}|148[0-7][0-9]{4}|14880000",
		"stormctrl_mcast_level":     "[0-9]|[1-9][0-9]|100",
		"stormctrl_mcast_rate":      "[0-9]|[1-9][0-9]{1,6}|1[0-3][0-9]{6}|14[0-7][0-9]{5}|148[0-7][0-9]{4}|14880000",
		"stormctrl_type":            "level|rate",
		"stormctrl_ucast_level":     "[0-9]|[1-9][0-9]|100",
		"stormctrl_ucast_rate":      "[0-9]|[1-9][0-9]{1,6}|1[0-3][0-9]{6}|14[0-7][0-9]{5}|148[0-7][0-9]{4}|14880000",
		"stp_edge_state":            "auto|enabled|disabled",
		"tagged_vlan_mgmt":          "auto|block_all|custom",
	},
	"PortProfileQOSMarking": {
		"cos_code":           "[0-7]",
		"dscp_code":          "0|8|16|24|32|40|48|56|10|12|14|18|20|22|26|28|30|34|36|38|44|46",
		"ip_precedence_code": "[0-7]",
		"queue":              "[0-7]",
	},
	"PortProfileQOSMatching": {
		"cos_code":           "[0-7]",
		"dscp_code":          "[0-9]|[1-5][0-9]|6[0-3]",
		"dst_port":           "[0-9]|[1-9][0-9]|[1-9][0-9][0-9]|[1-9][0-9][0-9][0-9]|[1-5][0-9][0-9][0-9][0-9]|6[0-4][0-9][0-9][0-9]|65[0-4][0-9][0-9]|655[0-2][0-9]|6553[0-4]|65535",
		"ip_precedence_code": "[0-7]",
		"protocol":           "([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])|ah|ax.25|dccp|ddp|egp|eigrp|encap|esp|etherip|fc|ggp|gre|hip|hmp|icmp|idpr-cmtp|idrp|igmp|igp|ip|ipcomp|ipencap|ipip|ipv6|ipv6-frag|ipv6-icmp|ipv6-nonxt|ipv6-opts|ipv6-route|isis|iso-tp4|l2tp|manet|mobility-header|mpls-in-ip|ospf|pim|pup|rdp|rohc|rspf|rsvp|sctp|shim6|skip|st|tcp|udp|udplite|vmtp|vrrp|wesp|xns-idp|xtp",
		"src_port":           "[0-9]|[1-9][0-9]|[1-9][0-9][0-9]|[1-9][0-9][0-9][0-9]|[1-5][0-9][0-9][0-9][0-9]|6[0-4][0-9][0-9][0-9]|65[0-4][0-9][0-9]|655[0-2][0-9]|6553[0-4]|65535",
	},
	"PortProfileQOSProfile": {
		"qos_profile_mode": "custom|unifi_play|aes67_audio|crestron_audio_video|dante_audio|ndi_aes67_audio|ndi_dante_audio|qsys_audio_video|qsys_video_dante_audio|sdvoe_aes67_audio|sdvoe_dante_audio|shure_audio",
	},
	"RADIUSProfile": {
		"interim_update_interval": "^([6-9][0-9]|[1-9][0-9]{2,3}|[1-7][0-9]{4}|8[0-5][0-9]{3}|86[0-3][0-9][0-9]|86400)$",
		"name":                    ".{1,128}",
		"vlan_wlan_mode":          "disabled|optional|required",
	},
	"RADIUSProfileAcctServers": {
		"ip":   "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"port": "^([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])$|^$",
	},
	"RADIUSProfileAuthServers": {
		"ip":   "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$",
		"port": "^([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])$|^$",
	},
	"Routing": {
		"gateway_device":         "^([0-9A-Fa-f]{2}[:]){5}([0-9A-Fa-f]{2})$",
		"gateway_type":           "default|switch",
		"name":                   ".{1,128}",
		"static-route_distance":  "^[1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]$|^$",
		"static-route_interface": "WAN[1-9]?|[\\d\\w]+|^$",
		"static-route_network":   "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\/([1-9]|[1-2][0-9]|3[0-2])$|^([a-fA-F0-9:]+\\/(([1-9]|[1-8][0-9]|9[0-9]|1[01][0-9]|12[0-8])))$",
		"static-route_nexthop":   "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^([a-fA-F0-9:]+)$|^$",
		"static-route_type":      "nexthop-route|interface-route|blackhole",
		"type":                   "static-route",
	},
	"ScheduleTask": {
		"action": "upgrade",
	},
	"ScheduleTaskUpgradeTargets": {
		"mac": "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
	},
	"SpatialRecord": {
		"name": ".{1,128}",
	},
	"SpatialRecordDevices": {
		"mac": "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
	},
	"SpatialRecordPosition": {
		"x": "(^([-]?[\\d]+)$)|(^([-]?[\\d]+[.]?[\\d]+)$)",
		"y": "(^([-]?[\\d]+)$)|(^([-]?[\\d]+[.]?[\\d]+)$)",
		"z": "(^([-]?[\\d]+)$)|(^([-]?[\\d]+[.]?[\\d]+)$)",
	},
	"TrafficRoute": {
		"description":     ".{0,128}",
		"matching_target": "DOMAIN|IP|INTERNET",
	},
	"TrafficRouteDomains": {
		"domain": ".{1,256}",
		"ports":  "[1-9][0-9]{0,4}",
	},
	"TrafficRouteIPAddresses": {
		"ip_version": "v4|v6",
		"ports":      "[1-9][0-9]{0,4}",
	},
	"TrafficRouteIPRanges": {
		"ip_version": "v4|v6",
	},
	"TrafficRoutePortRanges": {
		"port_start": "[1-9][0-9]{0,4}",
		"port_stop":  "[1-9][0-9]{0,4}",
	},
	"TrafficRouteTargetDevices": {
		"type": "ALL_CLIENTS|CLIENT|NETWORK",
	},
	"WLAN": {
		"ap_group_mode":              "all|groups|devices",
		"bc_filter_list":             "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
		"dns_assistance_mode":        "off|auto|manual",
		"dns_assistance_servers":     "^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$",
		"dpigroup_id":                "[\\d\\w]+|^$",
		"dtim_6e":                    "^([1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dtim_mode":                  "default|custom",
		"dtim_na":                    "^([1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"dtim_ng":                    "^([1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$",
		"group_rekey":                "^(0|[6-9][0-9]|[1-9][0-9]{2,3}|[1-7][0-9]{4}|8[0-5][0-9]{3}|86[0-3][0-9][0-9]|86400)$",
		"mac_filter_list":            "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
		"mac_filter_policy":          "allow|deny",
		"mdns_proxy_mode":            "off|auto|custom",
		"minrate_setting_preference": "auto|manual",
		"name":                       ".{1,32}",
		"name_combine_suffix":        ".{0,8}",
		"nas_identifier":             ".{0,48}",
		"nas_identifier_type":        "ap_name|ap_mac|bssid|site_name|custom",
		"pmf_cipher":                 "auto|aes-128-cmac|bip-gmac-256",
		"pmf_mode":                   "disabled|optional|required",
		"priority":                   "medium|high|low",
		"radius_macacl_format":       "none_lower|hyphen_lower|colon_lower|none_upper|hyphen_upper|colon_upper",
		"roam_cluster_id":            "[0-9]|[1-2][0-9]|[3][0-1]|^$",
		"roaming_assistant_6e_rssi":  "^-([7-8][0-9]|90)$",
		"roaming_assistant_na_rssi":  "^-([6-7][0-9]|80)$",
		"schedule":                   "(sun|mon|tue|wed|thu|fri|sat)(\\-(sun|mon|tue|wed|thu|fri|sat))?\\|([0-2][0-9][0-5][0-9])\\-([0-2][0-9][0-5][0-9])",
		"security":                   "open|wpapsk|wep|wpaeap|osen",
		"setting_preference":         "auto|manual",
		"vlan":                       "[2-9]|[1-9][0-9]{1,2}|[1-3][0-9]{3}|40[0-8][0-9]|409[0-5]|^$",
		"wep_idx":                    "[1-4]",
		"wlan_band":                  "2g|5g|both",
		"wlan_bands":                 "2g|5g|6g",
		"wpa_enc":                    "auto|ccmp|gcmp|ccmp-256|gcmp-256",
		"wpa_mode":                   "auto|wpa1|wpa2",
		"wpa_psk_radius":             "disabled|optional|required",
		"x_iapp_key":                 "[0-9A-Fa-f]{32}",
		"x_passphrase":               "[\\x20-\\x7E]{8,255}|[0-9a-fA-F]{64}",
	},
	"WLANCapab": {
		"port":     "^(0|[1-9][0-9]{0,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])|$",
		"protocol": "icmp|tcp_udp|tcp|udp|esp",
		"status":   "closed|open|unknown",
	},
	"WLANCellularNetworkList": {
		"country_code": "[1-9]{1}[0-9]{0,3}",
		"name":         ".{1,128}",
	},
	"WLANCustomServices": {
		"address": "^_[a-zA-Z0-9._-]+\\._(tcp|udp)(\\.local)?$",
	},
	"WLANFriendlyName": {
		"language": "[a-z]{3}",
		"text":     ".{1,128}",
	},
	"WLANGroup": {
		"name": ".{1,128}",
	},
	"WLANHotspot2": {
		"domain_name_list":         ".{1,128}",
		"ipaddr_type_avail_v4":     "0|1|2|3|4|5|6|7",
		"ipaddr_type_avail_v6":     "0|1|2",
		"metrics_info_link_status": "up|down|test",
		"network_type":             "0|1|2|3|4|5|14|15",
		"venue_group":              "0|1|2|3|4|5|6|7|8|9|10|11",
		"venue_type":               "0|1|2|3|4|5|6|7|8|9|10|11|12|13|14|15",
	},
	"WLANMdnsProxyCustom": {
		"ap_macs":       "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
		"ap_scope_mode": "all|specific|group",
		"services_mode": "all|specific|none",
	},
	"WLANNaiRealmList": {
		"auth_ids":   "0|1|2|3|4|5",
		"auth_vals":  "0|1|2|3|4|5|6|7|8|9|10",
		"eap_method": "13|21|18|23|50",
		"encoding":   "0|1",
		"name":       ".{1,128}",
	},
	"WLANPredefinedServices": {
		"code": "amazon_devices|android_tv_remote|apple_airDrop|apple_airPlay|apple_file_sharing|apple_iChat|apple_iTunes|aqara|bose|dns_service_discovery|ftp_servers|google_chromecast|homeKit|matter_network|philips_hue|printers|roku|scanners|sonos|spotify_connect|ssh_servers|time_capsule|web_servers|windows_file_sharing_samba",
	},
	"WLANPrivatePresharedKeys": {
		"password": "[\\x20-\\x7E]{8,255}",
	},
	"WLANRoamingConsortiumList": {
		"name": ".{1,128}",
		"oid":  ".{1,128}",
	},
	"WLANSaePsk": {
		"id":   ".{0,128}",
		"mac":  "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
		"psk":  "[\\x20-\\x7E]{8,255}",
		"vlan": "[0-9]|[1-9][0-9]{1,2}|[1-3][0-9]{3}|40[0-8][0-9]|409[0-5]|^$",
	},
	"WLANScheduleWithDuration": {
		"duration_minutes":   "^[1-9][0-9]*$",
		"name":               ".*",
		"start_days_of_week": "^(sun|mon|tue|wed|thu|fri|sat)$",
		"start_hour":         "^(1?[0-9])|(2[0-3])$",
		"start_minute":       "^[0-5]?[0-9]$",
	},
	"WLANVenueName": {
		"language": "[a-z]{0,3}",
	},
}

// ChannelPlanRadioTableTxPowerModeValues are the values the controller accepts for ChannelPlanRadioTable.tx_power_mode.
var ChannelPlanRadioTableTxPowerModeValues = []string{"auto", "medium", "high", "low", "custom"}

// ChannelPlanRadioTableWidthValues are the values the controller accepts for ChannelPlanRadioTable.width.
var ChannelPlanRadioTableWidthValues = []int64{20, 40, 80, 160}

// DHCPOptionTypeValues are the values the controller accepts for DHCPOption.type.
var DHCPOptionTypeValues = []string{"boolean", "hexarray", "integer", "ipaddress", "macaddress", "text"}

// DHCPOptionWidthValues are the values the controller accepts for DHCPOption.width.
var DHCPOptionWidthValues = []int64{8, 16, 32}

// DNSRecordRecordTypeValues are the values the controller accepts for DNSRecord.record_type.
var DNSRecordRecordTypeValues = []string{"A", "AAAA", "CNAME", "MX", "NS", "PTR", "SOA", "SRV", "TXT"}

// DeviceBandsteeringModeValues are the values the controller accepts for Device.bandsteering_mode.
var DeviceBandsteeringModeValues = []string{"off", "equal", "prefer_5g"}

// DeviceEavBridgeRoleValues are the values the controller accepts for Device.eav_bridge_role.
var DeviceEavBridgeRoleValues = []string{"host", "client"}

// DeviceFanModeOverrideValues are the values the controller accepts for Device.fan_mode_override.
var DeviceFanModeOverrideValues = []string{"default", "quiet"}

// DeviceGatewayVrrpModeValues are the values the controller accepts for Device.gateway_vrrp_mode.
var DeviceGatewayVrrpModeValues = []string{"primary", "secondary"}

// DeviceLcmOrientationOverrideValues are the values the controller accepts for Device.lcm_orientation_override.
var DeviceLcmOrientationOverrideValues = []int64{0, 90, 180, 270}

// DeviceLedOverrideValues are the values the controller accepts for Device.led_override.
var DeviceLedOverrideValues = []string{"default", "on", "off"}

// DeviceLteAuthTypeValues are the values the controller accepts for Device.lte_auth_type.
var DeviceLteAuthTypeValues = []string{"PAP", "CHAP", "PAP-CHAP", "NONE"}

// DeviceOutdoorModeOverrideValues are the values the controller accepts for Device.outdoor_mode_override.
var DeviceOutdoorModeOverrideValues = []string{"default", "on", "off"}

// DevicePeerToPeerModeValues are the values the controller accepts for Device.peer_to_peer_mode.
var DevicePeerToPeerModeValues = []string{"ap", "sta"}

// DevicePoeModeValues are the values the controller accepts for Device.poe_mode.
var DevicePoeModeValues = []string{"auto", "pasv24", "passthrough", "off"}

// DevicePowerSourceCtrlValues are the values the controller accepts for Device.power_source_ctrl.
var DevicePowerSourceCtrlValues = []string{"auto", "8023af", "8023at", "8023bt-type3", "8023bt-type4", "pasv24", "poe-injector", "ac", "adapter", "dc", "rps"}

// DeviceResetbtnEnabledValues are the values the controller accepts for Device.resetbtn_enabled.
var DeviceResetbtnEnabledValues = []string{"on", "off"}

// DeviceStationModeValues are the values the controller accepts for Device.station_mode.
var DeviceStationModeValues = []string{"ptp", "ptmp", "wifi"}

// DeviceStpPriorityValues are the values the controller accepts for Device.stp_priority.
var DeviceStpPriorityValues = []int64{0, 4096, 8192, 12288, 16384, 20480, 24576, 28672, 32768, 36864, 40960, 45056, 49152, 53248, 57344, 61440}

// DeviceStpVersionValues are the values the controller accepts for Device.stp_version.
var DeviceStpVersionValues = []string{"stp", "rstp", "disabled"}

// DeviceAudioInfoRoleValues are the values the controller accepts for DeviceAudioInfo.role.
var DeviceAudioInfoRoleValues = []string{"host", "client"}

// DeviceConfigNetworkTypeValues are the values the controller accepts for DeviceConfigNetwork.type.
var DeviceConfigNetworkTypeValues = []string{"dhcp", "static"}

// DeviceCurrentApnAuthTypeValues are the values the controller accepts for DeviceCurrentApn.auth_type.
var DeviceCurrentApnAuthTypeValues = []string{"PAP", "CHAP", "PAP-CHAP", "NONE"}

// DeviceCurrentApnPDpTypeValues are the values the controller accepts for DeviceCurrentApn.pdp_type.
var DeviceCurrentApnPDpTypeValues = []string{"IPv4", "IPv6", "IPv4v6"}

// DeviceEtherLightingBehaviorValues are the values the controller accepts for DeviceEtherLighting.behavior.
var DeviceEtherLightingBehaviorValues = []string{"breath", "steady"}

// DeviceEtherLightingLedModeValues are the values the controller accepts for DeviceEtherLighting.led_mode.
var DeviceEtherLightingLedModeValues = []string{"standard", "etherlighting"}

// DeviceEtherLightingModeValues are the values the controller accepts for DeviceEtherLighting.mode.
var DeviceEtherLightingModeValues = []string{"speed", "network"}

// DeviceHdmiPortsStateValues are the values the controller accepts for DeviceHdmiPorts.state.
var DeviceHdmiPortsStateValues = []string{"CLIENT_STATE_SUSPENDING", "WAITING_HOST_MODE", "OPERATING"}

// DeviceHdmiPortsTypeValues are the values the controller accepts for DeviceHdmiPorts.type.
var DeviceHdmiPortsTypeValues = []string{"in", "out"}

// DeviceIPV6TypeValues are the values the controller accepts for DeviceIPV6.type.
var DeviceIPV6TypeValues = []string{"slaac", "dhcp", "static", "none"}

// DeviceIPv4TypeValues are the values the controller accepts for DeviceIPv4.type.
var DeviceIPv4TypeValues = []string{"dhcp", "static"}

// DeviceMbbOverridesPrimarySlotValues are the values the controller accepts for DeviceMbbOverrides.primary_slot.
var DeviceMbbOverridesPrimarySlotValues = []int64{1, 2}

// DevicePortOverridesDot1XCtrlValues are the values the controller accepts for DevicePortOverrides.dot1x_ctrl.
var DevicePortOverridesDot1XCtrlValues = []string{"auto", "force_authorized", "force_unauthorized", "mac_based", "multi_host"}

// DevicePortOverridesFecModeValues are the values the controller accepts for DevicePortOverrides.fec_mode.
var DevicePortOverridesFecModeValues = []string{"rs-fec", "fc-fec", "default", "disabled"}

// DevicePortOverridesForwardValues are the values the controller accepts for DevicePortOverrides.forward.
var DevicePortOverridesForwardValues = []string{"all", "native", "customize", "disabled"}

// DevicePortOverridesMulticastRouterModeValues are the values the controller accepts for DevicePortOverrides.multicast_router_mode.
var DevicePortOverridesMulticastRouterModeValues = []string{"ALL", "CUSTOM", "NONE"}

// DevicePortOverridesOpModeValues are the values the controller accepts for DevicePortOverrides.op_mode.
var DevicePortOverridesOpModeValues = []string{"switch", "mirror", "aggregate", "routed", "routed_aggregate"}

// DevicePortOverridesPoeModeValues are the values the controller accepts for DevicePortOverrides.poe_mode.
var DevicePortOverridesPoeModeValues = []string{"auto", "pasv24", "passthrough", "off"}

// DevicePortOverridesSettingPreferenceValues are the values the controller accepts for DevicePortOverrides.setting_preference.
var DevicePortOverridesSettingPreferenceValues = []string{"auto", "manual"}

// DevicePortOverridesSpeedValues are the values the controller accepts for DevicePortOverrides.speed.
var DevicePortOverridesSpeedValues = []int64{10, 100, 1000, 2500, 5000, 10000, 20000, 25000, 40000, 50000, 100000}

// DevicePortOverridesStormctrlTypeValues are the values the controller accepts for DevicePortOverrides.stormctrl_type.
var DevicePortOverridesStormctrlTypeValues = []string{"level", "rate"}

// DevicePortOverridesStpEdgeStateValues are the values the controller accepts for DevicePortOverrides.stp_edge_state.
var DevicePortOverridesStpEdgeStateValues = []string{"auto", "enabled", "disabled"}

// DevicePortOverridesTaggedVLANMgmtValues are the values the controller accepts for DevicePortOverrides.tagged_vlan_mgmt.
var DevicePortOverridesTaggedVLANMgmtValues = []string{"auto", "block_all", "custom"}

// DevicePrecisionTimeProtocolConfigClockModeValues are the values the controller accepts for DevicePrecisionTimeProtocolConfig.clock_mode.
var DevicePrecisionTimeProtocolConfigClockModeValues = []string{"boundary", "sma", "transparent"}

// DevicePrecisionTimeProtocolConfigProfileValues are the values the controller accepts for DevicePrecisionTimeProtocolConfig.profile.
var DevicePrecisionTimeProtocolConfigProfileValues = []string{"smpte", "ieee1588", "aes67", "aes_r16", "custom"}

// DevicePrecisionTimeProtocolConfigTransportTypeValues are the values the controller accepts for DevicePrecisionTimeProtocolConfig.transport_type.
var DevicePrecisionTimeProtocolConfigTransportTypeValues = []string{"ipv4", "layer2"}

// DeviceQOSMarkingDscpCodeValues are the values the controller accepts for DeviceQOSMarking.dscp_code.
var DeviceQOSMarkingDscpCodeValues = []int64{0, 8, 16, 24, 32, 40, 48, 56, 10, 12, 14, 18, 20, 22, 26, 28, 30, 34, 36, 38, 44, 46}

// DeviceQOSProfileQOSProfileModeValues are the values the controller accepts for DeviceQOSProfile.qos_profile_mode.
var DeviceQOSProfileQOSProfileModeValues = []string{"custom", "unifi_play", "aes67_audio", "crestron_audio_video", "dante_audio", "ndi_aes67_audio", "ndi_dante_audio", "qsys_audio_video", "qsys_video_dante_audio", "sdvoe_aes67_audio", "sdvoe_dante_audio", "shure_audio", "smpte_st2110"}

// DeviceRadioTableHtValues are the values the controller accepts for DeviceRadioTable.ht.
var DeviceRadioTableHtValues = []int64{20, 40, 80, 160, 240, 320, 1080, 2160, 4320}

// DeviceRadioTableRadioValues are the values the controller accepts for DeviceRadioTable.radio.
var DeviceRadioTableRadioValues = []string{"ng", "na", "ad", "6e"}

// DeviceRadioTableTxPowerModeValues are the values the controller accepts for DeviceRadioTable.tx_power_mode.
var DeviceRadioTableTxPowerModeValues = []string{"auto", "medium", "high", "low", "custom", "disabled"}

// DeviceRpsOverridePowerManagementModeValues are the values the controller accepts for DeviceRpsOverride.power_management_mode.
var DeviceRpsOverridePowerManagementModeValues = []string{"dynamic", "static"}

// DeviceRpsPortTablePortModeValues are the values the controller accepts for DeviceRpsPortTable.port_mode.
var DeviceRpsPortTablePortModeValues = []string{"auto", "force_active", "manual", "disabled"}

// DeviceSimDataSoftLimitDisplayUnitValues are the values the controller accepts for DeviceSim.data_soft_limit_display_unit.
var DeviceSimDataSoftLimitDisplayUnitValues = []string{"MB", "GB"}

// DeviceSimResetPolicyValues are the values the controller accepts for DeviceSim.reset_policy.
var DeviceSimResetPolicyValues = []string{"day", "week", "month"}

// DeviceSimSlotValues are the values the controller accepts for DeviceSim.slot.
var DeviceSimSlotValues = []int64{1, 2}

// DeviceSmaPortConfigClockSourceValues are the values the controller accepts for DeviceSmaPortConfig.clock_source.
var DeviceSmaPortConfigClockSourceValues = []string{"gps", "external"}

// DeviceSmaPortConfigDisplayValues are the values the controller accepts for DeviceSmaPortConfig.display.
var DeviceSmaPortConfigDisplayValues = []string{"ns", "m", "ft"}

// DeviceVideoInfoAudioModeValues are the values the controller accepts for DeviceVideoInfo.audio_mode.
var DeviceVideoInfoAudioModeValues = []string{"auto", "pcm"}

// DeviceVideoInfoColorFormatValues are the values the controller accepts for DeviceVideoInfo.color_format.
var DeviceVideoInfoColorFormatValues = []string{"rgb", "ycbcr444", "ycbcr422"}

// DeviceVideoInfoModeValues are the values the controller accepts for DeviceVideoInfo.mode.
var DeviceVideoInfoModeValues = []string{"unicast", "multicast"}

// DeviceVideoInfoResolutionValues are the values the controller accepts for DeviceVideoInfo.resolution.
var DeviceVideoInfoResolutionValues = []string{"auto", "1080p", "1440p", "4k"}

// DeviceVideoInfoRoleValues are the values the controller accepts for DeviceVideoInfo.role.
var DeviceVideoInfoRoleValues = []string{"host", "client"}

// DynamicDNSServiceValues are the values the controller accepts for DynamicDNS.service.
var DynamicDNSServiceValues = []string{"afraid", "changeip", "cloudflare", "cloudxns", "ddnss", "dhis", "dnsexit", "dnsomatic", "dnspark", "dnspod", "dslreports", "dtdns", "duckdns", "duiadns", "dyn", "dyndns", "dynv6", "easydns", "freemyip", "googledomains", "loopia", "namecheap", "noip", "nsupdate", "ovh", "sitelutions", "spdyn", "strato", "tunnelbroker", "zoneedit", "custom"}

// FirewallGroupGroupTypeValues are the values the controller accepts for FirewallGroup.group_type.
var FirewallGroupGroupTypeValues = []string{"address-group", "port-group", "ipv6-address-group"}

// FirewallPolicyActionValues are the values the controller accepts for FirewallPolicy.action.
var FirewallPolicyActionValues = []string{"ALLOW", "BLOCK", "REJECT"}

// FirewallPolicyConnectionStateTypeValues are the values the controller accepts for FirewallPolicy.connection_state_type.
var FirewallPolicyConnectionStateTypeValues = []string{"ALL", "RESPOND_ONLY"}

// FirewallPolicyICMPTypenameValues are the values the controller accepts for FirewallPolicy.icmp_typename.
var FirewallPolicyICMPTypenameValues = []string{"ANY", "SPECIFIC", "LIST", "OBJECT"}

// FirewallPolicyICMPV6TypenameValues are the values the controller accepts for FirewallPolicy.icmp_v6_typename.
var FirewallPolicyICMPV6TypenameValues = []string{"ANY", "SPECIFIC", "LIST", "OBJECT"}

// FirewallPolicyVersionValues are the values the controller accepts for FirewallPolicy.ip_version.
var FirewallPolicyVersionValues = []string{"BOTH", "IPV4", "IPV6"}

// FirewallPolicyProtocolValues are the values the controller accepts for FirewallPolicy.protocol.
var FirewallPolicyProtocolValues = []string{"all", "tcp", "udp", "tcp_udp"}

// FirewallPolicyDestinationMatchingTargetValues are the values the controller accepts for FirewallPolicyDestination.matching_target.
var FirewallPolicyDestinationMatchingTargetValues = []string{"ANY", "DEVICE", "IP", "NETWORK", "CLIENT", "MAC", "WEB"}

// FirewallPolicyDestinationMatchingTargetTypeValues are the values the controller accepts for FirewallPolicyDestination.matching_target_type.
var FirewallPolicyDestinationMatchingTargetTypeValues = []string{"ANY", "SPECIFIC", "LIST", "OBJECT"}

// FirewallPolicyDestinationPortMatchingTypeValues are the values the controller accepts for FirewallPolicyDestination.port_matching_type.
var FirewallPolicyDestinationPortMatchingTypeValues = []string{"ANY", "SPECIFIC", "LIST", "OBJECT"}

// FirewallPolicyScheduleModeValues are the values the controller accepts for FirewallPolicySchedule.mode.
var FirewallPolicyScheduleModeValues = []string{"ALWAYS", "EVERY_DAY", "EVERY_WEEK", "ONE_TIME_ONLY"}

// FirewallPolicyScheduleRepeatOnDaysValues are the values the controller accepts for FirewallPolicySchedule.repeat_on_days.
var FirewallPolicyScheduleRepeatOnDaysValues = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// FirewallPolicySourceMatchingTargetValues are the values the controller accepts for FirewallPolicySource.matching_target.
var FirewallPolicySourceMatchingTargetValues = []string{"ANY", "DEVICE", "IP", "NETWORK", "CLIENT", "MAC", "WEB"}

// FirewallPolicySourceMatchingTargetTypeValues are the values the controller accepts for FirewallPolicySource.matching_target_type.
var FirewallPolicySourceMatchingTargetTypeValues = []string{"ANY", "SPECIFIC", "LIST", "OBJECT"}

// FirewallPolicySourcePortMatchingTypeValues are the values the controller accepts for FirewallPolicySource.port_matching_type.
var FirewallPolicySourcePortMatchingTypeValues = []string{"ANY", "SPECIFIC", "LIST", "OBJECT"}

// FirewallRuleActionValues are the values the controller accepts for FirewallRule.action.
var FirewallRuleActionValues = []string{"drop", "reject", "accept"}

// FirewallRuleDstNetworkTypeValues are the values the controller accepts for FirewallRule.dst_networkconf_type.
var FirewallRuleDstNetworkTypeValues = []string{"ADDRv4", "NETv4"}

// FirewallRuleRulesetValues are the values the controller accepts for FirewallRule.ruleset.
var FirewallRuleRulesetValues = []string{"WAN_IN", "WAN_OUT", "WAN_LOCAL", "LAN_IN", "LAN_OUT", "LAN_LOCAL", "GUEST_IN", "GUEST_OUT", "GUEST_LOCAL", "WANv6_IN", "WANv6_OUT", "WANv6_LOCAL", "LANv6_IN", "LANv6_OUT", "LANv6_LOCAL", "GUESTv6_IN", "GUESTv6_OUT", "GUESTv6_LOCAL"}

// FirewallRuleSettingPreferenceValues are the values the controller accepts for FirewallRule.setting_preference.
var FirewallRuleSettingPreferenceValues = []string{"auto", "manual"}

// FirewallRuleSrcNetworkTypeValues are the values the controller accepts for FirewallRule.src_networkconf_type.
var FirewallRuleSrcNetworkTypeValues = []string{"ADDRv4", "NETv4"}

// Hotspot2ConfIPaddrTypeAvailV4Values are the values the controller accepts for Hotspot2Conf.ipaddr_type_avail_v4.
var Hotspot2ConfIPaddrTypeAvailV4Values = []int64{0, 1, 2, 3, 4, 5, 6, 7}

// Hotspot2ConfIPaddrTypeAvailV6Values are the values the controller accepts for Hotspot2Conf.ipaddr_type_avail_v6.
var Hotspot2ConfIPaddrTypeAvailV6Values = []int64{0, 1, 2}

// Hotspot2ConfMetricsInfoLinkStatusValues are the values the controller accepts for Hotspot2Conf.metrics_info_link_status.
var Hotspot2ConfMetricsInfoLinkStatusValues = []string{"up", "down", "test"}

// Hotspot2ConfNetworkAuthTypeValues are the values the controller accepts for Hotspot2Conf.network_auth_type.
var Hotspot2ConfNetworkAuthTypeValues = []int64{-1, 0, 1, 2, 3}

// Hotspot2ConfNetworkTypeValues are the values the controller accepts for Hotspot2Conf.network_type.
var Hotspot2ConfNetworkTypeValues = []int64{0, 1, 2, 3, 4, 5, 14, 15}

// Hotspot2ConfVenueGroupValues are the values the controller accepts for Hotspot2Conf.venue_group.
var Hotspot2ConfVenueGroupValues = []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

// Hotspot2ConfVenueTypeValues are the values the controller accepts for Hotspot2Conf.venue_type.
var Hotspot2ConfVenueTypeValues = []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

// Hotspot2ConfCapabProtocolValues are the values the controller accepts for Hotspot2ConfCapab.protocol.
var Hotspot2ConfCapabProtocolValues = []string{"icmp", "tcp_udp", "tcp", "udp", "esp"}

// Hotspot2ConfCapabStatusValues are the values the controller accepts for Hotspot2ConfCapab.status.
var Hotspot2ConfCapabStatusValues = []string{"closed", "open", "unknown"}

// Hotspot2ConfNaiRealmListEapMethodValues are the values the controller accepts for Hotspot2ConfNaiRealmList.eap_method.
var Hotspot2ConfNaiRealmListEapMethodValues = []int64{13, 21, 18, 23, 50}

// Hotspot2ConfNaiRealmListEncodingValues are the values the controller accepts for Hotspot2ConfNaiRealmList.encoding.
var Hotspot2ConfNaiRealmListEncodingValues = []int64{0, 1}

// NatVersionValues are the values the controller accepts for Nat.ip_version.
var NatVersionValues = []string{"IPV4", "IPV6"}

// NatProtocolValues are the values the controller accepts for Nat.protocol.
var NatProtocolValues = []string{"all", "tcp", "udp", "tcp_udp"}

// NatSettingPreferenceValues are the values the controller accepts for Nat.setting_preference.
var NatSettingPreferenceValues = []string{"auto", "manual"}

// NatTypeValues are the values the controller accepts for Nat.type.
var NatTypeValues = []string{"DNAT", "SNAT", "MASQUERADE"}

// NatDestinationFilterFilterTypeValues are the values the controller accepts for NatDestinationFilter.filter_type.
var NatDestinationFilterFilterTypeValues = []string{"NONE", "ADDRESS_AND_PORT", "FIREWALL_GROUPS", "NETWORK_CONF"}

// NatSourceFilterFilterTypeValues are the values the controller accepts for NatSourceFilter.filter_type.
var NatSourceFilterFilterTypeValues = []string{"NONE", "ADDRESS_AND_PORT", "FIREWALL_GROUPS", "NETWORK_CONF"}

// NetworkGatewayTypeValues are the values the controller accepts for Network.gateway_type.
var NetworkGatewayTypeValues = []string{"default", "switch"}

// NetworkIGMPProxyForValues are the values the controller accepts for Network.igmp_proxy_for.
var NetworkIGMPProxyForValues = []string{"all", "some", "none"}

// NetworkIPSecDhGroupValues are the values the controller accepts for Network.ipsec_dh_group.
var NetworkIPSecDhGroupValues = []int64{2, 5, 14, 15, 16, 19, 20, 21, 25, 26}

// NetworkIPSecEncryptionValues are the values the controller accepts for Network.ipsec_encryption.
var NetworkIPSecEncryptionValues = []string{"aes128", "aes192", "aes256", "3des"}

// NetworkIPSecEspDhGroupValues are the values the controller accepts for Network.ipsec_esp_dh_group.
var NetworkIPSecEspDhGroupValues = []int64{1, 2, 5, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

// NetworkIPSecEspEncryptionValues are the values the controller accepts for Network.ipsec_esp_encryption.
var NetworkIPSecEspEncryptionValues = []string{"aes128", "aes192", "aes256", "3des"}

// NetworkIPSecEspHashValues are the values the controller accepts for Network.ipsec_esp_hash.
var NetworkIPSecEspHashValues = []string{"sha1", "md5", "sha256", "sha384", "sha512"}

// NetworkIPSecHashValues are the values the controller accepts for Network.ipsec_hash.
var NetworkIPSecHashValues = []string{"sha1", "md5", "sha256", "sha384", "sha512"}

// NetworkIPSecIkeDhGroupValues are the values the controller accepts for Network.ipsec_ike_dh_group.
var NetworkIPSecIkeDhGroupValues = []int64{1, 2, 5, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

// NetworkIPSecIkeEncryptionValues are the values the controller accepts for Network.ipsec_ike_encryption.
var NetworkIPSecIkeEncryptionValues = []string{"aes128", "aes192", "aes256", "3des"}

// NetworkIPSecIkeHashValues are the values the controller accepts for Network.ipsec_ike_hash.
var NetworkIPSecIkeHashValues = []string{"sha1", "md5", "sha256", "sha384", "sha512"}

// NetworkIPSecKeyExchangeValues are the values the controller accepts for Network.ipsec_key_exchange.
var NetworkIPSecKeyExchangeValues = []string{"ikev1", "ikev2"}

// NetworkIPSecProfileValues are the values the controller accepts for Network.ipsec_profile.
var NetworkIPSecProfileValues = []string{"customized", "azure_dynamic", "azure_static"}

// NetworkIPV6ClientAddressAssignmentValues are the values the controller accepts for Network.ipv6_client_address_assignment.
var NetworkIPV6ClientAddressAssignmentValues = []string{"slaac", "dhcpv6"}

// NetworkIPV6InterfaceTypeValues are the values the controller accepts for Network.ipv6_interface_type.
var NetworkIPV6InterfaceTypeValues = []string{"static", "pd", "single_network", "none"}

// NetworkIPV6RaPriorityValues are the values the controller accepts for Network.ipv6_ra_priority.
var NetworkIPV6RaPriorityValues = []string{"high", "medium", "low"}

// NetworkIPV6SettingPreferenceValues are the values the controller accepts for Network.ipv6_setting_preference.
var NetworkIPV6SettingPreferenceValues = []string{"auto", "manual"}

// NetworkIPV6WANDelegationTypeValues are the values the controller accepts for Network.ipv6_wan_delegation_type.
var NetworkIPV6WANDelegationTypeValues = []string{"pd", "single_network", "none"}

// NetworkL3InterfaceTypeValues are the values the controller accepts for Network.l3_interface_type.
var NetworkL3InterfaceTypeValues = []string{"vlan", "port", "lag"}

// NetworkMssClampValues are the values the controller accepts for Network.mss_clamp.
var NetworkMssClampValues = []string{"auto", "custom", "disabled"}

// NetworkMssClampIPV6Values are the values the controller accepts for Network.mss_clamp_ipv6.
var NetworkMssClampIPV6Values = []string{"auto", "custom", "disabled"}

// NetworkOpenVPNEncryptionCipherValues are the values the controller accepts for Network.openvpn_encryption_cipher.
var NetworkOpenVPNEncryptionCipherValues = []string{"AES_256_CBC", "BF_CBC"}

// NetworkOpenVPNModeValues are the values the controller accepts for Network.openvpn_mode.
var NetworkOpenVPNModeValues = []string{"site-to-site", "client", "server"}

// NetworkPurposeValues are the values the controller accepts for Network.purpose.
var NetworkPurposeValues = []string{"corporate", "guest", "remote-user-vpn", "site-vpn", "vlan-only", "vpn-client", "wan"}

// NetworkSettingPreferenceValues are the values the controller accepts for Network.setting_preference.
var NetworkSettingPreferenceValues = []string{"auto", "manual"}

// NetworkUidVPNTypeValues are the values the controller accepts for Network.uid_vpn_type.
var NetworkUidVPNTypeValues = []string{"openvpn", "wireguard"}

// NetworkVPNBindingModeValues are the values the controller accepts for Network.vpn_binding_mode.
var NetworkVPNBindingModeValues = []string{"static", "interface", "any"}

// NetworkVPNProtocolValues are the values the controller accepts for Network.vpn_protocol.
var NetworkVPNProtocolValues = []string{"TCP", "UDP"}

// NetworkVPNTypeValues are the values the controller accepts for Network.vpn_type.
var NetworkVPNTypeValues = []string{"auto", "ipsec-vpn", "openvpn-client", "openvpn-server", "openvpn-vpn", "pptp-client", "l2tp-server", "pptp-server", "sdwan-hub-spoke-tunnel", "sdwan-mesh-tunnel", "uid-server", "wireguard-server", "wireguard-client"}

// NetworkWANDNSPreferenceValues are the values the controller accepts for Network.wan_dns_preference.
var NetworkWANDNSPreferenceValues = []string{"auto", "manual"}

// NetworkWANIPV6DNSPreferenceValues are the values the controller accepts for Network.wan_ipv6_dns_preference.
var NetworkWANIPV6DNSPreferenceValues = []string{"auto", "manual"}

// NetworkWANLoadBalanceTypeValues are the values the controller accepts for Network.wan_load_balance_type.
var NetworkWANLoadBalanceTypeValues = []string{"failover-only", "weighted"}

// NetworkWANTypeValues are the values the controller accepts for Network.wan_type.
var NetworkWANTypeValues = []string{"disabled", "dhcp", "static", "pppoe", "dslite", "map-e,hubspoke", "map-e,jpix", "map-e,ntt", "dslite-over-pppoe"}

// NetworkWANTypeV6Values are the values the controller accepts for Network.wan_type_v6.
var NetworkWANTypeV6Values = []string{"disabled", "slaac", "dhcpv6", "static"}

// NetworkWireguardClientModeValues are the values the controller accepts for Network.wireguard_client_mode.
var NetworkWireguardClientModeValues = []string{"file", "manual"}

// NetworkWireguardInterfaceBindingModeIPVersionValues are the values the controller accepts for Network.wireguard_interface_binding_mode_ip_version.
var NetworkWireguardInterfaceBindingModeIPVersionValues = []string{"v4", "v6"}

// NetworkNATOutboundIPAddressesModeValues are the values the controller accepts for NetworkNATOutboundIPAddresses.mode.
var NetworkNATOutboundIPAddressesModeValues = []string{"all", "ip_address", "ip_address_pool"}

// PortForwardProtoValues are the values the controller accepts for PortForward.proto.
var PortForwardProtoValues = []string{"tcp_udp", "tcp", "udp"}

// PortForwardSrcLimitingTypeValues are the values the controller accepts for PortForward.src_limiting_type.
var PortForwardSrcLimitingTypeValues = []string{"ip", "firewall_group"}

// PortProfileDot1XCtrlValues are the values the controller accepts for PortProfile.dot1x_ctrl.
var PortProfileDot1XCtrlValues = []string{"auto", "force_authorized", "force_unauthorized", "mac_based", "multi_host"}

// PortProfileFecModeValues are the values the controller accepts for PortProfile.fec_mode.
var PortProfileFecModeValues = []string{"rs-fec", "fc-fec", "default", "disabled"}

// PortProfileForwardValues are the values the controller accepts for PortProfile.forward.
var PortProfileForwardValues = []string{"all", "native", "customize", "disabled"}

// PortProfileMulticastRouterModeValues are the values the controller accepts for PortProfile.multicast_router_mode.
var PortProfileMulticastRouterModeValues = []string{"ALL", "CUSTOM", "NONE"}

// PortProfilePoeModeValues are the values the controller accepts for PortProfile.poe_mode.
var PortProfilePoeModeValues = []string{"auto", "off"}

// PortProfileSettingPreferenceValues are the values the controller accepts for PortProfile.setting_preference.
var PortProfileSettingPreferenceValues = []string{"auto", "manual"}

// PortProfileSpeedValues are the values the controller accepts for PortProfile.speed.
var PortProfileSpeedValues = []int64{10, 100, 1000, 2500, 5000, 10000, 20000, 25000, 40000, 50000, 100000}

// PortProfileStormctrlTypeValues are the values the controller accepts for PortProfile.stormctrl_type.
var PortProfileStormctrlTypeValues = []string{"level", "rate"}

// PortProfileStpEdgeStateValues are the values the controller accepts for PortProfile.stp_edge_state.
var PortProfileStpEdgeStateValues = []string{"auto", "enabled", "disabled"}

// PortProfileTaggedVLANMgmtValues are the values the controller accepts for PortProfile.tagged_vlan_mgmt.
var PortProfileTaggedVLANMgmtValues = []string{"auto", "block_all", "custom"}

// PortProfileQOSMarkingDscpCodeValues are the values the controller accepts for PortProfileQOSMarking.dscp_code.
var PortProfileQOSMarkingDscpCodeValues = []int64{0, 8, 16, 24, 32, 40, 48, 56, 10, 12, 14, 18, 20, 22, 26, 28, 30, 34, 36, 38, 44, 46}

// PortProfileQOSProfileQOSProfileModeValues are the values the controller accepts for PortProfileQOSProfile.qos_profile_mode.
var PortProfileQOSProfileQOSProfileModeValues = []string{"custom", "unifi_play", "aes67_audio", "crestron_audio_video", "dante_audio", "ndi_aes67_audio", "ndi_dante_audio", "qsys_audio_video", "qsys_video_dante_audio", "sdvoe_aes67_audio", "sdvoe_dante_audio", "shure_audio"}

// RADIUSProfileVLANWLANModeValues are the values the controller accepts for RADIUSProfile.vlan_wlan_mode.
var RADIUSProfileVLANWLANModeValues = []string{"disabled", "optional", "required"}

// RoutingGatewayTypeValues are the values the controller accepts for Routing.gateway_type.
var RoutingGatewayTypeValues = []string{"default", "switch"}

// RoutingStaticRouteTypeValues are the values the controller accepts for Routing.static-route_type.
var RoutingStaticRouteTypeValues = []string{"nexthop-route", "interface-route", "blackhole"}

// TrafficRouteMatchingTargetValues are the values the controller accepts for TrafficRoute.matching_target.
var TrafficRouteMatchingTargetValues = []string{"DOMAIN", "IP", "INTERNET"}

// TrafficRouteIPAddressesVersionValues are the values the controller accepts for TrafficRouteIPAddresses.ip_version.
var TrafficRouteIPAddressesVersionValues = []string{"v4", "v6"}

// TrafficRouteIPRangesVersionValues are the values the controller accepts for TrafficRouteIPRanges.ip_version.
var TrafficRouteIPRangesVersionValues = []string{"v4", "v6"}

// TrafficRouteTargetDevicesTypeValues are the values the controller accepts for TrafficRouteTargetDevices.type.
var TrafficRouteTargetDevicesTypeValues = []string{"ALL_CLIENTS", "CLIENT", "NETWORK"}

// WLANApGroupModeValues are the values the controller accepts for WLAN.ap_group_mode.
var WLANApGroupModeValues = []string{"all", "groups", "devices"}

// WLANDNSAssistanceModeValues are the values the controller accepts for WLAN.dns_assistance_mode.
var WLANDNSAssistanceModeValues = []string{"off", "auto", "manual"}

// WLANDTIMModeValues are the values the controller accepts for WLAN.dtim_mode.
var WLANDTIMModeValues = []string{"default", "custom"}

// WLANMACFilterPolicyValues are the values the controller accepts for WLAN.mac_filter_policy.
var WLANMACFilterPolicyValues = []string{"allow", "deny"}

// WLANMdnsProxyModeValues are the values the controller accepts for WLAN.mdns_proxy_mode.
var WLANMdnsProxyModeValues = []string{"off", "auto", "custom"}

// WLANMinrateSettingPreferenceValues are the values the controller accepts for WLAN.minrate_setting_preference.
var WLANMinrateSettingPreferenceValues = []string{"auto", "manual"}

// WLANNasIDentifierTypeValues are the values the controller accepts for WLAN.nas_identifier_type.
var WLANNasIDentifierTypeValues = []string{"ap_name", "ap_mac", "bssid", "site_name", "custom"}

// WLANPMFCipherValues are the values the controller accepts for WLAN.pmf_cipher.
var WLANPMFCipherValues = []string{"auto", "aes-128-cmac", "bip-gmac-256"}

// WLANPMFModeValues are the values the controller accepts for WLAN.pmf_mode.
var WLANPMFModeValues = []string{"disabled", "optional", "required"}

// WLANPriorityValues are the values the controller accepts for WLAN.priority.
var WLANPriorityValues = []string{"medium", "high", "low"}

// WLANRADIUSMACaclFormatValues are the values the controller accepts for WLAN.radius_macacl_format.
var WLANRADIUSMACaclFormatValues = []string{"none_lower", "hyphen_lower", "colon_lower", "none_upper", "hyphen_upper", "colon_upper"}

// WLANSecurityValues are the values the controller accepts for WLAN.security.
var WLANSecurityValues = []string{"open", "wpapsk", "wep", "wpaeap", "osen"}

// WLANSettingPreferenceValues are the values the controller accepts for WLAN.setting_preference.
var WLANSettingPreferenceValues = []string{"auto", "manual"}

// WLANWLANBandValues are the values the controller accepts for WLAN.wlan_band.
var WLANWLANBandValues = []string{"2g", "5g", "both"}

// WLANWLANBandsValues are the values the controller accepts for WLAN.wlan_bands.
var WLANWLANBandsValues = []string{"2g", "5g", "6g"}

// WLANWPAEncValues are the values the controller accepts for WLAN.wpa_enc.
var WLANWPAEncValues = []string{"auto", "ccmp", "gcmp", "ccmp-256", "gcmp-256"}

// WLANWPAModeValues are the values the controller accepts for WLAN.wpa_mode.
var WLANWPAModeValues = []string{"auto", "wpa1", "wpa2"}

// WLANWPAPskRADIUSValues are the values the controller accepts for WLAN.wpa_psk_radius.
var WLANWPAPskRADIUSValues = []string{"disabled", "optional", "required"}

// WLANCapabProtocolValues are the values the controller accepts for WLANCapab.protocol.
var WLANCapabProtocolValues = []string{"icmp", "tcp_udp", "tcp", "udp", "esp"}

// WLANCapabStatusValues are the values the controller accepts for WLANCapab.status.
var WLANCapabStatusValues = []string{"closed", "open", "unknown"}

// WLANHotspot2IPaddrTypeAvailV4Values are the values the controller accepts for WLANHotspot2.ipaddr_type_avail_v4.
var WLANHotspot2IPaddrTypeAvailV4Values = []int64{0, 1, 2, 3, 4, 5, 6, 7}

// WLANHotspot2IPaddrTypeAvailV6Values are the values the controller accepts for WLANHotspot2.ipaddr_type_avail_v6.
var WLANHotspot2IPaddrTypeAvailV6Values = []int64{0, 1, 2}

// WLANHotspot2MetricsInfoLinkStatusValues are the values the controller accepts for WLANHotspot2.metrics_info_link_status.
var WLANHotspot2MetricsInfoLinkStatusValues = []string{"up", "down", "test"}

// WLANHotspot2NetworkTypeValues are the values the controller accepts for WLANHotspot2.network_type.
var WLANHotspot2NetworkTypeValues = []int64{0, 1, 2, 3, 4, 5, 14, 15}

// WLANHotspot2VenueGroupValues are the values the controller accepts for WLANHotspot2.venue_group.
var WLANHotspot2VenueGroupValues = []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

// WLANHotspot2VenueTypeValues are the values the controller accepts for WLANHotspot2.venue_type.
var WLANHotspot2VenueTypeValues = []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

// WLANMdnsProxyCustomApScopeModeValues are the values the controller accepts for WLANMdnsProxyCustom.ap_scope_mode.
var WLANMdnsProxyCustomApScopeModeValues = []string{"all", "specific", "group"}

// WLANMdnsProxyCustomServicesModeValues are the values the controller accepts for WLANMdnsProxyCustom.services_mode.
var WLANMdnsProxyCustomServicesModeValues = []string{"all", "specific", "none"}

// WLANNaiRealmListAuthIDsValues are the values the controller accepts for WLANNaiRealmList.auth_ids.
var WLANNaiRealmListAuthIDsValues = []int64{0, 1, 2, 3, 4, 5}

// WLANNaiRealmListAuthValsValues are the values the controller accepts for WLANNaiRealmList.auth_vals.
var WLANNaiRealmListAuthValsValues = []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

// WLANNaiRealmListEapMethodValues are the values the controller accepts for WLANNaiRealmList.eap_method.
var WLANNaiRealmListEapMethodValues = []int64{13, 21, 18, 23, 50}

// WLANNaiRealmListEncodingValues are the values the controller accepts for WLANNaiRealmList.encoding.
var WLANNaiRealmListEncodingValues = []int64{0, 1}

// WLANPredefinedServicesCodeValues are the values the controller accepts for WLANPredefinedServices.code.
var WLANPredefinedServicesCodeValues = []string{"amazon_devices", "android_tv_remote", "apple_airDrop", "apple_airPlay", "apple_file_sharing", "apple_iChat", "apple_iTunes", "aqara", "bose", "dns_service_discovery", "ftp_servers", "google_chromecast", "homeKit", "matter_network", "philips_hue", "printers", "roku", "scanners", "sonos", "spotify_connect", "ssh_servers", "time_capsule", "web_servers", "windows_file_sharing_samba"}

// WLANScheduleWithDurationStartDaysOfWeekValues are the values the controller accepts for WLANScheduleWithDuration.start_days_of_week.
var WLANScheduleWithDurationStartDaysOfWeekValues = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
