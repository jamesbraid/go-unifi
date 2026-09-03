//go:build integration

// unifi/network_field_probe_integration_test.go
package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationNetworkFieldProbe classifies every candidate field by
// creating a throwaway network with the field set (raw request — the SDK
// encoder would strip it), reading it back, and comparing.
//
//	PERSISTED: controller stored and returned the value -> wire it
//	STRIPPED:  create succeeded, field absent/zero on read-back
//	REJECTED:  create failed with the field present
func TestIntegrationNetworkFieldProbe(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	// Record what the site's devices contribute to the controller's set of
	// recognized local IPs before anything is created. The networkconf
	// validator's ipsec_local_ip check unions the site's own networks with
	// the adopted gateway's wan1..wan9 sub-documents, so "is there a device
	// arm at all here" is a fact worth measuring rather than inferring.
	logDeviceWANSlots(ctx, t, s, c.Site)

	// Seed a static WAN so the route-based site-vpn candidates have an
	// ipsec_local_ip the controller recognizes. Failures are carried rather
	// than fatal: only the candidates that actually reference the WAN are
	// invalidated by one, and the rest of the table still measures.
	wan, wanErr := ensureStaticWAN(ctx, t, s, c.Site)
	if wanErr != nil {
		t.Logf("static WAN precondition FAILED: %v", wanErr)
	} else {
		t.Logf("static WAN precondition satisfied: wan_ip=%s slot=%s (id %s)", wan.IP, wan.Interface, wan.ID)
	}

	// Resolve "@..." placeholders to real referenced-object ids once, so
	// candidates and their Prereq siblings can reference an object that
	// actually exists on this controller instead of a well-formed but
	// nonexistent id.
	placeholders := map[string]string{
		"@zone":               firstZoneID(ctx, t, s, c.Site),
		"@radiusprofile":      firstRadiusProfileID(ctx, t, s, c.Site),
		"@defaultnetwork":     firstObjectID(ctx, t, s, c.Site, "networkconf"),
		placeholderWANLocalIP: wan.IP,
		placeholderWANIf:      wan.Interface,
	}

	// require_mschapv2 and vpn_protocol select RADIUS-authenticated
	// remote-user-vpn modes (l2tp-server, openvpn-server); the controller
	// rejects them with api.err.RadiusServerNotEnabled unless the site's
	// RADIUS setting (distinct from the radiusprofile object referenced by
	// radiusprofile_id) is itself enabled.
	if _, status, err := s.PutJSON(ctx, "/api/s/"+c.Site+"/set/setting/radius", map[string]any{"key": "radius", "enabled": true}); err != nil || status != 200 {
		t.Logf("failed to enable site radius setting (status %d, %v); RADIUS-gated candidates may be REJECTED", status, err)
	}
	// A placeholder can appear on its own or inside a list: a field naming
	// one object takes a string, one naming several takes a slice of them.
	// Resolving only the scalar case would send the literal "@..." to the
	// controller and measure its rejection of a malformed id rather than
	// the field.
	var resolve func(v any) any
	resolve = func(v any) any {
		switch value := v.(type) {
		case string:
			if resolved, ok := placeholders[value]; ok {
				return resolved
			}
		case []string:
			out := make([]string, len(value))
			for i, item := range value {
				out[i], _ = resolve(item).(string)
			}
			return out
		}
		return v
	}

	base := func(purpose string, n int) map[string]any {
		m := map[string]any{
			"name":    fmt.Sprintf("probe-%d", n),
			"purpose": purpose,
			"enabled": true,
		}
		if purpose == PurposeCorporate || purpose == PurposeGuest {
			m["ip_subnet"] = fmt.Sprintf("10.99.%d.1/24", n%250)
			m["vlan_enabled"] = true
			m["vlan"] = 100 + n
		}
		return m
	}

	for i, cand := range networkFieldCandidates {
		t.Run(cand.Wire, func(t *testing.T) {
			// A candidate whose payload names the seeded WAN measures
			// nothing if the WAN never came up: its ipsec_local_ip would be
			// an address the controller cannot recognize, and the rejection
			// that follows describes the scaffolding, not the field. That is
			// exactly the non-result these three entries have carried since
			// the first pass, so fail instead of logging a verdict.
			if wanErr != nil && candidateUses(cand, placeholderWANLocalIP) {
				t.Fatalf("static WAN precondition failed (%v); %s cannot be measured without a recognized ipsec_local_ip", wanErr, cand.Wire)
			}

			payload := base(cand.Purpose, i)
			for k, v := range cand.Prereq {
				payload[k] = resolve(v)
			}
			value := resolve(cand.Value)
			payload[cand.Wire] = value

			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", payload)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}
			if status != 200 {
				if marker := prereqErrorMarker(body); marker != "" {
					t.Fatalf("PREREQUISITE FAILURE for %s: HTTP %d, %s. That error names the probe's own "+
						"scaffolding (WAN address, peer, remote subnets), not %s, so NO verdict was measured "+
						"for this field. Body: %s", cand.Wire, status, marker, cand.Wire, jsonText(body))
				}
				t.Logf("REJECTED %s (HTTP %d): %s", cand.Wire, status, jsonText(body))
				return
			}

			created := firstData(t, body)
			id, _ := created["_id"].(string)
			if id == "" {
				t.Errorf("create of %s returned HTTP 200 with no _id, so the stored document cannot be read back "+
					"and no verdict was measured: %s", cand.Wire, jsonText(created))
				return
			}
			defer deleteNetwork(ctx, t, s, c.Site, id)

			// Classify from the STORED document, never from the create
			// response. The controller echoes back the document it was
			// handed, so a field it accepted syntactically and then dropped
			// looks identical to one it wrote until you re-read it.
			stored := fetchNetwork(ctx, t, s, c.Site, id)
			if stored == nil {
				t.Errorf("%s: created %s but could not read it back; no verdict measured", cand.Wire, id)
				return
			}

			got, ok := stored[cand.Wire]
			switch {
			case ok && jsonEqual(got, value):
				t.Logf("PERSISTED %s = %v (read back from the stored document)", cand.Wire, got)
			case ok:
				t.Logf("MUTATED %s: sent %v, stored %v", cand.Wire, value, got)
			default:
				t.Logf("STRIPPED %s: create accepted, field absent from the stored document", cand.Wire)
			}
		})
	}

	// Negative control for the static-WAN precondition. The route-based
	// site-vpn candidates only stop drawing api.err.UnrecognizedLocalIp once
	// the site owns their ipsec_local_ip, and "the seeded WAN is what made
	// the difference" is worth measuring rather than assuming -- the same
	// payload changed in several ways at once. If the controller ever accepts
	// a foreign local IP, the verdicts above stop saying anything about the
	// prerequisite and this fails loudly instead of quietly meaning less.
	if wanErr == nil {
		t.Run("control_unrecognized_local_ip", func(t *testing.T) {
			payload := base(PurposeSiteVPN, len(networkFieldCandidates))
			for k, v := range mergeRouteBasedPrereq(map[string]any{
				"ipsec_peer_ip":           "203.0.113.14",
				"ipsec_tunnel_ip_enabled": true,
				"ipsec_tunnel_ip":         probeTunnelIP,
			}) {
				payload[k] = resolve(v)
			}
			payload["ipsec_local_ip"] = controlForeignLocalIP

			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", payload)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}
			if status == 200 {
				if id, _ := firstData(t, body)["_id"].(string); id != "" {
					deleteNetwork(ctx, t, s, c.Site, id)
				}
				t.Errorf("control: controller ACCEPTED ipsec_local_ip %s, which this site does not own. "+
					"The local-IP rule is not what the seeded static WAN clears, so the route-based "+
					"verdicts above do not show what the prerequisite is.", controlForeignLocalIP)
				return
			}
			if !strings.Contains(jsonText(body), "UnrecognizedLocalIp") {
				t.Errorf("control: ipsec_local_ip %s was rejected (HTTP %d) but not with api.err.UnrecognizedLocalIp: %s. "+
					"Something other than the local-IP rule is refusing this payload, so it is not a control.",
					controlForeignLocalIP, status, jsonText(body))
				return
			}
			t.Logf("control: ipsec_local_ip %s still draws api.err.UnrecognizedLocalIp (HTTP %d); "+
				"the seeded static WAN is what the route-based verdicts above depend on", controlForeignLocalIP, status)
		})
	}
}

// controlForeignLocalIP is an address no site the probe touches owns. It is
// the literal the first pass used as ipsec_local_ip, whose rejection was
// recorded as the route-based candidates' verdict.
const controlForeignLocalIP = "198.51.100.5"

// placeholderWANLocalIP and placeholderWANIf are substituted by the probe with
// the address and interface slot of the static WAN it seeds (ensureStaticWAN).
const (
	placeholderWANLocalIP = "@wanlocalip"
	placeholderWANIf      = "@wanif"
)

// candidateUses reports whether placeholder appears anywhere in cand's value
// or prereq siblings, i.e. whether cand depends on that placeholder resolving
// to something real.
func candidateUses(cand fieldCandidate, placeholder string) bool {
	if s, ok := cand.Value.(string); ok && s == placeholder {
		return true
	}
	for _, v := range cand.Prereq {
		if s, ok := v.(string); ok && s == placeholder {
			return true
		}
	}
	return false
}

// prereqErrorMarkers are the controller error codes that mean the probe's own
// scaffolding was refused, not the candidate field. Recording one of these as
// a field verdict is how "REJECTED, api.err.UnrecognizedLocalIp" sat in the
// encoder allowlist for months describing a payload defect rather than a
// controller rule. A REJECTED verdict is only honest when the error names the
// candidate's own rule (IpsecTunnelIpRequiresDynamicRouting, IpsecTunnelIpMissing,
// IpsecDynamicSubnetsRequireTunnelIp, ...), so those are deliberately absent.
var prereqErrorMarkers = []string{
	"UnrecognizedLocalIp",                       // ipsec_local_ip is not an address the site owns
	"IpsecDynamicRoutingRequiresLocalIPAddress", // ipsec_local_ip left at "any"
	"RemoteSubnetOverlapsWithLocalIp",           // ipsec_local_ip sits inside remote_vpn_subnets
	"IpsecPeerIpOverlapped",                     // a previous candidate's network outlived its cleanup
	"NetmaskNotSupported",                       // /32 in remote_vpn_subnets with no gateway on site
}

// prereqErrorMarker returns the prerequisite error code present in body, or ""
// if the rejection is about the candidate field itself.
func prereqErrorMarker(body any) string {
	raw := jsonText(body)
	for _, marker := range prereqErrorMarkers {
		if strings.Contains(raw, marker) {
			return marker
		}
	}
	return ""
}

func jsonText(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(raw)
}

func firstZoneID(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/v2/api/site/"+site+"/firewall/zone")
	if err != nil || status != 200 {
		t.Logf("no firewall zones available (status %d, %v); @zone candidates will be REJECTED", status, err)
		return "000000000000000000000000"
	}
	if items, ok := body.([]any); ok && len(items) > 0 {
		if m, ok := items[0].(map[string]any); ok {
			if id, ok := m["_id"].(string); ok {
				return id
			}
		}
	}
	return "000000000000000000000000"
}

// firstRadiusProfileID resolves "@radiusprofile" to a real radiusprofile id.
// Every site ships a default RADIUS profile, so radiusprofile_id-gated
// candidates (e.g. require_mschapv2, vpn_protocol on l2tp/openvpn-server
// remote-user-vpn networks) can reference an object the controller actually
// recognizes instead of a well-formed but nonexistent id.
func firstRadiusProfileID(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/rest/radiusprofile")
	if err != nil || status != 200 {
		t.Logf("no radius profiles available (status %d, %v); @radiusprofile candidates will be REJECTED", status, err)
		return "000000000000000000000000"
	}
	if m := firstData(t, body); m != nil {
		if id, ok := m["_id"].(string); ok {
			return id
		}
	}
	return "000000000000000000000000"
}

// ---------------------------------------------------------------------------
// Static WAN precondition
//
// The route-based (ipsec_dynamic_routing) site-vpn candidates need an
// ipsec_local_ip the controller recognizes as belonging to this site.
// api.err.UnrecognizedLocalIp is a set-membership rejection, and the set is
// built from the site's own configuration: the host addresses of enabled
// corporate/guest networks, the wan_ip of every enabled STATIC WAN network,
// its wan_ip_aliases, and the wan1..wan9 sub-documents of an adopted gateway
// device. The device arm is optional, so a static WAN network alone should
// satisfy it -- no hardware required, which is why this probe stays in the
// ungated suite. logDeviceWANSlots records whether any device contributed
// after all, rather than leaving that to inference.

const (
	probeWANName    = "probe-static-wan"
	probeWANIP      = "203.0.113.2"
	probeWANNetmask = "255.255.255.0"
	probeWANGateway = "203.0.113.1"
	probeWANDNS     = "203.0.113.53"
)

// staticWANInfo describes the WAN network ensureStaticWAN seeded.
type staticWANInfo struct {
	ID string
	// IP is wan_ip as the controller STORED it, read back from the
	// collection rather than taken from the create response. Only the stored
	// string is a legitimate ipsec_local_ip.
	IP string
	// Interface is the ipsec_interface spelling of the slot the WAN landed
	// in: "wan" for wan_networkgroup WAN, "wan2" for WAN2, and so on.
	Interface string
}

// wanNetworkGroups is the ordered set of wan_networkgroup slots the probe will
// claim, lowest first. WAN_LTE_FAILOVER is deliberately excluded: it has no
// ipsec_interface spelling matching the schema's wan[2-9]? pattern.
var wanNetworkGroups = []string{"WAN", "WAN2", "WAN3", "WAN4", "WAN5", "WAN6", "WAN7", "WAN8", "WAN9"}

// ensureStaticWAN creates a static WAN network on the site and returns its
// stored address. The returned error is the honest signal that the probe's
// prerequisite is unmet: callers must not record field verdicts against it.
func ensureStaticWAN(ctx context.Context, t *testing.T, s *controllertest.Session, site string) (staticWANInfo, error) {
	t.Helper()

	nets, err := listNetworks(ctx, s, site)
	if err != nil {
		return staticWANInfo{}, fmt.Errorf("list networkconf: %w", err)
	}

	used := map[string]bool{}
	for _, n := range nets {
		if purpose, _ := n["purpose"].(string); purpose != PurposeWAN {
			continue
		}
		if group, _ := n["wan_networkgroup"].(string); group != "" {
			used[strings.ToUpper(group)] = true
		}
	}
	slot := ""
	for _, group := range wanNetworkGroups {
		if !used[group] {
			slot = group
			break
		}
	}
	if slot == "" {
		return staticWANInfo{}, fmt.Errorf("no free wan_networkgroup slot (taken: %v)", used)
	}
	t.Logf("seeding a static WAN in slot %s; WAN slots already taken on this site: %v", slot, used)

	body, status, err := s.PostJSON(ctx, "/api/s/"+site+"/rest/networkconf", map[string]any{
		"name":                  probeWANName,
		"purpose":               PurposeWAN,
		"enabled":               true,
		"wan_networkgroup":      slot,
		"wan_type":              "static",
		"wan_type_v6":           "disabled",
		"wan_ip":                probeWANIP,
		"wan_netmask":           probeWANNetmask,
		"wan_gateway":           probeWANGateway,
		"wan_dns1":              probeWANDNS,
		"wan_load_balance_type": "failover-only",
		"report_wan_event":      false,
	})
	if err != nil {
		return staticWANInfo{}, fmt.Errorf("transport: %w", err)
	}
	if status != 200 {
		return staticWANInfo{}, fmt.Errorf("controller refused the static WAN (HTTP %d): %s", status, jsonText(body))
	}

	// HTTP 200 is not evidence of a write. Re-read the collection and work
	// from the document the controller actually has; a wan_ip that came back
	// stripped or rewritten means the probe measured nothing.
	nets, err = listNetworks(ctx, s, site)
	if err != nil {
		return staticWANInfo{}, fmt.Errorf("read back networkconf after seeding the WAN: %w", err)
	}
	var stored map[string]any
	for _, n := range nets {
		if name, _ := n["name"].(string); name == probeWANName {
			stored = n
			break
		}
	}
	if stored == nil {
		return staticWANInfo{}, fmt.Errorf("static WAN accepted (HTTP 200) but absent from the collection on read-back")
	}

	info := staticWANInfo{Interface: strings.ToLower(slot)}
	info.ID, _ = stored["_id"].(string)
	if info.ID != "" {
		id := info.ID
		t.Cleanup(func() {
			// The test's own context is cancelled by the time cleanups run.
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			deleteNetwork(cleanupCtx, t, s, site, id)
		})
	}

	if got, _ := stored["wan_type"].(string); got != "static" {
		return info, fmt.Errorf("stored wan_type is %q, want static: %s", got, jsonText(stored))
	}
	ip, _ := stored["wan_ip"].(string)
	if ip == "" {
		return info, fmt.Errorf("stored wan_ip is empty, so this WAN contributes no recognized address: %s", jsonText(stored))
	}
	if ip != probeWANIP {
		t.Logf("controller rewrote wan_ip: sent %s, stored %s; using the stored value", probeWANIP, ip)
	}
	info.IP = ip
	return info, nil
}

// listNetworks returns the site's networkconf documents.
func listNetworks(ctx context.Context, s *controllertest.Session, site string) ([]map[string]any, error) {
	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/rest/networkconf")
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", status, jsonText(body))
	}
	envelope, ok := body.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("networkconf body is not an object: %T", body)
	}
	data, _ := envelope["data"].([]any)
	nets := make([]map[string]any, 0, len(data))
	for _, item := range data {
		if m, ok := item.(map[string]any); ok {
			nets = append(nets, m)
		}
	}
	return nets, nil
}

// fetchNetwork returns the stored networkconf document for id, or nil. It
// prefers the per-id GET and falls back to scanning the collection, so a
// controller that answers only the list form still yields a read-back.
//
// The _id of what comes back is checked either way. A per-id GET that ignores
// the id and answers with the whole collection would otherwise hand back some
// other network's document and have its fields classified as the candidate's.
func fetchNetwork(ctx context.Context, t *testing.T, s *controllertest.Session, site, id string) map[string]any {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/rest/networkconf/"+id)
	if err == nil && status == 200 {
		if m := firstData(t, body); m != nil {
			got, _ := m["_id"].(string)
			if got == id {
				return m
			}
			t.Logf("per-id read-back of %s answered with _id %q instead", id, got)
		}
	}
	t.Logf("per-id read-back of %s did not yield the document (status %d, %v); falling back to the collection", id, status, err)
	nets, err := listNetworks(ctx, s, site)
	if err != nil {
		t.Logf("collection read-back also failed: %v", err)
		return nil
	}
	for _, n := range nets {
		if got, _ := n["_id"].(string); got == id {
			return n
		}
	}
	return nil
}

// deleteNetwork removes a probe network and reports a failure to do so. A
// leaked site-vpn network collides with the next candidate on peer IP
// (api.err.IpsecPeerIpOverlapped), which would masquerade as a field verdict.
func deleteNetwork(ctx context.Context, t *testing.T, s *controllertest.Session, site, id string) {
	t.Helper()
	body, status, err := s.DeleteJSON(ctx, "/api/s/"+site+"/rest/networkconf/"+id)
	if err != nil || status != 200 {
		t.Logf("cleanup of networkconf %s FAILED (status %d, %v): %s", id, status, err, jsonText(body))
	}
}

// logDeviceWANSlots records the wan1..wan9 sub-documents of every device the
// controller reports. Those keys are the only device-side contribution to the
// ipsec_local_ip check, so whether an adopted gateway supplies one decides
// whether hardware is a second route past api.err.UnrecognizedLocalIp.
// Measuring beats inferring it from the emulator's inform payload.
func logDeviceWANSlots(ctx context.Context, t *testing.T, s *controllertest.Session, site string) {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/stat/device")
	if err != nil || status != 200 {
		t.Logf("stat/device unavailable (status %d, %v); cannot record device wanN contributions", status, err)
		return
	}
	envelope, _ := body.(map[string]any)
	data, _ := envelope["data"].([]any)
	if len(data) == 0 {
		t.Logf("stat/device: no devices on this site, so no device contributes a wanN.ip to the recognized local-IP set")
		return
	}
	for _, item := range data {
		device, ok := item.(map[string]any)
		if !ok {
			continue
		}
		mac, _ := device["mac"].(string)
		var slots []string
		for i := 1; i <= 9; i++ {
			key := fmt.Sprintf("wan%d", i)
			if v, ok := device[key]; ok {
				slots = append(slots, key+"="+jsonText(v))
			}
		}
		if len(slots) == 0 {
			slots = append(slots, "no wan1..wan9 keys")
		}
		t.Logf("stat/device %s (model %v, type %v, state %v): %s",
			mac, device["model"], device["type"], device["state"], strings.Join(slots, " "))
	}
}
