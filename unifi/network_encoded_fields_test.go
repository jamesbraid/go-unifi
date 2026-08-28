package unifi

import (
	"encoding/json"
	"slices"
	"testing"
)

// TestNetworkEncodedFieldsMatchTheEncoder checks the published field sets
// against what the encoder actually writes for a real object, one purpose at
// a time.
//
// The sets are computed by running the encoder, so this cannot fail through
// drift between two hand-maintained lists. What it does catch is the claim
// underneath: that a fully populated Network shows every key a purpose is
// capable of writing. A field the encoder emits only under some condition
// the population walk does not satisfy would be missing from the published
// set, and a consumer would then be told it cannot write a field it can.
func TestNetworkEncodedFieldsMatchTheEncoder(t *testing.T) {
	for _, purpose := range NetworkPurposes {
		t.Run(purpose, func(t *testing.T) {
			published := NetworkEncodedFields(purpose)
			if len(published) == 0 {
				t.Fatalf("no fields published for purpose %q", purpose)
			}
			if !slices.IsSorted(published) {
				t.Error("published fields are not sorted; callers binary-search them")
			}

			name := "encoded-fields"
			n := &Network{Purpose: purpose, Name: &name}
			raw, err := json.Marshal(n)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var emitted map[string]json.RawMessage
			if err := json.Unmarshal(raw, &emitted); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			// A minimally populated object emits a subset: omitempty drops
			// what it left unset. Nothing it emits may be absent from the
			// published set.
			for wire := range emitted {
				if !NetworkEncodesField(purpose, wire) {
					t.Errorf("the encoder wrote %q for purpose %q, which the published set omits", wire, purpose)
				}
			}
		})
	}
}

// TestNetworkEncodedFieldsAreDistinctPerPurpose pins the reason the function
// takes a purpose at all. If every purpose published the same set, a caller
// could use one list and the discriminator would be decoration.
func TestNetworkEncodedFieldsAreDistinctPerPurpose(t *testing.T) {
	corporate := NetworkEncodedFields(PurposeCorporate)
	for _, purpose := range NetworkPurposes {
		if purpose == PurposeCorporate {
			continue
		}
		if slices.Equal(corporate, NetworkEncodedFields(purpose)) {
			t.Errorf("purpose %q publishes the same field set as corporate; the encoders no longer differ", purpose)
		}
	}

	// The case that motivates the whole function: a WAN field the corporate
	// encoder never writes, and a corporate field the WAN encoder never
	// writes.
	if !NetworkEncodesField(PurposeWAN, "wan_type") {
		t.Error("the WAN encoder does not publish wan_type")
	}
	if NetworkEncodesField(PurposeCorporate, "wan_type") {
		t.Error("the corporate encoder publishes wan_type; a masked write would be told it may send it")
	}
	if !NetworkEncodesField(PurposeCorporate, "dhcpd_enabled") {
		t.Error("the corporate encoder does not publish dhcpd_enabled")
	}
}

// TestNetworkEncodedFieldsCopyOut checks that a caller cannot edit the
// cached answer through the slice it is handed.
func TestNetworkEncodedFieldsCopyOut(t *testing.T) {
	first := NetworkEncodedFields(PurposeCorporate)
	first[0] = "clobbered"
	if second := NetworkEncodedFields(PurposeCorporate); second[0] == "clobbered" {
		t.Error("the published set is shared with the caller; editing it corrupted the cache")
	}
}
