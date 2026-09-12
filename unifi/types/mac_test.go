package types

import (
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
