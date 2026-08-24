package unifi

import (
	"encoding/json"
	"testing"
)

// TestNilSlicesMarshalAsEmpty checks the generated MarshalJSON renders a nil
// slice as [] for the fields that serialize unconditionally.
//
// A nil slice marshals as null, and the controller rejects null where it
// expects an array: an APGroup or Device built without touching these fields
// could not be written at all. TestIntegrationNullWrites measures the
// controller side of that; this checks the client no longer sends it.
func TestNilSlicesMarshalAsEmpty(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		wire  string
	}{
		{"APGroup.device_macs", &APGroup{Name: "example"}, "device_macs"},
		{"Device.port_overrides", &Device{Name: "example"}, "port_overrides"},
		{"FirewallZone.network_ids", &FirewallZone{Name: "example"}, "network_ids"},
		{"NetworkMembersGroup.members", &NetworkMembersGroup{Name: "example"}, "members"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var encoded map[string]json.RawMessage
			if err := json.Unmarshal(raw, &encoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			got, ok := encoded[tc.wire]
			if !ok {
				t.Fatalf("%s is absent; it is meant to serialize unconditionally so an empty "+
					"list can reach the wire and clear the value", tc.wire)
			}
			if string(got) != "[]" {
				t.Errorf("%s = %s, want []. A nil slice reaching the wire as null is rejected "+
					"by the controller, which is what nil_as_empty exists to prevent", tc.wire, got)
			}
		})
	}
}

// TestNilAsEmptyKeepsValues checks the fix only replaces nil, and does not
// flatten a list the caller actually set.
func TestNilAsEmptyKeepsValues(t *testing.T) {
	raw, err := json.Marshal(&APGroup{Name: "example", DeviceMacs: []string{"00:11:22:33:44:55"}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var encoded struct {
		DeviceMacs []string `json:"device_macs"`
	}
	if err := json.Unmarshal(raw, &encoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(encoded.DeviceMacs) != 1 || encoded.DeviceMacs[0] != "00:11:22:33:44:55" {
		t.Errorf("device_macs = %v, want the address the caller set", encoded.DeviceMacs)
	}
}
