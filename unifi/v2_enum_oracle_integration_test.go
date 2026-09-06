//go:build integration

package unifi

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationV2EnumsMatchTheController checks the generated value lists
// for the v2 resources against the controller itself, the same way the
// firewall-policy test does: handed a value it cannot deserialize, the
// controller answers with the enum class and every constant in it.
//
// These are v2 fields, hand-maintained in overrides/resources, and this
// oracle is what caught record_type promising PTR and SOA the controller has
// never accepted, and matching_target missing the REGION the controller
// grew. Deserialization runs before any site-state validation, so none of
// these probes needs seeding: a bare controller answers them all.
func TestIntegrationV2EnumsMatchTheController(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	// Ask the controller to name an enum's constants by handing it one it
	// cannot parse. The payload only has to be well-formed JSON that reaches
	// the field: Jackson fails on the bad token before validation runs.
	controllerEnum := func(t *testing.T, path string, payload map[string]any) []string {
		t.Helper()
		body, status, err := s.PostJSON(ctx, "/v2/api/site/"+c.Site+"/"+path, payload)
		if err != nil {
			t.Fatalf("transport: %v", err)
		}
		m, _ := body.(map[string]any)
		msg, _ := m["message"].(string)
		match := enumClassRe.FindStringSubmatch(msg)
		if match == nil {
			t.Fatalf("the controller did not answer with an enum class list, so this test "+
				"cannot read its accepted values (HTTP %d). It said: %s", status, msg)
		}
		values := strings.Split(match[1], ", ")
		slices.Sort(values)
		return values
	}

	for _, tc := range []struct {
		name      string
		path      string
		payload   map[string]any
		generated []string
	}{
		{
			name: "DNSRecord.record_type", path: "static-dns",
			payload:   map[string]any{"record_type": "NOT_A_VALID_VALUE_XYZ"},
			generated: DNSRecordRecordTypeValues,
		},
		{
			name: "TrafficRoute.matching_target", path: "trafficroutes",
			payload:   map[string]any{"matching_target": "NOT_A_VALID_VALUE_XYZ"},
			generated: TrafficRouteMatchingTargetValues,
		},
		{
			name: "Nat.type", path: "nat",
			payload:   map[string]any{"type": "NOT_A_VALID_VALUE_XYZ"},
			generated: NatTypeValues,
		},
		{
			name: "Nat.ip_version", path: "nat",
			payload:   map[string]any{"type": "SNAT", "ip_version": "NOT_A_VALID_VALUE_XYZ"},
			generated: NatVersionValues,
		},
		{
			name: "Nat.setting_preference", path: "nat",
			payload:   map[string]any{"type": "SNAT", "setting_preference": "NOT_A_VALID_VALUE_XYZ"},
			generated: NatSettingPreferenceValues,
		},
		{
			name: "NatSourceFilter.filter_type", path: "nat",
			payload: map[string]any{
				"type":          "SNAT",
				"source_filter": map[string]any{"filter_type": "NOT_A_VALID_VALUE_XYZ"},
			},
			generated: NatSourceFilterFilterTypeValues,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := controllerEnum(t, tc.path, tc.payload)
			got := slices.Clone(tc.generated)
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("the generated values for %s are %v; the controller accepts %v.\n\n"+
					"This is a v2 field, so its definition is hand-maintained in "+
					"overrides/resources -- update it there and regenerate.", tc.name, got, want)
			}
		})
	}

	// ospf/router area_type deserializes by toString(), which is lowercase,
	// so a bad value draws no constant list and the oracle above cannot read
	// it. Measure by acceptance instead: every generated value must create a
	// router (deleted again at once -- the controller allows only one), and
	// the uppercase constant name must stay rejected, or the lowercase list
	// in overrides/resources/OSPFRouter.json has gone stale.
	t.Run("OSPFRouterAreas.area_type", func(t *testing.T) {
		lanID := firstNetworkIDForPurpose(ctx, t, s, c.Site, PurposeCorporate)
		if lanID == "" {
			t.Skip("no corporate network to put in the OSPF area")
		}
		path := "/v2/api/site/" + c.Site + "/ospf/router"
		post := func(areaType string) (int, map[string]any) {
			// The backbone area 0.0.0.0 may only be "normal", so probe the
			// stub and nssa flavours on a non-backbone area id.
			areaID := "0.0.0.51"
			if areaType == "normal" || areaType == "NORMAL" {
				areaID = "0.0.0.0"
			}
			body, status, err := s.PostJSON(ctx, path, map[string]any{
				"enabled": true, "router_id": "10.255.1.1",
				"announce_default_route":        false,
				"redistribute_bgp_routes":       false,
				"redistribute_connected_routes": false,
				"redistribute_static_routes":    false,
				"areas": []map[string]any{{
					"name": "enum-probe", "area_id": areaID, "area_type": areaType,
					"network_ids": []string{lanID},
				}},
				"interfaces": []any{},
			})
			if err != nil {
				t.Fatalf("transport: %v", err)
			}
			m, _ := body.(map[string]any)
			if id, _ := m["_id"].(string); id != "" {
				s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
			}
			return status, m
		}

		for _, v := range OSPFRouterAreasAreaTypeValues {
			if status, body := post(v); status != 200 && status != 201 {
				t.Errorf("area_type %q was refused (HTTP %d): %v", v, status, body["message"])
			}
		}
		if status, _ := post("NORMAL"); status == 200 || status == 201 {
			t.Error("area_type NORMAL was accepted; the controller no longer takes the " +
				"lowercase-only vocabulary this test pins")
		}
	})
}
