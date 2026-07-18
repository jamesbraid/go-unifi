// Code generated from ace.jar fields *.json files
// DO NOT EDIT.

package settings

func generatedSettingKey(setting Setting) (string, bool) {
	switch setting.(type) {
	case *AutoSpeedtest:
		return "auto_speedtest", true
	case *Baresip:
		return "baresip", true
	case *Broadcast:
		return "broadcast", true
	case *Connectivity:
		return "connectivity", true
	case *Country:
		return "country", true
	case *Dashboard:
		return "dashboard", true
	case *DeviceSupervision:
		return "device_supervision", true
	case *Doh:
		return "doh", true
	case *Dpi:
		return "dpi", true
	case *ElementAdopt:
		return "element_adopt", true
	case *EtherLighting:
		return "ether_lighting", true
	case *GlobalAp:
		return "global_ap", true
	case *GlobalNat:
		return "global_nat", true
	case *GlobalSwitch:
		return "global_switch", true
	case *GuestAccess:
		return "guest_access", true
	case *IgmpSnooping:
		return "igmp_snooping", true
	case *Ips:
		return "ips", true
	case *IpsSuppression:
		return "ips_suppression", true
	case *Lcm:
		return "lcm", true
	case *Locale:
		return "locale", true
	case *MagicSiteToSiteVpn:
		return "magic_site_to_site_vpn", true
	case *Mdns:
		return "mdns", true
	case *Mgmt:
		return "mgmt", true
	case *Netflow:
		return "netflow", true
	case *NetworkOptimization:
		return "network_optimization", true
	case *Ntp:
		return "ntp", true
	case *Porta:
		return "porta", true
	case *RadioAi:
		return "radio_ai", true
	case *Radius:
		return "radius", true
	case *Rsyslogd:
		return "rsyslogd", true
	case *Snmp:
		return "snmp", true
	case *SslInspection:
		return "ssl_inspection", true
	case *SuperCloudaccess:
		return "super_cloudaccess", true
	case *SuperEvents:
		return "super_events", true
	case *SuperFwupdate:
		return "super_fwupdate", true
	case *SuperIdentity:
		return "super_identity", true
	case *SuperMail:
		return "super_mail", true
	case *SuperMgmt:
		return "super_mgmt", true
	case *SuperSdn:
		return "super_sdn", true
	case *SuperSmtp:
		return "super_smtp", true
	case *Teleport:
		return "teleport", true
	case *TrafficFlow:
		return "traffic_flow", true
	case *Usg:
		return "usg", true
	case *UsgGeo:
		return "usg_geo", true
	case *Usw:
		return "usw", true
	default:
		return "", false
	}
}
