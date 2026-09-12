package unifi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// newRetryTestClient builds an API-key client against srv with a small retry
// budget and near-zero backoff, so retry-count tests measure attempts rather
// than wall-clock time. API-key auth skips the login dance entirely.
func newRetryTestClient(t *testing.T, srvURL string, retryMax int) *ApiClient {
	t.Helper()
	c, err := New(context.Background(), &Config{
		BaseURL:  srvURL,
		APIKey:   "test-key",
		RetryMax: &retryMax,
	})
	if err != nil {
		t.Fatalf("unable to build client: %v", err)
	}
	c.c.RetryWaitMin = time.Millisecond
	c.c.RetryWaitMax = 5 * time.Millisecond
	return c
}

// TestTransportDoesNotReplayPostOn500 pins the create path to a single
// attempt. Measured on 10.6.101: a create the controller rejects with a 500
// can still persist the document, so a transport that retries the POST seeds
// one duplicate per replay. The controller's own error must still surface.
func TestTransportDoesNotReplayPostOn500(t *testing.T) {
	var postHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		if r.Method == http.MethodPost {
			atomic.AddInt32(&postHits, 1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"meta":{"rc":"error","msg":"api.err.Boom"},"data":[]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newRetryTestClient(t, srv.URL, 3)
	err := c.do(context.Background(), http.MethodPost, "api/s/default/rest/networkconf", &struct {
		Name string `json:"name"`
	}{Name: "x"}, nil)
	if err == nil {
		t.Fatal("expected the 500 to surface as an error")
	}
	if !strings.Contains(err.Error(), "api.err.Boom") {
		t.Errorf("expected the controller's own error, got: %v", err)
	}
	if got := atomic.LoadInt32(&postHits); got != 1 {
		t.Errorf("POST reached the controller %d times; a failed create must not be replayed", got)
	}
}

// TestTransportRetriesIdempotentMethodsOn500 pins that the POST exemption did
// not leak: GET, PUT and DELETE keep the default 5xx retry behaviour. PUT is
// a full-document replace on this API, so replaying it rewrites the same
// document.
func TestTransportRetriesIdempotentMethodsOn500(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			var hits int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if handleNewStyleSetup(w, r) {
					return
				}
				if r.Method == method && strings.HasSuffix(r.URL.Path, "/rest/networkconf/abc") {
					atomic.AddInt32(&hits, 1)
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"meta":{"rc":"error","msg":"api.err.Boom"},"data":[]}`))
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			const retryMax = 3
			c := newRetryTestClient(t, srv.URL, retryMax)
			err := c.do(context.Background(), method, "api/s/default/rest/networkconf/abc", nil, nil)
			if err == nil {
				t.Fatal("expected the 500 to surface as an error")
			}
			if got := atomic.LoadInt32(&hits); got != retryMax+1 {
				t.Errorf("expected %d attempts for %s on 500, got %d", retryMax+1, method, got)
			}
		})
	}
}

// TestTransportDoesNotReplayPostOnBrokenConnection covers the resp == nil
// arm: the connection drops after the request was written, so the controller
// may well have processed the create. Replaying is then exactly as unsafe as
// replaying a 500.
func TestTransportDoesNotReplayPostOnBrokenConnection(t *testing.T) {
	var postHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		if r.Method == http.MethodPost {
			atomic.AddInt32(&postHits, 1)
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("test server does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newRetryTestClient(t, srv.URL, 3)
	// Drop the connections left over from client setup so the POST rides a
	// fresh one; net/http would otherwise consider an internal replay of its
	// own on a reused connection, which is not the layer under test.
	c.c.HTTPClient.CloseIdleConnections()
	err := c.do(context.Background(), http.MethodPost, "api/s/default/rest/networkconf", &struct {
		Name string `json:"name"`
	}{Name: "x"}, nil)
	if err == nil {
		t.Fatal("expected the dropped connection to surface as an error")
	}
	if got := atomic.LoadInt32(&postHits); got != 1 {
		t.Errorf("POST reached the controller %d times; a request that may have been processed must not be replayed", got)
	}
}

// TestTransportRetriesPostOnDialFailure covers the one transport error a POST
// may still retry on: a dial failure proves the request never left, so
// replaying cannot duplicate anything.
func TestTransportRetriesPostOnDialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	const retryMax = 2
	c := newRetryTestClient(t, srv.URL, retryMax)

	// Nothing listens on the port any more, so every attempt fails to dial.
	// The idle connection left over from client setup has to go too, or the
	// first attempt would ride it and fail with a read error instead of a
	// dial error. The server never sees a request; count attempts at the
	// client instead.
	srv.Close()
	c.c.HTTPClient.CloseIdleConnections()
	var attempts int32
	c.c.RequestLogHook = func(_ retryablehttp.Logger, _ *http.Request, _ int) {
		atomic.AddInt32(&attempts, 1)
	}

	err := c.do(context.Background(), http.MethodPost, "api/s/default/rest/networkconf", &struct {
		Name string `json:"name"`
	}{Name: "x"}, nil)
	if err == nil {
		t.Fatal("expected the dial failure to surface as an error")
	}
	if got := atomic.LoadInt32(&attempts); got != retryMax+1 {
		t.Errorf("expected %d attempts on a dial failure, got %d", retryMax+1, got)
	}
}
