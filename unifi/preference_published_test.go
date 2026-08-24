package unifi

import (
	"slices"
	"testing"
)

// TestPublishedPreferenceCarriesUOSExclusions pins what consumers see.
//
// PreferenceOwnedFields is the only published record of which fields a mode
// takes over, and a Terraform provider settling the mode at plan time reads
// it. Publishing Owns alone would hand every consumer the standalone answer
// with nothing marking it as one of two -- so a provider talking to a UniFi
// OS console would describe two retention windows as mode-owned when the
// console pins them under either mode, and be wrong with no way to know.
func TestPublishedPreferenceCarriesUOSExclusions(t *testing.T) {
	var pref Preference
	for _, p := range PreferenceOwnedFields["SettingSuperMgmt"] {
		if p.Mode == "data_retention_setting_preference" {
			pref = p
		}
	}
	if pref.Mode == "" {
		t.Fatal("SettingSuperMgmt.data_retention_setting_preference is not published")
	}

	for _, wire := range []string{
		"data_retention_time_in_hours_for_5minutes_scale",
		"data_retention_time_in_hours_for_hourly_scale",
	} {
		if !slices.Contains(pref.UOSExcludes, wire) {
			t.Errorf("%s is missing from the published UOSExcludes; a consumer on UniFi OS would "+
				"treat a console-pinned field as settable", wire)
		}
	}

	// Both answers have to be reachable from the one entry.
	if got := pref.OwnsOn(false); len(got) != 3 {
		t.Errorf("standalone OwnsOn = %v, want all three", got)
	}
	if got := pref.OwnsOn(true); !slices.Equal(got, []string{"data_retention_time_in_hours_for_others"}) {
		t.Errorf("UniFi OS OwnsOn = %v, want only the others window", got)
	}
}

// TestPublishedExclusionsAreSubsetsOfOwns holds the invariant the type's doc
// states. An exclusion naming a field that is not owned describes nothing,
// and would quietly mean the generator or the table has drifted.
func TestPublishedExclusionsAreSubsetsOfOwns(t *testing.T) {
	for resource, prefs := range PreferenceOwnedFields {
		for _, p := range prefs {
			for _, wire := range p.UOSExcludes {
				if !slices.Contains(p.Owns, wire) {
					t.Errorf("%s.%s excludes %q on UniFi OS but does not own it anywhere",
						resource, p.Mode, wire)
				}
			}
		}
	}
}
