//go:build integration

// unifi/device_port_override_integration_test.go
package unifi

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

// TestIntegrationDevicePortOverridePreference measures the last of the
// sixteen auto|manual modes, and the last unmeasured null write.
//
// Both concern port_overrides, which is why they share an adopted device:
// adoption is the expensive part of this file, and running it twice to
// answer two questions about the same field would be wasteful.
//
//	setting_preference   nested one level down, inside each port override
//	port_overrides       a slice serialized unconditionally, so it marshals
//	                     as null when the caller sets no overrides
//
// The mode is per element. Each override carries its own, governing that
// port, which is why the preference table addresses it relatively.
func TestIntegrationDevicePortOverridePreference(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	device := controllertest.AdoptGateway(ctx, t, c, s)

	// The harness identifies a device by MAC; the REST collection wants the
	// controller's own id.
	id := deviceIDForMAC(ctx, t, s, c.Site, device.MAC)
	if id == "" {
		t.Skipf("adopted %s but the controller lists no device with that MAC", device.MAC)
	}
	// Devices are read from stat/device and written to rest/device: the
	// collection is split, and using the write path for a read answers 404.
	writePath := "/api/s/" + c.Site + "/rest/device/" + id
	// stat/device is keyed by MAC, rest/device by the controller's id: the
	// device collection is split by both path and identifier.
	readPath := "/api/s/" + c.Site + "/stat/device/" + device.MAC

	// override builds a port override rich enough to show what the mode
	// takes over, under the given mode.
	override := func(mode string) map[string]any {
		return map[string]any{
			"port_idx":           1,
			"setting_preference": mode,
			"name":               "probe-port",
			"op_mode":            "switch",
			"autoneg":            false,
			"full_duplex":        true,
			"isolation":          true,
			"lldpmed_enabled":    false,
			"stp_port_mode":      false,
			"eee_enabled":        true,
			"poe_mode":           "off",
		}
	}

	// stat/device lags a write by a second or two, so a read taken straight
	// after a PUT shows the object as it was. Reading once made a successful
	// write look like a discarded one.
	//
	// settled has to identify the write that just happened, not merely that
	// the field is populated. The second arm runs against an object the
	// first arm already populated, so "port_overrides is non-nil" is true
	// before the second write lands, and the arm would measure the first
	// arm's object -- or, once the mode check caught that, skip.
	write := func(t *testing.T, body map[string]any, settled func(map[string]any) bool) (map[string]any, bool) {
		t.Helper()
		resp, status, err := s.PutJSON(ctx, writePath, body)
		if err != nil || status != 200 {
			t.Logf("write rejected (HTTP %d): %v %v", status, resp, err)
			return nil, false
		}

		deadline := time.Now().Add(20 * time.Second)
		for {
			fresh, status, err := s.GetJSON(ctx, readPath)
			if err != nil || status != 200 {
				t.Fatalf("re-read failed (HTTP %d): %v", status, err)
			}
			stored := firstData(t, fresh)
			if settled == nil || settled(stored) {
				return stored, true
			}
			if time.Now().After(deadline) {
				t.Logf("the device never settled on the written value within the deadline")
				return stored, true
			}
			time.Sleep(time.Second)
		}
	}

	t.Run("port_overrides.setting_preference ownership", func(t *testing.T) {
		measure := func(mode string) map[string]string {
			asked := override(mode)
			// Settled means this arm's own mode is visible, which is the
			// only signal that distinguishes it from the arm before.
			stored, ok := write(t, map[string]any{"_id": id, "port_overrides": []any{asked}},
				func(fresh map[string]any) bool {
					return firstPortOverrideMode(fresh) == mode
				})
			if !ok {
				t.Skipf("the %s arm was rejected, so ownership here is unmeasured", mode)
			}
			scope := descend(t, stored, "port_overrides", "the stored device")
			if scope == nil {
				t.Skip("port_overrides did not survive the write, so nothing inside it can be measured")
			}
			if got, _ := scope[preferenceModeWire].(string); got != mode {
				t.Skipf("asked for %s = %q, and the device still reported %q after waiting for it "+
					"to settle; the arm did not run under its own mode, so anything measured from "+
					"it would be the previous arm's object",
					preferenceModeWire, mode, got)
			}
			return discardedFields(asked, scope)
		}

		manual := measure("manual")
		auto := measure("auto")

		owned := []string{}
		for wire, detail := range auto {
			if _, both := manual[wire]; both || wire == preferenceModeWire {
				continue
			}
			owned = append(owned, wire)
			t.Logf("OWNED %s (%s)", wire, detail)
		}
		sort.Strings(owned)
		for wire, detail := range manual {
			t.Logf("refused under both modes, not this mode's doing: %s (%s)", wire, detail)
		}

		recorded, err := fields.LoadPreferences()
		if err != nil {
			t.Fatalf("load overrides/fields.toml: %v", err)
		}
		entry, ok := recorded["Device"]["port_overrides.setting_preference"]
		if !ok {
			t.Fatalf("no ownership recorded for Device.port_overrides.setting_preference; measured %v", owned)
		}
		if diff := comparePreference(entry.OwnsOn(onUOSHarness()), owned, manual); diff != "" {
			t.Errorf("Device.port_overrides.setting_preference no longer matches overrides/fields.toml "+
				"(harness: %s):\n%s\n\nThe controller moved or the table was wrong. Re-measure before "+
				"editing it.", harnessName(), diff)
		}
	})

	// Measured on 10.4.57: rejected, the third field of this shape to be so
	// after WLAN.schedule_with_duration and APGroup.device_macs. A Device
	// built without touching port_overrides marshals null there, so it
	// cannot be written at all.
	t.Run("port_overrides null", func(t *testing.T) {
		if _, ok := write(t, map[string]any{"_id": id, "port_overrides": nil}, nil); ok {
			t.Error("null port_overrides was accepted. The controller stopped rejecting it, which " +
				"is worth knowing: it is one of the reasons the client cannot send this field unset.")
		}
	})
}

// firstPortOverrideMode reads the mode off the first port override, or ""
// when the device carries none.
//
// Deliberately quiet: it runs on every poll iteration, and descend's logging
// would report the same not-yet-settled state a dozen times per write.
func firstPortOverrideMode(device map[string]any) string {
	overrides, ok := device["port_overrides"].([]any)
	if !ok || len(overrides) == 0 {
		return ""
	}
	first, ok := overrides[0].(map[string]any)
	if !ok {
		return ""
	}
	mode, _ := first[preferenceModeWire].(string)
	return mode
}

// deviceIDForMAC resolves an adopted device's controller id.
func deviceIDForMAC(ctx context.Context, t *testing.T, s *controllertest.Session, site, mac string) string {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/stat/device")
	if err != nil || status != 200 {
		t.Logf("cannot list devices (HTTP %d): %v", status, err)
		return ""
	}
	envelope, ok := body.(map[string]any)
	if !ok {
		return ""
	}
	items, _ := envelope["data"].([]any)
	for _, item := range items {
		d, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if got, _ := d["mac"].(string); got == mac {
			id, _ := d["_id"].(string)
			return id
		}
	}
	return ""
}

// preferenceModeWire is the mode field's wire name inside a port override.
const preferenceModeWire = "setting_preference"
