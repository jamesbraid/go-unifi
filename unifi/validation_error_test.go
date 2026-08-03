package unifi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// errorBodyServer serves body with the given status for any non-setup request.
func errorBodyServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// invalidValueBody is a verbatim 400 from UniFi Network 10.4.57, captured by
// TestIntegrationValidationErrorShape by POSTing a remote-user-vpn network
// with an out-of-range openvpn_encryption_cipher.
//
// Note where the useful half lives. meta says only api.err.InvalidPayload;
// the data[] element carries api.err.InvalidValue plus the offending field
// and the pattern it had to match. The decoder used to read data[].meta -- a
// key this body does not have -- so it fell back to meta and the caller was
// told nothing but "invalid payload".
const invalidValueBody = `{
	"data": [
		{
			"msg": "api.err.InvalidValue",
			"rc": "error",
			"validationError": {
				"field": "openvpn_encryption_cipher",
				"pattern": "AES_256_CBC|BF_CBC"
			}
		}
	],
	"meta": {"msg": "api.err.InvalidPayload", "rc": "error"}
}`

// TestValidationErrorSurfaced checks that the controller's field-level detail
// reaches the caller, both in the message and as structured data.
func TestValidationErrorSurfaced(t *testing.T) {
	srv := errorBodyServer(t, http.StatusBadRequest, invalidValueBody)

	c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = c.do(context.Background(), http.MethodPost, "api/s/default/rest/networkconf", map[string]any{}, nil)
	if err == nil {
		t.Fatal("expected an error from the 400 response, got nil")
	}

	// The specific code, not the generic envelope one.
	if !strings.Contains(err.Error(), "api.err.InvalidValue") {
		t.Errorf("specific error code not surfaced; got: %v", err)
	}
	// The field and the pattern are what turn this from a guessing game
	// into a fix, so both have to be in the message.
	if !strings.Contains(err.Error(), "openvpn_encryption_cipher") {
		t.Errorf("rejected field not surfaced; got: %v", err)
	}
	if !strings.Contains(err.Error(), "AES_256_CBC|BF_CBC") {
		t.Errorf("required pattern not surfaced; got: %v", err)
	}

	// Structured, so a caller can map the failure back onto its own schema
	// instead of scraping the string.
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not an *APIError: %v", err)
	}
	if apiErr.Validation == nil {
		t.Fatal("APIError.Validation is nil; the controller sent one")
	}
	if apiErr.Validation.Field != "openvpn_encryption_cipher" {
		t.Errorf("Validation.Field = %q, want openvpn_encryption_cipher", apiErr.Validation.Field)
	}
	if apiErr.Validation.Pattern != "AES_256_CBC|BF_CBC" {
		t.Errorf("Validation.Pattern = %q, want AES_256_CBC|BF_CBC", apiErr.Validation.Pattern)
	}
}

// TestValidationErrorAbsentIsUnchanged guards the other direction: an error
// body without a validationError must read exactly as it did before this
// decoding existed, so nothing that parses these messages breaks.
func TestValidationErrorAbsentIsUnchanged(t *testing.T) {
	srv := errorBodyServer(t, http.StatusBadRequest,
		`{"meta": {"msg": "api.err.MissingIPAddress", "rc": "error"}, "data": []}`)

	c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = c.do(context.Background(), http.MethodPost, "api/s/default/rest/networkconf", map[string]any{}, nil)
	if err == nil {
		t.Fatal("expected an error from the 400 response, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is not an *APIError: %v", err)
	}
	if apiErr.Validation != nil {
		t.Errorf("Validation = %+v, want nil when the controller sent none", apiErr.Validation)
	}
	if apiErr.Error() != "api.err.MissingIPAddress" {
		t.Errorf("Error() = %q, want the bare controller message", apiErr.Error())
	}
}

// TestValidationErrorPositions covers the two shapes that were plausible
// before the capture settled it. Only the data[] element is confirmed on
// 10.4.57; the others are decoded so a controller that moves the object
// keeps working rather than silently reverting to an opaque error.
func TestValidationErrorPositions(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "on the data element (measured on 10.4.57)",
			body: invalidValueBody,
		},
		{
			name: "nested under data[].meta",
			body: `{"data":[{"meta":{"rc":"error","msg":"api.err.InvalidValue",
				"validationError":{"field":"openvpn_encryption_cipher","pattern":"AES_256_CBC|BF_CBC"}}}],
				"meta":{"rc":"error","msg":"api.err.InvalidPayload"}}`,
		},
		{
			name: "beside the envelope",
			body: `{"meta":{"rc":"error","msg":"api.err.InvalidValue"},
				"validationError":{"field":"openvpn_encryption_cipher","pattern":"AES_256_CBC|BF_CBC"}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := errorBodyServer(t, http.StatusBadRequest, tc.body)
			c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			err = c.do(context.Background(), http.MethodPost, "api/s/default/rest/networkconf", map[string]any{}, nil)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error is not an *APIError: %v", err)
			}
			if apiErr.Validation == nil || apiErr.Validation.Field != "openvpn_encryption_cipher" {
				t.Errorf("validation detail not decoded from this position; got %+v", apiErr.Validation)
			}
		})
	}
}
