package unifi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOverlayKeyedEntriesKeepsWhatItWasNotAsked(t *testing.T) {
	stored := []json.RawMessage{
		json.RawMessage(`{"port_idx":1,"name":"uplink","poe_mode":"auto","eee_enabled":false,"unmodelled":"kept"}`),
		json.RawMessage(`{"port_idx":2,"name":"desk","poe_mode":"auto"}`),
	}
	masked := []json.RawMessage{json.RawMessage(`{"port_idx":1,"poe_mode":"off"}`)}

	merged, err := overlayKeyedEntries(stored, "port_idx", masked)
	if err != nil {
		t.Fatalf("overlayKeyedEntries: %v", err)
	}
	if len(merged) != 2 {
		t.Fatalf("merged has %d entries, want 2; the untouched port was dropped", len(merged))
	}

	var first map[string]any
	if err := json.Unmarshal(merged[0], &first); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for wire, want := range map[string]any{
		"port_idx": float64(1),
		// the named member changed
		"poe_mode": "off",
		// and everything else in the entry survived, including a member
		// this client does not model and one at its zero value
		"name":        "uplink",
		"eee_enabled": false,
		"unmodelled":  "kept",
	} {
		if first[wire] != want {
			t.Errorf("entry 1 %s = %v, want %v", wire, first[wire], want)
		}
	}
	if string(merged[1]) != string(stored[1]) {
		t.Errorf("the untouched entry changed:\n got %s\nwant %s", merged[1], stored[1])
	}
}

func TestOverlayKeyedEntriesAddsAnUnknownKey(t *testing.T) {
	stored := []json.RawMessage{json.RawMessage(`{"port_idx":1,"name":"uplink"}`)}
	masked := []json.RawMessage{json.RawMessage(`{"port_idx":5,"poe_mode":"off"}`)}

	merged, err := overlayKeyedEntries(stored, "port_idx", masked)
	if err != nil {
		t.Fatalf("overlayKeyedEntries: %v", err)
	}
	if len(merged) != 2 {
		t.Fatalf("merged has %d entries, want 2; a declared port with no stored entry is added", len(merged))
	}
	if !strings.Contains(string(merged[1]), `"port_idx":5`) {
		t.Errorf("the added entry is %s", merged[1])
	}
}

func TestOverlayKeyedEntriesRejectsAnEntryWithNoKey(t *testing.T) {
	_, err := overlayKeyedEntries(nil, "port_idx", []json.RawMessage{json.RawMessage(`{"poe_mode":"off"}`)})
	if err == nil || !strings.Contains(err.Error(), "port_idx") {
		t.Fatalf("an entry with no key was accepted: %v", err)
	}
}

// TestUpdateDevicePortOverridesSendsTheMergedArray checks the whole path:
// what it reads, what it merges, and what it puts on the wire.
func TestUpdateDevicePortOverridesSendsTheMergedArray(t *testing.T) {
	const storedDevice = `{"meta":{"rc":"ok"},"data":[{
		"_id":"dev1","mac":"00:00:00:00:00:01","name":"switch",
		"port_overrides":[
			{"port_idx":1,"name":"uplink","poe_mode":"auto","eee_enabled":false,"qos_profile":{"qos_policies":[]}},
			{"port_idx":2,"name":"desk","poe_mode":"auto"}
		]}]}`

	var wrote map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &wrote); err != nil {
				t.Errorf("the write body is not a JSON object: %v\n%s", err, body)
			}
			_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[]}`))
			return
		}
		_, _ = w.Write([]byte(storedDevice))
	}))
	t.Cleanup(srv.Close)

	c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := c.UpdateDevicePortOverrides(context.Background(), "default",
		&Device{ID: "dev1", MAC: "00:00:00:00:00:01"},
		[]DevicePortOverrides{{PortIDX: ptrInt64(1), PoeMode: "off"}},
		"poe_mode")
	if err != nil {
		t.Fatalf("UpdateDevicePortOverrides: %v", err)
	}
	if got == nil || got.ID != "dev1" {
		t.Errorf("returned device = %+v, want the re-read device", got)
	}

	// Only the addressing field and the member are written at the top level:
	// this must not turn into a whole-device write.
	if len(wrote) != 2 || string(wrote["_id"]) != `"dev1"` {
		t.Errorf("the write carried %d top-level keys (%v); want _id and port_overrides", len(wrote), keysOf(wrote))
	}

	var entries []map[string]any
	if err := json.Unmarshal(wrote["port_overrides"], &entries); err != nil {
		t.Fatalf("port_overrides is not an array of objects: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("wrote %d entries, want both stored ports", len(entries))
	}
	if entries[0]["poe_mode"] != "off" {
		t.Errorf("port 1 poe_mode = %v, want off", entries[0]["poe_mode"])
	}
	for wire, want := range map[string]any{"name": "uplink", "eee_enabled": false} {
		if entries[0][wire] != want {
			t.Errorf("port 1 %s = %v, want %v -- an unnamed member was dropped", wire, entries[0][wire], want)
		}
	}
	if _, ok := entries[0]["qos_profile"]; !ok {
		t.Error("port 1 lost qos_profile, a member this client does not model")
	}
	if entries[1]["name"] != "desk" {
		t.Errorf("port 2 = %v; an untouched port must be resent verbatim", entries[1])
	}
}

func TestUpdateDevicePortOverridesRejectsAnEmptyMask(t *testing.T) {
	c, err := New(context.Background(), &Config{BaseURL: "https://example.invalid", APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.UpdateDevicePortOverrides(context.Background(), "default",
		&Device{ID: "dev1"}, []DevicePortOverrides{{PortIDX: ptrInt64(1)}}); err == nil ||
		!strings.Contains(err.Error(), "at least one member") {
		t.Fatalf("an empty mask was accepted: %v", err)
	}
	if _, err := c.UpdateDevicePortOverrides(context.Background(), "default",
		&Device{ID: "dev1"}, nil, "poe_mode"); err == nil ||
		!strings.Contains(err.Error(), "no port overrides") {
		t.Fatalf("an empty declaration was accepted: %v", err)
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
