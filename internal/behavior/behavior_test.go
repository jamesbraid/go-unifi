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
			"Nat":     {"ip_address": {Empty: "EMPTY-REJECTED", Omit: "OMIT-VARIES-BY-TYPE"}},
		},
		EmptyWhen: map[string]map[string]map[string]EmptySemantics{
			"Nat": {
				"ip_address": {
					"type=DNAT":       {Empty: "EMPTY-REJECTED", Omit: "OMIT-REJECTED"},
					"type=SNAT":       {Empty: "EMPTY-REJECTED", Omit: "OMIT-REJECTED"},
					"type=MASQUERADE": {Empty: "EMPTY-REJECTED", Omit: "OMIT-OK"},
				},
			},
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
			"Network": {
				CreateVerb: "POST", CreatePath: "rest/networkconf",
				RequiredOnCreateWhen: map[string][]string{
					"purpose=corporate": {"vlan_enabled", "vlan"},
					"purpose=wan":       {},
				},
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
	// A branch measured to require nothing has to survive as an empty list.
	// Losing it would make "no field is required for a WAN create" look like
	// "nobody measured a WAN create".
	when := got.Writes["Network"].RequiredOnCreateWhen
	if branch, has := when["purpose=wan"]; !has || branch == nil || len(branch) != 0 {
		t.Errorf("empty branch lost: %+v", when)
	}
	if branch := when["purpose=corporate"]; len(branch) != 2 || branch[0] != "vlan" {
		t.Errorf("branch not sorted on write: %v", branch)
	}
	// EmptyWhen mirrors RequiredOnCreateWhen one level down: resource, then
	// field, then branch. A branch's verdict must survive round-tripping
	// distinctly from its siblings, and the flat Empty entry beside it must
	// not silently pick up one branch's answer.
	ipWhen := got.EmptyWhen["Nat"]["ip_address"]
	if len(ipWhen) != 3 {
		t.Fatalf("expected 3 branches for Nat.ip_address, got %v", ipWhen)
	}
	if got := ipWhen["type=MASQUERADE"]; got != (EmptySemantics{Empty: "EMPTY-REJECTED", Omit: "OMIT-OK"}) {
		t.Errorf("MASQUERADE branch lost or changed: %+v", got)
	}
	if got := ipWhen["type=SNAT"]; got != (EmptySemantics{Empty: "EMPTY-REJECTED", Omit: "OMIT-REJECTED"}) {
		t.Errorf("SNAT branch lost or changed: %+v", got)
	}
	if flat := got.Empty["Nat"]["ip_address"]; flat.Omit != "OMIT-VARIES-BY-TYPE" {
		t.Errorf("flat Nat.ip_address.omit = %q, want the branch-disagreement marker, not one branch's own answer", flat.Omit)
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
