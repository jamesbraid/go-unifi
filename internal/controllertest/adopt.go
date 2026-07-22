package controllertest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Device is the subset of a stat/device document tests assert on.
type Device struct {
	MAC     string `json:"mac"`
	Model   string `json:"model"`
	Type    string `json:"type"`
	IP      string `json:"ip"`
	Name    string `json:"name"`
	State   int    `json:"state"`
	Adopted bool   `json:"adopted"`
	// Unsupported marks models this controller build refuses to finish
	// adopting (their handshake ends in state=7). Not asserted on; tests
	// filter it out when picking a device to adopt.
	Unsupported bool `json:"unsupported"`
}

// AdoptDevice adopts mac and blocks until the controller reports it
// connected (state=1, adopted=true).
//
// This controller build answers devmgr adopt with api.err.CannotAdopt /
// api.err.CanNotAdoptUnknownDevice when the pending doc is seconds old, and
// a failed attempt can reap the doc entirely (the device's next inform
// re-creates it). A human re-clicks Adopt; AdoptDevice does the same for up
// to two minutes, treating the device doc as the source of truth — a
// CannotAdopt after the adopt already landed controller-side is not a
// failure. Any other rejection is fatal immediately.
func (c *Controller) AdoptDevice(ctx context.Context, t *testing.T, s *Session, mac string) Device {
	t.Helper()

	adoptDeadline := time.Now().Add(2 * time.Minute)
	for {
		body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/cmd/devmgr", map[string]string{
			"cmd": "adopt",
			"mac": mac,
		})
		if err != nil {
			t.Fatalf("adopt %s: %v", mac, err)
		}
		rc, msg := metaOf(body)
		if rc == "ok" {
			break
		}
		if !strings.Contains(strings.ToLower(msg), "cannotadopt") {
			t.Fatalf("adopt %s rejected: HTTP %d: %v", mac, status, body)
		}
		if d, ok, err := deviceByMAC(ctx, s, c.Site, mac); err == nil && ok && d.Adopted {
			t.Logf("adopt %s answered %s but the doc already shows adopted; continuing", mac, msg)
			break
		}
		if time.Now().After(adoptDeadline) {
			t.Fatalf("adopt %s still rejected (%s) after 2m; last response: %v", mac, msg, body)
		}
		t.Logf("adopt %s: %s; retrying", mac, msg)
		select {
		case <-ctx.Done():
			t.Fatalf("adopt %s: %v", mac, ctx.Err())
		case <-time.After(3 * time.Second):
		}
	}

	// The adopt landed; wait for the device to finish its inform handshake.
	connectDeadline := time.Now().Add(2 * time.Minute)
	var last Device
	for {
		d, ok, err := deviceByMAC(ctx, s, c.Site, mac)
		if err != nil {
			t.Fatalf("poll stat/device for %s: %v", mac, err)
		}
		if ok {
			last = d
			if d.State == 1 && d.Adopted {
				return d
			}
		}
		if time.Now().After(connectDeadline) {
			t.Fatalf("device %s never connected; last doc: %+v", mac, last)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for %s to connect: %v (last doc %+v)", mac, ctx.Err(), last)
		case <-time.After(2 * time.Second):
		}
	}
}

// listDevices returns every stat/device document in site.
func listDevices(ctx context.Context, t *testing.T, s *Session, site string) []Device {
	t.Helper()

	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/stat/device")
	if err != nil || status != 200 {
		t.Fatalf("stat/device: status=%d err=%v", status, err)
	}
	devices, err := decodeDevices(body)
	if err != nil {
		t.Fatalf("decode stat/device: %v (body %v)", err, body)
	}
	return devices
}

// deviceByMAC returns the stat/device doc for mac in site; ok=false when
// the controller does not list it (a reaped pending doc is absent, not an
// error — the device's next inform re-creates it).
func deviceByMAC(ctx context.Context, s *Session, site, mac string) (Device, bool, error) {
	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/stat/device")
	if err != nil {
		return Device{}, false, err
	}
	if status != 200 {
		return Device{}, false, fmt.Errorf("stat/device: HTTP %d: %v", status, body)
	}
	devices, err := decodeDevices(body)
	if err != nil {
		return Device{}, false, err
	}
	for _, d := range devices {
		if strings.EqualFold(d.MAC, mac) {
			return d, true, nil
		}
	}
	return Device{}, false, nil
}

// decodeDevices re-marshals the session's generic JSON into typed docs;
// the raw session deliberately returns undecoded JSON, so typed consumers
// pay the re-encode at their boundary.
func decodeDevices(body any) ([]Device, error) {
	m, ok := body.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected stat/device shape: %T", body)
	}
	raw, err := json.Marshal(m["data"])
	if err != nil {
		return nil, err
	}
	var devices []Device
	if err := json.Unmarshal(raw, &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

// metaOf unwraps a classic {"meta":{"rc":...,"msg":...}} envelope.
func metaOf(body any) (rc, msg string) {
	m, ok := body.(map[string]any)
	if !ok {
		return "", ""
	}
	meta, ok := m["meta"].(map[string]any)
	if !ok {
		return "", ""
	}
	rc, _ = meta["rc"].(string)
	msg, _ = meta["msg"].(string)
	return rc, msg
}
