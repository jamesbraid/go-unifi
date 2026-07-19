package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeSensitiveMetadata(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "sensitive_metadata.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadSensitiveMetadata(t *testing.T) {
	path := writeSensitiveMetadata(t, `{
		"sensitive_db_fields_by_collection": {
			"networkconf": ["name", "x_wan_password", "wan_username"],
			"radiusprofile": ["auth_servers.x_secret"]
		},
		"sensitive_distinct_db_fields_by_collection": {
			"setting": ["lte_password", "lte_sim_pin", "hostname"],
			"rogue": "essid"
		}
	}`)

	index, err := loadSensitiveMetadata(path)
	require.NoError(t, err)

	require.True(t, index["networkconf"]["x_wan_password"])
	require.True(t, index["networkconf"]["name"])
	// Dotted paths are indexed by leaf.
	require.True(t, index["radiusprofile"]["x_secret"])
	require.True(t, index["setting"]["lte_password"])
	// Bare-string entries parse too.
	require.True(t, index["rogue"]["essid"])
	require.False(t, index["setting"]["x_wan_password"])
}

func TestLoadSensitiveMetadataMissingFile(t *testing.T) {
	index, err := loadSensitiveMetadata(filepath.Join(t.TempDir(), "nope.json"))
	require.NoError(t, err)
	require.Nil(t, index)
}

func TestSensitivePtr(t *testing.T) {
	index := sensitiveIndex{
		"networkconf": {
			"name":               true,
			"wan_username":       true,
			"ipsec_key_exchange": true,
			"x_wan_password":     true,
		},
		"setting": {
			"lte_password":            true,
			"lte_sim_pin":             true,
			"secret_verifier_encoded": true,
			"hostname":                true,
			"root_certificate":        true,
		},
	}
	gen := NewSpecificationGenerator("unifi", index)

	for _, tc := range []struct {
		collection string
		jsonName   string
		sensitive  bool
	}{
		// x_ prefix rule applies with or without metadata backing.
		{"networkconf", "x_wan_password", true},
		{"networkconf", "x_not_listed_anywhere", true},
		// Metadata-listed secrets without the x_ prefix.
		{"setting", "lte_password", true},
		{"setting", "lte_sim_pin", true},
		{"setting", "secret_verifier_encoded", true},
		// Anonymization-only metadata entries stay visible.
		{"networkconf", "name", false},
		{"networkconf", "wan_username", false},
		{"setting", "hostname", false},
		{"setting", "root_certificate", false},
		// Protocol setting that merely contains "key".
		{"networkconf", "ipsec_key_exchange", false},
		// Secret-looking name NOT listed in the metadata: not marked.
		{"networkconf", "some_password_mode", false},
	} {
		t.Run(tc.collection+"/"+tc.jsonName, func(t *testing.T) {
			r := &ResourceInfo{Collection: tc.collection}
			field := &FieldInfo{JSONName: tc.jsonName}

			got := gen.sensitivePtr(r, field)
			if tc.sensitive {
				require.NotNil(t, got)
				require.True(t, *got)
			} else {
				require.Nil(t, got)
			}
		})
	}
}
