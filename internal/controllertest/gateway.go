package controllertest

import (
	"context"
	"testing"
	"time"
)

// GatewayModel is the emulated model string for the modern, BGP-capable
// gateway used to probe gateway-dependent controller features. UXGENT
// ("Gateway Enterprise") is a UniFi next-gen gateway; its 5.0.x firmware is
// well above UniFi's BGP floor. Adopting it is how the gateway feature-gate
// (BGP, firewall zones, NAT, routes) gets a supporting device on a container
// sim.
const GatewayModel = "UXGENT"

// GatewayType is the device type UXGENT informs as. Modern UniFi gateways
// report "uxg", not the legacy "ugw" of the USG family, and the controller's
// stat/device echoes it — so gateway assertions check for this, not "ugw".
const GatewayType = "uxg"

// AdoptGateway starts one emulated gateway beside the controller, adopts it,
// and returns the connected controller-side Device. It runs the full live
// flow: inform until the pending doc appears, then drive the controller-side
// adopt.
//
// The gateway's identity is whatever the herder allocated — a locally
// administered MAC that cannot collide with the -sim image's seeded UBNT
// fleet — so callers adopt by the returned MAC rather than by one they chose.
//
// A site adopts exactly one gateway (a second returns
// api.err.NoSecondGateway), so callers must give it a fresh controller
// (Start) and must not also adopt the -sim image's seeded, still-pending UGW3.
func AdoptGateway(ctx context.Context, t *testing.T, c *Controller, s *Session) Device {
	t.Helper()

	devices := StartDevices(ctx, t, c, DeviceRequest{Model: GatewayModel})
	if devices == nil {
		t.Skip("no emulated devices available for this controller target")
	}
	if len(devices) != 1 {
		t.Fatalf("herder started %d devices, want 1", len(devices))
	}
	mac := devices[0].MAC

	// Wait for the gateway to inform in as pending before adopting. A couple
	// of inform cycles land the doc; 2m is the ceiling for an inform going
	// nowhere. It must not already be adopted — nobody has adopted it yet, and
	// a spontaneously adopted doc would mean the MAC collided with something.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		d, ok, err := deviceByMAC(ctx, s, c.Site, mac)
		if err != nil {
			t.Fatalf("poll stat/device for %s: %v", mac, err)
		}
		if ok {
			if d.Adopted {
				t.Fatalf("gateway %s appeared already adopted: %+v", mac, d)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("gateway %s never appeared in stat/device", mac)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for %s to appear: %v", mac, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}

	// Drive the controller-side adopt; AdoptDevice blocks until state=1 &&
	// adopted. That is the whole verdict now: the device's own state machine
	// lives inside its container, and the herder already holds it to the
	// container contract — a device that died or went unhealthy after ready
	// ends the run with a failed event, which the fixture reports at cleanup.
	return c.AdoptDevice(ctx, t, s, mac)
}
