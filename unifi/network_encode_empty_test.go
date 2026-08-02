package unifi

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// The encoder must not put an empty string on the wire for a field whose own
// schema pattern refuses one.
//
// Optional fields are modelled as *string with omitempty, which says
// "absent when unset". A caller holding a pointer to "" -- from a framework
// that maps an unset attribute that way, or from a read-modify-write cycle
// over a controller response -- defeats that, and the key goes out as "".
// Whether that matters is decided per field by the controller's own
// validator, which the generator now exports:
//
//	pattern rejects ""   sending it can only be rejected, so drop the key
//	pattern accepts ""   "" is a storable value; dropping it would take
//	                     away the caller's only way to clear the field
//	no pattern           the schema abstains; see the justified list below
//
// This test drives all three off FieldValidationPatterns, so a schema
// refresh that changes a field's pattern moves it between the cases and
// fails here rather than drifting.

// networkEmptyStringJustified lists wire names the encoder drops when empty
// even though the schema pins no pattern to say whether it should. Every
// entry needs a reason.
var networkEmptyStringJustified = map[string]string{
	"x_ca_crt":            certBlobJustification,
	"x_ca_key":            certBlobJustification,
	"x_server_crt":        certBlobJustification,
	"x_server_key":        certBlobJustification,
	"x_dh_key":            certBlobJustification,
	"x_shared_client_crt": certBlobJustification,
	"x_shared_client_key": certBlobJustification,
	"x_auth_key":          certBlobJustification,
}

// networkEmptyStringDroppedAnyway lists wire names the encoder drops when
// empty even though the schema pattern accepts "". Each needs a reason: by
// default this rule says such a field keeps its empty string, because that
// is how a caller clears it.
var networkEmptyStringDroppedAnyway = map[string]string{
	// Predates this rule -- wrapped in nilIfEmpty by a generic nullability
	// pass, not by a measurement. The pattern permits "", so by the rule
	// here it should keep it. What is measured is that turning
	// dhcpd_ntp_enabled on without dhcpd_ntp_1 gets api.err.NtpAddressInvalid
	// (see networkCrossFieldRules), so absence is not a way to clear the
	// address while the feature is on, and sending "" may well fail the
	// same way. Neither reading is tested, so the existing behaviour stays
	// rather than change on a theory.
	"dhcpd_ntp_1": "predates this rule; clearing semantics unmeasured, see the dhcpd_ntp_enabled row in networkCrossFieldRules",
	"dhcpd_ntp_2": "predates this rule; clearing semantics unmeasured, see the dhcpd_ntp_enabled row in networkCrossFieldRules",
}

// certBlobJustification records why the OpenVPN key material is dropped when
// empty on the model's terms rather than on a measurement.
//
// The schema pins no pattern to these, so whether the controller rejects ""
// is unmeasured, and it cannot be settled by the obvious experiment: a
// payload omitting them is rejected too, for an unrelated reason. (That
// rejection was originally blamed on these fields; it was an out-of-range
// openvpn_encryption_cipher, see network_openvpn_radius_integration_test.go
// for what an openvpn-server actually needs.) They are dropped because
// *string + omitempty means absent-when-unset, and a pointer to "" defeats
// what the model already says.
const certBlobJustification = "unmeasured: schema pins no pattern; dropped because *string + omitempty already means absent-when-unset"

// patternAcceptsEmpty reports whether a field's own validator accepts the
// empty string. The pattern is the controller's, so this asks the controller
// rather than guessing: compile it and see. A handful use lookarounds that
// RE2 will not compile, so those fall back to spotting an explicit ^$
// alternative.
func patternAcceptsEmpty(pattern string) bool {
	if re, err := regexp.Compile(pattern); err == nil {
		return re.MatchString("")
	}
	return strings.Contains(pattern, "^$")
}

// newPointerStringNetwork returns a Network whose every *string field points
// at value, with Purpose set.
//
// Marshalling one of these with "" and again with a non-empty sentinel is
// what separates "the encoder dropped this because it was empty" from "this
// purpose never emits this field at all" -- the two are indistinguishable
// from the empty run alone.
func newPointerStringNetwork(purpose, value string) *Network {
	n := &Network{}
	v := reflect.ValueOf(n).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Pointer && f.Type().Elem().Kind() == reflect.String && f.CanSet() {
			s := value
			f.Set(reflect.ValueOf(&s))
		}
	}
	n.Purpose = purpose
	return n
}

// marshalKeys returns the wire keys a Network marshals to, and their values.
func marshalKeys(t *testing.T, n *Network) map[string]any {
	t.Helper()

	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// TestNetworkEncoderDropsRefusedEmptyStrings fails when the encoder emits ""
// for a field whose schema pattern refuses it, and when it drops one the
// schema has nothing to say about without a justification on record.
func TestNetworkEncoderDropsRefusedEmptyStrings(t *testing.T) {
	patterns := FieldValidationPatterns["Network"]
	if len(patterns) == 0 {
		t.Fatal("no Network patterns in FieldValidationPatterns; the generator did not emit them")
	}

	// Which wire names are *string on the generated struct: only those can
	// carry a pointer to "". A plain string with omitempty already drops.
	pointerString := map[string]bool{}
	typ := reflect.TypeOf(Network{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Type.Kind() != reflect.Pointer || f.Type.Elem().Kind() != reflect.String {
			continue
		}
		if name, _, _ := strings.Cut(f.Tag.Get("json"), ","); name != "" && name != "-" {
			pointerString[name] = true
		}
	}

	for _, purpose := range networkEncoderPurposes {
		t.Run(purpose, func(t *testing.T) {
			empty := marshalKeys(t, newPointerStringNetwork(purpose, ""))
			control := marshalKeys(t, newPointerStringNetwork(purpose, "sentinel"))

			for wire := range pointerString {
				// Only fields this purpose emits at all are in scope.
				if _, emitted := control[wire]; !emitted {
					continue
				}

				value, present := empty[wire]
				pattern, hasPattern := patterns[wire]

				switch {
				case hasPattern && !patternAcceptsEmpty(pattern):
					if present && value == "" {
						t.Errorf("%s is emitted as \"\" but its schema pattern refuses one (%s); wrap it in nilIfEmpty", wire, pattern)
					}
				case hasPattern:
					// The controller stores "" for this field, so dropping
					// it would remove the caller's only way to clear it.
					if !present {
						if _, ok := networkEmptyStringDroppedAnyway[wire]; !ok {
							t.Errorf("%s is dropped when empty, but its schema pattern accepts \"\" (%s); dropping it takes away the caller's way to clear the field", wire, pattern)
						}
					}
				default:
					// No pattern: the schema abstains, so a drop needs a
					// reason on record.
					if !present {
						if _, ok := networkEmptyStringJustified[wire]; !ok {
							t.Errorf("%s is dropped when empty and the schema pins no pattern; add it to networkEmptyStringJustified with a reason, or stop dropping it", wire)
						}
					}
				}
			}
		})
	}
}
