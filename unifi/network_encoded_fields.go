package unifi

import (
	"encoding/json"
	"reflect"
	"slices"
	"sync"
)

// NetworkPurposes are the purpose values Network.MarshalJSON dispatches on.
// A Network with any other purpose is encoded by the corporate marshaler.
var NetworkPurposes = []string{
	PurposeCorporate,
	PurposeGuest,
	PurposeVLANOnly,
	PurposeWAN,
	PurposeSiteVPN,
	PurposeVPNClient,
	PurposeUserVPN,
}

// NetworkEncodedFields returns the wire names Network.MarshalJSON can write
// for a purpose, sorted. An unknown purpose returns the corporate set, which
// is what the encoder falls back to.
//
// Network is the one type here whose encoder varies by a discriminator: each
// purpose sends a different field set, so a name that is writable on a
// corporate network may be one the WAN encoder never emits. A caller that
// builds a masked write has to know which -- maskedBody rejects a name the
// encoder would drop, deliberately, and the alternative to asking is a
// hand-maintained list of exceptions per purpose that nothing checks.
//
// The answer is computed by running the encoder over a Network with every
// field set, not declared beside it, so it cannot describe a version of the
// encoder that no longer exists.
func NetworkEncodedFields(purpose string) []string {
	fields := networkEncodedFields()[purpose]
	if fields == nil {
		fields = networkEncodedFields()[PurposeCorporate]
	}
	return slices.Clone(fields)
}

// NetworkEncodesField reports whether the purpose's encoder can write the
// wire name.
func NetworkEncodesField(purpose, wire string) bool {
	_, found := slices.BinarySearch(networkEncodedFields()[purpose], wire)
	return found
}

// networkEncodedFields resolves every purpose's field set once. Marshalling
// a fully populated Network per purpose is cheap but not free, and the
// answer cannot change at runtime.
var networkEncodedFields = sync.OnceValue(func() map[string][]string {
	out := make(map[string][]string, len(NetworkPurposes))
	for _, purpose := range NetworkPurposes {
		out[purpose] = encodedFieldsFor(purpose)
	}
	return out
})

func encodedFieldsFor(purpose string) []string {
	var n Network
	populateNonZero(reflect.ValueOf(&n).Elem(), 10)

	// A parseable subnet keeps the encoder's DHCP-range calculation from
	// logging about the placeholder the population walk would otherwise
	// leave here. Which keys are emitted is unaffected.
	subnet := "10.0.0.0/24"
	n.IPSubnet = &subnet
	n.Purpose = purpose

	raw, err := json.Marshal(&n)
	if err != nil {
		return nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}

	fields := make([]string, 0, len(payload))
	for wire := range payload {
		fields = append(fields, wire)
	}
	slices.Sort(fields)
	return fields
}

// populateNonZero gives every field in v a value its encoder will not drop,
// so that marshalling the result shows which keys the encoder is capable of
// writing rather than which ones this particular value happens to carry.
//
// depth bounds the walk: the generated types nest, and a self-referential
// shape would otherwise not terminate.
func populateNonZero(v reflect.Value, depth int) {
	if depth <= 0 || !v.CanSet() {
		return
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		populateNonZero(v.Elem(), depth-1)
	case reflect.Slice:
		elem := reflect.New(v.Type().Elem()).Elem()
		populateNonZero(elem, depth-1)
		v.Set(reflect.Append(reflect.MakeSlice(v.Type(), 0, 1), elem))
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		key := reflect.New(v.Type().Key()).Elem()
		populateNonZero(key, depth-1)
		val := reflect.New(v.Type().Elem()).Elem()
		populateNonZero(val, depth-1)
		m.SetMapIndex(key, val)
		v.Set(m)
	case reflect.Struct:
		for i := range v.NumField() {
			populateNonZero(v.Field(i), depth-1)
		}
	}
}
