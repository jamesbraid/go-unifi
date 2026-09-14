//go:build integration

// unifi/behavior_probe_batch2_integration_test.go
package unifi

import (
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// Batch 2 of the write-contract re-measurement: Hotspot2Conf, WLANGroup,
// ChannelPlan, DynamicDNS, ScheduleTask, DHCPOption. Every one of these is a
// flat v1 REST collection with no branch discriminator, so all six go
// through measureSimpleV1Contract (behavior_probe_shared_integration_test.go).

// TestIntegrationHotspot2ConfWriteContract measures the Hotspot 2.0 config
// (api/s/{site}/rest/hotspot2conf). The struct carries roughly 35 fields;
// this sweeps the 13 in the known-good body used elsewhere in the codebase's
// gateway feature sweep, which is representative rather than exhaustive --
// the untested remainder is nested list-of-struct fields this probe does not
// have a referential fixture for (icons, nai_realm_list, and the rest).
// The four numeric-looking enum fields (network_auth_type, venue_group,
// venue_type, network_type) are seeded away from 0, which blankValue reads
// as empty -- the seeding trap.
func TestIntegrationHotspot2ConfWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	spec := simpleV1Spec{
		resource:   "Hotspot2Conf",
		collection: "hotspot2conf",
		seed: func(name string) map[string]any {
			return map[string]any{
				"name":                    name,
				"hessid":                  "02:00:00:aa:bb:01",
				"domain_name_list":        []any{"example.com"},
				"network_auth_type":       2,
				"network_type":            2,
				"venue_group":             3,
				"venue_type":              5,
				"network_access_internet": true,
				"gas_advanced":            true,
				"disable_dgaf":            true,
				"qos_map_status":          true,
				"metrics_status":          true,
				"osu_ssid":                "probe-osu",
			}
		},
		rename: "name",
		empties: []simpleV1Field{
			{"name", ""},
			{"hessid", ""},
			{"domain_name_list", []any{}},
			{"network_access_internet", false},
			{"network_auth_type", ""},
			{"venue_group", ""},
			{"venue_type", ""},
			{"network_type", ""},
		},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "Hotspot2Conf", "hotspot2conf", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "Hotspot2Conf", "hotspot2conf", contract, dropped, measured)
}

// TestIntegrationWLANGroupWriteContract measures the WLAN group
// (api/s/{site}/rest/wlangroup): a single field, optional on create despite
// its .{1,128} pattern -- that pattern only bites once a value is supplied.
func TestIntegrationWLANGroupWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	spec := simpleV1Spec{
		resource:   "WLANGroup",
		collection: "wlangroup",
		seed: func(name string) map[string]any {
			return map[string]any{"name": name}
		},
		rename:  "name",
		empties: []simpleV1Field{{"name", ""}},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "WLANGroup", "wlangroup", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "WLANGroup", "wlangroup", contract, dropped, measured)
}

// TestIntegrationChannelPlanWriteContract measures the channel plan
// (api/s/{site}/rest/channelplan): date and a radio_table array, both
// optional on create despite carrying no omitempty on the generated struct's
// Date field.
func TestIntegrationChannelPlanWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	spec := simpleV1Spec{
		resource:   "ChannelPlan",
		collection: "channelplan",
		seed: func(name string) map[string]any {
			return map[string]any{
				"date": "2025-01-01T00:00:00Z",
				"radio_table": []any{
					map[string]any{
						"channel": "36", "device_mac": "02:00:00:aa:bb:cc",
						"name": "radio0", "tx_power": "20", "tx_power_mode": "custom", "width": 40,
					},
				},
			}
		},
		rename:  "",
		empties: []simpleV1Field{{"date", ""}},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "ChannelPlan", "channelplan", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "ChannelPlan", "channelplan", contract, dropped, measured)
}

// TestIntegrationDynamicDNSWriteContract measures the dynamic DNS client
// (api/s/{site}/rest/dynamicdns): every field is optional on create, and an
// empty create ({}) is accepted with the controller defaulting interface to
// "wan" -- a controller default, not a required field, so this seeds
// "wan2" instead (the pattern is wan[2-9]?, so "wan" alone is not
// distinguishable as a non-default write).
func TestIntegrationDynamicDNSWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	spec := simpleV1Spec{
		resource:   "DynamicDNS",
		collection: "dynamicdns",
		seed: func(name string) map[string]any {
			return map[string]any{
				"interface": "wan2", "service": "dyndns",
				"host_name": name + ".example.com", "login": "probe-login",
				"x_password": "probe-secret", "server": "members.dyndns.org",
				"options": []any{"opt1=val1"},
			}
		},
		rename: "host_name",
		empties: []simpleV1Field{
			{"interface", ""},
			{"service", ""},
			{"host_name", ""},
			{"login", ""},
			{"x_password", ""},
			{"server", ""},
			{"options", []any{}},
		},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	// A prior finding recorded service=custom as creating fine without
	// custom_service. Measured here: it does not -- rejected outright -- so
	// this is swept both ways (without, then with) rather than trusted, and
	// the branch is recorded in RequiredOnCreateWhen instead of being left
	// as an aside.
	path := "/api/s/" + c.Site + "/rest/dynamicdns"
	customBase := map[string]any{
		"interface": "wan2", "service": "custom", "host_name": "custom-probe.example.com",
		"login": "probe-login", "x_password": "probe-secret",
	}
	tryCustom := func(doc map[string]any) (int, any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport: %v", err)
		}
		if id := objectID(firstData(t, body)); id != "" {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, body
	}
	if status, body := tryCustom(clone(customBase)); status/100 == 2 {
		t.Error("LOUD: service=custom without custom_service was accepted; a prior finding said this " +
			"was already the case, but this session measured it rejected first -- re-check before trusting either")
	} else {
		t.Logf("service=custom without server rejected (HTTP %d, %s, api.err.DynamicDNSCustomServerRequired) "+
			"-- contradicts the prior finding that service=custom creates fine on its own", status, v1ErrCode(body))
	}
	// The rejection names CustomServer, not custom_service -- the field this
	// branch actually needs is server (the custom DDNS server hostname),
	// which every other service value leaves optional.
	withServer := clone(customBase)
	withServer["server"] = "dyn.example.net"
	if status, body := tryCustom(withServer); status/100 != 2 {
		t.Errorf("service=custom with server present was rejected (HTTP %d, %s); expected accepted",
			status, v1ErrCode(body))
	} else {
		t.Logf("service=custom with server present accepted (HTTP %d) -- server, not custom_service, "+
			"is what this branch requires", status)
	}
	withCustomServiceOnly := clone(customBase)
	withCustomServiceOnly["custom_service"] = "probe.dyn.example.net"
	if status, body := tryCustom(withCustomServiceOnly); status/100 == 2 {
		t.Error("LOUD: service=custom with custom_service (but no server) was accepted; expected the " +
			"same CustomServer rejection measured above")
	} else {
		t.Logf("service=custom with custom_service but no server still rejected (HTTP %d, %s) -- "+
			"confirms server, not custom_service, is the required field", status, v1ErrCode(body))
	}

	contract.RequiredOnCreateWhen = map[string][]string{"service=custom": {"server"}}

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "DynamicDNS", "dynamicdns", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "DynamicDNS", "dynamicdns", contract, dropped, measured)
}

// TestIntegrationScheduleTaskWriteContract measures the scheduled task
// (api/s/{site}/rest/scheduletask). action is a fixed single-value field
// ("upgrade" is the only value the schema's pattern admits), so there is
// only one create branch. cron_expr wants a standard 5-field Unix cron, not
// the 6-field Quartz form some other UniFi endpoints accept -- found by
// trying both rather than assumed from either.
func TestIntegrationScheduleTaskWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	spec := simpleV1Spec{
		resource:   "ScheduleTask",
		collection: "scheduletask",
		seed: func(name string) map[string]any {
			return map[string]any{
				"action": "upgrade", "cron_expr": "0 3 * * *", "name": name,
				"execute_only_once": false,
				"upgrade_targets":   []any{map[string]any{"mac": "02:00:00:aa:bb:cc"}},
			}
		},
		rename: "name",
		empties: []simpleV1Field{
			{"cron_expr", ""},
			{"name", ""},
		},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	// The update side re-validates the create-time required pair: a PUT
	// naming only an optional field and omitting action/cron_expr is
	// refused, not merged. Only once both required fields are present does
	// the rest of the document merge on omission -- a two-tier behaviour
	// requiredFieldSweep's flat list cannot express, so it is asserted here
	// rather than folded into empty/omit.
	path := "/api/s/" + c.Site + "/rest/scheduletask"
	body, status, err := s.PostJSON(ctx, path, spec.seed("scheduletask-partial-base"))
	if status == 0 || status/100 != 2 {
		t.Fatalf("seed for the partial-update check was rejected (HTTP %d): %v", status, err)
	}
	id := objectID(firstData(t, body))
	defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	if _, st, _ := s.PutJSON(ctx, path+"/"+id, map[string]any{"name": "renamed-only"}); st/100 == 2 {
		t.Error("a PUT naming only name (omitting action/cron_expr) was accepted; the prior finding's " +
			"re-validate-on-every-write rule no longer holds")
	} else {
		t.Logf("PUT naming only name (omitting action/cron_expr) rejected (HTTP %d) -- required fields "+
			"must be resent on every write, not just create", st)
	}

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "ScheduleTask", "scheduletask", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "ScheduleTask", "scheduletask", contract, dropped, measured)
}

// TestIntegrationDHCPOptionWriteContract measures the DHCP option
// (api/s/{site}/rest/dhcpoption). type is required on both create and
// update -- unusual among this batch, whose OMIT-KEEPS fields all merge --
// and code/type are cross-validated against what looks like an internal
// per-code IANA-option table, recorded as an assertion rather than folded
// into required_on_create since it depends on the code chosen, not on
// whether type is present at all.
func TestIntegrationDHCPOptionWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	spec := simpleV1Spec{
		resource:   "DHCPOption",
		collection: "dhcpoption",
		seed: func(name string) map[string]any {
			return map[string]any{
				"type": "text", "code": 60, "name": "opt-" + name[len(name)-6:],
				"signed": true, "width": 16,
			}
		},
		rename: "name",
		empties: []simpleV1Field{
			{"name", ""},
			{"code", ""},
			{"width", ""},
			{"type", ""},
		},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	// code=70 (SMTP servers) only accepts type=ipaddress; type=text there is
	// refused by the per-code table, not by type's own pattern.
	path := "/api/s/" + c.Site + "/rest/dhcpoption"
	body, status, err := s.PostJSON(ctx, path, map[string]any{"type": "text", "code": 70, "name": "smtp-probe"})
	if status == 0 {
		t.Fatalf("transport: %v", err)
	}
	if id := objectID(firstData(t, body)); id != "" {
		s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
	}
	if status/100 == 2 {
		t.Error("code=70 with type=text was accepted; the per-code type table finding no longer holds")
	} else {
		t.Logf("code=70 (SMTP servers) with type=text rejected (HTTP %d, %s) -- a code-specific type "+
			"table, not a general type-format rejection", status, v1ErrCode(body))
	}

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "DHCPOption", "dhcpoption", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "DHCPOption", "dhcpoption", contract, dropped, measured)
}
