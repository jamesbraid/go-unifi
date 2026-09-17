//go:build integration

// unifi/firewall_policy_capability_matrix_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// firewallPolicyProtocolDomain returns the protocol names FirewallPolicy's
// own validation pattern declares legal, walking the pattern's alternation
// the same way TestProtocolNamesAreMatchedLiterally
// (validation_regexp_test.go) does: split on "|", skip the one branch that is
// a regex rather than a name (the numeric protocol-number range), and unescape
// the single literal that needs it ("ax\\.25").
//
// protocol is not enum-typed on the wire -- FieldValidationPatterns, not a
// generated Values slice, is where its legal names live -- so this, not a
// list this project maintains, is the domain the matrix below sweeps. A
// controller that adds a name to the pattern grows this domain with it.
// Numeric protocol numbers (the "6" in "protocol=6, ip_version=IPV4" the enum
// test already tries) are not enumerated here: the regex names a 256-value
// range, not a discrete capability, and every number the pattern would let
// through aliases one of these names or is untested territory this matrix
// does not claim to cover.
func firewallPolicyProtocolDomain(t *testing.T) []string {
	t.Helper()
	pattern, ok := FieldValidationPatterns["FirewallPolicy"]["protocol"]
	if !ok {
		t.Fatal("FirewallPolicy.protocol has no published validation pattern; the protocol domain cannot be derived")
	}
	var names []string
	for _, tok := range strings.Split(pattern, "|") {
		if strings.ContainsAny(tok, "()[]{}*+?^$") {
			continue // the numeric protocol-number range, not a name
		}
		names = append(names, strings.ReplaceAll(tok, `\.`, "."))
	}
	if len(names) == 0 {
		t.Fatal("no literal protocol names found in FirewallPolicy.protocol's pattern")
	}
	return names
}

// firewallCapabilityKey spells a (protocol, ip_version) branch the same way
// WriteContract.RequiredOnCreateWhen spells a branch key: comma-separated
// wire field and value, sorted by field name ("ip_version" before
// "protocol").
func firewallCapabilityKey(protocol, ipVersion string) string {
	return fmt.Sprintf("ip_version=%s,protocol=%s", ipVersion, protocol)
}

// v2ErrMessage pulls the human-readable message out of a v2 error envelope
// ({"code","message"}), falling back to a rendering of the whole body when
// there is none -- the same field
// TestIntegrationFirewallPolicyEnumsMatchTheController reads to recover the
// controller's own wording.
func v2ErrMessage(body map[string]any) string {
	if msg, _ := body["message"].(string); msg != "" {
		return msg
	}
	return fmt.Sprintf("%v", body)
}

// findFirewallPolicyByName re-reads the whole collection and returns the
// stored document named name, or nil. The v2 firewall-policies endpoint has
// no single-object GET -- the SDK's own GetFirewallPolicy lists and filters
// client-side -- so this is the same shape a caller has to use.
func findFirewallPolicyByName(ctx context.Context, t *testing.T, s *controllertest.Session, path, name string) map[string]any {
	t.Helper()
	body, status, err := s.GetJSON(ctx, path)
	if err != nil || status != 200 {
		t.Fatalf("list %s (HTTP %d): %v -- a create's acceptance cannot be confirmed without rereading the collection", path, status, err)
	}
	for _, item := range asSlice(body) {
		m, _ := item.(map[string]any)
		if got, _ := m["name"].(string); got == name {
			return m
		}
	}
	return nil
}

// TestIntegrationFirewallPolicyCapabilityMatrix measures, for every (protocol,
// ip_version) pair the controller's own field declarations admit, whether a
// firewall policy create using that pair is accepted -- and if refused, the
// controller's verbatim message.
//
// This replaces terraform-provider-unifi's firewall_policy_resource.go, which
// carries roughly sixty hand-written protocol/ip_version literals labelled
// "measured against 10.6.101" in a comment: a measurement with no instrument,
// invisible to a controller bump. This is that measurement, checkable and
// re-run by this probe.
//
// Every verdict here comes from a reread of the collection, never from the
// create's own response: a v2 create that answers 200 still has to be
// confirmed present and holding the asked protocol and ip_version, because a
// response is not evidence of what got stored (see nat_update_empty_reread's
// doc comment for the general form of this trap).
//
// Two candidate third axes were spot-checked rather than fully swept -- see
// the doc comment on the spot-check below for what was tried and why the
// full cross product was not measured. Do not delete a hand-written literal
// this matrix disagrees with until that spot-check either clears or the axis
// is measured in full.
func TestIntegrationFirewallPolicyCapabilityMatrix(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 45*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/v2/api/site/" + c.Site + "/firewall-policies"
	src, dst := firewallZonePair(ctx, t, s, c.Site)
	if src == "" {
		t.Fatal("no firewall zones; the matrix cannot be measured without an address for source and destination")
	}

	protocols := firewallPolicyProtocolDomain(t)
	versions := slices.Clone(FirewallPolicyVersionValues) // BOTH, IPV4, IPV6 -- the controller's own enum

	n := 0
	create := func(protocol, ipVersion string, overrides map[string]any) (int, string, map[string]any) {
		n++
		name := fmt.Sprintf("matrix-probe-%d", n)
		doc := firewallPolicyProbeBase(name, 24000+n, src, dst)
		doc["protocol"] = protocol
		doc["ip_version"] = ipVersion
		for k, v := range overrides {
			doc[k] = v
		}
		body, status, err := s.PostJSON(ctx, path, doc)
		mustTransport(t, err)
		m, _ := body.(map[string]any)
		return status, name, m
	}

	// measureOne creates one (protocol, ip_version) policy, rereads the
	// collection to confirm what actually landed, deletes it, and returns the
	// verdict. Shared by the baseline sweep and the spot-check below so both
	// apply the same reread discipline.
	measureOne := func(protocol, ipVersion string, overrides map[string]any) behavior.Capability {
		status, name, body := create(protocol, ipVersion, overrides)
		if status != 200 && status != 201 {
			return behavior.Capability{Accepted: false, Error: v2ErrMessage(body)}
		}
		stored := findFirewallPolicyByName(ctx, t, s, path, name)
		id := objectID(body)
		if stored != nil {
			id = objectID(stored)
		}
		if id != "" {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		if stored == nil {
			t.Errorf("protocol=%s ip_version=%s: create answered HTTP %d but a reread of %s found no "+
				"document named %q; the create's own response is not evidence of what was stored",
				protocol, ipVersion, status, path, name)
			return behavior.Capability{Accepted: false, Error: "accepted but absent on reread"}
		}
		if gotP, _ := stored["protocol"].(string); gotP != protocol {
			t.Errorf("protocol=%s ip_version=%s: accepted, but the reread shows protocol stored as %q",
				protocol, ipVersion, gotP)
		}
		if gotV, _ := stored["ip_version"].(string); gotV != ipVersion {
			t.Errorf("protocol=%s ip_version=%s: accepted, but the reread shows ip_version stored as %q",
				protocol, ipVersion, gotV)
		}
		return behavior.Capability{Accepted: true}
	}

	measured := map[string]behavior.Capability{}
	for _, ipVersion := range versions {
		for _, protocol := range protocols {
			measured[firewallCapabilityKey(protocol, ipVersion)] = measureOne(protocol, ipVersion, nil)
		}
	}

	// Third-axis spot check: does the matching_target on source/destination,
	// or the zone kind they name, change which (protocol, ip_version) pairs
	// are legal? A discriminator here would make the matrix above true only
	// for ANY-matched, custom-zone policies, and publishing it as universal
	// would be the same branch-blindness the artifact's own EmptyWhen section
	// exists to catch elsewhere.
	//
	// What was tried: the same boundary pairs re-created with source AND
	// destination set to matching_target IP (an address list, not a zone
	// reference) instead of ANY -- covering the one refusal the baseline
	// sweep is expected to produce (icmp on a non-IPV4 version) plus the
	// protocols that name an IP version directly (ipv6, icmpv6) and the two
	// controls (all, tcp). Both v4 and v6 literal addresses were used so an
	// IPV4 or IPV6 policy's address matches its own family.
	//
	// Zone kind was checked differently: a fresh site on this harness ships
	// with zero firewall zones (confirmed directly -- GET .../firewall/zone
	// on an unmigrated site returns an empty array), so "Internal"/
	// "External"/"Gateway" system zones to compare against a custom zone
	// simply do not exist here. Whether a zone the controller creates for
	// itself once a gateway is adopted behaves differently is UNSWEPT: this
	// probe adopts no device and cannot reach that state.
	//
	// What was NOT tried: DOMAIN/REGION/APP matching targets, and every
	// protocol outside this boundary subset against every matching target.
	// If the controller ties legality to a matching target this spot check
	// did not exercise, that dimension is unswept and the baseline matrix
	// above must not be read as covering it.
	spotCheckProtocols := []string{"all", "tcp", "icmp", "icmpv6", "ipv6", "ah"}
	spotCheck := map[string]behavior.Capability{}
	for _, ipVersion := range versions {
		// An IP-matched policy's address list has to suit the family it
		// polices: an IPV4 policy takes the v4 literal, an IPV6 policy the v6
		// one, and BOTH both -- mixing families into a version-specific
		// policy would draw a family-mismatch refusal that has nothing to do
		// with the protocol/ip_version pairing this spot check is after.
		var ips []string
		switch ipVersion {
		case "IPV4":
			ips = []string{"192.0.2.10/32"}
		case "IPV6":
			ips = []string{"2001:db8::10/128"}
		default: // BOTH
			ips = []string{"192.0.2.10/32", "2001:db8::10/128"}
		}
		for _, protocol := range spotCheckProtocols {
			overrides := map[string]any{
				"source": map[string]any{
					"zone_id": src, "matching_target": "IP", "matching_target_type": "SPECIFIC", "ips": ips,
				},
				"destination": map[string]any{
					"zone_id": dst, "matching_target": "IP", "matching_target_type": "SPECIFIC", "ips": ips,
				},
			}
			spotCheck[firewallCapabilityKey(protocol, ipVersion)] = measureOne(protocol, ipVersion, overrides)
		}
	}
	var diverged []string
	for key, want := range spotCheck {
		if got := measured[key]; got.Accepted != want.Accepted {
			diverged = append(diverged, fmt.Sprintf("%s: ANY-zone accepted=%v, IP-matched accepted=%v (%q)",
				key, got.Accepted, want.Accepted, want.Error))
		}
	}
	if len(diverged) > 0 {
		slices.Sort(diverged)
		t.Errorf("matching_target changes acceptance for %d of %d spot-checked pairs -- matching_target "+
			"is a real third axis and the flat matrix above is branch-blind:\n  %s",
			len(diverged), len(spotCheck), strings.Join(diverged, "\n  "))
	} else {
		t.Logf("spot-checked %d (protocol, ip_version) pairs under matching_target=IP: all agreed with "+
			"the ANY-zone baseline. matching_target is not ruled out as a third axis beyond this subset "+
			"-- see the test's doc comment for what was and was not tried.", len(spotCheck))
	}

	var summary []string
	for _, key := range slices.Sorted(maps.Keys(measured)) {
		v := measured[key]
		if v.Accepted {
			summary = append(summary, fmt.Sprintf("%-28s ACCEPTED", key))
		} else {
			summary = append(summary, fmt.Sprintf("%-28s REFUSED (%s)", key, v.Error))
		}
	}
	t.Logf("firewall policy (protocol, ip_version) capability matrix, %d pairs:\n  %s",
		len(measured), strings.Join(summary, "\n  "))

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Capabilities == nil {
				a.Capabilities = map[string]map[string]behavior.Capability{}
			}
			// This probe is FirewallPolicy's only writer into Capabilities, so
			// it replaces the whole per-resource map: a pair the controller
			// stops rejecting (or starts rejecting) must disappear from -- or
			// appear in -- the artifact, not linger from a stale prior run.
			a.Capabilities["FirewallPolicy"] = measured
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok || len(art.Capabilities["FirewallPolicy"]) == 0 {
		t.Logf("no pinned FirewallPolicy capabilities in %s; run with BEHAVIOR_WRITE=1 to record them", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	pinned := art.Capabilities["FirewallPolicy"]
	for _, key := range slices.Sorted(maps.Keys(measured)) {
		got := measured[key]
		want, ok := pinned[key]
		if !ok {
			t.Errorf("%s: measured (accepted=%v) but the artifact pins nothing for this pair; "+
				"re-measure with BEHAVIOR_WRITE=1", key, got.Accepted)
			continue
		}
		if want != got {
			t.Errorf("%s: artifact pins accepted=%v (%q), measured accepted=%v (%q); the controller "+
				"changed or the artifact is stale -- re-measure with BEHAVIOR_WRITE=1",
				key, want.Accepted, want.Error, got.Accepted, got.Error)
		}
	}
	for key := range pinned {
		if _, ok := measured[key]; !ok {
			t.Errorf("%s: the artifact pins this pair but the current sweep no longer produces it -- "+
				"the domain shrank or the key spelling changed; re-measure with BEHAVIOR_WRITE=1", key)
		}
	}
}
