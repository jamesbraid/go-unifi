//go:build integration

// unifi/firewall_zone_update_integration_test.go
package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationFirewallZoneUpdateAfterRead checks the SDK's half of
// update-after-read against a live controller: read a zone the way a consumer
// does, change a field, and confirm the bytes the SDK would PUT carry only
// fields the write DTO accepts.
//
// It asserts the payload, not the outcome, because the controller cannot
// complete a zone update on this harness at all. Measured 2026-07-26 on
// 10.4.57: with a DTO-exact body ({"_id","name","network_ids"}) the PUT answers
// 404 api.err.CouldNotFindHotspotFirewallZone and does NOT apply — the rename
// is absent from the collection afterwards. That is the same hotspot-zone
// lookup the POST hits, except POST persists despite it and PUT does not. A
// site that has never had a hotspot zone therefore cannot be updated, whatever
// the SDK sends.
//
// The SDK's own contribution was separate and is fixed: DefaultZone is a *bool,
// so any read left it non-nil and omitempty could not drop it, and ExternalID
// is always populated on a read — both used to ride along and draw HTTP 400
// "Unrecognized field" before the write DTO was ever consulted. This test
// guards that regression live, with values a real controller supplied; the
// unit tests in firewall_zone_test.go pin the same shape offline.
//
// It sends the marshalled struct over the raw session rather than through
// ApiClient so the subject is the encoder's bytes, which is where the bug was.
func TestIntegrationFirewallZoneUpdateAfterRead(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)
	path := fmt.Sprintf("/v2/api/site/%s/firewall/zone", c.Site)

	// Seed a zone to edit. The POST must be this site's FIRST request to the
	// zone collection or the write does not persist, so do not list first —
	// see seedFirewallZone in cmd/fields/drift_integration_test.go. The 404 is
	// expected and the write lands anyway.
	body, status, err := s.PostJSON(ctx, path, map[string]any{
		"name":        "update-probe-zone",
		"network_ids": []string{},
	})
	if err != nil {
		t.Fatalf("seed zone: transport error: %v", err)
	}
	t.Logf("seed zone: POST answered HTTP %d (the write lands regardless): %v", status, body)

	listBody, status, err := s.GetJSON(ctx, path)
	if err != nil || status != 200 {
		t.Fatalf("list zones: status=%d err=%v", status, err)
	}
	raw, err := json.Marshal(listBody)
	if err != nil {
		t.Fatalf("re-marshal list: %v", err)
	}
	var zones []FirewallZone
	if err := json.Unmarshal(raw, &zones); err != nil {
		t.Fatalf("decode zones into the SDK type: %v", err)
	}

	var zone *FirewallZone
	for i := range zones {
		if zones[i].Name == "update-probe-zone" {
			zone = &zones[i]
			break
		}
	}
	if zone == nil {
		t.Fatalf("seeded zone absent from the collection: %s", raw)
	}

	// The read populated exactly the fields that used to poison the write.
	// If the controller ever stops sending them this test still passes, but
	// it stops proving anything, so say so rather than assert.
	if zone.ExternalID == "" {
		t.Log("note: controller did not report external_id, so this run does not exercise that half")
	}
	if zone.DefaultZone == nil {
		t.Log("note: controller did not report default_zone, so this run does not exercise that half")
	}

	zone.Name = "update-probe-zone-renamed"
	payload, err := json.Marshal(zone)
	if err != nil {
		t.Fatalf("marshal zone: %v", err)
	}
	t.Logf("PUT body the SDK produces: %s", payload)

	var putBody any
	if err := json.Unmarshal(payload, &putBody); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	// The live assertion: the encoder must offer the controller only what its
	// write DTO accepts. This runs on a zone the controller itself populated,
	// so a regression that reintroduces default_zone or external_id fails here
	// even if the offline tests are edited to match it.
	var sent map[string]any
	if err := json.Unmarshal(payload, &sent); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	writable := map[string]bool{"_id": true, "name": true, "network_ids": true}
	for k := range sent {
		if !writable[k] {
			t.Errorf("SDK would PUT %q, which the zone write DTO rejects: %s", k, payload)
		}
	}

	respBody, status, err := s.PutJSON(ctx, fmt.Sprintf("%s/%s", path, zone.ID), sent)
	if err != nil {
		t.Fatalf("update zone: transport error: %v", err)
	}
	t.Logf("update answered HTTP %d: %v", status, respBody)

	// Record what the controller actually did. This is a finding, not a
	// verdict on the SDK: if a future build starts applying the update, this
	// log is the signal to promote it to an assertion.
	listBody, listStatus, err := s.GetJSON(ctx, path)
	if err != nil || listStatus != 200 {
		t.Fatalf("list zones after update: status=%d err=%v", listStatus, err)
	}
	after, _ := json.Marshal(listBody)
	var updated []FirewallZone
	if err := json.Unmarshal(after, &updated); err != nil {
		t.Fatalf("decode zones after update: %v", err)
	}
	for _, z := range updated {
		if z.ID != zone.ID {
			continue
		}
		if z.Name == "update-probe-zone-renamed" {
			t.Logf("UPDATE APPLIED (HTTP %d) — the controller now completes zone updates; "+
				"promote this to an assertion", status)
		} else {
			t.Logf("UPDATE NOT APPLIED (HTTP %d): name is still %q. The PUT body was DTO-exact (%s), "+
				"so this is the controller's hotspot-zone gate, not the SDK.", status, z.Name, payload)
		}
		return
	}
	t.Fatalf("zone %s vanished after the update: %s", zone.ID, after)
}
