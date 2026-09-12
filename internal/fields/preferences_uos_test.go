package fields

import (
	"slices"
	"testing"
)

// TestOwnsOnSubtractsUOSExclusions pins the two answers one entry carries.
//
// The standalone set is the entry as written. The UniFi OS set is that minus
// the fields the console pins, and it has to be a real subtraction rather
// than a different list: the standalone measurement stays true, and keeping
// one source for both is what stops the two drifting apart.
func TestOwnsOnSubtractsUOSExclusions(t *testing.T) {
	p := Preference{
		Owns: []string{
			"data_retention_time_in_hours_for_5minutes_scale",
			"data_retention_time_in_hours_for_hourly_scale",
			"data_retention_time_in_hours_for_others",
		},
		UOSExcludes: []string{
			"data_retention_time_in_hours_for_5minutes_scale",
			"data_retention_time_in_hours_for_hourly_scale",
		},
	}

	standalone := p.OwnsOn(false)
	if len(standalone) != 3 {
		t.Errorf("standalone should see the entry unchanged, got %v", standalone)
	}

	uos := p.OwnsOn(true)
	want := []string{"data_retention_time_in_hours_for_others"}
	if !slices.Equal(uos, want) {
		t.Errorf("UOS set = %v, want %v", uos, want)
	}

	// Subtracting must not disturb the recorded list: the standalone
	// measurement is the thing being preserved.
	if len(p.Owns) != 3 {
		t.Errorf("OwnsOn mutated the entry: %v", p.Owns)
	}
}

// TestSuperMgmtRetentionExclusionsAreRecorded reads the real record, so
// neither the uos_pins nor the owns in schemas/behavior.json can be deleted
// without a failure. The exclusions are the only reason the UOS integration
// leg passes.
func TestSuperMgmtRetentionExclusionsAreRecorded(t *testing.T) {
	prefs, err := LoadPreferences()
	if err != nil {
		t.Fatalf("load the recorded ownership: %v", err)
	}

	entry, ok := prefs["SettingSuperMgmt"]["data_retention_setting_preference"]
	if !ok {
		t.Fatal("SettingSuperMgmt.data_retention_setting_preference is gone from the table")
	}
	for _, wire := range []string{
		"data_retention_time_in_hours_for_5minutes_scale",
		"data_retention_time_in_hours_for_hourly_scale",
	} {
		if !slices.Contains(entry.UOSExcludes, wire) {
			t.Errorf("%s is not in uos_pins; UniFi OS pins it under both modes (measured: "+
				"asking 1 stores 24), so the UOS integration leg will fail on it", wire)
		}
		if !slices.Contains(entry.Owns, wire) {
			t.Errorf("%s left owns; the standalone controller does give it to manual mode, and "+
				"a pin is only meaningful against a recorded ownership", wire)
		}
	}
}
