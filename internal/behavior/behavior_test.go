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
			"OSPFRouter": {
				CreateVerb: "POST", CreatePath: "rest/ospf",
				MinItems: map[string]int{"areas[].network_ids": 1},
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
	if got.Writes["OSPFRouter"].MinItems["areas[].network_ids"] != 1 {
		t.Errorf("min-items lost: %+v", got.Writes["OSPFRouter"])
	}
	if got.Coercions["SettingUsg"]["icmp_timeout"].Stored != "30" {
		t.Errorf("coercion lost: %+v", got.Coercions)
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
	// Writing twice and comparing bytes proved nothing: Write sorts through
	// the artifact's maps, which are shared with the caller, so the second
	// call marshals an already-sorted value. The sort itself is the thing to
	// assert.
	d1 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d1, "schemas"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write(d1, a); err != nil {
		t.Fatal(err)
	}
	b1, _ := os.ReadFile(filepath.Join(d1, Path))
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
