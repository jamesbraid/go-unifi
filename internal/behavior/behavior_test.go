package behavior

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteThenLoadRoundTrips(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "schemas"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := Artifact{
		ControllerVersion: "10.6.101",
		Ownership: map[string]map[string][]string{
			"Network": {"setting_preference": {"igmp_snooping", "upnp_lan_enabled"}},
		},
		Discarded: map[string][]string{"Network": {"dhcpd_dns_enabled"}},
		Empty: map[string]map[string]EmptySemantics{
			"Network": {"dhcpd_gateway": {Empty: "EMPTY-REJECTED", Omit: "OMIT-CLEARS"}},
		},
		Coercions: map[string]map[string]Coercion{
			"SettingUsg": {"icmp_timeout": {Wrote: "1", Stored: "30"}},
		},
		Writes: map[string]WriteContract{
			"Nat": {
				CreateVerb: "POST", CreatePath: "rest/nat",
				UpdateVerb: "PUT", UpdatePath: "rest/nat/{id}",
				RequiredOnCreate: []string{"protocol", "source_filter"},
			},
		},
	}
	if err := Write(dir, want); err != nil {
		t.Fatal(err)
	}
	got, found, err := Load(dir)
	if err != nil || !found {
		t.Fatalf("Load: found=%v err=%v", found, err)
	}
	if got.ControllerVersion != "10.6.101" {
		t.Errorf("version = %q", got.ControllerVersion)
	}
	if got.Writes["Nat"].CreatePath != "rest/nat" {
		t.Errorf("write contract lost: %+v", got.Writes["Nat"])
	}
	if got.Coercions["SettingUsg"]["icmp_timeout"].Stored != "30" {
		t.Errorf("coercion lost: %+v", got.Coercions)
	}
}

// A missing artifact must degrade to "nothing measured", not an error, so a
// consumer predating it still builds.
func TestLoadMissingIsNotAnError(t *testing.T) {
	_, found, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("missing artifact errored: %v", err)
	}
	if found {
		t.Error("found reported true for a missing artifact")
	}
}

// Re-measuring the same behaviour must produce a byte-identical file, or every
// controller bump shows spurious diff noise and the reviewable-diff promise
// fails. Slice values are sorted on write for this reason.
func TestWriteIsDeterministic(t *testing.T) {
	a := Artifact{
		ControllerVersion: "10.6.101",
		Discarded:         map[string][]string{"Network": {"upnp_lan_enabled", "dhcpd_dns_enabled", "igmp_snooping"}},
	}
	d1, d2 := t.TempDir(), t.TempDir()
	for _, d := range []string{d1, d2} {
		if err := os.MkdirAll(filepath.Join(d, "schemas"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := Write(d1, a); err != nil {
		t.Fatal(err)
	}
	if err := Write(d2, a); err != nil {
		t.Fatal(err)
	}
	b1, _ := os.ReadFile(filepath.Join(d1, Path))
	b2, _ := os.ReadFile(filepath.Join(d2, Path))
	if string(b1) != string(b2) {
		t.Error("two writes of the same artifact differ")
	}
	if !strings.HasSuffix(string(b1), "\n") {
		t.Error("artifact does not end with a newline")
	}
	// sorted, not insertion order
	if !strings.Contains(string(b1), `"dhcpd_dns_enabled",`) {
		t.Fatal("expected member missing")
	}
	if strings.Index(string(b1), "dhcpd_dns_enabled") > strings.Index(string(b1), "igmp_snooping") {
		t.Error("discarded slice was not sorted")
	}
}
