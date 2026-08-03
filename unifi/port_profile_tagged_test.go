package unifi

import (
	"reflect"
	"strings"
	"testing"
)

// TestPortProfileDoesNotModelTaggedNetworks holds the SDK half of the
// tagged_networkconf_ids measurement.
//
// TestIntegrationPortProfileTaggedNetworks posts raw maps, so it measures the
// controller and says nothing about the generated type. Those can drift
// apart in the one direction that matters: if a later vendor schema
// reintroduces the field, a regeneration adds PortProfile.TaggedNetworkIDs
// while the controller carries on stripping it. The integration guard stays
// green -- stripping is exactly what it expects -- and the SDK ships the
// silent no-op the compatibility policy exists to forbid, with nothing red
// anywhere.
//
// This runs offline and on every `go test`, unlike the guard it completes,
// so a regeneration that adds the field fails the build immediately.
//
// The two are meant to flip together. When the controller is measured to
// genuinely honor the ids -- persistence plus the behavioural check
// assertTaggedFate now asks for -- restore the pin and delete this test in
// the same change.
func TestPortProfileDoesNotModelTaggedNetworks(t *testing.T) {
	const wire = "tagged_networkconf_ids"

	typ := reflect.TypeOf(PortProfile{})
	for i := range typ.NumField() {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name != wire {
			continue
		}
		t.Fatalf("PortProfile models %s as %s, but the controller strips it from every write "+
			"shape (measured 10.4.57, see TestIntegrationPortProfileTaggedNetworks). Shipping the "+
			"field would make callers set a key that does nothing and trunk no VLANs, with no "+
			"error to notice. If a regeneration added this, the controller behaviour has to be "+
			"re-measured before the field can stay -- and if it is genuinely honored now, flip "+
			"the integration guard and delete this test together", wire, field.Name)
	}
}

// TestPortProfileStillModelsTheExclusionForm is the paired positive: the
// mechanism the SDK does model has to keep existing, or the test above is
// guarding a type that no longer describes port profiles at all.
func TestPortProfileStillModelsTheExclusionForm(t *testing.T) {
	want := map[string]bool{
		"tagged_vlan_mgmt":         false,
		"excluded_networkconf_ids": false,
	}

	typ := reflect.TypeOf(PortProfile{})
	for i := range typ.NumField() {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if _, expected := want[name]; expected {
			want[name] = true
		}
	}

	for wire, found := range want {
		if !found {
			t.Errorf("PortProfile no longer models %s — the exclusion form is the only tagged-VLAN "+
				"semantics portconf retains, so losing it means the model needs re-measuring", wire)
		}
	}
}
