//go:build integration

// unifi/ap_group_roundtrip_integration_test.go
package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationAPGroupRoundTrip checks that an AP group read through the
// client can be written straight back.
//
// Two separate faults made this impossible, both measured on 10.4.57:
//
//   - device_macs serializes unconditionally, so a group whose membership the
//     caller never touched put null on the wire, and the controller answers
//     "deviceMACs: must not be null".
//   - the site's default group carries attr_hidden_id and attr_no_delete, and
//     the write DTO rejects both -- "hiddenId: must be null",
//     "nonDeletable: must be false".
//
// Reading a group and writing it back is how membership is edited, so this is
// the path that matters. The default group is used deliberately: it is the
// one that carries the envelope fields, and a group created by the test would
// not exercise them.
func TestIntegrationAPGroupRoundTrip(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	api, err := New(ctx, &Config{
		BaseURL:       c.BaseURL,
		Username:      c.Username,
		Password:      c.Password,
		AllowInsecure: true,
	})
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	id := firstAPGroupID(ctx, t, s, c.Site)
	if id == "" {
		t.Skip("no AP group on this controller to round-trip")
	}

	group, err := api.GetAPGroup(ctx, c.Site, id)
	if err != nil {
		t.Fatalf("read the default AP group: %v", err)
	}
	if group.DeviceMacs != nil {
		t.Logf("default group already has %d member(s); the nil path is not exercised", len(group.DeviceMacs))
	}

	if _, err := api.UpdateAPGroup(ctx, c.Site, group); err != nil {
		t.Fatalf("writing back an unmodified AP group failed: %v\n\n"+
			"A group read through the client has to be writable, or membership cannot be edited "+
			"at all. Check that device_macs still marshals as [] rather than null, and that the "+
			"envelope fields are still dropped from the write.", err)
	}
}
