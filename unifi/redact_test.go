package unifi

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRedactSensitivePayload guards that secrets embedded in a request body are
// not leaked when the body is included in an error message
// (ubiquiti-community/terraform-provider-unifi#256).
func TestRedactSensitivePayload(t *testing.T) {
	const (
		wgKey     = "fake-wireguard-key-value"
		pass      = "fake-passphrase-value"
		ipsecPSK  = "fake-ipsec-psk-value"
		radiusSec = "fake-radius-secret-value"
		pw        = "fake-password-value"
	)
	body := []byte(`{
		"name": "wgadmin",
		"purpose": "vpn-server",
		"x_wireguard_private_key": "` + wgKey + `",
		"x_passphrase": "` + pass + `",
		"x_ipsec_pre_shared_key": "` + ipsecPSK + `",
		"vlan": 50,
		"nested": {"radius_secret": "` + radiusSec + `", "ok": "keep"},
		"list": [{"password": "` + pw + `"}]
	}`)

	out := redactSensitivePayload(body)

	for _, leak := range []string{wgKey, pass, ipsecPSK, radiusSec, pw} {
		if strings.Contains(out, leak) {
			t.Errorf("redacted payload still leaks %q: %s", leak, out)
		}
	}

	// Non-sensitive fields must survive.
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("redacted payload is not valid JSON: %v\n%s", err, out)
	}
	if m["name"] != "wgadmin" {
		t.Errorf("name was lost: %v", m["name"])
	}
	if m["x_wireguard_private_key"] != "REDACTED" {
		t.Errorf("private key not redacted: %v", m["x_wireguard_private_key"])
	}
	if nested, ok := m["nested"].(map[string]any); !ok || nested["ok"] != "keep" {
		t.Errorf("nested non-sensitive value lost: %v", m["nested"])
	}

	// Non-JSON body is omitted, not echoed.
	if got := redactSensitivePayload([]byte("not json")); strings.Contains(got, "not json") {
		t.Errorf("non-JSON body echoed: %q", got)
	}
}

// TestRedactCoversTheControllersOwnSensitiveFields is the case the test above
// could not have caught: it uses five secrets whose names happen to match the
// six hand-written substrings, so it passed while every OpenVPN private key
// printed in full.
//
// The controller publishes sensitive_metadata.json. Of the 64 field names it
// marks sensitive, the substring list matched 13. x_ca_key, x_server_key,
// x_shared_client_key and x_dh_key -- the whole OpenVPN key set -- were
// among the 51 that did not, as were the SSH password hashes, the SSO and API
// tokens and the SIM PIN. wireguard_client_preshared_key missed by a single
// underscore against "pre_shared_key".
//
// The set is derived from that metadata now. This pins both directions,
// because a redactor that hides the object's name makes the error
// undiagnosable and gains nothing.
func TestRedactCoversTheControllersOwnSensitiveFields(t *testing.T) {
	secrets := []string{
		"x_ca_key", "x_server_key", "x_shared_client_key", "x_dh_key",
		"x_ssh_md5passwd", "x_ssh_sha512passwd", "x_sso_token", "x_api_token",
		"lte_sim_pin", "x_stripe_api_key", "google_maps_api_key", "auth_token",
		"wireguard_client_preshared_key", "wireguard_client_private_key",
		"x_iapp_key", "zone_key", "x_mgmt_key", "x_ble_adopt_key",
	}
	keep := []string{"name", "desc", "hostname", "purpose", "ip_subnet"}

	payload := map[string]any{
		"nested": map[string]any{"x_server_key": "SUPERSECRET"},
		"list":   []any{map[string]any{"x_ca_key": "SUPERSECRET"}},
	}
	for _, k := range secrets {
		payload[k] = "SUPERSECRET"
	}
	for _, k := range keep {
		payload[k] = "diagnostic-value"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	out := redactSensitivePayload(body)
	if strings.Contains(out, "SUPERSECRET") {
		for _, k := range secrets {
			if !isSensitiveKey(k) {
				t.Errorf("%s is not redacted; it prints in full in every non-2xx error", k)
			}
		}
		t.Fatalf("secret material survived redaction:\n%s", out)
	}
	for _, k := range keep {
		if !strings.Contains(out, k) {
			t.Errorf("%s vanished from the payload; an error that hides it is not diagnosable", k)
		}
	}
	if !strings.Contains(out, "diagnostic-value") {
		t.Error("every non-secret value was redacted; the error carries no diagnostic content")
	}
}

// An empty generated set silently reduces redaction to the substring fallback,
// which is where the defect was, and every other test here would still pass.
func TestSensitiveWireFieldsReachTheClient(t *testing.T) {
	if len(sensitiveWireFields) < 20 {
		t.Fatalf("sensitiveWireFields has %d entries; the generated set is not reaching the client",
			len(sensitiveWireFields))
	}
	for _, want := range []string{"x_ca_key", "x_server_key", "x_dh_key", "x_shared_client_key"} {
		if !sensitiveWireFields[want] {
			t.Errorf("%s missing from the generated set", want)
		}
	}
}

// TestRequestBodyIsNotInErrorsByDefault pins the default. Redaction is a
// second line, not the first: it cannot cover a secret the controller never
// declared sensitive, and when it misses, the value lands in an error string
// that callers put into logs, Terraform diagnostics and issue reports. Nothing
// about that failure is visible.
//
// So the body is out unless a caller asks for it in code. Opting in is a
// deliberate act at the call site rather than an ambient default or an
// environment variable someone sets in CI and forgets.
func TestRequestBodyIsNotInErrorsByDefault(t *testing.T) {
	var cfg Config
	if cfg.IncludeRequestBodyInErrors {
		t.Fatal("the zero-value Config includes request bodies in errors; the safe default is off")
	}

	client := &ApiClient{}
	if client.includeRequestBodyInErrors {
		t.Error("a zero-value client includes request bodies in errors")
	}

	client.includeRequestBodyInErrors = true
	if !client.includeRequestBodyInErrors {
		t.Error("the opt-in does not take effect; the flag is inert and the payload can never be shown")
	}
}

// TestSensitiveFormsAgree ties the exported per-collection declaration to the
// flat set the redactor consults. Both are emitted from one pass today, but
// this is what keeps an emitter edit from quietly splitting them into two
// definitions of "secret" -- one consumers derive from, one the redactor uses.
func TestSensitiveFormsAgree(t *testing.T) {
	// 8 of the metadata's 17 collections declare actual secrets; the other 9
	// carry only anonymization entries and are filtered out. Pin presence by
	// known members rather than a guessed count.
	if len(SensitiveFieldsByCollection) == 0 {
		t.Fatal("SensitiveFieldsByCollection is empty; the export is not populated")
	}
	for collection, field := range map[string]string{
		"device":     "x_authkey",
		"dynamicdns": "x_password", //nolint:misspell // the controller collection name
		"setting":    "x_ssh_password",
	} {
		found := false
		for _, f := range SensitiveFieldsByCollection[collection] {
			if f == field {
				found = true
			}
		}
		if !found {
			t.Errorf("%s.%s missing from the export", collection, field)
		}
	}
	union := map[string]bool{}
	for collection, fields := range SensitiveFieldsByCollection {
		if len(fields) == 0 {
			t.Errorf("collection %q declares no fields; an empty entry is noise", collection)
		}
		for _, f := range fields {
			union[f] = true
			if !sensitiveWireFields[f] {
				t.Errorf("%s.%s is exported as sensitive but the redactor does not know it", collection, f)
			}
		}
	}
	for f := range sensitiveWireFields {
		if !union[f] {
			t.Errorf("the redactor knows %q but no collection exports it", f)
		}
	}
}
