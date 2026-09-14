//go:build integration

// unifi/behavior_probe_batch4_integration_test.go
package unifi

import (
	"context"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/internal/probe"
)

// Batch 4 of the write-contract re-measurement: Device, DevicePortOverrides,
// PortProfile, Setting (mgmt), Dashboard (the settings singleton) and Site.
// None of the first five has a create verb in the ordinary REST sense --
// devices are provisioned by adopt, port overrides and settings live inside
// another object's write -- so their contracts have an empty
// CreateVerb/CreatePath. Site is the exception: it is a command, not a REST
// object, but the command that creates a site and the one that updates it
// share the same endpoint, so its contract carries a real CreateVerb/
// CreatePath equal to its UpdateVerb/UpdatePath.

// TestIntegrationDeviceWriteContract measures the Device write path
// (api/s/{site}/rest/device/{id}). There is no create: CreateDevice (POST
// api/s/{site}/stat/device) is confirmed here to answer 200 with the
// existing adopted device unchanged, and POST api/s/{site}/rest/device
// answers a hard 404 -- so this only measures the update side, on a real
// adopted gateway.
func TestIntegrationDeviceWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	devices := controllertest.StartDevices(ctx, t, c, controllertest.DeviceRequest{Model: controllertest.GatewayModel})
	if len(devices) != 1 {
		t.Skip("no emulated gateway available for this controller target")
	}
	adopted := c.AdoptDevice(ctx, t, s, devices[0].MAC)
	id := deviceIDForMAC(ctx, t, s, c.Site, adopted.MAC)
	if id == "" {
		t.Skipf("adopted %s but the controller lists no device with that MAC", adopted.MAC)
	}
	writePath := "/api/s/" + c.Site + "/rest/device/" + id
	statPath := "/api/s/" + c.Site + "/stat/device/" + adopted.MAC

	read := func() map[string]any {
		body, status, err := s.GetJSON(ctx, statPath)
		if err != nil || status != 200 {
			t.Fatalf("GET %s answered HTTP %d (%v)", statPath, status, err)
		}
		return firstData(t, body)
	}

	// Confirm there is no create verb: CreateDevice's own endpoint (POST
	// stat/device) is a no-op that returns the unchanged existing device,
	// and the REST collection root refuses a POST outright.
	before := read()
	body, status, err := s.PostJSON(ctx, statPath, map[string]any{"mac": "aa:bb:cc:dd:ee:ff", "name": "should-not-create"})
	if err != nil {
		t.Fatalf("POST %s transport: %v", statPath, err)
	}
	if status/100 == 2 {
		if got, _ := firstData(t, body)["mac"].(string); got == "aa:bb:cc:dd:ee:ff" {
			t.Error("LOUD: POST stat/device now creates a device from the posted body; CreateDevice may work after all")
		} else {
			t.Logf("POST %s answered HTTP 200 but echoed the existing device (mac=%s), not the posted body -- confirms no create", statPath, got)
		}
	} else {
		t.Logf("POST %s answered HTTP %d, not 2xx", statPath, status)
	}
	if after := read(); !jsonEqual(after["mac"], before["mac"]) || !jsonEqual(after["name"], before["name"]) {
		t.Errorf("the no-op CreateDevice POST changed the device: before mac=%v name=%v, after mac=%v name=%v",
			before["mac"], before["name"], after["mac"], after["name"])
	}
	if _, status, _ := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/device", map[string]any{"mac": "aa:bb:cc:dd:ee:ff"}); status == 404 {
		t.Logf("POST /api/s/{site}/rest/device answers HTTP 404 -- confirms no REST create either")
	} else {
		t.Logf("POST /api/s/{site}/rest/device answered HTTP %d (not the previously measured 404)", status)
	}

	// A representative, non-exhaustive sample of scalar fields, deliberately
	// including lcm_brightness -- the one that turns out dead on write.
	asked := map[string]any{
		"name": "device-probe-name", "led_override": "on",
		"lcm_brightness": 55, "lcm_brightness_override": true,
		"jumboframe_enabled": true, "disabled": false,
		"snmp_contact":          "probe-contact",
		"lcm_night_mode_begins": "22:00",
		"lcm_night_mode_ends":   "06:00",
	}
	// A write immediately after adopt can race the device's own settle: a
	// name-only write here was measured landing 200 with nothing changed on
	// the very first attempt, and clean on a retry a few seconds later. So
	// this applies the full write and confirms it landed before trusting
	// anything below, retrying rather than treating one race as the result.
	var stored map[string]any
	for attempt := 0; attempt < 5; attempt++ {
		if _, st, err := s.PutJSON(ctx, writePath, asked); st/100 != 2 {
			t.Fatalf("PUT %s answered HTTP %d (%v)", writePath, st, err)
		}
		stored = read()
		if got, _ := stored["name"].(string); got == "device-probe-name" {
			break
		}
		t.Logf("attempt %d: PUT answered 200 but name did not land yet (still %v); retrying after settle", attempt, stored["name"])
		time.Sleep(3 * time.Second)
	}
	if got, _ := stored["name"].(string); got != "device-probe-name" {
		t.Fatalf("PUT %s never landed after retries; name reads %v", writePath, stored["name"])
	}

	dropped := []string{}
	for _, r := range probe.Classify(asked, stored) {
		switch r.Verdict {
		case probe.Dropped:
			dropped = append(dropped, r.Wire)
			t.Logf("Device: DROPPED %-24s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Logf("Device: CHANGED %-24s (%s)", r.Wire, r.Detail)
		}
	}

	unknownKeyObserve(ctx, t, s, writePath, map[string]any{"name": stored["name"]})
	// unknownKeyObserve treats a 2xx with the key stripped as success and
	// deletes the "created" object by id, which does not apply to an
	// existing device -- re-read to make sure that probe write didn't
	// leave the unknown key or otherwise disturb the device.
	if _, present := read()[probeUnknownKey]; present {
		t.Errorf("PUT %s stored the unrecognised key; v1 was expected to strip it", writePath)
	}

	put := func(doc map[string]any) int {
		_, status, err := s.PutJSON(ctx, writePath, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", writePath, err)
		}
		return status
	}
	measured := map[string]behavior.EmptySemantics{
		"name":         storedEmptySemantics(t, "name", "", nil, stored, put, read),
		"led_override": storedEmptySemantics(t, "led_override", false, nil, stored, put, read),
	}

	contract := behavior.WriteContract{
		CreateVerb: "", CreatePath: "",
		UpdateVerb: "PUT", UpdatePath: "api/s/{site}/rest/device/{id}",
	}
	if behaviorWriteRequested() {
		recordWriteMergeEmpty(t, root, captured, "Device", "device", contract, dropped, measured)
		return
	}
	compareRecorded(t, root, running, "Device", "device", contract, dropped, measured)
}

// TestIntegrationDevicePortOverridesWriteContract adds the write contract
// and empty/omit half of what TestIntegrationDevicePortOverridesDiscard
// (behavior_probe_integration_test.go) already measured for the discard
// list: there is no create verb (a port override only exists inside a
// device's own PUT), and every field's omit is OMIT-CLEARS -- the resource's
// defining trait, since the array write replaces rather than merges at both
// the array and the member level. That replace is reconfirmed here directly
// (seed two ports, PUT naming only one, check the other), and it turned up
// something the discard probe did not need to know: a GET taken immediately
// after the write can still answer with the pre-write state for a couple of
// seconds before it catches up, which looked like a merge on one run of this
// probe and a replace on the next until the read was retried across a
// settle window instead of taken once.
func TestIntegrationDevicePortOverridesWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	emulated := controllertest.StartDevices(ctx, t, c, controllertest.DeviceRequest{Model: "USM8P"})
	if len(emulated) != 1 {
		t.Skip("no emulated switch available for this controller target")
	}
	adopted := c.AdoptDevice(ctx, t, s, emulated[0].MAC)
	id := deviceIDForMAC(ctx, t, s, c.Site, adopted.MAC)
	if id == "" {
		t.Skipf("adopted %s but the controller lists no device with that MAC", adopted.MAC)
	}
	devicePath := "/api/s/" + c.Site + "/rest/device/" + id

	putOverride := func(doc map[string]any) int {
		_, status, err := s.PutJSON(ctx, devicePath, map[string]any{"port_overrides": []any{doc}})
		if status == 0 {
			t.Fatalf("transport to %s: %v", devicePath, err)
		}
		return status
	}
	readOverride := func() map[string]any {
		return storedPortOverride(ctx, t, s, c.Site, adopted.MAC, 1)
	}

	seed := map[string]any{
		"port_idx": 1, "name": "po-probe", "autoneg": true,
		"isolation": true, "stp_port_mode": true, "poe_mode": "off",
	}
	// restoreSeed writes seed back and confirms it actually landed before
	// returning it, retrying a few times: a switch write can race its own
	// settle exactly like the Device probe measured after adopt, and an
	// unconfirmed read here previously came back nil, which made every
	// downstream empty-vs-omit check silently measure nothing instead of
	// failing loudly.
	restoreSeed := func() map[string]any {
		t.Helper()
		var got map[string]any
		for attempt := 0; attempt < 5; attempt++ {
			if st := putOverride(seed); st/100 != 2 {
				t.Fatalf("restoring port_idx=1 to its seed was rejected (HTTP %d)", st)
			}
			got = readOverride()
			if got != nil && jsonEqual(got["name"], "po-probe") {
				return got
			}
			t.Logf("attempt %d: port_idx=1 did not read back as seeded yet (got %v); retrying after settle", attempt, got)
			time.Sleep(2 * time.Second)
		}
		t.Fatalf("port_idx=1 never read back as seeded after retries (last read: %v)", got)
		return nil
	}
	seeded := restoreSeed()

	// Array-level replace vs merge, checked directly: seed port_idx=1 AND
	// port_idx=2 together in one write (so the starting state genuinely has
	// both, rather than each write's own single-element array trivially
	// evicting whatever came before), then PUT naming only port_idx=1 and
	// see whether port_idx=2's custom value survives.
	if _, st, err := s.PutJSON(ctx, devicePath, map[string]any{
		"port_overrides": []any{seed, map[string]any{"port_idx": 2, "name": "po-probe-2"}},
	}); st/100 != 2 {
		t.Fatalf("seeding both port_idx=1 and port_idx=2 together was rejected (HTTP %d): %v", st, err)
	}
	// Same settle window as restoreSeed: this read can race the write it
	// just made, especially under concurrent load from other containers on
	// the same host.
	var before2 map[string]any
	for attempt := 0; attempt < 5; attempt++ {
		before2 = storedPortOverride(ctx, t, s, c.Site, adopted.MAC, 2)
		if before2 != nil {
			break
		}
		t.Logf("attempt %d: port_idx=2 not visible yet after seeding it alongside port_idx=1; retrying after settle", attempt)
		time.Sleep(2 * time.Second)
	}
	if before2 == nil {
		t.Fatalf("port_idx=2 never appeared after seeding it alongside port_idx=1: both should be present")
	}
	putOverride(seed) // PUT naming only port_idx=1
	// Read a few times with a settle delay before trusting the answer: an
	// immediate read here came back still showing port_idx=2's full
	// content on one run, and empty a few seconds later on another --
	// which turned out to be the same thing observed at different points
	// of one settle window, not a nondeterministic result. The read right
	// after the PUT can be stale for a couple of seconds; the value once it
	// stops changing is the one this records.
	var reads []map[string]any
	for i := 0; i < 4; i++ {
		reads = append(reads, storedPortOverride(ctx, t, s, c.Site, adopted.MAC, 2))
		if i < 3 {
			time.Sleep(2 * time.Second)
		}
	}
	for i, r := range reads {
		t.Logf("port_idx=2 read %d after the port_idx=1-only PUT: %v", i, r)
	}
	after2, settled := reads[len(reads)-1], reads[len(reads)-1] == nil
	for _, r := range reads[1:] {
		if (r == nil) != settled {
			t.Log("LOUD: port_idx=2's presence kept changing across the whole settle window, not just " +
				"the first read; the eventual-consistency explanation below may not be the whole story")
		}
	}
	switch {
	case after2 == nil && reads[0] != nil:
		t.Log("PUT naming only port_idx=1 does replace the whole array and wipe port_idx=2, matching the " +
			"prior finding -- but a read taken immediately after the write can still show the pre-write " +
			"state for a couple of seconds before it catches up. A caller reading right back after this " +
			"write cannot trust what it sees for a couple of seconds.")
	case after2 == nil:
		t.Logf("PUT naming only port_idx=1 wiped port_idx=2 entirely -- array-level replace, not merge")
	case jsonEqual(after2["name"], "po-probe-2"):
		t.Logf("LOUD: port_idx=2's custom name survived the whole settle window after a PUT naming only "+
			"port_idx=1 (before=%v after=%v). This contradicts a prior finding that this write replaces "+
			"the whole port_overrides array: on this controller it merges by port_idx.", before2, after2)
	default:
		t.Logf("port_idx=2 still has an entry after the wipe (%v), but its custom name is gone -- looks like "+
			"the switch reports a default stub for every physical port regardless of what was written, "+
			"not a merge of the previously written override", after2)
	}
	seeded = restoreSeed()

	// port_idx pattern validation: 99 is out of the wire's 1-56 range.
	if st := putOverride(map[string]any{"port_idx": 99, "name": "po-bad-idx"}); st/100 == 2 {
		t.Error("port_idx=99 was accepted; the InvalidValue pattern rejection finding no longer holds")
	} else {
		t.Logf("port_idx=99 rejected (HTTP %d) -- pattern-validated against 1-56", st)
	}
	restoreSeed() // restore

	// forward:"native" coercion, matching the portconf family's known
	// forward-field quirk.
	if st := putOverride(map[string]any{"port_idx": 1, "name": "po-forward-probe", "forward": "native"}); st/100 == 2 {
		if got, _ := readOverride()["forward"].(string); got != "native" {
			t.Logf("forward:\"native\" on a port override was silently coerced to %q on write, "+
				"consistent with the same coercion measured on PortProfile.forward", got)
		}
	}
	restoreSeed() // restore

	// port_idx omitted: accepted (200) but the entry silently fails to
	// persist, and because the write also replaces the array, this can wipe
	// a different port's override entirely.
	if st := putOverride(map[string]any{"name": "po-no-idx"}); st/100 != 2 {
		t.Logf("an override with no port_idx was rejected (HTTP %d) -- stricter than previously measured", st)
	} else if storedPortOverride(ctx, t, s, c.Site, adopted.MAC, 1) != nil {
		t.Log("LOUD: an override with no port_idx did not wipe port_idx=1's entry; the finding may no longer hold")
	} else {
		t.Log("an override with no port_idx answers 200 but persists nothing, and wiped port_idx=1's " +
			"entry via the array-level replace -- confirms the finding rather than a clean requiredness fact")
	}
	restoreSeed() // restore

	measured := map[string]behavior.EmptySemantics{
		"name":          storedEmptySemantics(t, "name", "", nil, seeded, putOverride, readOverride),
		"autoneg":       storedEmptySemantics(t, "autoneg", false, nil, seeded, putOverride, readOverride),
		"isolation":     storedEmptySemantics(t, "isolation", false, nil, seeded, putOverride, readOverride),
		"stp_port_mode": storedEmptySemantics(t, "stp_port_mode", false, nil, seeded, putOverride, readOverride),
		"poe_mode":      storedEmptySemantics(t, "poe_mode", "", nil, seeded, putOverride, readOverride),
	}

	contract := behavior.WriteContract{
		CreateVerb: "", CreatePath: "",
		UpdateVerb: "PUT", UpdatePath: "api/s/{site}/rest/device/{id}",
	}
	if behaviorWriteRequested() {
		// Discarded is left untouched (nil): TestIntegrationDevicePortOverridesDiscard
		// already owns that list, and this probe measures the write
		// contract and empty/omit facts alongside it.
		// The Empty section is keyed "DevicePortOverrides" (the Go type
		// name), not a lowercase collection alias: cmd/wirecontract only
		// registers an api_paths alias for a type when one comes from the
		// schema capture, and this nested type carries none.
		recordWrite(t, root, captured, "DevicePortOverrides", "DevicePortOverrides", contract, nil, measured)
		return
	}
	compareRecorded(t, root, running, "DevicePortOverrides", "DevicePortOverrides", contract, nil, measured)
}

// TestIntegrationPortProfileWriteContract measures the port profile
// (api/s/{site}/rest/portconf). No field is required on create -- a
// completely empty body ({}) is confirmed to create fine, in addition to
// the field-by-field sweep. fec_mode and excluded_networkconf_ids are
// re-confirmed dead on write (accepted, absent on re-read), and
// dot1x_ctrl's empty-vs-omit pair is added to the portconf Empty section
// alongside what a prior probe already measured there.
func TestIntegrationPortProfileWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	networkID := defaultLANNetworkID(ctx, t, s, c.Site)
	path := "/api/s/" + c.Site + "/rest/portconf"

	base := func(name string) map[string]any {
		doc := map[string]any{
			"name": name, "forward": "customize", "poe_mode": "auto",
			"dot1x_ctrl": "force_authorized", "autoneg": true,
			"stormctrl_type": "level", "stormctrl_ucast_enabled": true,
			"lldpmed_enabled": true, "stp_port_mode": true,
			"setting_preference": "manual", "fec_mode": "disabled",
			"op_mode": "switch",
		}
		if networkID != "" {
			doc["native_networkconf_id"] = networkID
			doc["excluded_networkconf_ids"] = []any{networkID}
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

	asked := base("portprofile-probe")
	body, status, err := s.PostJSON(ctx, path, asked)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		t.Fatalf("the known-good PortProfile body was rejected (HTTP %d): %v", status, firstData(t, body))
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
			t.Logf("PortProfile: DROPPED %-24s (%s)", r.Wire, r.Detail)
		case probe.Changed:
			t.Logf("PortProfile: CHANGED %-24s (%s) -- stored, not discarded", r.Wire, r.Detail)
		}
	}

	// A literally empty body creates fine -- the strongest form of "nothing
	// is required", checked directly rather than only inferred from the
	// field-by-field sweep below.
	if status, body, _ := post(map[string]any{}); status/100 != 2 {
		t.Errorf("a completely empty PortProfile body was rejected (HTTP %d): %v", status, body)
	} else {
		t.Logf("an empty PortProfile body ({}) creates fine (HTTP %d) -- no field is required", status)
	}

	unknownKeyObserve(ctx, t, s, path, base("portprofile-unknown-key"))
	if _, listStatus, err := s.GetJSON(ctx, path); listStatus != 200 {
		t.Errorf("GET %s answered HTTP %d (%v)", path, listStatus, err)
	}

	renamed := clone(stored)
	renamed["name"] = "portprofile-probe-renamed"
	updateVerb, updatePath := "", ""
	if after, putStatus, err := s.PutJSON(ctx, path+"/"+id, renamed); putStatus/100 != 2 {
		t.Errorf("PUT %s/%s answered HTTP %d (%v %v)", path, id, putStatus, after, err)
	} else if got, _ := read()["name"].(string); got != "portprofile-probe-renamed" {
		t.Errorf("PUT %s/{id} answered HTTP %d but name still reads %q", path, putStatus, got)
	} else {
		updateVerb, updatePath = "PUT", "api/s/{site}/rest/portconf/{id}"
	}
	s.PutJSON(ctx, path+"/"+id, clone(stored)) //nolint:errcheck // restore

	required := requiredFieldSweep(t, sortedWireNames(asked), func(field string) (int, any) {
		doc := base("portprofile-req-" + field)
		delete(doc, field)
		status, body, _ := post(doc)
		return status, body
	})

	// multicast_router_mode=CUSTOM naming the site's only network is
	// rejected outright -- a refused-if-present rule under this branch,
	// not a requiredness fact, so it is asserted rather than recorded.
	if networkID != "" {
		doc := base("portprofile-multicast-custom")
		doc["multicast_router_mode"] = "CUSTOM"
		doc["multicast_router_networkconf_ids"] = []any{networkID}
		if status, body, _ := post(doc); status/100 == 2 {
			t.Log("LOUD: multicast_router_mode=CUSTOM naming the default LAN network is now accepted; " +
				"the InvalidMulticastRouterPort finding no longer holds on this controller")
		} else {
			t.Logf("multicast_router_mode=CUSTOM naming the default LAN network rejected (HTTP %d, %s)",
				status, v1ErrCode(body))
		}
	}

	put := func(doc map[string]any) int {
		_, status, err := s.PutJSON(ctx, path+"/"+id, doc)
		if status == 0 {
			t.Fatalf("transport to %s/%s: %v", path, id, err)
		}
		return status
	}
	// dot1x_ctrl is the one field this probe adds to the portconf Empty
	// section; forward/name/native_networkconf_id/op_mode/poe_mode/
	// setting_preference/stormctrl_type are already pinned there.
	newEmpties := map[string]behavior.EmptySemantics{
		"dot1x_ctrl": storedEmptySemantics(t, "dot1x_ctrl", "", nil, stored, put, read),
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "api/s/{site}/rest/portconf",
		UpdateVerb: updateVerb, UpdatePath: updatePath,
		RequiredOnCreate: required,
	}
	if behaviorWriteRequested() {
		recordWriteMergeEmpty(t, root, captured, "PortProfile", "portconf", contract, dropped, newEmpties)
		return
	}
	compareRecorded(t, root, running, "PortProfile", "portconf", contract, dropped, newEmpties)
}

// TestIntegrationSettingMgmtWriteContract measures the mgmt settings
// singleton (api/s/{site}/set/setting/mgmt). There is no create -- the key
// pre-exists on every site -- so only the update side is measured.
// x_ssh_username/x_ssh_password both default to "admin" on a fresh site,
// so both are seeded with sentinel values before the empty-vs-omit pass, or
// a re-defaulted value could not be told from a preserved one.
func TestIntegrationSettingMgmtWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	getPath := "/api/s/" + c.Site + "/get/setting/mgmt"
	putPath := "/api/s/" + c.Site + "/set/setting/mgmt"

	read := func() map[string]any {
		body, status, err := s.GetJSON(ctx, getPath)
		if err != nil || status != 200 {
			t.Fatalf("GET %s answered HTTP %d (%v)", getPath, status, err)
		}
		return firstData(t, body)
	}
	put := func(doc map[string]any) int {
		_, status, err := s.PutJSON(ctx, putPath, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", putPath, err)
		}
		return status
	}

	original := read()
	unknownKeyObserve(ctx, t, s, putPath, map[string]any{"key": "mgmt"})

	seed := clone(original)
	seed["key"] = "mgmt"
	seed["x_ssh_username"] = "probe-user-abc12"
	seed["x_ssh_password"] = "Probe$Pass!456"
	if st := put(seed); st/100 != 2 {
		t.Fatalf("seeding x_ssh_username/x_ssh_password was rejected (HTTP %d)", st)
	}
	seeded := read()

	measured := map[string]behavior.EmptySemantics{
		"x_ssh_username": storedEmptySemantics(t, "x_ssh_username", "", nil, seeded, put, read),
		"x_ssh_password": storedEmptySemantics(t, "x_ssh_password", "", nil, seeded, put, read),
	}

	// The merge itself, directly: a PUT naming only key+x_ssh_username
	// leaves auto_upgrade and the SSH password untouched.
	afterSeed := read()
	partial := map[string]any{"key": "mgmt", "x_ssh_username": "probe-user-renamed"}
	if st := put(partial); st/100 != 2 {
		t.Errorf("a partial PUT naming only x_ssh_username was rejected (HTTP %d)", st)
	} else {
		after := read()
		if !jsonEqual(after["x_ssh_password"], afterSeed["x_ssh_password"]) {
			t.Errorf("x_ssh_password did not survive a partial PUT that omitted it: was %v, now %v",
				afterSeed["x_ssh_password"], after["x_ssh_password"])
		} else {
			t.Log("a partial PUT naming only key+x_ssh_username left x_ssh_password untouched -- merges")
		}
	}

	// Restore admin/admin: this is the only site the container has, and
	// nothing else in this probe depends on knowing the secret material.
	restore := clone(read())
	restore["x_ssh_username"] = "admin"
	restore["x_ssh_password"] = "admin"
	put(restore) //nolint:errcheck

	contract := behavior.WriteContract{
		CreateVerb: "", CreatePath: "",
		UpdateVerb: "PUT", UpdatePath: "api/s/{site}/set/setting/mgmt",
	}
	if behaviorWriteRequested() {
		// settings.Mgmt carries no api_paths alias in the wire contract
		// (only generated: true types backed by a schema capture get one),
		// so the Empty section has to key on SettingMgmt itself.
		recordWrite(t, root, captured, "SettingMgmt", "SettingMgmt", contract, nil, measured)
		return
	}
	compareRecorded(t, root, running, "SettingMgmt", "SettingMgmt", contract, nil, measured)
}

// TestIntegrationSettingDashboardWriteContract measures the dashboard
// settings singleton (api/s/{site}/set/setting/dashboard). widgets is owned
// by layout_preference: it is only writable/retained while
// layout_preference="manual" is set in the same request, and is cleared (or
// never even ignored -- just absent) whenever layout_preference is "auto".
// This is measured across five consecutive writes rather than a single
// create, since a prior finding about this field's write behaviour turned
// out to depend on layout_preference and not on write order at all.
func TestIntegrationSettingDashboardWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	getPath := "/api/s/" + c.Site + "/get/setting/dashboard"
	putPath := "/api/s/" + c.Site + "/set/setting/dashboard"

	read := func() map[string]any {
		body, status, err := s.GetJSON(ctx, getPath)
		if err != nil || status != 200 {
			t.Fatalf("GET %s answered HTTP %d (%v)", getPath, status, err)
		}
		return firstData(t, body)
	}
	put := func(doc map[string]any) int {
		_, status, err := s.PutJSON(ctx, putPath, doc)
		if status == 0 {
			t.Fatalf("transport to %s: %v", putPath, err)
		}
		return status
	}

	pre := read()
	t.Logf("dashboard singleton pre-exists with no write: layout_preference=%v widgets=%v",
		pre["layout_preference"], pre["widgets"])

	unknownKeyObserve(ctx, t, s, putPath, map[string]any{"key": "dashboard"})

	widget := func(name string) []any {
		return []any{map[string]any{"name": name, "enabled": true}}
	}

	// Five consecutive writes under layout_preference=manual: set, change,
	// resend identical, and omit widgets entirely. Every one should persist
	// or merge -- none should silently discard.
	steps := []struct {
		label string
		doc   map[string]any
		check func(after map[string]any)
	}{
		{"set", map[string]any{"key": "dashboard", "layout_preference": "manual", "widgets": widget("cybersecure")},
			func(after map[string]any) {
				if !jsonEqual(after["widgets"], widget("cybersecure")) {
					t.Errorf("widgets did not persist on first manual write: %v", after["widgets"])
				}
			}},
		{"change", map[string]any{"key": "dashboard", "layout_preference": "manual", "widgets": widget("wan_activity")},
			func(after map[string]any) {
				if !jsonEqual(after["widgets"], widget("wan_activity")) {
					t.Errorf("widgets did not change on second manual write: %v", after["widgets"])
				}
			}},
		{"resend identical", map[string]any{"key": "dashboard", "layout_preference": "manual", "widgets": widget("wan_activity")},
			func(after map[string]any) {
				if !jsonEqual(after["widgets"], widget("wan_activity")) {
					t.Errorf("widgets did not survive an identical resend: %v", after["widgets"])
				}
			}},
		{"omit widgets, still manual", map[string]any{"key": "dashboard", "layout_preference": "manual"},
			func(after map[string]any) {
				if !jsonEqual(after["widgets"], widget("wan_activity")) {
					t.Errorf("widgets did not survive omission under layout_preference=manual: %v", after["widgets"])
				}
			}},
	}
	for _, step := range steps {
		if st := put(step.doc); st/100 != 2 {
			t.Fatalf("write %q was rejected (HTTP %d)", step.label, st)
		}
		step.check(read())
	}
	t.Log("widgets is never silently dropped across five writes while layout_preference stays manual -- " +
		"this contradicts a prior finding that widgets is accepted-once-then-discarded")

	// Switching to auto clears/ignores widgets even when the same write
	// still carries a widgets key.
	if st := put(map[string]any{"key": "dashboard", "layout_preference": "auto", "widgets": widget("wifi_technology")}); st/100 != 2 {
		t.Fatalf("switching to layout_preference=auto was rejected (HTTP %d)", st)
	}
	afterAuto := read()
	widgetsGone := afterAuto["widgets"] == nil
	if list, ok := afterAuto["widgets"].([]any); ok && len(list) == 0 {
		widgetsGone = true
	}
	if !widgetsGone {
		t.Errorf("widgets survived a write under layout_preference=auto: %v", afterAuto["widgets"])
	} else {
		t.Log("layout_preference=auto clears/ignores widgets even in the same write that sends them")
	}

	// Re-entering manual with a new widgets value works again, repeatably.
	if st := put(map[string]any{"key": "dashboard", "layout_preference": "manual", "widgets": widget("ap_radio_density")}); st/100 != 2 {
		t.Fatalf("re-entering layout_preference=manual was rejected (HTTP %d)", st)
	}
	if after := read(); !jsonEqual(after["widgets"], widget("ap_radio_density")) {
		t.Errorf("widgets did not persist on re-entering manual: %v", after["widgets"])
	} else {
		t.Log("re-entering layout_preference=manual with a new widgets value accepts and persists it again")
	}

	measured := map[string]behavior.EmptySemantics{
		"layout_preference": storedEmptySemantics(t, "layout_preference", "", nil, read(), put, read),
	}

	contract := behavior.WriteContract{
		CreateVerb: "", CreatePath: "",
		UpdateVerb: "PUT", UpdatePath: "api/s/{site}/set/setting/dashboard",
	}
	if behaviorWriteRequested() {
		// "dashboard" is already claimed as an api_paths alias by the
		// unrelated unifi.Dashboard REST-collection type (a separate Go
		// type this file does not measure); keying this section on that
		// string would silently attribute the settings singleton's data to
		// the wrong type. SettingDashboard carries no alias of its own, so
		// this uses the type name instead.
		recordWrite(t, root, captured, "SettingDashboard", "SettingDashboard", contract, nil, measured)
		return
	}
	compareRecorded(t, root, running, "SettingDashboard", "SettingDashboard", contract, nil, measured)
}

// TestIntegrationSiteWriteContract measures the site command surface
// (api/s/{site}/cmd/sitemgr): add-site, update-site and delete-site. Uses a
// site this test creates and deletes itself, never the harness's default
// one. delete-site is confirmed to need the site's own _id, not its
// name/slug -- DeleteSite's own doc comment is checked against the
// controller rather than trusted.
//
// Site is now generated (unifi/site.generated.go), so this records into
// the artifact: create and update share one command endpoint (there is no
// per-id path), desc is the only field either command writes, and it is
// optional on both -- add-site with no desc gets the controller's own
// default, and update-site with no desc key clears it exactly like "" does.
func TestIntegrationSiteWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	cmdPath := "/api/s/" + c.Site + "/cmd/sitemgr"

	// add-site without desc: accepted, and the controller supplies its own
	// default description.
	body, status, err := s.PostJSON(ctx, cmdPath, map[string]any{"cmd": "add-site"})
	if status == 0 || status/100 != 2 {
		t.Fatalf("add-site without desc was rejected (HTTP %d): %v %v", status, body, err)
	}
	created := firstData(t, body)
	newSiteID, _ := created["_id"].(string)
	newSiteName, _ := created["name"].(string)
	if newSiteID == "" || newSiteName == "" {
		t.Fatalf("add-site returned no usable id/name: %v", created)
	}
	t.Logf("add-site without desc created %s (id=%s) with desc=%q -- desc is not required on create",
		newSiteName, newSiteID, created["desc"])
	defer s.PostJSON(ctx, cmdPath, map[string]any{"cmd": "delete-site", "site": newSiteID}) //nolint:errcheck

	// unknown key on add-site is silently stripped.
	body2, status2, err2 := s.PostJSON(ctx, cmdPath, map[string]any{"cmd": "add-site", "desc": "unknown-key-probe", probeUnknownKey: "x"})
	if status2 == 0 {
		t.Fatalf("transport: %v", err2)
	}
	if status2/100 == 2 {
		unkSiteID, _ := firstData(t, body2)["_id"].(string)
		if unkSiteID != "" {
			defer s.PostJSON(ctx, cmdPath, map[string]any{"cmd": "delete-site", "site": unkSiteID}) //nolint:errcheck
		}
		if _, present := firstData(t, body2)[probeUnknownKey]; present {
			t.Error("add-site stored the unrecognised key; expected it stripped")
		} else {
			t.Log("add-site accepts an unrecognised key and stores the site without it")
		}
	} else {
		t.Errorf("add-site with an unrecognised key was rejected (HTTP %d)", status2)
	}

	newSitePath := "/api/s/" + newSiteName + "/cmd/sitemgr"

	// update-site addresses the TARGET site's own slug in the URL, and
	// desc is the only field it writes.
	if _, st, err := s.PostJSON(ctx, newSitePath, map[string]any{"cmd": "update-site", "desc": "probe-renamed"}); st/100 != 2 {
		t.Fatalf("update-site rejected (HTTP %d): %v", st, err)
	}
	sites, err := harnessClient(ctx, t, c).ListSites(ctx)
	if err != nil {
		t.Fatalf("ListSites: %v", err)
	}
	found := false
	for _, site := range sites {
		if site.ID == newSiteID {
			found = true
			if site.Description != "probe-renamed" {
				t.Errorf("update-site did not land: desc=%q", site.Description)
			}
		}
	}
	if !found {
		t.Fatalf("the created site is not in ListSites")
	}

	// OMIT-CLEARS on update-site: leaving desc out of the command body
	// clears it, the same as sending "" -- unlike every v1 REST resource's
	// merging PUT, because update-site is a single-field command, not a
	// masked merge.
	if _, st, err := s.PostJSON(ctx, newSitePath, map[string]any{"cmd": "update-site", "desc": ""}); st/100 != 2 {
		t.Fatalf("update-site with desc=\"\" rejected (HTTP %d): %v", st, err)
	}
	emptyDesc := siteDescByID(ctx, t, c, newSiteID)
	if _, st, err := s.PostJSON(ctx, newSitePath, map[string]any{"cmd": "update-site", "desc": "probe-renamed-2"}); st/100 != 2 {
		t.Fatalf("update-site rejected (HTTP %d): %v", st, err)
	}
	if _, st, err := s.PostJSON(ctx, newSitePath, map[string]any{"cmd": "update-site"}); st/100 != 2 {
		t.Fatalf("update-site with no desc key rejected (HTTP %d): %v", st, err)
	}
	omitDesc := siteDescByID(ctx, t, c, newSiteID)
	measured := map[string]behavior.EmptySemantics{
		"desc": {
			Empty: ternary(emptyDesc == "", "EMPTY-CLEARS", "EMPTY-REPLACED-"+emptyDesc),
			Omit:  ternary(omitDesc == "", "OMIT-CLEARS", "OMIT-KEEPS"),
		},
	}
	t.Logf("update-site: desc=\"\" -> %q, omitted desc -> %q", emptyDesc, omitDesc)

	// delete-site requires the _id, not the name/slug -- DeleteSite's own
	// argument is checked against this directly.
	if _, st, _ := s.PostJSON(ctx, cmdPath, map[string]any{"cmd": "delete-site", "site": newSiteName}); st/100 == 2 {
		t.Error("LOUD: delete-site now accepts the site's name/slug; DeleteSite's doc comment may be stale")
	} else {
		t.Logf("delete-site with the site's name/slug rejected (HTTP %d) -- confirms it needs the _id", st)
	}
	if _, st, err := s.PostJSON(ctx, cmdPath, map[string]any{"cmd": "delete-site", "site": newSiteID}); st/100 != 2 {
		t.Fatalf("delete-site with the real _id was rejected (HTTP %d): %v", st, err)
	}

	// Site is now generated (unifi/site.generated.go, from
	// overrides/resources/Site.json), so its resource key is claimed in the
	// wire contract and this lands in the artifact same as any other
	// resource -- create, update and delete all share one command endpoint,
	// which the generic verb/path shape represents as identical create and
	// update paths.
	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "api/s/{site}/cmd/sitemgr",
		UpdateVerb: "POST", UpdatePath: "api/s/{site}/cmd/sitemgr",
	}
	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "Site", "Site", contract, nil, measured)
		return
	}
	compareRecorded(t, root, running, "Site", "Site", contract, nil, measured)
}

// siteDescByID reads one site's desc by id via ListSites. t.Fatal if the
// site is missing entirely -- a caller checking desc after a write needs the
// site to still exist, and a silent "" for "not found" would be
// indistinguishable from a genuinely cleared desc.
func siteDescByID(ctx context.Context, t *testing.T, c *controllertest.Controller, id string) string {
	t.Helper()
	sites, err := harnessClient(ctx, t, c).ListSites(ctx)
	if err != nil {
		t.Fatalf("ListSites: %v", err)
	}
	for _, site := range sites {
		if site.ID == id {
			return site.Description
		}
	}
	t.Fatalf("site id %s not found in ListSites", id)
	return ""
}

// ternary is a tiny helper so the two update-site verdicts above read as one
// expression apiece instead of an if/else block each.
func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
