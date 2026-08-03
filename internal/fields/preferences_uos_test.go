package fields

import (
	"slices"
	"testing"

	"github.com/BurntSushi/toml"
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

// TestDecodeRejectsMisspelledPreferenceProperty is the guard for the failure
// mode TOML makes easy and quiet.
//
// A key no struct field claims is skipped, not rejected, so "onws" leaves
// Owns empty while "measured" still decodes and every other check passes. An
// empty Owns is a measured result in this table -- the mode was probed and
// governs nothing -- so the typo publishes a real-looking claim that the mode
// owns nothing at all.
//
// Exercises the same decode-and-inspect the loader does, on a fixture rather
// than the real file, since the real file has to stay valid.
func TestDecodeRejectsMisspelledPreferenceProperty(t *testing.T) {
	const doc = `
[Thing.preference.setting_preference]
onws = ["igmp_snooping"]
measured = "10.4.57"
`
	var file map[string]preferenceFile
	md, err := toml.Decode(doc, &file)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	// The typo decodes clean and leaves a mode that owns nothing.
	if got := file["Thing"].Preference["setting_preference"].Owns; len(got) != 0 {
		t.Fatalf("expected the misspelling to leave owns empty, got %v", got)
	}

	var unknown []string
	for _, key := range md.Undecoded() {
		if len(key) == 4 && key[1] == "preference" {
			unknown = append(unknown, key.String())
		}
	}
	if len(unknown) != 1 || unknown[0] != "Thing.preference.setting_preference.onws" {
		t.Errorf("the misspelled property was not reported as undecoded, got %v", unknown)
	}
}

// TestDecodeAcceptsEveryRealPreferenceProperty keeps the check above from
// rejecting the properties that do exist -- a guard that fails the whole
// table would be worse than the typo it catches.
func TestDecodeAcceptsEveryRealPreferenceProperty(t *testing.T) {
	const doc = `
[Thing.preference.setting_preference]
owns = ["igmp_snooping"]
uos_excludes = ["igmp_snooping"]
measured = "10.4.57"
`
	var file map[string]preferenceFile
	md, err := toml.Decode(doc, &file)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range md.Undecoded() {
		if len(key) == 4 && key[1] == "preference" {
			t.Errorf("%s is a real property but reads as unknown", key.String())
		}
	}

	entry := file["Thing"].Preference["setting_preference"]
	if len(entry.Owns) != 1 || len(entry.UOSExcludes) != 1 || entry.Measured == "" {
		t.Errorf("a fully populated entry did not round-trip: %+v", entry)
	}
}
