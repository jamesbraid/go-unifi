//go:build integration

package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationDHCPSlotsClearWithAnEmptyString pins the behaviour the
// encoder's clearableSlots exception rests on, through the public client.
//
// Two facts, opposite to the rule that governs every other optional string
// in this encoder:
//
//   - omitting a slot leaves the stored value alone, so omission is not a
//     way to clear one
//   - sending "" clears it
//
// Which is why these eight fields are *string and are not wrapped in
// nilIfEmpty: a caller emptying a DHCP DNS, NTP or WINS list has no other
// way to say so.
func TestIntegrationDHCPSlotsClearWithAnEmptyString(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)
	client := harnessClient(ctx, t, c)

	body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", map[string]any{
		"name": "slot-clear", "purpose": PurposeCorporate, "enabled": true,
		"ip_subnet": "10.95.1.1/24", "vlan_enabled": true, "vlan": 951,
		"setting_preference": "manual", "networkgroup": "LAN",
		"dhcpd_enabled": true, "dhcpd_start": "10.95.1.6", "dhcpd_stop": "10.95.1.254",
		"dhcpd_dns_enabled": true, "dhcpd_wins_enabled": true,
		"dhcpd_dns_1": "1.1.1.1", "dhcpd_dns_2": "1.0.0.1",
		"dhcpd_dns_3": "9.9.9.9", "dhcpd_dns_4": "8.8.8.8",
		"dhcpd_wins_1": "10.95.1.4",
	})
	if err != nil || status != 200 {
		t.Fatalf("seed rejected (HTTP %d): %v %v", status, body, err)
	}
	id, _ := firstData(t, body)["_id"].(string)
	defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck

	read := func() *Network {
		n, err := client.GetNetwork(ctx, c.Site, id)
		if err != nil {
			t.Fatalf("read network: %v", err)
		}
		return n
	}
	deref := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}

	stored := read()
	if deref(stored.DHCPDDNS2) != "1.0.0.1" {
		t.Fatalf("seed did not store dhcpd_dns_2: %q", deref(stored.DHCPDDNS2))
	}

	// Leaving a slot nil must not disturb it: nil means "not mine to set".
	stored.DHCPDDNS2 = nil
	if _, err := client.UpdateNetwork(ctx, c.Site, stored); err != nil {
		t.Fatalf("update with a nil slot: %v", err)
	}
	if got := deref(read().DHCPDDNS2); got != "1.0.0.1" {
		t.Errorf("a nil slot changed the stored value to %q; nil must leave it alone", got)
	}

	// An explicit empty clears it, and leaves its neighbours alone.
	current := read()
	empty := ""
	current.DHCPDDNS2 = &empty
	if _, err := client.UpdateNetwork(ctx, c.Site, current); err != nil {
		t.Fatalf("update with an explicit empty slot: %v", err)
	}
	after := read()
	if got := deref(after.DHCPDDNS2); got != "" && got != "<nil>" {
		t.Errorf("dhcpd_dns_2 is %q after an explicit empty; it should be cleared.\n\n"+
			"This is the one way a caller can empty a DHCP slot -- omitting the field "+
			"preserves it -- so if the controller stopped accepting \"\" here, "+
			"clearableSlots in network_encode_empty_test.go is what needs revisiting.", got)
	}
	if got := deref(after.DHCPDDNS1); got != "1.1.1.1" {
		t.Errorf("clearing dhcpd_dns_2 also changed dhcpd_dns_1 to %q", got)
	}
	if got := deref(after.DHCPDDNS3); got != "9.9.9.9" {
		t.Errorf("clearing dhcpd_dns_2 also changed dhcpd_dns_3 to %q", got)
	}

	// dhcpd_ntp_1 is the one slot the controller refuses to empty while its
	// feature toggle is on. Recorded rather than worked around: a caller
	// clearing it turns dhcpd_ntp_enabled off in the same write.
	ntp := read()
	ntpValue := "10.95.1.9"
	ntp.DHCPDNtp1 = &ntpValue
	ntp.DHCPDNtpEnabled = true
	if _, err := client.UpdateNetwork(ctx, c.Site, ntp); err != nil {
		t.Fatalf("seeding dhcpd_ntp_1: %v", err)
	}
	ntp = read()
	ntp.DHCPDNtp1 = &empty
	if _, err := client.UpdateNetwork(ctx, c.Site, ntp); err == nil {
		t.Log("note: this controller now accepts an empty dhcpd_ntp_1 with NTP enabled; " +
			"the pairing constraint recorded here has gone")
	}
	ntp = read()
	ntp.DHCPDNtp1 = &empty
	ntp.DHCPDNtpEnabled = false
	if _, err := client.UpdateNetwork(ctx, c.Site, ntp); err != nil {
		t.Errorf("clearing dhcpd_ntp_1 alongside dhcpd_ntp_enabled=false was refused: %v", err)
	}
	if got := deref(read().DHCPDNtp1); got != "" && got != "<nil>" {
		t.Errorf("dhcpd_ntp_1 is %q after being cleared with NTP disabled", got)
	}
}
