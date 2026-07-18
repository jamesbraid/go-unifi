package main

import (
	"archive/zip"
	"bytes"
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

type zipEntry struct {
	name    string
	content []byte
}

// buildAceJarEntries returns ace.jar bytes holding exactly the given entries,
// in order.
func buildAceJarEntries(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		require.NoError(t, err)
		_, err = w.Write(e.content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestFindInternalJar(t *testing.T) {
	aceJar := writeTempFile(t, "ace.jar", buildTestAceJar(t, map[string]string{
		"api/fields/NetworkConf.json": `{}`,
	}, "10.4.57"))

	internal, err := findInternalJar(aceJar)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(internal), int64(len(internal)))
	require.NoError(t, err)
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	assert.Contains(t, names, "api/fields/NetworkConf.json")
}

func TestFindInternalJarMissing(t *testing.T) {
	aceJar := writeTempFile(t, "ace.jar", buildAceJarEntries(t,
		zipEntry{"BOOT-INF/classes/product.properties", []byte("version=1.2.3\n")},
	))

	_, err := findInternalJar(aceJar)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal-dependencies.jar")
}

func TestFindInternalJarPrefersExactMatch(t *testing.T) {
	realInternal := buildTestZip(t, "api/fields/NetworkConf.json", []byte(`{}`))
	aceJar := writeTempFile(t, "ace.jar", buildAceJarEntries(t,
		zipEntry{"BOOT-INF/lib/internal-dependencies.jar.bak", []byte("decoy")},
		zipEntry{"BOOT-INF/lib/internal-dependencies.jar", realInternal},
	))

	internal, err := findInternalJar(aceJar)
	require.NoError(t, err)
	assert.Equal(t, realInternal, internal)
}

func TestReadNetworkVersion(t *testing.T) {
	aceJar := writeTempFile(t, "ace.jar", buildTestAceJar(t, nil, "10.4.57"))

	v, err := readNetworkVersion(aceJar)
	require.NoError(t, err)
	assert.Equal(t, "10.4.57", v)
}

func TestReadNetworkVersionMissingProperties(t *testing.T) {
	aceJar := writeTempFile(t, "ace.jar", buildAceJarEntries(t,
		zipEntry{"BOOT-INF/lib/internal-dependencies.jar", buildTestZip(t, "x", []byte("y"))},
	))

	_, err := readNetworkVersion(aceJar)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "product.properties")
}

func TestReadNetworkVersionMissingVersion(t *testing.T) {
	aceJar := writeTempFile(t, "ace.jar", buildAceJarEntries(t,
		zipEntry{"BOOT-INF/classes/product.properties", []byte("product=UniFi\n")},
	))

	_, err := readNetworkVersion(aceJar)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version=")
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

func TestExtractDefsNoFieldDefs(t *testing.T) {
	internal := buildTestZip(t, "com/ubnt/SomeClass.class", []byte("CAFEBABE"))

	_, err := extractDefs(internal, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no api/fields")
}
