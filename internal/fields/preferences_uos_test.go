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

// TestOwnsOnWithoutExclusionsIsIdentical keeps the common case honest -- an
// entry with no exclusions answers the same on both harnesses.
func TestOwnsOnWithoutExclusionsIsIdentical(t *testing.T) {
	p := Preference{Owns: []string{"cron_expr"}}

	if got := p.OwnsOn(true); !slices.Equal(got, p.Owns) {
		t.Errorf("no exclusions should mean no difference, got %v", got)
	}
	if got := p.OwnsOn(false); !slices.Equal(got, p.Owns) {
		t.Errorf("standalone changed an entry with no exclusions: %v", got)
	}
}

// TestSuperMgmtRetentionExclusionsAreRecorded reads the real table, so the
// measurement behind this cannot be deleted from fields.toml without a
// failure. The entry is the only reason the UOS integration leg passes.
func TestSuperMgmtRetentionExclusionsAreRecorded(t *testing.T) {
	prefs, err := LoadPreferences()
	if err != nil {
		t.Fatalf("load overrides/fields.toml: %v", err)
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
			t.Errorf("%s is not in uos_excludes; UniFi OS pins it under both modes (measured: "+
				"asking 1 stores 24), so the UOS integration leg will fail on it", wire)
		}
		if !slices.Contains(entry.Owns, wire) {
			t.Errorf("%s left owns; the standalone controller does give it to manual mode, and "+
				"an exclusion is only meaningful against a recorded ownership", wire)
		}
	}
}
