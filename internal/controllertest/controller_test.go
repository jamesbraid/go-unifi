package controllertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestControllerImageFromEnv(t *testing.T) {
	t.Setenv("UNIFI_TEST_IMAGE", "example.invalid/unifi:v99")

	if got := imageFromEnv(); got != "example.invalid/unifi:v99" {
		t.Errorf("image = %q", got)
	}
}

func TestControllerImageDefault(t *testing.T) {
	t.Setenv("UNIFI_TEST_IMAGE", "")

	if got := imageFromEnv(); got != defaultImage {
		t.Errorf("image = %q, want %q", got, defaultImage)
	}
}

// TestWaitReadyRetriesThroughBootPlaceholder pins the URL-mode readiness
// rule: an HTTP 200 with a non-JSON body (the controller's boot placeholder
// page) means "still booting", not "ready" — the poll must retry through it
// and succeed once real JSON appears.
func TestWaitReadyRetriesThroughBootPlaceholder(t *testing.T) {
	restore := readyPollInterval
	readyPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { readyPollInterval = restore })

	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.Write([]byte("<html>Server status</html>"))
			return
		}
		w.Write([]byte(`{"meta":{"rc":"ok"}}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	c := &Controller{BaseURL: srv.URL, Username: "admin", Password: "admin"}
	waitReady(context.Background(), t, c)

	if got := calls.Load(); got < 3 {
		t.Fatalf("login called %d times, want at least 3", got)
	}
}
