package unifi

import (
	"maps"
	"reflect"
	"testing"
)

// This file guards the encoder's emission rules against drifting from the
// generated Network declaration.
//
// The encoder used to re-declare every field per purpose in alias structs,
// and these tests read those declarations from the source to catch a copied
// tag gaining an omitempty the struct does not have. The alias structs are
// gone: emission now derives each rule from the generated struct's own json
// tag, so a per-purpose tag cannot drift because there is no per-purpose
// tag. What is left to guard is the exception tables that deliberately
// deviate from the generated declaration, and the behaviour the old checks
// pinned.

// A FALSE BOOL MUST REACH THE WIRE for every purpose that sends the field.
//
// MEASURED, not theorised, back when the alias structs made it possible: a
// site-vpn Network with IPSecPfs=false emitted no ipsec_pfs at all, so PFS
// could be turned on and never off, by this library and therefore by every
// caller of it. Today an override closure is the only code that could
// reintroduce the bug; this test would catch it doing so.
func TestPurposeEncodersDoNotDropFalseBools(t *testing.T) {
	byWire := networkFieldByWire()
	typ := reflect.TypeFor[Network]()

	for purpose, fields := range networkPurposeFields {
		t.Run(purpose, func(t *testing.T) {
			emitted := marshalKeys(t, &Network{Purpose: purpose})

			checked := 0
			for _, wire := range fields {
				field, ok := byWire[wire]
				if !ok || field.omitEmpty {
					continue
				}
				if typ.FieldByIndex(field.index).Type.Kind() != reflect.Bool {
					continue
				}
				checked++
				if value, present := emitted[wire]; !present {
					t.Errorf("%s is not emitted for a zero Network, so a false is dropped "+
						"and the setting can be turned on and never off", wire)
				} else if value != false {
					t.Errorf("%s is emitted as %v for a zero Network, want false", wire, value)
				}
			}
			if checked == 0 {
				t.Fatalf("purpose %q lists no unconditional bools; the check has stopped covering anything", purpose)
			}
		})
	}
}

// THE STRING DRIFT IS RECORDED RATHER THAN FIXED, and the difference is
// whether the zero value is legal.
//
// A false is always a meaningful bool. An empty string may not be: several
// controller fields reject "" and are omitted deliberately, which is why the
// provider has an optStr helper doing the same thing on its side.
//
// Pinning the list keeps the check useful in the meantime: a NEW deviation
// fails here, and the day one of these is measured it comes off the list.
//
// The four dhcpd_dns_N slots came off it on 2026-08-28. They were the ones
// this comment called probable defects, and measuring them on 10.6.101 said
// so: omitting a slot leaves the stored value alone and "" is what clears
// it, so a caller emptying a DNS list could not say so. They are *string
// now, and an explicit empty reaches the wire -- see clearableSlots in
// network_encode_empty_test.go.
//
// domain_name came off it on 2026-09-13, for the same reason and with the
// same evidence: 10.6.101 clears it on "" and preserves it on omission, and
// the generated field is already a *string, so an explicit empty can reach
// the wire without every write carrying one.
//
// The two that remain are measured too, and stay for reasons the
// measurement gave rather than for want of one. dhcpd_boot_server rejects
// ""; dropping it is correct. mac_override accepts "" and clears on it, and
// the generated field is a plain string, so sending "" unconditionally would
// clear it on every write -- a caller who wants it cleared says so with
// UpdateNetworkFields instead. Letting the unmasked write express it means
// making the field a *string, which is a breaking change to an exported
// field and belongs with the next major bump.
func TestKnownStringOmitemptyDriftIsUnchanged(t *testing.T) {
	droppedEmptyByMeasurement := map[string]bool{
		"dhcpd_boot_server": true,
		"mac_override":      true,
	}

	if !maps.Equal(networkEmptyStringOmitted, droppedEmptyByMeasurement) {
		t.Errorf("networkEmptyStringOmitted = %v, want %v; an entry only joins or leaves with a "+
			"controller measurement of what an explicit \"\" does",
			networkEmptyStringOmitted, droppedEmptyByMeasurement)
	}

	// An entry is only meaningful for a field the generated struct would
	// otherwise send unconditionally: a plain string without omitempty.
	byWire := networkFieldByWire()
	typ := reflect.TypeFor[Network]()
	for wire := range networkEmptyStringOmitted {
		field, ok := byWire[wire]
		if !ok {
			t.Errorf("%s is listed in networkEmptyStringOmitted but is not on the generated struct; take it off the list", wire)
			continue
		}
		if field.omitEmpty || typ.FieldByIndex(field.index).Type.Kind() != reflect.String {
			t.Errorf("%s is listed in networkEmptyStringOmitted but the generated declaration "+
				"already omits its empty value; the entry is dead, take it off the list", wire)
		}
	}
}

// TestNetworkPurposeFieldListsResolve keeps the per-purpose lists honest
// against regeneration: every listed wire name must be a field on the
// generated Network struct, or a synthetic key the purpose derives, and no
// list may name a field twice. A regeneration that renames or removes a
// field fails here instead of erroring at marshal time.
func TestNetworkPurposeFieldListsResolve(t *testing.T) {
	byWire := networkFieldByWire()

	for purpose, fields := range networkPurposeFields {
		overrides := (&Network{Purpose: purpose}).networkPurposeOverrides()
		seen := map[string]bool{}
		for _, wire := range fields {
			if seen[wire] {
				t.Errorf("purpose %q lists %q twice", purpose, wire)
			}
			seen[wire] = true

			if _, generated := byWire[wire]; generated {
				continue
			}
			if _, derived := overrides[wire]; derived {
				// Synthetic keys also need their entry in
				// networkEncoderSyntheticKeys; the value-flow test checks that.
				continue
			}
			t.Errorf("purpose %q lists %q, which is neither on the generated Network struct "+
				"nor derived by the purpose's overrides; the list is stale", purpose, wire)
		}
	}
}

// TestNetworkEncoderExceptionTablesAreLive fails on an exception entry
// nothing uses or whose field no longer has the shape the exception assumes.
// A measured exception that has drifted into fiction is worse than none: it
// documents behaviour the encoder does not have.
func TestNetworkEncoderExceptionTablesAreLive(t *testing.T) {
	byWire := networkFieldByWire()
	typ := reflect.TypeFor[Network]()

	listed := map[string]bool{}
	for _, fields := range networkPurposeFields {
		for _, wire := range fields {
			listed[wire] = true
		}
	}

	check := func(table map[string]bool, name string, valid func(reflect.Type) bool, shape string) {
		for wire := range table {
			if !listed[wire] {
				t.Errorf("%s lists %s but no purpose sends it; the entry is dead", name, wire)
				continue
			}
			field, ok := byWire[wire]
			if !ok {
				t.Errorf("%s lists %s but the generated struct has no such field", name, wire)
				continue
			}
			if !valid(typ.FieldByIndex(field.index).Type) {
				t.Errorf("%s lists %s, which is not %s any more; the measured exception no longer applies as written", name, wire, shape)
			}
		}
	}

	check(networkClearableSlots, "networkClearableSlots", func(ft reflect.Type) bool {
		return ft.Kind() == reflect.Pointer && ft.Elem().Kind() == reflect.String
	}, "a *string")
	check(networkAlwaysArrays, "networkAlwaysArrays", func(ft reflect.Type) bool {
		return ft.Kind() == reflect.Slice
	}, "a slice")
	check(networkEmptyStringOmitted, "networkEmptyStringOmitted", func(ft reflect.Type) bool {
		return ft.Kind() == reflect.String
	}, "a string")
}
