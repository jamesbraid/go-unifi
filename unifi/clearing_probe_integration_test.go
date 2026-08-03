//go:build integration

// unifi/clearing_probe_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"maps"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationClearingSemantics answers the question the empty-string rule
// rests on and nobody had measured: for a given field, does the controller
// treat an empty string and an absent key the same way?
//
// The encoder drops a field whose schema pattern refuses "" and keeps one
// whose pattern permits it, on the reasoning that "" is how a caller clears
// such a field. That reasoning assumes PUT is a merge -- that omitting a key
// leaves the stored value alone. If PUT is a full replace, omitting clears
// too, the two are equivalent, and several justifications in
// network_encode_empty_test.go are unnecessary.
//
// Rather than guess a valid value for every field, this seeds each resource
// and then asks the controller which fields it stored a non-empty string
// for. Those are the only ones with anything to clear. For each, it PUTs the
// stored document twice: once with the field emptied, once with the key
// removed, resetting in between.
//
//	EMPTY-REJECTED   the controller refuses "" -- dropping it loses nothing
//	EMPTY-CLEARS     "" is how the field is cleared
//	EMPTY-IGNORED    "" is accepted and the old value survives
//	OMIT-CLEARS      leaving the key out clears it (PUT replaces)
//	OMIT-KEEPS       leaving the key out preserves it (PUT merges)
func TestIntegrationClearingSemantics(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	setSiteRadiusEnabled(ctx, t, s, c.Site, true)

	for _, res := range clearingProbeResources(t, ctx, s, c.Site) {
		t.Run(res.path, func(t *testing.T) {
			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/"+res.path, res.seed)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}
			if status != 200 {
				t.Fatalf("seed for %s rejected (HTTP %d): %v", res.path, status, body)
			}
			stored := firstData(t, body)
			id, _ := stored["_id"].(string)
			if id == "" {
				t.Fatalf("seeded %s has no id", res.path)
			}
			defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/"+res.path+"/"+id) //nolint:errcheck

			var fields []string
			for k, v := range stored {
				str, ok := v.(string)
				if !ok || str == "" || clearingProbeSkip[k] {
					continue
				}
				fields = append(fields, k)
			}
			sort.Strings(fields)

			put := func(doc map[string]any) (map[string]any, int) {
				body, status, err := s.PutJSON(ctx, "/api/s/"+c.Site+"/rest/"+res.path+"/"+id, doc)
				if err != nil {
					t.Fatalf("transport: %v", err)
				}
				if status != 200 {
					return nil, status
				}
				return firstData(t, body), status
			}

			var summary []string
			for _, field := range fields {
				original, _ := stored[field].(string)

				// Empty string.
				doc := maps.Clone(stored)
				doc[field] = ""
				after, status := put(doc)
				empty := "EMPTY-REJECTED"
				if status == 200 {
					if got, _ := after[field].(string); got == "" {
						empty = "EMPTY-CLEARS"
					} else if got == original {
						empty = "EMPTY-IGNORED"
					} else {
						empty = "EMPTY-REPLACED-" + got
					}
				}

				put(maps.Clone(stored)) // reset

				// Absent key.
				doc = maps.Clone(stored)
				delete(doc, field)
				after, status = put(doc)
				omit := "OMIT-REJECTED"
				if status == 200 {
					if got, _ := after[field].(string); got == "" {
						omit = "OMIT-CLEARS"
					} else if got == original {
						omit = "OMIT-KEEPS"
					} else {
						omit = "OMIT-REPLACED-" + got
					}
				}

				put(maps.Clone(stored)) // reset

				summary = append(summary, fmt.Sprintf("%-34s %-16s %s", field, empty, omit))

				// The finding the encoder rule rests on: leaving a key out
				// clears it, so dropping an empty field is never worse than
				// sending it. If a controller ever merges instead, the rule
				// needs revisiting and this is where that shows up.
				if omit != "OMIT-CLEARS" && !clearingOmitExceptions[field] {
					t.Errorf("%s.%s: omitting the key gave %s, not OMIT-CLEARS; "+
						"the empty-string rule assumes PUT replaces", res.path, field, omit)
				}
			}

			t.Logf("clearing semantics for %s (%d fields):\n  %s", res.path, len(summary), strings.Join(summary, "\n  "))
		})
	}
}

// clearingOmitExceptions lists fields where leaving the key out is not simply
// a clear. Each needs a reason.
var clearingOmitExceptions = map[string]bool{
	// dhcpguard_enabled requires a trusted server address, so removing
	// dhcpd_ip_1 while the guard is on is api.err.MissingIPAddress rather
	// than a clear. Same pairing networkCrossFieldRules records.
	"dhcpd_ip_1": true,
}

// clearingProbeSkip lists keys that carry identity or dispatch rather than
// configuration. Clearing them measures nothing useful: purpose selects which
// encoder branch runs at all, and the envelope keys are the controller's.
var clearingProbeSkip = map[string]bool{
	"_id": true, "site_id": true, "key": true, "purpose": true,
	"attr_hidden": true, "attr_hidden_id": true, "attr_no_delete": true, "attr_no_edit": true,
}

type clearingProbeResource struct {
	path string
	seed map[string]any
}

// clearingProbeResources returns one richly-populated seed per resource under
// test, so there is something for the probe to try clearing.
func clearingProbeResources(t *testing.T, ctx context.Context, s *controllertest.Session, site string) []clearingProbeResource {
	t.Helper()

	return []clearingProbeResource{
		{
			path: "networkconf",
			seed: map[string]any{
				"name": "clear-net", "purpose": PurposeCorporate, "enabled": true,
				"ip_subnet": "10.94.10.1/24", "vlan_enabled": true, "vlan": 940,
				"setting_preference": "manual", "networkgroup": "LAN",
				"domain_name": "clear.example", "igmp_snooping": true,
				"dhcpd_enabled": true, "dhcpd_start": "10.94.10.6", "dhcpd_stop": "10.94.10.254",
				"dhcpd_dns_enabled": true, "dhcpd_dns_1": "10.94.10.53", "dhcpd_dns_3": "10.94.10.54",
				"dhcpd_gateway_enabled": true, "dhcpd_gateway": "10.94.10.1",
				"dhcpd_ntp_enabled": true, "dhcpd_ntp_1": "10.94.10.123",
				"dhcpd_boot_enabled": true, "dhcpd_boot_server": "10.94.10.9",
				"dhcpd_boot_filename": "pxelinux.0",
				"dhcpguard_enabled":   true, "dhcpd_ip_1": "10.94.10.2",
				"dhcpd_mac_1":          "00:11:22:33:44:55",
				"mac_override_enabled": true, "mac_override": "00:11:22:33:44:66",
			},
		},
		{
			path: "portconf",
			seed: map[string]any{
				"name": "clear-port", "forward": "all",
				"poe_mode": "auto", "op_mode": "switch",
				"stormctrl_type": "level", "stormctrl_bcast_enabled": true,
				"stormctrl_bcast_level": 50,
			},
		},
		{
			path: "wlanconf",
			seed: map[string]any{
				"name": "clear-wlan", "enabled": true,
				"security": "wpapsk", "x_passphrase": "probe-passphrase",
				"wpa_mode": "wpa2", "wpa_enc": "ccmp",
				"usergroup_id": firstUserGroupID(ctx, t, s, site),
				"wlangroup_id": firstWLANGroupID(ctx, t, s, site),
				"ap_group_ids": []string{requiredAPGroupID(ctx, t, s, site)},
			},
		},
	}
}

// firstUserGroupID and firstWLANGroupID resolve the stock objects a WLAN must
// reference before the controller will create it.
func firstUserGroupID(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	return firstObjectID(ctx, t, s, site, "usergroup")
}

func firstWLANGroupID(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	return firstObjectID(ctx, t, s, site, "wlangroup")
}

// requiredAPGroupID resolves the site's default AP group. Creating a WLAN
// without one is api.err.ApGroupMissing. It is a v2 endpoint returning a bare
// array rather than the v1 data envelope.
func requiredAPGroupID(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/v2/api/site/"+site+"/apgroups")
	if err != nil || status != 200 {
		t.Fatalf("list apgroups: status %d, %v", status, err)
	}
	items, _ := body.([]any)
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			if id, _ := obj["_id"].(string); id != "" {
				return id
			}
		}
	}
	t.Fatalf("no ap groups on this site; body %v", body)
	return ""
}

func firstObjectID(ctx context.Context, t *testing.T, s *controllertest.Session, site, collection string) string {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/rest/"+collection)
	if err != nil || status != 200 {
		t.Fatalf("list %s: status %d, %v", collection, status, err)
	}
	m, _ := body.(map[string]any)
	items, _ := m["data"].([]any)
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			if id, _ := obj["_id"].(string); id != "" {
				return id
			}
		}
	}
	t.Fatalf("no %s objects on this site", collection)
	return ""
}
