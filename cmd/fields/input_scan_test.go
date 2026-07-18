package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScanExtractedInputsAcceptsKnownShapes(t *testing.T) {
	root := t.TempDir()
	writeScanFixture(t, root, "Device.json", `{"name":".*","nested":{"type":"string"}}`)
	writeScanFixture(t, root, "metadata/sensitive_metadata.json", `{"min_field_size":1,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{}}`)
	writeScanFixture(t, root, "metadata/event_defs.json", `{"events":[]}`)
	require.NoError(t, ScanExtractedInputs(root))
}

func TestScanExtractedInputsRejectsSecretsAndUnknownMetadata(t *testing.T) {
	tests := []struct{ name, path, body string }{
		{"PEM", "Device.json", `{"value":"-----BEGIN PRIVATE KEY-----\\nabc"}`},
		{"JWT", "Device.json", `{"value":"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signaturevalue"}`},
		{"AWS", "Device.json", `{"value":"AKIAIOSFODNN7EXAMPLE"}`},
		{"unexpected scalar", "Device.json", `{"value":"literal-secret-value"}`},
		{"unknown metadata", "metadata/new.json", `{}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeScanFixture(t, root, tc.path, tc.body)
			require.Error(t, ScanExtractedInputs(root))
		})
	}
}

func writeScanFixture(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}
