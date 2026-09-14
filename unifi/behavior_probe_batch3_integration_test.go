//go:build integration

// unifi/behavior_probe_batch3_integration_test.go
package unifi

import (
	"context"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/internal/probe"
)

// Batch 3 of the write-contract re-measurement: RADIUSProfile, SpatialRecord,
// FirewallRule, NetworkMembersGroup, Account, Routing. These carry branch
// discriminators or nested required fields the flat simpleV1Spec shape
// cannot express, so (FirewallRule aside) each is written by hand.

// TestIntegrationRADIUSProfileWriteContract measures the RADIUS profile
// (api/s/{site}/rest/radiusprofile). None of the 17 top-level fields are
// required on create -- a profile with both usg flags false, both server
// lists absent, and tls_enabled true with no cert material all create fine,
// which this asserts explicitly rather than leaving as an inference from the
// naming. required-on-create only bites two of the four fields on a server
// list ITEM, and only once an item is present at all: ip and x_secret,
// dotted per the artifact's nested-field convention.
func TestIntegrationRADIUSProfileWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/api/s/" + c.Site + "/rest/radiusprofile"

	base := func(name string) map[string]any {
		return map[string]any{
			"name": name,
			"auth_servers": []any{
				map[string]any{"ip": "192.0.2.10", "x_secret": "probe-secret-1", "port": 1812},
			},
			"acct_servers": []any{
				map[string]any{"ip": "192.0.2.11", "x_secret": "probe-secret-2", "port": 1813},
			},
			"vlan_wlan_mode":                "optional",
			"interim_update_interval":       600,
			"x_client_crt":                  "probe-cert-body",
			"x_client_crt_filename":         "probe.crt",
			"x_client_private_key":          "probe-key-body",
			"x_client_private_key_filename": "probe.key",
			"x_client_private_key_password": "probe-pass",
			"x_ca_crts":                     []any{map[string]any{"x_ca_crt": "probe-ca-body", "filename": "ca.pem"}},
		}
	}
	post := func(doc map[string]any) (int, any, map[string]any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		created := firstData(t, body)
		if id := objectID(created); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, body, created
	}

	asked := base("radiusprofile-probe")
	body, status, err := s.PostJSON(ctx, path, asked)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		t.Fatalf("the known-good RADIUSProfile body was rejected (HTTP %d): %v", status, firstData(t, body))
	}
	id := objectID(firstData(t, body))
	if id == "" {
		t.Fatalf("the created RADIUSProfile carries no id: %v", body)
	}
	defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck

	read := func() map[string]any {
		got, status, err := s.GetJSON(ctx, path+"/"+id)
		if err != nil || status != 200 {
			t.Fatalf("GET %s/%s answered HTTP %d (%v)", path, id, status, err)
		}
		return firstData(t, got)
	}
	stored := read()

	dropped := []string{}
	for _, r := range probe.Classify(asked, stored) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("RADIUSProfile: DROPPED %-32s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Logf("RADIUSProfile: CHANGED %-32s (%s)", r.Wire, r.Detail)
		}
	}

	unknownKeyObserve(ctx, t, s, path, base("radiusprofile-unknown-key"))
	if _, listStatus, err := s.GetJSON(ctx, path); listStatus != 200 {
		t.Errorf("GET %s answered HTTP %d (%v)", path, listStatus, err)
	}

	renamed := clone(stored)
	renamed["name"] = "radiusprofile-probe-renamed"
	updateVerb, updatePath := "", ""
	if after, putStatus, err := s.PutJSON(ctx, path+"/"+id, renamed); putStatus/100 != 2 {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v)", path, id, putStatus, after, err)
	} else if got, _ := read()["name"].(string); got != "radiusprofile-probe-renamed" {
		t.Errorf("PUT %s/{id} answered HTTP %d but name still reads %q", path, putStatus, got)
	} else {
		updateVerb, updatePath = "PUT", "api/s/{site}/rest/radiusprofile/{id}"
	}
	s.PutJSON(ctx, path+"/"+id, clone(stored)) //nolint:errcheck // restore

	required := requiredFieldSweep(t, sortedWireNames(asked), func(field string) (int, any) {
		doc := base("radiusprofile-req-" + field)
		delete(doc, field)
		status, body, _ := post(doc)
		return status, body
	})

	// use_usg_auth_server/use_usg_acct_server and tls_enabled all look like
	// discriminators (RADIUS-servers-vs-USG's-own, cert-vs-no-cert). Checked
	// explicitly and refuted: none of them force a companion field.
	if status, body, _ := post(map[string]any{
		"name": "radiusprofile-no-servers", "use_usg_auth_server": false, "use_usg_acct_server": false,
	}); status/100 != 2 {
		t.Errorf("no server lists + both usg flags false was rejected (HTTP %d, %s); expected accepted",
			status, v1ErrCode(body))
	} else {
		t.Logf("no server lists + both usg flags false accepted (HTTP %d) -- no cross-field requirement", status)
	}
	if status, body, _ := post(map[string]any{
		"name": "radiusprofile-tls-no-cert", "tls_enabled": true,
	}); status/100 != 2 {
		t.Errorf("tls_enabled=true with no cert material was rejected (HTTP %d, %s); expected accepted",
			status, v1ErrCode(body))
	} else {
		t.Logf("tls_enabled=true with no cert material accepted (HTTP %d) -- no cross-field requirement", status)
	}

	// Nested requiredness: ip and x_secret bite once a server-list item
	// exists; port does not. Both server lists are swept the same way.
	nestedRequired := []string{}
	nestedEmpty := map[string]behavior.EmptySemantics{}
	for _, server := range []string{"auth_servers", "acct_servers"} {
		item := func() map[string]any {
			return map[string]any{"ip": "192.0.2.20", "x_secret": "probe-nested-secret"}
		}
		for _, field := range []string{"ip", "x_secret", "port"} {
			doc := map[string]any{"name": "radiusprofile-" + server + "-" + field}
			blankItem := item()
			delete(blankItem, field)
			doc[server] = []any{blankItem}
			status, body, _ := post(doc)
			key := server + "." + field
			if status/100 == 2 {
				t.Logf("%s create with an item missing %-10s accepted (HTTP %d) -- not required", server, field, status)
				continue
			}
			t.Logf("%s create with an item missing %-10s rejected (HTTP %d, %s) -- required on create",
				server, field, status, v1ErrCode(body))
			if field != "port" {
				nestedRequired = append(nestedRequired, key)
				nestedEmpty[key] = behavior.EmptySemantics{Empty: "EMPTY-REJECTED", Omit: "OMIT-REJECTED"}
			}
		}
	}

	put := func(doc map[string]any) int {
		_, status, err := s.PutJSON(ctx, path+"/"+id, doc)
		if status == 0 {
			t.Fatalf("transport to %s/%s: %v", path, id, err)
		}
		return status
	}
	measured := map[string]behavior.EmptySemantics{
		"name":                          storedEmptySemantics(t, "name", "", nil, stored, put, read),
		"auth_servers":                  storedEmptySemantics(t, "auth_servers", []any{}, nil, stored, put, read),
		"acct_servers":                  storedEmptySemantics(t, "acct_servers", []any{}, nil, stored, put, read),
		"x_client_crt":                  storedEmptySemantics(t, "x_client_crt", "", nil, stored, put, read),
		"x_client_crt_filename":         storedEmptySemantics(t, "x_client_crt_filename", "", nil, stored, put, read),
		"x_client_private_key":          storedEmptySemantics(t, "x_client_private_key", "", nil, stored, put, read),
		"x_client_private_key_filename": storedEmptySemantics(t, "x_client_private_key_filename", "", nil, stored, put, read),
		"x_client_private_key_password": storedEmptySemantics(t, "x_client_private_key_password", "", nil, stored, put, read),
		"vlan_wlan_mode":                storedEmptySemantics(t, "vlan_wlan_mode", "", nil, stored, put, read),
		"interim_update_interval":       storedEmptySemantics(t, "interim_update_interval", "", nil, stored, put, read),
		"x_ca_crts":                     storedEmptySemantics(t, "x_ca_crts", []any{}, nil, stored, put, read),
	}
	for k, v := range nestedEmpty {
		measured[k] = v
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "api/s/{site}/rest/radiusprofile",
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: append(required, nestedRequired...),
	}
	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "RADIUSProfile", "radiusprofile", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "RADIUSProfile", "radiusprofile", contract, dropped, measured)
}

// TestIntegrationSpatialRecordWriteContract measures the spatial record
// (api/s/{site}/rest/spatialrecord). devices may be an empty list, but a
// present item's mac, position, and position.x/y/z are all required, and
// mac is cross-validated against the site's real device inventory -- a
// well-formed but nonexistent MAC is refused exactly like a missing field,
// which this measures by adopting a real device via the herder rather than
// trusting an opaque literal.
func TestIntegrationSpatialRecordWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	devices := controllertest.StartDevices(ctx, t, c, controllertest.DeviceRequest{Model: controllertest.GatewayModel})
	if len(devices) != 1 {
		t.Skip("no emulated gateway available for this controller target")
	}
	adopted := c.AdoptDevice(ctx, t, s, devices[0].MAC)

	path := "/api/s/" + c.Site + "/rest/spatialrecord"

	base := func(name string) map[string]any {
		return map[string]any{
			"name": name,
			"devices": []any{
				map[string]any{
					"mac":      adopted.MAC,
					"position": map[string]any{"x": 1.5, "y": 2.5, "z": 0.5},
				},
			},
		}
	}
	post := func(doc map[string]any) (int, any, map[string]any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		created := firstData(t, body)
		if id := objectID(created); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, body, created
	}

	asked := base("spatialrecord-probe")
	body, status, err := s.PostJSON(ctx, path, asked)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		t.Fatalf("the known-good SpatialRecord body was rejected (HTTP %d): %v", status, firstData(t, body))
	}
	id := objectID(firstData(t, body))
	defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck

	read := func() map[string]any {
		got, status, err := s.GetJSON(ctx, path+"/"+id)
		if err != nil || status != 200 {
			t.Fatalf("GET %s/%s answered HTTP %d (%v)", path, id, status, err)
		}
		return firstData(t, got)
	}
	stored := read()

	dropped := []string{}
	for _, r := range probe.Classify(asked, stored) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("SpatialRecord: DROPPED %-16s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Logf("SpatialRecord: CHANGED %-16s (%s)", r.Wire, r.Detail)
		}
	}

	unknownKeyObserve(ctx, t, s, path, base("spatialrecord-unknown-key"))
	if _, listStatus, err := s.GetJSON(ctx, path); listStatus != 200 {
		t.Errorf("GET %s answered HTTP %d (%v)", path, listStatus, err)
	}

	renamed := clone(stored)
	renamed["name"] = "spatialrecord-probe-renamed"
	updateVerb, updatePath := "", ""
	if after, putStatus, err := s.PutJSON(ctx, path+"/"+id, renamed); putStatus/100 != 2 {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v)", path, id, putStatus, after, err)
	} else if got, _ := read()["name"].(string); got != "spatialrecord-probe-renamed" {
		t.Errorf("PUT %s/{id} answered HTTP %d but name still reads %q", path, putStatus, got)
	} else {
		updateVerb, updatePath = "PUT", "api/s/{site}/rest/spatialrecord/{id}"
	}
	s.PutJSON(ctx, path+"/"+id, clone(stored)) //nolint:errcheck // restore

	// name and devices (the key, not a populated item) are both required on
	// create -- the struct tags both omitempty, which hides it.
	required := requiredFieldSweep(t, []string{"name", "devices"}, func(field string) (int, any) {
		doc := base("spatialrecord-req-" + field)
		delete(doc, field)
		status, body, _ := post(doc)
		return status, body
	})

	// devices=[] (present, empty) has to be independently confirmed
	// accepted, or the item-level required fields below would be
	// indistinguishable from devices itself being required non-empty.
	if status, body, _ := post(map[string]any{"name": "spatialrecord-empty-devices", "devices": []any{}}); status/100 != 2 {
		t.Errorf("devices=[] was rejected (HTTP %d, %s); expected accepted", status, v1ErrCode(body))
	} else {
		t.Logf("devices=[] (present, empty) accepted (HTTP %d)", status)
	}

	// Item-level required fields: mac, position, position.x/y/z. Every
	// rejection on this resource answers the same generic api.err.Invalid,
	// so each field has to be isolated one at a time.
	itemFields := []string{"mac", "position"}
	nestedRequired := []string{}
	for _, field := range itemFields {
		item := map[string]any{"mac": adopted.MAC, "position": map[string]any{"x": 1.0, "y": 1.0, "z": 1.0}}
		delete(item, field)
		status, body, _ := post(map[string]any{
			"name": "spatialrecord-item-" + field, "devices": []any{item},
		})
		if status/100 == 2 {
			t.Errorf("a device item missing %s was accepted (HTTP %d); expected rejected", field, status)
			continue
		}
		t.Logf("a device item missing %-10s rejected (HTTP %d, %s) -- required on create", field, status, v1ErrCode(body))
		nestedRequired = append(nestedRequired, "devices."+field)
	}
	for _, axis := range []string{"x", "y", "z"} {
		position := map[string]any{"x": 1.0, "y": 1.0, "z": 1.0}
		delete(position, axis)
		status, body, _ := post(map[string]any{
			"name":    "spatialrecord-position-" + axis,
			"devices": []any{map[string]any{"mac": adopted.MAC, "position": position}},
		})
		if status/100 == 2 {
			t.Errorf("a position missing %s was accepted (HTTP %d); expected rejected", axis, status)
			continue
		}
		t.Logf("a position missing %-3s rejected (HTTP %d, %s) -- required on create", axis, status, v1ErrCode(body))
		nestedRequired = append(nestedRequired, "devices.position."+axis)
	}

	// The referential check: a well-formed but non-adopted MAC is refused
	// exactly like a missing field would be -- this is invisible to the
	// wire schema and to required_on_create.
	if status, body, _ := post(map[string]any{
		"name":    "spatialrecord-unknown-mac",
		"devices": []any{map[string]any{"mac": "aa:bb:cc:dd:ee:01", "position": map[string]any{"x": 0.0, "y": 0.0, "z": 0.0}}},
	}); status/100 == 2 {
		t.Error("a MAC no device on the site carries was accepted; the referential-integrity finding no longer holds")
	} else {
		t.Logf("a well-formed but non-adopted MAC rejected (HTTP %d, %s) -- devices[].mac is "+
			"cross-validated against the real device inventory, not just its pattern", status, v1ErrCode(body))
	}

	put := func(doc map[string]any) int {
		_, status, err := s.PutJSON(ctx, path+"/"+id, doc)
		if status == 0 {
			t.Fatalf("transport to %s/%s: %v", path, id, err)
		}
		return status
	}
	measured := map[string]behavior.EmptySemantics{
		"name":    storedEmptySemantics(t, "name", "", nil, stored, put, read),
		"devices": storedEmptySemantics(t, "devices", []any{}, nil, stored, put, read),
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "api/s/{site}/rest/spatialrecord",
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: append(required, nestedRequired...),
	}
	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "SpatialRecord", "spatialrecord", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "SpatialRecord", "spatialrecord", contract, dropped, measured)
}

// TestIntegrationFirewallRuleWriteContract measures the firewall rule
// (api/s/{site}/rest/firewallrule). action, rule_index and ruleset are
// required on create; every other field, including name, is optional.
// rule_index has to be unique per rule, so the seed advances a counter on
// every call rather than reusing one literal -- otherwise the required-on-
// create sweep would collide with the round-trip object still alive from
// the top of the test and every "not required" verdict below it would
// really be measuring a duplicate-index rejection instead.
//
// dst_port is deliberately NOT in the seed the required-on-create sweep
// runs against: a first pass that included it there made "without protocol"
// look required, because the sweep's own confound was dst_port staying
// present with protocol removed, which api.err.invalidProtocolWithPortSetting
// rejects for a reason that has nothing to do with protocol's own
// requiredness. dst_port's empty-vs-omit pair is measured afterwards, seeded
// in independently once protocol=tcp is already established.
func TestIntegrationFirewallRuleWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	ruleIndex := 2010
	nextIndex := func() int { ruleIndex++; return ruleIndex }

	spec := simpleV1Spec{
		resource:   "FirewallRule",
		collection: "firewallrule",
		seed: func(name string) map[string]any {
			return map[string]any{
				"name": name, "ruleset": "LAN_IN", "rule_index": nextIndex(),
				"action": "accept", "enabled": true, "protocol": "tcp",
				"protocol_match_excepted": false,
				"src_address":             "192.0.2.0/24",
				"dst_address":             "203.0.113.0/24",
				"logging":                 false,
				"setting_preference":      "manual",
				"state_established":       false,
				"state_invalid":           false,
				"state_new":               true,
				"state_related":           false,
			}
		},
		rename: "name",
		empties: []simpleV1Field{
			{"name", ""},
			{"src_address", ""},
		},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	// Confirm the confound directly: without dst_port in the picture at
	// all, omitting protocol is accepted -- so protocol is NOT required on
	// create, contrary to what a sweep run with dst_port present would say.
	path := "/api/s/" + c.Site + "/rest/firewallrule"
	post := func(doc map[string]any) (int, any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport: %v", err)
		}
		if id := objectID(firstData(t, body)); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, body
	}
	if status, body := post(map[string]any{
		"name": "firewallrule-no-protocol", "ruleset": "LAN_IN", "rule_index": nextIndex(),
		"action": "accept", "enabled": true,
	}); status/100 != 2 {
		t.Errorf("LOUD: omitting protocol (with no dst_port present) was rejected (HTTP %d, %s); "+
			"protocol may be genuinely required after all, not a sweep confound",
			status, v1ErrCode(body))
	} else {
		t.Logf("omitting protocol with no dst_port present is accepted (HTTP %d) -- confirms protocol "+
			"is optional; the earlier rejection was dst_port colliding with a defaulted protocol", status)
	}

	// dst_port's own empty-vs-omit pair, seeded independently once
	// protocol=tcp already holds -- and the protocol=all interaction,
	// refused-if-present rather than a requiredness fact.
	seedID, seedStored := firewallRuleSeedDstPort(ctx, t, s, path, nextIndex())
	defer s.DeleteJSON(ctx, path+"/"+seedID) //nolint:errcheck
	read := func() map[string]any {
		got, status, err := s.GetJSON(ctx, path+"/"+seedID)
		if err != nil || status != 200 {
			t.Fatalf("GET %s/%s answered HTTP %d (%v)", path, seedID, status, err)
		}
		return firstData(t, got)
	}
	put := func(doc map[string]any) int {
		_, status, err := s.PutJSON(ctx, path+"/"+seedID, doc)
		if status == 0 {
			t.Fatalf("transport to %s/%s: %v", path, seedID, err)
		}
		return status
	}
	measured["dst_port"] = storedEmptySemantics(t, "dst_port", "", nil, seedStored, put, read)

	if status, body := post(map[string]any{
		"name": "firewallrule-protocol-all-port", "ruleset": "LAN_IN", "rule_index": nextIndex(),
		"action": "accept", "enabled": true, "protocol": "all", "dst_port": "8080",
	}); status/100 == 2 {
		t.Error("dst_port under protocol=all was accepted; the invalidProtocolWithPortSetting finding no longer holds")
	} else {
		t.Logf("dst_port under protocol=all rejected (HTTP %d, %s) -- refused under this branch, not "+
			"unconditionally required", status, v1ErrCode(body))
	}

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "FirewallRule", "firewallrule", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "FirewallRule", "firewallrule", contract, dropped, measured)
}

// firewallRuleSeedDstPort creates a protocol=tcp rule carrying a real
// dst_port, for the empty-vs-omit pass to seed from independently of the
// required-on-create sweep's body.
func firewallRuleSeedDstPort(
	ctx context.Context, t *testing.T, s *controllertest.Session, path string, index int,
) (id string, stored map[string]any) {
	t.Helper()
	body, status, err := s.PostJSON(ctx, path, map[string]any{
		"name": "firewallrule-dst-port-seed", "ruleset": "LAN_IN", "rule_index": index,
		"action": "accept", "enabled": true, "protocol": "tcp", "dst_port": "8080",
	})
	if status == 0 || status/100 != 2 {
		t.Fatalf("seeding dst_port for the empty-vs-omit pass was rejected (HTTP %d): %v %v", status, body, err)
	}
	id = objectID(firstData(t, body))
	if id == "" {
		t.Fatalf("the dst_port seed rule carries no id: %v", body)
	}
	return id, firstData(t, body)
}

// TestIntegrationNetworkMembersGroupWriteContract measures the network
// members group (v2/api/site/{site}/network-members-group, singular --
// TestIntegrationNetworkMembersGroupCRUD already pins that only List uses
// the plural path). name, type and members are required on every write, not
// just create: a PUT naming only one of them is refused with the identical
// validation shape create uses, so there is no field this resource can ever
// legally omit from a write.
func TestIntegrationNetworkMembersGroupWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/v2/api/site/" + c.Site + "/network-members-group"

	base := func(name string) map[string]any {
		return map[string]any{"name": name, "type": "CLIENTS", "members": []any{}}
	}
	post := func(doc map[string]any) (int, any, map[string]any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		created := firstData(t, body)
		if id := objectID(created); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, body, created
	}

	asked := base("nmg-probe")
	body, status, err := s.PostJSON(ctx, path, asked)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		t.Fatalf("the known-good NetworkMembersGroup body was rejected (HTTP %d): %v; confirm the "+
			"path is singular, not plural (only List uses the plural)", status, firstData(t, body))
	}
	id := objectID(firstData(t, body))
	defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck

	read := func() map[string]any {
		got, status, err := s.GetJSON(ctx, path+"/"+id)
		if err != nil || status != 200 {
			t.Fatalf("GET %s/%s answered HTTP %d (%v)", path, id, status, err)
		}
		return firstData(t, got)
	}
	stored := read()

	dropped := []string{}
	for _, r := range probe.Classify(asked, stored) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("NetworkMembersGroup: DROPPED %-16s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Logf("NetworkMembersGroup: CHANGED %-16s (%s)", r.Wire, r.Detail)
		}
	}

	// LOUD: this collection is documented elsewhere as an example of the
	// general v2 rule that an unrecognised key draws a 400 naming it.
	// Measured directly, it does not -- it strips the key like a v1
	// collection, a second exception alongside TrafficRoute.
	tag := unknownKeyObserve(ctx, t, s, path, base("nmg-unknown-key"))
	if tag != "STRIPPED" {
		t.Errorf("LOUD: network-members-group now answers an unrecognised key with %s, not STRIPPED; "+
			"the v2-exception finding for this collection no longer holds", tag)
	}

	if _, listStatus, err := s.GetJSON(ctx, "/v2/api/site/"+c.Site+"/network-members-groups"); listStatus != 200 {
		t.Errorf("GET the plural list path answered HTTP %d (%v)", listStatus, err)
	}
	if _, singularListStatus, _ := s.GetJSON(ctx, path); singularListStatus == 200 {
		t.Logf("note: the singular path now also answers a list GET (HTTP 200); harmless, just no longer distinctive")
	}

	renamed := clone(stored)
	renamed["name"] = "nmg-probe-renamed"
	updateVerb, updatePath := "", ""
	if after, putStatus, err := s.PutJSON(ctx, path+"/"+id, renamed); putStatus/100 != 2 {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v)", path, id, putStatus, after, err)
	} else if got, _ := read()["name"].(string); got != "nmg-probe-renamed" {
		t.Errorf("PUT %s/{id} answered HTTP %d but name still reads %q", path, putStatus, got)
	} else {
		updateVerb, updatePath = "PUT", "v2/api/site/{site}/network-members-group/{id}"
	}
	s.PutJSON(ctx, path+"/"+id, clone(stored)) //nolint:errcheck // restore

	required := requiredFieldSweep(t, []string{"name", "type", "members"}, func(field string) (int, any) {
		doc := base("nmg-req-" + field)
		delete(doc, field)
		status, body, _ := post(doc)
		return status, body
	})

	// The finding worth its own assertion: an update naming only one field
	// is rejected exactly like a create missing the other two, not merged.
	for _, field := range []string{"name", "type", "members"} {
		doc := map[string]any{field: stored[field]}
		if _, st, _ := s.PutJSON(ctx, path+"/"+id, doc); st/100 == 2 {
			t.Errorf("PUT naming only %s was accepted (HTTP %d); update no longer requires the full document", field, st)
		} else {
			t.Logf("PUT naming only %-8s rejected (HTTP %d) -- update re-validates the full document", field, st)
		}
	}
	s.PutJSON(ctx, path+"/"+id, clone(stored)) //nolint:errcheck // restore in case any partial took

	measured := map[string]behavior.EmptySemantics{
		"name": storedEmptySemantics(t, "name", "", nil, stored,
			func(doc map[string]any) int { _, st, _ := s.PutJSON(ctx, path+"/"+id, doc); return st }, read),
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "v2/api/site/{site}/network-members-group",
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: required,
		RequiredOnUpdate: []string{"members", "name", "type"},
	}
	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "NetworkMembersGroup", "network-members-group", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "NetworkMembersGroup", "network-members-group", contract, dropped, measured)
}

// TestIntegrationAccountWriteContract measures the RADIUS tunnel account
// (api/s/{site}/rest/account). Every field is individually optional on
// create -- including name -- and tunnel_config_type is a discriminator
// (vpn|802.1x|custom|absent) swept across all four values rather than just
// the vpn branch a single known-good body would exercise, closing the gap a
// prior measurement of this resource left open.
func TestIntegrationAccountWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	spec := simpleV1Spec{
		resource:   "Account",
		collection: "account",
		seed: func(name string) map[string]any {
			return map[string]any{
				"name": name, "x_password": "probe-secret",
				"tunnel_config_type": "vpn", "tunnel_medium_type": 6, "tunnel_type": 13, "vlan": 100,
				// ip and ulp_user_id carry no controller default (blank
				// unless asked), so both are seeded with a real value here
				// too, or their own empty-vs-omit pass measures nothing.
				"ip":          "192.0.2.99",
				"ulp_user_id": "probe-ulp-id",
			}
		},
		rename: "name",
		empties: []simpleV1Field{
			{"name", ""},
			{"x_password", ""},
			{"ip", ""},
			{"tunnel_config_type", ""},
			{"tunnel_medium_type", ""},
			{"tunnel_type", ""},
			{"vlan", ""},
			{"ulp_user_id", ""},
		},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	// tunnel_config_type is a discriminator; the vpn branch is what the
	// generic sweep above exercised. Sweep the other three branches too,
	// each with only name+x_password+the branch flag, closing the branch-
	// blindness gap a prior measurement of this resource left open.
	path := "/api/s/" + c.Site + "/rest/account"
	for _, branch := range []string{"802.1x", "custom", ""} {
		doc := map[string]any{"name": "account-branch-probe", "x_password": "probe-secret"}
		if branch != "" {
			doc["tunnel_config_type"] = branch
		}
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport: %v", err)
		}
		if id := objectID(firstData(t, body)); id != "" {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		label := branch
		if label == "" {
			label = "(absent)"
		}
		if status/100 != 2 {
			t.Errorf("tunnel_config_type=%s alone (no companion fields) was rejected (HTTP %d, %s); "+
				"expected accepted -- this branch would then need its own required-on-create sweep",
				label, status, v1ErrCode(body))
		} else {
			t.Logf("tunnel_config_type=%-8s alone accepted (HTTP %d) -- no companion field required", label, status)
		}
	}

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "Account", "account", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "Account", "account", contract, dropped, measured)
}

// TestIntegrationRoutingWriteContract measures the static route
// (api/s/{site}/rest/routing). static-route_type is a create-time
// discriminator swept across all three values: nexthop-route additionally
// needs static-route_nexthop, interface-route needs static-route_interface,
// blackhole needs neither. Omitting static-route_type entirely crashes the
// controller with a bare HTML 500 on every branch -- a real bug, measured
// and logged rather than silently absorbed into the required-field list.
func TestIntegrationRoutingWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/api/s/" + c.Site + "/rest/routing"

	branchBody := func(name, routeType string) map[string]any {
		doc := map[string]any{
			"name": name, "enabled": true, "type": "static-route",
			"static-route_type": routeType, "static-route_network": "203.0.113.0/24",
		}
		switch routeType {
		case "nexthop-route":
			doc["static-route_nexthop"] = "192.0.2.1"
		case "interface-route":
			doc["static-route_interface"] = "WAN1"
		}
		return doc
	}
	post := func(doc map[string]any) (int, any, map[string]any) {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		created := firstData(t, body)
		if id := objectID(created); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status, body, created
	}

	// The nexthop-route branch is the one carried through the round-trip,
	// unknown-key and update checks; the other two are swept for their own
	// required-on-create set only.
	asked := branchBody("routing-probe", "nexthop-route")
	body, status, err := s.PostJSON(ctx, path, asked)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		t.Fatalf("the known-good nexthop-route body was rejected (HTTP %d): %v", status, firstData(t, body))
	}
	id := objectID(firstData(t, body))
	defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck

	read := func() map[string]any {
		got, status, err := s.GetJSON(ctx, path+"/"+id)
		if err != nil || status != 200 {
			t.Fatalf("GET %s/%s answered HTTP %d (%v)", path, id, status, err)
		}
		return firstData(t, got)
	}
	stored := read()

	dropped := []string{}
	for _, r := range probe.Classify(asked, stored) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("Routing: DROPPED %-24s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Logf("Routing: CHANGED %-24s (%s)", r.Wire, r.Detail)
		}
	}

	unknownKeyObserve(ctx, t, s, path, branchBody("routing-unknown-key", "nexthop-route"))
	if _, listStatus, err := s.GetJSON(ctx, path); listStatus != 200 {
		t.Errorf("GET %s answered HTTP %d (%v)", path, listStatus, err)
	}

	renamed := clone(stored)
	renamed["name"] = "routing-probe-renamed"
	updateVerb, updatePath := "", ""
	if after, putStatus, err := s.PutJSON(ctx, path+"/"+id, renamed); putStatus/100 != 2 {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v)", path, id, putStatus, after, err)
	} else if got, _ := read()["name"].(string); got != "routing-probe-renamed" {
		t.Errorf("PUT %s/{id} answered HTTP %d but name still reads %q", path, putStatus, got)
	} else {
		updateVerb, updatePath = "PUT", "api/s/{site}/rest/routing/{id}"
	}
	s.PutJSON(ctx, path+"/"+id, clone(stored)) //nolint:errcheck // restore

	commonRequired := requiredFieldSweep(t, []string{"type", "static-route_network", "static-route_type"},
		func(field string) (int, any) {
			doc := branchBody("routing-req-"+field, "nexthop-route")
			delete(doc, field)
			status, body, _ := post(doc)
			return status, body
		})

	// static-route_type omitted entirely crashes the controller with a bare
	// HTML 500, not a clean validation error -- worth its own log line
	// rather than folding silently into "required".
	crashDoc := branchBody("routing-crash-probe", "nexthop-route")
	delete(crashDoc, "static-route_type")
	if status, _, _ := post(crashDoc); status == 500 {
		t.Logf("LOUD: omitting static-route_type crashes the controller with HTTP 500 (no clean " +
			"validation error), confirming a real bug rather than a normal required-field rejection")
	} else {
		t.Logf("omitting static-route_type answered HTTP %d (not the previously measured bare 500)", status)
	}

	when := map[string][]string{}
	for _, branch := range []struct{ routeType, extra string }{
		{"nexthop-route", "static-route_nexthop"},
		{"interface-route", "static-route_interface"},
		{"blackhole", ""},
	} {
		if status, body, _ := post(branchBody("routing-branch-"+branch.routeType, branch.routeType)); status/100 != 2 {
			t.Fatalf("the known-good %s body was rejected (HTTP %d): %v", branch.routeType, status, body)
		}
		var extra []string
		if branch.extra != "" {
			doc := branchBody("routing-branch-missing-"+branch.routeType, branch.routeType)
			delete(doc, branch.extra)
			status, body, _ := post(doc)
			if status/100 == 2 {
				t.Errorf("%s without %s was accepted (HTTP %d); expected rejected", branch.routeType, branch.extra, status)
			} else {
				t.Logf("%s without %-24s rejected (HTTP %d, %s) -- required on this branch",
					branch.routeType, branch.extra, status, v1ErrCode(body))
				extra = []string{branch.extra}
			}
		} else {
			t.Logf("blackhole with only the common fields creates fine -- no branch-specific requirement")
		}
		when["static-route_type="+branch.routeType] = extra
	}

	put := func(doc map[string]any) int {
		_, status, err := s.PutJSON(ctx, path+"/"+id, doc)
		if status == 0 {
			t.Fatalf("transport to %s/%s: %v", path, id, err)
		}
		return status
	}

	// static-route_distance carries no value on the known-good body, so it
	// is seeded with a real non-default one first -- the seeding trap.
	distanceSeed := clone(stored)
	distanceSeed["static-route_distance"] = 5
	if st := put(distanceSeed); st/100 != 2 {
		t.Fatalf("seeding static-route_distance for the empty-vs-omit pass was rejected (HTTP %d)", st)
	}
	distanceSeed = read()

	measured := map[string]behavior.EmptySemantics{
		"name":                  storedEmptySemantics(t, "name", "", nil, stored, put, read),
		"static-route_distance": storedEmptySemantics(t, "static-route_distance", "", nil, distanceSeed, put, read),
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "api/s/{site}/rest/routing",
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate:     commonRequired,
		RequiredOnCreateWhen: when,
	}
	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "Routing", "routing", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "Routing", "routing", contract, dropped, measured)
}
