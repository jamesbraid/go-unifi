package unifi

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// identityFields are carried by a masked write regardless of the mask: they
// address the object rather than configure it.
var identityFields = []string{"_id", "id", "site_id"}

// maskedBody renders d as a write carrying only the named wire fields.
//
// The generated structs declare 374 bools without omitempty (pinned by
// TestGeneratedWriteShape), so a full write asserts false for every toggle
// the caller left alone, and the controller stores false. A key the payload
// omits keeps its stored value instead -- TestIntegrationPartialWriteMerges
// pins that -- so naming the fields you mean is enough to leave the rest
// untouched.
//
// An unknown name is an error rather than a silent omission. A mask that
// quietly dropped a typo would reproduce the failure it exists to prevent,
// one layer up.
func maskedBody(d any, fields []string) (json.RawMessage, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("a masked write needs at least one field; to write the whole object, use the unmasked call")
	}

	// Marshal first so any custom MarshalJSON is honoured: the mask filters
	// what the encoder would have sent, not what the struct happens to hold.
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("unable to encode the object: %w", err)
	}

	var encoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, fmt.Errorf("unable to read back the encoded object: %w", err)
	}

	out := make(map[string]json.RawMessage, len(fields)+len(identityFields))
	for _, wire := range identityFields {
		if value, ok := encoded[wire]; ok {
			out[wire] = value
		}
	}

	var unknown []string
	for _, wire := range fields {
		value, ok := encoded[wire]
		if !ok {
			unknown = append(unknown, wire)
			continue
		}
		out[wire] = value
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return nil, fmt.Errorf(
			"masked write names %d field(s) the encoded object does not carry: %s.\n\n"+
				"Either the wire name is wrong, or the field is unset and was omitted by the encoder. "+
				"Names are wire names, as they appear in the struct's json tags",
			len(unknown), strings.Join(unknown, ", "))
	}

	return json.Marshal(out)
}
