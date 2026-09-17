//go:build integration

// unifi/firewall_policy_zone_kind_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// migrateToZoneBasedFirewall switches site onto zone-based firewalling,
// which is the only thing that creates the controller's own predefined
// firewall zones (Internal, External, Gateway, Vpn, Hotspot, Dmz) -- and, as
// a consequence, the only way this SDK's probes can reach a KIND of zone
// other than the plain custom ones every other firewall-policy test creates
// for itself.
//
// No adopted gateway is required. POST /v2/api/site/{site}/firewall/migrate
// runs the ZONE_BASED_FIREWALL site-feature migration; its first step
// creates the six default zones from a hardcoded key list regardless of
// whether the site has a gateway device. (A gateway changes a DIFFERENT part
// of the same migration -- it also converts any existing firewall/traffic
// rules into ten predefined firewall policies -- which this test does not
// need and does not get by leaving the site gateway-less.) Measured on
// 10.6.101: the endpoint answers 204 with an empty body, which Session.do
// reports as ErrNotJSON, so success is read from the status and confirmed by
// rereading the zone collection, never from the POST's own response -- the
// same discipline every other probe in this package applies to a write.
//
// This route was found in a never-merged branch of this repository
// (git log 94e4a31/ea3be75/cd27e10) that had already discovered and then
// corrected the same mistake this task started from: the firewall/zone 404
// on an unmigrated site (api.err.CouldNotFindHotspotFirewallZone) reads like
// "needs a gateway" but is actually "needs this migration", and needs no
// hardware at all.
//
// A skip here would read exactly like a pass on an axis nothing then
// measured, so a site that comes back without all six zone_keys is a hard
// failure, not a skip -- consistent with this artifact's rule that
// zone-kind coverage must say UNSWEPT out loud rather than look covered.
func migrateToZoneBasedFirewall(ctx context.Context, t *testing.T, s *controllertest.Session, site string) map[string]string {
	t.Helper()

	_, status, err := s.PostJSON(ctx, "/v2/api/site/"+site+"/firewall/migrate", nil)
	if status == 0 {
		t.Fatalf("zone-based firewall migration: transport error: %v", err)
	}
	if status >= 300 {
		t.Fatalf("zone-based firewall migration: POST returned HTTP %d (want 204)", status)
	}

	zones, status, err := s.GetJSON(ctx, "/v2/api/site/"+site+"/firewall/zone")
	if err != nil || status != 200 {
		t.Fatalf("zone-based firewall migration: list zones: HTTP %d: %v", status, err)
	}
	byKey := map[string]string{}
	for _, item := range asSlice(zones) {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		key, _ := m["zone_key"].(string)
		id, _ := m["_id"].(string)
		if key != "" && id != "" {
			byKey[key] = id
		}
	}
	want := []string{"internal", "external", "gateway", "vpn", "hotspot", "dmz"}
	var missing []string
	for _, k := range want {
		if _, ok := byKey[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("zone-based firewall migration: POST answered HTTP %d but the site has no %v zone(s); "+
			"the zone-kind axis is UNSWEPT this run, not covered -- either the migration no longer creates "+
			"the defaults, or this controller wired a no-op migration service that still answers 204 and "+
			"does nothing. Collection: %v", status, missing, zones)
	}
	return byKey
}

// TestIntegrationFirewallPolicyZoneKindSpotCheck measures GAP 2's other
// half: does the KIND of zone a policy addresses -- a controller-owned
// predefined zone, rather than the plain custom ones every other probe in
// this package creates for itself -- change which (protocol, ip_version)
// pairs TestIntegrationFirewallPolicyCapabilityMatrix's flat matrix says are
// legal?
//
// What this spot check found, and what it did not:
//
//   - Internal -> External (two predefined zones spanning the LAN/WAN
//     boundary -- the pairing most likely to differ from a custom zone,
//     since External and Gateway are the two zones the controller marks
//     attr_no_edit) with create_allow_respond=false and matching_target=ANY
//     on both sides, across the boundary protocol set and all 3
//     ip_versions: every verdict agreed with the flat matrix's pinned value
//     for the same (protocol, ip_version) key.
//   - The identical sweep with source matching_target=NETWORK (a real LAN
//     network) and destination matching_target=WEB (a domain literal) --
//     both of which a CUSTOM zone refuses outright regardless of protocol,
//     see TestIntegrationFirewallPolicyCapabilityMatrix's spot check -- also
//     agreed with the pinned baseline once addressed at zones whose kind
//     accepts those targets. So the custom-zone refusal was purely about
//     the target's applicability to THAT zone kind, not evidence that
//     protocol legality itself depends on zone kind.
//   - A DISTINCT, real zone-kind dependency was found and is deliberately
//     NOT folded into this matrix, because it has nothing to do with
//     protocol or ip_version: create_allow_respond=true (every other probe
//     in this file leaves it at firewallPolicyProbeBase's default, true) is
//     refused with api.err.FirewallPolicyCreateRespondTrafficPolicyNotAllowed
//     whenever the DESTINATION is External or Gateway, for every protocol
//     tried -- while the identical create against Internal, Hotspot, Dmz or
//     a plain custom zone succeeds. That is a create_allow_respond x
//     destination-zone-kind interaction. It is confirmed below (logged, not
//     asserted -- it is not what this test measures) and is exactly why
//     every measurement in this file fixes create_allow_respond=false: left
//     at the default it would mask the (protocol, ip_version) signal this
//     test exists to isolate behind a rejection that has nothing to do with
//     either.
//   - matching_target=REGION and APP stay UNSWEPT for protocol legality
//     even against a predefined zone: the controller's refusal moves past
//     the zone-applicability gate ("Empty firewall destination regions",
//     "Invalid firewall destination APP IDs"), but neither can be satisfied
//     without a field FirewallPolicySource/Destination does not model (a
//     region list) or a real app-catalog id this harness has no source of
//     truth for. Guessing a shape for either would be exactly the
//     hand-coded-field problem this project refuses elsewhere -- see
//     TestIntegrationFirewallPolicyCapabilityMatrix's spot check doc
//     comment for the rest of the matching_target findings.
//
// This test does not write to the artifact: it is a cross-check against
// TestIntegrationFirewallPolicyCapabilityMatrix's own Capabilities entries,
// and a second writer of the same keys would erase whichever ran second.
func TestIntegrationFirewallPolicyZoneKindSpotCheck(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 20*time.Minute)
	root, _, running := behaviorGate(ctx, t, s, c.Site)

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok || len(art.Capabilities["FirewallPolicy"]) == 0 {
		t.Skip("no pinned FirewallPolicy capabilities to cross-check against; run " +
			"TestIntegrationFirewallPolicyCapabilityMatrix with BEHAVIOR_WRITE=1 first")
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	pinned := art.Capabilities["FirewallPolicy"]

	byKey := migrateToZoneBasedFirewall(ctx, t, s, c.Site)
	t.Logf("zone-based firewall migration created zone kinds: %v", slices.Sorted(maps.Keys(byKey)))

	lan := defaultLANNetworkID(ctx, t, s, c.Site)
	if lan == "" {
		t.Fatal("no LAN network; matching_target=NETWORK cannot be exercised")
	}

	internal, ok1 := byKey["internal"]
	external, ok2 := byKey["external"]
	if !ok1 || !ok2 {
		t.Fatal("migration did not yield both internal and external zone ids")
	}

	path := "/v2/api/site/" + c.Site + "/firewall-policies"
	n := 0
	measureOne := func(protocol, ipVersion string, source, destination map[string]any) behavior.Capability {
		n++
		name := fmt.Sprintf("zone-kind-probe-%d", n)
		doc := firewallPolicyProbeBase(name, 29000+n, internal, external)
		doc["protocol"] = protocol
		doc["ip_version"] = ipVersion
		doc["create_allow_respond"] = false // isolate zone kind's OWN create_allow_respond gate; see doc comment
		doc["source"] = source
		doc["destination"] = destination
		body, status, err := s.PostJSON(ctx, path, doc)
		mustTransport(t, err)
		m, _ := body.(map[string]any)
		if status != 200 && status != 201 {
			return behavior.Capability{Accepted: false, Error: v2ErrMessage(m)}
		}
		stored := findFirewallPolicyByName(ctx, t, s, path, name)
		id := objectID(m)
		if stored != nil {
			id = objectID(stored)
		}
		if id != "" {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		if stored == nil {
			t.Errorf("protocol=%s ip_version=%s: create answered HTTP %d but a reread found no document "+
				"named %q", protocol, ipVersion, status, name)
			return behavior.Capability{Accepted: false, Error: "accepted but absent on reread"}
		}
		return behavior.Capability{Accepted: true}
	}

	compareAgainstPinned := func(label, protocol, ipVersion string, got behavior.Capability) {
		key := firewallCapabilityKey(protocol, ipVersion)
		want, ok := pinned[key]
		if !ok {
			t.Logf("%s %s: no pinned verdict to compare against", label, key)
			return
		}
		if want.Accepted != got.Accepted {
			t.Errorf("%s %s: zone kind changes acceptance -- flat matrix pins accepted=%v (%q), this zone "+
				"kind measured accepted=%v (%q). Zone kind IS a real third axis; the flat matrix is "+
				"branch-blind and must be re-published keyed by zone kind.",
				label, key, want.Accepted, want.Error, got.Accepted, got.Error)
		}
	}

	boundary := []string{"all", "tcp", "icmp", "icmpv6", "ipv6", "ah"}
	versions := slices.Clone(FirewallPolicyVersionValues)

	agreed := 0
	for _, ipVersion := range versions {
		for _, protocol := range boundary {
			got := measureOne(protocol, ipVersion,
				map[string]any{"zone_id": internal, "matching_target": "ANY"},
				map[string]any{"zone_id": external, "matching_target": "ANY"})
			compareAgainstPinned("predefined internal->external", protocol, ipVersion, got)
			agreed++
		}
	}
	for _, ipVersion := range versions {
		for _, protocol := range boundary {
			got := measureOne(protocol, ipVersion,
				map[string]any{
					"zone_id": internal, "matching_target": "NETWORK",
					"matching_target_type": "SPECIFIC", "network_ids": []string{lan},
				},
				map[string]any{
					"zone_id": external, "matching_target": "WEB",
					"matching_target_type": "SPECIFIC", "web_domains": []string{"example.com"},
				})
			compareAgainstPinned("predefined NETWORK/WEB internal->external", protocol, ipVersion, got)
			agreed++
		}
	}
	t.Logf("zone-kind spot check: %d (protocol, ip_version) measurements across predefined zones and two "+
		"matching_target shapes, compared against the pinned flat matrix", agreed)

	// The distinct create_allow_respond x destination-zone-kind interaction
	// this test's doc comment describes -- confirmed here, logged, and
	// deliberately not asserted: it is not what this test measures.
	respondDoc := firewallPolicyProbeBase("zone-kind-respond-probe", 29900, internal, external)
	respondDoc["protocol"] = "tcp"
	respondDoc["create_allow_respond"] = true
	body, status, err := s.PostJSON(ctx, path, respondDoc)
	mustTransport(t, err)
	m, _ := body.(map[string]any)
	if id := objectID(m); id != "" {
		s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	}
	if status/100 == 2 {
		t.Logf("LOUD: create_allow_respond=true against a predefined External destination now creates "+
			"(HTTP %d); the create_allow_respond x zone-kind interaction this test's doc comment describes "+
			"may have changed", status)
	} else {
		t.Logf("confirmed: create_allow_respond=true against a predefined External destination is refused "+
			"(HTTP %d, %s) regardless of protocol -- a zone-kind gate on THAT field, not on protocol; "+
			"see this test's doc comment", status, v2ErrMessage(m))
	}
}
