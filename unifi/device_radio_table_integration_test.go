//go:build integration

package unifi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationUpdateDeviceRadioTable drives the masked radio write against
// real adopted access points, and pins the two controller behaviours the
// writer rests on.
//
// The first is the key. A radio_table entry is addressed by name -- the
// interface name the AP reports, wifi-ng and its kin -- and this checks that
// on two models with different radio counts, because a key that is unique on
// one AP and ambiguous on the next would write one radio's settings onto
// another. The controller agrees: it refuses a name no radio of the device
// carries, which is why the last arm here expects an error rather than a
// silently misplaced write.
//
// The second is the merge. Measured on 10.6.101, radio_table merges on both
// levels: a member the entry omits keeps its stored value, and a radio the
// array omits keeps its whole entry. That is the opposite of port_overrides,
// which replaces on both levels in the same document -- so the two writers
// differ, and this is what says the difference is real.
func TestIntegrationUpdateDeviceRadioTable(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	emulated := controllertest.StartDevices(ctx, t, c,
		controllertest.DeviceRequest{Model: "U7PRO"},
		controllertest.DeviceRequest{Model: "U6M"},
	)
	if len(emulated) != 2 {
		t.Skip("no emulated access points available for this controller target")
	}

	client := harnessClient(ctx, t, c)

	// stored reads the radio table as the controller holds it, including the
	// members this client does not model.
	stored := func(mac string) map[string]map[string]any {
		body, status, err := s.GetJSON(ctx, "/api/s/"+c.Site+"/stat/device/"+mac)
		if err != nil || status != 200 {
			t.Fatalf("stat/device/%s (HTTP %d): %v", mac, status, err)
		}
		raw, _ := json.Marshal(firstData(t, body)["radio_table"])
		var entries []map[string]any
		if err := json.Unmarshal(raw, &entries); err != nil {
			t.Fatalf("radio_table of %s is not an array of objects: %v", mac, err)
		}
		out := make(map[string]map[string]any, len(entries))
		for _, entry := range entries {
			name, _ := entry["name"].(string)
			out[name] = entry
		}
		return out
	}

	// The AP keeps informing after adoption, so a read taken straight after a
	// write can still show the table as it was. Wait for this write's own
	// value rather than for any table.
	settled := func(mac, radio, wire string, want any) map[string]map[string]any {
		t.Helper()
		deadline := time.Now().Add(60 * time.Second)
		for {
			table := stored(mac)
			if jsonEqual(table[radio][wire], want) {
				return table
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s never reported %s %s = %v; last read: %v", mac, radio, wire, want, table[radio])
			}
			time.Sleep(2 * time.Second)
		}
	}

	var target *Device
	for _, e := range emulated {
		adopted := c.AdoptDevice(ctx, t, s, e.MAC)
		device, err := client.GetDeviceByMAC(ctx, c.Site, adopted.MAC)
		if err != nil {
			t.Fatalf("resolve the adopted %s: %v", e.Model, err)
		}

		deadline := time.Now().Add(3 * time.Minute)
		for len(device.RadioTable) == 0 {
			if time.Now().After(deadline) {
				t.Fatalf("%s reported no radio_table, so nothing here can be measured", e.Model)
			}
			time.Sleep(3 * time.Second)
			if device, err = client.GetDeviceByMAC(ctx, c.Site, adopted.MAC); err != nil {
				t.Fatalf("re-read the adopted %s: %v", e.Model, err)
			}
		}

		seen := map[string]bool{}
		for i, radio := range device.RadioTable {
			if radio.Name == "" {
				t.Errorf("%s radio %d carries no name (%+v); the write addresses radios by name",
					e.Model, i, radio)
			}
			if seen[radio.Name] {
				t.Errorf("%s reports two radios named %q; the name no longer identifies a radio "+
					"on this model, and a write keyed on it would land on either", e.Model, radio.Name)
			}
			seen[radio.Name] = true
			t.Logf("%s radio %d: name=%q radio=%q", e.Model, i, radio.Name, radio.Radio)
		}
		if target == nil || len(device.RadioTable) > len(target.RadioTable) {
			target = device
		}
	}
	if len(target.RadioTable) < 2 {
		t.Skip("no adopted AP has two radios, so nothing here can show one radio surviving a write to another")
	}

	// Give two radios a value the AP never reports on its own, so what
	// follows can tell a preserved setting from a re-reported one.
	if _, err := client.UpdateDeviceRadioTable(ctx, c.Site, target,
		[]DeviceRadioTable{{Name: "wifi-na", Maxsta: ptrInt64(42)}}, "maxsta"); err != nil {
		t.Fatalf("seed wifi-na: %v", err)
	}
	settled(target.MAC, "wifi-na", "maxsta", 42)

	if _, err := client.UpdateDeviceRadioTable(ctx, c.Site, target,
		[]DeviceRadioTable{{Name: "wifi-ng", Maxsta: ptrInt64(77)}}, "maxsta"); err != nil {
		t.Fatalf("seed wifi-ng: %v", err)
	}
	settled(target.MAC, "wifi-ng", "maxsta", 77)

	// Change a different member of one radio, and say nothing about the rest.
	if _, err := client.UpdateDeviceRadioTable(ctx, c.Site, target,
		[]DeviceRadioTable{{Name: "wifi-na", MinRssiEnabled: true, MinRssi: ptrInt64(-80)}},
		"min_rssi_enabled", "min_rssi"); err != nil {
		t.Fatalf("UpdateDeviceRadioTable: %v", err)
	}
	after := settled(target.MAC, "wifi-na", "min_rssi_enabled", true)

	if got := after["wifi-na"]["maxsta"]; !jsonEqual(got, 42) {
		t.Errorf("wifi-na maxsta = %v, want 42.\n\n"+
			"A member the mask did not name must survive. If the controller has stopped "+
			"merging radio_table, this writer has to resend the stored entry the way "+
			"UpdateDevicePortOverrides does.", got)
	}
	if got := after["wifi-ng"]["maxsta"]; !jsonEqual(got, 77) {
		t.Errorf("wifi-ng maxsta = %v, want 77; a radio the write did not name must survive", got)
	}
	if _, ok := after["wifi-na"]["nss"]; !ok {
		t.Error("wifi-na lost nss, a member this client does not model at all -- exactly what a " +
			"write built from the Go struct drops")
	}
	if got := after["wifi-na"]["radio"]; got != "na" {
		t.Errorf("wifi-na radio = %v, want na; the write named neither radio nor channel", got)
	}

	// A name no radio carries is the controller's own rejection, not a write
	// that lands somewhere else.
	if _, err := client.UpdateDeviceRadioTable(ctx, c.Site, target,
		[]DeviceRadioTable{{Name: "wifi-nope", Maxsta: ptrInt64(11)}}, "maxsta"); err == nil {
		t.Error("a radio the device does not have was accepted; the name is supposed to be the " +
			"controller's own key, and an unknown one refused")
	} else {
		t.Logf("unknown radio name refused: %v", err)
	}
}
