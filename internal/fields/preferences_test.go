package fields

import (
	"slices"
	"strings"
	"testing"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
)

// TestLoadPreferencesReadsTheRealFile checks the loader against the real
// schemas/behavior.json and overrides/fields.toml rather than fixtures. The
// point of the merged view is that the generator and the integration test
// read the same facts, so a fixture would test the struct tags and miss the
// thing that matters.
func TestLoadPreferencesReadsTheRealFile(t *testing.T) {
	loaded, err := LoadPreferences()
	if err != nil {
		t.Fatalf("LoadPreferences: %v", err)
	}

	network, ok := loaded["Network"]
	if !ok {
		t.Fatalf("no preference entries for Network; got %d resource(s)", len(loaded))
	}

	mode, ok := network["setting_preference"]
	if !ok {
		t.Fatal("no entry for Network.setting_preference")
	}
	if len(mode.Owns) == 0 {
		t.Error("Network.setting_preference owns nothing; the controller was measured owning eleven fields")
	}
	if mode.Measured == "" {
		t.Error("Network.setting_preference records no measured build, so a stale set cannot be told from a current one")
	}

	// An entry that owns nothing is a result, not an absence, and it has to
	// survive the round trip through TOML to stay one.
	if _, ok := network["wan_ipv6_dns_preference"]; !ok {
		t.Error("Network.wan_ipv6_dns_preference is missing: a measured empty set must load, not vanish")
	}
}

// TestMergePreferences pins how the artifact and the residual table combine:
// the artifact is the measurement, the table adds only what the artifact has
// no slot for, and anything else in the table is a drift hazard and errors.
func TestMergePreferences(t *testing.T) {
	artifact := behavior.Artifact{
		ControllerVersion: "10.6.101",
		Ownership: map[string]map[string][]string{
			"Network": {"setting_preference": {"domain_name", "igmp_snooping"}},
			"Setting": {"data_retention_setting_preference": {"scale_a", "scale_b"}},
		},
	}

	merged, err := mergePreferences(artifact, map[string]map[string]Preference{
		"Setting": {
			"data_retention_setting_preference": {UOSExcludes: []string{"scale_a"}, Measured: "10.6.101"},
		},
		"Device": {
			"port_overrides.setting_preference": {Owns: []string{}, Measured: "10.6.101"},
		},
	})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	network := merged["Network"]["setting_preference"]
	if !slices.Equal(network.Owns, []string{"domain_name", "igmp_snooping"}) || network.Measured != "10.6.101" {
		t.Errorf("artifact entry did not carry through: %+v", network)
	}
	setting := merged["Setting"]["data_retention_setting_preference"]
	if !slices.Equal(setting.Owns, []string{"scale_a", "scale_b"}) ||
		!slices.Equal(setting.UOSExcludes, []string{"scale_a"}) {
		t.Errorf("uos_excludes overlay did not land on the artifact's owns: %+v", setting)
	}
	if _, ok := merged["Device"]["port_overrides.setting_preference"]; !ok {
		t.Error("a mode the artifact does not cover must pass through from the table")
	}
}

// TestMergePreferencesRejectsDrift covers the table entries that must not
// survive beside the artifact: a hand-stamped copy of a measurement the
// artifact already records, an entry that adds nothing, and exclusions whose
// provenance is missing or names a different build than the artifact's.
func TestMergePreferencesRejectsDrift(t *testing.T) {
	artifact := behavior.Artifact{
		ControllerVersion: "10.6.101",
		Ownership: map[string]map[string][]string{
			"Setting": {"setting_preference": {"scale_a"}},
		},
	}

	for _, tc := range []struct {
		name    string
		entry   Preference
		wantErr string
	}{
		{
			name:    "owns duplicated from the artifact",
			entry:   Preference{Owns: []string{"scale_a"}, Measured: "10.6.101"},
			wantErr: "already measures",
		},
		{
			name:    "entry adds nothing",
			entry:   Preference{Measured: "10.6.101"},
			wantErr: "adds nothing",
		},
		{
			name:    "exclusions without provenance",
			entry:   Preference{UOSExcludes: []string{"scale_a"}},
			wantErr: "no measured build",
		},
		{
			name:    "exclusions measured on another build",
			entry:   Preference{UOSExcludes: []string{"scale_a"}, Measured: "10.4.57"},
			wantErr: "re-measure the UniFi OS harness",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mergePreferences(artifact, map[string]map[string]Preference{
				"Setting": {"setting_preference": tc.entry},
			})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
