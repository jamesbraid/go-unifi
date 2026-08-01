package controllertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestInformURLForAcceptsANetworkAddress pins the shape device containers
// need: an http URL whose host is a canonical IPv4 literal and whose path is
// exactly /inform.
func TestInformURLForAcceptsANetworkAddress(t *testing.T) {
	got, err := informURLFor("172.28.0.2")
	if err != nil {
		t.Fatalf("informURLFor: %v", err)
	}
	if want := "http://172.28.0.2:8080/inform"; got != want {
		t.Errorf("informURLFor = %q, want %q", got, want)
	}
}

// TestInformURLForRejectsUnreachableHosts pins the rejections that would
// otherwise produce a fleet that starts cleanly and never adopts. A hostname
// is rejected by the controller itself post-adopt ("invalid inform_ip
// localhost", HTTP 400); a loopback or unspecified address means nothing
// inside a device container; an IPv6 literal is outside the contract.
func TestInformURLForRejectsUnreachableHosts(t *testing.T) {
	for _, host := range []string{"", "localhost", "127.0.0.1", "0.0.0.0", "::1", "fd00::2", "172.028.0.2"} {
		if got, err := informURLFor(host); err == nil {
			t.Errorf("informURLFor(%q) = %q, want an error", host, got)
		}
	}
}

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
