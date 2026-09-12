//go:build integration

// unifi/rejected_create_probe_integration_test.go
package unifi

import (
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// rejectedCreateProbe is one endpoint and a create the controller rejects,
// marked so the collection can be searched for the document afterwards.
type rejectedCreateProbe struct {
	resource string
	path     string // absolute request path, built per site below
	marker   string // wire field carrying the probe's identifying value
	payload  func(name string, deps probeDeps) map[string]any
}

// One probe per rejection class the suite has met, because the classes
// answer differently and a row that repeats another's mechanism measures
// nothing new:
//
//   - FirewallZone: the body is VALID; the write path stores the document
//     and then fails resolving the site's HOTSPOT-keyed zone, which an
//     unmigrated site lacks (see cmd/fields/gateway_probe_integration_test.go).
//     The stored document survives only when this is the site's first zone
//     call, so this row runs first and nothing here touches the collection
//     before it.
//   - Network: a v1 validation rejection (pattern mismatch).
//   - Nat: the v2 unhandled rejection -- a missing filter object draws a
//     non-JSON 500.
func rejectedCreateProbes(site string) []rejectedCreateProbe {
	return []rejectedCreateProbe{
		{
			resource: "FirewallZone",
			path:     "/v2/api/site/" + site + "/firewall/zone",
			marker:   "name",
			payload: func(name string, deps probeDeps) map[string]any {
				return map[string]any{"name": name, "network_ids": []string{}}
			},
		},
		{
			resource: "Network",
			path:     "/api/s/" + site + "/rest/networkconf",
			marker:   "name",
			payload: func(name string, deps probeDeps) map[string]any {
				return map[string]any{
					"name": name, "purpose": PurposeCorporate, "enabled": true,
					"ip_subnet": "not-a-subnet",
				}
			},
		},
		{
			resource: "Nat",
			path:     "/v2/api/site/" + site + "/nat",
			marker:   "description",
			payload: func(name string, deps probeDeps) map[string]any {
				return map[string]any{
					"description": name, "enabled": true,
					"type": "MASQUERADE", "ip_version": "IPV4", "protocol": "all",
					"out_interface":      deps.wanNetworkID,
					"destination_filter": map[string]any{"filter_type": "NONE", "firewall_group_ids": []string{}},
				}
			},
		},
	}
}

// TestIntegrationRejectedCreatePersistence measures what a rejected create
// leaves behind: POST a body the controller refuses, then search the
// collection for the document anyway.
//
// A rejection that stores is why the transport must never replay a POST it
// saw fail -- each replay files another copy. Measured on 10.6.101: the
// FirewallZone create is that case (rejected 404, stored), while the v1
// validation rejection and the v2 unhandled 500 store nothing.
func TestIntegrationRejectedCreatePersistence(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 20*time.Minute)

	root, captured := capturedBehaviorVersion(t)
	running := runningControllerVersion(ctx, t, s, c.Site)
	if behaviorWriteRequested() && running != captured {
		t.Fatalf("BEHAVIOR_WRITE=1 but the booted controller reports %s while schemas/VERSION says %s; "+
			"recording would file the measurement against the wrong controller", running, captured)
	}

	deps := probeDeps{wanNetworkID: ensureWANNetwork(ctx, t, s, c.Site)}

	measured := map[string]behavior.RejectedCreate{}
	for _, probe := range rejectedCreateProbes(c.Site) {
		t.Run(probe.resource, func(t *testing.T) {
			const name = "rejected-create-probe"

			body, status, err := s.PostJSON(ctx, probe.path, probe.payload(name, deps))
			if status == 0 {
				t.Fatalf("transport to %s: %v", probe.path, err)
			}
			// The response body names the rejection (api.err.*); a non-JSON
			// body is itself half the 500-class finding.
			t.Logf("POST %s -> HTTP %d: %v %v", probe.path, status, body, err)
			if status/100 == 2 {
				t.Errorf("the payload was accepted (HTTP %d); it no longer measures a rejected create, "+
					"so this probe needs a body the controller still refuses", status)
				if id := objectID(firstData(t, body)); id != "" {
					s.DeleteJSON(ctx, probe.path+"/"+id) //nolint:errcheck
				}
				return
			}

			listBody, listStatus, err := s.GetJSON(ctx, probe.path)
			if err != nil || listStatus != 200 {
				t.Fatalf("list %s (HTTP %d): %v -- persistence cannot be measured without the collection",
					probe.path, listStatus, err)
			}
			var items []any
			switch v := listBody.(type) {
			case map[string]any: // v1 meta/data envelope
				items, _ = v["data"].([]any)
			case []any: // v2 bare array
				items = v
			}
			id := ""
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
					if got, _ := m[probe.marker].(string); got == name {
						id = objectID(m)
						break
					}
				}
			}
			if id != "" {
				t.Logf("%s: rejected with HTTP %d and STORED anyway (id %s)", probe.resource, status, id)
				// Best effort: a later probe in this container must not
				// measure against the corpse.
				s.DeleteJSON(ctx, probe.path+"/"+id) //nolint:errcheck
			} else {
				t.Logf("%s: rejected with HTTP %d, nothing stored", probe.resource, status)
			}
			measured[probe.resource] = behavior.RejectedCreate{Status: status, Stored: id != ""}
		})
	}

	if behaviorWriteRequested() {
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			// This probe is the section's only writer, so it replaces the
			// whole map: a row removed from the table leaves the artifact
			// too instead of pinning a measurement nothing re-runs.
			a.RejectedCreates = measured
		})
		return
	}

	art, ok, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if !ok || len(art.RejectedCreates) == 0 {
		t.Logf("no pinned rejected-create facts in %s; run with BEHAVIOR_WRITE=1 to record them", behavior.Path)
		return
	}
	if art.ControllerVersion != running {
		t.Skipf("artifact was measured on %s, this controller reports %s; comparing them would file "+
			"a version difference as drift", art.ControllerVersion, running)
	}
	for resource, got := range measured {
		want, pinned := art.RejectedCreates[resource]
		if !pinned {
			t.Errorf("%s: measured a rejected create (HTTP %d, stored=%v) but the artifact pins nothing; "+
				"re-measure with BEHAVIOR_WRITE=1", resource, got.Status, got.Stored)
			continue
		}
		if want != got {
			t.Errorf("%s: artifact pins HTTP %d stored=%v, measured HTTP %d stored=%v; the controller "+
				"changed or the artifact is stale -- re-measure with BEHAVIOR_WRITE=1",
				resource, want.Status, want.Stored, got.Status, got.Stored)
		}
	}
}
