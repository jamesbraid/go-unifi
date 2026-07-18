package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScanExtractedInputsAcceptsKnownShapes(t *testing.T) {
	root := t.TempDir()
	writeScanFixture(t, root, "Device.json", `{"name":".*","nested":{"type":"string"}}`)
	writeScanFixture(t, root, "metadata/sensitive_metadata.json", `{"min_field_size":1,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{}}`)
	writeScanFixture(t, root, "metadata/country_codes_list.json", `[{"name":"Canada","key":"CA","hints":[],"code":"124","channels_ng":[1,2],"channels_ad_ext_1080":[1],"channels_ad_ext_2160":[2],"afc":{"channels_6e":[]}}]`)
	writeScanFixture(t, root, "metadata/event_defs.json", `{"EVT_AP_Test":{"subsystem":"wlan","alert_repeat":true,"alert_sendmail":false,"alert_subject":"Test","key":"EVT_AP_Test","event_enabled":true,"msg":"{ap} connected","is_alert":false,"is_negative":false}}`)
	writeScanFixture(t, root, "metadata/geo_ip_country_codes_list.json", `{"countries":["CA","US"]}`)
	writeScanFixture(t, root, "metadata/legacy_endpoint_segments.json", `["api","hourly.site","upgrade-external"]`)
	writeScanFixture(t, root, "metadata/radio_specification.json", `{"ng":{"20":{"1":{"lowerFrequency":2402,"centerFrequency":2412,"upperFrequency":2422,"subChannels":[1]}}}}`)
	writeScanFixture(t, root, "metadata/ssl-inspection-file-extension.json", `{"json":{"mime":"application/json"}}`)
	writeScanFixture(t, root, "metadata/timezones.json", `{"America/Vancouver":{"TZ":"PST8PDT,M3.2.0,M11.1.0"}}`)
	require.NoError(t, ScanExtractedInputs(root))
}

func TestScanExtractedInputsAcceptsSingletonSchemaEnums(t *testing.T) {
	root := t.TempDir()
	writeScanFixture(t, root, "PortConf.json", `{"route_type":"static-route","op_mode":"switch","upgrade_mode":"upgrade"}`)
	require.NoError(t, ScanExtractedInputs(root))
}

func TestScanExtractedInputsAcceptsObservedFalseBooleanLiteral(t *testing.T) {
	root := t.TempDir()
	writeScanFixture(t, root, "Setting.json", `{"super_mgmt":{"default_site_device_auth_password_alert":"false"}}`)
	require.NoError(t, ScanExtractedInputs(root))
}

func TestScanExtractedInputsRejectsUnreviewedShortSingletons(t *testing.T) {
	for _, value := range []string{"password", "letmein", "secret123", "true"} {
		t.Run(value, func(t *testing.T) {
			root := t.TempDir()
			writeScanFixture(t, root, "PortConf.json", `{"validator":"`+value+`"}`)
			err := ScanExtractedInputs(root)
			require.Error(t, err)
			require.ErrorContains(t, err, "/validator")
		})
	}
}

func TestScanExtractedInputsReportsRejectedSchemaJSONPath(t *testing.T) {
	root := t.TempDir()
	writeScanFixture(t, root, "PortConf.json", `{"outer":{"op_mode":"literal-secret-value"}}`)
	err := ScanExtractedInputs(root)
	require.Error(t, err)
	require.ErrorContains(t, err, "/outer/op_mode")
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
	writeScanFixture(t, root, "metadata/source.json", `{"os_version":"5.1.21","network_version":"10.4.57","firmware_id":"f5e2a400-1111-4222-8333-123456789abc","installer_url":"https://user:pass@fw-download.ui.com/file","installer_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","installer_md5":"","product":"unifi-os-server","platform":"linux-x64","channel":"release","schema_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","sensitivity_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","notice_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","policy_version":"1","installer_size":1,"created":null,"updated":null,"artifacts":{},"missing_optional":[]}`)
	require.ErrorContains(t, ScanExtractedInputs(root), "installer URL")
}

func TestScanExtractedInputsRejectsOpaqueValuesInEverySensitivePosition(t *testing.T) {
	opaque := []struct{ name, body string }{
		{"default name", `{"min_field_size":1,"default_names":["AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj"],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{}}`},
		{"system property", `{"min_field_size":1,"default_names":[],"sensitive_system_properties":["abcdefabcdefabcdefabcdefabcdefabcdef"],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{}}`},
		{"collection key", `{"min_field_size":1,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{"AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj":[]},"sensitive_distinct_db_fields_by_collection":{}}`},
		{"field path", `{"min_field_size":1,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{"site":["abcdefabcdefabcdefabcdefabcdefabcdef"]},"sensitive_distinct_db_fields_by_collection":{}}`},
		{"distinct path", `{"min_field_size":1,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{"site":"AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj"}}`}}
	for _, tc := range opaque {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeScanFixture(t, root, "metadata/sensitive_metadata.json", tc.body)
			require.Error(t, ScanExtractedInputs(root))
		})
	}
}

func TestScanExtractedInputsRejectsOpaqueRadioBandAndSchemaTokens(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"radio band", "metadata/radio_specification.json", `{"na":{"20":{"36":{"lowerFrequency":1,"centerFrequency":2,"upperFrequency":3,"subChannels":[36],"band":"AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj"}}}}`},
		{"lower hex", "Device.json", `{"value":"abcdefabcdefabcdefabcdefabcdefabcdef.*"}`},
		{"letters only", "Device.json", `{"value":"AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj.*"}`}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeScanFixture(t, root, tc.path, tc.body)
			require.Error(t, ScanExtractedInputs(root))
		})
	}
}

func TestScanExtractedInputsRejectsBlockCaseOpaqueSchemaAndSensitivity(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"upper then lower schema", "Device.json", `{"value":"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop.*"}`}, {"lower then upper schema", "Device.json", `{"value":"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOP.*"}`}, {"upper then lower sensitive", "metadata/sensitive_metadata.json", `{"min_field_size":1,"default_names":[],"sensitive_system_properties":["ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop"],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{}}`}, {"lower then upper sensitive", "metadata/sensitive_metadata.json", `{"min_field_size":1,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{"site":["abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOP"]},"sensitive_distinct_db_fields_by_collection":{}}`}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeScanFixture(t, root, tc.path, tc.body)
			require.Error(t, ScanExtractedInputs(root))
		})
	}
}

func TestScanExtractedInputsAllowsObservedLongEventIdentifier(t *testing.T) {
	root := t.TempDir()
	body := eventFixture("Auto backup failed", "backup failed")
	body = strings.Replace(body, "EVT_AP_Test", "EVT_AD_AutoBackupFailedCloudKeySDCardNotFound", 2)
	writeScanFixture(t, root, "metadata/event_defs.json", body)
	require.NoError(t, ScanExtractedInputs(root))
}

func TestScanExtractedInputsEnforcesObservedMetadataInvariants(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"country key", "metadata/country_codes_list.json", `[{"name":"Canada","key":"Canada","code":"124","hints":[],"afc":{}}]`},
		{"country code", "metadata/country_codes_list.json", `[{"name":"Canada","key":"CA","code":"12x","hints":[],"afc":{}}]`},
		{"event key mismatch", "metadata/event_defs.json", `{"EVT_AP_Test":{"subsystem":"wlan","alert_repeat":true,"alert_sendmail":false,"alert_subject":"Test","key":"EVT_AP_Other","event_enabled":true,"msg":"ok","is_alert":false,"is_negative":false}}`},
		{"radio extra", "metadata/radio_specification.json", `{"ng":{"20":{"1":{"lowerFrequency":1,"centerFrequency":2,"upperFrequency":3,"subChannels":[1],"unexpected":true}}}}`},
		{"radio band grammar", "metadata/radio_specification.json", `{"na":{"20":{"36":{"lowerFrequency":1,"centerFrequency":2,"upperFrequency":3,"subChannels":[36],"band":"other"}}}}`}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeScanFixture(t, root, tc.path, tc.body)
			require.Error(t, ScanExtractedInputs(root))
		})
	}
}

func TestScanExtractedInputsRejectsUncheckedProvenanceStrings(t *testing.T) {
	base := map[string]any{"os_version": "5.1.21", "network_version": "10.4.57", "firmware_id": "f5e2a400-1111-4222-8333-123456789abc", "installer_url": "https://fw-download.ui.com/file", "installer_sha256": strings.Repeat("a", 64), "installer_md5": "", "product": "unifi-os-server", "platform": "linux-x64", "channel": "release", "schema_digest": strings.Repeat("a", 64), "sensitivity_digest": strings.Repeat("a", 64), "notice_digest": strings.Repeat("a", 64), "policy_version": "1", "installer_size": 1, "created": nil, "updated": nil, "artifacts": map[string]string{}, "missing_optional": []string{}}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{{"product", func(v map[string]any) { v["product"] = "AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj" }}, {"firmware", func(v map[string]any) { v["firmware_id"] = "abcdefabcdefabcdefabcdefabcdefabcdef" }}, {"version", func(v map[string]any) { v["network_version"] = "NotAVersion" }}, {"artifact", func(v map[string]any) {
		v["artifacts"] = map[string]string{"AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj": strings.Repeat("a", 64)}
	}}, {"missing optional", func(v map[string]any) { v["missing_optional"] = []string{"AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj"} }}} {
		t.Run(tc.name, func(t *testing.T) {
			copy := maps.Clone(base)
			tc.mutate(copy)
			body, err := json.Marshal(copy)
			require.NoError(t, err)
			root := t.TempDir()
			writeScanFixture(t, root, "metadata/source.json", string(body))
			require.Error(t, ScanExtractedInputs(root))
		})
	}
}

func TestScanExtractedInputsRejectsOpaqueDynamicMetadataStrings(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"timezone key", "metadata/timezones.json", `{"AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj":{"TZ":"GMT0"}}`}, {"timezone value", "metadata/timezones.json", `{"America/Vancouver":{"TZ":"AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj"}}`}, {"mime key", "metadata/ssl-inspection-file-extension.json", `{"AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj":{"mime":"application/json"}}`}, {"mime value", "metadata/ssl-inspection-file-extension.json", `{"json":{"mime":"application/AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj"}}`}, {"event subject", "metadata/event_defs.json", eventFixture("AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGhIj", "ok")}, {"event message", "metadata/event_defs.json", eventFixture("ok", "ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMN")}, {"all lower schema", "Device.json", `{"value":"abcdefghijklmnopqrstuvwxyzabcdefghijklmnop.*"}`}, {"all upper schema", "Device.json", `{"value":"ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOP.*"}`}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeScanFixture(t, root, tc.path, tc.body)
			require.Error(t, ScanExtractedInputs(root))
		})
	}
}
func eventFixture(subject, msg string) string {
	body, _ := json.Marshal(map[string]any{"EVT_AP_Test": map[string]any{"subsystem": "wlan", "alert_repeat": true, "alert_sendmail": false, "alert_subject": subject, "key": "EVT_AP_Test", "event_enabled": true, "msg": msg, "is_alert": false, "is_negative": false}})
	return string(body)
}

func writeScanFixture(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}
