package unifi

import "testing"

// TestDeviceDiffKeepsUntouchedPortOverrides is the regression guard for the
// interaction between the nil-as-empty encoder and the read-modify-write
// diff.
//
// Device's encoder renders nil port_overrides as [], because the controller
// rejects null there and [] is the only way to clear the field. Feed that
// through a diff against a device that has overrides and the diff reads it as
// "the caller asked for none", so changing a device's name would clear every
// port override on it.
//
// Worth pinning rather than trusting to review: before the encoder sent [],
// the same path sent null, which the controller rejected outright. The bug
// this guards is the upgrade from a loud failure to a silent one.
func TestDeviceDiffKeepsUntouchedPortOverrides(t *testing.T) {
	stored := &Device{
		ID:   "dev1",
		MAC:  "00:11:22:33:44:55",
		Name: "switch",
		PortOverrides: []DevicePortOverrides{
			{PortIDX: intPtr(1)},
			{PortIDX: intPtr(2)},
		},
	}

	// A caller renaming the device: everything else is left at its zero
	// value, port_overrides included.
	target := &Device{ID: "dev1", MAC: "00:11:22:33:44:55", Name: "renamed"}

	patch, err := getDeviceDiff(stored, target)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if _, ok := patch["port_overrides"]; ok {
		t.Errorf("a device the caller never touched port_overrides on produced %v in the patch, "+
			"which clears every override on the device", patch["port_overrides"])
	}
	if patch["name"] != "renamed" {
		t.Errorf("the field the caller did change is missing from the patch: %v", patch)
	}
}

// TestDeviceDiffClearsExplicitlyEmptyPortOverrides is the other half: an
// empty slice is not nil, and it still means "clear them".
func TestDeviceDiffClearsExplicitlyEmptyPortOverrides(t *testing.T) {
	stored := &Device{
		ID:            "dev1",
		MAC:           "00:11:22:33:44:55",
		PortOverrides: []DevicePortOverrides{{PortIDX: intPtr(1)}},
	}
	target := &Device{
		ID:            "dev1",
		MAC:           "00:11:22:33:44:55",
		PortOverrides: []DevicePortOverrides{},
	}

	patch, err := getDeviceDiff(stored, target)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	got, ok := patch["port_overrides"]
	if !ok {
		t.Fatal("an explicitly empty port_overrides must reach the wire; that is how the field is cleared")
	}
	list, ok := got.([]any)
	if !ok || len(list) != 0 {
		t.Errorf("port_overrides should clear as an empty list, got %#v", got)
	}
}

func intPtr(i int64) *int64 { return &i }
