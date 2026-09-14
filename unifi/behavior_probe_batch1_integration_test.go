//go:build integration

// unifi/behavior_probe_batch1_integration_test.go
package unifi

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/internal/probe"
)

// lanHostIP builds a plausible host address inside cidr (the site's default
// LAN ip_subnet, e.g. "192.168.1.1/24") by replacing the last octet. A
// fixed_ip outside the client's LAN is refused as api.err.InvalidFixedIP --
// measured, not assumed, when an out-of-subnet literal (192.0.2.50) was
// tried first and rejected -- so the probe needs a real in-subnet address
// rather than an arbitrary one.
func lanHostIP(t *testing.T, cidr string, host byte) string {
	t.Helper()
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("default LAN ip_subnet %q does not parse as a CIDR: %v", cidr, err)
	}
	base := ipnet.IP.To4()
	if base == nil {
		t.Fatalf("default LAN ip_subnet %q is not IPv4", cidr)
	}
	return net.IPv4(base[0], base[1], base[2], host).String()
}

// Batch 1 of the write-contract re-measurement: APGroup, Client, ClientGroup,
// DpiApp, DpiGroup, HotspotOp. See behavior_probe_shared_integration_test.go
// for the machinery these share.

// TestIntegrationAPGroupWriteContract measures the AP group (v2/api/site/
// {site}/apgroups). Its base branch -- for_wlanconf omitted or false -- needs
// nothing beyond name and a device_macs key (which may be an empty list);
// the for_wlanconf=true branch additionally requires device_macs to name a
// real, adopted AP, which this probe measures by herding and adopting one
// rather than leaving the branch unmeasured.
func TestIntegrationAPGroupWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/v2/api/site/" + c.Site + "/apgroups"

	base := func(name string) map[string]any {
		return map[string]any{"name": name, "device_macs": []any{}}
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

	asked := base("apgroup-probe")
	body, status, err := s.PostJSON(ctx, path, asked)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		t.Fatalf("the known-good AP group body was rejected (HTTP %d): %v\n\nNothing removed from a "+
			"body that does not create can measure anything.", status, firstData(t, body))
	}
	id := objectID(firstData(t, body))
	if id == "" {
		t.Fatalf("the created AP group carries no id: %v", body)
	}
	defer s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck

	// The collection has no by-id GET (GetAPGroup lists and filters, per the
	// generated client): a GET to path+"/"+id answers a bare HTTP 405, not a
	// per-object read, so every verdict below lists and picks by id instead.
	read := func() map[string]any {
		got, status, err := s.GetJSON(ctx, path)
		if err != nil || status != 200 {
			t.Fatalf("GET %s answered HTTP %d (%v); the generated list reads that path", path, status, err)
		}
		for _, item := range asSlice(got) {
			if m, _ := item.(map[string]any); objectID(m) == id {
				return m
			}
		}
		t.Fatalf("GET %s does not list the AP group just created (id=%s)", path, id)
		return nil
	}
	stored := read()

	dropped := []string{}
	for _, r := range probe.Classify(asked, stored) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("APGroup: DROPPED %-16s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Logf("APGroup: CHANGED %-16s (%s)", r.Wire, r.Detail)
		}
	}

	unknownKeyObserve(ctx, t, s, path, base("apgroup-unknown-key"))

	if _, listStatus, err := s.GetJSON(ctx, path); listStatus != 200 {
		t.Errorf("GET %s answered HTTP %d (%v); the generated list reads that path", path, listStatus, err)
	}

	renamed := clone(stored)
	renamed["name"] = "apgroup-probe-renamed"
	updateVerb, updatePath := "", ""
	if after, putStatus, err := s.PutJSON(ctx, path+"/"+id, renamed); putStatus/100 != 2 {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v); the generated update writes to that path",
			path, id, putStatus, after, err)
	} else if got, _ := read()["name"].(string); got != "apgroup-probe-renamed" {
		t.Errorf("PUT %s/{id} answered HTTP %d but the group still reports name %q", path, putStatus, got)
	} else {
		updateVerb, updatePath = "PUT", "v2/api/site/{site}/apgroups/{id}"
	}
	s.PutJSON(ctx, path+"/"+id, clone(stored)) //nolint:errcheck // restore

	// The masked-write trap this resource's own generated helper falls
	// into: a PUT naming only name, without device_macs, is measured
	// against the full-document validation a create would apply -- not
	// merged.
	partialStatus := -1
	if _, st, err := s.PutJSON(ctx, path+"/"+id, map[string]any{"name": "apgroup-partial-probe"}); err == nil {
		partialStatus = st
	}
	if partialStatus/100 == 2 {
		t.Log("LOUD: a PUT naming only name (no device_macs) is now accepted; UpdateAPGroupFields's " +
			"masked write may no longer need every caller to supply both fields")
	} else {
		t.Logf("PUT %s/%s with only name (no device_macs) -> HTTP %d, confirming the update validates "+
			"the full document rather than merging a partial one", path, id, partialStatus)
	}
	s.PutJSON(ctx, path+"/"+id, clone(stored)) //nolint:errcheck // restore again, in case the partial took

	required := requiredFieldSweep(t, []string{"name", "device_macs"}, func(field string) (int, any) {
		doc := base("apgroup-req-" + field)
		delete(doc, field)
		status, body, _ := post(doc)
		return status, body
	})

	put := func(doc map[string]any) int {
		_, status, err := s.PutJSON(ctx, path+"/"+id, doc)
		if status == 0 {
			t.Fatalf("transport to %s/%s: %v", path, id, err)
		}
		return status
	}
	measured := map[string]behavior.EmptySemantics{
		"name": storedEmptySemantics(t, "name", "", nil, stored, put, read),
	}

	// The for_wlanconf=true branch: real per findings, but only measurable
	// with an adopted AP on the site. Herd and adopt a U7PRO so this branch
	// is asserted rather than left as a documented gap.
	devices := controllertest.StartDevices(ctx, t, c, controllertest.DeviceRequest{Model: "U7PRO"})
	if len(devices) != 1 {
		t.Log("no emulated AP available on this target; for_wlanconf=true stays unmeasured " +
			"(nothing here contradicts the base-branch contract above)")
	} else {
		adopted := c.AdoptDevice(ctx, t, s, devices[0].MAC)

		if status, body, created := post(map[string]any{
			"name": "apgroup-wlanconf-ok", "for_wlanconf": true, "device_macs": []any{adopted.MAC},
		}); status/100 == 2 {
			t.Logf("for_wlanconf=true with a real adopted AP MAC creates (HTTP %d, id=%s)",
				status, objectID(created))
		} else {
			t.Errorf("for_wlanconf=true with a real adopted AP MAC was rejected (HTTP %d): %v",
				status, body)
		}

		if status, body, _ := post(map[string]any{
			"name": "apgroup-wlanconf-empty", "for_wlanconf": true, "device_macs": []any{},
		}); status/100 == 2 {
			t.Error("for_wlanconf=true with device_macs=[] was accepted; the prior finding's " +
				"ApGroupForWlanMustHaveDevices rejection no longer holds")
		} else {
			t.Logf("for_wlanconf=true with device_macs=[] rejected (HTTP %d, %s) -- matches "+
				"api.err.ApGroupForWlanMustHaveDevices", status, v1ErrCode(body))
		}

		if status, body, _ := post(map[string]any{
			"name": "apgroup-wlanconf-bad-mac", "for_wlanconf": true,
			"device_macs": []any{"aa:bb:cc:dd:ee:ff"},
		}); status/100 == 2 {
			t.Error("for_wlanconf=true naming a MAC no device on the site carries was accepted; the " +
				"prior finding's InvalidDeviceInApGroup rejection no longer holds")
		} else {
			t.Logf("for_wlanconf=true with an unadopted MAC rejected (HTTP %d, %s) -- matches "+
				"api.err.InvalidDeviceInApGroup", status, v1ErrCode(body))
		}
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "v2/api/site/{site}/apgroups",
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: required,
	}
	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "APGroup", "apgroups", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "APGroup", "apgroups", contract, dropped, measured)
}

// TestIntegrationClientWriteContract measures the Client (api/s/{site}/
// rest/user). mac is unconditionally required; use_fixedip and
// local_dns_record_enabled each open a dependent-field branch, and the two
// chain (local_dns_record_enabled=true transitively needs use_fixedip=true).
// Each branch is swept in isolation rather than from one body carrying every
// flag true, which would confound the three rules into one flat list.
func TestIntegrationClientWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/api/s/" + c.Site + "/rest/user"
	networkID := ensureWANNetwork(ctx, t, s, c.Site)

	lanCIDR := ""
	if nets, err := listNetworks(ctx, s, c.Site); err == nil {
		for _, n := range nets {
			if purpose, _ := n["purpose"].(string); purpose == PurposeCorporate {
				lanCIDR, _ = n["ip_subnet"].(string)
				break
			}
		}
	}
	if lanCIDR == "" {
		t.Fatalf("no default LAN network with ip_subnet found; cannot build a valid fixed_ip")
	}

	macFor := func(last byte) string { return fmt.Sprintf("02:00:00:00:00:%02x", last) }

	// post creates and forgets the client again via the real removal path
	// (stamgr forget-sta): the REST DELETE this collection's generated
	// client omits is measured below to answer 404, not merely absent from
	// the generator's output.
	forget := func(mac string) {
		client := harnessClient(ctx, t, c)
		client.DeleteClientByMAC(ctx, c.Site, mac) //nolint:errcheck
	}

	base := func(mac string) map[string]any {
		return map[string]any{"mac": mac}
	}

	asked := base(macFor(1))
	body, status, err := s.PostJSON(ctx, path, asked)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		t.Fatalf("the known-good client body (mac alone) was rejected (HTTP %d): %v", status, firstData(t, body))
	}
	id := objectID(firstData(t, body))
	if id == "" {
		t.Fatalf("the created client carries no id: %v", body)
	}
	defer forget(macFor(1))

	read := func() map[string]any {
		got, status, err := s.GetJSON(ctx, path+"/"+id)
		if err != nil || status != 200 {
			t.Fatalf("GET %s/%s answered HTTP %d (%v)", path, id, status, err)
		}
		return firstData(t, got)
	}
	stored := read()
	t.Logf("Client: mac-only create accepted (HTTP %d); every other field on the struct is genuinely optional", status)

	unknownKeyObserve(ctx, t, s, path, map[string]any{"mac": macFor(2), probeUnknownKey: "x"})
	forget(macFor(2))

	if _, listStatus, err := s.GetJSON(ctx, path); listStatus != 200 {
		t.Errorf("GET %s answered HTTP %d (%v); the generated list reads that path", path, listStatus, err)
	}

	updateVerb, updatePath := "", ""
	renamed := clone(stored)
	renamed["name"] = "client-probe-renamed"
	if after, putStatus, err := s.PutJSON(ctx, path+"/"+id, renamed); putStatus/100 != 2 {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v)", path, id, putStatus, after, err)
	} else if got, _ := read()["name"].(string); got != "client-probe-renamed" {
		t.Errorf("PUT %s/{id} answered HTTP %d but name still reads %q", path, putStatus, got)
	} else {
		updateVerb, updatePath = "PUT", "api/s/{site}/rest/user/{id}"
	}

	put := func(doc map[string]any) int {
		_, status, err := s.PutJSON(ctx, path+"/"+id, doc)
		if status == 0 {
			t.Fatalf("transport to %s/%s: %v", path, id, err)
		}
		return status
	}
	afterRename := read()

	// note and network_id both start absent/blank on a mac-only create, so
	// each is seeded with a value the controller owns no default for before
	// its empty-vs-omit pass runs -- otherwise a re-defaulted value could
	// not be told from a preserved one (the seeding trap).
	seedDoc := clone(afterRename)
	seedDoc["note"] = "probe-note"
	seedDoc["network_id"] = networkID
	if st := put(seedDoc); st/100 != 2 {
		t.Fatalf("seeding note/network_id for the empty-vs-omit pass was rejected (HTTP %d)", st)
	}
	seeded := read()

	measured := map[string]behavior.EmptySemantics{
		"name":       storedEmptySemantics(t, "name", "", nil, afterRename, put, read),
		"note":       storedEmptySemantics(t, "note", "", nil, seeded, put, read),
		"network_id": storedEmptySemantics(t, "network_id", "", nil, seeded, put, read),
	}

	// Required-on-create: mac alone.
	required := requiredFieldSweep(t, []string{"mac"}, func(field string) (int, any) {
		mac := macFor(10)
		doc := base(mac)
		delete(doc, field)
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport: %v", err)
		}
		if status/100 == 2 {
			forget(mac)
		}
		return status, body
	})

	// Branch 1: use_fixedip=true alone needs fixed_ip.
	tryCreate := func(mac string, doc map[string]any) (int, any) {
		doc["mac"] = mac
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport: %v", err)
		}
		if status/100 == 2 {
			forget(mac)
		}
		return status, body
	}
	if status, body := tryCreate(macFor(20), map[string]any{"use_fixedip": true}); status/100 == 2 {
		t.Error("use_fixedip=true without fixed_ip was accepted; the InvalidFixedIP finding no longer holds")
	} else {
		t.Logf("use_fixedip=true without fixed_ip rejected (HTTP %d, %s)", status, v1ErrCode(body))
	}
	// fixed_ip has to sit inside the client's own LAN subnet: an
	// out-of-subnet literal (192.0.2.50) was tried first and rejected as
	// api.err.InvalidFixedIP, indistinguishable in the error code alone from
	// the field being missing -- so this uses a real in-subnet address.
	if status, body := tryCreate(macFor(21), map[string]any{
		"use_fixedip": true, "fixed_ip": lanHostIP(t, lanCIDR, 50),
	}); status/100 != 2 {
		t.Errorf("use_fixedip=true with an in-subnet fixed_ip was rejected (HTTP %d, %s); expected accepted",
			status, v1ErrCode(body))
	} else {
		t.Logf("use_fixedip=true with an in-subnet fixed_ip accepted (HTTP %d)", status)
	}

	// Branch 2: local_dns_record_enabled=true chains through use_fixedip
	// and fixed_ip, then needs local_dns_record itself.
	if status, body := tryCreate(macFor(22), map[string]any{"local_dns_record_enabled": true}); status/100 == 2 {
		t.Error("local_dns_record_enabled=true alone was accepted; the LocalDnsRecordRequiresFixedIp finding no longer holds")
	} else {
		t.Logf("local_dns_record_enabled=true alone rejected (HTTP %d, %s)", status, v1ErrCode(body))
	}
	if status, body := tryCreate(macFor(23), map[string]any{
		"local_dns_record_enabled": true, "use_fixedip": true, "fixed_ip": lanHostIP(t, lanCIDR, 51),
	}); status/100 == 2 {
		t.Error("local_dns_record_enabled=true + use_fixedip/fixed_ip but no local_dns_record was accepted; the LocalDnsRecordMissing finding no longer holds")
	} else {
		t.Logf("local_dns_record_enabled=true + use_fixedip/fixed_ip without local_dns_record rejected (HTTP %d, %s)",
			status, v1ErrCode(body))
	}
	if status, body := tryCreate(macFor(24), map[string]any{
		"local_dns_record_enabled": true, "use_fixedip": true, "fixed_ip": lanHostIP(t, lanCIDR, 52),
		"local_dns_record": "probe.example.internal",
	}); status/100 != 2 {
		t.Errorf("the full local-DNS chain was rejected (HTTP %d, %s); expected accepted", status, v1ErrCode(body))
	} else {
		t.Logf("the full local-DNS chain (local_dns_record_enabled+use_fixedip+fixed_ip+local_dns_record) accepted (HTTP %d)", status)
	}

	// The generated client has no DeleteClient: confirm why by trying the
	// REST DELETE directly against an id a prior GET just confirmed exists.
	if _, delStatus, err := s.DeleteJSON(ctx, path+"/"+id); delStatus == 404 {
		t.Logf("DELETE %s/%s -> HTTP 404, confirming REST delete does not work; forget-sta (stamgr) is "+
			"the only removal path, which is why no DeleteClient is generated", path, id)
	} else if delStatus/100 == 2 {
		t.Errorf("LOUD: DELETE %s/%s now succeeds (HTTP %d, %v); a generated DeleteClient may now be correct",
			path, id, delStatus, err)
	} else {
		t.Logf("DELETE %s/%s -> HTTP %d (not 404, not 2xx): %v", path, id, delStatus, err)
	}
	forget(macFor(1)) // the real removal path, since REST DELETE does not work

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "api/s/{site}/rest/user",
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: required,
		RequiredOnCreateWhen: map[string][]string{
			"use_fixedip=true":              {"fixed_ip"},
			"local_dns_record_enabled=true": {"local_dns_record", "use_fixedip"},
		},
	}
	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "Client", "user", contract, nil, measured)
		return
	}
	compareRecorded(t, root, running, "Client", "user", contract, nil, measured)
}

// TestIntegrationClientGroupWriteContract measures the client group
// (api/s/{site}/rest/usergroup): three scalar fields, all optional on
// create, a merging PUT.
func TestIntegrationClientGroupWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	spec := simpleV1Spec{
		resource:   "ClientGroup",
		collection: "usergroup",
		seed: func(name string) map[string]any {
			return map[string]any{"name": name, "qos_rate_max_down": 1000, "qos_rate_max_up": 500}
		},
		rename: "name",
		empties: []simpleV1Field{
			{"name", ""},
			{"qos_rate_max_down", ""},
			{"qos_rate_max_up", ""},
		},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "ClientGroup", "usergroup", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "ClientGroup", "usergroup", contract, dropped, measured)
}

// TestIntegrationDpiAppWriteContract measures the DPI app
// (api/s/{site}/rest/dpiapp). blocked and log are seeded true, since their
// controller default is false and a seeding trap would make a re-defaulted
// value unreadable as a clear.
func TestIntegrationDpiAppWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	spec := simpleV1Spec{
		resource:   "DpiApp",
		collection: "dpiapp",
		seed: func(name string) map[string]any {
			return map[string]any{
				"name": name, "enabled": true, "blocked": true, "log": true,
				"apps": []any{65536}, "cats": []any{4},
				"qos_rate_max_up": 500, "qos_rate_max_down": 1000,
			}
		},
		rename: "name",
		empties: []simpleV1Field{
			{"name", ""},
			{"apps", []any{}},
			{"cats", []any{}},
			{"qos_rate_max_up", ""},
			{"qos_rate_max_down", ""},
			{"blocked", false},
			{"log", false},
		},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	// The apps/cats pairing looks like a branch (one substituting for the
	// other), so it is swept independently rather than folded into the
	// generic required-on-create pass: stripping one while keeping the
	// other still creates.
	path := "/api/s/" + c.Site + "/rest/dpiapp"
	post := func(doc map[string]any) int {
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport: %v", err)
		}
		if id := objectID(firstData(t, body)); id != "" && status/100 == 2 {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
		return status
	}
	if st := post(map[string]any{"name": "dpiapp-apps-only", "apps": []any{65536}}); st/100 != 2 {
		t.Errorf("apps alone (no cats) was rejected (HTTP %d); expected accepted", st)
	}
	if st := post(map[string]any{"name": "dpiapp-cats-only", "cats": []any{4}}); st/100 != 2 {
		t.Errorf("cats alone (no apps) was rejected (HTTP %d); expected accepted", st)
	}

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "DpiApp", "dpiapp", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "DpiApp", "dpiapp", contract, dropped, measured)
}

// TestIntegrationDpiGroupWriteContract measures the DPI group
// (api/s/{site}/rest/dpigroup). dpiapp_ids references real DpiApp
// documents, so this seeds one live rather than trusting an opaque id
// string.
func TestIntegrationDpiGroupWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	appPath := "/api/s/" + c.Site + "/rest/dpiapp"
	appBody, appStatus, err := s.PostJSON(ctx, appPath, map[string]any{
		"name": "dpigroup-probe-app", "apps": []any{65536},
	})
	if err != nil || appStatus/100 != 2 {
		t.Fatalf("seeding a DpiApp for dpiapp_ids failed (HTTP %d): %v", appStatus, err)
	}
	appID := objectID(firstData(t, appBody))
	defer s.DeleteJSON(ctx, appPath+"/"+appID) //nolint:errcheck

	spec := simpleV1Spec{
		resource:   "DpiGroup",
		collection: "dpigroup",
		seed: func(name string) map[string]any {
			return map[string]any{"name": name, "enabled": true, "dpiapp_ids": []any{appID}}
		},
		rename: "name",
		empties: []simpleV1Field{
			{"name", ""},
			{"dpiapp_ids", []any{}},
			{"enabled", false},
		},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "DpiGroup", "dpigroup", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "DpiGroup", "dpigroup", contract, dropped, measured)
}

// TestIntegrationHotspotOpWriteContract measures the hotspot operator
// (api/s/{site}/rest/hotspotop): three scalar fields, all optional.
func TestIntegrationHotspotOpWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	spec := simpleV1Spec{
		resource:   "HotspotOp",
		collection: "hotspotop",
		seed: func(name string) map[string]any {
			return map[string]any{"name": name, "note": "hotspotop probe", "x_password": "probe-secret-1"}
		},
		rename: "name",
		empties: []simpleV1Field{
			{"name", ""},
			{"note", ""},
			{"x_password", ""},
		},
	}
	contract, dropped, measured := measureSimpleV1Contract(ctx, t, s, c.Site, spec)

	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "HotspotOp", "hotspotop", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "HotspotOp", "hotspotop", contract, dropped, measured)
}
