//go:build integration

package unifi

import (
	"fmt"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationDevicePortOverridesOpModeSweep measures a provisional claim
// that never named a controller version or a reproduction: that the
// controller rejects op_mode="switch" -- the enum's own default -- on a
// gateway PUT, and that omitting op_mode while configuring a non-default
// mode leaves link aggregation never engaging.
//
// Neither half of that claim is taken on faith. This writes op_mode=switch
// bare (no companion fields) to an adopted device of each class this SDK
// probes elsewhere -- a switch and a gateway -- so a class-specific gate
// would show up as a difference between the two arms rather than being
// assumed from the one class the report named. It then drives the sequence
// the second half of the claim implies: op_mode=aggregate with real
// aggregate_members, then a follow-up write naming op_mode=switch to back
// out of it, which is the one place "switch, the default" is a deliberate
// transition rather than an initial value.
func TestIntegrationDevicePortOverridesOpModeSweep(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	emulated := controllertest.StartDevices(ctx, t, c, controllertest.DeviceRequest{Model: "USM8P"})
	if len(emulated) != 1 {
		t.Skip("no emulated switch available for this controller target")
	}
	sw := c.AdoptDevice(ctx, t, s, emulated[0].MAC)
	swID := deviceIDForMAC(ctx, t, s, c.Site, sw.MAC)
	if swID == "" {
		t.Skipf("adopted %s but the controller lists no device with that MAC", sw.MAC)
	}

	gw := controllertest.AdoptGateway(ctx, t, c, s)
	gwID := deviceIDForMAC(ctx, t, s, c.Site, gw.MAC)
	if gwID == "" {
		t.Skipf("adopted %s but the controller lists no device with that MAC", gw.MAC)
	}

	// writeAndAwait PUTs a single-entry port_overrides array and waits for
	// stat/device to echo this write specifically, identified by its own
	// name -- the same reason the discard probe and the preference sweep
	// both wait on a name rather than on the array becoming non-nil: a
	// write that landed a moment ago must not be misread as this one's
	// result.
	writeAndAwait := func(t *testing.T, id, mac, name string, entry map[string]any) (status int, body any, settled map[string]any) {
		t.Helper()
		entry["name"] = name
		body, status, err := s.PutJSON(ctx, "/api/s/"+c.Site+"/rest/device/"+id,
			map[string]any{"port_overrides": []any{entry}})
		if err != nil {
			t.Fatalf("transport writing port_idx 1 on %s: %v", mac, err)
		}
		if status != 200 {
			return status, body, nil
		}
		deadline := time.Now().Add(30 * time.Second)
		for {
			got := storedPortOverride(ctx, t, s, c.Site, mac, 1)
			if n, _ := got["name"].(string); n == name {
				return status, body, got
			}
			if time.Now().After(deadline) {
				t.Logf("%s never settled on write %q within the deadline; last read: %v", mac, name, got)
				return status, body, got
			}
			time.Sleep(2 * time.Second)
		}
	}

	sweep := func(t *testing.T, class, id, mac string) {
		// op_mode at its reported-default value, bare -- no companion
		// fields, the simplest case the report's first half describes.
		t.Run("switch bare", func(t *testing.T) {
			status, body, got := writeAndAwait(t, id, mac, "opmode-switch-bare",
				map[string]any{"port_idx": 1, "op_mode": "switch"})
			if status != 200 {
				t.Errorf("%s: op_mode=switch (bare) REJECTED (HTTP %d): %s", class, status, jsonText(body))
				return
			}
			if got == nil {
				t.Fatalf("%s: accepted (HTTP 200) but the write never settled on stat/device", class)
			}
			if stored, _ := got["op_mode"].(string); stored != "switch" {
				t.Errorf("%s: op_mode=switch accepted (HTTP 200) but stored as %q, not echoed back", class, stored)
				return
			}
			t.Logf("%s: op_mode=switch (bare) ACCEPTED and stored verbatim", class)
		})

		// The report's second half: configure a non-default mode, then back
		// out of it by naming the default explicitly. aggregate_members
		// names two OTHER port indices, so this also exercises op_mode
		// alongside its one real companion field rather than in isolation.
		t.Run("aggregate then revert to switch", func(t *testing.T) {
			aggStatus, aggBody, agg := writeAndAwait(t, id, mac, "opmode-aggregate",
				map[string]any{"port_idx": 1, "op_mode": "aggregate", "aggregate_members": []any{2, 3}})
			if aggStatus != 200 {
				t.Logf("%s: op_mode=aggregate REJECTED (HTTP %d): %s -- cannot test reverting from it",
					class, aggStatus, jsonText(aggBody))
				return
			}
			if agg == nil {
				t.Fatalf("%s: aggregate accepted (HTTP 200) but never settled", class)
			}
			if stored, _ := agg["op_mode"].(string); stored != "aggregate" {
				t.Logf("%s: op_mode=aggregate accepted but stored as %q, not \"aggregate\" -- "+
					"cannot test reverting from a mode that was not actually entered", class, stored)
				return
			}
			t.Logf("%s: op_mode=aggregate ACCEPTED and stored", class)

			revStatus, revBody, rev := writeAndAwait(t, id, mac, "opmode-revert",
				map[string]any{"port_idx": 1, "op_mode": "switch"})
			if revStatus != 200 {
				t.Errorf("%s: reverting op_mode to switch AFTER aggregate REJECTED (HTTP %d): %s\n\n"+
					"This is the transition the provisional report may have meant: not \"switch\" refused "+
					"outright, but refused specifically as a downgrade away from an active aggregate mode.",
					class, revStatus, jsonText(revBody))
				return
			}
			if rev == nil {
				t.Fatalf("%s: revert accepted (HTTP 200) but never settled", class)
			}
			if stored, _ := rev["op_mode"].(string); stored != "switch" {
				t.Errorf("%s: revert accepted (HTTP 200) but op_mode stored as %q, not \"switch\"", class, stored)
				return
			}
			if _, stillMember := rev["aggregate_members"]; stillMember {
				t.Logf("%s: reverted to switch but aggregate_members is still stored: %v -- "+
					"stale LAG membership beside op_mode=switch, which is one way a caller could "+
					"observe the aggregate never actually stopped", class, rev["aggregate_members"])
			}
			t.Logf("%s: op_mode=aggregate -> switch ACCEPTED both ways", class)
		})
	}

	t.Run(fmt.Sprintf("USM8P switch (%s)", sw.MAC), func(t *testing.T) { sweep(t, "USM8P switch", swID, sw.MAC) })
	t.Run(fmt.Sprintf("UXGENT gateway (%s)", gw.MAC), func(t *testing.T) { sweep(t, "UXGENT gateway", gwID, gw.MAC) })
}
