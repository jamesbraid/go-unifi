//go:build integration

// unifi/network_factory_default_probe_integration_test.go
package unifi

import (
	"context"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationNetworkFactoryDefaults measures what the controller fills
// in for a networkconf create that names only the fields
// required_on_create_when says the controller refuses to do without --
// nothing else. terraform-provider-unifi's modelToNetwork instead builds a
// full struct literal asserting its OWN idea of the controller's factory
// defaults (17 fields for a DHCP-disabled network alone), so a value here
// that the provider also hardcodes is evidence the assertion is redundant:
// omitting the field and trusting the controller reaches the same document.
//
// This intentionally does not attempt every branch. site-vpn/vpn-client/
// remote-user-vpn each need a working peer (a real IPsec gateway, a
// reachable WireGuard endpoint, a RADIUS profile) to create at all, which is
// a different, heavier probe than "what does an empty document default to".
// Only corporate, guest, vlan-only, and wan are measured; the rest are
// named, not guessed at.
func TestIntegrationNetworkFactoryDefaults(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 15*time.Minute)
	site := c.Site

	cases := []struct {
		purpose string
		doc     map[string]any
	}{
		// vlan_enabled=false ("untagged"/native) was tried first and always
		// answers api.err.VlanUsed here regardless of the vlan value sent
		// (empty, 0, or a fresh number) -- the demo site already ships one
		// native corporate LAN, and only one untagged network can exist per
		// site. That collision is itself the measurement for the untagged
		// case: it says nothing about corporate/guest factory defaults, so
		// these use vlan_enabled=true with a fresh VLAN instead.
		{
			purpose: "corporate",
			doc: map[string]any{
				"name":         "probe-factory-default-corporate",
				"purpose":      PurposeCorporate,
				"enabled":      true,
				"vlan_enabled": true,
				"vlan":         100,
			},
		},
		{
			purpose: "guest",
			doc: map[string]any{
				"name":         "probe-factory-default-guest",
				"purpose":      PurposeGuest,
				"enabled":      true,
				"vlan_enabled": true,
				"vlan":         101,
			},
		},
		{
			purpose: "vlan-only",
			doc: map[string]any{
				"name":         "probe-factory-default-vlan-only",
				"purpose":      PurposeVLANOnly,
				"enabled":      true,
				"vlan_enabled": true,
				"vlan":         999,
			},
		},
		{
			purpose: "wan",
			doc: map[string]any{
				"name":    "probe-factory-default-wan",
				"purpose": PurposeWAN,
				"enabled": true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.purpose, func(t *testing.T) {
			status, body := createRaw(ctx, s, site, "networkconf", tc.doc)
			if status/100 != 2 {
				t.Fatalf("smallest %s create was refused (HTTP %d, %s)", tc.purpose, status, v1ErrCode(body))
			}
			id := objectID(firstData(t, body))
			t.Cleanup(func() { deleteNetwork(ctx, t, s, site, id) })

			stored := fetchNetwork(ctx, t, s, site, id)
			if stored == nil {
				t.Fatalf("could not read back the smallest %s network", tc.purpose)
			}
			t.Logf("MEASURED factory-default document for purpose=%s (asked %d keys, controller stores %d): %s",
				tc.purpose, len(tc.doc), len(stored), jsonText(stored))
		})
	}
}

// TestIntegrationNetworkZeroValueReachesTheFactoryDefault asks the design
// question Fact 2 turns on: does a Go caller who leaves every DHCP-related
// field at its zero value -- rather than hand-asserting an explicit
// "DHCP disabled" struct literal, as terraform-provider-unifi's
// modelToNetwork does with 17 fields -- reach a network the controller
// treats identically to one that never named those keys at all?
//
// unifi.Network's own generated struct settles this without any new SDK
// code: DHCPDEnabled and its bool siblings carry no `omitempty` json tag
// (they are not optional on the wire the way *string/`*int64` fields are),
// so networkFieldValue's `b || !omitEmpty` rule sends them explicitly on
// every corporate/guest/vlan-only create regardless of what the caller set.
// A caller therefore cannot omit dhcpd_enabled through this client even if
// they tried, and does not need to: leaving the Go field at its zero value
// (false) already produces the wire's own definition of "off". That is the
// runtime derivation the roadmap asked about -- it already exists, driven
// by the same field-inclusion lists CreateNetwork always used, and costs
// nothing extra to keep in sync because there is nothing extra: the struct
// tag IS the fact.
func TestIntegrationNetworkZeroValueReachesTheFactoryDefault(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 15*time.Minute)
	site := c.Site
	client := harnessClient(ctx, t, c)

	name := "probe-zero-value-corporate"
	vlan := int64(150)
	created, err := client.CreateNetwork(ctx, site, &Network{
		Name:        &name,
		Purpose:     PurposeCorporate,
		Enabled:     true,
		VLANEnabled: true,
		VLAN:        &vlan,
		// Every DHCP field, NetworkGroup, DomainName, etc. is left at its Go
		// zero value on purpose -- this is the whole point of the probe.
	})
	if err != nil {
		t.Fatalf("CreateNetwork with a near-zero-value struct was refused: %v", err)
	}
	t.Cleanup(func() { deleteNetwork(ctx, t, s, site, created.ID) })

	stored := fetchNetwork(ctx, t, s, site, created.ID)
	if stored == nil {
		t.Fatalf("could not read back the zero-value network")
	}
	t.Logf("MEASURED stored document for a zero-value corporate Network{} (client sent every networkCorporateFields key, most at Go zero value): %s", jsonText(stored))

	dhcpEnabled, has := stored["dhcpd_enabled"]
	if !has {
		t.Fatalf("dhcpd_enabled is absent from the read-back; expected an explicit false since the field has no omitempty tag")
	}
	if dhcpEnabled != false {
		t.Fatalf("dhcpd_enabled = %v; expected false from a zero-value DHCPDEnabled bool", dhcpEnabled)
	}
	t.Logf("MEASURED: a zero-value DHCPDEnabled bool round-trips as an explicit dhcpd_enabled=false, the same DHCP-disabled state terraform-provider-unifi's hardcoded default block asserts by hand -- no captured default document is needed to reach it")

	for _, wire := range []string{"dhcpd_start", "dhcpd_stop", "dhcpd_leasetime", "dhcpd_gateway", "dhcpd_ntp_1", "dhcpd_wins_1"} {
		if v, present := stored[wire]; present {
			t.Fatalf("%s = %v; a nil *string/*int64 field with omitempty should have dropped this key entirely", wire, v)
		}
	}
	t.Logf("MEASURED: every omitempty pointer field left nil (dhcpd_start/stop/leasetime/gateway/ntp_1/wins_1) is genuinely absent from the read-back, not defaulted by the controller")
}

// createRaw POSTs a raw document to a v1 rest collection and returns the
// controller's answer without asserting on it.
func createRaw(ctx context.Context, s *controllertest.Session, site, collection string, doc map[string]any) (int, any) {
	body, status, err := s.PostJSON(ctx, "/api/s/"+site+"/rest/"+collection, doc)
	if err != nil {
		return 0, map[string]any{"transport_error": err.Error()}
	}
	return status, body
}
