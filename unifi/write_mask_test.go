package unifi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMaskedBody(t *testing.T) {
	w := &WLAN{
		ID:     "abc123",
		SiteID: "default",
		Name:   "example",
		// One toggle the caller means, and one it does not. Both marshal as
		// bools; only the named one may reach the wire.
		Enabled:                   true,
		RoamingAssistantNaEnabled: false,
	}

	body, err := maskedBody(w, []string{"enabled"})
	if err != nil {
		t.Fatalf("maskedBody: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal masked body: %v", err)
	}

	if _, ok := got["enabled"]; !ok {
		t.Error("the named field is missing from the masked write")
	}
	// The point of the whole exercise: an unnamed toggle must not be
	// asserted, or the controller stores false for it.
	if _, ok := got["roaming_assistant_na_enabled"]; ok {
		t.Error("an unnamed toggle reached the wire; the controller would store false for it")
	}
	for _, wire := range []string{"_id", "site_id"} {
		if _, ok := got[wire]; !ok {
			t.Errorf("identity field %s was dropped; the write would not address the object", wire)
		}
	}
	if _, ok := got["name"]; ok {
		t.Error("an unnamed field reached the wire")
	}
}

func TestMaskedBodyRejectsUnknownField(t *testing.T) {
	_, err := maskedBody(&WLAN{ID: "abc123", Name: "example"}, []string{"enabled", "no_such_field"})
	if err == nil {
		t.Fatal("a misspelled field was accepted; it would have been silently omitted")
	}
	if got := err.Error(); !strings.Contains(got, "no_such_field") {
		t.Errorf("error does not name the offending field: %s", got)
	}
}

func TestMaskedBodyRejectsEmptyMask(t *testing.T) {
	if _, err := maskedBody(&WLAN{ID: "abc123"}, nil); err == nil {
		t.Fatal("an empty mask was accepted; it would have written nothing but identity")
	}
}

// TestMaskedBodyWritesSelectedZeroValues covers the case a mask exists for:
// clearing a value. A field with omitempty is absent from the encoding at its
// zero value, and rejecting it there would make "set this back to false"
// inexpressible.
func TestMaskedBodyWritesSelectedZeroValues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		wire  string
		want  string
	}{
		{"bool false", &Device{ID: "abc123"}, "beep_enabled", "false"},
		{"empty string", &Client{ID: "abc123"}, "note", `""`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := maskedBody(tc.value, []string{tc.wire})
			if err != nil {
				t.Fatalf("selecting %s at its zero value: %v", tc.wire, err)
			}

			var got map[string]json.RawMessage
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			value, ok := got[tc.wire]
			if !ok {
				t.Fatalf("%s was selected but did not reach the wire, so the value cannot be cleared", tc.wire)
			}
			if string(value) != tc.want {
				t.Errorf("%s = %s, want %s", tc.wire, value, tc.want)
			}
		})
	}
}

// TestMaskedBodyRejectsFieldsTheEncoderDrops keeps the other half honest. A
// field can be missing from the encoding because the encoder suppresses it,
// not because it is unset, and writing a zero for one of those would send a
// key the encoder exists to withhold.
func TestMaskedBodyRejectsFieldsTheEncoderDrops(t *testing.T) {
	// Read-only: the generated MarshalJSON shadows it out entirely.
	if _, err := maskedBody(&FirewallZone{ID: "z"}, []string{"external_id"}); err == nil {
		t.Error("named a read-only field and the mask accepted it; the controller rejects that key on a write")
	}

	// Purpose-specific: the vlan-only encoder carries no WAN fields.
	if _, err := maskedBody(&Network{ID: "n", Purpose: PurposeVLANOnly}, []string{"wan_dns1"}); err == nil {
		t.Error("named a field the vlan-only encoder never emits and the mask accepted it")
	}
}

// TestMaskedBodyHonoursCustomEncoder checks the mask filters what the encoder
// would have sent rather than what the struct holds. Network's MarshalJSON
// emits a purpose-specific subset, so a field that encoder drops cannot be
// named -- and saying so beats writing a key the encoder deliberately omits.
func TestMaskedBodyHonoursCustomEncoder(t *testing.T) {
	n := &Network{
		ID:      "netid",
		Name:    strPtr("example"),
		Purpose: PurposeVLANOnly,
	}
	if _, err := maskedBody(n, []string{"wan_dns1"}); err == nil {
		t.Fatal("named a field the vlan-only encoder never emits, and the mask accepted it")
	}
}

// TestMaskedBodyNeedsTheDiscriminator pins the cost of the design above: a
// masked Network write needs Purpose set even when the mask names only
// "name".
//
// That is deliberate, not an oversight. The mask decides what a field's fate
// would have been by running the encoder, and Network's encoder cannot decide
// anything without a purpose -- so the alternative to failing here is
// guessing, and a wrong guess writes WAN keys onto a vlan-only network.
// Callers have the purpose: it comes back on the read that preceded the
// write. What this asserts is that they are told so.
func TestMaskedBodyNeedsTheDiscriminator(t *testing.T) {
	_, err := maskedBody(&Network{ID: "netid", Name: strPtr("example")}, []string{"name"})
	if err == nil {
		t.Fatal("a network with no Purpose encoded anyway; the mask cannot know what that encoder would send")
	}
	if !strings.Contains(err.Error(), "Network.Purpose") {
		t.Errorf("the error does not say what to set, so the caller cannot act on it: %v", err)
	}

	// With the discriminator, the same masked write goes through.
	body, err := maskedBody(&Network{ID: "netid", Name: strPtr("example"), Purpose: PurposeVLANOnly}, []string{"name"})
	if err != nil {
		t.Fatalf("purpose set and the mask still refused: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["name"]; !ok {
		t.Errorf("masked write dropped the one field it named: %s", body)
	}
	if _, ok := got["purpose"]; ok {
		t.Errorf("purpose was needed to encode, but it was not named and must not be written: %s", body)
	}
}
