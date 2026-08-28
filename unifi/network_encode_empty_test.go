package unifi

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The encoder must not put an empty string on the wire for an optional
// field. Ever.
//
// Optional fields are modelled as *string with omitempty, which says "absent
// when unset". A caller holding a pointer to "" defeats that -- from a
// framework that maps an unset attribute that way, or from reading an object
// back, editing it and writing it again -- and the key goes out as "".
//
// This rule was originally per-field, driven by the schema pattern: drop the
// empty string where the pattern refused one, keep it where the pattern
// allowed one, on the reasoning that "" was the caller's only way to clear
// such a field. TestIntegrationClearingSemantics measured both halves of
// that and found them wrong:
//
//   - Omitting a key clears the stored value. PUT is a full document
//     replace, not a merge, on all 39 fields probed across three resources.
//     So dropping a field is never worse than sending it empty.
//   - The pattern does not predict what the controller accepts. dhcpd_gateway,
//     dhcpd_ntp_1, dhcpd_boot_server, dhcpd_start and dhcpd_stop all carry ^$
//     in their published pattern and all reject "". The controller is
//     stricter than its own schema.
//
// Which left one rule: an optional *string that is empty is absent. It
// clears the field just the same, and it never hands the controller a value
// it refuses.
//
// That rule now has exactly one class of exception, measured on 10.6.101 and
// listed in clearableSlots below. The eight DHCP slot fields behave the
// opposite way round: omitting one PRESERVES the stored value, and sending
// "" is what clears it. Seven of the eight accept "" outright; dhcpd_ntp_1
// accepts it only in a write that also turns dhcpd_ntp_enabled off, which is
// a pairing constraint rather than a refusal.
//
// So for those eight, dropping the empty string is not a harmless
// simplification -- it is the reason a caller could not empty a DHCP DNS,
// NTP or WINS list at all. They are sent as "" deliberately.
//
// Nothing else is exempt. The other optional strings were never measured
// this way, and the rule stands for them until they are.

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

// clearableSlots are the wire names measured to need an explicit "" to
// clear, listed in one place so the exception stays a short, checkable list
// rather than a habit. Every entry was measured on 10.6.101 by seeding a
// corporate network with all eight populated, then writing each one back
// both ways.
var clearableSlots = map[string]bool{
	"dhcpd_dns_1": true, "dhcpd_dns_2": true, "dhcpd_dns_3": true, "dhcpd_dns_4": true,
	"dhcpd_ntp_1": true, "dhcpd_ntp_2": true,
	"dhcpd_wins_1": true, "dhcpd_wins_2": true,
}

// TestNetworkEncoderDropsEmptyStrings fails when the encoder puts an empty
// string on the wire for an optional field, and when it drops one for a
// field that can only be cleared that way.
func TestNetworkEncoderDropsEmptyStrings(t *testing.T) {
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
				sendsEmpty := present && value == ""

				if clearableSlots[wire] {
					if !sendsEmpty {
						t.Errorf("%s drops an explicit empty; it must reach the wire as \"\". "+
							"Measured on 10.6.101: omitting this field leaves the stored value "+
							"alone, so \"\" is the only way a caller can clear it. Do not wrap "+
							"it in nilIfEmpty.", wire)
					}
					continue
				}

				if sendsEmpty {
					t.Errorf("%s is emitted as \"\" for an unset pointer; wrap it in nilIfEmpty. "+
						"Omitting it clears the field just the same, and several fields reject "+
						"\"\" outright. If this field is one a caller has to clear explicitly, "+
						"measure it and add it to clearableSlots rather than removing the wrap.", wire)
				}
			}
		})
	}
}
