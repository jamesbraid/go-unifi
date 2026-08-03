//go:build integration

// unifi/validation_error_integration_test.go
package unifi

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationValidationErrorShape pins the controller's field-level
// rejection detail, which the SDK now surfaces on APIError.
//
// This is the measurement behind that decoding. A remote-user-vpn network
// whose openvpn_encryption_cipher is out of range comes back as HTTP 400,
// and the caller used to see only api.err.InvalidPayload -- no indication
// which of eighty fields was wrong. The controller had in fact named it, in
// a validationError object the decoder discarded. The bug that prompted this
// (a provider defaulting the cipher to AES_256_GCM, against a schema that
// has only ever said AES_256_CBC|BF_CBC) cost an afternoon for want of a
// field the response already contained.
//
// The test logs the whole body: if a controller ever moves validationError
// or renames its keys, the log says what the new shape is instead of leaving
// a bare assertion failure.
func TestIntegrationValidationErrorShape(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", map[string]any{
		"name":                      "validation-error-probe",
		"purpose":                   PurposeUserVPN,
		"enabled":                   true,
		"vpn_type":                  "openvpn-server",
		"openvpn_mode":              "server",
		"ip_subnet":                 "10.120.0.1/24",
		"openvpn_encryption_cipher": "AES_256_GCM", // not in AES_256_CBC|BF_CBC
	})
	if err != nil {
		t.Fatalf("transport: %v", err)
	}

	raw, _ := json.Marshal(body)
	t.Logf("HTTP %d, body: %s", status, raw)

	if status == 200 {
		if created := firstData(t, body); created != nil {
			if id, _ := created["_id"].(string); id != "" {
				defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck
			}
		}
		t.Fatalf("controller accepted an out-of-range openvpn_encryption_cipher; the enum in FieldValidationPatterns may be stale")
	}

	// Find validationError wherever it sits. Measured on 10.4.57 it rides on
	// the data[] element, alongside the specific api.err.InvalidValue, while
	// meta carries only the generic api.err.InvalidPayload. The client
	// decodes all three positions; this records which one is real, so a
	// controller that moves it fails here with the new shape in the log.
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("error body is not an object: %s", raw)
	}
	var validation any
	where := "nowhere"
	if data, ok := m["data"].([]any); ok {
		for _, item := range data {
			d, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if v := d["validationError"]; v != nil {
				validation, where = v, "on the data[] element"
				break
			}
			if nested, ok := d["meta"].(map[string]any); ok && nested["validationError"] != nil {
				validation, where = nested["validationError"], "inside data[].meta"
				break
			}
		}
	}
	if validation == nil {
		if v := m["validationError"]; v != nil {
			validation, where = v, "at the top level"
		} else if meta, ok := m["meta"].(map[string]any); ok && meta["validationError"] != nil {
			validation, where = meta["validationError"], "inside meta"
		}
	}
	if validation == nil {
		t.Fatalf("controller sent no validationError for an out-of-range enum; body: %s", raw)
	}
	t.Logf("validationError found %s: %v", where, validation)

	detail, ok := validation.(map[string]any)
	if !ok {
		t.Fatalf("validationError is not an object: %v", validation)
	}
	if got := detail["field"]; got != "openvpn_encryption_cipher" {
		t.Errorf("validationError.field = %v, want openvpn_encryption_cipher", got)
	}
	if got, _ := detail["pattern"].(string); got == "" {
		t.Errorf("validationError.pattern is empty; the pattern is the half that makes the error actionable")
	} else if got != FieldValidationPatterns["Network"]["openvpn_encryption_cipher"] {
		t.Errorf("controller pattern %q disagrees with the generated schema table %q",
			got, FieldValidationPatterns["Network"]["openvpn_encryption_cipher"])
	}
}
