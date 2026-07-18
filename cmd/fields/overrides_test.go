package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resourceWithFields builds a minimal ResourceInfo whose base type carries
// the given wire-name -> FieldInfo entries.
func resourceWithFields(structName string, fields map[string]*FieldInfo) *ResourceInfo {
	base := NewFieldInfo(structName, "x", "struct", "", false, false, false, "")
	base.Fields = fields
	return &ResourceInfo{
		StructName: structName,
		Collection: "x",
		Types:      map[string]*FieldInfo{structName: base},
	}
}

func withOverrides(t *testing.T, overrides map[string]resourceOverride, fn func()) {
	t.Helper()
	resourceOverridesOnce.Do(func() {})
	saved := resourceOverridesMap
	resourceOverridesMap = overrides
	t.Cleanup(func() { resourceOverridesMap = saved })
	fn()
}

func TestApplyOverridesPinRenameRetagRemove(t *testing.T) {
	r := resourceWithFields("Thing", map[string]*FieldInfo{
		"   ID":    NewFieldInfo("ID", "_id", "string", "", true, false, false, ""),
		"  Hidden": NewFieldInfo("Hidden", "attr_hidden", "bool", "", true, false, false, ""),
		"Wonky":    NewFieldInfo("Wonky", "wonky_name", "string", "", true, false, true, ""),
	})

	withOverrides(t, map[string]resourceOverride{
		"Thing": {Field: map[string]fieldOverride{
			// Retag the envelope id for a true-v2 object.
			"_id": {JSON: "id"},
			// Drop an envelope field the wire doesn't carry.
			"attr_hidden": {Remove: true},
			// Pin shape and rename.
			"wonky_name": {Name: "Sane", OmitEmpty: boolPtr(false), Pointer: boolPtr(false)},
		}},
	}, func() {
		require.NoError(t, r.applyOverrides())
	})

	base := r.Types["Thing"]
	require.Equal(t, "id", base.Fields["   ID"].JSONName)
	require.NotContains(t, base.Fields, "  Hidden")
	require.NotContains(t, base.Fields, "Wonky")
	sane := base.Fields["Sane"]
	require.NotNil(t, sane)
	require.Equal(t, "wonky_name", sane.JSONName)
	require.False(t, sane.OmitEmpty)
	require.False(t, sane.IsPointer)
}

func TestApplyOverridesAddOnlyWhenMissing(t *testing.T) {
	r := resourceWithFields("Thing", map[string]*FieldInfo{
		"Existing": NewFieldInfo("Existing", "existing", "string", "from-schema", true, false, false, ""),
	})

	withOverrides(t, map[string]resourceOverride{
		"Thing": {Field: map[string]fieldOverride{
			// Present in schema: add is ignored, schema validation kept.
			"existing": {Add: true, Name: "Existing", Type: "string", Validation: "clobbered"},
			// Absent: created with the given shape.
			"compat": {Add: true, Name: "Compat", Type: "bool", OmitEmpty: boolPtr(true), Pointer: boolPtr(true)},
		}},
	}, func() {
		require.NoError(t, r.applyOverrides())
	})

	base := r.Types["Thing"]
	require.Equal(t, "from-schema", base.Fields["Existing"].FieldValidation)
	compat := base.Fields["Compat"]
	require.NotNil(t, compat)
	require.Equal(t, "compat", compat.JSONName)
	require.True(t, compat.OmitEmpty)
	require.True(t, compat.IsPointer)
}

func TestApplyOverridesRejectsUnsafeRetag(t *testing.T) {
	r := resourceWithFields("Thing", map[string]*FieldInfo{
		"   ID": NewFieldInfo("ID", "_id", "string", "", true, false, false, ""),
	})

	withOverrides(t, map[string]resourceOverride{
		"Thing": {Field: map[string]fieldOverride{
			"_id": {JSON: "evil\"`"},
		}},
	}, func() {
		require.ErrorContains(t, r.applyOverrides(), "unsafe json retag")
	})
}

func boolPtr(b bool) *bool { return &b }
