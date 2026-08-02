package main

import "testing"

func TestNumericRange(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		low     int64
		high    int64
		ok      bool
	}{
		{
			name:    "interface_mtu",
			pattern: `^(6[89]|[7-9][0-9]|[1-9][0-9]{2,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-6])$`,
			low:     68, high: 65536, ok: true,
		},
		{
			name:    "local_port",
			pattern: `^([1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-4][0-9]{3}|[6][5][0-4][0-9]{2}|[6][5][5][0-2][0-9]|[6][5][5][3][0-5])$`,
			low:     1, high: 65535, ok: true,
		},
		{
			name:    "dhcpd_time_offset spans negatives",
			pattern: `^0$|^-?([1-9]([0-9]{1,3})?|[1-7][0-9]{4}|[8][0-5][0-9]{3}|86[0-3][0-9]{2}|86400)$`,
			low:     -86400, high: 86400, ok: true,
		},

		// Refused.
		{
			name:    "stp_priority is a set, not a range",
			pattern: `0|4096|8192|12288|16384|20480|24576|28672|32768|36864|40960|45056|49152|53248|57344|61440`,
		},
		{
			name: "a length rule on an int64 field matches every integer",
			// priority. Without the domain-edge guard this reports the scan
			// bounds as if they were the schema's.
			pattern: `.{1,128}`,
		},
		{
			name:    "single value is a constant",
			pattern: `^5$`,
		},
		{
			name:    "matches nothing numeric",
			pattern: `auto|manual`,
		},
		{
			name:    "empty",
			pattern: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			low, high, ok := numericRange(tc.pattern)
			if ok != tc.ok {
				t.Fatalf("numericRange(%q) ok = %v, want %v (got %d..%d)", tc.pattern, ok, tc.ok, low, high)
			}
			if ok && (low != tc.low || high != tc.high) {
				t.Errorf("numericRange(%q) = %d..%d, want %d..%d", tc.pattern, low, high, tc.low, tc.high)
			}
		})
	}
}

func TestLengthBounds(t *testing.T) {
	tests := []struct {
		pattern string
		low     int64
		high    int64
		ok      bool
	}{
		{pattern: ".{1,128}", low: 1, high: 128, ok: true},
		{pattern: ".{0,255}", low: 0, high: 255, ok: true},
		{pattern: "^.{1,64}$", low: 1, high: 64, ok: true},
		// Constrains content as well as length: a length bound would lose
		// the part that matters.
		{pattern: `^[^"' ]{1,32}$`},
		{pattern: "auto|manual"},
		{pattern: ""},
	}

	for _, tc := range tests {
		t.Run(tc.pattern, func(t *testing.T) {
			low, high, ok := lengthBounds(tc.pattern)
			if ok != tc.ok {
				t.Fatalf("lengthBounds(%q) ok = %v, want %v", tc.pattern, ok, tc.ok)
			}
			if ok && (low != tc.low || high != tc.high) {
				t.Errorf("lengthBounds(%q) = %d..%d, want %d..%d", tc.pattern, low, high, tc.low, tc.high)
			}
		})
	}
}
