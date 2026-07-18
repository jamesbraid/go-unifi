package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostProcessFieldsDir(t *testing.T) {
	fieldsDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(fieldsDir, "Setting.json"),
		[]byte(`{"usg":{"mdns_enabled":"true|false"},"radius":{"enabled":"true|false"}}`),
		0o644,
	))

	require.NoError(t, postProcessFieldsDir(fieldsDir))

	b, err := os.ReadFile(filepath.Join(fieldsDir, "SettingUsg.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"mdns_enabled":"true|false"}`, string(b))
	assert.FileExists(t, filepath.Join(fieldsDir, "SettingRadius.json"))

	// custom files copied from cmd/fields/custom
	assert.FileExists(t, filepath.Join(fieldsDir, "DnsRecord.json"))
	assert.FileExists(t, filepath.Join(fieldsDir, "FirewallPolicy.json"))

	// idempotent: re-running (cache-hit path) must not error or change output
	require.NoError(t, postProcessFieldsDir(fieldsDir))
	b2, err := os.ReadFile(filepath.Join(fieldsDir, "SettingUsg.json"))
	require.NoError(t, err)
	assert.Equal(t, b, b2)
}

func TestPostProcessFieldsDirNoSettingJSON(t *testing.T) {
	fieldsDir := t.TempDir()

	require.NoError(t, postProcessFieldsDir(fieldsDir))

	// custom defs are still copied
	assert.FileExists(t, filepath.Join(fieldsDir, "DnsRecord.json"))

	// no Setting*.json files appear
	matches, err := filepath.Glob(filepath.Join(fieldsDir, "Setting*.json"))
	require.NoError(t, err)
	assert.Empty(t, matches)
}
