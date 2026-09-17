//go:build integration

// unifi/firewall_policy_capability_matrix_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
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
//
// The pattern's other branch -- a bare protocol NUMBER, 0-255 -- is not a
// name and is not returned here; firewallPolicyNumericProtocolDomain sweeps
// that branch instead, into the same matrix under a differently-named key.
// The two are NOT aliases of each other: this session measured the
// controller accepting protocol=58 under IPV4 while refusing protocol=icmpv6
// (58's own name) under the same version, so a name-only matrix is
// incomplete in a way that matters, not just in a way that is theoretically
// possible.
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

// firewallCapabilityNumericKey spells a (protocol NUMBER, ip_version) branch
// the way firewallCapabilityKey spells a (protocol NAME, ip_version) one, but
// under a different field name -- protocol_number, not protocol -- so a
// consumer reading the key can tell a numeric row from a named one without
// guessing from the value's shape (no name in firewallPolicyProtocolDomain's
// domain is all digits, but a key format should not rely on that holding
// forever). protocol_number names what KIND of row this is; the wire field
// the create actually sent is still plain "protocol", carrying the decimal
// string -- confirmed round-tripping verbatim as a JSON string for both a
// name and a bare number, never coerced to a JSON number
// ([[go-unifi-number-or-word-decode]]).
func firewallCapabilityNumericKey(protocolNumber int, ipVersion string) string {
	return fmt.Sprintf("ip_version=%s,protocol_number=%d", ipVersion, protocolNumber)
}

// firewallCapabilityKeyIdentity splits a key this file spells
// ("ip_version=X,protocol=Y" or "ip_version=X,protocol_number=N") into the
// ip_version and everything else -- the part naming WHICH protocol, by name
// or by number, the row measures. Used to group a name row and its
// corresponding number row under the same identity for a cross-check, and to
// check the BOTH-is-intersection invariant per protocol regardless of which
// key shape it was measured under.
func firewallCapabilityKeyIdentity(key string) (ipVersion, identity string) {
	ipVersion, identity, ok := strings.Cut(strings.TrimPrefix(key, "ip_version="), ",")
	if !ok {
		return "", key
	}
	return ipVersion, identity
}

// firewallPolicyProtocolNumberVocabulary maps a protocol NUMBER (in the
// pattern's 0-255 numeric branch, which firewallPolicyProtocolDomain's walk
// skips) to the NAME the standard IANA "Assigned Internet Protocol Numbers"
// registry -- the table most Unix systems ship as /etc/protocols, and the
// evident source of firewallPolicyProtocolDomain's own vocabulary, judging by
// names like "ip", "ipencap" and "ipip" existing as three separate entries
// exactly the way that file lists them -- assigns the same value. These are
// the numbers where "does the number agree with its own name" is even
// askable, which is how this session found the counter-example that started
// this sweep: the controller accepts protocol=58 under IPV4 while it refuses
// protocol=icmpv6 (58's registry name) under IPV4.
//
// This map is NOT exhaustive over firewallPolicyProtocolDomain's names. Every
// entry here was checked against the registry by this session; a domain name
// with no entry means its number could not be confirmed with the confidence
// this artifact requires, not that no number exists. "rspf" is the one
// case, found in the pattern but not confirmed against the registry.
// "tcp_udp" and "all" are controller synonyms with no single registry number
// at all, so neither has one either. Do not fill in a guess -- measure it and
// add the entry once confirmed, the same rule this codebase applies
// everywhere a fact must come from the controller rather than from memory.
var firewallPolicyProtocolNumberVocabulary = map[int]string{
	0: "ip", 1: "icmp", 2: "igmp", 3: "ggp", 4: "ipencap", 5: "st", 6: "tcp",
	8: "egp", 9: "igp", 12: "pup", 17: "udp", 20: "hmp", 22: "xns-idp",
	27: "rdp", 29: "iso-tp4", 33: "dccp", 36: "xtp", 37: "ddp", 38: "idpr-cmtp",
	41: "ipv6", 43: "ipv6-route", 44: "ipv6-frag", 45: "idrp", 46: "rsvp",
	47: "gre", 50: "esp", 51: "ah", 57: "skip", 58: "icmpv6", 59: "ipv6-nonxt",
	60: "ipv6-opts", 81: "vmtp", 88: "eigrp", 89: "ospf", 93: "ax.25",
	94: "ipip", 97: "etherip", 98: "encap", 103: "pim", 108: "ipcomp",
	112: "vrrp", 115: "l2tp", 124: "isis", 132: "sctp", 133: "fc",
	135: "mobility-header", 136: "udplite", 137: "mpls-in-ip", 138: "manet",
	139: "hip", 140: "shim6", 141: "wesp", 142: "rohc",
}

// firewallPolicyNumericSampleStep is the spacing of the systematic sample
// firewallPolicyNumericProtocolDomain takes across the numeric range beyond
// the named vocabulary.
const firewallPolicyNumericSampleStep = 5

// firewallPolicyNumericProtocolDomain returns the numeric protocol values
// this sweep measures: every number firewallPolicyProtocolNumberVocabulary
// names (so a disagreement between a number and its own name, like the
// icmpv6/58 one this sweep exists to close, would surface), plus a
// systematic sample of the rest of the pattern's 0-255 numeric branch --
// every multiple of firewallPolicyNumericSampleStep not already covered by a
// name. That sample necessarily includes both boundaries, 0 and 255.
//
// This is a declared PARTIAL sweep of the numeric branch, not the full
// 256-value range crossed with 3 ip_versions (768 creates) the task that
// added it judged impractical to run on every measurement. It instead
// prioritises the numbers most likely to disagree with something (the named
// ones) and takes an even-spaced look at the numbers with no name attached
// at all, rather than either skipping the numeric branch entirely (the prior
// state) or attempting every value. A number this sweep does not visit is
// UNSWEPT, not "assumed to behave like its neighbours" -- said here rather
// than left for a reader to infer from the artifact's row count.
func firewallPolicyNumericProtocolDomain(t *testing.T) []int {
	t.Helper()
	seen := map[int]bool{}
	var out []int
	add := func(n int) {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for n := range firewallPolicyProtocolNumberVocabulary {
		add(n)
	}
	for n := 0; n <= 255; n += firewallPolicyNumericSampleStep {
		add(n)
	}
	slices.Sort(out)
	return out
}

// assertBothIsIntersectionHolds checks a fact TestIntegrationFirewallPolicyCapabilityMatrix
// already established before this sweep grew it: BOTH accepts a protocol --
// named or numeric -- exactly when IPV4 and IPV6 both accept it, never more
// and never less. A protocol measured under fewer than all three versions
// cannot be checked and is logged, not failed -- that only happens if a
// future change stops sweeping every version uniformly.
func assertBothIsIntersectionHolds(t *testing.T, measured map[string]behavior.Capability) {
	t.Helper()
	byIdentity := map[string]map[string]behavior.Capability{}
	for key, verdict := range measured {
		ipVersion, identity := firewallCapabilityKeyIdentity(key)
		if byIdentity[identity] == nil {
			byIdentity[identity] = map[string]behavior.Capability{}
		}
		byIdentity[identity][ipVersion] = verdict
	}
	checked := 0
	for _, identity := range slices.Sorted(maps.Keys(byIdentity)) {
		byVersion := byIdentity[identity]
		both, hasBoth := byVersion["BOTH"]
		v4, hasV4 := byVersion["IPV4"]
		v6, hasV6 := byVersion["IPV6"]
		if !hasBoth || !hasV4 || !hasV6 {
			t.Logf("%s: measured under %d of 3 ip_versions; BOTH-is-intersection cannot be checked",
				identity, len(byVersion))
			continue
		}
		checked++
		want := v4.Accepted && v6.Accepted
		if both.Accepted != want {
			t.Errorf("%s: BOTH accepted=%v, but IPV4 accepted=%v and IPV6 accepted=%v (their intersection "+
				"says %v) -- BOTH's accepted set is no longer exactly the intersection of IPV4's and IPV6's",
				identity, both.Accepted, v4.Accepted, v6.Accepted, want)
		}
	}
	t.Logf("BOTH-is-intersection checked across %d protocol identities (name and number rows alike)", checked)
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
// ip_version) pair the controller's own field declarations admit -- by NAME
// and, for a justified subset, by the numeric protocol values the same
// pattern also admits -- whether a firewall policy create using that pair is
// accepted, and if refused, the controller's verbatim message.
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
// A number and its own name are NOT interchangeable to the controller: this
// session measured protocol=58 accepted under IPV4 while protocol=icmpv6
// (58's own name) is refused under IPV4. So the numeric branch of the
// pattern -- 0-255, which an earlier version of this matrix skipped as "a
// range, not a discrete capability" -- gets its own sweep,
// firewallPolicyNumericProtocolDomain, keyed distinctly (protocol_number, not
// protocol) so a consumer can tell the two kinds of row apart without
// guessing.
//
// Candidate third axes beyond protocol and ip_version -- the matching_target
// on source/destination, and the KIND of zone a policy addresses -- are
// spot-checked, not fully swept, below and in
// TestIntegrationFirewallPolicyZoneKindSpotCheck. See those doc comments for
// what was tried and what is still open. Do not delete a hand-written
// literal this matrix disagrees with until that coverage either clears the
// literal or the axis it depends on is measured in full.
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

	// The pattern's numeric branch (GAP 1): a justified partial sweep of
	// 0-255, not every value -- see firewallPolicyNumericProtocolDomain for
	// exactly what is and is not covered and why. Recorded under a
	// protocol_number key so it never collides with, or is mistaken for, a
	// named row.
	numbers := firewallPolicyNumericProtocolDomain(t)
	for _, ipVersion := range versions {
		for _, number := range numbers {
			measured[firewallCapabilityNumericKey(number, ipVersion)] = measureOne(strconv.Itoa(number), ipVersion, nil)
		}
	}
	t.Logf("swept %d numeric protocol values (of the pattern's 0-255 range) x %d ip_versions = %d additional pairs; "+
		"%d of the %d numbers correspond to a name in firewallPolicyProtocolDomain's vocabulary",
		len(numbers), len(versions), len(numbers)*len(versions), len(firewallPolicyProtocolNumberVocabulary), len(numbers))

	// A fact this matrix already established, now checked directly rather
	// than only asserted in a comment: BOTH accepts a protocol -- named or
	// numeric -- exactly when IPV4 and IPV6 both do.
	assertBothIsIntersectionHolds(t, measured)

	// Third-axis spot check (GAP 2, matching_target half): does the
	// matching_target on source/destination change which (protocol,
	// ip_version) pairs are legal? A discriminator here would make the
	// matrix above true only for ANY-matched policies, and publishing it as
	// universal would be the same branch-blindness the artifact's own
	// EmptyWhen section exists to catch elsewhere.
	//
	// What was tried, against the two CUSTOM zones this test already made:
	//
	//   - matching_target=IP, EVERY protocol firewallPolicyProtocolDomain
	//     returns (not a boundary subset -- the full domain), source AND
	//     destination both IP-matched, across all 3 ip_versions. Both v4 and
	//     v6 literal addresses were used so an IPV4 or IPV6 policy's address
	//     matches its own family.
	//   - matching_target=CLIENT on source (a client_macs list; ANY on
	//     destination), across the boundary protocols below and all 3
	//     ip_versions.
	//
	// What was tried and found NOT constructible against a custom zone at
	// all, regardless of protocol -- measured directly, not inferred:
	// matching_target=NETWORK, WEB, REGION, APP, APP_CATEGORY,
	// EXTERNAL_SOURCE and VPN_USER are every one refused with
	// api.err.FirewallPolicy{Source,Destination}MatchingTargetNotApplicableForZone
	// before the controller ever looks at protocol -- a zone-kind gate on
	// the matching_target itself, not a protocol-dependent one, and
	// therefore uninformative about THIS matrix on a custom zone. Once
	// addressed at a zone kind that accepts them (a predefined zone; see
	// TestIntegrationFirewallPolicyZoneKindSpotCheck), NETWORK and WEB
	// (destination's variant of "domain" matching, via web_domains) were
	// re-tried there instead and agreed with the flat matrix -- REGION and
	// APP still could not be constructed even there without a field this
	// SDK does not model (the controller's refusal names "regions" and
	// validates real app-catalog ids, neither of which
	// FirewallPolicySource/Destination expose), so those two stay UNSWEPT
	// for protocol legality: attempting a payload for them without a
	// confirmed field shape would be exactly the "no hand-coded fields" this
	// project refuses to do elsewhere. matching_target=MAC on source (as
	// opposed to CLIENT, which does work) was tried with the same
	// client_macs shape and refused for an unrelated reason
	// (api.err.EmptyFirewallPolicySourceMacs) that this session did not
	// resolve; matching_target=IID crashes the endpoint outright (HTTP 500,
	// non-JSON body) on both source and destination -- a controller defect
	// worth its own report, unrelated to this matrix.
	spotCheckProtocols := []string{"all", "tcp", "icmp", "icmpv6", "ipv6", "ah"}
	spotCheck := map[string]behavior.Capability{}
	ipsFor := func(ipVersion string) []string {
		// An IP-matched policy's address list has to suit the family it
		// polices: an IPV4 policy takes the v4 literal, an IPV6 policy the v6
		// one, and BOTH both -- mixing families into a version-specific
		// policy would draw a family-mismatch refusal that has nothing to do
		// with the protocol/ip_version pairing this spot check is after.
		switch ipVersion {
		case "IPV4":
			return []string{"192.0.2.10/32"}
		case "IPV6":
			return []string{"2001:db8::10/128"}
		default: // BOTH
			return []string{"192.0.2.10/32", "2001:db8::10/128"}
		}
	}
	for _, ipVersion := range versions {
		ips := ipsFor(ipVersion)
		for _, protocol := range protocols {
			overrides := map[string]any{
				"source": map[string]any{
					"zone_id": src, "matching_target": "IP", "matching_target_type": "SPECIFIC", "ips": ips,
				},
				"destination": map[string]any{
					"zone_id": dst, "matching_target": "IP", "matching_target_type": "SPECIFIC", "ips": ips,
				},
			}
			key := "matching_target=IP," + firewallCapabilityKey(protocol, ipVersion)
			spotCheck[key] = measureOne(protocol, ipVersion, overrides)
		}
		for _, protocol := range spotCheckProtocols {
			overrides := map[string]any{
				"source": map[string]any{
					"zone_id": src, "matching_target": "CLIENT", "matching_target_type": "SPECIFIC",
					"client_macs": []string{"00:11:22:33:44:55"},
				},
			}
			key := "matching_target=CLIENT," + firewallCapabilityKey(protocol, ipVersion)
			spotCheck[key] = measureOne(protocol, ipVersion, overrides)
		}
	}
	var diverged []string
	for key, want := range spotCheck {
		_, baseline, _ := strings.Cut(key, ",")
		if got := measured[baseline]; got.Accepted != want.Accepted {
			diverged = append(diverged, fmt.Sprintf("%s: ANY-zone accepted=%v, spot-check accepted=%v (%q)",
				key, got.Accepted, want.Accepted, want.Error))
		}
	}
	if len(diverged) > 0 {
		slices.Sort(diverged)
		t.Errorf("matching_target changes acceptance for %d of %d spot-checked pairs -- matching_target "+
			"is a real third axis and the flat matrix above is branch-blind:\n  %s",
			len(diverged), len(spotCheck), strings.Join(diverged, "\n  "))
	} else {
		t.Logf("spot-checked %d (protocol, ip_version) pairs under matching_target=IP (every protocol) and "+
			"matching_target=CLIENT (the boundary subset): all agreed with the ANY-zone baseline. See the "+
			"test's doc comment for the matching_target values that could not be constructed against a "+
			"custom zone at all, and TestIntegrationFirewallPolicyZoneKindSpotCheck for where those are "+
			"tried against a zone kind that accepts them.", len(spotCheck))
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
