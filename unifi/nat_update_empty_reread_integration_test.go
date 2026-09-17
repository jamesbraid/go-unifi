//go:build integration

// unifi/nat_update_empty_reread_integration_test.go
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

// TestIntegrationNatUpdateEmptyVsAbsentReread re-measures the three nat
// fields TestIntegrationNatUpdateEmptyVsAbsent used to record in
// unifi/behavior_probe_integration_test.go. That test took its EMPTY/OMIT
// verdict from the PUT's own response rather than a GET of the stored
// document -- the exact mistake that put OMIT-CLEARS in schemas/behavior.json
// for 39 fields across hotspotpackage, networkconf, portconf and wlanconf
// before 8b4eeae fixed it there and introduced storedEmptySemantics to do the
// reading correctly. nat was not swept by that fix.
//
// This lives in its own file and calls storedEmptySemantics rather than
// having lived alongside the old test, because both were landing in
// behavior_probe_integration_test.go at once. storedEmptySemantics is
// unexported but package-scoped, so this reaches it without touching the
// file it was declared in. TestIntegrationNatUpdateEmptyVsAbsent has since
// been deleted: its own re-derived verdict for ip_address (OMIT-CLEARS,
// from a PUT response that never carried the field either way) disagreed
// with the ones this test records off a GET and would have failed against
// the artifact this test's own BEHAVIOR_WRITE run produces. There was
// nothing left it measured that this test does not.
//
// Two things came out of measuring this correctly instead of assuming the
// old verdicts were wrong:
//
//  1. description and in_interface, both on a MASQUERADE rule -- the same
//     rule shape the original probe uses -- re-measure IDENTICALLY off a
//     GET: EMPTY-CLEARS/OMIT-CLEARS for description, EMPTY-REJECTED/
//     OMIT-CLEARS for in_interface. The old verdicts were not wrong here.
//     Unlike the v1 rest PUT the earlier bug was measured against, a v2 nat
//     PUT that changes nothing still answers with the full stored document,
//     not an empty data array -- confirmed directly against this
//     controller (a no-op PUT and a PUT with an already-absent key both
//     answered with the complete object) -- so the failure mode that made
//     40 verdicts wrong elsewhere had nothing to bite on here. Same
//     methodology, and it happens to be a distinction without a difference
//     on this particular collection.
//
//  2. ip_address is a different story, and not the one the artifact used to
//     record. "type" (DNAT|SNAT|MASQUERADE) is a discriminator, and
//     ip_address's disposition is not one fact but three:
//
//     - SNAT and DNAT both take a real ip_address (201, stored verbatim)
//     and both reject a create OR update that leaves it out --
//     measured directly, not inferred from one covering the other.
//     - MASQUERADE refuses ip_address outright, for ANY value, seed
//     included -- measured directly, api.err.NatRuleInvalidParameters,
//     "MASQUERADE may not have an IP address translation" for a real
//     value and "Invalid IP Address, Subnet or Range" for "". Omitting
//     the key entirely is accepted every time, because the field can
//     never hold a value on this branch at all -- not because a value
//     was cleared, there was never one to clear.
//
//     The original probe's own seed loop hit exactly the MASQUERADE
//     rejection, logged it, and fell back to "measuring against the bare
//     rule": a baseline where the field was never present to begin with,
//     which makes EMPTY-CLEARS and EMPTY-IGNORED (and OMIT-CLEARS and
//     OMIT-KEEPS) indistinguishable -- storedEmptySemantics refuses to run
//     in that shape at all, for exactly this reason. So the pinned
//     ip_address entry never described a value the field could hold on any
//     branch that was actually tried; it described SNAT alone, filed flat
//     as if every type agreed with it. DNAT does agree with SNAT.
//     MASQUERADE does not, and neverHeldEmptySemantics (this file) records
//     that directly instead of working around the guard that exists to
//     catch exactly this mistake.
//
//     schemas/behavior.json now carries all three under
//     Artifact.EmptyWhen["nat"]["ip_address"], keyed "type=DNAT" /
//     "type=SNAT" / "type=MASQUERADE". The flat Empty["nat"]["ip_address"]
//     entry keeps only what every branch agrees on: all three reject "",
//     so Empty stays EMPTY-REJECTED there. Omit is not unanimous -- DNAT
//     and SNAT reject an absent key, MASQUERADE accepts it unconditionally
//     -- and flattening either answer would tell one type's callers
//     something false (that a write will fail when it will not, or the
//     reverse), so the flat entry says OMIT-VARIES-BY-TYPE and points a
//     reader at EmptyWhen for the real answer, rather than repeating the
//     original mistake with a different single branch's number.
func TestIntegrationNatUpdateEmptyVsAbsentReread(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	wanID := ensureWANNetwork(ctx, t, s, c.Site)
	if wanID == "" {
		t.Fatal("no WAN network; every NAT write would fail for the wrong reason")
	}

	path := "/v2/api/site/" + c.Site + "/nat"
	filter := func() map[string]any {
		return map[string]any{
			"filter_type": "NONE", "firewall_group_ids": []string{},
			"invert_address": false, "invert_port": false,
		}
	}

	// putFor and readFor close over one NAT rule's id. Every verdict here
	// comes from readFor -- a GET of api/s/{site}/nat/{id} -- never from
	// what putFor's own response happened to say, which is the entire
	// point of re-measuring this.
	putFor := func(id string) func(map[string]any) int {
		return func(doc map[string]any) int {
			_, status, err := s.PutJSON(ctx, path+"/"+id, doc)
			mustTransport(t, err)
			return status
		}
	}
	readFor := func(id string) func() map[string]any {
		return func() map[string]any {
			body, status, err := s.GetJSON(ctx, path+"/"+id)
			mustTransport(t, err)
			doc := firstData(t, body)
			if status != 200 || doc == nil {
				t.Fatalf("re-reading %s/%s answered HTTP %d with %v; a verdict cannot be taken "+
					"from a document that did not come back", path, id, status, body)
			}
			return doc
		}
	}

	// measured holds the flat, resource-wide verdicts: description and
	// in_interface (unconditional, one value covers every type), plus the
	// ip_address entry this test derives from ipAddressByType below.
	measured := map[string]behavior.EmptySemantics{}
	// ipAddressByType holds nat.ip_address's verdict per "type" branch, the
	// data EmptyWhen publishes and measured["ip_address"] is derived from.
	ipAddressByType := map[string]behavior.EmptySemantics{}

	t.Run("masquerade", func(t *testing.T) {
		base := map[string]any{
			"enabled": true, "type": "MASQUERADE", "ip_version": "IPV4",
			"protocol": "all", "out_interface": wanID,
			"source_filter": filter(), "destination_filter": filter(),
		}
		body, status, err := s.PostJSON(ctx, path, base)
		mustTransport(t, err)
		if status/100 != 2 {
			t.Fatalf("the known-good MASQUERADE body did not create (HTTP %d): %v.\n\n"+
				"A probe with nothing to update measures nothing.", status, body)
		}
		stored := firstData(t, body)
		id := objectID(stored)
		if id == "" {
			t.Fatalf("no id in create response: %v", stored)
		}
		defer s.DeleteJSON(context.WithoutCancel(ctx), path+"/"+id) //nolint:errcheck
		put, read := putFor(id), readFor(id)

		for _, tc := range []struct{ field, seed string }{
			{"in_interface", wanID},
			{"description", "clear-probe"},
		} {
			seedDoc := clone(stored)
			seedDoc[tc.field] = tc.seed
			if st := put(seedDoc); st/100 != 2 {
				t.Fatalf("seeding %s=%q on a MASQUERADE rule was rejected (HTTP %d); this field "+
					"was expected to hold a value here", tc.field, tc.seed, st)
			}
			seeded := read()
			measured[tc.field] = storedEmptySemantics(t, tc.field, "", nil, seeded, put, read)
			put(clone(stored)) // reset for the next field
		}

		// ip_address is never seeded above: a create carrying one is
		// rejected outright (confirmed separately, see the doc comment),
		// so `stored` already is the never-held baseline this helper
		// requires.
		ipAddressByType["MASQUERADE"] = neverHeldEmptySemantics(t, "ip_address", stored, put, read)
	})

	t.Run("snat", func(t *testing.T) {
		base := map[string]any{
			"enabled": true, "type": "SNAT", "ip_version": "IPV4",
			"protocol": "all", "out_interface": wanID, "ip_address": "192.0.2.55",
			"source_filter": filter(), "destination_filter": filter(),
		}
		body, status, err := s.PostJSON(ctx, path, base)
		mustTransport(t, err)
		if status/100 != 2 {
			t.Fatalf("the known-good SNAT body did not create (HTTP %d): %v.\n\n"+
				"A probe with nothing to update measures nothing.", status, body)
		}
		stored := firstData(t, body)
		id := objectID(stored)
		if id == "" {
			t.Fatalf("no id in create response: %v", stored)
		}
		defer s.DeleteJSON(context.WithoutCancel(ctx), path+"/"+id) //nolint:errcheck
		put, read := putFor(id), readFor(id)

		ipAddressByType["SNAT"] = storedEmptySemantics(t, "ip_address", "", nil, stored, put, read)
	})

	t.Run("dnat", func(t *testing.T) {
		// DNAT rewrites the destination of traffic arriving on an
		// interface, so it takes in_interface where SNAT/MASQUERADE take
		// out_interface -- confirmed directly (out_interface alone was
		// never tried against a live DNAT create; in_interface plus
		// ip_address created and re-read verbatim on the first attempt).
		base := map[string]any{
			"enabled": true, "type": "DNAT", "ip_version": "IPV4",
			"protocol": "all", "in_interface": wanID, "ip_address": "192.0.2.66",
			"source_filter": filter(), "destination_filter": filter(),
		}
		body, status, err := s.PostJSON(ctx, path, base)
		mustTransport(t, err)
		if status/100 != 2 {
			t.Fatalf("the known-good DNAT body did not create (HTTP %d): %v.\n\n"+
				"A probe with nothing to update measures nothing.", status, body)
		}
		stored := firstData(t, body)
		id := objectID(stored)
		if id == "" {
			t.Fatalf("no id in create response: %v", stored)
		}
		defer s.DeleteJSON(context.WithoutCancel(ctx), path+"/"+id) //nolint:errcheck
		put, read := putFor(id), readFor(id)

		ipAddressByType["DNAT"] = storedEmptySemantics(t, "ip_address", "", nil, stored, put, read)
	})

	// The flat entry keeps only what every branch agrees on -- the same
	// relationship WriteContract.RequiredOnCreate has to RequiredOnCreateWhen.
	// Where the branches disagree, a single flattened answer would be false
	// for whichever branch it does not describe, so the flat side names the
	// disagreement instead of picking a branch to speak for the others.
	agree := func(get func(behavior.EmptySemantics) string, marker string) string {
		types := slices.Sorted(maps.Keys(ipAddressByType))
		first := get(ipAddressByType[types[0]])
		for _, typ := range types[1:] {
			if get(ipAddressByType[typ]) != first {
				return marker
			}
		}
		return first
	}
	measured["ip_address"] = behavior.EmptySemantics{
		Empty: agree(func(e behavior.EmptySemantics) string { return e.Empty }, "EMPTY-VARIES-BY-TYPE"),
		Omit:  agree(func(e behavior.EmptySemantics) string { return e.Omit }, "OMIT-VARIES-BY-TYPE"),
	}

	var summary []string
	for _, f := range slices.Sorted(maps.Keys(measured)) {
		summary = append(summary, fmt.Sprintf("%-16s %-24s %s", f, measured[f].Empty, measured[f].Omit))
	}
	for _, typ := range slices.Sorted(maps.Keys(ipAddressByType)) {
		summary = append(summary, fmt.Sprintf("%-16s %-24s %s", "ip_address[type="+typ+"]",
			ipAddressByType[typ].Empty, ipAddressByType[typ].Omit))
	}
	t.Logf("nat empty-vs-absent semantics, re-measured off a GET rather than the PUT response:\n  %s",
		strings.Join(summary, "\n  "))

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Empty == nil {
				a.Empty = map[string]map[string]behavior.EmptySemantics{}
			}
			if a.Empty["nat"] == nil {
				a.Empty["nat"] = map[string]behavior.EmptySemantics{}
			}
			for f, sem := range measured {
				a.Empty["nat"][f] = sem
			}
			if a.EmptyWhen == nil {
				a.EmptyWhen = map[string]map[string]map[string]behavior.EmptySemantics{}
			}
			if a.EmptyWhen["nat"] == nil {
				a.EmptyWhen["nat"] = map[string]map[string]behavior.EmptySemantics{}
			}
			branch := map[string]behavior.EmptySemantics{}
			for typ, sem := range ipAddressByType {
				branch["type="+typ] = sem
			}
			a.EmptyWhen["nat"]["ip_address"] = branch
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	pinned := art.Empty["nat"]
	pinnedWhen := art.EmptyWhen["nat"]["ip_address"]
	if !ok || pinned == nil {
		t.Logf("no pinned empty semantics for nat in %s; run with BEHAVIOR_WRITE=1 to record them", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would "+
			"file a version difference as drift", art.ControllerVersion, running)
	}
	for f, got := range measured {
		want, has := pinned[f]
		if !has {
			t.Errorf("nat.%s: measured empty=%s omit=%s but the artifact pins nothing; "+
				"re-measure with BEHAVIOR_WRITE=1", f, got.Empty, got.Omit)
			continue
		}
		if got != want {
			t.Errorf("nat.%s: re-measured off a GET as empty=%s omit=%s, but the artifact pins "+
				"empty=%s omit=%s. Re-measure with BEHAVIOR_WRITE=1 once this is understood.",
				f, got.Empty, got.Omit, want.Empty, want.Omit)
		}
	}
	for typ, got := range ipAddressByType {
		key := "type=" + typ
		want, has := pinnedWhen[key]
		if !has {
			t.Errorf("nat.ip_address[%s]: measured empty=%s omit=%s but the artifact's EmptyWhen "+
				"pins nothing for this branch; re-measure with BEHAVIOR_WRITE=1", key, got.Empty, got.Omit)
			continue
		}
		if got != want {
			t.Errorf("nat.ip_address[%s]: re-measured off a GET as empty=%s omit=%s, but the "+
				"artifact pins empty=%s omit=%s. Re-measure with BEHAVIOR_WRITE=1 once this is "+
				"understood.", key, got.Empty, got.Omit, want.Empty, want.Omit)
		}
	}
}
