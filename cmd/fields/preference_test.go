package main

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-codegen-spec/resource"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

func TestGeneratePreferenceFile(t *testing.T) {
	overrides := map[string]resourceOverride{
		"Thing": {Preference: map[string]fields.Preference{
			"setting_preference": {Owns: []string{"beta", "alpha"}, Measured: "10.4.57"},
		}},
		// A measured empty set has to survive into the generated map: it
		// says "probed, owns nothing", which is not the same as absent.
		"Quiet": {Preference: map[string]fields.Preference{
			"setting_preference": {Measured: "10.4.57"},
		}},
		"NoPreferences": {Field: map[string]fieldOverride{"x": {Name: "X"}}},
	}

	var out []byte
	withOverrides(t, overrides, func() {
		var err error
		out, err = generatePreferenceFile(map[string]bool{"Thing": true, "Quiet": true, "NoPreferences": true})
		require.NoError(t, err)
	})

	src := string(out)
	require.Contains(t, src, `"Thing": {`)
	require.Contains(t, src, `"Quiet": {`)
	require.Contains(t, src, `"setting_preference": {},`)
	require.NotContains(t, src, `"NoPreferences"`)

	// Owned names are sorted, so regeneration does not churn the file on
	// map iteration order.
	require.Less(t, indexOf(src, `"alpha"`), indexOf(src, `"beta"`))
}

// TestGeneratePreferenceFileRejectsVanishedResource covers a table left
// behind after a resource leaves the schema. Emitting it would produce a map
// entry describing an object the SDK no longer has.
func TestGeneratePreferenceFileRejectsVanishedResource(t *testing.T) {
	overrides := map[string]resourceOverride{
		"Gone": {Preference: map[string]fields.Preference{
			"setting_preference": {Owns: []string{"alpha"}},
		}},
	}

	withOverrides(t, overrides, func() {
		_, err := generatePreferenceFile(map[string]bool{})
		require.ErrorContains(t, err, "which this run did not generate")
	})
}

// TestDescribePreferenceNamesSpecAttributes pins the thing that is easy to
// get wrong: a description is read beside the attribute name a practitioner
// types, and the wire name is not always that name. ipv6_setting_preference
// is emitted as ipv_6_setting_preference.
func TestDescribePreferenceNamesSpecAttributes(t *testing.T) {
	r := resourceWithFields("Thing", map[string]*FieldInfo{
		"IPV6SettingPreference": NewFieldInfo("IPV6SettingPreference", "ipv6_setting_preference", "string", "", true, false, true, ""),
		"IPV6RAEnabled":         NewFieldInfo("IPV6RAEnabled", "ipv6_ra_enabled", "bool", "", false, false, false, ""),
	})

	overrides := map[string]resourceOverride{
		"Thing": {Preference: map[string]fields.Preference{
			"ipv6_setting_preference": {Owns: []string{"ipv6_ra_enabled"}, Measured: "10.4.57"},
		}},
	}

	withOverrides(t, overrides, func() {
		mode := &resource.Attribute{Name: "x", String: &resource.StringAttribute{}}
		describePreference(r, r.Types["Thing"].Fields["IPV6SettingPreference"], mode)
		require.NotNil(t, mode.String.Description)
		require.Contains(t, *mode.String.Description, "ipv_6_ra_enabled")
		require.Contains(t, *mode.String.Description, "10.4.57")

		owned := &resource.Attribute{Name: "y", Bool: &resource.BoolAttribute{}}
		describePreference(r, r.Types["Thing"].Fields["IPV6RAEnabled"], owned)
		require.NotNil(t, owned.Bool.Description)
		require.Contains(t, *owned.Bool.Description, `ipv_6_setting_preference is "auto"`)

		// A field in neither role is left alone.
		other := &resource.Attribute{Name: "z", Bool: &resource.BoolAttribute{}}
		describePreference(r, NewFieldInfo("Unrelated", "unrelated", "bool", "", false, false, false, ""), other)
		require.Nil(t, other.Bool.Description)
	})
}

func TestModeDescriptionEmptySetReadsAsMeasured(t *testing.T) {
	r := resourceWithFields("Thing", map[string]*FieldInfo{})
	got := modeDescription(r, fields.Preference{Measured: "10.4.57"})
	require.Contains(t, got, "governs no fields")
	require.Contains(t, got, "10.4.57")
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
