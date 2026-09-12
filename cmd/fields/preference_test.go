package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

func TestGeneratePreferenceFile(t *testing.T) {
	tables := map[string]map[string]fields.Preference{
		"Thing": {
			"setting_preference": {Owns: []string{"beta", "alpha"}, Measured: "10.4.57"},
		},
		// A measured empty set has to survive into the generated map: it
		// says "probed, owns nothing", which is not the same as absent.
		"Quiet": {
			"setting_preference": {Measured: "10.4.57"},
		},
	}

	var out []byte
	withPreferences(t, tables, func() {
		var err error
		// NoPreferences is generated but owns no entry, so it must not be
		// emitted.
		out, err = generatePreferenceFile(map[string]bool{"Thing": true, "Quiet": true, "NoPreferences": true})
		require.NoError(t, err)
	})

	src := string(out)
	require.Contains(t, src, `"Thing": {`)
	require.Contains(t, src, `"Quiet": {`)
	require.Contains(t, src, `Mode: "setting_preference", Owns: []string{}}`)
	require.NotContains(t, src, `"NoPreferences"`)

	// Owned names are sorted, so regeneration does not churn the file on
	// map iteration order.
	require.Less(t, indexOf(src, `"alpha"`), indexOf(src, `"beta"`))
}

// TestGeneratePreferenceFileRejectsVanishedResource covers an ownership
// record left behind after a resource leaves the schema. Emitting it would
// produce a map entry describing an object the SDK no longer has.
func TestGeneratePreferenceFileRejectsVanishedResource(t *testing.T) {
	tables := map[string]map[string]fields.Preference{
		"Gone": {
			"setting_preference": {Owns: []string{"alpha"}},
		},
	}

	withPreferences(t, tables, func() {
		_, err := generatePreferenceFile(map[string]bool{})
		require.ErrorContains(t, err, "which this run did not generate")
	})
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
