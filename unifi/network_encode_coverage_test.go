package unifi

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// This file guards against silent drift between the generated Network struct
// (network.generated.go, regenerated from controller schemas) and the
// hand-written purpose-specific encoder (network_encode.go). The encoder
// enumerates fields explicitly in anonymous structs, so a newly generated
// field is silently dropped from update payloads until it is added by hand.
// TestNetworkEncoderCoversGeneratedFields fails loudly when that happens.

// networkEncoderPurposes lists every purpose value Network.MarshalJSON
// dispatches on. When a new purpose is added to the switch in
// network_encode.go, add it here too.
var networkEncoderPurposes = []string{
	PurposeCorporate,
	PurposeGuest,
	PurposeVLANOnly,
	PurposeWAN,
	PurposeSiteVPN,
	PurposeVPNClient,
	PurposeUserVPN,
}

// networkEncoderPresenceAllowlistTODOs lists generated wire names that were
// probed against a live simulation-mode controller (TestIntegrationNetworkFieldProbe,
// see network_field_probe_integration_test.go) and did NOT come back
// PERSISTED. Every PERSISTED field from the same probe run is wired into the
// matching purpose marshal struct in network_encode.go and removed from this
// slice (the coverage test now requires the encoder to emit it, which is
// self-verifying). What remains here is STRIPPED (controller silently drops
// the field) or REJECTED (controller rejects the create/update), each with
// the measured outcome recorded below. Each entry still has a matching
// fieldCandidate in networkFieldCandidates (network_field_candidates_test.go);
// TestFieldCandidatesCoverAllTODOs keeps the two lists in lockstep.
var networkEncoderPresenceAllowlistTODOs = []string{
	// probe 2026-07 (10.0.162 sim controller): STRIPPED. Create succeeds but
	// the field is absent/empty on read-back regardless of value sent; a real
	// referenced networkconf id might behave differently on real hardware.
	"igmp_proxy_downstream_networkconf_ids",

	// probe 2026-07 (10.0.162 sim controller): REJECTED, api.err.UnrecognizedLocalIp
	// on sibling field ipsec_local_ip. These are route-based (ipsec_dynamic_routing
	// true) site-vpn fields; the controller requires ipsec_local_ip to be a real
	// address bound to an adopted gateway's WAN interface, which this bare
	// container-only simulation (no adopted hardware) cannot provide. Every other
	// prerequisite (PSK, profile, phase 1/2 params, remote subnets, the tunnel_ip
	// value itself) was supplied and did not change the outcome -- see
	// mergeRouteBasedPrereq in network_field_candidates_test.go.
	"ipsec_tunnel_ip",
	"ipsec_tunnel_ip_enabled",
	"remote_vpn_dynamic_subnets_enabled",
}

// networkEncoderAllowlist contains generated wire names that are intentionally
// not emitted by Network.MarshalJSON for any purpose: permanent entries
// justified below, plus every wire name in
// networkEncoderPresenceAllowlistTODOs (fields that look like genuine encoder
// gaps, tracked there instead so they get classified against a live
// controller rather than silently allowlisted forever).
var networkEncoderAllowlist = buildNetworkEncoderAllowlist()

func buildNetworkEncoderAllowlist() map[string]bool {
	allowlist := map[string]bool{
		// Controller-computed / read-only status fields. Reported by the
		// controller, not settable through create/update payloads.
		"gateway_device":       true, // MAC of the gateway device serving the network, assigned by the controller
		"is_nat":               true, // NAT status computed by the controller
		"wireguard_public_key": true, // derived by the controller from x_wireguard_private_key
		"wan_sla":              true, // WAN SLA/monitoring data reported by the controller

		// Legacy IKE phase-1 field aliases. marshalSiteVPN sends phase-1
		// parameters under the legacy names ipsec_encryption / ipsec_hash /
		// ipsec_dh_group instead of these newer ipsec_ike_* spellings.
		"ipsec_ike_dh_group":   true,
		"ipsec_ike_encryption": true,
		"ipsec_ike_hash":       true,

		// Legacy PPTP VPN client. Deprecated by UniFi and not a vpn_type the
		// encoder's vpn-client marshaler (WireGuard only) supports.
		"pptpc_require_mppe":   true,
		"pptpc_route_distance": true,
		"pptpc_server_ip":      true,
		"pptpc_username":       true,
		"x_pptpc_password":     true,

		// Site-to-site / client OpenVPN tunnels. The encoder only manages IPsec
		// site-vpn and WireGuard vpn-client networks; OpenVPN tunnel flavors of
		// vpn_type are not supported, so their config fields are never sent.
		"openvpn_configuration":          true,
		"openvpn_configuration_filename": true,
		"openvpn_local_address":          true,
		"openvpn_local_port":             true,
		"openvpn_remote_address":         true,
		"openvpn_remote_host":            true,
		"openvpn_remote_port":            true,
		"openvpn_username":               true,
		"x_openvpn_password":             true,
		"x_openvpn_shared_secret_key":    true,

		// UniFi Identity (UID) Enterprise VPN. Requires a UID workspace and is
		// provisioned by the controller/UID service, not by this client.
		"uid_policy_enabled":                          true,
		"uid_policy_name":                             true,
		"uid_public_gateway_port":                     true,
		"uid_traffic_rules_allowed_ips_and_hostnames": true,
		"uid_traffic_rules_enabled":                   true,
		"uid_vpn_custom_routing":                      true,
		"uid_vpn_default_dns_suffix":                  true,
		"uid_vpn_masquerade_enabled":                  true,
		"uid_vpn_max_connection_time_seconds":         true,
		"uid_vpn_sync_public_ip":                      true,
		"uid_vpn_type":                                true,
		"uid_workspace_url":                           true,

		// SD-WAN / Site Magic tunnels. These networks are created and managed by
		// the controller's SD-WAN orchestration, not through this encoder.
		"remote_site_id":       true,
		"sdwan_remote_site_id": true,

		// VRRP / shadow-mode gateway HA. Managed by the controller when a
		// secondary gateway is adopted.
		"vrrp_ip_subnet_gw1": true,
		"vrrp_ip_subnet_gw2": true,
		"vrrp_vrid":          true,

		// Legacy per-network DPI restriction assignment; DPI is managed through
		// the dedicated DPI app/group endpoints.
		"dpi_enabled": true,
		"dpigroup_id": true,

		// Legacy trusted-DHCP-server MAC slots paired with dhcpd_ip_1..3; the
		// encoder sends the IP variants (vlan-only) which is what the UI uses.
		"dhcpd_mac_1": true,
		"dhcpd_mac_2": true,
		"dhcpd_mac_3": true,

		// Third/fourth WAN DNS slots; the controller UI exposes only two and the
		// encoder sends wan_dns1/wan_dns2.
		"wan_dns3": true,
		"wan_dns4": true,

		// Legacy classic-UI flag advertising a LAN over auto site-to-site VPN;
		// superseded by explicit site-vpn networks.
		"exposed_to_site_vpn": true,

		// Legacy/rarely used settings not exposed in current controller UIs.
		"priority":     true, // legacy network priority (1-4)
		"usergroup_id": true, // legacy user-group assignment (a WLAN-era concept)
	}
	for _, w := range networkEncoderPresenceAllowlistTODOs {
		allowlist[w] = true
	}
	return allowlist
}

// networkWireNames returns the JSON wire names declared on the generated
// Network struct, with tag options (",omitempty") stripped. Fields with no
// json tag or a "-" tag are skipped.
func networkWireNames(t *testing.T) []string {
	t.Helper()

	typ := reflect.TypeOf(Network{})
	names := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" { // unexported
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// populateNonZero recursively sets v to a non-zero value: strings "x", bools
// true, numbers 1, pointers allocated and populated, slices with one populated
// element, nested structs populated field by field. MarshalJSON does not
// validate field contents, so placeholder values that fail enum or pattern
// validation are fine here. The depth limit guards against unbounded recursion
// through self-referential types.
func populateNonZero(v reflect.Value, depth int) {
	if depth <= 0 || !v.CanSet() {
		return
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		populateNonZero(v.Elem(), depth-1)
	case reflect.Slice:
		elem := reflect.New(v.Type().Elem()).Elem()
		populateNonZero(elem, depth-1)
		v.Set(reflect.Append(reflect.MakeSlice(v.Type(), 0, 1), elem))
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		key := reflect.New(v.Type().Key()).Elem()
		populateNonZero(key, depth-1)
		val := reflect.New(v.Type().Elem()).Elem()
		populateNonZero(val, depth-1)
		m.SetMapIndex(key, val)
		v.Set(m)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			populateNonZero(v.Field(i), depth-1)
		}
	}
}

// networkEmittedKeys marshals a fully-populated Network with the given purpose
// and returns the set of top-level JSON keys the encoder emitted.
func networkEmittedKeys(t *testing.T, purpose string) map[string]bool {
	t.Helper()

	var n Network
	populateNonZero(reflect.ValueOf(&n).Elem(), 10)

	// A parseable subnet keeps the encoder's DHCP-range calculation from
	// logging errors about the "x" placeholder. Coverage is unaffected.
	subnet := "10.0.0.0/24"
	n.IPSubnet = &subnet
	n.Purpose = purpose

	data, err := json.Marshal(&n)
	if err != nil {
		t.Fatalf("marshal Network with purpose %q: %v", purpose, err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal encoder output for purpose %q: %v", purpose, err)
	}

	keys := make(map[string]bool, len(payload))
	for k := range payload {
		keys[k] = true
	}
	return keys
}

// TestNetworkEncoderCoversGeneratedFields fails when a field generated on the
// Network struct is neither emitted by Network.MarshalJSON for any purpose nor
// present in networkEncoderAllowlist. It fires at the first regeneration that
// adds fields the hand-written encoder does not know about.
func TestNetworkEncoderCoversGeneratedFields(t *testing.T) {
	covered := map[string]bool{}
	for _, purpose := range networkEncoderPurposes {
		for key := range networkEmittedKeys(t, purpose) {
			covered[key] = true
		}
	}

	generated := map[string]bool{}
	var missing []string
	for _, name := range networkWireNames(t) {
		generated[name] = true
		if !covered[name] && !networkEncoderAllowlist[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("generated Network fields never emitted by Network.MarshalJSON for any purpose:\n  %s\n\n"+
			"These fields are silently dropped from create/update payloads. Add each one to the\n"+
			"appropriate purpose struct(s) in network_encode.go, or -- if it is intentionally not\n"+
			"sent to the controller -- add it to networkEncoderAllowlist in\n"+
			"network_encode_coverage_test.go with a comment justifying why.",
			strings.Join(missing, "\n  "))
	}

	// Keep the allowlist honest: entries must still exist on the generated
	// struct and must still be uncovered by the encoder.
	var stale []string
	for name := range networkEncoderAllowlist {
		if !generated[name] || covered[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)

	if len(stale) > 0 {
		t.Errorf("stale networkEncoderAllowlist entries (no longer generated, or now emitted by the encoder):\n  %s\n\n"+
			"Remove them from networkEncoderAllowlist in network_encode_coverage_test.go.",
			strings.Join(stale, "\n  "))
	}
}

// ---------------------------------------------------------------------------
// Value-flow check
//
// The presence test above proves every generated wire name is emitted by some
// purpose marshaler, but not that the emitted value comes from the RIGHT
// struct field. A real bug of that shape existed: marshalCorporate emitted
// dhcp_relay_servers sourced from n.RemoteVPNSubnets (correct key, wrong
// source field), which presence coverage cannot catch.
// TestNetworkEncoderValueFlow catches it by giving every generated field a
// value derived from its own wire name and asserting each emitted key carries
// the value of the same-named generated field.

// networkEncoderSyntheticKeys lists keys Network.MarshalJSON emits that do not
// exist on the generated Network struct at all (client-only synthetic keys).
// They have no same-named generated field to compare against, so the value
// check skips them; the map keeps them enumerated so a new unexplained key
// still fails the test. Every entry needs a justification.
var networkEncoderSyntheticKeys = map[string]string{
	// marshalWAN derives ipv6_enabled from wan_type_v6 != "disabled"; the
	// controller schema has no ipv6_enabled field on networks.
	"ipv6_enabled": "synthetic bool derived by marshalWAN from wan_type_v6",
}

// networkEncoderValueFlowExemptions lists emitted keys whose value is
// intentionally NOT a verbatim copy of the same-named generated field for some
// purpose. Every entry needs a justification; cross-sourced values that are
// not clearly intentional must be marked "TODO: possibly a real bug:".
//
// Currently empty. Notably, the legacy IKE phase-1 aliases need no exemption:
// marshalSiteVPN emits ipsec_encryption / ipsec_hash / ipsec_dh_group from
// n.IPSecEncryption / n.IPSecHash / n.IPSecDhGroup, whose generated json tags
// are exactly those names, so key and source field agree. The helper
// transforms (valueOrDefault, nilIfEmpty, derefOrEmpty, orEmptySlice) are all
// identity functions once the source field is populated non-zero, as it is
// here.
var networkEncoderValueFlowExemptions = map[string]string{}

// isNetworkValueCheckableKind reports whether a generated field of type t can
// carry a value unique to its wire name: strings, string slices, and numbers
// (optionally behind one pointer). Bools are excluded -- every bool is
// populated true, so a bool sourced from the wrong bool field is
// indistinguishable. Nested structs and slices of structs are excluded too;
// they are passed through wholesale and have no single per-key signature.
func isNetworkValueCheckableKind(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	case reflect.Slice:
		return t.Elem().Kind() == reflect.String
	default:
		return false
	}
}

// setNetworkTaggedValue sets a value-checkable field to a value derived from
// its own wire name: strings get "v-<wirename>", string slices get
// ["v-<wirename>"], and numbers get 1000+fieldIndex (stable and unique per
// field, since every wire name maps to exactly one field).
func setNetworkTaggedValue(fv reflect.Value, wireName string, fieldIndex int) {
	switch fv.Kind() {
	case reflect.Pointer:
		fv.Set(reflect.New(fv.Type().Elem()))
		setNetworkTaggedValue(fv.Elem(), wireName, fieldIndex)
	case reflect.String:
		fv.SetString("v-" + wireName)
	case reflect.Slice:
		elem := reflect.New(fv.Type().Elem()).Elem()
		elem.SetString("v-" + wireName)
		fv.Set(reflect.Append(reflect.MakeSlice(fv.Type(), 0, 1), elem))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fv.SetInt(int64(1000 + fieldIndex))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fv.SetUint(uint64(1000 + fieldIndex))
	case reflect.Float32, reflect.Float64:
		fv.SetFloat(float64(1000 + fieldIndex))
	}
}

// newTaggedNetwork returns a Network where every value-checkable generated
// field carries a value derived from its own wire name. Fields that cannot
// carry a unique signature (bools, nested structs, ...) are populated with
// generic non-zero values so encoder code paths still run.
func newTaggedNetwork(t *testing.T) *Network {
	t.Helper()

	n := &Network{}
	v := reflect.ValueOf(n).Elem()
	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" { // unexported
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" || !isNetworkValueCheckableKind(field.Type) {
			populateNonZero(v.Field(i), 10)
			continue
		}
		setNetworkTaggedValue(v.Field(i), name, i)
	}
	return n
}

// networkValueExpectations maps each value-checkable generated wire name to
// the JSON encoding of the value the struct currently carries for it. It reads
// the live struct (rather than re-deriving from the wire name) so that
// per-purpose overrides like Purpose and IPSubnet are reflected automatically.
func networkValueExpectations(t *testing.T, n *Network) map[string]json.RawMessage {
	t.Helper()

	v := reflect.ValueOf(n).Elem()
	typ := v.Type()
	expected := make(map[string]json.RawMessage, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" { // unexported
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" || !isNetworkValueCheckableKind(field.Type) {
			continue
		}
		raw, err := json.Marshal(v.Field(i).Interface())
		if err != nil {
			t.Fatalf("marshal generated field %s (%s): %v", field.Name, name, err)
		}
		expected[name] = raw
	}
	return expected
}

// TestNetworkEncoderValueFlow asserts that for every purpose, each emitted
// top-level key whose generated field is a string / []string / number carries
// exactly the value of the generated field with that wire name. A mismatch
// means the encoder sourced the key from a different struct field.
func TestNetworkEncoderValueFlow(t *testing.T) {
	generated := map[string]bool{}
	for _, name := range networkWireNames(t) {
		generated[name] = true
	}

	for _, purpose := range networkEncoderPurposes {
		t.Run(purpose, func(t *testing.T) {
			n := newTaggedNetwork(t)
			n.Purpose = purpose

			// A parseable subnet keeps the encoder's DHCP-range calculation
			// from logging errors about the "v-ip_subnet" placeholder.
			// Expectations are computed from the struct afterwards, so
			// ip_subnet is still value-checked.
			subnet := "10.0.0.0/24"
			n.IPSubnet = &subnet

			expected := networkValueExpectations(t, n)

			data, err := json.Marshal(n)
			if err != nil {
				t.Fatalf("marshal Network with purpose %q: %v", purpose, err)
			}

			var payload map[string]json.RawMessage
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatalf("unmarshal encoder output for purpose %q: %v", purpose, err)
			}

			checked := 0
			for key, got := range payload {
				want, ok := expected[key]
				if !ok {
					// No comparable expectation: either a generated field of a
					// non-checkable kind (bool, nested struct, ...) or a
					// synthetic client-only key. Unknown keys still fail.
					if !generated[key] && networkEncoderSyntheticKeys[key] == "" {
						t.Errorf("purpose %q emits key %q that is neither on the generated Network struct nor in networkEncoderSyntheticKeys; add it there with a justification", purpose, key)
					}
					continue
				}
				if _, exempt := networkEncoderValueFlowExemptions[key]; exempt {
					continue
				}
				if !bytes.Equal(got, want) {
					t.Errorf("purpose %q key %q: encoder emitted %s but the generated field with that wire name carries %s -- the encoder is likely sourcing %q from a different struct field. If the transform is intentional, add the key to networkEncoderValueFlowExemptions with a justification.",
						purpose, key, got, want, key)
				}
				checked++
			}

			if checked == 0 {
				t.Errorf("purpose %q: value-checked no keys; the value-flow test has stopped covering anything", purpose)
			}
			t.Logf("purpose %q: value-checked %d emitted keys", purpose, checked)
		})
	}
}
