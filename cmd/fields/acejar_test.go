package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, content, 0o644))
	return p
}

func TestFindInternalJar(t *testing.T) {
	aceJar := writeTempFile(t, "ace.jar", buildTestAceJar(t, map[string]string{
		"api/fields/NetworkConf.json": `{}`,
	}, "10.4.57"))

	internal, err := findInternalJar(aceJar)
	require.NoError(t, err)
	assert.Contains(t, string(internal), "PK")
}

func TestReadNetworkVersion(t *testing.T) {
	aceJar := writeTempFile(t, "ace.jar", buildTestAceJar(t, nil, "10.4.57"))

	v, err := readNetworkVersion(aceJar)
	require.NoError(t, err)
	assert.Equal(t, "10.4.57", v)
}

func TestExtractDefs(t *testing.T) {
	aceJar := writeTempFile(t, "ace.jar", buildTestAceJar(t, map[string]string{
		"api/fields/NetworkConf.json": `{"name":".{1,128}"}`,
		"api/fields/WlanConf.json":    `{"ssid":".{1,32}"}`,
		"sensitive_metadata.json":     `{"sensitive_db_fields_by_collection":{}}`,
		"timezones.json":              `[]`,
		"com/ubnt/SomeClass.class":    "CAFEBABE",
		"META-INF/MANIFEST.MF":        "Manifest-Version: 1.0",
	}, "10.4.57"))

	internal, err := findInternalJar(aceJar)
	require.NoError(t, err)

	fieldsDir := t.TempDir()
	n, err := extractDefs(internal, fieldsDir)
	require.NoError(t, err)
	assert.Equal(t, 4, n)

	// api/fields flattened into root
	b, err := os.ReadFile(filepath.Join(fieldsDir, "NetworkConf.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":".{1,128}"}`, string(b))
	assert.FileExists(t, filepath.Join(fieldsDir, "WlanConf.json"))

	// top-level metadata into metadata/
	assert.FileExists(t, filepath.Join(fieldsDir, "metadata", "sensitive_metadata.json"))
	assert.FileExists(t, filepath.Join(fieldsDir, "metadata", "timezones.json"))

	// class files and manifests ignored
	assert.NoFileExists(t, filepath.Join(fieldsDir, "MANIFEST.MF"))
}
