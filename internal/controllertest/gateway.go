//go:build integration

package controllertest

import (
	"context"
	"testing"
	"time"

	emu "github.com/jamesbraid/unifi-emu"
)

// GatewayModel is the emu model string for the modern, BGP-capable gateway
// used to probe gateway-dependent controller features. UXGENT ("Gateway
// Enterprise") is a UniFi next-gen gateway; its 5.0.x firmware is well above
// UniFi's BGP floor. Adopting it is how the gateway feature-gate (BGP,
// firewall zones, NAT, routes) gets a supporting device on a container sim.
const GatewayModel = "UXGENT"

// GatewayType is the device type UXGENT informs as. Modern UniFi gateways
// report "uxg", not the legacy "ugw" of the USG family, and the controller's
// stat/device echoes it — so gateway assertions check for this, not "ugw".
const GatewayType = "uxg"

// AdoptGateway brings up an in-process emu fleet holding one gateway, adopts
// it, and returns the connected controller-side Device. It runs the full live
// flow: inform until the pending doc appears, drive the controller-side
// adopt, then confirm the sim's own state machine also reached CONNECTED.
//
// A site adopts exactly one gateway (a second returns
// api.err.NoSecondGateway), so callers must give it a fresh controller
// (Start) and must not also adopt the -sim image's seeded, still-pending UGW3.
func AdoptGateway(ctx context.Context, t *testing.T, c *Controller, s *Session, spec emu.DeviceSpec) Device {
	t.Helper()

	sim := StartDeviceSim(ctx, t, c, spec)
	if sim == nil {
		t.Skip("controller target has no host-reachable inform URL")
	}

	// Wait for the gateway to inform in as pending before adopting. A couple
	// of 2s inform cycles land the doc; 2m is the ceiling for an inform going
	// nowhere. It must not already be adopted — nobody has adopted it yet, and
	// a spontaneously adopted doc would mean the MAC collided with something.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		d, ok, err := deviceByMAC(ctx, s, c.Site, spec.MAC)
		if err != nil {
			t.Fatalf("poll stat/device for %s: %v", spec.MAC, err)
		}
		if ok {
			if d.Adopted {
				t.Fatalf("gateway %s appeared already adopted: %+v", spec.MAC, d)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("gateway %s never appeared in stat/device", spec.MAC)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for %s to appear: %v", spec.MAC, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}

	// Drive the controller-side adopt; AdoptDevice blocks until state=1 &&
	// adopted, retrying the CannotAdopt window.
	d := c.AdoptDevice(ctx, t, s, spec.MAC)

	// Confirm the sim's own state machine also reached CONNECTED — it flips
	// after the controller delivers the authkey in the post-adopt inform.
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := sim.WaitState(waitCtx, spec.MAC, emu.StateConnected); err != nil {
		t.Fatalf("gateway sim %s never reached CONNECTED: %v", spec.MAC, err)
	}
	return d
}
