package types

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "already canonical", in: "aa:bb:cc:dd:ee:ff", want: "aa:bb:cc:dd:ee:ff"},
		{name: "uppercase colons", in: "AA:BB:CC:DD:EE:FF", want: "aa:bb:cc:dd:ee:ff"},
		{name: "mixed case", in: "aA:Bb:cC:Dd:eE:Ff", want: "aa:bb:cc:dd:ee:ff"},
		{name: "uppercase hyphens", in: "76-5A-1B-2C-3D-4E", want: "76:5a:1b:2c:3d:4e"},
		{name: "lowercase hyphens", in: "76-5a-1b-2c-3d-4e", want: "76:5a:1b:2c:3d:4e"},
		{name: "cisco dotted", in: "aabb.ccdd.eeff", want: "aa:bb:cc:dd:ee:ff"},
		{name: "no separators", in: "AABBCCDDEEFF", want: "aa:bb:cc:dd:ee:ff"},

		// Left alone. Several MAC-patterned fields permit "", and for those
		// the empty string is how a caller clears the field.
		{name: "empty survives", in: "", want: ""},
		{name: "too short", in: "aa:bb:cc", want: "aa:bb:cc"},
		{name: "too long", in: "aa:bb:cc:dd:ee:ff:00", want: "aa:bb:cc:dd:ee:ff:00"},
		{name: "not hex", in: "not-a-mac", want: "not-a-mac"},
		{name: "hostname", in: "switch-01.lan", want: "switch-01.lan"},
		{name: "arbitrary text", in: "any", want: "any"},
		{name: "twelve non-hex chars", in: "zzzzzzzzzzzz", want: "zzzzzzzzzzzz"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeMAC(tc.in); got != tc.want {
				t.Errorf("NormalizeMAC(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestNormalizeMACIsIdempotent guards the property callers actually rely on:
// normalising twice is the same as normalising once, so a value that has been
// through the SDK compares equal to one that has been through it twice.
func TestNormalizeMACIsIdempotent(t *testing.T) {
	for _, in := range []string{"AA:BB:CC:DD:EE:FF", "76-5a-1b-2c-3d-4e", "aabb.ccdd.eeff", "", "any"} {
		once := NormalizeMAC(in)
		if twice := NormalizeMAC(once); twice != once {
			t.Errorf("NormalizeMAC not idempotent for %q: %q then %q", in, once, twice)
		}
	}
}

func TestMACUnmarshalJSON(t *testing.T) {
	var m MAC
	if err := json.Unmarshal([]byte(`"AA-BB-CC-DD-EE-FF"`), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("MAC = %q, want aa:bb:cc:dd:ee:ff", m)
	}

	// null must not blow up or invent a value.
	var null MAC
	if err := json.Unmarshal([]byte(`null`), &null); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if null != "" {
		t.Errorf("MAC from null = %q, want empty", null)
	}

	// A non-string is an error, not a silent zero.
	var bad MAC
	if err := json.Unmarshal([]byte(`42`), &bad); err == nil {
		t.Error("expected an error unmarshalling a number into MAC")
	}
}

func TestMACSliceUnmarshalJSON(t *testing.T) {
	var got []MAC
	if err := json.Unmarshal([]byte(`["AA-BB-CC-DD-EE-FF","11:22:33:44:55:66",""]`), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := []string{"aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66", ""}
	if diff := MACStrings(got); !reflect.DeepEqual(diff, want) {
		t.Errorf("MACStrings = %v, want %v", diff, want)
	}
}

func TestMACStringsNilStaysNil(t *testing.T) {
	// The generated unmarshalers assign this straight into a []string
	// field; turning a nil into an empty slice would change what the
	// encoder then sends.
	if got := MACStrings(nil); got != nil {
		t.Errorf("MACStrings(nil) = %v, want nil", got)
	}
}
