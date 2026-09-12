package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
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
	// Force the real load first so restoring doesn't leave a nil map for
	// later tests regardless of execution order. This global mutation is
	// also why this package's tests must not use t.Parallel.
	_ = resourceOverrides()
	saved := resourceOverridesMap
	resourceOverridesMap = overrides
	t.Cleanup(func() { resourceOverridesMap = saved })
	fn()
}

// withPreferences swaps the merged ownership tables, the same way and with
// the same no-t.Parallel caveat as withOverrides.
func withPreferences(t *testing.T, tables map[string]map[string]fields.Preference, fn func()) {
	t.Helper()
	_ = preferenceTables()
	saved := preferenceTablesMap
	preferenceTablesMap = tables
	t.Cleanup(func() { preferenceTablesMap = saved })
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
			"wonky_name": {Name: "Sane", OmitEmpty: ptr(false), Pointer: ptr(false)},
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
			"compat": {Add: true, Name: "Compat", Type: "bool", OmitEmpty: ptr(true), Pointer: ptr(true)},
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

func TestApplyOverridesPreferenceNamesMustExist(t *testing.T) {
	fieldsOf := func() map[string]*FieldInfo {
		return map[string]*FieldInfo{
			"SettingPreference": NewFieldInfo("SettingPreference", "setting_preference", "string", "", true, false, true, ""),
			"IGMPSnooping":      NewFieldInfo("IGMPSnooping", "igmp_snooping", "bool", "", false, false, false, ""),
		}
	}

	for _, tc := range []struct {
		name    string
		pref    map[string]fields.Preference
		wantErr string
	}{
		{
			name: "resolves",
			pref: map[string]fields.Preference{
				"setting_preference": {Owns: []string{"igmp_snooping"}, Measured: "10.4.57"},
			},
		},
		{
			// The failure this guard exists for: a typo owns nothing and
			// reports nothing, which is the bug the tables document.
			name: "owned field misspelled",
			pref: map[string]fields.Preference{
				"setting_preference": {Owns: []string{"igmp_snoping"}, Measured: "10.4.57"},
			},
			wantErr: `owns "igmp_snoping", which is not a field on the resource`,
		},
		{
			name: "mode field misspelled",
			pref: map[string]fields.Preference{
				"settings_preference": {Owns: []string{"igmp_snooping"}, Measured: "10.4.57"},
			},
			wantErr: `preference "settings_preference": no such field on the resource`,
		},
		{
			name: "mode owns itself",
			pref: map[string]fields.Preference{
				"setting_preference": {Owns: []string{"setting_preference"}, Measured: "10.4.57"},
			},
			wantErr: `lists itself in owns`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := resourceWithFields("Thing", fieldsOf())
			withPreferences(t, map[string]map[string]fields.Preference{
				"Thing": tc.pref,
			}, func() {
				err := r.applyOverrides()
				if tc.wantErr == "" {
					require.NoError(t, err)
					return
				}
				require.ErrorContains(t, err, tc.wantErr)
			})
		})
	}
}

// nestedResource builds a resource whose base type holds an array sub-object
// and a single sub-object, mirroring the two nested modes in the schema:
// Device.port_overrides ([]DevicePortOverrides) and
// SettingUsg.dns_verification (*SettingUsgDNSVerification).
func nestedResource() *ResourceInfo {
	override := NewFieldInfo("PortOverrides", "port_overrides", "ThingPortOverrides", "", false, true, false, "")
	verification := NewFieldInfo("DNSVerification", "dns_verification", "ThingDNSVerification", "", true, false, true, "")

	r := resourceWithFields("Thing", map[string]*FieldInfo{
		"SettingPreference": NewFieldInfo("SettingPreference", "setting_preference", "string", "", true, false, true, ""),
		"PortOverrides":     override,
		"DNSVerification":   verification,
	})
	// Sub-objects reached by type name, the shape the generator produces for
	// a named nested type.
	r.Types["ThingPortOverrides"] = &FieldInfo{FieldName: "ThingPortOverrides", Fields: map[string]*FieldInfo{
		"SettingPreference": NewFieldInfo("SettingPreference", "setting_preference", "string", "", true, false, true, ""),
		"StpPortMode":       NewFieldInfo("StpPortMode", "stp_port_mode", "bool", "", false, false, false, ""),
	}}
	r.Types["ThingDNSVerification"] = &FieldInfo{FieldName: "ThingDNSVerification", Fields: map[string]*FieldInfo{
		"SettingPreference": NewFieldInfo("SettingPreference", "setting_preference", "string", "", true, false, true, ""),
		"Domain":            NewFieldInfo("Domain", "domain", "string", "", true, false, false, ""),
	}}
	return r
}

func TestApplyOverridesNestedPreferencePaths(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pref    map[string]fields.Preference
		wantErr string
	}{
		{
			// An array container: one mode per element, governing that element.
			name: "through an array",
			pref: map[string]fields.Preference{
				"port_overrides.setting_preference": {Owns: []string{"stp_port_mode"}, Measured: "10.4.57"},
			},
		},
		{
			name: "through a single sub-object",
			pref: map[string]fields.Preference{
				"dns_verification.setting_preference": {Owns: []string{"domain"}, Measured: "10.4.57"},
			},
		},
		{
			// The nested and top-level modes share a wire name. Resolving in
			// the wrong scope would silently annotate the wrong attribute.
			name: "nested and top-level modes coexist",
			pref: map[string]fields.Preference{
				"setting_preference":                {Owns: []string{}, Measured: "10.4.57"},
				"port_overrides.setting_preference": {Owns: []string{"stp_port_mode"}, Measured: "10.4.57"},
			},
		},
		{
			name: "owned name from another object",
			pref: map[string]fields.Preference{
				"port_overrides.setting_preference": {Owns: []string{"domain"}, Measured: "10.4.57"},
			},
			wantErr: `owns "domain", which is not a field on Thing.port_overrides`,
		},
		{
			// Owned names are relative. Accepting a dotted one would invite
			// two spellings of the same thing.
			name: "dotted owned name is rejected",
			pref: map[string]fields.Preference{
				"port_overrides.setting_preference": {Owns: []string{"port_overrides.stp_port_mode"}, Measured: "10.4.57"},
			},
			wantErr: "owned names are relative to Thing.port_overrides",
		},
		{
			name: "container does not exist",
			pref: map[string]fields.Preference{
				"port_overides.setting_preference": {Owns: []string{"stp_port_mode"}, Measured: "10.4.57"},
			},
			wantErr: `no field "port_overides" on the resource`,
		},
		{
			name: "container is not an object",
			pref: map[string]fields.Preference{
				"setting_preference.nested": {Owns: []string{}},
			},
			wantErr: "carries no sub-object to nest into",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := nestedResource()
			withPreferences(t, map[string]map[string]fields.Preference{
				"Thing": tc.pref,
			}, func() {
				err := r.applyOverrides()
				if tc.wantErr == "" {
					require.NoError(t, err)
					return
				}
				require.ErrorContains(t, err, tc.wantErr)
			})
		})
	}
}

// TestApplyOverridesPreferenceUOSPins covers the rule that keeps a console
// pin meaningful: it is a claim about a field the mode owns elsewhere, so
// one naming an unowned field describes nothing and would mean the artifact
// sections had drifted apart.
func TestApplyOverridesPreferenceUOSPins(t *testing.T) {
	fieldsOf := func() map[string]*FieldInfo {
		return map[string]*FieldInfo{
			"SettingPreference": NewFieldInfo("SettingPreference", "setting_preference", "string", "", true, false, true, ""),
			"IGMPSnooping":      NewFieldInfo("IGMPSnooping", "igmp_snooping", "bool", "", false, false, false, ""),
			"MulticastEnhance":  NewFieldInfo("MulticastEnhance", "multicast_enhance_enabled", "bool", "", false, false, false, ""),
		}
	}

	for _, tc := range []struct {
		name    string
		pref    map[string]fields.Preference
		wantErr string
	}{
		{
			name: "uos pin of an owned field",
			pref: map[string]fields.Preference{
				"setting_preference": {
					Owns:        []string{"igmp_snooping", "multicast_enhance_enabled"},
					UOSExcludes: []string{"igmp_snooping"},
					Measured:    "10.4.57",
				},
			},
		},
		{
			name: "uos pin of a field the mode does not own",
			pref: map[string]fields.Preference{
				"setting_preference": {
					Owns:        []string{"igmp_snooping"},
					UOSExcludes: []string{"multicast_enhance_enabled"},
					Measured:    "10.4.57",
				},
			},
			wantErr: `uos_pins names "multicast_enhance_enabled", which is not in owns`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := resourceWithFields("Thing", fieldsOf())
			withPreferences(t, map[string]map[string]fields.Preference{
				"Thing": tc.pref,
			}, func() {
				err := r.applyOverrides()
				if tc.wantErr == "" {
					require.NoError(t, err)
					return
				}
				require.ErrorContains(t, err, tc.wantErr)
			})
		})
	}
}
