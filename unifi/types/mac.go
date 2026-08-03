package types

import (
	"encoding/json"
	"strings"
)

// MAC is a MAC address field, normalised on decode to the lowercase
// colon-separated form the controller's own schema requires
// (^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$).
//
// Callers compare MACs with == -- against a value they were given, against
// one they typed, against one read back from a different endpoint. That only
// works if every MAC the SDK hands out has the same shape, and the
// controller is not consistent about it. Decoding through this type makes
// the read side canonical; the write side is deliberately untouched, since
// rewriting a caller's MAC on the way out would make the SDK accept input
// the controller itself refuses.
type MAC string

func (m *MAC) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*m = MAC(NormalizeMAC(s))
	return nil
}

// String returns the normalised address.
func (m MAC) String() string {
	return string(m)
}

// NormalizeMAC rewrites a MAC address to lowercase colon-separated form,
// accepting the separators seen in the wild: colons, hyphens, Cisco-style
// dots, and none at all.
//
// Anything that is not recognisably a MAC comes back untouched. That
// matters more than the conversion: these fields are identified by their
// schema pattern, and several of them permit the empty string, so a
// normaliser that mangled unexpected input would corrupt real values to buy
// nothing. The empty string in particular has to survive as itself -- for
// fields like dhcpd_mac_1 the controller stores it, and it is how a caller
// clears the field.
func NormalizeMAC(s string) string {
	if len(s) == 0 {
		return s
	}

	var hex strings.Builder
	hex.Grow(12)
	for _, r := range s {
		switch {
		case r == ':' || r == '-' || r == '.':
			continue
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f':
			hex.WriteRune(r)
		case r >= 'A' && r <= 'F':
			hex.WriteRune(r + ('a' - 'A'))
		default:
			// Not a MAC at all; hand it back as it came.
			return s
		}
	}

	digits := hex.String()
	if len(digits) != 12 {
		return s
	}

	var out strings.Builder
	out.Grow(17)
	for i := 0; i < 12; i += 2 {
		if i > 0 {
			out.WriteByte(':')
		}
		out.WriteString(digits[i : i+2])
	}
	return out.String()
}

// MACString converts a decoded MAC back to a plain string, for the generated
// unmarshalers to assign into a string field.
func MACString(m MAC) string {
	return string(m)
}

// MACStrings is MACString over a slice, for generated []string fields.
func MACStrings(m []MAC) []string {
	if m == nil {
		return nil
	}
	out := make([]string, len(m))
	for i, v := range m {
		out[i] = string(v)
	}
	return out
}
