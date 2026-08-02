//go:build integration

// unifi/network_dhcpguard_integration_test.go
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

// TestIntegrationDHCPGuard posts the real encoder output for a DHCP
// Guard-enabled network and then round-trips it back through the encoder.
//
// marshalCorporate and marshalGuest emitted dhcpguard_enabled without the
// dhcpd_ip_1..3 trusted-server slots, so a caller-set DHCPDIP1 was dropped on
// the wire and the controller answered api.err.MissingIPAddress (it requires a
// non-empty dhcpd_ip_1 whenever the guard is on). The update arm is the sharper
// half: with the slots missing, even a read-modify-write of an untouched
// guarded network was rejected, which made such a network unmanageable through
// the SDK. Neither failure is visible to a unit test -- only the controller
// enforces the pairing.
func TestIntegrationDHCPGuard(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	for i, purpose := range []string{PurposeCorporate, PurposeGuest} {
		t.Run(purpose, func(t *testing.T) {
			octet := 90 + i
			vlan := int64(190 + i)
			trusted := fmt.Sprintf("10.98.%d.2", octet)

			// setting_preference must be "manual": the encoder otherwise
			// defaults it to "auto", and on auto the controller owns the
			// advanced toggles and stores dhcpguard_enabled=false whatever the
			// payload said (measured by dropping one encoder field at a time
			// until the flag stuck). The trusted-server slots themselves
			// persist either way.
			n := &Network{
				Name:              strPtr(fmt.Sprintf("dhcpguard-%s", purpose)),
				Purpose:           purpose,
				Enabled:           true,
				IPSubnet:          strPtr(fmt.Sprintf("10.98.%d.1/24", octet)),
				VLAN:              &vlan,
				VLANEnabled:       true,
				SettingPreference: strPtr("manual"),
				DHCPDEnabled:      true,
				DHCPguardEnabled:  true,
				DHCPDIP1:          trusted,
				DHCPDMAC1:         "00:11:22:33:44:55",
			}

			// PostJSON marshals n through Network.MarshalJSON, so this posts
			// exactly what the SDK would send.
			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", n)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}
			if status != 200 {
				t.Fatalf("controller rejected the encoder's guarded %s network (HTTP %d): %v", purpose, status, body)
			}

			created := firstData(t, body)
			id, _ := created["_id"].(string)
			if id == "" {
				t.Fatalf("no _id in create response: %v", created)
			}
			defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck

			if created["dhcpguard_enabled"] != true {
				t.Errorf("dhcpguard_enabled = %v, want true", created["dhcpguard_enabled"])
			}
			if created["dhcpd_ip_1"] != trusted {
				t.Errorf("dhcpd_ip_1 = %v, want %q", created["dhcpd_ip_1"], trusted)
			}
			if created["dhcpd_mac_1"] != "00:11:22:33:44:55" {
				t.Errorf("dhcpd_mac_1 = %v, want 00:11:22:33:44:55", created["dhcpd_mac_1"])
			}

			// Read-modify-write: decode the controller's own representation
			// back into a Network and PUT it unchanged. Before the encoder
			// carried the trusted-server slots this failed with
			// api.err.MissingIPAddress.
			raw, err := json.Marshal(created)
			if err != nil {
				t.Fatalf("re-marshal read-back: %v", err)
			}
			var round Network
			if err := json.Unmarshal(raw, &round); err != nil {
				t.Fatalf("decode read-back into Network: %v", err)
			}
			if round.DHCPDIP1 != trusted {
				t.Fatalf("decoded DHCPDIP1 = %q, want %q", round.DHCPDIP1, trusted)
			}

			body, status, err = s.PutJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id, &round)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}
			if status != 200 {
				t.Fatalf("controller rejected a round-trip PUT of a guarded %s network (HTTP %d): %v", purpose, status, body)
			}

			fresh, status, err := s.GetJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id)
			if err != nil || status != 200 {
				t.Fatalf("re-read after PUT (HTTP %d): %v", status, err)
			}
			after := firstData(t, fresh)
			if after["dhcpd_ip_1"] != trusted {
				t.Errorf("dhcpd_ip_1 after round-trip PUT = %v, want %q", after["dhcpd_ip_1"], trusted)
			}
			if after["dhcpguard_enabled"] != true {
				t.Errorf("dhcpguard_enabled after round-trip PUT = %v, want true", after["dhcpguard_enabled"])
			}
		})
	}
}
