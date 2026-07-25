//go:build integration

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// deviceGateMarkers are the controller error codes that mean "no device here
// supports this feature" — the 404s a bare, gateway-less sim returns. Seeing
// one with a gateway adopted means the gate did not clear; seeing anything
// else (2xx, or a validation complaint about the payload) means it did.
var deviceGateMarkers = []string{
	"BgpUnsupportedDevice",
	"CouldNotFindHotspotFirewallZone",
	"UnsupportedDevice",
	"NoSupportingDevice",
	"UnrecognizedLocalIp",
}

// classify turns a live response into the verdict this probe exists to
// produce. A validation rejection is deliberately NOT "still gated": it means
// the request reached field validation, which the device gate short-circuits.
func classify(status int, body any) string {
	switch {
	case status >= 200 && status < 300:
		return "UNBLOCKED"
	case hasDeviceGateMarker(body):
		return "STILL-GATED"
	default:
		return "REJECTED-NOT-GATED"
	}
}

func hasDeviceGateMarker(body any) bool {
	raw, err := json.Marshal(body)
	if err != nil {
		return false
	}
	for _, m := range deviceGateMarkers {
		if strings.Contains(string(raw), m) {
			return true
		}
	}
	return false
}

func jsonString(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(raw)
}

// dumpDevice logs the controller's stat/device document for one MAC. A model
// the controller does not recognize carries no feature capabilities, so this
// separates "the gate says no" from "the controller has no idea what this is".
func dumpDevice(ctx context.Context, t *testing.T, s *controllertest.Session, site, mac string) {
	t.Helper()

	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/stat/device")
	if err != nil || status != 200 {
		t.Fatalf("stat/device: status=%d err=%v", status, err)
	}
	envelope, _ := body.(map[string]any)
	data, _ := envelope["data"].([]any)
	for _, item := range data {
		d, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if m, _ := d["mac"].(string); !strings.EqualFold(m, mac) {
			continue
		}
		keys := make([]string, 0, len(d))
		for k := range d {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if strings.Contains(k, "model") || strings.Contains(k, "support") ||
				strings.Contains(k, "cap") || strings.Contains(k, "feature") {
				t.Logf("device %s = %s", k, jsonString(d[k]))
			}
		}
		return
	}
	t.Fatalf("device %s absent from stat/device", mac)
}

// siteNetworks returns the site's networkconf documents, so probe payloads can
// name real network ids instead of guesses.
func siteNetworks(ctx context.Context, t *testing.T, s *controllertest.Session, site string) []map[string]any {
	t.Helper()

	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/rest/networkconf")
	if err != nil || status != 200 {
		t.Fatalf("list networkconf: status=%d err=%v", status, err)
	}
	envelope, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("networkconf body is not an object: %T", body)
	}
	data, _ := envelope["data"].([]any)
	nets := make([]map[string]any, 0, len(data))
	for _, item := range data {
		if m, ok := item.(map[string]any); ok {
			nets = append(nets, m)
		}
	}
	return nets
}

func networkID(nets []map[string]any, purpose string) string {
	for _, n := range nets {
		if p, _ := n["purpose"].(string); p == purpose {
			if id, _ := n["_id"].(string); id != "" {
				return id
			}
		}
	}
	return ""
}

// ensureWAN returns the site's WAN networkconf id, creating one if the site has
// none. The -sim demo site ships a single corporate network and adopting a
// gateway does not add a WAN, but NAT rules and traffic routes must name a WAN
// network — without one they fail on the payload before any device check, which
// would masquerade as a device gate.
func ensureWAN(ctx context.Context, t *testing.T, s *controllertest.Session, site string) (lanID, wanID string) {
	t.Helper()

	nets := siteNetworks(ctx, t, s, site)
	lanID, wanID = networkID(nets, "corporate"), networkID(nets, "wan")
	if wanID != "" {
		return lanID, wanID
	}

	wan := map[string]any{
		"name":                  "WAN",
		"purpose":               "wan",
		"wan_type":              "dhcp",
		"wan_networkgroup":      "WAN",
		"enabled":               true,
		"wan_load_balance_type": "failover-only",
		"report_wan_event":      false,
	}
	body, status, err := s.PostJSON(ctx, "/api/s/"+site+"/rest/networkconf", wan)
	if err != nil || status >= 300 {
		t.Fatalf("seed WAN networkconf: status=%d err=%v body=%s", status, err, jsonString(body))
	}
	nets = siteNetworks(ctx, t, s, site)
	lanID, wanID = networkID(nets, "corporate"), networkID(nets, "wan")
	if wanID == "" {
		t.Fatalf("WAN networkconf seeded but absent from the list: %s", jsonString(nets))
	}
	return lanID, wanID
}

type gatewayProbe struct {
	name    string
	methods []string
	path    string
	payload map[string]any
}

// gatewayProbes builds a schema-complete payload for every v2 endpoint the
// bare sim device-gates. Complete matters: a payload missing a required field
// draws a 400 from validation before the device check runs, which reads as a
// cleared gate when it is nothing of the sort (the lesson from the UOS BGP
// probe, where an omitted uploaded_file_name hid the 404).
func gatewayProbes(lanID, wanID string) []gatewayProbe {
	return []gatewayProbe{
		{
			name: "bgp", methods: []string{"POST"}, path: "bgp/config",
			payload: map[string]any{
				"enabled":            true,
				"description":        "emu-probe",
				"frr_bgpd_config":    "router bgp 65000\n",
				"uploaded_file_name": "bgp.conf",
			},
		},
		{
			name: "firewall-zone", methods: []string{"POST"}, path: "firewall/zone",
			payload: map[string]any{
				"name":        "emu-probe-zone",
				"network_ids": []string{},
			},
		},
		{
			name: "nat", methods: []string{"POST"}, path: "nat",
			payload: map[string]any{
				"description":        "emu-probe",
				"enabled":            true,
				"type":               "MASQUERADE",
				"ip_version":         "IPV4",
				"protocol":           "all",
				"rule_index":         "1",
				"logging":            false,
				"exclude":            false,
				"is_predefined":      false,
				"setting_preference": "auto",
				"out_interface":      wanID,
				"source_filter":      map[string]any{"filter_type": "NONE", "firewall_group_ids": []string{}},
				"destination_filter": map[string]any{"filter_type": "NONE", "firewall_group_ids": []string{}},
			},
		},
		{
			name: "traffic-route", methods: []string{"POST"}, path: "trafficroutes",
			payload: map[string]any{
				"description":         "emu-probe",
				"enabled":             true,
				"matching_target":     "INTERNET",
				"network_id":          wanID,
				"kill_switch_enabled": false,
				"domains":             []any{},
				"ip_addresses":        []any{},
				"ip_ranges":           []any{},
				"regions":             []any{},
				"target_devices":      []any{map[string]any{"type": "NETWORK", "network_id": lanID}},
			},
		},
		{
			// ospf/router is POST-only (PUT answers 405), rejects an empty
			// areas list, and takes area_type as a lowercase enum: "normal"
			// is accepted where "NORMAL" and "STANDARD" are rejected as
			// unknown enum values. A backbone area over the LAN is the
			// smallest router document it will construct.
			name: "ospf", methods: []string{"POST"}, path: "ospf/router",
			payload: map[string]any{
				"enabled":                       true,
				"router_id":                     "0.0.0.1",
				"announce_default_route":        false,
				"redistribute_connected_routes": false,
				"redistribute_static_routes":    false,
				"interfaces":                    []any{},
				"areas": []any{map[string]any{
					"area_id":     "0.0.0.0",
					"area_type":   "normal",
					"name":        "emu-probe-area",
					"network_ids": []string{lanID},
				}},
			},
		},
	}
}

// sweepGatewayEndpoints runs every gateway-gated v2 endpoint against the
// controller as it currently stands and logs a verdict per endpoint. It is
// called twice — once with a gateway adopted, once without — because a site
// change made along the way (the seeded WAN network) could otherwise be
// mistaken for the gateway's doing.
func sweepGatewayEndpoints(ctx context.Context, t *testing.T, c *controllertest.Controller, s *controllertest.Session) {
	t.Helper()

	lanID, wanID := ensureWAN(ctx, t, s, c.Site)
	t.Logf("network ids: lan=%q wan=%q", lanID, wanID)

	for _, p := range gatewayProbes(lanID, wanID) {
		t.Run(p.name, func(t *testing.T) {
			full := "/v2/api/site/" + c.Site + "/" + p.path
			for _, method := range p.methods {
				var (
					body   any
					status int
					err    error
				)
				if method == "PUT" {
					body, status, err = s.PutJSON(ctx, full, p.payload)
				} else {
					body, status, err = s.PostJSON(ctx, full, p.payload)
				}
				switch {
				case errors.Is(err, controllertest.ErrNotJSON):
					// Still a verdict: the v2 API answers a wrong-method
					// request with an empty body rather than a JSON error.
					t.Logf("NON-JSON %s %s: HTTP %d", method, p.path, status)
				case err != nil:
					t.Fatalf("%s %s transport error: %v", method, full, err)
				default:
					t.Logf("%s %s %s: HTTP %d body=%s", classify(status, body), method, p.path, status, jsonString(body))
				}
			}
		})
	}
}

// TestIntegrationGatewayFeatureGate adopts an emulated modern gateway (UXGENT,
// type uxg) and POSTs a schema-complete payload to every v2 endpoint the bare
// sim was thought to device-gate, recording per endpoint what the controller
// does. Read it together with TestIntegrationGatewayFeatureGateNoDevice, which
// runs the identical sweep with no gateway adopted.
//
// Measured on UniFi Network 10.4.57 (both the :10.4.57-sim and :sim tags,
// which carry the same build), with UXGENT CONNECTED and flagged neither
// unsupported nor model_incompatible. Re-measured 2026-08-01 once the
// emulator began reporting device capabilities:
//
//	endpoint       adopted UXGENT                            no gateway
//	nat            201 created                               201 created
//	trafficroutes  201 created                               201 created
//	ospf/router    201 created                               201 created
//	bgp/config     201 created                               404 api.err.BgpUnsupportedDevice
//	firewall/zone  404 CouldNotFindHotspotFirewallZone       identical
//
// Only bgp/config distinguishes the two arms, and only since the emulated
// gateway started claiming udapi_caps. On 2026-07-24, with an emulator that
// reported no capabilities, this row read 404 in BOTH columns and the whole
// table was identical — which is why the sweep is written as a comparison
// rather than a single run. The endpoint was never gated on a gateway
// existing; it is gated on one that says it can do BGP.
//
// The other four rows still answer the same on both sides, so what blocked
// nat, trafficroutes and ospf/router was never hardware: it was site state and
// payload shape. The demo site has no WAN networkconf (and adoption does not
// create one — see ensureWAN), ospf/router rejects an empty areas list, and
// its area_type is a lowercase enum. Supply those and all three accept a write
// on a gateway-less controller.
//
// firewall/zone stays 404 in both columns. That one is not about the device at
// all: the zone write path resolves the site's HOTSPOT-keyed zone before doing
// anything else, and a site whose default zone set was never created has none.
//
// CAVEAT on the firewall/zone row: that 404 is not the whole story. The handler
// PERSISTS the zone and then throws, so the write lands even though the status
// says otherwise — but only when the POST is the first request to the zone
// collection, which it is not here (this sweep lists every collection first).
// TestIntegrationSeededUOSFirewallZoneSeed is the test that pins that down.
// Read "STILL-GATED firewall-zone" below as "answered 404", not "wrote
// nothing": this sweep classifies status codes, and only a read-back can tell
// those apart.
//
// CAVEAT on the firewall/zone row: that 404 is not the whole story. The handler
// PERSISTS the zone and then throws, so the write lands even though the status
// says otherwise — but only when the POST is the first request to the zone
// collection, which it is not here (this sweep lists every collection first).
// TestIntegrationSeededUOSFirewallZoneSeed is the test that pins that down.
// Read "STILL-GATED firewall-zone" below as "answered 404", not "wrote
// nothing": this sweep classifies status codes, and only a read-back can tell
// those apart.
//
// The test asserts only that the harness boots and the gateway adopts — a valid
// smoke test. The endpoint verdicts are logged data, not assertions: the point
// is to record what the controller does, not to freeze what we assume. Gated
// behind UNIFI_GATEWAY_TEST (like the UOS probe's UNIFI_UOS_TEST) because a
// controller boot plus gateway adoption is a multi-minute run.
func TestIntegrationGatewayFeatureGate(t *testing.T) {
	if os.Getenv("UNIFI_GATEWAY_TEST") == "" {
		t.Skip("set UNIFI_GATEWAY_TEST=1 to run the gateway feature-gate sweep")
	}
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating gateway probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	c := controllertest.Start(ctx, t)
	s := c.NewSession(ctx, t)

	d := controllertest.AdoptGateway(ctx, t, c, s)
	t.Logf("gateway adopted: mac=%s model=%q type=%q state=%d adopted=%v",
		d.MAC, d.Model, d.Type, d.State, d.Adopted)

	// Whether the controller recognizes the model at all: an unsupported or
	// incompatible device would carry no feature capabilities, which would
	// explain a blanket gate without saying anything about gateways. The MAC
	// is whatever the herder allocated, so ask about the device we adopted.
	dumpDevice(ctx, t, s, c.Site, d.MAC)

	sweepGatewayEndpoints(ctx, t, c, s)
}

// TestIntegrationGatewayFeatureGateNoDevice is the control for
// TestIntegrationGatewayFeatureGate: the same sweep on the same image with no
// gateway adopted. It is what makes that test a measurement rather than an
// anecdote — the sweep also creates the WAN network the demo site lacks, so
// without this arm every accepted write is unattributable.
//
// Measured 2026-07-24: every endpoint answers exactly as it does with a gateway
// adopted (nat, trafficroutes and ospf/router accept the write; bgp/config and
// firewall/zone return their 404s). That is the finding — see the result table
// on TestIntegrationGatewayFeatureGate.
func TestIntegrationGatewayFeatureGateNoDevice(t *testing.T) {
	if os.Getenv("UNIFI_GATEWAY_TEST") == "" {
		t.Skip("set UNIFI_GATEWAY_TEST=1 to run the gateway feature-gate sweep")
	}
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating gateway probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	c := controllertest.Start(ctx, t)
	s := c.NewSession(ctx, t)
	t.Log("control run: no gateway adopted")

	sweepGatewayEndpoints(ctx, t, c, s)
}
