package unifi

import (
	"encoding/json"
	"fmt"
	"reflect"
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
	//
	// This is why a masked write still needs the object's discriminator, even
	// when the mask does not name it. Network is the only type here with one:
	// its encoder emits a different field set per purpose, so without Purpose
	// there is no answer to "would this field have been sent", and a mask that
	// guessed would be free to write wan_dns1 onto a vlan-only network --
	// exactly what TestMaskedBodyRejectsFieldsTheEncoderDrops exists to stop.
	// Populating it is a read away, and every real caller has it already.
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to encode the object: %w.\n\n"+
				"A masked write filters what this type's encoder would have sent, so the encoder has "+
				"to be able to run. Where the encoder varies by a discriminator -- Network.Purpose is "+
				"the one such field in this package -- set it, even if the mask does not name it",
			err)
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

	// Names are checked against the type's own tags, not against what the
	// encoder emitted. A field with omitempty is absent from the encoding at
	// its zero value, and rejecting it there would make the mask unable to
	// express the thing masks are for: clearing a note, turning a beeper off.
	// Selecting such a field sends its zero value explicitly.
	declared := declaredWireFields(d)

	var unknown []string
	for _, wire := range fields {
		value, encodedOK := encoded[wire]
		if encodedOK {
			out[wire] = value
			continue
		}
		field, declaredOK := declared[wire]
		if !declaredOK || !encoderEmits(d, field, wire) {
			unknown = append(unknown, wire)
			continue
		}
		zero, err := zeroJSONFor(field)
		if err != nil {
			return nil, fmt.Errorf("masked write cannot express a zero value for %q: %w", wire, err)
		}
		out[wire] = zero
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return nil, fmt.Errorf(
			"masked write names %d field(s) this type does not write: %s.\n\n"+
				"Either the wire name is wrong, or the encoder drops the field for this object -- a "+
				"read-only field, or one the purpose does not carry. Names are wire names, as they "+
				"appear in the struct's json tags",
			len(unknown), strings.Join(unknown, ", "))
	}

	return json.Marshal(out)
}

// declaredWireFields maps the wire names a type declares to their fields.
//
// The mask validates against this rather than against what the encoder
// emitted, so a field omitted at its zero value is still a name the caller
// may legitimately select.
//
// Embedded structs are walked, since the generated settings types embed
// BaseSetting and its key reaches the wire from there.
func declaredWireFields(d any) map[string]reflect.StructField {
	out := map[string]reflect.StructField{}

	typ := reflect.TypeOf(d)
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == nil || typ.Kind() != reflect.Struct {
		return out
	}

	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for i := range t.NumField() {
			field := t.Field(i)
			if field.Anonymous {
				embedded := field.Type
				for embedded.Kind() == reflect.Pointer {
					embedded = embedded.Elem()
				}
				if embedded.Kind() == reflect.Struct {
					walk(embedded)
				}
				continue
			}
			if !field.IsExported() {
				continue
			}
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			if _, taken := out[name]; !taken {
				out[name] = field
			}
		}
	}
	walk(typ)

	return out
}

// encoderEmits reports whether this object's encoder would put wire on the
// wire at all.
//
// A selected field can be missing from the encoding for two opposite reasons,
// and they need opposite answers. omitempty drops it at its zero value, and
// that is exactly the case a mask has to be able to express -- clearing a
// note, turning a beeper off. But the encoder may also drop it deliberately:
// a read-only field is shadowed out, and Network's purpose-specific encoder
// carries no wan_dns1 on a vlan-only network. Sending a zero for those would
// write a key the encoder exists to suppress.
//
// Rather than infer which, ask: set the field to a non-zero value on a copy
// and see whether the key appears. The encoder answers for itself, so this
// stays correct as the encoders change.
func encoderEmits(d any, field reflect.StructField, wire string) bool {
	v := reflect.ValueOf(d)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}

	probe := reflect.New(v.Type())
	probe.Elem().Set(v)

	target := probe.Elem().FieldByIndex(field.Index)
	if !target.CanSet() || !setNonZero(target) {
		return false
	}

	raw, err := json.Marshal(probe.Interface())
	if err != nil {
		return false
	}
	var encoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return false
	}
	_, ok := encoded[wire]
	return ok
}

// setNonZero gives v a value omitempty will not drop, reporting whether it
// managed to.
func setNonZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(true)
	case reflect.String:
		v.SetString("x")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		v.SetMapIndex(reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem())
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
	case reflect.Struct:
		// A struct is never dropped by omitempty, so its zero value already
		// tells us whether the encoder carries it.
	default:
		return false
	}
	return true
}

// zeroJSONFor renders a field's zero value as the controller would see it.
//
// A nil slice or map is rendered as an empty one rather than null, matching
// what nil_as_empty does on the fields the controller rejects nulls for: a
// caller selecting a list field means "no members", not "unset".
func zeroJSONFor(field reflect.StructField) (json.RawMessage, error) {
	typ := field.Type
	if typ.Kind() == reflect.Pointer {
		return json.RawMessage("null"), nil
	}

	zero := reflect.New(typ).Elem()
	switch typ.Kind() {
	case reflect.Slice:
		zero.Set(reflect.MakeSlice(typ, 0, 0))
	case reflect.Map:
		zero.Set(reflect.MakeMap(typ))
	}

	raw, err := json.Marshal(zero.Interface())
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// emptyIfNil renders a nil slice as an empty one.
//
// Used by generated MarshalJSON for slices that serialize unconditionally: an
// empty list has to reach the wire to clear the value, but a caller that
// never touched the field holds nil, which marshals as null. The controller
// rejects null where it expects an array -- measured on APGroup.device_macs
// and Device.port_overrides by TestIntegrationNullWrites.
func emptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
