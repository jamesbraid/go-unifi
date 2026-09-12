//go:build integration

package unifi

import (
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
	ctx, c, s := controllertest.MutatingHarness(t, 20*time.Minute)

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

// TestIntegrationV2NatIidFormat pins the accepted format of a NAT filter's
// iid, which no enum oracle can name: the controller validates it in code.
// The measured rule (10.6.101) is two plain IPv6 addresses joined by "/" --
// an interface-identifier/mask pair in ip6tables' address syntax; anything
// else, blank included, draws api.err.NatRuleInvalidIidAndPortFilter. Every
// earlier attempt at this field was rejected before storage, so the accepted
// case is also the proof the write path exists: the value must survive a
// create and read back verbatim.
func TestIntegrationV2NatIidFormat(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 20*time.Minute)

	wanID := ensureWANNetwork(ctx, t, s, c.Site)
	if wanID == "" {
		t.Fatal("no WAN network; every NAT create would fail for the wrong reason")
	}
	path := "/v2/api/site/" + c.Site + "/nat"

	post := func(iid any) (int, string, map[string]any) {
		srcFilter := map[string]any{
			"filter_type": "IID_AND_PORT", "firewall_group_ids": []string{},
			"invert_address": false, "invert_port": false,
		}
		if iid != nil {
			srcFilter["iid"] = iid
		}
		body, status, err := s.PostJSON(ctx, path, map[string]any{
			"enabled": true, "type": "MASQUERADE", "ip_version": "IPV6",
			"protocol": "all", "out_interface": wanID,
			"source_filter": srcFilter,
			"destination_filter": map[string]any{
				"filter_type": "NONE", "firewall_group_ids": []string{},
				"invert_address": false, "invert_port": false,
			},
		})
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		m, _ := body.(map[string]any)
		id, _ := m["_id"].(string)
		return status, id, m
	}

	const accepted = "::1:2:3:4/::ffff:ffff:ffff:ffff"
	status, id, body := post(accepted)
	if status != 200 && status != 201 {
		t.Fatalf("iid %q was refused (HTTP %d, %s): %v\n\nThe v6/v6 pair is the one format the "+
			"10.6.101 jar accepts; either the rule changed or this probe's base body no longer "+
			"reaches the iid validation.", accepted, status, v2ErrCode(body), body["message"])
	}
	if id == "" {
		t.Errorf("created NAT rule carries no id; it cannot be deleted: %v", body)
	} else {
		stored, getStatus, err := s.GetJSON(ctx, path+"/"+id)
		if err != nil || getStatus != 200 {
			t.Errorf("read back the created rule: HTTP %d, %v", getStatus, err)
		} else if f, _ := firstData(t, stored)["source_filter"].(map[string]any); f["iid"] != accepted {
			t.Errorf("iid stored as %v, wrote %q; the controller rewrote it in silence", f["iid"], accepted)
		}
		s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	}

	for _, tc := range []struct {
		name string
		iid  any
	}{
		{"a bare address with no mask", "::1:2:3:4"},
		{"a prefix length instead of a mask", "::1:2:3:4/64"},
		{"three parts", "::1/::2/::3"},
		{"no iid at all", nil},
	} {
		status, id, m := post(tc.iid)
		if status == 200 || status == 201 {
			if id != "" {
				s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
			}
			t.Errorf("%s (%v) was accepted; the iid format this test pins has loosened", tc.name, tc.iid)
			continue
		}
		if code := v2ErrCode(m); code != "api.err.NatRuleInvalidIidAndPortFilter" {
			t.Errorf("%s (%v) was refused with %q, wanted api.err.NatRuleInvalidIidAndPortFilter; "+
				"a different refusal may be about something other than the iid", tc.name, tc.iid, code)
		}
	}
}
