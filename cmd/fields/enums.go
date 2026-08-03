package main

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strconv"
	"strings"

	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

// validationEntry is one generated field's schema validation metadata,
// captured while the resource is generated so it can be written out as
// consumable Go instead of a comment nobody can read at compile time.
type validationEntry struct {
	TypeName  string // the Go type the field belongs to
	FieldName string
	JSONName  string
	Pattern   string
	FieldType string
}

// collectValidation walks every type a resource generates and returns the
// fields carrying a validation pattern.
func collectValidation(r *ResourceInfo) []validationEntry {
	var out []validationEntry
	for typeName, typ := range r.Types {
		if typ == nil {
			continue
		}
		for _, f := range typ.Fields {
			if f == nil || f.FieldValidation == "" || f.JSONName == "" {
				continue
			}
			out = append(out, validationEntry{
				TypeName:  typeName,
				FieldName: f.FieldName,
				JSONName:  f.JSONName,
				Pattern:   f.FieldValidation,
				FieldType: f.FieldType,
			})
		}
	}
	return out
}

// renderValidationFile writes the pattern table and the enumeration values
// for one package.
//
// The controller ships a validation regex per field and the generator has
// always had them, but only ever rendered them as a trailing `//` comment.
// A comment cannot be consumed, so every downstream project re-typed the
// enumerations by hand and had no way to notice when the schema moved
// underneath them. This file is that same data, in a form a compiler sees.
func renderValidationFile(pkg string, entries []validationEntry) ([]byte, error) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TypeName != entries[j].TypeName {
			return entries[i].TypeName < entries[j].TypeName
		}
		return entries[i].JSONName < entries[j].JSONName
	})

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `// Code generated from ace.jar fields *.json files
// DO NOT EDIT.

package %s

// FieldValidationPatterns holds the controller's own validation regex for
// every generated field that has one, keyed by Go type name and then by wire
// (JSON) name.
//
// This is the raw form, useful for building a regex-matching validator. The
// TypeFieldValues variables below are the subset of these patterns that is a
// plain enumeration, already split into values.
var FieldValidationPatterns = map[string]map[string]string{
`, pkg)

	var currentType string
	for _, e := range entries {
		if e.TypeName != currentType {
			if currentType != "" {
				buf.WriteString("\t},\n")
			}
			fmt.Fprintf(&buf, "\t%q: {\n", e.TypeName)
			currentType = e.TypeName
		}
		fmt.Fprintf(&buf, "\t\t%q: %q,\n", e.JSONName, e.Pattern)
	}
	if currentType != "" {
		buf.WriteString("\t},\n")
	}
	buf.WriteString("}\n")

	// numericRange scans a large domain per pattern, and patterns repeat
	// across fields, so resolve each one once.
	type bounds struct {
		low, high int64
		ok        bool
	}
	rangeCache := map[string]bounds{}

	for _, e := range entries {
		prefix := e.TypeName + e.FieldName
		values := prefix + "Values"

		if e.FieldType == fields.Int {
			if vs := enumInt64Values(e.Pattern); vs != nil {
				parts := make([]string, len(vs))
				for i, v := range vs {
					parts[i] = strconv.FormatInt(v, 10)
				}
				fmt.Fprintf(&buf, "\n// %s are the values the controller accepts for %s.%s.\nvar %s = []int64{%s}\n",
					values, e.TypeName, e.JSONName, values, strings.Join(parts, ", "))
				continue
			}
			b, cached := rangeCache[e.Pattern]
			if !cached {
				b.low, b.high, b.ok = numericRange(e.Pattern)
				rangeCache[e.Pattern] = b
			}
			if b.ok {
				fmt.Fprintf(&buf, "\n// %sMin and %sMax are the inclusive bounds the controller accepts for %s.%s.\nconst (\n\t%sMin int64 = %d\n\t%sMax int64 = %d\n)\n",
					prefix, prefix, e.TypeName, e.JSONName, prefix, b.low, prefix, b.high)
			}
			continue
		}

		if vs := enumValues(e.Pattern); vs != nil {
			parts := make([]string, len(vs))
			for i, v := range vs {
				parts[i] = strconv.Quote(v)
			}
			fmt.Fprintf(&buf, "\n// %s are the values the controller accepts for %s.%s.\nvar %s = []string{%s}\n",
				values, e.TypeName, e.JSONName, values, strings.Join(parts, ", "))
			continue
		}

		if low, high, ok := lengthBounds(e.Pattern); ok {
			fmt.Fprintf(&buf, "\n// %sMinLength and %sMaxLength are the character-count bounds the controller accepts for %s.%s.\nconst (\n\t%sMinLength int64 = %d\n\t%sMaxLength int64 = %d\n)\n",
				prefix, prefix, e.TypeName, e.JSONName, prefix, low, prefix, high)
		}
	}

	return format.Source(buf.Bytes())
}

// enumValues extracts the alternatives from a schema validation pattern that
// is a plain enumeration, and returns nil for everything else.
//
// The controller ships one regex per field, and only some of those regexes
// are enumerations wearing a disguise. Consumers want the enumerations: the
// Terraform provider hand-transcribes ~91 of them into OneOf validators
// today, and a transcription that drifts from the schema is invisible until
// a user hits it (openvpn_encryption_cipher was transcribed as AES_256_GCM
// against a schema that has only ever said AES_256_CBC|BF_CBC).
//
// So the bar here is not "extract as many as possible". A wrong list is worse
// than no list: it makes a consumer reject values the controller accepts, and
// that failure reads as a controller bug rather than a generator bug. Every
// alternative must be a bare literal; anything carrying regex syntax, and any
// pattern that also permits the empty string, yields nil.
//
// Accepted:
//
//	AES_256_CBC|BF_CBC   bare alternation
//	^(a|b|c)$            one anchored wrapping group
//	^a$|^b$|^c$          per-alternative anchors
//
// Rejected, with real examples:
//
//	^any$|^(([0-9]…)\.){3}…$   ipsec_local_ip -- a literal mixed with a pattern
//	^0$|^-?([1-9]…)$           dhcpd_time_offset -- a numeric range, not a set
//	[^\"\' ]+                  x_ipsec_pre_shared_key -- not an alternation
//	.{1,128}                   priority -- a length bound
//	^auto$|^manual$|^$         permits "", which a bare OneOf would reject
func enumValues(pattern string) []string {
	p := strings.TrimSpace(pattern)
	if p == "" {
		return nil
	}

	// Unwrap exactly one anchored group -- ^(a|b)$ -- provided it holds no
	// nested groups. Anything deeper is a pattern, not an enumeration.
	if strings.HasPrefix(p, "^(") && strings.HasSuffix(p, ")$") {
		if inner := p[2 : len(p)-2]; !strings.ContainsAny(inner, "()") {
			p = inner
		}
	}

	if !strings.Contains(p, "|") {
		return nil
	}

	parts := strings.Split(p, "|")
	values := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		v = strings.TrimPrefix(v, "^")
		v = strings.TrimSuffix(v, "$")

		// An empty alternative means the field also accepts "". Callers
		// build membership checks from this list, and one that omits ""
		// would reject a value the controller takes, so refuse the lot.
		if v == "" || !isLiteral(v) {
			return nil
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		values = append(values, v)
	}

	// A single alternative is a fixed value, not a choice.
	if len(values) < 2 {
		return nil
	}
	return values
}

// enumInt64Values is enumValues projected onto integers, for fields the
// generator typed as int64. It yields nothing unless every alternative parses,
// so a string enumeration can never reach an int64 consumer.
func enumInt64Values(pattern string) []int64 {
	values := enumValues(pattern)
	if len(values) == 0 {
		return nil
	}

	out := make([]int64, 0, len(values))
	for _, v := range values {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}

// isLiteral reports whether s is free of regex syntax, i.e. whether it stands
// for exactly itself. Hyphens and underscores are literals outside a
// character class, and real enum values use both (aes-128-cmac, tcp_udp).
func isLiteral(s string) bool {
	for _, r := range s {
		switch r {
		case '^', '$', '|', '(', ')', '[', ']', '{', '}', '.', '*', '+', '?', '\\':
			return false
		}
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}
