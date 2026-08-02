//go:build integration

// unifi/network_ipsec_local_ip_gateway_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationGatewayIPSecLocalIP measures whether an adopted gateway is a
// second way past api.err.UnrecognizedLocalIp.
//
// TestIntegrationNetworkFieldProbe clears that rule with a static WAN network
// and no hardware at all, which is why it stays in the ungated suite. The
// question this answers is the other half: the controller also accepts an
// ipsec_local_ip that an adopted gateway reports under one of its wan1..wan9
// sub-documents, so a site with a gateway but no static WAN might qualify
// anyway. That mattered enough to measure rather than infer from what the
// emulator's inform payload appears to send.
//
// The site here deliberately has NO static WAN: the gateway is the only thing
// that could contribute an address, so an acceptance is attributable to it.
// controlForeignLocalIP is posted alongside as the control — without it, "the
// gateway's address was rejected" and "everything is rejected" look the same.
//
// Gated behind UNIFI_GATEWAY_TEST because adopting a device is a multi-minute
// run. Verdicts are logged, not asserted: the point is to record what the
// controller does.
func TestIntegrationGatewayIPSecLocalIP(t *testing.T) {
	if os.Getenv("UNIFI_GATEWAY_TEST") == "" {
		t.Skip("set UNIFI_GATEWAY_TEST=1 to run the gateway ipsec_local_ip probe")
	}
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	c := controllertest.Start(ctx, t)
	s := c.NewSession(ctx, t)

	d := controllertest.AdoptGateway(ctx, t, c, s)
	t.Logf("gateway adopted: mac=%s model=%q type=%q state=%d adopted=%v", d.MAC, d.Model, d.Type, d.State, d.Adopted)

	// Everything the site could offer as a local IP, before any of it is used.
	logDeviceWANSlots(ctx, t, s, c.Site)
	if nets, err := listNetworks(ctx, s, c.Site); err == nil {
		for _, n := range nets {
			if purpose, _ := n["purpose"].(string); purpose == PurposeWAN {
				t.Logf("site WAN network: name=%v wan_type=%v wan_ip=%v wan_networkgroup=%v",
					n["name"], n["wan_type"], n["wan_ip"], n["wan_networkgroup"])
			}
		}
	} else {
		t.Logf("could not list networkconf: %v", err)
	}

	addrs := gatewayWANAddresses(ctx, t, s, c.Site, d.MAC)
	if len(addrs) == 0 {
		t.Logf("MEASURED: adopted gateway %s reports no address under wan1..wan9, so it contributes "+
			"nothing to the controller's recognized local-IP set", d.MAC)
	}
	for slot, ip := range addrs {
		t.Logf("MEASURED: adopted gateway %s reports %s.ip = %s", d.MAC, slot, ip)
	}

	// Post one route-based site-vpn per address the gateway offers, plus the
	// control. Distinct peer IPs keep a create that outlives its cleanup from
	// colliding with the next (api.err.IpsecPeerIpOverlapped).
	type attempt struct {
		label string
		ip    string
	}
	attempts := []attempt{{label: "control-foreign-ip", ip: controlForeignLocalIP}}
	for slot, ip := range addrs {
		attempts = append(attempts, attempt{label: "gateway-" + slot, ip: ip})
	}

	for i, a := range attempts {
		t.Run(a.label, func(t *testing.T) {
			payload := map[string]any{
				"name":    fmt.Sprintf("gw-localip-probe-%d", i),
				"purpose": PurposeSiteVPN,
				"enabled": true,
			}
			for k, v := range mergeRouteBasedPrereq(map[string]any{
				"ipsec_tunnel_ip_enabled": true,
				"ipsec_tunnel_ip":         probeTunnelIP,
			}) {
				payload[k] = v
			}
			// The prereq carries the ungated probe's placeholders; this test
			// resolves them itself. ipsec_interface is wan because the
			// gateway's first uplink is what an address would come from.
			payload["ipsec_local_ip"] = a.ip
			payload["ipsec_interface"] = "wan"
			payload["ipsec_peer_ip"] = fmt.Sprintf("203.0.113.%d", 20+i)

			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", payload)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}
			if status != 200 {
				t.Logf("REJECTED ipsec_local_ip %s (HTTP %d): %s", a.ip, status, jsonText(body))
				return
			}

			id, _ := firstData(t, body)["_id"].(string)
			if id == "" {
				t.Logf("ACCEPTED ipsec_local_ip %s (HTTP 200) but no _id came back: %s", a.ip, jsonText(body))
				return
			}
			defer deleteNetwork(ctx, t, s, c.Site, id)

			stored := fetchNetwork(ctx, t, s, c.Site, id)
			if stored == nil {
				t.Logf("ACCEPTED ipsec_local_ip %s (HTTP 200) but the document could not be read back", a.ip)
				return
			}
			t.Logf("ACCEPTED ipsec_local_ip %s; stored ipsec_local_ip=%v ipsec_tunnel_ip=%v ipsec_tunnel_ip_enabled=%v",
				a.ip, stored["ipsec_local_ip"], stored["ipsec_tunnel_ip"], stored["ipsec_tunnel_ip_enabled"])
		})
	}
}

// gatewayWANAddresses returns the addresses one device reports under its
// wan1..wan9 sub-documents, keyed by slot. Those keys are the device-side
// half of the controller's recognized local-IP set; a device that informs an
// uplink without them contributes nothing to it.
func gatewayWANAddresses(ctx context.Context, t *testing.T, s *controllertest.Session, site, mac string) map[string]string {
	t.Helper()

	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/stat/device")
	if err != nil || status != 200 {
		t.Fatalf("stat/device: status=%d err=%v", status, err)
	}
	envelope, _ := body.(map[string]any)
	data, _ := envelope["data"].([]any)

	addrs := map[string]string{}
	for _, item := range data {
		device, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if got, _ := device["mac"].(string); !strings.EqualFold(got, mac) {
			continue
		}
		for i := 1; i <= 9; i++ {
			slot := fmt.Sprintf("wan%d", i)
			sub, ok := device[slot].(map[string]any)
			if !ok {
				continue
			}
			if ip, _ := sub["ip"].(string); ip != "" {
				addrs[slot] = ip
			}
		}
		return addrs
	}
	t.Fatalf("device %s absent from stat/device", mac)
	return nil
}
