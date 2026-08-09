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
		"mode":           "ALWAYS|EVERY_DAY|EVERY_WEEK|ONE_TIME_ONLY|CUSTOM",
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

// AccountTunnelMediumTypeMin and AccountTunnelMediumTypeMax are the inclusive bounds the controller accepts for Account.tunnel_medium_type.
const (
	AccountTunnelMediumTypeMin int64 = 1
	AccountTunnelMediumTypeMax int64 = 15
)

// AccountTunnelTypeMin and AccountTunnelTypeMax are the inclusive bounds the controller accepts for Account.tunnel_type.
const (
	AccountTunnelTypeMin int64 = 1
	AccountTunnelTypeMax int64 = 13
)

// AccountVLANMin and AccountVLANMax are the inclusive bounds the controller accepts for Account.vlan.
const (
	AccountVLANMin int64 = 2
	AccountVLANMax int64 = 4009
)

// BGPConfigDescriptionMinLength and BGPConfigDescriptionMaxLength are the character-count bounds the controller accepts for BGPConfig.description.
const (
	BGPConfigDescriptionMinLength int64 = 0
	BGPConfigDescriptionMaxLength int64 = 128
)

// BGPConfigUploadedFileNameMinLength and BGPConfigUploadedFileNameMaxLength are the character-count bounds the controller accepts for BGPConfig.uploaded_file_name.
const (
	BGPConfigUploadedFileNameMinLength int64 = 0
	BGPConfigUploadedFileNameMaxLength int64 = 256
)

// ChannelPlanRadioTableTxPowerModeValues are the values the controller accepts for ChannelPlanRadioTable.tx_power_mode.
var ChannelPlanRadioTableTxPowerModeValues = []string{"auto", "medium", "high", "low", "custom"}

// ChannelPlanRadioTableWidthValues are the values the controller accepts for ChannelPlanRadioTable.width.
var ChannelPlanRadioTableWidthValues = []int64{20, 40, 80, 160}

// ClientGroupNameMinLength and ClientGroupNameMaxLength are the character-count bounds the controller accepts for ClientGroup.name.
const (
	ClientGroupNameMinLength int64 = 1
	ClientGroupNameMaxLength int64 = 128
)

// DHCPOptionTypeValues are the values the controller accepts for DHCPOption.type.
var DHCPOptionTypeValues = []string{"boolean", "hexarray", "integer", "ipaddress", "macaddress", "text"}

// DHCPOptionWidthValues are the values the controller accepts for DHCPOption.width.
var DHCPOptionWidthValues = []int64{8, 16, 32}

// DNSRecordKeyMinLength and DNSRecordKeyMaxLength are the character-count bounds the controller accepts for DNSRecord.key.
const (
	DNSRecordKeyMinLength int64 = 1
	DNSRecordKeyMaxLength int64 = 128
)

// DNSRecordPortMin and DNSRecordPortMax are the inclusive bounds the controller accepts for DNSRecord.port.
const (
	DNSRecordPortMin int64 = 1
	DNSRecordPortMax int64 = 99999
)

// DNSRecordRecordTypeValues are the values the controller accepts for DNSRecord.record_type.
var DNSRecordRecordTypeValues = []string{"A", "AAAA", "CNAME", "MX", "NS", "PTR", "SOA", "SRV", "TXT"}

// DNSRecordValueMinLength and DNSRecordValueMaxLength are the character-count bounds the controller accepts for DNSRecord.value.
const (
	DNSRecordValueMinLength int64 = 1
	DNSRecordValueMaxLength int64 = 256
)

// DeviceBandsteeringModeValues are the values the controller accepts for Device.bandsteering_mode.
var DeviceBandsteeringModeValues = []string{"off", "equal", "prefer_5g"}

// DeviceEavBridgeRoleValues are the values the controller accepts for Device.eav_bridge_role.
var DeviceEavBridgeRoleValues = []string{"host", "client"}

// DeviceFanModeOverrideValues are the values the controller accepts for Device.fan_mode_override.
var DeviceFanModeOverrideValues = []string{"default", "quiet"}

// DeviceGatewayVrrpModeValues are the values the controller accepts for Device.gateway_vrrp_mode.
var DeviceGatewayVrrpModeValues = []string{"primary", "secondary"}

// DeviceGatewayVrrpPriorityMin and DeviceGatewayVrrpPriorityMax are the inclusive bounds the controller accepts for Device.gateway_vrrp_priority.
const (
	DeviceGatewayVrrpPriorityMin int64 = 10
	DeviceGatewayVrrpPriorityMax int64 = 999
)

// DeviceHostnameMinLength and DeviceHostnameMaxLength are the character-count bounds the controller accepts for Device.hostname.
const (
	DeviceHostnameMinLength int64 = 1
	DeviceHostnameMaxLength int64 = 128
)

// DeviceLcmBrightnessMin and DeviceLcmBrightnessMax are the inclusive bounds the controller accepts for Device.lcm_brightness.
const (
	DeviceLcmBrightnessMin int64 = 1
	DeviceLcmBrightnessMax int64 = 100
)

// DeviceLcmIDleTimeoutMin and DeviceLcmIDleTimeoutMax are the inclusive bounds the controller accepts for Device.lcm_idle_timeout.
const (
	DeviceLcmIDleTimeoutMin int64 = 10
	DeviceLcmIDleTimeoutMax int64 = 3600
)

// DeviceLcmOrientationOverrideValues are the values the controller accepts for Device.lcm_orientation_override.
var DeviceLcmOrientationOverrideValues = []int64{0, 90, 180, 270}

// DeviceLcmTrackerSeedMinLength and DeviceLcmTrackerSeedMaxLength are the character-count bounds the controller accepts for Device.lcm_tracker_seed.
const (
	DeviceLcmTrackerSeedMinLength int64 = 0
	DeviceLcmTrackerSeedMaxLength int64 = 50
)

// DeviceLedOverrideValues are the values the controller accepts for Device.led_override.
var DeviceLedOverrideValues = []string{"default", "on", "off"}

// DeviceLedOverrideColorBrightnessMin and DeviceLedOverrideColorBrightnessMax are the inclusive bounds the controller accepts for Device.led_override_color_brightness.
const (
	DeviceLedOverrideColorBrightnessMin int64 = 0
	DeviceLedOverrideColorBrightnessMax int64 = 100
)

// DeviceLteApnMinLength and DeviceLteApnMaxLength are the character-count bounds the controller accepts for Device.lte_apn.
const (
	DeviceLteApnMinLength int64 = 1
	DeviceLteApnMaxLength int64 = 128
)

// DeviceLteAuthTypeValues are the values the controller accepts for Device.lte_auth_type.
var DeviceLteAuthTypeValues = []string{"PAP", "CHAP", "PAP-CHAP", "NONE"}

// DeviceNameMinLength and DeviceNameMaxLength are the character-count bounds the controller accepts for Device.name.
const (
	DeviceNameMinLength int64 = 0
	DeviceNameMaxLength int64 = 128
)

// DeviceOutdoorModeOverrideValues are the values the controller accepts for Device.outdoor_mode_override.
var DeviceOutdoorModeOverrideValues = []string{"default", "on", "off"}

// DeviceOutletPowerCycleOnAcRecoverySecondsMin and DeviceOutletPowerCycleOnAcRecoverySecondsMax are the inclusive bounds the controller accepts for Device.outlet_power_cycle_on_ac_recovery_seconds.
const (
	DeviceOutletPowerCycleOnAcRecoverySecondsMin int64 = 60
	DeviceOutletPowerCycleOnAcRecoverySecondsMax int64 = 600
)

// DevicePeerToPeerModeValues are the values the controller accepts for Device.peer_to_peer_mode.
var DevicePeerToPeerModeValues = []string{"ap", "sta"}

// DevicePoeModeValues are the values the controller accepts for Device.poe_mode.
var DevicePoeModeValues = []string{"auto", "pasv24", "passthrough", "off"}

// DevicePowerSourceCtrlValues are the values the controller accepts for Device.power_source_ctrl.
var DevicePowerSourceCtrlValues = []string{"auto", "8023af", "8023at", "8023bt-type3", "8023bt-type4", "pasv24", "poe-injector", "ac", "adapter", "dc", "rps"}

// DeviceResetbtnEnabledValues are the values the controller accepts for Device.resetbtn_enabled.
var DeviceResetbtnEnabledValues = []string{"on", "off"}

// DeviceSnmpContactMinLength and DeviceSnmpContactMaxLength are the character-count bounds the controller accepts for Device.snmp_contact.
const (
	DeviceSnmpContactMinLength int64 = 0
	DeviceSnmpContactMaxLength int64 = 255
)

// DeviceSnmpLocationMinLength and DeviceSnmpLocationMaxLength are the character-count bounds the controller accepts for Device.snmp_location.
const (
	DeviceSnmpLocationMinLength int64 = 0
	DeviceSnmpLocationMaxLength int64 = 255
)

// DeviceStationModeValues are the values the controller accepts for Device.station_mode.
var DeviceStationModeValues = []string{"ptp", "ptmp", "wifi"}

// DeviceStpPriorityValues are the values the controller accepts for Device.stp_priority.
var DeviceStpPriorityValues = []int64{0, 4096, 8192, 12288, 16384, 20480, 24576, 28672, 32768, 36864, 40960, 45056, 49152, 53248, 57344, 61440}

// DeviceStpVersionValues are the values the controller accepts for Device.stp_version.
var DeviceStpVersionValues = []string{"stp", "rstp", "disabled"}

// DeviceUbbPairNameMinLength and DeviceUbbPairNameMaxLength are the character-count bounds the controller accepts for Device.ubb_pair_name.
const (
	DeviceUbbPairNameMinLength int64 = 1
	DeviceUbbPairNameMaxLength int64 = 128
)

// DeviceUpsShutdownRemainingMinutesMin and DeviceUpsShutdownRemainingMinutesMax are the inclusive bounds the controller accepts for Device.ups_shutdown_remaining_minutes.
const (
	DeviceUpsShutdownRemainingMinutesMin int64 = 1
	DeviceUpsShutdownRemainingMinutesMax int64 = 15
)

// DeviceVolumeMin and DeviceVolumeMax are the inclusive bounds the controller accepts for Device.volume.
const (
	DeviceVolumeMin int64 = 0
	DeviceVolumeMax int64 = 100
)

// DeviceAudioInfoChannelMin and DeviceAudioInfoChannelMax are the inclusive bounds the controller accepts for DeviceAudioInfo.channel.
const (
	DeviceAudioInfoChannelMin int64 = 2
	DeviceAudioInfoChannelMax int64 = 9999
)

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

// DeviceEtherLightingBrightnessMin and DeviceEtherLightingBrightnessMax are the inclusive bounds the controller accepts for DeviceEtherLighting.brightness.
const (
	DeviceEtherLightingBrightnessMin int64 = 1
	DeviceEtherLightingBrightnessMax int64 = 100
)

// DeviceEtherLightingLedModeValues are the values the controller accepts for DeviceEtherLighting.led_mode.
var DeviceEtherLightingLedModeValues = []string{"standard", "etherlighting"}

// DeviceEtherLightingModeValues are the values the controller accepts for DeviceEtherLighting.mode.
var DeviceEtherLightingModeValues = []string{"speed", "network"}

// DeviceHdmiPortsStateValues are the values the controller accepts for DeviceHdmiPorts.state.
var DeviceHdmiPortsStateValues = []string{"CLIENT_STATE_SUSPENDING", "WAITING_HOST_MODE", "OPERATING"}

// DeviceHdmiPortsTypeValues are the values the controller accepts for DeviceHdmiPorts.type.
var DeviceHdmiPortsTypeValues = []string{"in", "out"}

// DeviceIPV6NetmaskMin and DeviceIPV6NetmaskMax are the inclusive bounds the controller accepts for DeviceIPV6.netmask.
const (
	DeviceIPV6NetmaskMin int64 = 0
	DeviceIPV6NetmaskMax int64 = 128
)

// DeviceIPV6TypeValues are the values the controller accepts for DeviceIPV6.type.
var DeviceIPV6TypeValues = []string{"slaac", "dhcp", "static", "none"}

// DeviceIPv4NetmaskMin and DeviceIPv4NetmaskMax are the inclusive bounds the controller accepts for DeviceIPv4.netmask.
const (
	DeviceIPv4NetmaskMin int64 = 0
	DeviceIPv4NetmaskMax int64 = 32
)

// DeviceIPv4TypeValues are the values the controller accepts for DeviceIPv4.type.
var DeviceIPv4TypeValues = []string{"dhcp", "static"}

// DeviceMbbOverridesPrimarySlotValues are the values the controller accepts for DeviceMbbOverrides.primary_slot.
var DeviceMbbOverridesPrimarySlotValues = []int64{1, 2}

// DeviceOutletOverridesNameMinLength and DeviceOutletOverridesNameMaxLength are the character-count bounds the controller accepts for DeviceOutletOverrides.name.
const (
	DeviceOutletOverridesNameMinLength int64 = 0
	DeviceOutletOverridesNameMaxLength int64 = 128
)

// DevicePortOverridesAggregateMembersMin and DevicePortOverridesAggregateMembersMax are the inclusive bounds the controller accepts for DevicePortOverrides.aggregate_members.
const (
	DevicePortOverridesAggregateMembersMin int64 = 1
	DevicePortOverridesAggregateMembersMax int64 = 56
)

// DevicePortOverridesDot1XCtrlValues are the values the controller accepts for DevicePortOverrides.dot1x_ctrl.
var DevicePortOverridesDot1XCtrlValues = []string{"auto", "force_authorized", "force_unauthorized", "mac_based", "multi_host"}

// DevicePortOverridesDot1XIDleTimeoutMin and DevicePortOverridesDot1XIDleTimeoutMax are the inclusive bounds the controller accepts for DevicePortOverrides.dot1x_idle_timeout.
const (
	DevicePortOverridesDot1XIDleTimeoutMin int64 = 0
	DevicePortOverridesDot1XIDleTimeoutMax int64 = 65535
)

// DevicePortOverridesFecModeValues are the values the controller accepts for DevicePortOverrides.fec_mode.
var DevicePortOverridesFecModeValues = []string{"rs-fec", "fc-fec", "default", "disabled"}

// DevicePortOverridesForwardValues are the values the controller accepts for DevicePortOverrides.forward.
var DevicePortOverridesForwardValues = []string{"all", "native", "customize", "disabled"}

// DevicePortOverridesMirrorPortIDXMin and DevicePortOverridesMirrorPortIDXMax are the inclusive bounds the controller accepts for DevicePortOverrides.mirror_port_idx.
const (
	DevicePortOverridesMirrorPortIDXMin int64 = 1
	DevicePortOverridesMirrorPortIDXMax int64 = 56
)

// DevicePortOverridesMulticastRouterModeValues are the values the controller accepts for DevicePortOverrides.multicast_router_mode.
var DevicePortOverridesMulticastRouterModeValues = []string{"ALL", "CUSTOM", "NONE"}

// DevicePortOverridesNameMinLength and DevicePortOverridesNameMaxLength are the character-count bounds the controller accepts for DevicePortOverrides.name.
const (
	DevicePortOverridesNameMinLength int64 = 0
	DevicePortOverridesNameMaxLength int64 = 128
)

// DevicePortOverridesOpModeValues are the values the controller accepts for DevicePortOverrides.op_mode.
var DevicePortOverridesOpModeValues = []string{"switch", "mirror", "aggregate", "routed", "routed_aggregate"}

// DevicePortOverridesPoeModeValues are the values the controller accepts for DevicePortOverrides.poe_mode.
var DevicePortOverridesPoeModeValues = []string{"auto", "pasv24", "passthrough", "off"}

// DevicePortOverridesPortIDXMin and DevicePortOverridesPortIDXMax are the inclusive bounds the controller accepts for DevicePortOverrides.port_idx.
const (
	DevicePortOverridesPortIDXMin int64 = 1
	DevicePortOverridesPortIDXMax int64 = 56
)

// DevicePortOverridesPriorityQueue1LevelMin and DevicePortOverridesPriorityQueue1LevelMax are the inclusive bounds the controller accepts for DevicePortOverrides.priority_queue1_level.
const (
	DevicePortOverridesPriorityQueue1LevelMin int64 = 0
	DevicePortOverridesPriorityQueue1LevelMax int64 = 100
)

// DevicePortOverridesPriorityQueue2LevelMin and DevicePortOverridesPriorityQueue2LevelMax are the inclusive bounds the controller accepts for DevicePortOverrides.priority_queue2_level.
const (
	DevicePortOverridesPriorityQueue2LevelMin int64 = 0
	DevicePortOverridesPriorityQueue2LevelMax int64 = 100
)

// DevicePortOverridesPriorityQueue3LevelMin and DevicePortOverridesPriorityQueue3LevelMax are the inclusive bounds the controller accepts for DevicePortOverrides.priority_queue3_level.
const (
	DevicePortOverridesPriorityQueue3LevelMin int64 = 0
	DevicePortOverridesPriorityQueue3LevelMax int64 = 100
)

// DevicePortOverridesPriorityQueue4LevelMin and DevicePortOverridesPriorityQueue4LevelMax are the inclusive bounds the controller accepts for DevicePortOverrides.priority_queue4_level.
const (
	DevicePortOverridesPriorityQueue4LevelMin int64 = 0
	DevicePortOverridesPriorityQueue4LevelMax int64 = 100
)

// DevicePortOverridesSettingPreferenceValues are the values the controller accepts for DevicePortOverrides.setting_preference.
var DevicePortOverridesSettingPreferenceValues = []string{"auto", "manual"}

// DevicePortOverridesSpeedValues are the values the controller accepts for DevicePortOverrides.speed.
var DevicePortOverridesSpeedValues = []int64{10, 100, 1000, 2500, 5000, 10000, 20000, 25000, 40000, 50000, 100000}

// DevicePortOverridesStormctrlBroadcastastLevelMin and DevicePortOverridesStormctrlBroadcastastLevelMax are the inclusive bounds the controller accepts for DevicePortOverrides.stormctrl_bcast_level.
const (
	DevicePortOverridesStormctrlBroadcastastLevelMin int64 = 0
	DevicePortOverridesStormctrlBroadcastastLevelMax int64 = 100
)

// DevicePortOverridesStormctrlMcastLevelMin and DevicePortOverridesStormctrlMcastLevelMax are the inclusive bounds the controller accepts for DevicePortOverrides.stormctrl_mcast_level.
const (
	DevicePortOverridesStormctrlMcastLevelMin int64 = 0
	DevicePortOverridesStormctrlMcastLevelMax int64 = 100
)

// DevicePortOverridesStormctrlTypeValues are the values the controller accepts for DevicePortOverrides.stormctrl_type.
var DevicePortOverridesStormctrlTypeValues = []string{"level", "rate"}

// DevicePortOverridesStormctrlUcastLevelMin and DevicePortOverridesStormctrlUcastLevelMax are the inclusive bounds the controller accepts for DevicePortOverrides.stormctrl_ucast_level.
const (
	DevicePortOverridesStormctrlUcastLevelMin int64 = 0
	DevicePortOverridesStormctrlUcastLevelMax int64 = 100
)

// DevicePortOverridesStpEdgeStateValues are the values the controller accepts for DevicePortOverrides.stp_edge_state.
var DevicePortOverridesStpEdgeStateValues = []string{"auto", "enabled", "disabled"}

// DevicePortOverridesTaggedVLANMgmtValues are the values the controller accepts for DevicePortOverrides.tagged_vlan_mgmt.
var DevicePortOverridesTaggedVLANMgmtValues = []string{"auto", "block_all", "custom"}

// DevicePrecisionTimeProtocolConfigClockModeValues are the values the controller accepts for DevicePrecisionTimeProtocolConfig.clock_mode.
var DevicePrecisionTimeProtocolConfigClockModeValues = []string{"boundary", "sma", "transparent"}

// DevicePrecisionTimeProtocolConfigCustomAnnounceIntervalMin and DevicePrecisionTimeProtocolConfigCustomAnnounceIntervalMax are the inclusive bounds the controller accepts for DevicePrecisionTimeProtocolConfig.custom_announce_interval.
const (
	DevicePrecisionTimeProtocolConfigCustomAnnounceIntervalMin int64 = -4
	DevicePrecisionTimeProtocolConfigCustomAnnounceIntervalMax int64 = 4
)

// DevicePrecisionTimeProtocolConfigCustomAnnounceTimeoutMin and DevicePrecisionTimeProtocolConfigCustomAnnounceTimeoutMax are the inclusive bounds the controller accepts for DevicePrecisionTimeProtocolConfig.custom_announce_timeout.
const (
	DevicePrecisionTimeProtocolConfigCustomAnnounceTimeoutMin int64 = 2
	DevicePrecisionTimeProtocolConfigCustomAnnounceTimeoutMax int64 = 10
)

// DevicePrecisionTimeProtocolConfigCustomDelayReqIntervalMin and DevicePrecisionTimeProtocolConfigCustomDelayReqIntervalMax are the inclusive bounds the controller accepts for DevicePrecisionTimeProtocolConfig.custom_delay_req_interval.
const (
	DevicePrecisionTimeProtocolConfigCustomDelayReqIntervalMin int64 = -7
	DevicePrecisionTimeProtocolConfigCustomDelayReqIntervalMax int64 = 4
)

// DevicePrecisionTimeProtocolConfigCustomDomainMin and DevicePrecisionTimeProtocolConfigCustomDomainMax are the inclusive bounds the controller accepts for DevicePrecisionTimeProtocolConfig.custom_domain.
const (
	DevicePrecisionTimeProtocolConfigCustomDomainMin int64 = 0
	DevicePrecisionTimeProtocolConfigCustomDomainMax int64 = 255
)

// DevicePrecisionTimeProtocolConfigCustomSyncIntervalMin and DevicePrecisionTimeProtocolConfigCustomSyncIntervalMax are the inclusive bounds the controller accepts for DevicePrecisionTimeProtocolConfig.custom_sync_interval.
const (
	DevicePrecisionTimeProtocolConfigCustomSyncIntervalMin int64 = -7
	DevicePrecisionTimeProtocolConfigCustomSyncIntervalMax int64 = 4
)

// DevicePrecisionTimeProtocolConfigPriority1Min and DevicePrecisionTimeProtocolConfigPriority1Max are the inclusive bounds the controller accepts for DevicePrecisionTimeProtocolConfig.priority1.
const (
	DevicePrecisionTimeProtocolConfigPriority1Min int64 = 0
	DevicePrecisionTimeProtocolConfigPriority1Max int64 = 255
)

// DevicePrecisionTimeProtocolConfigPriority2Min and DevicePrecisionTimeProtocolConfigPriority2Max are the inclusive bounds the controller accepts for DevicePrecisionTimeProtocolConfig.priority2.
const (
	DevicePrecisionTimeProtocolConfigPriority2Min int64 = 0
	DevicePrecisionTimeProtocolConfigPriority2Max int64 = 255
)

// DevicePrecisionTimeProtocolConfigProfileValues are the values the controller accepts for DevicePrecisionTimeProtocolConfig.profile.
var DevicePrecisionTimeProtocolConfigProfileValues = []string{"smpte", "ieee1588", "aes67", "aes_r16", "custom"}

// DevicePrecisionTimeProtocolConfigTransportTypeValues are the values the controller accepts for DevicePrecisionTimeProtocolConfig.transport_type.
var DevicePrecisionTimeProtocolConfigTransportTypeValues = []string{"ipv4", "layer2"}

// DeviceQOSMarkingCosCodeMin and DeviceQOSMarkingCosCodeMax are the inclusive bounds the controller accepts for DeviceQOSMarking.cos_code.
const (
	DeviceQOSMarkingCosCodeMin int64 = 0
	DeviceQOSMarkingCosCodeMax int64 = 7
)

// DeviceQOSMarkingDscpCodeValues are the values the controller accepts for DeviceQOSMarking.dscp_code.
var DeviceQOSMarkingDscpCodeValues = []int64{0, 8, 16, 24, 32, 40, 48, 56, 10, 12, 14, 18, 20, 22, 26, 28, 30, 34, 36, 38, 44, 46}

// DeviceQOSMarkingIPPrecedenceCodeMin and DeviceQOSMarkingIPPrecedenceCodeMax are the inclusive bounds the controller accepts for DeviceQOSMarking.ip_precedence_code.
const (
	DeviceQOSMarkingIPPrecedenceCodeMin int64 = 0
	DeviceQOSMarkingIPPrecedenceCodeMax int64 = 7
)

// DeviceQOSMarkingQueueMin and DeviceQOSMarkingQueueMax are the inclusive bounds the controller accepts for DeviceQOSMarking.queue.
const (
	DeviceQOSMarkingQueueMin int64 = 0
	DeviceQOSMarkingQueueMax int64 = 7
)

// DeviceQOSMatchingCosCodeMin and DeviceQOSMatchingCosCodeMax are the inclusive bounds the controller accepts for DeviceQOSMatching.cos_code.
const (
	DeviceQOSMatchingCosCodeMin int64 = 0
	DeviceQOSMatchingCosCodeMax int64 = 7
)

// DeviceQOSMatchingDscpCodeMin and DeviceQOSMatchingDscpCodeMax are the inclusive bounds the controller accepts for DeviceQOSMatching.dscp_code.
const (
	DeviceQOSMatchingDscpCodeMin int64 = 0
	DeviceQOSMatchingDscpCodeMax int64 = 63
)

// DeviceQOSMatchingDstPortMin and DeviceQOSMatchingDstPortMax are the inclusive bounds the controller accepts for DeviceQOSMatching.dst_port.
const (
	DeviceQOSMatchingDstPortMin int64 = 0
	DeviceQOSMatchingDstPortMax int64 = 65535
)

// DeviceQOSMatchingIPPrecedenceCodeMin and DeviceQOSMatchingIPPrecedenceCodeMax are the inclusive bounds the controller accepts for DeviceQOSMatching.ip_precedence_code.
const (
	DeviceQOSMatchingIPPrecedenceCodeMin int64 = 0
	DeviceQOSMatchingIPPrecedenceCodeMax int64 = 7
)

// DeviceQOSMatchingSrcPortMin and DeviceQOSMatchingSrcPortMax are the inclusive bounds the controller accepts for DeviceQOSMatching.src_port.
const (
	DeviceQOSMatchingSrcPortMin int64 = 0
	DeviceQOSMatchingSrcPortMax int64 = 65535
)

// DeviceQOSProfileQOSProfileModeValues are the values the controller accepts for DeviceQOSProfile.qos_profile_mode.
var DeviceQOSProfileQOSProfileModeValues = []string{"custom", "unifi_play", "aes67_audio", "crestron_audio_video", "dante_audio", "ndi_aes67_audio", "ndi_dante_audio", "qsys_audio_video", "qsys_video_dante_audio", "sdvoe_aes67_audio", "sdvoe_dante_audio", "shure_audio", "smpte_st2110"}

// DeviceRadioTableAntennaGainMin and DeviceRadioTableAntennaGainMax are the inclusive bounds the controller accepts for DeviceRadioTable.antenna_gain.
const (
	DeviceRadioTableAntennaGainMin int64 = -99
	DeviceRadioTableAntennaGainMax int64 = 99
)

// DeviceRadioTableAntennaIDMin and DeviceRadioTableAntennaIDMax are the inclusive bounds the controller accepts for DeviceRadioTable.antenna_id.
const (
	DeviceRadioTableAntennaIDMin int64 = -1
	DeviceRadioTableAntennaIDMax int64 = 9
)

// DeviceRadioTableHtValues are the values the controller accepts for DeviceRadioTable.ht.
var DeviceRadioTableHtValues = []int64{20, 40, 80, 160, 240, 320, 1080, 2160, 4320}

// DeviceRadioTableMaxstaMin and DeviceRadioTableMaxstaMax are the inclusive bounds the controller accepts for DeviceRadioTable.maxsta.
const (
	DeviceRadioTableMaxstaMin int64 = 1
	DeviceRadioTableMaxstaMax int64 = 200
)

// DeviceRadioTableMinRssiMin and DeviceRadioTableMinRssiMax are the inclusive bounds the controller accepts for DeviceRadioTable.min_rssi.
const (
	DeviceRadioTableMinRssiMin int64 = -90
	DeviceRadioTableMinRssiMax int64 = -67
)

// DeviceRadioTableRadioValues are the values the controller accepts for DeviceRadioTable.radio.
var DeviceRadioTableRadioValues = []string{"ng", "na", "ad", "6e"}

// DeviceRadioTableSensLevelMin and DeviceRadioTableSensLevelMax are the inclusive bounds the controller accepts for DeviceRadioTable.sens_level.
const (
	DeviceRadioTableSensLevelMin int64 = -90
	DeviceRadioTableSensLevelMax int64 = -50
)

// DeviceRadioTableTxPowerModeValues are the values the controller accepts for DeviceRadioTable.tx_power_mode.
var DeviceRadioTableTxPowerModeValues = []string{"auto", "medium", "high", "low", "custom", "disabled"}

// DeviceRpsOverridePowerManagementModeValues are the values the controller accepts for DeviceRpsOverride.power_management_mode.
var DeviceRpsOverridePowerManagementModeValues = []string{"dynamic", "static"}

// DeviceRpsPortTableNameMinLength and DeviceRpsPortTableNameMaxLength are the character-count bounds the controller accepts for DeviceRpsPortTable.name.
const (
	DeviceRpsPortTableNameMinLength int64 = 0
	DeviceRpsPortTableNameMaxLength int64 = 32
)

// DeviceRpsPortTablePortIDXMin and DeviceRpsPortTablePortIDXMax are the inclusive bounds the controller accepts for DeviceRpsPortTable.port_idx.
const (
	DeviceRpsPortTablePortIDXMin int64 = 1
	DeviceRpsPortTablePortIDXMax int64 = 8
)

// DeviceRpsPortTablePortModeValues are the values the controller accepts for DeviceRpsPortTable.port_mode.
var DeviceRpsPortTablePortModeValues = []string{"auto", "force_active", "manual", "disabled"}

// DeviceSimDataSoftLimitDisplayUnitValues are the values the controller accepts for DeviceSim.data_soft_limit_display_unit.
var DeviceSimDataSoftLimitDisplayUnitValues = []string{"MB", "GB"}

// DeviceSimDataWarningThresholdMin and DeviceSimDataWarningThresholdMax are the inclusive bounds the controller accepts for DeviceSim.data_warning_threshold.
const (
	DeviceSimDataWarningThresholdMin int64 = 0
	DeviceSimDataWarningThresholdMax int64 = 100
)

// DeviceSimResetDateMin and DeviceSimResetDateMax are the inclusive bounds the controller accepts for DeviceSim.reset_date.
const (
	DeviceSimResetDateMin int64 = 0
	DeviceSimResetDateMax int64 = 31
)

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

// DeviceVideoInfoChannelMin and DeviceVideoInfoChannelMax are the inclusive bounds the controller accepts for DeviceVideoInfo.channel.
const (
	DeviceVideoInfoChannelMin int64 = 2
	DeviceVideoInfoChannelMax int64 = 9999
)

// DeviceVideoInfoColorFormatValues are the values the controller accepts for DeviceVideoInfo.color_format.
var DeviceVideoInfoColorFormatValues = []string{"rgb", "ycbcr444", "ycbcr422"}

// DeviceVideoInfoModeValues are the values the controller accepts for DeviceVideoInfo.mode.
var DeviceVideoInfoModeValues = []string{"unicast", "multicast"}

// DeviceVideoInfoResolutionValues are the values the controller accepts for DeviceVideoInfo.resolution.
var DeviceVideoInfoResolutionValues = []string{"auto", "1080p", "1440p", "4k"}

// DeviceVideoInfoRoleValues are the values the controller accepts for DeviceVideoInfo.role.
var DeviceVideoInfoRoleValues = []string{"host", "client"}

// DeviceVideoInfoTvwallEndXMin and DeviceVideoInfoTvwallEndXMax are the inclusive bounds the controller accepts for DeviceVideoInfo.tvwall_end_x.
const (
	DeviceVideoInfoTvwallEndXMin int64 = 2
	DeviceVideoInfoTvwallEndXMax int64 = 5
)

// DeviceVideoInfoTvwallEndYMin and DeviceVideoInfoTvwallEndYMax are the inclusive bounds the controller accepts for DeviceVideoInfo.tvwall_end_y.
const (
	DeviceVideoInfoTvwallEndYMin int64 = 2
	DeviceVideoInfoTvwallEndYMax int64 = 5
)

// DeviceVideoInfoTvwallLayoutXMin and DeviceVideoInfoTvwallLayoutXMax are the inclusive bounds the controller accepts for DeviceVideoInfo.tvwall_layout_x.
const (
	DeviceVideoInfoTvwallLayoutXMin int64 = 1
	DeviceVideoInfoTvwallLayoutXMax int64 = 4
)

// DeviceVideoInfoTvwallLayoutYMin and DeviceVideoInfoTvwallLayoutYMax are the inclusive bounds the controller accepts for DeviceVideoInfo.tvwall_layout_y.
const (
	DeviceVideoInfoTvwallLayoutYMin int64 = 1
	DeviceVideoInfoTvwallLayoutYMax int64 = 4
)

// DeviceVideoInfoTvwallStartXMin and DeviceVideoInfoTvwallStartXMax are the inclusive bounds the controller accepts for DeviceVideoInfo.tvwall_start_x.
const (
	DeviceVideoInfoTvwallStartXMin int64 = 2
	DeviceVideoInfoTvwallStartXMax int64 = 5
)

// DeviceVideoInfoTvwallStartYMin and DeviceVideoInfoTvwallStartYMax are the inclusive bounds the controller accepts for DeviceVideoInfo.tvwall_start_y.
const (
	DeviceVideoInfoTvwallStartYMin int64 = 2
	DeviceVideoInfoTvwallStartYMax int64 = 5
)

// DpiAppNameMinLength and DpiAppNameMaxLength are the character-count bounds the controller accepts for DpiApp.name.
const (
	DpiAppNameMinLength int64 = 1
	DpiAppNameMaxLength int64 = 128
)

// DpiGroupNameMinLength and DpiGroupNameMaxLength are the character-count bounds the controller accepts for DpiGroup.name.
const (
	DpiGroupNameMinLength int64 = 1
	DpiGroupNameMaxLength int64 = 128
)

// DynamicDNSServiceValues are the values the controller accepts for DynamicDNS.service.
var DynamicDNSServiceValues = []string{"afraid", "changeip", "cloudflare", "cloudxns", "ddnss", "dhis", "dnsexit", "dnsomatic", "dnspark", "dnspod", "dslreports", "dtdns", "duckdns", "duiadns", "dyn", "dyndns", "dynv6", "easydns", "freemyip", "googledomains", "loopia", "namecheap", "noip", "nsupdate", "ovh", "sitelutions", "spdyn", "strato", "tunnelbroker", "zoneedit", "custom"}

// FirewallGroupGroupTypeValues are the values the controller accepts for FirewallGroup.group_type.
var FirewallGroupGroupTypeValues = []string{"address-group", "port-group", "ipv6-address-group"}

// FirewallGroupNameMinLength and FirewallGroupNameMaxLength are the character-count bounds the controller accepts for FirewallGroup.name.
const (
	FirewallGroupNameMinLength int64 = 1
	FirewallGroupNameMaxLength int64 = 64
)

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
var FirewallPolicyScheduleModeValues = []string{"ALWAYS", "EVERY_DAY", "EVERY_WEEK", "ONE_TIME_ONLY", "CUSTOM"}

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

// FirewallRuleNameMinLength and FirewallRuleNameMaxLength are the character-count bounds the controller accepts for FirewallRule.name.
const (
	FirewallRuleNameMinLength int64 = 1
	FirewallRuleNameMaxLength int64 = 128
)

// FirewallRuleRulesetValues are the values the controller accepts for FirewallRule.ruleset.
var FirewallRuleRulesetValues = []string{"WAN_IN", "WAN_OUT", "WAN_LOCAL", "LAN_IN", "LAN_OUT", "LAN_LOCAL", "GUEST_IN", "GUEST_OUT", "GUEST_LOCAL", "WANv6_IN", "WANv6_OUT", "WANv6_LOCAL", "LANv6_IN", "LANv6_OUT", "LANv6_LOCAL", "GUESTv6_IN", "GUESTv6_OUT", "GUESTv6_LOCAL"}

// FirewallRuleSettingPreferenceValues are the values the controller accepts for FirewallRule.setting_preference.
var FirewallRuleSettingPreferenceValues = []string{"auto", "manual"}

// FirewallRuleSrcNetworkTypeValues are the values the controller accepts for FirewallRule.src_networkconf_type.
var FirewallRuleSrcNetworkTypeValues = []string{"ADDRv4", "NETv4"}

// Hotspot2ConfAnqpDomainIDMin and Hotspot2ConfAnqpDomainIDMax are the inclusive bounds the controller accepts for Hotspot2Conf.anqp_domain_id.
const (
	Hotspot2ConfAnqpDomainIDMin int64 = 0
	Hotspot2ConfAnqpDomainIDMax int64 = 65535
)

// Hotspot2ConfDeauthReqTimeoutMin and Hotspot2ConfDeauthReqTimeoutMax are the inclusive bounds the controller accepts for Hotspot2Conf.deauth_req_timeout.
const (
	Hotspot2ConfDeauthReqTimeoutMin int64 = 10
	Hotspot2ConfDeauthReqTimeoutMax int64 = 3600
)

// Hotspot2ConfDomainNameListMinLength and Hotspot2ConfDomainNameListMaxLength are the character-count bounds the controller accepts for Hotspot2Conf.domain_name_list.
const (
	Hotspot2ConfDomainNameListMinLength int64 = 1
	Hotspot2ConfDomainNameListMaxLength int64 = 128
)

// Hotspot2ConfIPaddrTypeAvailV4Values are the values the controller accepts for Hotspot2Conf.ipaddr_type_avail_v4.
var Hotspot2ConfIPaddrTypeAvailV4Values = []int64{0, 1, 2, 3, 4, 5, 6, 7}

// Hotspot2ConfIPaddrTypeAvailV6Values are the values the controller accepts for Hotspot2Conf.ipaddr_type_avail_v6.
var Hotspot2ConfIPaddrTypeAvailV6Values = []int64{0, 1, 2}

// Hotspot2ConfMetricsInfoLinkStatusValues are the values the controller accepts for Hotspot2Conf.metrics_info_link_status.
var Hotspot2ConfMetricsInfoLinkStatusValues = []string{"up", "down", "test"}

// Hotspot2ConfNameMinLength and Hotspot2ConfNameMaxLength are the character-count bounds the controller accepts for Hotspot2Conf.name.
const (
	Hotspot2ConfNameMinLength int64 = 1
	Hotspot2ConfNameMaxLength int64 = 128
)

// Hotspot2ConfNetworkAuthTypeValues are the values the controller accepts for Hotspot2Conf.network_auth_type.
var Hotspot2ConfNetworkAuthTypeValues = []int64{-1, 0, 1, 2, 3}

// Hotspot2ConfNetworkTypeValues are the values the controller accepts for Hotspot2Conf.network_type.
var Hotspot2ConfNetworkTypeValues = []int64{0, 1, 2, 3, 4, 5, 14, 15}

// Hotspot2ConfTCFilenameMinLength and Hotspot2ConfTCFilenameMaxLength are the character-count bounds the controller accepts for Hotspot2Conf.t_c_filename.
const (
	Hotspot2ConfTCFilenameMinLength int64 = 1
	Hotspot2ConfTCFilenameMaxLength int64 = 256
)

// Hotspot2ConfVenueGroupValues are the values the controller accepts for Hotspot2Conf.venue_group.
var Hotspot2ConfVenueGroupValues = []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

// Hotspot2ConfVenueTypeValues are the values the controller accepts for Hotspot2Conf.venue_type.
var Hotspot2ConfVenueTypeValues = []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

// Hotspot2ConfCapabPortMin and Hotspot2ConfCapabPortMax are the inclusive bounds the controller accepts for Hotspot2ConfCapab.port.
const (
	Hotspot2ConfCapabPortMin int64 = 0
	Hotspot2ConfCapabPortMax int64 = 65535
)

// Hotspot2ConfCapabProtocolValues are the values the controller accepts for Hotspot2ConfCapab.protocol.
var Hotspot2ConfCapabProtocolValues = []string{"icmp", "tcp_udp", "tcp", "udp", "esp"}

// Hotspot2ConfCapabStatusValues are the values the controller accepts for Hotspot2ConfCapab.status.
var Hotspot2ConfCapabStatusValues = []string{"closed", "open", "unknown"}

// Hotspot2ConfCellularNetworkListNameMinLength and Hotspot2ConfCellularNetworkListNameMaxLength are the character-count bounds the controller accepts for Hotspot2ConfCellularNetworkList.name.
const (
	Hotspot2ConfCellularNetworkListNameMinLength int64 = 1
	Hotspot2ConfCellularNetworkListNameMaxLength int64 = 128
)

// Hotspot2ConfDescriptionTextMinLength and Hotspot2ConfDescriptionTextMaxLength are the character-count bounds the controller accepts for Hotspot2ConfDescription.text.
const (
	Hotspot2ConfDescriptionTextMinLength int64 = 1
	Hotspot2ConfDescriptionTextMaxLength int64 = 128
)

// Hotspot2ConfFriendlyNameTextMinLength and Hotspot2ConfFriendlyNameTextMaxLength are the character-count bounds the controller accepts for Hotspot2ConfFriendlyName.text.
const (
	Hotspot2ConfFriendlyNameTextMinLength int64 = 1
	Hotspot2ConfFriendlyNameTextMaxLength int64 = 128
)

// Hotspot2ConfIconNameMinLength and Hotspot2ConfIconNameMaxLength are the character-count bounds the controller accepts for Hotspot2ConfIcon.name.
const (
	Hotspot2ConfIconNameMinLength int64 = 1
	Hotspot2ConfIconNameMaxLength int64 = 128
)

// Hotspot2ConfIconsFilenameMinLength and Hotspot2ConfIconsFilenameMaxLength are the character-count bounds the controller accepts for Hotspot2ConfIcons.filename.
const (
	Hotspot2ConfIconsFilenameMinLength int64 = 1
	Hotspot2ConfIconsFilenameMaxLength int64 = 256
)

// Hotspot2ConfIconsMediaMinLength and Hotspot2ConfIconsMediaMaxLength are the character-count bounds the controller accepts for Hotspot2ConfIcons.media.
const (
	Hotspot2ConfIconsMediaMinLength int64 = 1
	Hotspot2ConfIconsMediaMaxLength int64 = 256
)

// Hotspot2ConfIconsNameMinLength and Hotspot2ConfIconsNameMaxLength are the character-count bounds the controller accepts for Hotspot2ConfIcons.name.
const (
	Hotspot2ConfIconsNameMinLength int64 = 1
	Hotspot2ConfIconsNameMaxLength int64 = 256
)

// Hotspot2ConfNaiRealmListEapMethodValues are the values the controller accepts for Hotspot2ConfNaiRealmList.eap_method.
var Hotspot2ConfNaiRealmListEapMethodValues = []int64{13, 21, 18, 23, 50}

// Hotspot2ConfNaiRealmListEncodingValues are the values the controller accepts for Hotspot2ConfNaiRealmList.encoding.
var Hotspot2ConfNaiRealmListEncodingValues = []int64{0, 1}

// Hotspot2ConfNaiRealmListNameMinLength and Hotspot2ConfNaiRealmListNameMaxLength are the character-count bounds the controller accepts for Hotspot2ConfNaiRealmList.name.
const (
	Hotspot2ConfNaiRealmListNameMinLength int64 = 1
	Hotspot2ConfNaiRealmListNameMaxLength int64 = 128
)

// Hotspot2ConfQOSMapExceptionsUpMin and Hotspot2ConfQOSMapExceptionsUpMax are the inclusive bounds the controller accepts for Hotspot2ConfQOSMapExceptions.up.
const (
	Hotspot2ConfQOSMapExceptionsUpMin int64 = 0
	Hotspot2ConfQOSMapExceptionsUpMax int64 = 7
)

// Hotspot2ConfRoamingConsortiumListNameMinLength and Hotspot2ConfRoamingConsortiumListNameMaxLength are the character-count bounds the controller accepts for Hotspot2ConfRoamingConsortiumList.name.
const (
	Hotspot2ConfRoamingConsortiumListNameMinLength int64 = 1
	Hotspot2ConfRoamingConsortiumListNameMaxLength int64 = 128
)

// Hotspot2ConfRoamingConsortiumListOidMinLength and Hotspot2ConfRoamingConsortiumListOidMaxLength are the character-count bounds the controller accepts for Hotspot2ConfRoamingConsortiumList.oid.
const (
	Hotspot2ConfRoamingConsortiumListOidMinLength int64 = 1
	Hotspot2ConfRoamingConsortiumListOidMaxLength int64 = 128
)

// HotspotOpNameMinLength and HotspotOpNameMaxLength are the character-count bounds the controller accepts for HotspotOp.name.
const (
	HotspotOpNameMinLength int64 = 1
	HotspotOpNameMaxLength int64 = 256
)

// HotspotOpPasswordMinLength and HotspotOpPasswordMaxLength are the character-count bounds the controller accepts for HotspotOp.x_password.
const (
	HotspotOpPasswordMinLength int64 = 1
	HotspotOpPasswordMaxLength int64 = 256
)

// NatVersionValues are the values the controller accepts for Nat.ip_version.
var NatVersionValues = []string{"IPV4", "IPV6"}

// NatPortMin and NatPortMax are the inclusive bounds the controller accepts for Nat.port.
const (
	NatPortMin int64 = 1
	NatPortMax int64 = 99999
)

// NatProtocolValues are the values the controller accepts for Nat.protocol.
var NatProtocolValues = []string{"all", "tcp", "udp", "tcp_udp"}

// NatSettingPreferenceValues are the values the controller accepts for Nat.setting_preference.
var NatSettingPreferenceValues = []string{"auto", "manual"}

// NatTypeValues are the values the controller accepts for Nat.type.
var NatTypeValues = []string{"DNAT", "SNAT", "MASQUERADE"}

// NatDestinationFilterFilterTypeValues are the values the controller accepts for NatDestinationFilter.filter_type.
var NatDestinationFilterFilterTypeValues = []string{"NONE", "ADDRESS_AND_PORT", "FIREWALL_GROUPS", "NETWORK_CONF"}

// NatDestinationFilterPortMin and NatDestinationFilterPortMax are the inclusive bounds the controller accepts for NatDestinationFilter.port.
const (
	NatDestinationFilterPortMin int64 = 1
	NatDestinationFilterPortMax int64 = 99999
)

// NatSourceFilterFilterTypeValues are the values the controller accepts for NatSourceFilter.filter_type.
var NatSourceFilterFilterTypeValues = []string{"NONE", "ADDRESS_AND_PORT", "FIREWALL_GROUPS", "NETWORK_CONF"}

// NatSourceFilterPortMin and NatSourceFilterPortMax are the inclusive bounds the controller accepts for NatSourceFilter.port.
const (
	NatSourceFilterPortMin int64 = 1
	NatSourceFilterPortMax int64 = 99999
)

// NetworkDHCPDBootFilenameMinLength and NetworkDHCPDBootFilenameMaxLength are the character-count bounds the controller accepts for Network.dhcpd_boot_filename.
const (
	NetworkDHCPDBootFilenameMinLength int64 = 1
	NetworkDHCPDBootFilenameMaxLength int64 = 256
)

// NetworkDHCPDTimeOffsetMin and NetworkDHCPDTimeOffsetMax are the inclusive bounds the controller accepts for Network.dhcpd_time_offset.
const (
	NetworkDHCPDTimeOffsetMin int64 = -86400
	NetworkDHCPDTimeOffsetMax int64 = 86400
)

// NetworkGatewayTypeValues are the values the controller accepts for Network.gateway_type.
var NetworkGatewayTypeValues = []string{"default", "switch"}

// NetworkIGMPGroupmembershipMin and NetworkIGMPGroupmembershipMax are the inclusive bounds the controller accepts for Network.igmp_groupmembership.
const (
	NetworkIGMPGroupmembershipMin int64 = 2
	NetworkIGMPGroupmembershipMax int64 = 3600
)

// NetworkIGMPMaxresponseMin and NetworkIGMPMaxresponseMax are the inclusive bounds the controller accepts for Network.igmp_maxresponse.
const (
	NetworkIGMPMaxresponseMin int64 = 1
	NetworkIGMPMaxresponseMax int64 = 25
)

// NetworkIGMPMcrtrexpiretimeMin and NetworkIGMPMcrtrexpiretimeMax are the inclusive bounds the controller accepts for Network.igmp_mcrtrexpiretime.
const (
	NetworkIGMPMcrtrexpiretimeMin int64 = 0
	NetworkIGMPMcrtrexpiretimeMax int64 = 3600
)

// NetworkIGMPProxyForValues are the values the controller accepts for Network.igmp_proxy_for.
var NetworkIGMPProxyForValues = []string{"all", "some", "none"}

// NetworkInterfaceMtuMin and NetworkInterfaceMtuMax are the inclusive bounds the controller accepts for Network.interface_mtu.
const (
	NetworkInterfaceMtuMin int64 = 68
	NetworkInterfaceMtuMax int64 = 65536
)

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

// NetworkIPSecEspLifetimeMin and NetworkIPSecEspLifetimeMax are the inclusive bounds the controller accepts for Network.ipsec_esp_lifetime.
const (
	NetworkIPSecEspLifetimeMin int64 = 30
	NetworkIPSecEspLifetimeMax int64 = 86400
)

// NetworkIPSecHashValues are the values the controller accepts for Network.ipsec_hash.
var NetworkIPSecHashValues = []string{"sha1", "md5", "sha256", "sha384", "sha512"}

// NetworkIPSecIkeDhGroupValues are the values the controller accepts for Network.ipsec_ike_dh_group.
var NetworkIPSecIkeDhGroupValues = []int64{1, 2, 5, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

// NetworkIPSecIkeEncryptionValues are the values the controller accepts for Network.ipsec_ike_encryption.
var NetworkIPSecIkeEncryptionValues = []string{"aes128", "aes192", "aes256", "3des"}

// NetworkIPSecIkeHashValues are the values the controller accepts for Network.ipsec_ike_hash.
var NetworkIPSecIkeHashValues = []string{"sha1", "md5", "sha256", "sha384", "sha512"}

// NetworkIPSecIkeLifetimeMin and NetworkIPSecIkeLifetimeMax are the inclusive bounds the controller accepts for Network.ipsec_ike_lifetime.
const (
	NetworkIPSecIkeLifetimeMin int64 = 30
	NetworkIPSecIkeLifetimeMax int64 = 86400
)

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

// NetworkLocalPortMin and NetworkLocalPortMax are the inclusive bounds the controller accepts for Network.local_port.
const (
	NetworkLocalPortMin int64 = 1
	NetworkLocalPortMax int64 = 65535
)

// NetworkMssClampValues are the values the controller accepts for Network.mss_clamp.
var NetworkMssClampValues = []string{"auto", "custom", "disabled"}

// NetworkMssClampIPV6Values are the values the controller accepts for Network.mss_clamp_ipv6.
var NetworkMssClampIPV6Values = []string{"auto", "custom", "disabled"}

// NetworkMssClampMssMin and NetworkMssClampMssMax are the inclusive bounds the controller accepts for Network.mss_clamp_mss.
const (
	NetworkMssClampMssMin int64 = 500
	NetworkMssClampMssMax int64 = 8960
)

// NetworkMssClampMssIPV6Min and NetworkMssClampMssIPV6Max are the inclusive bounds the controller accepts for Network.mss_clamp_mss_ipv6.
const (
	NetworkMssClampMssIPV6Min int64 = 500
	NetworkMssClampMssIPV6Max int64 = 8960
)

// NetworkNameMinLength and NetworkNameMaxLength are the character-count bounds the controller accepts for Network.name.
const (
	NetworkNameMinLength int64 = 1
	NetworkNameMaxLength int64 = 128
)

// NetworkOpenVPNEncryptionCipherValues are the values the controller accepts for Network.openvpn_encryption_cipher.
var NetworkOpenVPNEncryptionCipherValues = []string{"AES_256_CBC", "BF_CBC"}

// NetworkOpenVPNLocalPortMin and NetworkOpenVPNLocalPortMax are the inclusive bounds the controller accepts for Network.openvpn_local_port.
const (
	NetworkOpenVPNLocalPortMin int64 = 1
	NetworkOpenVPNLocalPortMax int64 = 65535
)

// NetworkOpenVPNModeValues are the values the controller accepts for Network.openvpn_mode.
var NetworkOpenVPNModeValues = []string{"site-to-site", "client", "server"}

// NetworkOpenVPNRemotePortMin and NetworkOpenVPNRemotePortMax are the inclusive bounds the controller accepts for Network.openvpn_remote_port.
const (
	NetworkOpenVPNRemotePortMin int64 = 1
	NetworkOpenVPNRemotePortMax int64 = 65535
)

// NetworkPptpcRouteDistanceMin and NetworkPptpcRouteDistanceMax are the inclusive bounds the controller accepts for Network.pptpc_route_distance.
const (
	NetworkPptpcRouteDistanceMin int64 = 1
	NetworkPptpcRouteDistanceMax int64 = 255
)

// NetworkPriorityMin and NetworkPriorityMax are the inclusive bounds the controller accepts for Network.priority.
const (
	NetworkPriorityMin int64 = 1
	NetworkPriorityMax int64 = 4
)

// NetworkPurposeValues are the values the controller accepts for Network.purpose.
var NetworkPurposeValues = []string{"corporate", "guest", "remote-user-vpn", "site-vpn", "vlan-only", "vpn-client", "wan"}

// NetworkRouteDistanceMin and NetworkRouteDistanceMax are the inclusive bounds the controller accepts for Network.route_distance.
const (
	NetworkRouteDistanceMin int64 = 1
	NetworkRouteDistanceMax int64 = 255
)

// NetworkRoutedLagIDXMin and NetworkRoutedLagIDXMax are the inclusive bounds the controller accepts for Network.routed_lag_idx.
const (
	NetworkRoutedLagIDXMin int64 = 0
	NetworkRoutedLagIDXMax int64 = 99
)

// NetworkRoutedPortIDXMin and NetworkRoutedPortIDXMax are the inclusive bounds the controller accepts for Network.routed_port_idx.
const (
	NetworkRoutedPortIDXMin int64 = 0
	NetworkRoutedPortIDXMax int64 = 99
)

// NetworkSettingPreferenceValues are the values the controller accepts for Network.setting_preference.
var NetworkSettingPreferenceValues = []string{"auto", "manual"}

// NetworkUidPublicGatewayPortMin and NetworkUidPublicGatewayPortMax are the inclusive bounds the controller accepts for Network.uid_public_gateway_port.
const (
	NetworkUidPublicGatewayPortMin int64 = 1
	NetworkUidPublicGatewayPortMax int64 = 65535
)

// NetworkUidVPNTypeValues are the values the controller accepts for Network.uid_vpn_type.
var NetworkUidVPNTypeValues = []string{"openvpn", "wireguard"}

// NetworkVLANMin and NetworkVLANMax are the inclusive bounds the controller accepts for Network.vlan.
const (
	NetworkVLANMin int64 = 2
	NetworkVLANMax int64 = 4018
)

// NetworkVPNBindingModeValues are the values the controller accepts for Network.vpn_binding_mode.
var NetworkVPNBindingModeValues = []string{"static", "interface", "any"}

// NetworkVPNProtocolValues are the values the controller accepts for Network.vpn_protocol.
var NetworkVPNProtocolValues = []string{"TCP", "UDP"}

// NetworkVPNTypeValues are the values the controller accepts for Network.vpn_type.
var NetworkVPNTypeValues = []string{"auto", "ipsec-vpn", "openvpn-client", "openvpn-server", "openvpn-vpn", "pptp-client", "l2tp-server", "pptp-server", "sdwan-hub-spoke-tunnel", "sdwan-mesh-tunnel", "uid-server", "wireguard-server", "wireguard-client"}

// NetworkVrrpVridMin and NetworkVrrpVridMax are the inclusive bounds the controller accepts for Network.vrrp_vrid.
const (
	NetworkVrrpVridMin int64 = 1
	NetworkVrrpVridMax int64 = 99
)

// NetworkWANDHCPCosMin and NetworkWANDHCPCosMax are the inclusive bounds the controller accepts for Network.wan_dhcp_cos.
const (
	NetworkWANDHCPCosMin int64 = 0
	NetworkWANDHCPCosMax int64 = 7
)

// NetworkWANDHCPv6CosMin and NetworkWANDHCPv6CosMax are the inclusive bounds the controller accepts for Network.wan_dhcpv6_cos.
const (
	NetworkWANDHCPv6CosMin int64 = 0
	NetworkWANDHCPv6CosMax int64 = 7
)

// NetworkWANDHCPv6PDSizeMin and NetworkWANDHCPv6PDSizeMax are the inclusive bounds the controller accepts for Network.wan_dhcpv6_pd_size.
const (
	NetworkWANDHCPv6PDSizeMin int64 = 48
	NetworkWANDHCPv6PDSizeMax int64 = 64
)

// NetworkWANDNSPreferenceValues are the values the controller accepts for Network.wan_dns_preference.
var NetworkWANDNSPreferenceValues = []string{"auto", "manual"}

// NetworkWANEgressQOSMin and NetworkWANEgressQOSMax are the inclusive bounds the controller accepts for Network.wan_egress_qos.
const (
	NetworkWANEgressQOSMin int64 = 1
	NetworkWANEgressQOSMax int64 = 7
)

// NetworkWANFailoverPriorityMin and NetworkWANFailoverPriorityMax are the inclusive bounds the controller accepts for Network.wan_failover_priority.
const (
	NetworkWANFailoverPriorityMin int64 = 1
	NetworkWANFailoverPriorityMax int64 = 9
)

// NetworkWANIPV6DNSPreferenceValues are the values the controller accepts for Network.wan_ipv6_dns_preference.
var NetworkWANIPV6DNSPreferenceValues = []string{"auto", "manual"}

// NetworkWANLoadBalanceTypeValues are the values the controller accepts for Network.wan_load_balance_type.
var NetworkWANLoadBalanceTypeValues = []string{"failover-only", "weighted"}

// NetworkWANLoadBalanceWeightMin and NetworkWANLoadBalanceWeightMax are the inclusive bounds the controller accepts for Network.wan_load_balance_weight.
const (
	NetworkWANLoadBalanceWeightMin int64 = 1
	NetworkWANLoadBalanceWeightMax int64 = 99
)

// NetworkWANPrefixlenMin and NetworkWANPrefixlenMax are the inclusive bounds the controller accepts for Network.wan_prefixlen.
const (
	NetworkWANPrefixlenMin int64 = 1
	NetworkWANPrefixlenMax int64 = 128
)

// NetworkWANTypeValues are the values the controller accepts for Network.wan_type.
var NetworkWANTypeValues = []string{"disabled", "dhcp", "static", "pppoe", "dslite", "map-e,hubspoke", "map-e,jpix", "map-e,ntt", "dslite-over-pppoe"}

// NetworkWANTypeV6Values are the values the controller accepts for Network.wan_type_v6.
var NetworkWANTypeV6Values = []string{"disabled", "slaac", "dhcpv6", "static"}

// NetworkWANVLANMin and NetworkWANVLANMax are the inclusive bounds the controller accepts for Network.wan_vlan.
const (
	NetworkWANVLANMin int64 = 0
	NetworkWANVLANMax int64 = 4094
)

// NetworkWireguardClientModeValues are the values the controller accepts for Network.wireguard_client_mode.
var NetworkWireguardClientModeValues = []string{"file", "manual"}

// NetworkWireguardClientPeerPortMin and NetworkWireguardClientPeerPortMax are the inclusive bounds the controller accepts for Network.wireguard_client_peer_port.
const (
	NetworkWireguardClientPeerPortMin int64 = 1
	NetworkWireguardClientPeerPortMax int64 = 65535
)

// NetworkWireguardInterfaceBindingModeIPVersionValues are the values the controller accepts for Network.wireguard_interface_binding_mode_ip_version.
var NetworkWireguardInterfaceBindingModeIPVersionValues = []string{"v4", "v6"}

// NetworkNATOutboundIPAddressesModeValues are the values the controller accepts for NetworkNATOutboundIPAddresses.mode.
var NetworkNATOutboundIPAddressesModeValues = []string{"all", "ip_address", "ip_address_pool"}

// NetworkWANDHCPOptionsOptionNumberMin and NetworkWANDHCPOptionsOptionNumberMax are the inclusive bounds the controller accepts for NetworkWANDHCPOptions.optionNumber.
const (
	NetworkWANDHCPOptionsOptionNumberMin int64 = 1
	NetworkWANDHCPOptionsOptionNumberMax int64 = 254
)

// PortForwardNameMinLength and PortForwardNameMaxLength are the character-count bounds the controller accepts for PortForward.name.
const (
	PortForwardNameMinLength int64 = 1
	PortForwardNameMaxLength int64 = 128
)

// PortForwardProtoValues are the values the controller accepts for PortForward.proto.
var PortForwardProtoValues = []string{"tcp_udp", "tcp", "udp"}

// PortForwardSrcLimitingTypeValues are the values the controller accepts for PortForward.src_limiting_type.
var PortForwardSrcLimitingTypeValues = []string{"ip", "firewall_group"}

// PortProfileDot1XCtrlValues are the values the controller accepts for PortProfile.dot1x_ctrl.
var PortProfileDot1XCtrlValues = []string{"auto", "force_authorized", "force_unauthorized", "mac_based", "multi_host"}

// PortProfileDot1XIDleTimeoutMin and PortProfileDot1XIDleTimeoutMax are the inclusive bounds the controller accepts for PortProfile.dot1x_idle_timeout.
const (
	PortProfileDot1XIDleTimeoutMin int64 = 0
	PortProfileDot1XIDleTimeoutMax int64 = 65535
)

// PortProfileFecModeValues are the values the controller accepts for PortProfile.fec_mode.
var PortProfileFecModeValues = []string{"rs-fec", "fc-fec", "default", "disabled"}

// PortProfileForwardValues are the values the controller accepts for PortProfile.forward.
var PortProfileForwardValues = []string{"all", "native", "customize", "disabled"}

// PortProfileMulticastRouterModeValues are the values the controller accepts for PortProfile.multicast_router_mode.
var PortProfileMulticastRouterModeValues = []string{"ALL", "CUSTOM", "NONE"}

// PortProfilePoeModeValues are the values the controller accepts for PortProfile.poe_mode.
var PortProfilePoeModeValues = []string{"auto", "off"}

// PortProfilePriorityQueue1LevelMin and PortProfilePriorityQueue1LevelMax are the inclusive bounds the controller accepts for PortProfile.priority_queue1_level.
const (
	PortProfilePriorityQueue1LevelMin int64 = 0
	PortProfilePriorityQueue1LevelMax int64 = 100
)

// PortProfilePriorityQueue2LevelMin and PortProfilePriorityQueue2LevelMax are the inclusive bounds the controller accepts for PortProfile.priority_queue2_level.
const (
	PortProfilePriorityQueue2LevelMin int64 = 0
	PortProfilePriorityQueue2LevelMax int64 = 100
)

// PortProfilePriorityQueue3LevelMin and PortProfilePriorityQueue3LevelMax are the inclusive bounds the controller accepts for PortProfile.priority_queue3_level.
const (
	PortProfilePriorityQueue3LevelMin int64 = 0
	PortProfilePriorityQueue3LevelMax int64 = 100
)

// PortProfilePriorityQueue4LevelMin and PortProfilePriorityQueue4LevelMax are the inclusive bounds the controller accepts for PortProfile.priority_queue4_level.
const (
	PortProfilePriorityQueue4LevelMin int64 = 0
	PortProfilePriorityQueue4LevelMax int64 = 100
)

// PortProfileSettingPreferenceValues are the values the controller accepts for PortProfile.setting_preference.
var PortProfileSettingPreferenceValues = []string{"auto", "manual"}

// PortProfileSpeedValues are the values the controller accepts for PortProfile.speed.
var PortProfileSpeedValues = []int64{10, 100, 1000, 2500, 5000, 10000, 20000, 25000, 40000, 50000, 100000}

// PortProfileStormctrlBroadcastastLevelMin and PortProfileStormctrlBroadcastastLevelMax are the inclusive bounds the controller accepts for PortProfile.stormctrl_bcast_level.
const (
	PortProfileStormctrlBroadcastastLevelMin int64 = 0
	PortProfileStormctrlBroadcastastLevelMax int64 = 100
)

// PortProfileStormctrlMcastLevelMin and PortProfileStormctrlMcastLevelMax are the inclusive bounds the controller accepts for PortProfile.stormctrl_mcast_level.
const (
	PortProfileStormctrlMcastLevelMin int64 = 0
	PortProfileStormctrlMcastLevelMax int64 = 100
)

// PortProfileStormctrlTypeValues are the values the controller accepts for PortProfile.stormctrl_type.
var PortProfileStormctrlTypeValues = []string{"level", "rate"}

// PortProfileStormctrlUcastLevelMin and PortProfileStormctrlUcastLevelMax are the inclusive bounds the controller accepts for PortProfile.stormctrl_ucast_level.
const (
	PortProfileStormctrlUcastLevelMin int64 = 0
	PortProfileStormctrlUcastLevelMax int64 = 100
)

// PortProfileStpEdgeStateValues are the values the controller accepts for PortProfile.stp_edge_state.
var PortProfileStpEdgeStateValues = []string{"auto", "enabled", "disabled"}

// PortProfileTaggedVLANMgmtValues are the values the controller accepts for PortProfile.tagged_vlan_mgmt.
var PortProfileTaggedVLANMgmtValues = []string{"auto", "block_all", "custom"}

// PortProfileQOSMarkingCosCodeMin and PortProfileQOSMarkingCosCodeMax are the inclusive bounds the controller accepts for PortProfileQOSMarking.cos_code.
const (
	PortProfileQOSMarkingCosCodeMin int64 = 0
	PortProfileQOSMarkingCosCodeMax int64 = 7
)

// PortProfileQOSMarkingDscpCodeValues are the values the controller accepts for PortProfileQOSMarking.dscp_code.
var PortProfileQOSMarkingDscpCodeValues = []int64{0, 8, 16, 24, 32, 40, 48, 56, 10, 12, 14, 18, 20, 22, 26, 28, 30, 34, 36, 38, 44, 46}

// PortProfileQOSMarkingIPPrecedenceCodeMin and PortProfileQOSMarkingIPPrecedenceCodeMax are the inclusive bounds the controller accepts for PortProfileQOSMarking.ip_precedence_code.
const (
	PortProfileQOSMarkingIPPrecedenceCodeMin int64 = 0
	PortProfileQOSMarkingIPPrecedenceCodeMax int64 = 7
)

// PortProfileQOSMarkingQueueMin and PortProfileQOSMarkingQueueMax are the inclusive bounds the controller accepts for PortProfileQOSMarking.queue.
const (
	PortProfileQOSMarkingQueueMin int64 = 0
	PortProfileQOSMarkingQueueMax int64 = 7
)

// PortProfileQOSMatchingCosCodeMin and PortProfileQOSMatchingCosCodeMax are the inclusive bounds the controller accepts for PortProfileQOSMatching.cos_code.
const (
	PortProfileQOSMatchingCosCodeMin int64 = 0
	PortProfileQOSMatchingCosCodeMax int64 = 7
)

// PortProfileQOSMatchingDscpCodeMin and PortProfileQOSMatchingDscpCodeMax are the inclusive bounds the controller accepts for PortProfileQOSMatching.dscp_code.
const (
	PortProfileQOSMatchingDscpCodeMin int64 = 0
	PortProfileQOSMatchingDscpCodeMax int64 = 63
)

// PortProfileQOSMatchingDstPortMin and PortProfileQOSMatchingDstPortMax are the inclusive bounds the controller accepts for PortProfileQOSMatching.dst_port.
const (
	PortProfileQOSMatchingDstPortMin int64 = 0
	PortProfileQOSMatchingDstPortMax int64 = 65535
)

// PortProfileQOSMatchingIPPrecedenceCodeMin and PortProfileQOSMatchingIPPrecedenceCodeMax are the inclusive bounds the controller accepts for PortProfileQOSMatching.ip_precedence_code.
const (
	PortProfileQOSMatchingIPPrecedenceCodeMin int64 = 0
	PortProfileQOSMatchingIPPrecedenceCodeMax int64 = 7
)

// PortProfileQOSMatchingSrcPortMin and PortProfileQOSMatchingSrcPortMax are the inclusive bounds the controller accepts for PortProfileQOSMatching.src_port.
const (
	PortProfileQOSMatchingSrcPortMin int64 = 0
	PortProfileQOSMatchingSrcPortMax int64 = 65535
)

// PortProfileQOSProfileQOSProfileModeValues are the values the controller accepts for PortProfileQOSProfile.qos_profile_mode.
var PortProfileQOSProfileQOSProfileModeValues = []string{"custom", "unifi_play", "aes67_audio", "crestron_audio_video", "dante_audio", "ndi_aes67_audio", "ndi_dante_audio", "qsys_audio_video", "qsys_video_dante_audio", "sdvoe_aes67_audio", "sdvoe_dante_audio", "shure_audio"}

// RADIUSProfileInterimUpdateIntervalMin and RADIUSProfileInterimUpdateIntervalMax are the inclusive bounds the controller accepts for RADIUSProfile.interim_update_interval.
const (
	RADIUSProfileInterimUpdateIntervalMin int64 = 60
	RADIUSProfileInterimUpdateIntervalMax int64 = 86400
)

// RADIUSProfileNameMinLength and RADIUSProfileNameMaxLength are the character-count bounds the controller accepts for RADIUSProfile.name.
const (
	RADIUSProfileNameMinLength int64 = 1
	RADIUSProfileNameMaxLength int64 = 128
)

// RADIUSProfileVLANWLANModeValues are the values the controller accepts for RADIUSProfile.vlan_wlan_mode.
var RADIUSProfileVLANWLANModeValues = []string{"disabled", "optional", "required"}

// RADIUSProfileAcctServersPortMin and RADIUSProfileAcctServersPortMax are the inclusive bounds the controller accepts for RADIUSProfileAcctServers.port.
const (
	RADIUSProfileAcctServersPortMin int64 = 1
	RADIUSProfileAcctServersPortMax int64 = 65535
)

// RADIUSProfileAuthServersPortMin and RADIUSProfileAuthServersPortMax are the inclusive bounds the controller accepts for RADIUSProfileAuthServers.port.
const (
	RADIUSProfileAuthServersPortMin int64 = 1
	RADIUSProfileAuthServersPortMax int64 = 65535
)

// RoutingGatewayTypeValues are the values the controller accepts for Routing.gateway_type.
var RoutingGatewayTypeValues = []string{"default", "switch"}

// RoutingNameMinLength and RoutingNameMaxLength are the character-count bounds the controller accepts for Routing.name.
const (
	RoutingNameMinLength int64 = 1
	RoutingNameMaxLength int64 = 128
)

// RoutingStaticRouteDistanceMin and RoutingStaticRouteDistanceMax are the inclusive bounds the controller accepts for Routing.static-route_distance.
const (
	RoutingStaticRouteDistanceMin int64 = 1
	RoutingStaticRouteDistanceMax int64 = 255
)

// RoutingStaticRouteTypeValues are the values the controller accepts for Routing.static-route_type.
var RoutingStaticRouteTypeValues = []string{"nexthop-route", "interface-route", "blackhole"}

// SpatialRecordNameMinLength and SpatialRecordNameMaxLength are the character-count bounds the controller accepts for SpatialRecord.name.
const (
	SpatialRecordNameMinLength int64 = 1
	SpatialRecordNameMaxLength int64 = 128
)

// TrafficRouteDescriptionMinLength and TrafficRouteDescriptionMaxLength are the character-count bounds the controller accepts for TrafficRoute.description.
const (
	TrafficRouteDescriptionMinLength int64 = 0
	TrafficRouteDescriptionMaxLength int64 = 128
)

// TrafficRouteMatchingTargetValues are the values the controller accepts for TrafficRoute.matching_target.
var TrafficRouteMatchingTargetValues = []string{"DOMAIN", "IP", "INTERNET"}

// TrafficRouteDomainsDomainMinLength and TrafficRouteDomainsDomainMaxLength are the character-count bounds the controller accepts for TrafficRouteDomains.domain.
const (
	TrafficRouteDomainsDomainMinLength int64 = 1
	TrafficRouteDomainsDomainMaxLength int64 = 256
)

// TrafficRouteDomainsPortsMin and TrafficRouteDomainsPortsMax are the inclusive bounds the controller accepts for TrafficRouteDomains.ports.
const (
	TrafficRouteDomainsPortsMin int64 = 1
	TrafficRouteDomainsPortsMax int64 = 99999
)

// TrafficRouteIPAddressesVersionValues are the values the controller accepts for TrafficRouteIPAddresses.ip_version.
var TrafficRouteIPAddressesVersionValues = []string{"v4", "v6"}

// TrafficRouteIPAddressesPortsMin and TrafficRouteIPAddressesPortsMax are the inclusive bounds the controller accepts for TrafficRouteIPAddresses.ports.
const (
	TrafficRouteIPAddressesPortsMin int64 = 1
	TrafficRouteIPAddressesPortsMax int64 = 99999
)

// TrafficRouteIPRangesVersionValues are the values the controller accepts for TrafficRouteIPRanges.ip_version.
var TrafficRouteIPRangesVersionValues = []string{"v4", "v6"}

// TrafficRoutePortRangesStartMin and TrafficRoutePortRangesStartMax are the inclusive bounds the controller accepts for TrafficRoutePortRanges.port_start.
const (
	TrafficRoutePortRangesStartMin int64 = 1
	TrafficRoutePortRangesStartMax int64 = 99999
)

// TrafficRoutePortRangesStopMin and TrafficRoutePortRangesStopMax are the inclusive bounds the controller accepts for TrafficRoutePortRanges.port_stop.
const (
	TrafficRoutePortRangesStopMin int64 = 1
	TrafficRoutePortRangesStopMax int64 = 99999
)

// TrafficRouteTargetDevicesTypeValues are the values the controller accepts for TrafficRouteTargetDevices.type.
var TrafficRouteTargetDevicesTypeValues = []string{"ALL_CLIENTS", "CLIENT", "NETWORK"}

// WLANApGroupModeValues are the values the controller accepts for WLAN.ap_group_mode.
var WLANApGroupModeValues = []string{"all", "groups", "devices"}

// WLANDNSAssistanceModeValues are the values the controller accepts for WLAN.dns_assistance_mode.
var WLANDNSAssistanceModeValues = []string{"off", "auto", "manual"}

// WLANDTIM6EMin and WLANDTIM6EMax are the inclusive bounds the controller accepts for WLAN.dtim_6e.
const (
	WLANDTIM6EMin int64 = 1
	WLANDTIM6EMax int64 = 255
)

// WLANDTIMModeValues are the values the controller accepts for WLAN.dtim_mode.
var WLANDTIMModeValues = []string{"default", "custom"}

// WLANDTIMNaMin and WLANDTIMNaMax are the inclusive bounds the controller accepts for WLAN.dtim_na.
const (
	WLANDTIMNaMin int64 = 1
	WLANDTIMNaMax int64 = 255
)

// WLANDTIMNgMin and WLANDTIMNgMax are the inclusive bounds the controller accepts for WLAN.dtim_ng.
const (
	WLANDTIMNgMin int64 = 1
	WLANDTIMNgMax int64 = 255
)

// WLANMACFilterPolicyValues are the values the controller accepts for WLAN.mac_filter_policy.
var WLANMACFilterPolicyValues = []string{"allow", "deny"}

// WLANMdnsProxyModeValues are the values the controller accepts for WLAN.mdns_proxy_mode.
var WLANMdnsProxyModeValues = []string{"off", "auto", "custom"}

// WLANMinrateSettingPreferenceValues are the values the controller accepts for WLAN.minrate_setting_preference.
var WLANMinrateSettingPreferenceValues = []string{"auto", "manual"}

// WLANNameMinLength and WLANNameMaxLength are the character-count bounds the controller accepts for WLAN.name.
const (
	WLANNameMinLength int64 = 1
	WLANNameMaxLength int64 = 32
)

// WLANNameCombineSuffixMinLength and WLANNameCombineSuffixMaxLength are the character-count bounds the controller accepts for WLAN.name_combine_suffix.
const (
	WLANNameCombineSuffixMinLength int64 = 0
	WLANNameCombineSuffixMaxLength int64 = 8
)

// WLANNasIDentifierMinLength and WLANNasIDentifierMaxLength are the character-count bounds the controller accepts for WLAN.nas_identifier.
const (
	WLANNasIDentifierMinLength int64 = 0
	WLANNasIDentifierMaxLength int64 = 48
)

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

// WLANRoamClusterIDMin and WLANRoamClusterIDMax are the inclusive bounds the controller accepts for WLAN.roam_cluster_id.
const (
	WLANRoamClusterIDMin int64 = 0
	WLANRoamClusterIDMax int64 = 31
)

// WLANRoamingAssistant6ERssiMin and WLANRoamingAssistant6ERssiMax are the inclusive bounds the controller accepts for WLAN.roaming_assistant_6e_rssi.
const (
	WLANRoamingAssistant6ERssiMin int64 = -90
	WLANRoamingAssistant6ERssiMax int64 = -70
)

// WLANRoamingAssistantNaRssiMin and WLANRoamingAssistantNaRssiMax are the inclusive bounds the controller accepts for WLAN.roaming_assistant_na_rssi.
const (
	WLANRoamingAssistantNaRssiMin int64 = -80
	WLANRoamingAssistantNaRssiMax int64 = -60
)

// WLANSecurityValues are the values the controller accepts for WLAN.security.
var WLANSecurityValues = []string{"open", "wpapsk", "wep", "wpaeap", "osen"}

// WLANSettingPreferenceValues are the values the controller accepts for WLAN.setting_preference.
var WLANSettingPreferenceValues = []string{"auto", "manual"}

// WLANVLANMin and WLANVLANMax are the inclusive bounds the controller accepts for WLAN.vlan.
const (
	WLANVLANMin int64 = 2
	WLANVLANMax int64 = 4095
)

// WLANWEPIDXMin and WLANWEPIDXMax are the inclusive bounds the controller accepts for WLAN.wep_idx.
const (
	WLANWEPIDXMin int64 = 1
	WLANWEPIDXMax int64 = 4
)

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

// WLANCapabPortMin and WLANCapabPortMax are the inclusive bounds the controller accepts for WLANCapab.port.
const (
	WLANCapabPortMin int64 = 0
	WLANCapabPortMax int64 = 65535
)

// WLANCapabProtocolValues are the values the controller accepts for WLANCapab.protocol.
var WLANCapabProtocolValues = []string{"icmp", "tcp_udp", "tcp", "udp", "esp"}

// WLANCapabStatusValues are the values the controller accepts for WLANCapab.status.
var WLANCapabStatusValues = []string{"closed", "open", "unknown"}

// WLANCellularNetworkListCountryCodeMin and WLANCellularNetworkListCountryCodeMax are the inclusive bounds the controller accepts for WLANCellularNetworkList.country_code.
const (
	WLANCellularNetworkListCountryCodeMin int64 = 1
	WLANCellularNetworkListCountryCodeMax int64 = 9999
)

// WLANCellularNetworkListNameMinLength and WLANCellularNetworkListNameMaxLength are the character-count bounds the controller accepts for WLANCellularNetworkList.name.
const (
	WLANCellularNetworkListNameMinLength int64 = 1
	WLANCellularNetworkListNameMaxLength int64 = 128
)

// WLANFriendlyNameTextMinLength and WLANFriendlyNameTextMaxLength are the character-count bounds the controller accepts for WLANFriendlyName.text.
const (
	WLANFriendlyNameTextMinLength int64 = 1
	WLANFriendlyNameTextMaxLength int64 = 128
)

// WLANGroupNameMinLength and WLANGroupNameMaxLength are the character-count bounds the controller accepts for WLANGroup.name.
const (
	WLANGroupNameMinLength int64 = 1
	WLANGroupNameMaxLength int64 = 128
)

// WLANHotspot2DomainNameListMinLength and WLANHotspot2DomainNameListMaxLength are the character-count bounds the controller accepts for WLANHotspot2.domain_name_list.
const (
	WLANHotspot2DomainNameListMinLength int64 = 1
	WLANHotspot2DomainNameListMaxLength int64 = 128
)

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

// WLANNaiRealmListNameMinLength and WLANNaiRealmListNameMaxLength are the character-count bounds the controller accepts for WLANNaiRealmList.name.
const (
	WLANNaiRealmListNameMinLength int64 = 1
	WLANNaiRealmListNameMaxLength int64 = 128
)

// WLANPredefinedServicesCodeValues are the values the controller accepts for WLANPredefinedServices.code.
var WLANPredefinedServicesCodeValues = []string{"amazon_devices", "android_tv_remote", "apple_airDrop", "apple_airPlay", "apple_file_sharing", "apple_iChat", "apple_iTunes", "aqara", "bose", "dns_service_discovery", "ftp_servers", "google_chromecast", "homeKit", "matter_network", "philips_hue", "printers", "roku", "scanners", "sonos", "spotify_connect", "ssh_servers", "time_capsule", "web_servers", "windows_file_sharing_samba"}

// WLANRoamingConsortiumListNameMinLength and WLANRoamingConsortiumListNameMaxLength are the character-count bounds the controller accepts for WLANRoamingConsortiumList.name.
const (
	WLANRoamingConsortiumListNameMinLength int64 = 1
	WLANRoamingConsortiumListNameMaxLength int64 = 128
)

// WLANRoamingConsortiumListOidMinLength and WLANRoamingConsortiumListOidMaxLength are the character-count bounds the controller accepts for WLANRoamingConsortiumList.oid.
const (
	WLANRoamingConsortiumListOidMinLength int64 = 1
	WLANRoamingConsortiumListOidMaxLength int64 = 128
)

// WLANSaePskIDMinLength and WLANSaePskIDMaxLength are the character-count bounds the controller accepts for WLANSaePsk.id.
const (
	WLANSaePskIDMinLength int64 = 0
	WLANSaePskIDMaxLength int64 = 128
)

// WLANSaePskVLANMin and WLANSaePskVLANMax are the inclusive bounds the controller accepts for WLANSaePsk.vlan.
const (
	WLANSaePskVLANMin int64 = 0
	WLANSaePskVLANMax int64 = 4095
)

// WLANScheduleWithDurationStartDaysOfWeekValues are the values the controller accepts for WLANScheduleWithDuration.start_days_of_week.
var WLANScheduleWithDurationStartDaysOfWeekValues = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// WLANScheduleWithDurationStartHourMin and WLANScheduleWithDurationStartHourMax are the inclusive bounds the controller accepts for WLANScheduleWithDuration.start_hour.
const (
	WLANScheduleWithDurationStartHourMin int64 = 0
	WLANScheduleWithDurationStartHourMax int64 = 23
)

// WLANScheduleWithDurationStartMinuteMin and WLANScheduleWithDurationStartMinuteMax are the inclusive bounds the controller accepts for WLANScheduleWithDuration.start_minute.
const (
	WLANScheduleWithDurationStartMinuteMin int64 = 0
	WLANScheduleWithDurationStartMinuteMax int64 = 59
)
