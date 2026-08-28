package unifi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ubiquiti-community/go-unifi/unifi/settings"
)

// recordedWrite is one write the fake controller saw.
type recordedWrite struct {
	Method, Path string
	Body         []byte
}

// maskedUpdateServer answers reads from a canned table and records every
// write, so a test can assert exactly what a masked update put on the wire.
func maskedUpdateServer(t *testing.T, reads map[string]string, writeReply string) (*httptest.Server, *[]recordedWrite) {
	t.Helper()
	var seen []recordedWrite
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			body, ok := reads[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(body))
			return
		}
		body, _ := io.ReadAll(r.Body)
		seen = append(seen, recordedWrite{Method: r.Method, Path: r.URL.Path, Body: body})
		_, _ = w.Write([]byte(writeReply))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func maskedUpdateClient(t *testing.T, srv *httptest.Server) *ApiClient {
	t.Helper()
	c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func decodeObject(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("write body is not an object: %v\n%s", err, raw)
	}
	return out
}

func decodeBatch(t *testing.T, raw []byte) []map[string]json.RawMessage {
	t.Helper()
	var out []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("write body is not an array of objects: %v\n%s", err, raw)
	}
	return out
}

// expectWire checks that body carries exactly want, key for key.
func expectWire(t *testing.T, body map[string]json.RawMessage, want map[string]string) {
	t.Helper()
	for k, v := range want {
		got, ok := body[k]
		if !ok {
			t.Errorf("%s missing from the write", k)
			continue
		}
		if string(got) != v {
			t.Errorf("%s = %s, want %s", k, got, v)
		}
	}
	for k := range body {
		if _, ok := want[k]; !ok {
			t.Errorf("%s reached the wire; the mask did not name it", k)
		}
	}
}

func expectOneWrite(t *testing.T, seen []recordedWrite, method, path string) recordedWrite {
	t.Helper()
	if len(seen) != 1 {
		t.Fatalf("saw %d writes, want 1: %+v", len(seen), seen)
	}
	if seen[0].Method != method || seen[0].Path != path {
		t.Fatalf("write went to %s %s, want %s %s", seen[0].Method, seen[0].Path, method, path)
	}
	return seen[0]
}

func TestOverlayMaskedKeepsUnmodelledKeys(t *testing.T) {
	stored := json.RawMessage(`{"_id":"x","name":"old","nested":{"a":1,"b":2},"future_key":true}`)
	masked := json.RawMessage(`{"_id":"x","name":"new","nested":{"a":9}}`)

	merged, err := overlayMasked(stored, masked)
	if err != nil {
		t.Fatalf("overlayMasked: %v", err)
	}
	expectWire(t, decodeObject(t, merged), map[string]string{
		"_id":  `"x"`,
		"name": `"new"`,
		// A named field replaces the stored value whole; there is no deep
		// merge, because the mask names top-level wire fields.
		"nested": `{"a":9}`,
		// The point: a key the struct does not model survives the write.
		"future_key": `true`,
	})
}

func TestUpdateSettingFieldsSendsOnlyTheNamedFields(t *testing.T) {
	srv, seen := maskedUpdateServer(t, nil,
		`{"meta":{"rc":"ok"},"data":[{"_id":"s1","site_id":"x","key":"ntp","ntp_server_1":"5.5.5.5","ntp_server_2":"2.2.2.2","setting_preference":"manual"}]}`)
	c := maskedUpdateClient(t, srv)

	// Stale values the caller does not mean to write.
	ntp := &settings.Ntp{NtpServer1: "5.5.5.5", NtpServer2: "stale", SettingPreference: "auto"}
	ntp.ID = "s1"

	if err := c.UpdateSettingFields(context.Background(), "default", ntp, "ntp_server_1"); err != nil {
		t.Fatalf("UpdateSettingFields: %v", err)
	}

	w := expectOneWrite(t, *seen, http.MethodPut, "/proxy/network/api/s/default/set/setting/ntp")
	expectWire(t, decodeObject(t, w.Body), map[string]string{
		"_id":          `"s1"`,
		"key":          `"ntp"`, // addresses the section, so it rides along unnamed
		"ntp_server_1": `"5.5.5.5"`,
	})

	// The section is refreshed from the controller's answer, as UpdateSetting does.
	if ntp.NtpServer2 != "2.2.2.2" || ntp.SettingPreference != "manual" {
		t.Errorf("section not refreshed from the response: %+v", ntp)
	}
}

func TestUpdateSettingFieldsRejectsEmptyMask(t *testing.T) {
	srv, seen := maskedUpdateServer(t, nil, `{}`)
	c := maskedUpdateClient(t, srv)

	err := c.UpdateSettingFields(context.Background(), "default", &settings.Ntp{})
	if err == nil || !strings.Contains(err.Error(), "at least one field") {
		t.Fatalf("empty mask accepted: %v", err)
	}
	if len(*seen) != 0 {
		t.Errorf("a rejected mask still wrote: %+v", *seen)
	}
}

func TestUpdateSettingFieldsRejectsUnknownField(t *testing.T) {
	srv, seen := maskedUpdateServer(t, nil, `{}`)
	c := maskedUpdateClient(t, srv)

	err := c.UpdateSettingFields(context.Background(), "default", &settings.Ntp{}, "ntp_server_9")
	if err == nil || !strings.Contains(err.Error(), "ntp_server_9") {
		t.Fatalf("unknown field accepted: %v", err)
	}
	if len(*seen) != 0 {
		t.Errorf("a rejected mask still wrote: %+v", *seen)
	}
}

func TestUpdateWireGuardPeerFieldsWritesTheStoredPeerBack(t *testing.T) {
	const list = "/proxy/network/v2/api/site/default/wireguard/net1/users"
	srv, seen := maskedUpdateServer(t, map[string]string{
		list: `[
			{"_id":"p1","network_id":"net1","name":"peer1","interface_ip":"10.0.0.2","public_key":"K1","allowed_ips":["10.0.0.2/32"],"future_key":true},
			{"_id":"p2","network_id":"net1","name":"peer2","interface_ip":"10.0.0.3","public_key":"K2","allowed_ips":["10.0.0.3/32"]}
		]`,
	}, `[{"_id":"p1","network_id":"net1","name":"renamed","interface_ip":"10.0.0.2","public_key":"K1","allowed_ips":["10.0.0.2/32"]}]`)
	c := maskedUpdateClient(t, srv)

	// Every unnamed field is zero here. A full write would have asserted
	// those zeros; the batch PUT would have refused the partial body.
	got, err := c.UpdateWireGuardPeerFields(context.Background(), "default", "net1",
		&WireGuardPeer{ID: "p1", Name: "renamed"}, "name")
	if err != nil {
		t.Fatalf("UpdateWireGuardPeerFields: %v", err)
	}
	if got.Name != "renamed" {
		t.Errorf("returned peer name = %q, want renamed", got.Name)
	}

	w := expectOneWrite(t, *seen, http.MethodPut, list+"/batch")
	batch := decodeBatch(t, w.Body)
	if len(batch) != 1 {
		t.Fatalf("batch carries %d peers, want the one addressed", len(batch))
	}
	expectWire(t, batch[0], map[string]string{
		"_id":          `"p1"`,
		"network_id":   `"net1"`,
		"name":         `"renamed"`,
		"interface_ip": `"10.0.0.2"`,
		"public_key":   `"K1"`,
		"allowed_ips":  `["10.0.0.2/32"]`,
		"future_key":   `true`,
	})
}

func TestUpdateWireGuardPeerFieldsUnknownPeer(t *testing.T) {
	srv, seen := maskedUpdateServer(t, map[string]string{
		"/proxy/network/v2/api/site/default/wireguard/net1/users": `[{"_id":"p1","name":"peer1"}]`,
	}, `[]`)
	c := maskedUpdateClient(t, srv)

	_, err := c.UpdateWireGuardPeerFields(context.Background(), "default", "net1",
		&WireGuardPeer{ID: "nope", Name: "x"}, "name")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("missing peer did not surface as NotFoundError: %v", err)
	}
	if len(*seen) != 0 {
		t.Errorf("wrote despite the peer being absent: %+v", *seen)
	}
}

func TestUpdateBGPConfigFieldsPostsTheStoredConfigBack(t *testing.T) {
	const path = "/proxy/network/v2/api/site/default/bgp/config"
	srv, seen := maskedUpdateServer(t, map[string]string{
		path: `[{"_id":"b1","site_id":"x","enabled":true,"frr_bgpd_config":"router bgp 65000","description":"prod","future_key":1}]`,
	}, `{"_id":"b1","site_id":"x","enabled":false,"frr_bgpd_config":"router bgp 65000","description":"prod"}`)
	c := maskedUpdateClient(t, srv)

	got, err := c.UpdateBGPConfigFields(context.Background(), "default", &BGPConfig{Enabled: false}, "enabled")
	if err != nil {
		t.Fatalf("UpdateBGPConfigFields: %v", err)
	}
	if got.Enabled {
		t.Error("returned config still enabled")
	}

	w := expectOneWrite(t, *seen, http.MethodPost, path)
	expectWire(t, decodeObject(t, w.Body), map[string]string{
		"_id":     `"b1"`,
		"site_id": `"x"`,
		"enabled": `false`,
		// The whole-object POST would have dropped these from a zero struct.
		"frr_bgpd_config": `"router bgp 65000"`,
		"description":     `"prod"`,
		"future_key":      `1`,
	})
}

func TestUpdatePowerSupervisorFieldsWritesTheStoredObjectBack(t *testing.T) {
	const list = "/proxy/network/v2/api/site/default/power-supervisors"
	srv, seen := maskedUpdateServer(t, map[string]string{
		list: `[
			{"id":"ps0","site_id":"x","client_mac":"00:00:00:00:00:01","enabled":true,"settings":{"heartbeat_interval":1,"silence_threshold":2,"power_off_duration":3},"power_sources":[]},
			{"id":"ps1","site_id":"x","client_mac":"00:00:00:00:00:02","enabled":true,"settings":{"heartbeat_interval":30,"silence_threshold":60,"power_off_duration":10},"power_sources":[{"power_source_mac":"00:00:00:00:00:99","power_source_index":1}],"consecutive_failures":2,"future_key":"k"}
		]`,
	}, `{"id":"ps1","site_id":"x","client_mac":"00:00:00:00:00:02","enabled":false,"settings":{"heartbeat_interval":30,"silence_threshold":60,"power_off_duration":10},"power_sources":[{"power_source_mac":"00:00:00:00:00:99","power_source_index":1}]}`)
	c := maskedUpdateClient(t, srv)

	got, err := c.UpdatePowerSupervisorFields(context.Background(), "default", &PowerSupervisor{ID: "ps1", Enabled: false}, "enabled")
	if err != nil {
		t.Fatalf("UpdatePowerSupervisorFields: %v", err)
	}
	if got.Enabled {
		t.Error("returned supervisor still enabled")
	}

	w := expectOneWrite(t, *seen, http.MethodPut, list+"/ps1")
	expectWire(t, decodeObject(t, w.Body), map[string]string{
		"id":                   `"ps1"`,
		"site_id":              `"x"`,
		"client_mac":           `"00:00:00:00:00:02"`,
		"enabled":              `false`,
		"settings":             `{"heartbeat_interval":30,"silence_threshold":60,"power_off_duration":10}`,
		"power_sources":        `[{"power_source_mac":"00:00:00:00:00:99","power_source_index":1}]`,
		"consecutive_failures": `2`,
		"future_key":           `"k"`,
	})
}

func TestUpdateSiteFieldsAcceptsDescAlone(t *testing.T) {
	srv, seen := maskedUpdateServer(t, nil,
		`{"meta":{"rc":"ok"},"data":[{"_id":"s1","name":"default","desc":"Renamed"},{"_id":"s2","name":"other","desc":"Other"}]}`)
	c := maskedUpdateClient(t, srv)

	_, err := c.UpdateSiteFields(context.Background(), &Site{Name: "default", Description: "x"}, "desc", "name")
	if err == nil || !strings.Contains(err.Error(), "name") || !strings.Contains(err.Error(), "desc") {
		t.Fatalf("a mask naming the slug was accepted: %v", err)
	}
	if _, err := c.UpdateSiteFields(context.Background(), &Site{Name: "default"}); err == nil {
		t.Fatal("empty mask accepted")
	}
	if len(*seen) != 0 {
		t.Fatalf("a rejected mask still wrote: %+v", *seen)
	}

	got, err := c.UpdateSiteFields(context.Background(), &Site{Name: "default", Description: "Renamed"}, "desc")
	if err != nil {
		t.Fatalf("UpdateSiteFields: %v", err)
	}
	if got.ID != "s1" || got.Description != "Renamed" {
		t.Errorf("returned site = %+v, want s1/Renamed", got)
	}

	w := expectOneWrite(t, *seen, http.MethodPost, "/proxy/network/api/s/default/cmd/sitemgr")
	expectWire(t, decodeObject(t, w.Body), map[string]string{
		"cmd":  `"update-site"`,
		"desc": `"Renamed"`,
	})
}
