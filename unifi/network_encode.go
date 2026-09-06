package unifi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/netip"
	"reflect"
	"slices"
	"strings"
	"sync"
)

const (
	PurposeCorporate = "corporate"
	PurposeGuest     = "guest"
	PurposeVLANOnly  = "vlan-only"
	PurposeWAN       = "wan"
	PurposeSiteVPN   = "site-vpn"
	PurposeVPNClient = "vpn-client"
	PurposeUserVPN   = "remote-user-vpn"
)

// MarshalJSON writes only the fields relevant to the network's Purpose.
//
// Which fields a purpose sends is the per-purpose lists below: hand-kept
// data, because the controller's own definitions carry no per-purpose
// applicability, so every entry is a measurement by usage or by probe. HOW a
// listed field is sent is not hand-kept: the emission rule is derived from
// the generated struct's own declaration (type and omitempty), plus the
// measured exception tables and the per-purpose derivations below. A field
// added by regeneration therefore needs exactly one decision -- which
// purposes send it -- and its wire behaviour follows the schema.
func (n *Network) MarshalJSON() ([]byte, error) {
	fields := networkPurposeFields[n.Purpose]
	if fields == nil {
		return nil, fmt.Errorf("unknown network purpose: %s", n.Purpose)
	}
	overrides := n.networkPurposeOverrides()
	byWire := networkFieldByWire()
	v := reflect.ValueOf(n).Elem()

	var buf bytes.Buffer
	buf.WriteByte('{')
	for _, wire := range fields {
		var value any
		var emit bool
		if override, ok := overrides[wire]; ok {
			value, emit = override()
		} else {
			field, ok := byWire[wire]
			if !ok {
				return nil, fmt.Errorf("network purpose %s lists field %q, which is not on the generated Network struct", n.Purpose, wire)
			}
			value, emit = networkFieldValue(wire, v.FieldByIndex(field.index), field.omitEmpty)
		}
		if !emit {
			continue
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("marshal network field %s: %w", wire, err)
		}
		if buf.Len() > 1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('"')
		buf.WriteString(wire)
		buf.WriteString(`":`)
		buf.Write(raw)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// Per-purpose field lists. Order is the wire order, pinned by the corpus
// golden (testdata/network_encode_corpus.txt).
//
// networkCommonFields is the controller envelope plus the identity every
// purpose sends.
var networkCommonFields = []string{
	"_id", "site_id",
	"attr_hidden", "attr_hidden_id", "attr_no_delete", "attr_no_edit",
	"name", "purpose", "enabled",
}

// networkCorporateFields is what a Corporate/LAN network sends.
//
// The dhcpd_ip_1..3 / dhcpd_mac_1..3 DHCP-guard slots ride along
// unconditionally, exactly as the generated struct declares them (plain
// string, no omitempty). Measured on 10.4.57: with dhcpguard_enabled true
// the controller rejects any write whose dhcpd_ip_1 is absent with
// api.err.MissingIPAddress, so omitting the slots made every PUT on a
// guarded network fail, not just creates.
var networkCorporateFields = slices.Concat(networkCommonFields, []string{
	"networkgroup", "ip_subnet", "vlan", "vlan_enabled",
	"l3_interface_type", "routed_port_idx", "routed_lag_idx",
	"domain_name", "auto_scale_enabled", "gateway_type",
	"internet_access_enabled", "network_isolation_enabled",
	"setting_preference", "firewall_zone_id",
	"igmp_snooping", "igmp_fastleave", "igmp_flood_unknown_multicast",
	"igmp_groupmembership", "igmp_maxresponse", "igmp_mcrtrexpiretime",
	"igmp_querier_switches", "igmp_supression",
	"dhcpguard_enabled",
	"dhcpd_ip_1", "dhcpd_ip_2", "dhcpd_ip_3",
	"dhcpd_mac_1", "dhcpd_mac_2", "dhcpd_mac_3",
	"mdns_enabled", "lte_lan_enabled", "upnp_lan_enabled",
	"ip_aliases", "ipv6_aliases", "nat_outbound_ip_addresses",
	"mac_override", "mac_override_enabled",

	// DHCP server
	"dhcpd_enabled", "dhcpd_start", "dhcpd_stop", "dhcpd_leasetime",
	"dhcpd_dns_enabled", "dhcpd_dns_1", "dhcpd_dns_2", "dhcpd_dns_3", "dhcpd_dns_4",
	"dhcpd_gateway_enabled", "dhcpd_gateway",
	"dhcpd_ntp_enabled", "dhcpd_ntp_1", "dhcpd_ntp_2",
	"dhcpd_wins_enabled", "dhcpd_wins_1", "dhcpd_wins_2",
	"dhcpd_time_offset_enabled", "dhcpd_time_offset",
	"dhcpd_conflict_checking",
	"dhcpd_boot_enabled", "dhcpd_boot_server", "dhcpd_boot_filename",
	"dhcpd_tftp_server", "dhcpd_wpad_url", "dhcpd_unifi_controller",

	// DHCP relay
	"dhcp_relay_enabled", "dhcp_relay_servers",

	// IPv6
	"ipv6_interface_type", "ipv6_client_address_assignment",
	"ipv6_setting_preference", "ipv6_ra_priority", "ipv6_subnet",
	"ipv6_ra_enabled", "ipv6_ra_preferred_lifetime", "ipv6_ra_valid_lifetime",
	"ipv6_pd_interface", "ipv6_pd_prefixid", "ipv6_pd_start", "ipv6_pd_stop",
	"ipv6_pd_auto_prefixid_enabled",
	"ipv6_single_network_interface", "single_network_lan",

	// DHCPv6
	"dhcpdv6_enabled", "dhcpdv6_dns_auto",
	"dhcpdv6_dns_1", "dhcpdv6_dns_2", "dhcpdv6_dns_3", "dhcpdv6_dns_4",
	"dhcpdv6_allow_slaac", "dhcpdv6_start", "dhcpdv6_stop", "dhcpdv6_leasetime",
})

// networkGuestFields is derived rather than listed: a guest-purpose probe
// pass (TestIntegrationGuestParityProbe) confirmed the controller persists
// the same advanced fields on a guest network as on a corporate one. The
// ipv6 single-network pair is the exception -- it was not part of the guest
// probe and is corporate-specific ipv6 addressing.
var networkGuestFields = withoutNetworkFields(networkCorporateFields,
	"ipv6_single_network_interface", "single_network_lan")

// networkVLANOnlyFields is what a VLAN-only network (Layer 2, no routing)
// sends.
var networkVLANOnlyFields = slices.Concat(networkCommonFields, []string{
	"networkgroup", "vlan", "vlan_enabled",
	"igmp_snooping", "network_isolation_enabled", "mdns_enabled",
	"dhcpguard_enabled",
	"dhcpd_ip_1", "dhcpd_ip_2", "dhcpd_ip_3",
	"dhcpd_mac_1", "dhcpd_mac_2", "dhcpd_mac_3",
})

// networkWANFields is what a WAN network sends. ipv6_enabled exists nowhere
// on the generated struct; it is synthesized from wan_type_v6 by the WAN
// overrides below.
var networkWANFields = slices.Concat(networkCommonFields, []string{
	"setting_preference", "ipv6_setting_preference",
	"wan_type", "wan_type_v6", "wan_networkgroup",

	// Static addressing (wan_type "static" / wan_type_v6 "static")
	"wan_ip", "wan_netmask", "wan_gateway",
	"wan_ipv6", "wan_gateway_v6", "wan_prefixlen",

	// PPPoE credentials (wan_type "pppoe")
	"wan_username", "x_wan_password",
	"wan_pppoe_username_enabled", "wan_pppoe_password_enabled",

	// DS-Lite (wan_type "dslite")
	"wan_dslite_remote_host", "wan_dslite_remote_host_auto",

	"interface_mtu", "interface_mtu_enabled",
	"wan_vlan_enabled", "wan_vlan",
	"wan_dhcp_cos", "wan_dhcpv6_cos",
	"wan_dns1", "wan_dns2", "wan_dns_preference",
	"wan_ipv6_dns1", "wan_ipv6_dns2", "wan_ipv6_dns_preference",
	"wan_dhcpv6_pd_size", "wan_dhcpv6_pd_size_auto", "wan_dhcpv6_options",
	"ipv6_wan_delegation_type", "ipv6_enabled",
	"wan_egress_qos_enabled", "wan_egress_qos",
	"wan_smartq_enabled", "wan_smartq_up_rate", "wan_smartq_down_rate",
	"mss_clamp", "mss_clamp_mss", "mss_clamp_ipv6", "mss_clamp_mss_ipv6",
	"upnp_enabled", "upnp_wan_interface", "upnp_nat_pmp_enabled", "upnp_secure_mode",
	"wan_load_balance_type", "wan_load_balance_weight", "wan_failover_priority",
	"igmp_proxy_for", "igmp_proxy_upstream",
	"report_wan_event", "wan_ip_aliases", "wan_dhcp_options",
	"wan_provider_capabilities",
})

// networkSiteVPNFields is what a site-to-site IPsec VPN network sends. The
// IKE phase-1 parameters go under the legacy names ipsec_encryption /
// ipsec_hash / ipsec_dh_group, not the newer ipsec_ike_* spellings.
var networkSiteVPNFields = slices.Concat(networkCommonFields, []string{
	"vpn_type",
	"ipsec_interface", "ipsec_peer_ip", "ipsec_local_ip",
	"ipsec_key_exchange", "x_ipsec_pre_shared_key", "ipsec_profile",

	// IKE (phase 1)
	"ipsec_encryption", "ipsec_hash", "ipsec_dh_group", "ipsec_ike_lifetime",

	// IKE peer identifiers and per-child-SA networks (policy-based mode)
	"ipsec_local_identifier", "ipsec_local_identifier_enabled",
	"ipsec_remote_identifier", "ipsec_remote_identifier_enabled",
	"ipsec_separate_ikev2_networks",

	// ESP (phase 2)
	"ipsec_esp_encryption", "ipsec_esp_hash", "ipsec_esp_dh_group", "ipsec_esp_lifetime",

	"ipsec_pfs", "ipsec_dynamic_routing",
	"remote_vpn_subnets", "remote_site_subnets", "route_distance",
})

// networkVPNClientFields is what a VPN client (WireGuard client) network
// sends. dhcpd_dns_enabled is a plain bool sent on every write like any
// other: omitting it turned the flag off on every read-modify-write.
var networkVPNClientFields = slices.Concat(networkCommonFields, []string{
	"ip_subnet", "vpn_type",
	"vpn_client_default_route", "vpn_client_pull_dns",
	"wireguard_client_mode",
	"wireguard_client_configuration_file", "wireguard_client_configuration_filename",
	"wireguard_client_peer_ip", "wireguard_client_peer_port",
	"wireguard_client_peer_public_key",
	"wireguard_client_preshared_key_enabled", "wireguard_client_preshared_key",
	"wireguard_interface", "x_wireguard_private_key",
	"dhcpd_dns_1", "dhcpd_dns_2", "dhcpd_dns_3", "dhcpd_dns_4", "dhcpd_dns_enabled",
})

// networkUserVPNFields is what a remote-user VPN network sends. mss_clamp is
// here because live 10.4.57 controllers report it on remote-user-vpn
// networks (tunnel MTU), not only on WANs.
var networkUserVPNFields = slices.Concat(networkCommonFields, []string{
	"setting_preference", "ip_subnet", "vpn_type",
	"vpn_binding_mode",
	"mss_clamp", "mss_clamp_mss", "mss_clamp_ipv6", "mss_clamp_mss_ipv6",
	"dhcpd_dns_1", "dhcpd_dns_2", "dhcpd_dns_3", "dhcpd_dns_4", "dhcpd_dns_enabled",
	"dhcpd_start", "dhcpd_stop",
	"radiusprofile_id",

	// WireGuard server
	"wireguard_interface", "x_wireguard_private_key", "wireguard_local_wan_ip",
	"local_port", "wireguard_interface_binding_mode_ip_version",
	"vpn_client_configuration_remote_ip_override",
	"vpn_client_configuration_remote_ip_override_enabled",

	// L2TP server
	"l2tp_interface", "l2tp_local_wan_ip", "l2tp_allow_weak_ciphers",
	"x_ipsec_pre_shared_key", "require_mschapv2",

	// OpenVPN server
	"openvpn_interface", "openvpn_local_wan_ip", "openvpn_mode",
	"openvpn_encryption_cipher", "vpn_protocol",
	"x_server_crt", "x_server_key", "x_dh_key",
	"x_shared_client_key", "x_shared_client_crt",
	"x_auth_key", "x_ca_crt", "x_ca_key",
})

var networkPurposeFields = map[string][]string{
	PurposeCorporate: networkCorporateFields,
	PurposeGuest:     networkGuestFields,
	PurposeVLANOnly:  networkVLANOnlyFields,
	PurposeWAN:       networkWANFields,
	PurposeSiteVPN:   networkSiteVPNFields,
	PurposeVPNClient: networkVPNClientFields,
	PurposeUserVPN:   networkUserVPNFields,
}

// withoutNetworkFields returns fields with the named entries removed,
// preserving order.
func withoutNetworkFields(fields []string, drop ...string) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if !slices.Contains(drop, f) {
			out = append(out, f)
		}
	}
	return out
}

// networkClearableSlots are the *string fields whose explicit "" must reach
// the wire. The blanket rule is the opposite -- an optional string that is
// empty is absent (see TestNetworkEncoderDropsEmptyStrings) -- but these
// eight were measured on 10.6.101 to behave the other way round: omitting
// one PRESERVES the stored value, and "" is what clears it. Dropping the
// empty here is why a caller could not empty a DHCP DNS, NTP or WINS list
// at all.
var networkClearableSlots = map[string]bool{
	"dhcpd_dns_1": true, "dhcpd_dns_2": true, "dhcpd_dns_3": true, "dhcpd_dns_4": true,
	"dhcpd_ntp_1": true, "dhcpd_ntp_2": true,
	"dhcpd_wins_1": true, "dhcpd_wins_2": true,
}

// networkEmptyStringOmitted are plain-string fields the generated struct
// sends unconditionally but the encoder drops when empty. Both await a
// controller measurement of what an explicit "" does; until one exists the
// long-standing omission stands (TestKnownStringOmitemptyDriftIsUnchanged
// in network_encode_drift_test.go keeps this list honest).
var networkEmptyStringOmitted = map[string]bool{
	"dhcpd_boot_server": true,
	"mac_override":      true,
}

// networkAlwaysArrays are slice fields sent as [] even when the caller left
// them nil, against their generated omitempty. The controller wants an
// array for these, and for remote_vpn_subnets it is load-bearing twice
// over, measured on 10.6.101: a site-to-site create without the key is
// refused with api.err.Invalid, and an explicit [] is how
// remote_site_subnets is cleared once it holds something. encoding/json
// drops an empty slice exactly as it drops a nil one, so omitempty could
// never send either.
var networkAlwaysArrays = map[string]bool{
	"ip_aliases":                true,
	"ipv6_aliases":              true,
	"nat_outbound_ip_addresses": true,
	"dhcp_relay_servers":        true,
	"wan_ip_aliases":            true,
	"wan_dhcp_options":          true,
	"remote_vpn_subnets":        true,
	"remote_site_subnets":       true,
}

// networkPurposeOverrides returns the fields whose value this purpose
// derives instead of copying from the struct. Everything here is behaviour
// with a measurement behind it; enumeration stays in the field lists.
func (n *Network) networkPurposeOverrides() map[string]func() (any, bool) {
	switch n.Purpose {
	case PurposeCorporate, PurposeGuest:
		// Derive DHCP range defaults from ip_subnet only for a create,
		// which is the only write without an _id yet. The controller
		// rejects a corporate create without a range, so inventing one
		// there is load-bearing. On an update the same derivation put a
		// range the caller never set and the controller never stored on
		// the wire -- under a masked update it replaced the caller's value
		// for a named dhcpd_start/stop, and a masked write must carry
		// exactly what the mask names.
		var defaultStart, defaultEnd string
		if n.ID == "" && n.IPSubnet != nil {
			var err error
			defaultStart, defaultEnd, err = dhcpRange(*n.IPSubnet)
			if err != nil {
				log.Default().Printf("error calculating DHCP range: %s", err)
			}
		}
		rangeBound := func(caller *string, derived string) func() (any, bool) {
			return func() (any, bool) {
				value := derived
				if caller != nil && *caller != "" {
					value = *caller
				}
				return value, value != ""
			}
		}
		return map[string]func() (any, bool){
			"dhcpd_start": rangeBound(n.DHCPDStart, defaultStart),
			"dhcpd_stop":  rangeBound(n.DHCPDStop, defaultEnd),
		}
	case PurposeVLANOnly:
		return map[string]func() (any, bool){
			// A VLAN id with vlan_enabled false is not a usable config: the
			// controller ignores the id, falls back to VLAN 1, and rejects
			// the create with api.err.VlanUsed naming the Default network --
			// measured on 10.4.57. Turning the flag on with the id is what
			// the caller meant.
			"vlan_enabled": func() (any, bool) {
				return n.VLANEnabled || (n.VLAN != nil && *n.VLAN > 0), true
			},
			// Measured on 10.4.57: a corporate or guest network created
			// without networkgroup comes back with "LAN" anyway, so the
			// default was dropped there. A vlan-only network created
			// without it comes back with no networkgroup at all, so
			// dropping it here would change what is stored rather than
			// restate it.
			"networkgroup": func() (any, bool) {
				if n.NetworkGroup != nil && *n.NetworkGroup != "" {
					return *n.NetworkGroup, true
				}
				return "LAN", true
			},
		}
	case PurposeWAN:
		return map[string]func() (any, bool){
			// ipv6_enabled is derived from wan_type_v6, so both have to
			// read the same value: an empty wan_type_v6 is dropped as
			// unset, and must not leave ipv6_enabled asserting IPv6 off a
			// field that was not sent.
			"ipv6_enabled": func() (any, bool) {
				v6 := n.WANTypeV6
				return v6 != nil && *v6 != "" && *v6 != "disabled", true
			},
		}
	}
	return nil
}

type networkWireField struct {
	index     []int
	omitEmpty bool
}

// networkFieldByWire indexes the generated Network struct by wire name,
// once. The json tag carries each field's own emission contract, which is
// what networkFieldValue applies.
var networkFieldByWire = sync.OnceValue(func() map[string]networkWireField {
	typ := reflect.TypeFor[Network]()
	out := make(map[string]networkWireField, typ.NumField())
	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.PkgPath != "" { // unexported
			continue
		}
		name, opts, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		out[name] = networkWireField{
			index:     field.Index,
			omitEmpty: strings.Contains(opts, "omitempty"),
		}
	}
	return out
})

// networkFieldValue applies the generated declaration's emission rule to one
// field, with the measured exceptions layered on:
//
//   - an optional *string that is empty is absent -- omitting clears the
//     stored value just the same, and several fields reject "" outright
//     (measured by TestIntegrationClearingSemantics) -- except the
//     networkClearableSlots, where "" is the only way to clear;
//   - networkEmptyStringOmitted drops the empty for two unconditional
//     strings still awaiting measurement;
//   - networkAlwaysArrays sends [] where the controller demands an array.
//
// Everything else is exactly what marshalling the generated struct would do.
func networkFieldValue(wire string, fv reflect.Value, omitEmpty bool) (any, bool) {
	switch fv.Kind() {
	case reflect.Pointer:
		if fv.IsNil() {
			return nil, !omitEmpty
		}
		if s, ok := fv.Interface().(*string); ok && *s == "" && !networkClearableSlots[wire] {
			return nil, false
		}
		return fv.Interface(), true
	case reflect.String:
		s := fv.String()
		if s == "" && (omitEmpty || networkEmptyStringOmitted[wire]) {
			return nil, false
		}
		return s, true
	case reflect.Bool:
		b := fv.Bool()
		return b, b || !omitEmpty
	case reflect.Slice:
		if networkAlwaysArrays[wire] {
			if fv.Len() == 0 {
				return reflect.MakeSlice(fv.Type(), 0, 0).Interface(), true
			}
			return fv.Interface(), true
		}
		return fv.Interface(), fv.Len() > 0 || !omitEmpty
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := fv.Int()
		return i, i != 0 || !omitEmpty
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u := fv.Uint()
		return u, u != 0 || !omitEmpty
	case reflect.Float32, reflect.Float64:
		f := fv.Float()
		return f, f != 0 || !omitEmpty
	default:
		return fv.Interface(), true
	}
}

// dhcpRange derives the controller's default DHCP range for a subnet.
func dhcpRange(cidr string) (start, end string, err error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return "", "", err
	}

	// Only support IPv4
	if !prefix.Addr().Is4() {
		return "", "", fmt.Errorf("only IPv4 supported")
	}

	networkAddr := prefix.Masked().Addr()
	bits := prefix.Bits()

	// Calculate the number of host addresses
	hostBits := 32 - bits
	numHosts := uint32(1) << hostBits

	// UniFi's rules based on subnet size:
	// /30 or smaller (4 or fewer IPs): No DHCP (too small)
	// /29 (8 IPs): Start at +2, End at -2 (gives 4 usable IPs)
	// /28 to /24: Start at +6, End at -1 (broadcast)
	// /23 and larger: Start at +6, End at -1

	if bits >= 30 {
		return "", "", fmt.Errorf("subnet too small for DHCP (/%d)", bits)
	}

	// Convert network address to uint32 for arithmetic
	ip4 := networkAddr.As4()
	baseIP := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])

	var startOffset, endOffset uint32

	if bits == 29 {
		// /29: 8 IPs total
		// Network: .0, Gateway: .1, DHCP: .2-.5, Reserved: .6, Broadcast: .7
		startOffset = 2
		endOffset = 2
	} else {
		// /28 and larger
		// Network: .0, Gateway: .1, Reserved: .2-.5, DHCP: .6 to (broadcast-1)
		startOffset = 6
		endOffset = 1
	}

	startIP := baseIP + startOffset
	endIP := baseIP + numHosts - 1 - endOffset

	// Convert back to netip.Addr
	start = netip.AddrFrom4([4]byte{
		byte(startIP >> 24),
		byte(startIP >> 16),
		byte(startIP >> 8),
		byte(startIP),
	}).String()

	end = netip.AddrFrom4([4]byte{
		byte(endIP >> 24),
		byte(endIP >> 16),
		byte(endIP >> 8),
		byte(endIP),
	}).String()

	return start, end, nil
}
