package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadInstaller(t *testing.T) {
	payload := []byte("fake installer bytes")
	sum := sha256.Sum256(payload)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, _ = rw.Write(payload)
	}))
	t.Cleanup(server.Close)

	path, err := downloadInstaller(server.URL, hex.EncodeToString(sum[:]), t.TempDir())
	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestDownloadInstallerHashMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, _ = rw.Write([]byte("payload"))
	}))
	t.Cleanup(server.Close)

	_, err := downloadInstaller(server.URL, "0000000000000000000000000000000000000000000000000000000000000000", t.TempDir())
	assert.ErrorContains(t, err, "sha256 mismatch")
}

func TestExtractInstallerDefsEndToEnd(t *testing.T) {
	aceJar := buildTestAceJar(t, map[string]string{
		"api/fields/NetworkConf.json": `{"name":".{1,128}"}`,
		"sensitive_metadata.json":     `{"sensitive_db_fields_by_collection":{}}`,
	}, "10.4.57")
	layoutDir := writeTestLayout(t, map[string][]byte{
		"usr/lib/unifi/lib/ace.jar": aceJar,
	})
	installer := writeTestInstaller(t, tarDir(t, layoutDir))

	staging, networkVersion, err := extractInstallerDefs(installer, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "10.4.57", networkVersion)
	assert.FileExists(t, filepath.Join(staging, "NetworkConf.json"))
	assert.FileExists(t, filepath.Join(staging, "metadata", "sensitive_metadata.json"))
}

func TestPublishAndFindCachedFieldsDir(t *testing.T) {
	base := t.TempDir()
	staging := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(staging, "NetworkConf.json"), []byte(`{}`), 0o644))

	fieldsDir, err := publishFieldsDir(staging, base, "10.4.57", sourceInfo{
		OsServerVersion: "5.1.21",
		NetworkVersion:  "10.4.57",
		URL:             "https://example/installer",
		SHA256:          "abc",
	})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(base, "v10.4.57"), fieldsDir)
	assert.FileExists(t, filepath.Join(fieldsDir, "NetworkConf.json"))
	assert.FileExists(t, filepath.Join(fieldsDir, "source.json"))

	assert.Equal(t, fieldsDir, findCachedFieldsDir(base, "5.1.21", ""))
	assert.Equal(t, fieldsDir, findCachedFieldsDir(base, "", "https://example/installer"))
	assert.Empty(t, findCachedFieldsDir(base, "9.9.9", ""))
	assert.Empty(t, findCachedFieldsDir(base, "", "https://example/other"))
}

func TestParseOsServerVersionFromName(t *testing.T) {
	assert.Equal(t, "5.1.21", parseOsServerVersionFromName("f5e2-linux-x64-5.1.21-a400c9c6-8328-4634-b223-ebfcf742720a.21-x64"))
	assert.Equal(t, "5.0.8", parseOsServerVersionFromName("162a-linux-arm64-5.0.8-c2775845.8-arm64"))
	assert.Empty(t, parseOsServerVersionFromName("installer"))
}
