//go:build integration

// unifi/network_setting_preference_integration_test.go
package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationSettingPreferenceUnset checks that a caller who sets the
// advanced toggles and leaves SettingPreference nil gets them stored.
//
// The encoder used to send setting_preference "auto" in that case, which tells
// the controller to manage the advanced block itself. Measured on 10.4.57, six
// of eighteen advanced fields came back at the controller's choice rather than
// the caller's, silently: the three asserted here plus the dhcpd dns, ntp and
// time-offset toggles. Omitting the key instead makes the controller store
// "manual", inferred from the settings it was given, and keep all of them.
func TestIntegrationSettingPreferenceUnset(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	vlan := int64(181)
	offset := int64(3600)
	n := &Network{
		Name:        strPtr("setting-preference-unset"),
		Purpose:     PurposeCorporate,
		Enabled:     true,
		IPSubnet:    strPtr("10.93.81.1/24"),
		VLAN:        &vlan,
		VLANEnabled: true,
		// SettingPreference deliberately left nil.
		DHCPDEnabled:           true,
		DHCPguardEnabled:       true,
		DHCPDIP1:               "10.93.81.2",
		IGMPSnooping:           true,
		UPnPLanEnabled:         true,
		DHCPDDNSEnabled:        true,
		DHCPDDNS1:              "10.93.81.53",
		DHCPDNtpEnabled:        true,
		DHCPDNtp1:              strPtr("10.93.81.123"),
		DHCPDTimeOffsetEnabled: true,
		DHCPDTimeOffset:        &offset,
	}

	body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", n)
	if err != nil {
		t.Fatalf("transport: %v", err)
	}
	if status != 200 {
		t.Fatalf("controller rejected the encoder output (HTTP %d): %v", status, body)
	}
	created := firstData(t, body)
	if id, _ := created["_id"].(string); id != "" {
		defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck
	}

	if got := created["setting_preference"]; got != "manual" {
		t.Errorf("setting_preference = %v, want manual (inferred by the controller)", got)
	}
	for _, field := range []string{
		"dhcpguard_enabled",
		"igmp_snooping",
		"upnp_lan_enabled",
		"dhcpd_dns_enabled",
		"dhcpd_ntp_enabled",
		"dhcpd_time_offset_enabled",
	} {
		if created[field] != true {
			t.Errorf("%s = %v, want true (caller set it; the controller discarded it)", field, created[field])
		}
	}
}
