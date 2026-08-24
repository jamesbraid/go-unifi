//go:build integration

// unifi/ipv6_gateway_integration_test.go
package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationIPV6InterfaceTypeWithGateway settles whether the encoder
// should keep filling a nil ipv6_interface_type with "none".
//
// marshalCorporate and marshalGuest do that today, and it is the last entry
// in networkEncoderJustifiedNonZero -- the one exception to "the encoder
// invents nothing". The justification records that it could not be resolved:
// telling "absent" from "none" needs the static path, and on a gateway-less
// controller that is rejected outright with
// api.err.NotDeployableIPv6Subnet, because a deployable prefix requires real
// hardware.
//
// Adopting a real gateway settles it. A network created without the field
// stores no ipv6_interface_type at all; one created with "none" stores
// "none". They are distinguishable, so the forced default was not a no-op --
// it wrote a value the caller never chose. The encoder stopped, and the
// exception is gone from networkEncoderJustifiedNonZero, which now carries
// nothing but the dispatch field.
//
// The static path is still rejected (api.err.NotDeployableIPv6Subnet) even
// with a gateway adopted, since a documentation prefix is not deployable.
// It was not needed: absent versus "none" answers the question on its own.
func TestIntegrationIPV6InterfaceTypeWithGateway(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	gw := controllertest.AdoptGateway(ctx, t, c, s)
	t.Logf("adopted gateway %s (%s) state=%v", gw.MAC, gw.Model, gw.State)

	// A gateway needs a WAN before it can deploy anything downstream.
	wanBody, wanStatus, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", map[string]any{
		"name": "ipv6-probe-wan", "purpose": PurposeWAN, "enabled": true,
		"wan_networkgroup": "WAN", "wan_type": "dhcp", "wan_type_v6": "dhcpv6",
	})
	if wanStatus != 200 {
		raw, _ := json.Marshal(wanBody)
		t.Logf("WAN seed rejected (HTTP %d): %s -- the static case may still fail", wanStatus, raw)
	} else if created := firstData(t, wanBody); created != nil {
		if id, _ := created["_id"].(string); id != "" {
			defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck
		}
	}

	cases := []struct {
		name  string
		extra map[string]any
	}{
		{name: "absent", extra: nil},
		{name: "none", extra: map[string]any{"ipv6_interface_type": "none"}},
		{name: "static", extra: map[string]any{
			"ipv6_interface_type": "static",
			"ipv6_subnet":         "2001:db8:1::1/64",
			"ipv6_ra_enabled":     true,
		}},
	}

	stored := map[string]map[string]any{}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{
				"name": fmt.Sprintf("ipv6-%s", tc.name), "purpose": PurposeCorporate,
				"enabled": true, "ip_subnet": fmt.Sprintf("10.95.%d.1/24", i),
				"vlan_enabled": true, "vlan": 950 + i,
				"setting_preference": "manual", "networkgroup": "LAN",
			}
			for k, v := range tc.extra {
				payload[k] = v
			}

			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", payload)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}
			if status != 200 {
				raw, _ := json.Marshal(body)
				t.Logf("REJECTED %-8s HTTP %d %s", tc.name, status, raw)
				return
			}
			created := firstData(t, body)
			if id, _ := created["_id"].(string); id != "" {
				defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck
			}
			stored[tc.name] = created
			t.Logf("STORED   %-8s ipv6_interface_type=%v ipv6_subnet=%v ipv6_ra_enabled=%v",
				tc.name, created["ipv6_interface_type"], created["ipv6_subnet"], created["ipv6_ra_enabled"])
		})
	}

	absent, none := stored["absent"], stored["none"]
	if absent == nil || none == nil {
		t.Skip("could not create both the absent and none cases; nothing to compare")
	}
	// Measured with a UXGENT adopted and CONNECTED: the two are stored
	// differently, so filling a nil ipv6_interface_type with "none" wrote a
	// value the caller never chose and the controller kept it. The encoder no
	// longer does that, and this is the measurement that says why.
	if absent["ipv6_interface_type"] == none["ipv6_interface_type"] {
		t.Errorf("absent and explicit none now store the same ipv6_interface_type (%v); "+
			"the encoder could fill the default again, and networkEncoderJustifiedNonZero "+
			"should say so", absent["ipv6_interface_type"])
	}
	if got := absent["ipv6_interface_type"]; got != nil {
		t.Errorf("a network created without ipv6_interface_type stores %v, want it absent", got)
	}
	if got := none["ipv6_interface_type"]; got != "none" {
		t.Errorf("a network created with ipv6_interface_type=none stores %v, want none", got)
	}
	_ = err
}
