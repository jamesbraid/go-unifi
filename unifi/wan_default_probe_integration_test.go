//go:build integration

// unifi/wan_default_probe_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationDefaultWANNetworkIdentity measures what actually marks one
// networkconf as the site's DEFAULT (primary) WAN network -- the fact
// terraform-provider-unifi currently hand-mines in two files by scanning
// every network for purpose=="wan" && wan_networkgroup=="WAN", a magic
// string duplicated in traffic_route_resource.go's defaultWANNetworkID and
// wan_resource.go's adoptExistingWAN.
//
// The wan_networkgroup field's own validator pattern (WAN[2-9]?|
// WAN_LTE_FAILOVER, schemas/fields/NetworkConf.json) already says bare
// "WAN" is a distinct value from "WAN2".."WAN9" and "WAN_LTE_FAILOVER". What
// that pattern cannot say is whether "WAN" is genuinely the server's notion
// of "the default slot" or just the first of ten legal strings a caller
// picked by convention. This probe tells them apart the only way that
// works: by making the server show its own bookkeeping.
//
//  1. Create a static WAN network with wan_networkgroup OMITTED entirely.
//  2. Attempt a second static WAN explicitly claiming wan_networkgroup
//     "WAN". If the server internally treats an absent key as the "WAN"
//     slot, this collides (api.err.WanConfigurationForNetworkGroupAlreadyExists);
//     if omitted and "WAN" are unrelated states, both networks coexist.
//  3. Create a third static WAN explicitly in "WAN2" and confirm it
//     coexists with both -- multi-WAN slots are independent of each other.
//  4. Read back the omitted-group network's full stored document and log
//     it, so a human can check whether any OTHER field (attr_no_delete,
//     attr_hidden_id, ...) also marks default-WAN status.
func TestIntegrationDefaultWANNetworkIdentity(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 15*time.Minute)
	site := c.Site

	// Step 1: wan_networkgroup omitted entirely. Each attempt below gets its
	// own /24 (different gateway and DNS, not just a different host
	// address) so api.err.DuplicateWanIpInOtherConfiguration -- a real
	// separate rule about reusing a gateway/DNS pair -- cannot masquerade as
	// a wan_networkgroup collision.
	status1, body1 := attemptStaticWANGroup(ctx, s, site, "probe-wan-omit-group", 10, "")
	if status1/100 != 2 {
		t.Fatalf("create with wan_networkgroup omitted was refused (HTTP %d, %s); cannot measure the default without a baseline WAN to collide against", status1, v1ErrCode(body1))
	}
	omittedID := objectID(firstData(t, body1))
	t.Cleanup(func() { deleteNetwork(ctx, t, s, site, omittedID) })

	stored := fetchNetwork(ctx, t, s, site, omittedID)
	if stored == nil {
		t.Fatalf("could not read back the omitted-group WAN network")
	}
	t.Logf("full stored document for the omitted-group WAN network: %s", jsonText(stored))
	if group, _ := stored["wan_networkgroup"].(string); group != WANNetworkGroupDefault {
		t.Fatalf("an omitted wan_networkgroup on create stored as %q, want %q (unifi.WANNetworkGroupDefault) -- "+
			"the controller's own default has changed and WANNetworkGroupDefault needs re-measuring", group, WANNetworkGroupDefault)
	}
	if hidden := stored["attr_hidden_id"]; hidden != WANNetworkGroupDefault {
		t.Fatalf("attr_hidden_id = %v, want %q mirroring wan_networkgroup", hidden, WANNetworkGroupDefault)
	}

	// Step 2: an explicit "WAN" must collide with the omitted-group network
	// -- if it did not, WANNetworkGroupDefault would be documenting a
	// caller convention rather than the server's own bookkeeping.
	status2, body2 := attemptStaticWANGroup(ctx, s, site, "probe-wan-explicit-WAN", 20, WANNetworkGroupDefault)
	if status2/100 == 2 {
		id2 := objectID(firstData(t, body2))
		t.Cleanup(func() { deleteNetwork(ctx, t, s, site, id2) })
		t.Fatalf("an explicit wan_networkgroup=%q create SUCCEEDED alongside the omitted-group network (HTTP %d); "+
			"expected api.err.WanConfigurationForNetworkGroupAlreadyExists -- an absent key no longer collides with the literal default", WANNetworkGroupDefault, status2)
	}
	if reason := v1ErrCode(body2); reason != "api.err.WanConfigurationForNetworkGroupAlreadyExists" {
		t.Fatalf("explicit wan_networkgroup=%q create was refused for a different reason (HTTP %d, %s); "+
			"re-check whether the collision this test relies on still holds", WANNetworkGroupDefault, status2, reason)
	}
	t.Logf("MEASURED: an explicit wan_networkgroup=%q create collided with the omitted-group network (api.err.WanConfigurationForNetworkGroupAlreadyExists), confirming the server treats the omitted key as the %q slot", WANNetworkGroupDefault, WANNetworkGroupDefault)

	// Step 3: WAN2 must coexist with everything above -- multi-WAN slots are
	// independent of the WAN slot's state.
	status3, body3 := attemptStaticWANGroup(ctx, s, site, "probe-wan-explicit-WAN2", 30, "WAN2")
	if status3/100 != 2 {
		t.Fatalf("a WAN2-group create was refused unexpectedly (HTTP %d, %s); multi-WAN slots should be independent of the WAN slot's state", status3, v1ErrCode(body3))
	}
	id3 := objectID(firstData(t, body3))
	t.Cleanup(func() { deleteNetwork(ctx, t, s, site, id3) })
	t.Logf("MEASURED: an explicit wan_networkgroup=\"WAN2\" create coexists with the WAN-slot networks above (HTTP %d)", status3)
}

// attemptStaticWANGroup POSTs a minimal static WAN networkconf on its own
// /24 (203.0.$octet.0/24), optionally naming a wan_networkgroup (empty
// string omits the key entirely rather than sending it as ""). It does not
// assert on the result: callers decide what a given status means for the
// fact under measurement.
func attemptStaticWANGroup(ctx context.Context, s *controllertest.Session, site, name string, octet int, group string) (int, any) {
	base := fmt.Sprintf("203.0.%d.", octet)
	doc := map[string]any{
		"name":                  name,
		"purpose":               PurposeWAN,
		"enabled":               true,
		"wan_type":              "static",
		"wan_type_v6":           "disabled",
		"wan_ip":                base + "2",
		"wan_netmask":           "255.255.255.0",
		"wan_gateway":           base + "1",
		"wan_dns1":              base + "53",
		"wan_load_balance_type": "failover-only",
		"report_wan_event":      false,
	}
	if group != "" {
		doc["wan_networkgroup"] = group
	}
	body, status, err := s.PostJSON(ctx, "/api/s/"+site+"/rest/networkconf", doc)
	if err != nil {
		return 0, map[string]any{"transport_error": err.Error()}
	}
	return status, body
}
