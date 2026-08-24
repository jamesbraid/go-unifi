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
