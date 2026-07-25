//go:build integration

package controllertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	emu "github.com/jamesbraid/unifi-emu"
)

// TestIntegrationDeviceSimAdoption runs an in-process emulated switch
// against a fresh controller end to end: inform, pending stat/device doc,
// AdoptDevice, connected on both sides — the controller's device doc and
// the sim's own state machine. The MAC sits in the UBNT OUI with a suffix
// the -sim image's seeded demo fleet never uses, so stat/device lookups
// can only match the sim's device.
func TestIntegrationDeviceSimAdoption(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	c := Start(ctx, t)
	s := c.NewSession(ctx, t)

	const mac = "00:27:22:e0:00:41"
	sim := StartDeviceSim(ctx, t, c, emu.DeviceSpec{MAC: mac, Model: "USM8P", IP: "192.168.1.250"})
	if sim == nil {
		t.Skip("controller target has no host-reachable inform URL")
	}

	// A couple of 2s inform cycles should land the pending doc; 2m is the
	// generous ceiling for an inform going nowhere. The doc must NOT be
	// adopted: nobody adopted it, and a spontaneously adopted doc would
	// mean the fleet collided with something else.
	var pending Device
	deadline := time.Now().Add(2 * time.Minute)
	for {
		d, ok, err := deviceByMAC(ctx, s, c.Site, mac)
		if err != nil {
			t.Fatalf("poll stat/device for %s: %v", mac, err)
		}
		if ok {
			if d.Adopted {
				t.Fatalf("device %s appeared already adopted: %+v", mac, d)
			}
			pending = d
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("device %s never appeared in stat/device", mac)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for %s to appear: %v", mac, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
	t.Logf("pending doc: %+v", pending)

	d := c.AdoptDevice(ctx, t, s, mac)
	if d.State != 1 || !d.Adopted {
		t.Fatalf("device %s: state=%d adopted=%v, want state=1 adopted=true", mac, d.State, d.Adopted)
	}

	waitCtx, waitCancel := context.WithTimeout(ctx, 30*time.Second)
	defer waitCancel()
	if err := sim.WaitState(waitCtx, mac, emu.StateConnected); err != nil {
		t.Fatalf("sim: %v", err)
	}
}

// TestIntegrationStartDeviceSimNilForURLTarget pins the guard: a
// UNIFI_TEST_URL controller gets no InformURL and StartDeviceSim returns
// nil rather than pointing devices at an external controller. The stub
// login server drives Start's URL branch, so no container boots.
func TestIntegrationStartDeviceSimNilForURLTarget(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"meta":{"rc":"ok"}}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	t.Setenv("UNIFI_TEST_URL", srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	c := Start(ctx, t)
	if c.InformURL != "" {
		t.Fatalf("URL-targeted controller InformURL = %q, want empty", c.InformURL)
	}
	if sim := StartDeviceSim(ctx, t, c, emu.DeviceSpec{MAC: "00:27:22:e0:00:42", Model: "USM8P"}); sim != nil {
		sim.Stop()
		t.Fatal("StartDeviceSim against a URL target returned a fleet, want nil")
	}
}
