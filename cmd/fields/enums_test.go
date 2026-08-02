package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"
)

// TestEnumValues pins the extractor against real validation patterns taken
// from the generated files. The rejection cases matter more than the
// acceptance ones: a wrong enum hands consumers a list that rejects values
// the controller accepts, and that failure reads as a controller bug rather
// than a generator bug.
func TestEnumValues(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    []string
	}{
		// --- accepted -------------------------------------------------
		{
			name:    "bare alternation",
			pattern: "AES_256_CBC|BF_CBC",
			want:    []string{"AES_256_CBC", "BF_CBC"},
		},
		{
			name:    "slice field enum (wlan_bands)",
			pattern: "2g|5g|6g",
			want:    []string{"2g", "5g", "6g"},
		},
		{
			name:    "hyphenated values (wlan pmf cipher)",
			pattern: "auto|aes-128-cmac|bip-gmac-256",
			want:    []string{"auto", "aes-128-cmac", "bip-gmac-256"},
		},
		{
			name:    "underscored values",
			pattern: "icmp|tcp_udp|tcp|udp|esp",
			want:    []string{"icmp", "tcp_udp", "tcp", "udp", "esp"},
		},
		{
			name:    "single anchored wrapping group",
			pattern: "^(auto|manual)$",
			want:    []string{"auto", "manual"},
		},
		{
			name:    "per-alternative anchors",
			pattern: "^up$|^down$|^test$",
			want:    []string{"up", "down", "test"},
		},

		// --- rejected -------------------------------------------------
		{
			name:    "not an alternation at all (x_ipsec_pre_shared_key)",
			pattern: `[^\"\' ]+`,
			want:    nil,
		},
		{
			name:    "length pattern, not an enum (priority, on an int64 field)",
			pattern: ".{1,128}",
			want:    nil,
		},
		{
			name:    "literal mixed with a pattern (ipsec_local_ip)",
			pattern: `^any$|^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`,
			want:    nil,
		},
		{
			name:    "numeric range regex, not an enum (dhcpd_time_offset)",
			pattern: "^0$|^-?([1-9]([0-9]{1,3})?|[1-7][0-9]{4}|[8][0-5][0-9]{3}|86[0-3][0-9]{2}|86400)$",
			want:    nil,
		},
		{
			name:    "range regex with a leading group (interface_mtu)",
			pattern: "^(6[89]|[7-9][0-9]|[1-9][0-9]{2,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-6])$",
			want:    nil,
		},
		{
			name:    "alternation of patterns, not literals (schedule)",
			pattern: `(sun|mon|tue|wed|thu|fri|sat)(\-(sun|mon|tue|wed|thu|fri|sat))?\|([0-2][0-9][0-5][0-9])\-([0-2][0-9][0-5][0-9])`,
			want:    nil,
		},
		{
			name:    "MAC pattern with an empty alternative (dhcpd_mac_1)",
			pattern: "(^$|^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$)",
			want:    nil,
		},
		{
			name: "literal alternation that also permits empty",
			// A bare OneOf built from these would reject "", which the
			// controller accepts, so the whole pattern is refused.
			pattern: "^auto$|^manual$|^$",
			want:    nil,
		},
		{
			name:    "single value is not an enumeration",
			pattern: "^auto$",
			want:    nil,
		},
		{
			name:    "empty pattern",
			pattern: "",
			want:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := enumValues(tc.pattern)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("enumValues(%q)\n got %#v\nwant %#v", tc.pattern, got, tc.want)
			}
		})
	}
}

// TestEnumInt64Values checks the integer projection: it yields values only
// when every alternative parses as an int64, so a string enum never reaches
// an int64validator.OneOf.
func TestEnumInt64Values(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    []int64
	}{
		{name: "integer alternation", pattern: "1|2|5", want: []int64{1, 2, 5}},
		{name: "negative values", pattern: "^-1$|^0$|^1$", want: []int64{-1, 0, 1}},
		{name: "string enum yields nothing", pattern: "auto|manual", want: nil},
		{name: "mixed yields nothing", pattern: "1|auto", want: nil},
		{name: "not an enum", pattern: ".{1,128}", want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := enumInt64Values(tc.pattern)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("enumInt64Values(%q)\n got %#v\nwant %#v", tc.pattern, got, tc.want)
			}
		})
	}
}

// TestEnumValuesRejectsEveryNonEnumPattern is the guard that matters at
// scale: it runs the extractor over every distinct validation pattern the
// schema produces and asserts that whatever survives really is a list of
// plain literals. It cannot check that the values are *right* -- only the
// controller knows that -- but it does catch an extractor that starts
// accepting regex fragments after a schema refresh introduces a new shape.
func TestEnumValuesYieldsOnlyLiterals(t *testing.T) {
	patterns := distinctSchemaPatterns(t)

	// The corpus is scraped with a regex, and a regex that quietly stops
	// matching turns this into a test that passes by doing nothing. It was
	// exactly that until the scraper was fixed, so pin a floor.
	if len(patterns) < 100 {
		t.Fatalf("only %d distinct patterns scraped from the generated files; the scraper is broken", len(patterns))
	}

	enums := 0
	for _, pattern := range patterns {
		values := enumValues(pattern)
		if values == nil {
			continue
		}
		enums++
		if len(values) < 2 {
			t.Errorf("pattern %q yielded %d values; an enum needs at least two", pattern, len(values))
		}
		for _, v := range values {
			if !isLiteral(v) {
				t.Errorf("pattern %q yielded non-literal value %q", pattern, v)
			}
		}
	}
	t.Logf("%d distinct patterns, %d recognised as enumerations", len(patterns), enums)
}

// generatedFieldPattern matches the trailing validation comment the template
// writes after each generated struct field.
var generatedFieldPattern = regexp.MustCompile("(?m)`json:\"[^\"]*\"`[^\\S\\n]*//[^\\S\\n]*(.+?)[^\\S\\n]*$")

// distinctSchemaPatterns reads every validation pattern out of the committed
// generated files. They are the closest checked-in record of what the
// controller schema actually contains -- the extracted JSON it came from is
// transient and gitignored, so it cannot be relied on in a plain `go test`.
func distinctSchemaPatterns(t *testing.T) []string {
	t.Helper()

	seen := map[string]bool{}
	for _, dir := range []string{"../../unifi", "../../unifi/settings"} {
		matches, err := filepath.Glob(filepath.Join(dir, "*.generated.go"))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		for _, file := range matches {
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			for _, m := range generatedFieldPattern.FindAllSubmatch(b, -1) {
				seen[string(m[1])] = true
			}
		}
	}
	if len(seen) == 0 {
		t.Skip("no generated files to read patterns from")
	}

	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
