//go:build integration

// unifi/behavior_probe_batch5_integration_test.go
package unifi

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// Batch 5 of the write-contract re-measurement: BGPConfig, FirewallZone and
// WireGuardPeer, plus a re-confirmation pass on the three resources a prior
// session could not measure at all (PowerSupervisor, BroadcastGroup,
// MediaFile) -- these three record nothing into the artifact, since nothing
// here measures anything new about them; the point is to check the
// unmeasurability itself still holds rather than carry it forward
// unverified.

// TestIntegrationBGPConfigWriteContract measures the BGP config
// (v2/api/site/{site}/bgp/config), gated on a BGP-capable gateway being
// adopted -- every write answers api.err.BgpUnsupportedDevice (404) until
// one is, which this measures directly before doing anything else. It is a
// plain JSON POST despite uploaded_file_name's name: no multipart upload is
// involved.
func TestIntegrationBGPConfigWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/v2/api/site/" + c.Site + "/bgp/config"

	base := func() map[string]any {
		return map[string]any{
			"frr_bgpd_config":    "router bgp 65000\n bgp router-id 192.0.2.1\n",
			"uploaded_file_name": "probe.conf", "enabled": true,
			"description": "bgp-config-probe",
		}
	}

	if _, status, err := s.PostJSON(ctx, path, base()); status == 404 {
		t.Logf("POST %s without a BGP-capable gateway answers HTTP 404 (api.err.BgpUnsupportedDevice) "+
			"-- confirms the device-capability gate fires before any field validation", path)
	} else {
		t.Logf("POST %s without a gateway answered HTTP %d (%v), not the previously measured 404", path, status, err)
	}

	devices := controllertest.StartDevices(ctx, t, c, controllertest.DeviceRequest{Model: controllertest.GatewayModel})
	if len(devices) != 1 {
		t.Skip("no emulated gateway available for this controller target")
	}
	c.AdoptDevice(ctx, t, s, devices[0].MAC)

	asked := base()
	body, status, err := s.PostJSON(ctx, path, asked)
	if status == 0 {
		t.Fatalf("transport to %s: %v", path, err)
	}
	if status/100 != 2 {
		t.Fatalf("the known-good BGPConfig body was rejected (HTTP %d) even with a gateway adopted: %v",
			status, firstData(t, body))
	}

	read := func() map[string]any {
		got, status, err := s.GetJSON(ctx, path)
		if err != nil || status != 200 {
			t.Fatalf("GET %s answered HTTP %d (%v)", path, status, err)
		}
		return firstData(t, got)
	}
	stored := read()
	if !jsonEqual(stored["frr_bgpd_config"], asked["frr_bgpd_config"]) {
		t.Errorf("frr_bgpd_config did not persist: %v", stored["frr_bgpd_config"])
	}

	unknownKeyObserve(ctx, t, s, path, map[string]any{
		"frr_bgpd_config": asked["frr_bgpd_config"], "uploaded_file_name": "unknown-key-probe.conf",
	})

	required := requiredFieldSweep(t, []string{"frr_bgpd_config", "uploaded_file_name"}, func(field string) (int, any) {
		doc := base()
		delete(doc, field)
		body, status, err := s.PostJSON(ctx, path, doc)
		if status == 0 {
			t.Fatalf("transport: %v", err)
		}
		return status, body
	})

	description := storedEmptySemantics(t, "description", "", nil, stored,
		func(doc map[string]any) int {
			doc["frr_bgpd_config"] = asked["frr_bgpd_config"]
			doc["uploaded_file_name"] = asked["uploaded_file_name"]
			_, status, err := s.PostJSON(ctx, path, doc)
			if status == 0 {
				t.Fatalf("transport to %s: %v", path, err)
			}
			return status
		}, read)

	// Enabled has no omitempty, and this endpoint replaces rather than
	// merges: omitting it from a write resets it to false.
	withoutEnabled := base()
	delete(withoutEnabled, "enabled")
	if _, status, err := s.PostJSON(ctx, path, withoutEnabled); status/100 != 2 {
		t.Fatalf("re-post without enabled was rejected (HTTP %d): %v", status, err)
	}
	if got, _ := read()["enabled"].(bool); got {
		t.Error("enabled survived a write that omitted it; expected it reset to false (replace, not merge)")
	} else {
		t.Log("enabled resets to false when omitted -- confirms this endpoint replaces rather than merges")
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "v2/api/site/{site}/bgp/config",
		UpdateVerb: "POST", UpdatePath: "v2/api/site/{site}/bgp/config",
		RequiredOnCreate: required,
	}
	measured := map[string]behavior.EmptySemantics{"description": description}
	if behaviorWriteRequested() {
		// The Empty section keys on the wire contract's own api_paths alias
		// for this type ("bgp/config", the literal path segment), not a
		// shorthand -- a mismatched key here fails go generate outright
		// (cmd/wirecontract has no type registered under a bare "bgp").
		recordWrite(t, root, captured, "BGPConfig", "bgp/config", contract, nil, measured)
		return
	}
	compareRecorded(t, root, running, "BGPConfig", "bgp/config", contract, nil, measured)
}

// TestIntegrationFirewallZoneWriteContract re-measures the firewall zone
// (v2/api/site/{site}/firewall/zone). A prior measurement found every write
// verb answering api.err.CouldNotFindHotspotFirewallZone unconditionally,
// both before and after adopting a gateway, and found list/persistence
// behaviour unstable enough that no required-on-create set could be
// published. This checks whether that still holds rather than assuming it
// does, and records nothing beyond the create/update verb and path if it
// does -- there is nothing else this controller lets it measure.
func TestIntegrationFirewallZoneWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	root, captured, running := behaviorGate(ctx, t, s, c.Site)

	path := "/v2/api/site/" + c.Site + "/firewall/zone"
	base := map[string]any{"name": "firewallzone-probe", "network_ids": []any{}}

	preBody, preStatus, _ := s.PostJSON(ctx, path, base)
	t.Logf("POST %s before any gateway is adopted: HTTP %d (%v)", path, preStatus, v2ErrCode(preBody))

	devices := controllertest.StartDevices(ctx, t, c, controllertest.DeviceRequest{Model: controllertest.GatewayModel})
	if len(devices) != 1 {
		t.Skip("no emulated gateway available for this controller target")
	}
	c.AdoptDevice(ctx, t, s, devices[0].MAC)

	postBody, postStatus, _ := s.PostJSON(ctx, path, base)
	t.Logf("POST %s after a gateway is adopted: HTTP %d (%v)", path, postStatus, v2ErrCode(postBody))

	unknownBody, unknownStatus, unknownErr := s.PostJSON(ctx, path, map[string]any{"name": "x", probeUnknownKey: "y"})
	if unknownStatus/100 == 2 {
		if id := objectID(firstData(t, unknownBody)); id != "" {
			s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
		}
	}
	t.Logf("POST %s with an unrecognised key: HTTP %d (%v, %v) -- logged rather than checked against "+
		"either general v2 rule, since a create that 404s for every body proves nothing about unknown-key "+
		"handling specifically", path, unknownStatus, v2Rejection(unknownBody), unknownErr)

	if postStatus/100 == 2 {
		t.Logf("LOUD: firewall zone create now succeeds (HTTP %d) with a gateway adopted; the "+
			"CouldNotFindHotspotFirewallZone finding no longer holds and this resource can be fully "+
			"measured now", postStatus)
		id := objectID(firstData(t, postBody))
		if id != "" {
			after, listStatus, _ := s.GetJSON(ctx, path)
			t.Logf("GET %s after a successful create: HTTP %d, %v", path, listStatus, after)
		}
	} else if postStatus == preStatus {
		t.Logf("firewall zone create answers the identical HTTP %d both before and after adopting a "+
			"gateway -- reconfirms the write path is gated on something this controller never satisfies, "+
			"not on gateway presence alone", postStatus)
	} else {
		t.Logf("LOUD: firewall zone create's answer CHANGED with a gateway adopted (%d -> %d); the "+
			"prior finding that adoption changes nothing about the write path may need revisiting",
			preStatus, postStatus)
	}

	contract := behavior.WriteContract{
		CreateVerb: "POST", CreatePath: "v2/api/site/{site}/firewall/zone",
		UpdateVerb: "PUT", UpdatePath: "v2/api/site/{site}/firewall/zone/{id}",
	}
	if behaviorWriteRequested() {
		recordWrite(t, root, captured, "FirewallZone", "firewallzone", contract, nil, nil)
		return
	}
	compareRecorded(t, root, running, "FirewallZone", "firewallzone", contract, nil, nil)
}

// randomWireGuardKey returns a fresh base64-shaped 32-byte key, so create
// sweeps never collide on a previously-used one (PublicKeyUsedByAnotherUser
// masquerading as a required-field rejection is exactly the trap a prior
// measurement of this resource fell into and had to work around).
func randomWireGuardKey(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("crypto/rand: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// TestIntegrationWireGuardPeerWriteContract measures the WireGuard peer
// batch endpoints (v2/api/site/{site}/wireguard/{network_id}/users/batch).
// Every field -- name, interface_ip, public_key, allowed_ips -- is
// individually omittable from a create; the batch PUT does not merge a
// partial body (measured elsewhere as a bare HTTP 500), and omitting a
// previously-set key from a full-array PUT clears it exactly like sending
// "" does, so update is a replace by the field-level as well as the
// array-level definition.
func TestIntegrationWireGuardPeerWriteContract(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 30*time.Minute)
	_, _, _ = behaviorGate(ctx, t, s, c.Site) // gates the controller version; nothing here writes to the artifact -- see below

	body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", map[string]any{
		"name": "wireguardpeer-probe", "purpose": PurposeUserVPN, "enabled": true,
		"vpn_type": "wireguard-server", "ip_subnet": "10.199.0.1/24", "local_port": 51822,
		"x_wireguard_private_key": randomWireGuardKey(t),
	})
	if err != nil || status/100 != 2 {
		t.Fatalf("wireguard-server network rejected (HTTP %d): %v %v", status, body, err)
	}
	networkID, _ := firstData(t, body)["_id"].(string)
	if networkID == "" {
		t.Fatalf("wireguard-server network carries no id: %v", body)
	}
	defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+networkID) //nolint:errcheck

	usersPath := "/v2/api/site/" + c.Site + "/wireguard/" + networkID + "/users"
	batchPath := usersPath + "/batch"

	base := func() map[string]any {
		return map[string]any{
			"name": "wg-peer-probe", "interface_ip": "10.199.0.2",
			"public_key": randomWireGuardKey(t), "allowed_ips": []any{"10.199.0.2/32"},
		}
	}
	create := func(doc map[string]any) (int, any, string) {
		body, status, err := s.PostJSON(ctx, batchPath, []any{doc})
		if status == 0 {
			t.Fatalf("transport to %s: %v", batchPath, err)
		}
		id := ""
		if status/100 == 2 {
			if peers, ok := body.([]any); ok && len(peers) == 1 {
				if m, ok := peers[0].(map[string]any); ok {
					id = objectID(m)
				}
			}
		}
		return status, body, id
	}

	asked := base()
	status, body, id := create(asked)
	if status/100 != 2 {
		t.Fatalf("the known-good WireGuardPeer body was rejected (HTTP %d): %v", status, body)
	}
	if id == "" {
		t.Fatalf("the created peer carries no id: %v", body)
	}

	read := func() map[string]any {
		got, status, err := s.GetJSON(ctx, usersPath)
		if err != nil || status != 200 {
			t.Fatalf("GET %s answered HTTP %d (%v)", usersPath, status, err)
		}
		for _, item := range asSlice(got) {
			if m, _ := item.(map[string]any); objectID(m) == id {
				return m
			}
		}
		t.Fatalf("the created peer does not appear in GET %s", usersPath)
		return nil
	}
	stored := read()
	for _, field := range []string{"name", "interface_ip", "public_key", "allowed_ips"} {
		if !jsonEqual(stored[field], asked[field]) {
			t.Logf("WireGuardPeer: %s asked %v, stored %v", field, asked[field], stored[field])
		}
	}
	deletePeer := func(peerID string) {
		s.PostJSON(ctx, usersPath+"/batch_delete", []any{peerID}) //nolint:errcheck
	}
	defer deletePeer(id)

	// unknownKeyObserve assumes a bare-object create body; this endpoint's
	// create body is always an array, so the check is inline here instead.
	unknownDoc := base()
	unknownDoc[probeUnknownKey] = "x"
	unkStatus, unkBody, unkID := create(unknownDoc)
	if unkID != "" {
		defer deletePeer(unkID)
	}
	if unkStatus/100 != 2 {
		t.Logf("POST %s with an unrecognised key rejected (HTTP %d, %v)", batchPath, unkStatus, unkBody)
	} else {
		for _, item := range asSlice(unkBody) {
			if m, _ := item.(map[string]any); objectID(m) == unkID {
				if _, present := m[probeUnknownKey]; present {
					t.Errorf("POST %s stored the unrecognised key %q", batchPath, probeUnknownKey)
				} else {
					t.Logf("POST %s accepts an unrecognised key and stores the peer without it (HTTP %d)", batchPath, unkStatus)
				}
			}
		}
	}

	fields := []string{"name", "interface_ip", "public_key", "allowed_ips"}
	required := requiredFieldSweep(t, fields, func(field string) (int, any) {
		doc := base()
		delete(doc, field)
		status, body, sweepID := create(doc)
		if status/100 == 2 && sweepID != "" {
			deletePeer(sweepID)
		}
		return status, body
	})

	// A partial batch PUT is refused outright (measured elsewhere as a bare
	// HTTP 500), and a full-array PUT that omits a previously-set key
	// clears it exactly like "" does -- both land at "the field is gone",
	// which is why omit is OMIT-CLEARS here rather than OMIT-KEEPS.
	renamed := clone(stored)
	renamed["name"] = ""
	if _, st, err := s.PutJSON(ctx, batchPath, []any{renamed}); st/100 != 2 {
		t.Errorf("a full-array PUT with name=\"\" was rejected (HTTP %d, %v)", st, err)
	}
	emptyName := read()["name"]

	restored := clone(stored)
	restored["name"] = "wg-peer-probe"
	if _, st, err := s.PutJSON(ctx, batchPath, []any{restored}); st/100 != 2 {
		t.Fatalf("restoring name was rejected (HTTP %d): %v", st, err)
	}
	omittedDoc := clone(stored)
	delete(omittedDoc, "name")
	omittedDoc["_id"] = id
	if _, st, err := s.PutJSON(ctx, batchPath, []any{omittedDoc}); st/100 != 2 {
		t.Errorf("a full-array PUT omitting name was rejected (HTTP %d, %v)", st, err)
	}
	omittedName := read()["name"]

	nameEmpty := "EMPTY-CLEARS"
	if !blankValue(emptyName) {
		nameEmpty = "EMPTY-REPLACED-" + renderAny(emptyName)
	}
	nameOmit := "OMIT-CLEARS"
	if !blankValue(omittedName) {
		nameOmit = "OMIT-KEEPS"
	}
	// Not recorded into schemas/behavior.json: WireGuardPeer is hand-written
	// (wireguard_peer.go), not generated from a schema capture, and
	// cmd/wirecontract only assigns a resource key to generated types --
	// confirmed by running go generate against a Writes["WireGuardPeer"]
	// entry, which failed with "names \"WireGuardPeer\", which no type in
	// the artifact claims". Everything measured above is real; it stays in
	// this test's own log and doc comment, which is the only place it has
	// anywhere to land.
	t.Logf("WireGuardPeer write surface (not recorded into the artifact -- see comment above): "+
		"required_on_create=%v, name empty=%s omit=%s", required, nameEmpty, nameOmit)
}

// renderAny renders an arbitrary stored value for an EMPTY-REPLACED-<value>
// tag, matching the vocabulary storedEmptySemantics uses elsewhere.
func renderAny(v any) string {
	if v == nil {
		return "nil"
	}
	return jsonText(v)
}

// TestIntegrationUnmeasurableResourcesRecheck re-confirms, rather than
// re-asserts from a prior session's notes, that PowerSupervisor,
// BroadcastGroup and MediaFile still cannot be measured on this controller.
// Nothing here is recorded into the artifact: there is nothing new to
// record, only a check that the reason nothing can be recorded still holds.
func TestIntegrationUnmeasurableResourcesRecheck(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 20*time.Minute)
	_, _, _ = behaviorGate(ctx, t, s, c.Site)

	for _, collection := range []string{"broadcastgroup", "mediafile"} {
		path := "/api/s/" + c.Site + "/rest/" + collection
		body, status, err := s.PostJSON(ctx, path, map[string]any{"name": collection + "-recheck-probe"})
		if status == 0 {
			t.Fatalf("transport to %s: %v", path, err)
		}
		if status/100 == 2 {
			t.Errorf("LOUD: POST %s now succeeds (HTTP %d); %s may no longer be unmeasurable -- a full "+
				"write-contract probe can be written for it now", path, status, collection)
			if id := objectID(firstData(t, body)); id != "" {
				s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
			}
			continue
		}
		code := v1ErrCode(body)
		if code == "api.err.InvalidObject" {
			t.Logf("%s: POST %s still answers api.err.InvalidObject (HTTP %d) -- the collection is not "+
				"registered on this controller build, same as a made-up collection name", collection, path, status)
		} else {
			t.Logf("%s: POST %s answered HTTP %d, %s -- not the previously measured InvalidObject; "+
				"worth a fresh look rather than assuming it is still unmeasurable for the same reason",
				collection, path, status, code)
		}
	}

	emulated := controllertest.StartDevices(ctx, t, c, controllertest.DeviceRequest{Model: "USM8P"})
	if len(emulated) != 1 {
		t.Log("no emulated switch available for this controller target; PowerSupervisor recheck skipped")
		return
	}
	adopted := c.AdoptDevice(ctx, t, s, emulated[0].MAC)

	// Raw JSON rather than the typed client, so a rejection's body is
	// visible directly instead of folded into a wrapped Go error string.
	psPath := "/v2/api/site/" + c.Site + "/power-supervisors"
	body, status, err := s.PostJSON(ctx, psPath, map[string]any{
		"client_mac": adopted.MAC, "enabled": true, "power_sources": []any{},
		"settings": map[string]any{"heartbeat_interval": 30, "silence_threshold": 300, "power_off_duration": 10},
	})
	if status/100 == 2 {
		if id := objectID(firstData(t, body)); id != "" {
			s.DeleteJSON(ctx, psPath+"/"+id) //nolint:errcheck
		}
		t.Error("LOUD: creating a PowerSupervisor now succeeds; PowerSupervisor may no longer be unmeasurable -- " +
			"a full write-contract probe can be written for it now")
		return
	}
	t.Logf("POST %s rejected (HTTP %d): %v (transport err: %v)", psPath, status, body, err)
	err = fmt.Errorf("%v", body)
	if strings.Contains(err.Error(), "PurePoeRequiresUplinkException") {
		t.Logf("CreatePowerSupervisor still fails with PurePoeRequiresUplinkException: %v -- the "+
			"disposable environment still has no PoE source to supervise, same reason as before", err)
	} else {
		t.Logf("CreatePowerSupervisor failed with a different error than previously measured: %v", err)
	}
}
