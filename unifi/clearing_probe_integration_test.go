//go:build integration

// unifi/clearing_probe_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationClearingSemantics answers the question the empty-string rule
// rests on: for a given field, does the controller treat an empty string and
// an absent key the same way?
//
// The encoder drops a field whose schema pattern refuses "" and keeps one
// whose pattern permits it, on the reasoning that "" is how a caller clears
// such a field and that dropping an empty field is never worse than sending
// it. That second half only holds if PUT is a full replace, so that omitting
// a key clears the stored value.
//
// It is not. Measured on 10.6.101, the v1 rest PUT merges: an omitted key
// leaves the stored value alone on all four collections below, save two
// fields of the DHCP guard pairing that carry their own rule. A caller
// meaning to clear a field by dropping it gets a 200 and no change.
//
// Rather than guess a valid value for every field, this seeds each resource
// and then asks the controller which fields it stored a non-empty string
// for. Those are the only ones with anything to clear. For each, it PUTs the
// stored document twice: once with the field emptied, once with the key
// removed, resetting in between.
//
//	EMPTY-REJECTED     the controller refuses "" -- dropping it loses nothing
//	EMPTY-CLEARS       "" is how the field is cleared
//	EMPTY-IGNORED      "" is accepted and the old value survives
//	OMIT-CLEARS        leaving the key out clears it (PUT replaces)
//	OMIT-KEEPS         leaving the key out preserves it (PUT merges)
//	*-REPLACED-default the write landed on the controller's own default
//
// Two things make those verdicts trustworthy, and both were once missing.
//
// Every verdict comes from a GET of the stored document. A v1 PUT that
// changed nothing answers 200 with an empty data array, so a probe reading
// the field out of the write's own response sees "" for a field the
// controller did not touch. That is how this probe recorded OMIT-CLEARS for
// 39 of the 42 fields it sweeps while the controller was preserving every
// one of them.
//
// And no field is measured from the value the controller would have chosen
// for it anyway. "The value survived" and "the controller put its own
// default back" are the same observation when the two coincide, so the sweep
// creates the smallest object each collection accepts, reads its defaults,
// and moves any colliding field onto a value from nonDefault first. That is
// not bookkeeping: wlanconf.usergroup_id reads EMPTY-IGNORED from the site's
// default user group and EMPTY-REPLACED-default from any other, because ""
// silently reassigns the WLAN to the default group.
func TestIntegrationClearingSemantics(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)

	setSiteRadiusEnabled(ctx, t, s, c.Site, true)

	// The artifact is the cross-run memory: without BEHAVIOR_WRITE the
	// measured verdicts are checked against it, with BEHAVIOR_WRITE=1 they
	// are recorded into it. measured accumulates across the sequential
	// subtests so the artifact is written once, after all resources ran.
	root, captured, running := behaviorGate(ctx, t, s, c.Site)
	recording := behaviorWriteRequested()
	artifact, artifactFound, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	if artifactFound && artifact.ControllerVersion != running {
		// Comparing two controllers' measurements would file a version
		// difference as drift, so the artifact stops being this run's
		// baseline; the in-file assertions below still hold.
		t.Logf("artifact was measured on %s, this controller reports %s; skipping the artifact comparison",
			artifact.ControllerVersion, running)
		artifactFound = false
	}
	measured := map[string]map[string]behavior.EmptySemantics{}

	for _, res := range clearingProbeResources(t, ctx, s, c.Site) {
		t.Run(res.path, func(t *testing.T) {
			collection := "/api/s/" + c.Site + "/rest/" + res.path

			body, status, err := s.PostJSON(ctx, collection, res.seed)
			mustTransport(t, err)
			if status != 200 {
				t.Fatalf("seed for %s rejected (HTTP %d): %v", res.path, status, body)
			}
			stored := firstData(t, body)
			id, _ := stored["_id"].(string)
			if id == "" {
				t.Fatalf("seeded %s has no id", res.path)
			}
			defer s.DeleteJSON(ctx, collection+"/"+id) //nolint:errcheck

			defaults, bareID := clearingProbeDefaults(ctx, t, s, collection, res)
			if bareID != "" {
				defer s.DeleteJSON(ctx, collection+"/"+bareID) //nolint:errcheck
			}

			put := func(doc map[string]any) int {
				_, status, err := s.PutJSON(ctx, collection+"/"+id, doc)
				mustTransport(t, err)
				return status
			}

			// The instrument. Every verdict is read from here rather than
			// from the write's own response, and a re-read that does not
			// come back stops the run: a GET answered with nothing looks
			// exactly like a cleared field to the classifier, which is the
			// shape of the bug this replaced.
			read := func() map[string]any {
				body, status, err := s.GetJSON(ctx, collection+"/"+id)
				mustTransport(t, err)
				doc := firstData(t, body)
				if status != 200 || doc == nil {
					t.Fatalf("re-reading %s/%s answered HTTP %d with %v; a verdict cannot be "+
						"taken from a document that did not come back", res.path, id, status, body)
				}
				return doc
			}

			var fields []string
			for k, v := range stored {
				str, ok := v.(string)
				if !ok || str == "" || clearingProbeSkip[k] {
					continue
				}
				fields = append(fields, k)
			}
			sort.Strings(fields)

			var summary []string
			for _, field := range fields {
				original, _ := stored[field].(string)

				baseline := stored
				if def, ok := defaults[field]; ok && jsonEqual(def, original) {
					baseline = clearingProbeStrengthen(t, res, field, stored, put, read)
				}

				sem := storedEmptySemantics(t, field, "", defaults[field], baseline, put, read)

				// Back to the seeded document before the next field, and
				// checked. Under a merging PUT every verdict is measured
				// against whatever the previous field left behind, so a
				// reset that quietly failed would be attributed to the
				// wrong field.
				put(clone(stored))
				if got := read()[field]; !jsonEqual(got, original) {
					t.Errorf("%s.%s: the seeded document did not restore -- %v is stored where %q "+
						"was seeded, so the verdicts after this one are measured from the wrong "+
						"baseline", res.path, field, got, original)
				}

				summary = append(summary, fmt.Sprintf("%-34s %-16s %s", field, sem.Empty, sem.Omit))

				if measured[res.path] == nil {
					measured[res.path] = map[string]behavior.EmptySemantics{}
				}
				measured[res.path][field] = sem
				if artifactFound && !recording {
					if prev, ok := artifact.Empty[res.path][field]; ok && prev != sem {
						t.Errorf("%s.%s: measured empty=%s omit=%s but %s records empty=%s omit=%s; "+
							"re-measure with BEHAVIOR_WRITE=1 once the change is understood",
							res.path, field, sem.Empty, sem.Omit, behavior.Path, prev.Empty, prev.Omit)
					}
				}
			}

			t.Logf("clearing semantics for %s (%d fields):\n  %s", res.path, len(summary), strings.Join(summary, "\n  "))
		})
	}

	if recording {
		// Per-field upsert, not a section replace: only fields the seed
		// populated with a non-empty string get measured on any given run,
		// so replacing a resource's whole map would erase real measurements
		// of fields this run happened not to reach.
		mergeBehaviorArtifact(t, root, captured, func(a *behavior.Artifact) {
			if a.Empty == nil {
				a.Empty = map[string]map[string]behavior.EmptySemantics{}
			}
			for res, byField := range measured {
				if a.Empty[res] == nil {
					a.Empty[res] = map[string]behavior.EmptySemantics{}
				}
				for field, sem := range byField {
					a.Empty[res][field] = sem
				}
			}
		})
		total := 0
		for _, byField := range measured {
			total += len(byField)
		}
		t.Logf("recorded empty semantics for %d fields across %d resources into %s",
			total, len(measured), behavior.Path)
	}
}

// clearingProbeDefaults creates the smallest object the collection accepts and
// returns what came back, which is the controller's own default for every
// field it filled in unasked. The caller deletes the object by the id
// returned alongside.
//
// The sweep needs this to know which of its verdicts it is not entitled to.
// A field seeded with the same value the controller defaults to cannot tell
// OMIT-KEEPS from a reset, and eleven of the forty-two fields swept here are
// in that position -- every *_setting_preference, both network references,
// pmf_mode, wlan_band and the rest.
func clearingProbeDefaults(
	ctx context.Context,
	t *testing.T,
	s *controllertest.Session,
	collection string,
	res clearingProbeResource,
) (map[string]any, string) {
	t.Helper()

	body, status, err := s.PostJSON(ctx, collection, res.bare)
	mustTransport(t, err)
	doc := firstData(t, body)
	id, _ := doc["_id"].(string)
	if status != 200 || id == "" {
		t.Fatalf("the bare %s create was refused (HTTP %d): %v\n\nWithout it the sweep cannot tell "+
			"a value the controller preserved from one it reset to its own default.",
			res.path, status, body)
	}
	// The envelope keys are per-object, not defaults, and nothing probes
	// them; leaving them in would flag a collision that does not exist.
	delete(doc, "_id")
	delete(doc, "site_id")
	return doc, id
}

// clearingProbeStrengthen moves one field off the value the controller would
// have chosen for it anyway and returns the document to measure against, so
// that the verdict which follows cannot be explained by a reset to the
// default. It returns the seeded document unchanged when it cannot, having
// said why.
//
// This is the assertion that replaced the one this probe used to carry. That
// one held every field to OMIT-CLEARS, which the controller has never done;
// it survived because the broken instrument reported OMIT-CLEARS for
// everything. There is no behaviour left to assert globally -- the artifact
// comparison above is what catches a controller changing its mind, field by
// field -- but a measurement the probe knows it cannot make is still worth
// failing on.
func clearingProbeStrengthen(
	t *testing.T,
	res clearingProbeResource,
	field string,
	stored map[string]any,
	put func(map[string]any) int,
	read func() map[string]any,
) map[string]any {
	t.Helper()

	key := res.path + "." + field
	alt, ok := res.nonDefault[field]
	if !ok {
		if clearingProbeDefaultBound[key] == "" {
			t.Errorf("%s: the seed's value is the one the controller defaults this field to, so a "+
				"preserved value and a reset to that default are the same observation. Give the "+
				"field an entry in nonDefault, or record in clearingProbeDefaultBound what the "+
				"controller did when another value was asked for.", key)
		}
		return stored
	}

	doc := clone(stored)
	doc[field] = alt
	if status := put(doc); status/100 != 2 {
		put(clone(stored))
		t.Errorf("%s: the controller refused %v (HTTP %d), so the verdict below cannot tell a "+
			"preserved value from a reset to the default", key, alt, status)
		return stored
	}
	baseline := read()
	if !jsonEqual(baseline[field], alt) {
		put(clone(stored))
		t.Errorf("%s: asked for %v and the controller stored %v, so the verdict below cannot tell "+
			"a preserved value from a reset to the default", key, alt, baseline[field])
		return stored
	}
	return baseline
}

// clearingProbeDefaultBound lists probed fields this controller stores its own
// default for whatever is asked, keyed resource.field. Their verdicts cannot
// distinguish a preserved value from a reset, and no seed can fix that, so the
// measurement behind each exemption is recorded here instead.
var clearingProbeDefaultBound = map[string]string{
	// The field document allows all|native|customize|disabled and the
	// controller accepts every one of them with a 200, on create and on
	// update alike -- and stores "all" each time. Measured on 10.6.101.
	"portconf.forward": `every value but "all" is accepted and discarded`,

	// Same shape: all|groups|devices, "groups" and "devices" both answer
	// 200 and both store "all". A WLAN's AP selection lives in
	// ap_group_ids, which is a list and not swept here.
	"wlanconf.ap_group_mode": `every value but "all" is accepted and discarded`,
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
	// bare is the smallest body the collection accepts. Whatever comes back
	// beyond it is the controller's default.
	bare map[string]any
	// nonDefault gives a field whose seeded value collides with that default
	// a value the controller would not have chosen, so the verdict measured
	// from it means what it says.
	nonDefault map[string]any
}

// clearingProbeResources returns one richly-populated seed per resource under
// test, so there is something for the probe to try clearing.
func clearingProbeResources(t *testing.T, ctx context.Context, s *controllertest.Session, site string) []clearingProbeResource {
	t.Helper()

	// Resolved before the alternates below exist: the stock user group is
	// whichever one the site shipped with, and asking after a second has
	// been created would be a coin toss between them.
	usergroup := firstObjectID(ctx, t, s, site, "usergroup")
	apGroup := requiredAPGroupID(ctx, t, s, site)

	// Two objects nothing else on the site points at. The id-valued fields
	// below all default to the site's own LAN and user group, so these are
	// what lets them be measured from somewhere else.
	altNetwork := clearingProbeAltNetwork(ctx, t, s, site)
	altUsergroup := clearingProbeAltUsergroup(ctx, t, s, site)

	return []clearingProbeResource{
		{
			// A free-trial package: the controller's sanitizer refuses a
			// package carrying both duration fields and refuses one
			// carrying neither, so trial_duration_minutes has to be here
			// and hours must not be. Neither is a string, so the sweep
			// below leaves both alone and the reset write stays valid.
			path: "hotspotpackage",
			seed: map[string]any{
				"name": "clear-package", "charged_as": "hour", "currency": "USD",
				"trial_duration_minutes": 60, "trial_reset": 24,
				"limit_overwrite": true, "limit_up": 1024, "limit_down": 2048,
			},
			// The same sanitizer applies to the bare create, so the
			// duration stays; the controller defaults nothing else.
			bare: map[string]any{"name": "bare-package", "trial_duration_minutes": 60},
		},
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
			// A VLAN needs a subnet and a tag, so the bare network carries
			// its own -- different from the seed's, or ip_subnet would look
			// like a defaulted field.
			bare: map[string]any{
				"name": "bare-net", "purpose": PurposeCorporate, "enabled": true,
				"ip_subnet": "10.94.11.1/24", "vlan_enabled": true, "vlan": 941,
			},
			nonDefault: map[string]any{
				"setting_preference":      "auto",
				"ipv6_setting_preference": "auto",
				"networkgroup":            "LAN2",
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
			bare: map[string]any{"name": "bare-port"},
			nonDefault: map[string]any{
				"setting_preference":    "auto",
				"native_networkconf_id": altNetwork,
			},
		},
		{
			path: "wlanconf",
			seed: map[string]any{
				"name": "clear-wlan", "enabled": true,
				"security": "wpapsk", "x_passphrase": "probe-passphrase",
				"wpa_mode": "wpa2", "wpa_enc": "ccmp",
				// The stock usergroup: a WLAN must reference one before the
				// controller will create it. wlangroup_id used to be seeded
				// beside it on the same belief; 10.6.101 creates the WLAN
				// without it, and drops the key when it is sent.
				"usergroup_id": usergroup,
				"ap_group_ids": []string{apGroup},
			},
			// No security here. The controller does not default it, and a
			// bare create that carried one would teach the sweep a default
			// the controller never chose.
			bare: map[string]any{
				"name":         "bare-wlan",
				"ap_group_ids": []string{apGroup},
			},
			nonDefault: map[string]any{
				"setting_preference":         "auto",
				"minrate_setting_preference": "auto",
				"pmf_mode":                   "optional",
				"wlan_band":                  "5g",
				"networkconf_id":             altNetwork,
				"usergroup_id":               altUsergroup,
			},
		},
	}
}

// clearingProbeAltNetwork creates a corporate network nothing else on the site
// references, so the fields that default to the site's own LAN can be measured
// from a network the controller would not have chosen.
func clearingProbeAltNetwork(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	path := "/api/s/" + site + "/rest/networkconf"
	body, status, err := s.PostJSON(ctx, path, map[string]any{
		"name": "clear-alt-net", "purpose": PurposeCorporate, "enabled": true,
		"ip_subnet": "10.94.99.1/24", "vlan_enabled": true, "vlan": 949,
	})
	mustTransport(t, err)
	id, _ := firstData(t, body)["_id"].(string)
	if status != 200 || id == "" {
		t.Fatalf("the alternate network was refused (HTTP %d): %v", status, body)
	}
	t.Cleanup(func() {
		s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	})
	return id
}

// clearingProbeAltUsergroup is clearingProbeAltNetwork for usergroup_id, which
// defaults to the group every site ships.
func clearingProbeAltUsergroup(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	path := "/api/s/" + site + "/rest/usergroup"
	body, status, err := s.PostJSON(ctx, path, map[string]any{
		"name": "clear-alt-group", "qos_rate_max_down": -1, "qos_rate_max_up": -1,
	})
	mustTransport(t, err)
	id, _ := firstData(t, body)["_id"].(string)
	if status != 200 || id == "" {
		t.Fatalf("the alternate user group was refused (HTTP %d): %v", status, body)
	}
	t.Cleanup(func() {
		s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	})
	return id
}

// requiredAPGroupID is firstAPGroupID for a caller that cannot continue
// without one: creating a WLAN with no AP group is api.err.ApGroupMissing, so
// a sweep that carried on would measure that rejection instead of the field it
// came to measure. firstAPGroupID has already logged the status it got.
func requiredAPGroupID(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	id := firstAPGroupID(ctx, t, s, site)
	if id == "" {
		t.Fatal("no ap groups on this site; a WLAN cannot be created without one")
	}
	return id
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
