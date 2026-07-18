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
	writeScanFixture(t, root, "metadata/country_codes_list.json", `[{"name":"Canada","key":"CA","hints":[],"code":"124","channels_ng":[1,2],"afc":{"channels_6e":[]}}]`)
	writeScanFixture(t, root, "metadata/event_defs.json", `{"EVT_AP_Test":{"subsystem":"wlan","alert_repeat":true,"alert_sendmail":false,"alert_subject":"Test","key":"EVT_AP_Test","event_enabled":true,"msg":"{ap} connected","is_alert":false,"is_negative":false}}`)
	writeScanFixture(t, root, "metadata/geo_ip_country_codes_list.json", `{"countries":["CA","US"]}`)
	writeScanFixture(t, root, "metadata/legacy_endpoint_segments.json", `["api","hourly.site","upgrade-external"]`)
	writeScanFixture(t, root, "metadata/radio_specification.json", `{"ng":{"20":{"1":{"lowerFrequency":2402,"centerFrequency":2412,"upperFrequency":2422,"subChannels":[1]}}}}`)
	writeScanFixture(t, root, "metadata/ssl-inspection-file-extension.json", `{"json":{"mime":"application/json"}}`)
	writeScanFixture(t, root, "metadata/timezones.json", `{"America/Vancouver":{"TZ":"PST8PDT,M3.2.0,M11.1.0"}}`)
	require.NoError(t, ScanExtractedInputs(root))
}

func TestScanExtractedInputsRejectsSecretsAndUnknownMetadata(t *testing.T) {
	tests := []struct{ name, path, body string }{
		{"PEM", "Device.json", `{"value":"-----BEGIN PRIVATE KEY-----\\nabc"}`},
		{"JWT", "Device.json", `{"value":"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signaturevalue"}`},
		{"AWS", "Device.json", `{"value":"AKIAIOSFODNN7EXAMPLE"}`},
		{"unexpected scalar", "Device.json", `{"value":"literal-secret-value"}`},
		{"unknown metadata", "metadata/new.json", `{}`},
		{"high entropy schema literal", "Device.json", `{"value":"QWxhZGRpbjpvcGVuIHNlc2FtZV9hYmNkZWZnaGlqa2xtbm9wPT0="}`},
		{"high entropy regex ending", "Device.json", `{"value":"QWxhZGRpbjpvcGVuIHNlc2FtZV9hYmNkZWZnaGlqa2xtbm9wPT0=.*"}`},
		{"bad timezone shape", "metadata/timezones.json", `[{"unexpected":[]}]`},
		{"bad event shape", "metadata/event_defs.json", `{"events":"not-an-array"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeScanFixture(t, root, tc.path, tc.body)
			require.Error(t, ScanExtractedInputs(root))
		})
	}
}

func TestScanExtractedInputsValidatesSourceMetadataURL(t *testing.T) {
	root := t.TempDir()
	writeScanFixture(t, root, "Device.json", `{"name":".*"}`)
	writeScanFixture(t, root, "metadata/source.json", `{"os_version":"5.1.21","network_version":"10.4.57","firmware_id":"id","installer_url":"https://user:pass@fw-download.ui.com/file","installer_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","installer_md5":"","product":"unifi-os-server","platform":"linux-x64","channel":"release","schema_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","sensitivity_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","notice_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","policy_version":"1","installer_size":1,"created":null,"updated":null,"artifacts":{},"missing_optional":[]}`)
	require.ErrorContains(t, ScanExtractedInputs(root), "installer URL")
}

func writeScanFixture(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}
