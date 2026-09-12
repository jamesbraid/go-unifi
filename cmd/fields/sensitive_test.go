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

// TestSecretNameRe pins the line between secret material and the
// anonymization-only entries that share sensitive_metadata.json with it.
//
// The rule decides what renderSensitiveFile emits, so it decides what the
// client redacts from the request body it attaches to a non-2xx error. Too
// narrow and key material prints in full; too wide and the error loses the
// names, descriptions and hostnames that make it diagnosable.
func TestSecretNameRe(t *testing.T) {
	for _, tc := range []struct {
		name   string
		secret bool
	}{
		// Secret material.
		{"x_wan_password", true},
		{"lte_password", true},
		{"lte_sim_pin", true},
		{"secret_verifier_encoded", true},
		{"x_secret", true},
		// Every OpenVPN key, which the old substring list missed.
		{"x_ca_key", true},
		{"x_server_key", true},
		{"x_shared_client_key", true},
		{"x_dh_key", true},
		// One underscore away from the old hand-written "pre_shared_key",
		// and printing in full because of it.
		{"wireguard_client_preshared_key", true},

		// Anonymization-only entries, which stay visible.
		{"name", false},
		{"wan_username", false},
		{"hostname", false},
		{"root_certificate", false},
		// A protocol setting that merely contains "key", which is why the
		// key match is suffix-anchored.
		{"ipsec_key_exchange", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.secret, secretNameRe.MatchString(tc.name))
		})
	}
}
