package main

import (
	"slices"
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
