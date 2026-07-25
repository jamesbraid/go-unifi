//go:build integration

package controllertest

import (
	"context"
	"testing"
	"time"

	emu "github.com/jamesbraid/unifi-emu"
)

// StartDeviceSim starts an in-process unifi-emu fleet of specs informing
// this controller and returns it, or nil when the controller has no
// usable inform URL: a UNIFI_TEST_URL target is an external controller
// whose inform plane is never the suite's to point devices at, and the
// UOS harness is unsupported for now — its -sim image seeds its own demo
// devices, and a seeded UOS image (the real target there) is not yet
// available.
//
// The fleet informs every 2s (the library default of 10s turns
// pending-doc discovery into a crawl) and is stopped by t.Cleanup. emu's
// own logs go to stderr rather than t.Log: routing them through
// log.SetOutput(t.LogWriter()) would be process-global and cross-wire
// every test's output.
func StartDeviceSim(ctx context.Context, t *testing.T, c *Controller, specs ...emu.DeviceSpec) *emu.Emu {
	t.Helper()

	if c.InformURL == "" {
		t.Log("controller has no host-reachable inform URL; not starting a device sim")
		return nil
	}

	sim := emu.New(c.InformURL, emu.WithInformInterval(2*time.Second))
	if err := sim.Add(specs...); err != nil {
		t.Fatalf("device sim: add specs: %v", err)
	}
	if err := sim.Start(ctx); err != nil {
		t.Fatalf("device sim: start: %v", err)
	}
	t.Cleanup(sim.Stop)
	return sim
}
