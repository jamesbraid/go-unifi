//go:build integration

// unifi/network_guest_parity_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/internal/probe"
)

// guestParityCandidates are the corporate-family fields that came back
// PERSISTED on purpose=corporate in phase 2 and that plausibly apply to a
// guest network too (guest shares the networkconf collection and is
// LAN-like). This probe re-tests them on purpose=guest to decide which
// marshalGuest should emit. WAN/site-vpn/user-vpn fields are excluded --
// they don't apply to a guest LAN. ipv6 single-network and the id-shaped
// references (single_network_lan) are left out of this first pass.
var guestParityCandidates = []fieldCandidate{
	{Wire: "dhcpd_time_offset", Purpose: PurposeGuest, Value: 3600, Prereq: map[string]any{"dhcpd_time_offset_enabled": true}},
	{Wire: "mac_override_enabled", Purpose: PurposeGuest, Value: true, Prereq: map[string]any{"mac_override": "02:00:00:00:00:01"}},
	{Wire: "firewall_zone_id", Purpose: PurposeGuest, Value: "@zone", Prereq: nil},
	{Wire: "igmp_fastleave", Purpose: PurposeGuest, Value: true, Prereq: nil},
	{Wire: "igmp_flood_unknown_multicast", Purpose: PurposeGuest, Value: true, Prereq: nil},
	{Wire: "igmp_groupmembership", Purpose: PurposeGuest, Value: 260, Prereq: nil},
	{Wire: "igmp_maxresponse", Purpose: PurposeGuest, Value: 10, Prereq: nil},
	{Wire: "igmp_mcrtrexpiretime", Purpose: PurposeGuest, Value: 300, Prereq: nil},
	{Wire: "igmp_querier_switches", Purpose: PurposeGuest, Value: []NetworkIGMPQuerierSwitches{{QuerierAddress: "10.0.0.254"}}, Prereq: nil},
	{Wire: "igmp_supression", Purpose: PurposeGuest, Value: true, Prereq: nil},
	{Wire: "upnp_lan_enabled", Purpose: PurposeGuest, Value: true, Prereq: nil},
}

// TestIntegrationGuestParityProbe classifies each guestParityCandidate on a
// throwaway guest network the same way TestIntegrationNetworkFieldProbe does
// for corporate, and prints a summary so the wiring decision for marshalGuest
// rests on observed behavior.
func TestIntegrationGuestParityProbe(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	zone := firstZoneID(ctx, t, s, c.Site)
	resolve := func(v any) any {
		if str, ok := v.(string); ok && str == "@zone" {
			return zone
		}
		return v
	}

	results := map[string]string{}
	for i, cand := range guestParityCandidates {
		payload := map[string]any{
			"name":         fmt.Sprintf("guestprobe-%d", i),
			"purpose":      PurposeGuest,
			"enabled":      true,
			"ip_subnet":    fmt.Sprintf("10.98.%d.1/24", i%250),
			"vlan_enabled": true,
			"vlan":         200 + i,
		}
		for k, v := range cand.Prereq {
			payload[k] = resolve(v)
		}
		value := resolve(cand.Value)
		payload[cand.Wire] = value

		body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", payload)
		if err != nil {
			t.Fatalf("transport: %v", err)
		}
		if status != 200 {
			results[cand.Wire] = fmt.Sprintf("REJECTED (HTTP %d): %v", status, body)
			continue
		}

		created := firstData(t, body)
		id, _ := created["_id"].(string)
		if id == "" {
			t.Errorf("create for %s answered HTTP 200 with no _id, so the stored document cannot be "+
				"read back and no verdict was measured", cand.Wire)
			continue
		}
		defer deleteNetwork(ctx, t, s, c.Site, id)

		// Classify from the STORED document, never from the create response
		// (network_field_probe_integration_test.go): the controller echoes
		// back the document it was handed, so an accepted-then-dropped field
		// reads as PERSISTED until the re-read.
		stored := fetchNetwork(ctx, t, s, c.Site, id)
		if stored == nil {
			t.Errorf("%s: created %s but could not read it back; no verdict measured", cand.Wire, id)
			continue
		}
		switch moved := probe.Classify(map[string]any{cand.Wire: value}, stored); {
		case len(moved) == 0:
			results[cand.Wire] = "PERSISTED"
		case moved[0].Verdict == probe.Dropped:
			results[cand.Wire] = "STRIPPED"
		default:
			results[cand.Wire] = "MUTATED " + moved[0].Detail
		}
	}

	keys := make([]string, 0, len(results))
	for k := range results {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t.Log("=== GUEST PARITY RESULTS ===")
	for _, k := range keys {
		t.Logf("%-42s %s", k, results[k])
		// marshalGuest emits every one of these, so a non-PERSISTED result
		// means the encoder now sends a field this controller version
		// rejects or drops on a guest network -- a regression to catch.
		if results[k] != "PERSISTED" {
			t.Errorf("%s: want PERSISTED, got %s", k, results[k])
		}
	}
}
