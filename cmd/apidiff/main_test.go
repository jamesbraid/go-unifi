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

func TestWireSurfaceDelta(t *testing.T) {
	added, removed := wireDelta(
		[]string{"A.x", "B.y", "C.z"},
		[]string{"A.x", "C.z", "D.w"},
	)
	if !slices.Equal(added, []string{"D.w"}) || !slices.Equal(removed, []string{"B.y"}) {
		t.Fatalf("wireDelta() = +%q -%q, want +[D.w] -[B.y]", added, removed)
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

// TestBaselineLinesTellsMissingFromEmpty covers the distinction the
// wire-surface report rests on.
//
// A release that predates a baseline has no record of what its wire surface
// was. Treating that as an empty record would report the whole of the new
// baseline as additions the first time one is introduced, burying whatever
// really moved. Treating it as "not recorded" says nothing, which is the
// truth.
func TestBaselineLinesTellsMissingFromEmpty(t *testing.T) {
	dir := t.TempDir()

	if _, recorded, err := baselineLines(filepath.Join(dir, "absent.txt")); err != nil || recorded {
		t.Errorf("a missing baseline reported recorded=%v err=%v, want false and no error", recorded, err)
	}

	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, []byte("\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, recorded, err := baselineLines(empty)
	if err != nil {
		t.Fatalf("baselineLines: %v", err)
	}
	if !recorded {
		t.Error("a file that exists but holds no entries reported recorded=false")
	}
	if len(lines) != 0 {
		t.Errorf("lines = %q, want none", lines)
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
