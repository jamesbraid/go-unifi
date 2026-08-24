package fields

import "testing"

// TestLoadPreferencesReadsTheRealFile checks the loader against
// overrides/fields.toml itself rather than a fixture. The point of the file
// is that the generator and the integration test read the same bytes, so a
// fixture would test the struct tags and miss the thing that matters.
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
		t.Error("Network.setting_preference owns nothing; the controller was measured owning twelve fields")
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
