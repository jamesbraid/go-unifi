package unifi

import (
	"encoding/json"
	"strings"
	"testing"
)

// Regression for terraform-provider-unifi zone CREATE 500: an empty NetworkIDs
// must serialize as "network_ids":[] (the controller 500s when the field is
// omitted). Before dropping omitempty this body had no network_ids key.
func TestFirewallZoneMarshalIncludesEmptyNetworkIDs(t *testing.T) {
	b, err := json.Marshal(&FirewallZone{Name: "tf-canary", NetworkIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, `"network_ids":[]`) {
		t.Fatalf("expected network_ids to be present as []; got %s", got)
	}
	if strings.Contains(got, "default_zone") {
		t.Fatalf("default_zone should be omitted on write; got %s", got)
	}
}

// TestFirewallZoneMarshalDropsReadOnlyFields pins the write shape of a zone.
//
// The controller's zone write DTO accepts exactly _id, name and network_ids and
// answers HTTP 400 "Unrecognized field <x>" for anything else, while its read
// emits the server-assigned fields too. Marshalling a zone straight back
// therefore has to drop them, which is what the generated MarshalJSON does.
//
// The regression this guards is update-after-read: every field below is set
// here the way a read from a live controller sets it, so a MarshalJSON that
// stopped dropping them would put back the 400 rather than merely widen the
// payload.
func TestFirewallZoneMarshalDropsReadOnlyFields(t *testing.T) {
	defaultZone := false
	zone := FirewallZone{
		ID:            "6a644f0126fed89807ac4720",
		SiteID:        "6a643d246616fcb0dfc16fea",
		NoEdit:        true,
		Hidden:        true,
		HiddenID:      "hotspot",
		NoDelete:      true,
		CloudTemplate: "some-template",
		DefaultZone:   &defaultZone,
		ExternalID:    "838e83e6-b4e1-4bc2-b28c-3a6a418ac478",
		ZoneKey:       "hotspot",
		Name:          "probe-zone",
		NetworkIDs:    []string{},
	}

	raw, err := json.Marshal(&zone)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal round trip: %v", err)
	}

	want := map[string]bool{"_id": true, "name": true, "network_ids": true}
	for k := range got {
		if !want[k] {
			t.Errorf("marshalled zone carries %q, which the controller rejects on a write: %s", k, raw)
		}
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("marshalled zone is missing writable field %q: %s", k, raw)
		}
	}
}

// TestFirewallZoneUnmarshalKeepsReadOnlyFields is the other half of the
// contract: read-only means "not written", not "not read". Dropping these from
// the struct would lose data every consumer can see in the UI.
func TestFirewallZoneUnmarshalKeepsReadOnlyFields(t *testing.T) {
	const body = `{
		"_id": "6a644f0126fed89807ac4720",
		"attr_no_edit": true,
		"cloud_template": "tmpl",
		"default_zone": true,
		"external_id": "838e83e6-b4e1-4bc2-b28c-3a6a418ac478",
		"name": "probe-zone",
		"network_ids": ["abc"],
		"zone_key": "hotspot"
	}`

	var zone FirewallZone
	if err := json.Unmarshal([]byte(body), &zone); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if zone.ExternalID != "838e83e6-b4e1-4bc2-b28c-3a6a418ac478" {
		t.Errorf("ExternalID = %q, want it preserved on read", zone.ExternalID)
	}
	if zone.ZoneKey != "hotspot" {
		t.Errorf("ZoneKey = %q, want it preserved on read", zone.ZoneKey)
	}
	if zone.CloudTemplate != "tmpl" {
		t.Errorf("CloudTemplate = %q, want it preserved on read", zone.CloudTemplate)
	}
	if zone.DefaultZone == nil || !*zone.DefaultZone {
		t.Errorf("DefaultZone = %v, want it preserved on read", zone.DefaultZone)
	}
	if !zone.NoEdit {
		t.Error("NoEdit = false, want it preserved on read")
	}
}
