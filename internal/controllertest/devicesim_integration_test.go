//go:build integration

package controllertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestIntegrationDeviceSimAdoption runs an emulated switch beside a fresh
// controller end to end: inform, pending stat/device doc, AdoptDevice,
// connected. The herder allocates the identity from a locally administered
// MAC range, so it can never collide with the -sim image's seeded demo fleet
// and the stat/device lookup can only match this device.
func TestIntegrationDeviceSimAdoption(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	c := Start(ctx, t)
	s := c.NewSession(ctx, t)

	devices := StartDevices(ctx, t, c, DeviceRequest{Model: "USM8P"})
	if devices == nil {
		t.Skip("no emulated devices available for this controller target")
	}
	if len(devices) != 1 {
		t.Fatalf("herder started %d devices, want 1", len(devices))
	}
	mac := devices[0].MAC

	// A couple of inform cycles should land the pending doc; 2m is the
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
}

// TestIntegrationStartDevicesNilForURLTarget pins the guard: a UNIFI_TEST_URL
// controller owns no network, so StartDevices returns nil rather than
// starting containers that would inform an external controller. The stub
// login server drives Start's URL branch, so no container boots.
func TestIntegrationStartDevicesNilForURLTarget(t *testing.T) {
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
	if c.Network != "" || c.InformURL != "" {
		t.Fatalf("URL-targeted controller Network = %q InformURL = %q, want both empty", c.Network, c.InformURL)
	}
	if devices := StartDevices(ctx, t, c, DeviceRequest{Model: "USM8P"}); devices != nil {
		t.Fatalf("StartDevices against a URL target returned %+v, want nil", devices)
	}
}
