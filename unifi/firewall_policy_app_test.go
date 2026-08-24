package unifi

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestFirewallPolicyAppMatchingRoundTrip covers APP/APP_CATEGORY matching on
// zone-based firewall policies: the controller stores app_ids /
// app_category_ids as arrays of JSON numbers on the source/destination
// endpoint objects (payload shaped like a live UDM APP policy, neutral IDs),
// and string-form numbers must decode too via the tolerant types.Number
// path.
func TestFirewallPolicyAppMatchingRoundTrip(t *testing.T) {
	raw := `{
		"_id": "aaaa0000000000000000f001",
		"action": "BLOCK",
		"enabled": true,
		"name": "block app dns",
		"index": 10000,
		"predefined": false,
		"protocol": "all",
		"ip_version": "BOTH",
		"destination": {
			"app_ids": [589885, 1310919],
			"app_category_ids": [13],
			"match_opposite_ports": false,
			"matching_target": "APP",
			"port_matching_type": "ANY",
			"zone_id": "aaaa0000000000000000e001"
		},
		"source": {
			"client_macs": ["aa:bb:cc:00:00:01"],
			"matching_target": "CLIENT",
			"matching_target_type": "SPECIFIC",
			"port_matching_type": "ANY",
			"zone_id": "aaaa0000000000000000e002"
		}
	}`

	var p FirewallPolicy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Destination == nil {
		t.Fatal("destination = nil")
	}
	if p.Destination.MatchingTarget != "APP" {
		t.Errorf("matching_target = %q, want APP", p.Destination.MatchingTarget)
	}
	if len(p.Destination.AppIDs) != 2 || p.Destination.AppIDs[0] != 589885 || p.Destination.AppIDs[1] != 1310919 {
		t.Errorf("app_ids = %v", p.Destination.AppIDs)
	}
	if len(p.Destination.AppCategoryIDs) != 1 || p.Destination.AppCategoryIDs[0] != 13 {
		t.Errorf("app_category_ids = %v", p.Destination.AppCategoryIDs)
	}

	// String-form numbers must decode too (types.Number tolerance).
	var src FirewallPolicySource
	if err := json.Unmarshal([]byte(`{"app_ids": ["589885"], "matching_target": "APP", "zone_id": "z"}`), &src); err != nil {
		t.Fatalf("unmarshal string-form app_ids: %v", err)
	}
	if len(src.AppIDs) != 1 || src.AppIDs[0] != 589885 {
		t.Errorf("string-form app_ids = %v", src.AppIDs)
	}

	// Marshal: populated app fields serialize as JSON number arrays…
	b, err := json.Marshal(p.Destination)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"app_ids":[589885,1310919]`) {
		t.Errorf("marshal lost app_ids: %s", b)
	}
	if !strings.Contains(string(b), `"app_category_ids":[13]`) {
		t.Errorf("marshal lost app_category_ids: %s", b)
	}

	// …and absent app fields are omitted entirely (omitempty), so non-APP
	// policies keep their existing wire shape.
	b2, err := json.Marshal(FirewallPolicySource{ZoneID: "z", MatchingTarget: "ANY"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b2), "app_ids") || strings.Contains(string(b2), "app_category_ids") {
		t.Errorf("empty app fields must be omitted: %s", b2)
	}
}
