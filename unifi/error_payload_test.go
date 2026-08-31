package unifi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// badRequestServer answers every non-setup request with a v1 400.
func badRequestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"meta":{"rc":"error","msg":"api.err.InvalidObject"},"data":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestErrorOmitsRequestBodyByDefault drives a real 400 through the client and
// checks the error text, rather than checking the flag's zero value -- which
// would hold even if doRequest ignored the flag entirely.
//
// It asserts both directions on purpose. A test that only asserts the secret
// is absent passes just as well when the payload was never attached for some
// unrelated reason, and would then keep passing if the protection were
// removed. Proving the opt-in still produces a payload is what makes the
// default's silence meaningful.
func TestErrorOmitsRequestBodyByDefault(t *testing.T) {
	const secret = "SUPERSECRET-KEY-MATERIAL"
	body := map[string]any{"name": "probe", "x_passphrase": secret, "x_ca_key": secret}

	t.Run("default omits the body entirely", func(t *testing.T) {
		srv := badRequestServer(t)
		c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "k"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		err = c.do(context.Background(), http.MethodPost, "api/s/default/rest/networkconf", body, nil)
		if err == nil {
			t.Fatal("expected an error from the 400")
		}
		if strings.Contains(err.Error(), "payload:") {
			t.Errorf("the request body is attached by default:\n%v", err)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("secret material reached the error text:\n%v", err)
		}
		if !strings.Contains(err.Error(), "api.err.InvalidObject") {
			t.Errorf("the controller's own message was lost; the error is now less useful "+
				"without being safer:\n%v", err)
		}
	})

	t.Run("opting in still attaches it, redacted", func(t *testing.T) {
		srv := badRequestServer(t)
		c, err := New(context.Background(), &Config{
			BaseURL: srv.URL, APIKey: "k", IncludeRequestBodyInErrors: true,
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		err = c.do(context.Background(), http.MethodPost, "api/s/default/rest/networkconf", body, nil)
		if err == nil {
			t.Fatal("expected an error from the 400")
		}
		if !strings.Contains(err.Error(), "payload:") {
			t.Fatalf("IncludeRequestBodyInErrors had no effect; the flag is inert, so the "+
				"default above proves nothing:\n%v", err)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("the opted-in payload is not redacted:\n%v", err)
		}
		if !strings.Contains(err.Error(), "probe") {
			t.Errorf("the opted-in payload carries no diagnostic content:\n%v", err)
		}
	})
}
