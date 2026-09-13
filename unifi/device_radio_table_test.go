package unifi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// Regression test for ubiquiti-community/terraform-provider-unifi#112:
// UniFi 10.x controllers return radio_table channel/tx_power as JSON numbers,
// while older ones (and the schema) use strings such as "auto". The
// DeviceRadioTable unmarshaler must accept either form for these string fields.
func TestDeviceRadioTableUnmarshalChannelTxPower(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantChannel string
		wantTxPower string
	}{
		{
			name:        "numeric channel and tx_power (UniFi 10.x)",
			raw:         `{"radio":"na","channel":36,"tx_power":23}`,
			wantChannel: "36",
			wantTxPower: "23",
		},
		{
			name:        "string auto (older controllers)",
			raw:         `{"radio":"ng","channel":"auto","tx_power":"auto"}`,
			wantChannel: "auto",
			wantTxPower: "auto",
		},
		{
			name:        "numeric string channel",
			raw:         `{"radio":"na","channel":"149","tx_power":"high"}`,
			wantChannel: "149",
			wantTxPower: "high",
		},
		{
			name:        "fractional channel (6GHz)",
			raw:         `{"radio":"6e","channel":1.5}`,
			wantChannel: "1.5",
			wantTxPower: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var rt DeviceRadioTable
			if err := json.Unmarshal([]byte(tc.raw), &rt); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if rt.Channel != tc.wantChannel {
				t.Errorf("Channel = %q, want %q", rt.Channel, tc.wantChannel)
			}
			if rt.TxPower != tc.wantTxPower {
				t.Errorf("TxPower = %q, want %q", rt.TxPower, tc.wantTxPower)
			}
		})
	}
}

// TestUpdateDeviceRadioTableSendsOnlyWhatWasNamed checks the whole path: what
// reaches the wire is the declared radios, carrying their name and the named
// members and nothing else. The controller merges by name -- measured on
// 10.6.101 -- so anything extra here would be a member the caller never asked
// to change, written from whatever the struct happened to hold.
func TestUpdateDeviceRadioTableSendsOnlyWhatWasNamed(t *testing.T) {
	const storedDevice = `{"meta":{"rc":"ok"},"data":[{
		"_id":"dev1","mac":"00:00:00:00:00:01","name":"ap",
		"radio_table":[
			{"name":"wifi-ng","radio":"ng","channel":"auto","nss":2},
			{"name":"wifi-na","radio":"na","channel":"auto","nss":2}
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

	got, err := c.UpdateDeviceRadioTable(context.Background(), "default",
		&Device{ID: "dev1", MAC: "00:00:00:00:00:01"},
		// Radio and Channel are set the way a caller who read the device
		// first holds them, and neither is named, so neither may travel: the
		// controller refuses a radio member that disagrees with the name, so
		// sending one nobody asked for turns a good write into an HTTP 400.
		[]DeviceRadioTable{{Name: "wifi-na", Radio: "na", Channel: "auto", TxPowerMode: "custom", TxPower: "17"}},
		"tx_power_mode", "tx_power")
	if err != nil {
		t.Fatalf("UpdateDeviceRadioTable: %v", err)
	}
	if got == nil || got.ID != "dev1" {
		t.Errorf("returned device = %+v, want the re-read device", got)
	}

	// Only the addressing field and the member are written at the top level:
	// this must not turn into a whole-device write.
	if len(wrote) != 2 || string(wrote["_id"]) != `"dev1"` {
		t.Errorf("the write carried %d top-level keys (%v); want _id and radio_table", len(wrote), keysOf(wrote))
	}

	var entries []map[string]any
	if err := json.Unmarshal(wrote["radio_table"], &entries); err != nil {
		t.Fatalf("radio_table is not an array of objects: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("wrote %d entries (%v), want the declared radio alone: the controller keeps the rest", len(entries), entries)
	}
	want := map[string]any{"name": "wifi-na", "tx_power_mode": "custom", "tx_power": "17"}
	if !reflect.DeepEqual(entries[0], want) {
		t.Errorf("wrote %v, want %v", entries[0], want)
	}
}

func TestUpdateDeviceRadioTableRejectsAWriteItCannotAddress(t *testing.T) {
	c, err := New(context.Background(), &Config{BaseURL: "https://example.invalid", APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := c.UpdateDeviceRadioTable(context.Background(), "default",
		&Device{ID: "dev1"}, []DeviceRadioTable{{Name: "wifi-na"}}); err == nil ||
		!strings.Contains(err.Error(), "at least one member") {
		t.Fatalf("an empty mask was accepted: %v", err)
	}
	if _, err := c.UpdateDeviceRadioTable(context.Background(), "default",
		&Device{ID: "dev1"}, nil, "channel"); err == nil ||
		!strings.Contains(err.Error(), "no radios") {
		t.Fatalf("an empty declaration was accepted: %v", err)
	}
	// Name is omitempty like every other member, so a nameless entry would
	// otherwise reach the controller as a body it answers with MissingValue.
	if _, err := c.UpdateDeviceRadioTable(context.Background(), "default",
		&Device{ID: "dev1"}, []DeviceRadioTable{{Channel: "36"}}, "channel"); err == nil ||
		!strings.Contains(err.Error(), "carries no name") {
		t.Fatalf("a radio with no name was accepted: %v", err)
	}
}
