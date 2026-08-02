//go:build integration

// unifi/network_partial_write_integration_test.go
package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationPartialWriteMerges pins the controller as merging a PUT
// rather than replacing the object.
//
// This is the fact the client's write model rests on. The generated structs
// declare 374 bools without omitempty (see TestGeneratedWriteShape), so every
// write asserts false for every toggle the caller left alone -- and that is
// the whole of the damage, because a key the payload omits keeps its stored
// value. If the controller replaced instead of merged, omitting a key would
// be as destructive as asserting false, and nothing short of sending the
// complete object would be safe.
//
// Measured on networkconf, a v1 REST collection. The v2 collections are not
// covered here and should not be assumed to agree.
func TestIntegrationPartialWriteMerges(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	seed := map[string]any{
		"name": "partial-write", "purpose": PurposeCorporate, "enabled": true,
		"ip_subnet": "10.97.1.1/24", "vlan_enabled": true, "vlan": 971,
		"setting_preference": "manual", "networkgroup": "LAN",
		"domain_name": "probe.example", "igmp_snooping": true,
		"upnp_lan_enabled": true, "dhcpd_enabled": true,
		"dhcpd_start": "10.97.1.6", "dhcpd_stop": "10.97.1.254",
		"dhcpd_leasetime": 7200,
	}
	body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", seed)
	if err != nil || status != 200 {
		t.Fatalf("seed rejected (HTTP %d): %v %v", status, body, err)
	}
	stored := firstData(t, body)
	id, _ := stored["_id"].(string)
	if id == "" {
		t.Fatal("no _id on the created network")
	}
	defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck

	// Identity and addressing only. Every field asserted below is absent.
	if _, status, err = s.PutJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id, map[string]any{
		"_id": id, "name": "partial-write", "purpose": PurposeCorporate,
		"ip_subnet": "10.97.1.1/24", "vlan_enabled": true, "vlan": 971,
		"networkgroup": "LAN", "enabled": true,
	}); err != nil || status != 200 {
		t.Fatalf("partial PUT rejected (HTTP %d): %v", status, err)
	}

	fresh, status, err := s.GetJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id)
	if err != nil || status != 200 {
		t.Fatalf("re-read failed (HTTP %d): %v", status, err)
	}
	after := firstData(t, fresh)

	for _, wire := range []string{
		"igmp_snooping",
		"upnp_lan_enabled",
		"domain_name",
		"dhcpd_leasetime",
		"setting_preference",
	} {
		if !jsonEqual(after[wire], stored[wire]) {
			t.Errorf("%s did not survive a PUT that omitted it: was %v, now %v.\n\n"+
				"The controller no longer merges. Omitting a field is now as destructive as "+
				"asserting a zero value, which invalidates the write model the client assumes.",
				wire, stored[wire], after[wire])
		}
	}
}
