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
// such a field. What replaced it was a blanket drop, justified by a
// measurement that said omitting a key clears the stored value -- so
// dropping a field could never be worse than sending it empty.
//
// That justification is gone. TestIntegrationClearingSemantics was taking
// its verdicts from the PUT's own response, which a v1 write that changed
// nothing answers with an empty data array, so a preserved field read as a
// cleared one. Re-measured against a re-read of the stored document on
// 10.6.101: the v1 rest PUT MERGES. An omitted key preserves the stored
// value on 39 of the 42 fields swept across four collections, and the three
// that do not preserve do something else again -- they do not clear on
// omission either, bar one.
//
// So dropping an empty field is not free. Where the controller accepts ""
// and clears, a caller who empties the field gets a 200 and no change.
// domain_name was in that position and is not any more -- it is a *string,
// so it can say "empty" without saying it on every write, and it joined
// clearableSlots below. mac_override cannot: it is a plain string, which
// cannot carry "the caller asked for empty" at all. Clearing that one goes
// through UpdateNetworkFields, which force-sends the zero value of a field
// the mask names.
//
// The one thing the blanket drop still buys is safety. The pattern does not
// predict what the controller accepts -- dhcpd_gateway, dhcpd_ntp_1,
// dhcpd_boot_server, dhcpd_start and dhcpd_stop all carry ^$ in their
// published pattern and all reject "" -- and for those, dropping the empty
// is what keeps the write from being refused.
//
// clearableSlots below is where the encoder does send "". These nine are
// not special in their omit behaviour; nothing is. They are listed because
// they accept "" and clear, and a caller emptying a DHCP DNS, NTP or WINS
// slot, or a network's search domain, has no other way to say so. Eight
// accept "" outright; dhcpd_ntp_1 accepts it only in a write that also turns
// dhcpd_ntp_enabled off, which is a pairing constraint rather than a
// refusal.

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
//
// The list is short because these were measured, not because they are the
// only fields it applies to. Every collection the clearing probe sweeps
// merges on PUT, so any *string field the controller clears on "" belongs
// here on the same reasoning -- which is how domain_name joined them.
var clearableSlots = map[string]bool{
	"dhcpd_dns_1": true, "dhcpd_dns_2": true, "dhcpd_dns_3": true, "dhcpd_dns_4": true,
	"dhcpd_ntp_1": true, "dhcpd_ntp_2": true,
	"dhcpd_wins_1": true, "dhcpd_wins_2": true,
	"domain_name": true,
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

	for _, purpose := range NetworkPurposes {
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
						"That is not free -- omitting the key preserves the stored value on this "+
						"controller, so a caller who empties a field the controller would have "+
						"cleared gets a 200 and no change -- but several fields reject \"\" "+
						"outright and the drop is what keeps the write from being refused. If "+
						"this field is one a caller has to clear explicitly, check what "+
						"schemas/behavior.json records for it and add it to clearableSlots "+
						"rather than removing the wrap.", wire)
				}
			}
		})
	}
}
