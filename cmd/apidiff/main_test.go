package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFilterIncompatibilities(t *testing.T) {
	got := filterIncompatibilities([]string{
		"- ./cmd/fields: removed",
		"- ./unifi: HeatMap: removed",
		"- ./unifi: UnifiVersion: value changed from \"10.4.57\" to \"10.5.67\"",
		"",
		"- ./unifi/settings: Ips.Suppression: removed",
	})
	want := []string{
		"- ./unifi: HeatMap: removed",
		"- ./unifi/settings: Ips.Suppression: removed",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("filterIncompatibilities() = %q, want %q", got, want)
	}
}

func TestWireSectionRendering(t *testing.T) {
	section := wireSection("v1.103.0", []string{"P.native"}, nil)
	for _, want := range []string{"Wire surface", "v1.103.0", "+ P.native", "now always sent"} {
		if !strings.Contains(section, want) {
			t.Fatalf("wireSection() missing %q in:\n%s", want, section)
		}
	}
	if clean := wireSection("v1.103.0", nil, nil); !strings.Contains(clean, "No wire-surface changes") {
		t.Fatalf("clean wireSection() = %q", clean)
	}
}

// TestWireFloorsMeasureTheRealTree is the positive control for both floors.
//
// Nothing else here reads the repository: a floor that silently returned
// nothing would make every comparison empty, and apidiff would report "no
// wire-surface changes" forever -- indistinguishable from a clean release,
// and the exact failure the floors exist to prevent. The assertions are
// structural rather than a recorded count, so a regeneration that legitimately
// moves the floor does not need anybody to re-accept a number.
func TestWireFloorsMeasureTheRealTree(t *testing.T) {
	declared, err := declaredFloor("../..")
	if err != nil {
		t.Fatalf("declaredFloor: %v", err)
	}
	if len(declared) < 100 {
		t.Fatalf("read %d always-serialized fields; the scanner is not working", len(declared))
	}
	if !slices.Contains(declared, "WLAN.roaming_assistant_na_enabled") {
		t.Error("WLAN.roaming_assistant_na_enabled missing; keys are not Type.wire_name")
	}
	if !slices.ContainsFunc(declared, func(key string) bool {
		return strings.HasPrefix(key, "settings.")
	}) {
		t.Error("no settings. keys; the settings package is unscanned, or unqualified and colliding with unifi's")
	}
	// Hand-written types are on the wire like any other. settings.BaseSetting
	// is embedded by every settings struct and its key has no omitempty, so it
	// rides every settings write; a scan narrowed back to *.generated.go would
	// drop it and 28 others out of the gate without failing anything else.
	for _, handWritten := range []string{"settings.BaseSetting.key", "Site.name", "WireGuardPeer.public_key"} {
		if !slices.Contains(declared, handWritten) {
			t.Errorf("%s missing; the floor covers generated code only", handWritten)
		}
	}

	purposes, err := purposeFloor("../..")
	if err != nil {
		t.Fatalf("purposeFloor: %v", err)
	}
	// The DHCP-guard slots ride every corporate write: the controller rejects
	// a write that omits dhcpd_ip_1 on a guarded network.
	if !slices.Contains(purposes, "corporate dhcpd_ip_1") {
		t.Errorf("corporate dhcpd_ip_1 missing from %d purpose entries; the encoder is not being measured", len(purposes))
	}
	if !slices.ContainsFunc(purposes, func(pair string) bool {
		return strings.HasPrefix(pair, "site-vpn ")
	}) {
		t.Error("no site-vpn entries; only one purpose is being encoded")
	}
}

// TestDeclaredFloorRejectsATreeWithNoGeneratedCode pins the distinction the
// report rests on.
//
// A baseline that cannot be measured has no floor to compare against, which
// is not the same as a floor that did not move. wireSurfaceDelta skips that
// half and says why; treating it as an empty floor instead would report every
// entry as removed and bury whatever really changed.
//
// Hand-written files alone are not a floor either. Now that they are scanned,
// an ungenerated tree parses far enough to produce a short answer, and that
// answer would read as the generated surface having been deleted.
func TestDeclaredFloorRejectsATreeWithNoGeneratedCode(t *testing.T) {
	if _, err := declaredFloor(t.TempDir()); err == nil {
		t.Error("declaredFloor(empty tree) returned no error; an unmeasurable tree reads as an empty floor")
	}

	handWrittenOnly := t.TempDir()
	if err := os.MkdirAll(filepath.Join(handWrittenOnly, "unifi"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := "package unifi\n\ntype Site struct {\n\tName string `json:\"name\"`\n}\n"
	if err := os.WriteFile(filepath.Join(handWrittenOnly, "unifi", "site.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := declaredFloor(handWrittenOnly); err == nil {
		t.Error("declaredFloor(hand-written only) returned no error; an ungenerated tree reads as a floor")
	}
}

// TestWireDeltaReportsBothDirections pins what the report is built from.
func TestWireDeltaReportsBothDirections(t *testing.T) {
	added, removed := wireDelta(
		[]string{"corporate dhcpd_enabled", "site-vpn ipsec_profile"},
		[]string{"corporate dhcpd_enabled", "site-vpn remote_vpn_subnets"},
	)
	if !slices.Equal(added, []string{"site-vpn remote_vpn_subnets"}) {
		t.Errorf("added = %q", added)
	}
	if !slices.Equal(removed, []string{"site-vpn ipsec_profile"}) {
		t.Errorf("removed = %q", removed)
	}
}
