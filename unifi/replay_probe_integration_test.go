//go:build integration

// unifi/replay_probe_integration_test.go
package unifi

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/internal/probe"
)

// TestIntegrationWriteReplay measures what re-sending an identical write
// does, per API family. The transport replays PUT on 5xx and never replays
// POST; this is the measured ground under that policy, recorded where a
// controller bump re-measures it.
//
// Four measurements, each its own fact:
//   - WLAN create replay: nothing in the body is unique to the controller,
//     and the replay files a second document under the same name.
//   - Network create replay: the identical body re-offers its VLAN, which
//     the first document now holds, so the copy is rejected -- an
//     attribute collision, not a replay guard, which is why the WLAN row
//     answers differently.
//   - Nat create replay: the other API family; the controller assigns
//     rule_index and refuses the replay as a duplicate of it.
//   - Network update replay: PUT the document exactly as stored, twice,
//     and compare what each left -- the idempotency the retry policy
//     leans on.
func TestIntegrationWriteReplay(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 20*time.Minute)

	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	v1Path := "/api/s/" + c.Site + "/rest/networkconf"
	v2Path := "/v2/api/site/" + c.Site + "/nat"

	measured := map[string]behavior.Replay{}

	// replayCreate POSTs the same body twice and classifies the second
	// answer. Both documents are deleted so the next measurement runs on a
	// clean collection.
	replayCreate := func(t *testing.T, path string, body map[string]any) string {
		first, status, err := s.PostJSON(ctx, path, body)
		if err != nil || status/100 != 2 {
			t.Fatalf("first create rejected (HTTP %d): %v %v -- a replay of a failed create measures "+
				"the rejection, not the replay", status, first, err)
		}
		id1 := objectID(firstData(t, first))
		defer s.DeleteJSON(ctx, path+"/"+id1) //nolint:errcheck

		second, status, err := s.PostJSON(ctx, path, body)
		if status == 0 {
			t.Fatalf("transport on the replay: %v", err)
		}
		if status/100 != 2 {
			t.Logf("replay rejected (HTTP %d): %v", status, second)
			return fmt.Sprintf("REJECTED-%d", status)
		}
		id2 := objectID(firstData(t, second))
		if id2 == "" || id2 == id1 {
			t.Fatalf("replay accepted (HTTP %d) but returned id %q (first was %q); neither a duplicate "+
				"nor a rejection, so the vocabulary does not cover it: %v", status, id2, id1, second)
		}
		t.Logf("replay accepted: second document %s beside %s", id2, id1)
		s.DeleteJSON(ctx, path+"/"+id2) //nolint:errcheck
		return "DUPLICATES"
	}

	t.Run("WLAN create", func(t *testing.T) {
		verdict := replayCreate(t, "/api/s/"+c.Site+"/rest/wlanconf",
			wlanBase("replay-create-probe", probeDeps{apGroupID: firstAPGroupID(ctx, t, s, c.Site)}))
		entry := measured["WLAN"]
		entry.Create = verdict
		measured["WLAN"] = entry
	})

	t.Run("Network create", func(t *testing.T) {
		verdict := replayCreate(t, v1Path, map[string]any{
			"name": "replay-create-probe", "purpose": PurposeCorporate, "enabled": true,
			"ip_subnet": "10.97.0.1/24", "vlan_enabled": true, "vlan": 970, "networkgroup": "LAN",
		})
		entry := measured["Network"]
		entry.Create = verdict
		measured["Network"] = entry
	})

	t.Run("Nat create", func(t *testing.T) {
		wanID := ensureWANNetwork(ctx, t, s, c.Site)
		if wanID == "" {
			t.Fatal("no WAN network; the NAT create would fail for the wrong reason")
		}
		filter := map[string]any{"filter_type": "NONE", "firewall_group_ids": []string{}}
		verdict := replayCreate(t, v2Path, map[string]any{
			"description": "replay-create-probe", "enabled": true,
			"type": "MASQUERADE", "ip_version": "IPV4", "protocol": "all",
			"out_interface": wanID, "source_filter": filter, "destination_filter": filter,
		})
		entry := measured["Nat"]
		entry.Create = verdict
		measured["Nat"] = entry
	})

	t.Run("Network update", func(t *testing.T) {
		body, status, err := s.PostJSON(ctx, v1Path, map[string]any{
			"name": "replay-update-probe", "purpose": PurposeCorporate, "enabled": true,
			"ip_subnet": "10.98.0.1/24", "vlan_enabled": true, "vlan": 980, "networkgroup": "LAN",
		})
		if err != nil || status != 200 {
			t.Fatalf("seed network rejected (HTTP %d): %v %v", status, body, err)
		}
		doc := firstData(t, body)
		id := objectID(doc)
		defer s.DeleteJSON(ctx, v1Path+"/"+id) //nolint:errcheck

		// PUT the document exactly as the controller stored it -- the bytes
		// a read-modify-write client would replay on a timeout.
		put := func(n int) map[string]any {
			body, status, err := s.PutJSON(ctx, v1Path+"/"+id, doc)
			if err != nil || status != 200 {
				t.Fatalf("PUT %d rejected (HTTP %d): %v %v -- an update replay that cannot even run "+
					"once measures nothing", n, status, body, err)
			}
			return firstData(t, body)
		}
		first := put(1)
		second := put(2)

		var changed []string
		for _, r := range probe.Classify(first, second) {
			changed = append(changed, r.Wire)
			t.Logf("replayed PUT moved %s (%s)", r.Wire, r.Detail)
		}
		sort.Strings(changed)
		entry := measured["Network"]
		if len(changed) == 0 {
			entry.Update = "IDEMPOTENT"
		} else {
			entry.Update = "CHANGED-" + strings.Join(changed, ",")
		}
		measured["Network"] = entry
	})

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			// This probe is the section's only writer; replacing the map
			// keeps a removed row from pinning a measurement nothing re-runs.
			a.Replays = measured
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok || len(art.Replays) == 0 {
		t.Logf("no pinned replay facts in %s; run with BEHAVIOR_WRITE=1 to record them", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	for resource, got := range measured {
		want, pinned := art.Replays[resource]
		if !pinned {
			t.Errorf("%s: measured a replay (%+v) but the artifact pins nothing; re-measure with "+
				"BEHAVIOR_WRITE=1", resource, got)
			continue
		}
		if want != got {
			t.Errorf("%s: artifact pins %+v, measured %+v; the controller changed or the artifact is "+
				"stale -- re-measure with BEHAVIOR_WRITE=1", resource, want, got)
		}
	}
}
