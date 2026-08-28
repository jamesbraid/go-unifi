//go:build integration

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationUpdateDevicePortOverrides drives the keyed overlay against
// a real adopted switch, and pins the two controller behaviours that make it
// necessary.
//
// Measured on 10.6.101, port_overrides replaces at both levels: an entry the
// payload leaves out is dropped, and a member an entry leaves out is dropped
// from that entry. So a caller changing one port's PoE mode has to resend
// every other port and every other member of that port -- including members
// this client does not model, which is why the merge is done on raw JSON.
func TestIntegrationUpdateDevicePortOverrides(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	emulated := controllertest.StartDevices(ctx, t, c, controllertest.DeviceRequest{Model: "USM8P"})
	if len(emulated) != 1 {
		t.Skip("no emulated switch available for this controller target")
	}
	adopted := c.AdoptDevice(ctx, t, s, emulated[0].MAC)

	client := harnessClient(ctx, t, c)
	device, err := client.GetDeviceByMAC(ctx, c.Site, adopted.MAC)
	if err != nil {
		t.Fatalf("resolve the adopted switch: %v", err)
	}

	// Seed two ports. Port 1 carries isolation:false deliberately: every
	// member of the generated struct is omitempty, so a member sitting at
	// its zero value is exactly what a struct round-trip drops, and the
	// controller replaces the entry with whatever it receives.
	//
	// A member the struct does not model at all would make the same point
	// more starkly, but the generated type currently covers everything this
	// controller stores here, and the controller strips a key it does not
	// recognise -- so a synthetic one cannot be seeded to prove it.
	seed := []any{
		map[string]any{"port_idx": 1, "name": "probe-one", "poe_mode": "auto", "isolation": false, "autoneg": true},
		map[string]any{"port_idx": 2, "name": "probe-two", "poe_mode": "auto"},
	}
	seedBody, status, err := s.PutJSON(ctx, "/api/s/"+c.Site+"/rest/device/"+device.ID,
		map[string]any{"port_overrides": seed})
	if err != nil || status != 200 {
		t.Fatalf("seeding port overrides (HTTP %d): %v", status, err)
	}
	seedRaw, _ := json.Marshal(seedBody)
	t.Logf("seed response: %s", seedRaw)

	stored := func() map[string]map[string]any {
		body, status, err := s.GetJSON(ctx, "/api/s/"+c.Site+"/stat/device")
		if err != nil || status != 200 {
			t.Fatalf("list devices (HTTP %d): %v", status, err)
		}
		m, _ := body.(map[string]any)
		data, _ := m["data"].([]any)
		for _, e := range data {
			d, _ := e.(map[string]any)
			if got, _ := d["mac"].(string); got != device.MAC {
				continue
			}
			raw, _ := json.Marshal(d["port_overrides"])
			var entries []map[string]any
			_ = json.Unmarshal(raw, &entries)
			out := map[string]map[string]any{}
			for _, entry := range entries {
				out[fmt.Sprintf("%v", entry["port_idx"])] = entry
			}
			return out
		}
		t.Fatalf("device %s vanished", device.MAC)
		return nil
	}

	// The device keeps informing after adoption, so give the controller a
	// moment to settle before reading the seed back.
	var before map[string]map[string]any
	for attempt := range 10 {
		before = stored()
		if _, ok := before["1"]; ok {
			break
		}
		t.Logf("attempt %d: port 1 override not stored yet (%d entries)", attempt, len(before))
		time.Sleep(3 * time.Second)
	}
	if _, ok := before["1"]["isolation"]; !ok {
		t.Fatalf("the seed did not store isolation on port 1, so this run cannot "+
			"show a zero-valued member surviving: %v", before["1"])
	}

	// Change one member of port 1 and say nothing about port 2.
	if _, err := client.UpdateDevicePortOverrides(ctx, c.Site, device,
		[]DevicePortOverrides{{PortIDX: ptrInt64(1), PoeMode: "off"}}, "poe_mode"); err != nil {
		t.Fatalf("UpdateDevicePortOverrides: %v", err)
	}

	after := stored()
	if got := after["1"]["poe_mode"]; got != "off" {
		t.Errorf("port 1 poe_mode = %v, want off", got)
	}
	if got := after["1"]["name"]; got != "probe-one" {
		t.Errorf("port 1 lost its name (%v); a member the mask did not name must survive", got)
	}
	if _, ok := after["1"]["isolation"]; !ok {
		t.Errorf("port 1 lost isolation.\n\n" +
			"It was stored as false, and every member of the generated struct is " +
			"omitempty, so this is what a merge done on the Go structs drops. Merging " +
			"the raw JSON is what keeps it.")
	}
	if got := after["1"]["autoneg"]; got != true {
		t.Errorf("port 1 autoneg = %v, want true", got)
	}
	if _, ok := after["2"]; !ok {
		t.Error("port 2 was dropped; an entry the caller said nothing about must survive")
	} else if got := after["2"]["name"]; got != "probe-two" {
		t.Errorf("port 2 name = %v, want probe-two", got)
	}

	// A port with no stored entry is created rather than refused.
	if _, err := client.UpdateDevicePortOverrides(ctx, c.Site, device,
		[]DevicePortOverrides{{PortIDX: ptrInt64(5), PoeMode: "off"}}, "poe_mode"); err != nil {
		t.Fatalf("adding an override for a port that had none: %v", err)
	}
	added := stored()
	if _, ok := added["5"]; !ok {
		t.Error("port 5 was not created; adding an override to a port that had none is how a caller adds one")
	}
	if _, ok := added["1"]; !ok {
		t.Error("adding port 5 dropped port 1")
	}
}
