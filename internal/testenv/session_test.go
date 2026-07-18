// internal/testenv/session_test.go
package testenv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeController mimics the classic controller's /api/login cookie flow.
func fakeController(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		var creds struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil || creds.Username != "admin" || creds.Password != "admin" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "unifises", Value: "fake-session", Path: "/"})
		w.Write([]byte(`{"meta":{"rc":"ok"}}`))
	})
	mux.HandleFunc("/v2/api/site/default/trafficroutes", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("unifises"); err != nil || c.Value != "fake-session" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`[{"_id":"x","enabled":true}]`))
	})
	return httptest.NewTLSServer(mux)
}

func TestSessionLoginAndGetJSON(t *testing.T) {
	srv := fakeController(t)
	defer srv.Close()

	s := NewSession(srv.URL)
	ctx := context.Background()

	if err := s.Login(ctx, "admin", "admin"); err != nil {
		t.Fatalf("login: %v", err)
	}

	body, status, err := s.GetJSON(ctx, "/v2/api/site/default/trafficroutes")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	items, ok := body.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("body = %#v, want 1-item array", body)
	}
}

func TestSessionLoginRejected(t *testing.T) {
	srv := fakeController(t)
	defer srv.Close()

	s := NewSession(srv.URL)
	if err := s.Login(context.Background(), "admin", "wrong"); err == nil {
		t.Fatal("expected login error")
	}
}

// TestSessionLoginBootPlaceholder mimics the controller's boot window: for
// the first ~25s it answers every path — including /api/login — with HTTP
// 200 and an HTML "Server status" placeholder page. Login must not mistake
// that for success.
func TestSessionLoginBootPlaceholder(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html lang="en"><head><title>UniFi Network - Server status</title></head><body>starting up</body></html>`))
	}))
	defer srv.Close()

	s := NewSession(srv.URL)
	if err := s.Login(context.Background(), "admin", "admin"); err == nil {
		t.Fatal("expected login error for HTML placeholder body")
	}
}

// TestSessionLoginMetaRCError covers controllers that report failure inside
// an HTTP 200 JSON envelope: meta.rc != "ok" must be a login error.
func TestSessionLoginMetaRCError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"meta":{"rc":"error"}}`))
	}))
	defer srv.Close()

	s := NewSession(srv.URL)
	err := s.Login(context.Background(), "admin", "admin")
	if err == nil {
		t.Fatal(`expected login error for meta.rc == "error"`)
	}
	if !strings.Contains(err.Error(), "error") {
		t.Fatalf("error %q should include the rc value", err)
	}
}

func TestSessionGetJSONNotFound(t *testing.T) {
	srv := fakeController(t)
	defer srv.Close()

	s := NewSession(srv.URL)
	_ = s.Login(context.Background(), "admin", "admin")
	_, status, err := s.GetJSON(context.Background(), "/v2/api/site/default/nope")
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
}
