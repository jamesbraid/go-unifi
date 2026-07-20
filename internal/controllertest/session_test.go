package controllertest

import (
	"context"
	"encoding/json"
	"errors"
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
	// POST endpoint: requires the session cookie like any other v2 API
	// call, decodes the request body, stamps a server-assigned _id onto
	// it, and echoes the result — mimicking a controller's create response.
	mux.HandleFunc("/v2/api/site/default/objects", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("unifises"); err != nil || c.Value != "fake-session" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"meta":{"rc":"error","msg":"api.err.LoginRequired"}}`))
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var created map[string]any
		if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		created["_id"] = "created-1"
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(created); err != nil {
			panic(err)
		}
	})
	// Unknown paths: a real controller answers with a JSON error envelope,
	// not the plain-text page Go's ServeMux defaults to — model that so
	// GetJSON callers see a genuine 404 rather than a spurious ErrNotJSON.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"meta":{"rc":"error","msg":"api.err.NotFound"}}`))
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
		w.Write([]byte(`{"meta":{"rc":"invalid_credentials"}}`))
	}))
	defer srv.Close()

	s := NewSession(srv.URL)
	err := s.Login(context.Background(), "admin", "admin")
	if err == nil {
		t.Fatal(`expected login error for meta.rc == "invalid_credentials"`)
	}
	if !strings.Contains(err.Error(), "invalid_credentials") {
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

// TestSessionGetJSONNull covers legal JSON null bodies (e.g. an empty v2
// endpoint): GetJSON must return a nil body with a nil error, not
// ErrNotJSON — null is valid JSON, distinct from a non-JSON body.
func TestSessionGetJSONNull(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`null`))
	}))
	defer srv.Close()

	s := NewSession(srv.URL)
	body, status, err := s.GetJSON(context.Background(), "/whatever")
	if err != nil {
		t.Fatalf("unexpected error for JSON null body: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if body != nil {
		t.Fatalf("body = %#v, want nil", body)
	}
}

// TestSessionPostJSON covers the authenticated create path: PostJSON must
// send the marshaled body with a JSON content type, and decode the
// controller's response the same way GetJSON does.
func TestSessionPostJSON(t *testing.T) {
	srv := fakeController(t)
	defer srv.Close()

	s := NewSession(srv.URL)
	ctx := context.Background()

	if err := s.Login(ctx, "admin", "admin"); err != nil {
		t.Fatalf("login: %v", err)
	}

	body, status, err := s.PostJSON(ctx, "/v2/api/site/default/objects", map[string]string{"name": "probe"})
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("body = %#v, want object", body)
	}
	if m["name"] != "probe" {
		t.Fatalf("body[name] = %v, want %q", m["name"], "probe")
	}
	if m["_id"] != "created-1" {
		t.Fatalf("body[_id] = %v, want the server-assigned id", m["_id"])
	}
}

// TestSessionPostJSONUnauthenticated covers the unauthenticated path: a
// 401 from the controller must pass through as a status, not an error —
// mirroring GetJSON's non-2xx-is-not-an-error contract.
func TestSessionPostJSONUnauthenticated(t *testing.T) {
	srv := fakeController(t)
	defer srv.Close()

	s := NewSession(srv.URL)
	body, status, err := s.PostJSON(context.Background(), "/v2/api/site/default/objects", map[string]string{"name": "probe"})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("body = %#v, want decoded error envelope", body)
	}
	if meta, _ := m["meta"].(map[string]any); meta == nil || meta["rc"] != "error" {
		t.Fatalf("body[meta] = %v, want the controller's error envelope", m["meta"])
	}
}

// TestSessionGetJSONNotJSON covers a non-JSON 200 body (e.g. the
// controller's boot-time HTML placeholder): GetJSON must return
// ErrNotJSON so callers can distinguish this from a legal JSON null.
func TestSessionGetJSONNotJSON(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><body>starting up</body></html>`))
	}))
	defer srv.Close()

	s := NewSession(srv.URL)
	body, status, err := s.GetJSON(context.Background(), "/whatever")
	if !errors.Is(err, ErrNotJSON) {
		t.Fatalf("err = %v, want ErrNotJSON", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if body != nil {
		t.Fatalf("body = %#v, want nil", body)
	}
}
