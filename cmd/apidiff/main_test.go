package main

import (
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
	generated, err := generatedFloor("../..")
	if err != nil {
		t.Fatalf("generatedFloor: %v", err)
	}
	if len(generated) < 100 {
		t.Fatalf("read %d always-serialized fields; the scanner is not working", len(generated))
	}
	if !slices.Contains(generated, "WLAN.roaming_assistant_na_enabled") {
		t.Error("WLAN.roaming_assistant_na_enabled missing; keys are not Type.wire_name")
	}
	if !slices.ContainsFunc(generated, func(key string) bool {
		return strings.HasPrefix(key, "settings.")
	}) {
		t.Error("no settings. keys; the settings package is unscanned, or unqualified and colliding with unifi's")
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

// TestGeneratedFloorRejectsATreeWithNoGeneratedCode pins the distinction the
// report rests on.
//
// A baseline that cannot be measured has no floor to compare against, which
// is not the same as a floor that did not move. wireSurfaceDelta skips that
// half and says why; treating it as an empty floor instead would report every
// entry as removed and bury whatever really changed.
func TestGeneratedFloorRejectsATreeWithNoGeneratedCode(t *testing.T) {
	if _, err := generatedFloor(t.TempDir()); err == nil {
		t.Error("generatedFloor(empty tree) returned no error; an unmeasurable tree reads as an empty floor")
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
