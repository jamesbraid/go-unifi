package unifi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// deviceRereadTestServer stands up a controller carrying two devices and
// records which read path a re-read took.
//
// The distinction that matters: stat/device/{mac} answers one device, while
// stat/device -- which is also what stat/device/ with an empty identifier
// resolves to -- answers both. A re-read that loses the identifier therefore
// does not fail loudly; it lands on the list and comes back with the wrong
// number of rows.
func deviceRereadTestServer(t *testing.T, site string, gotPath *string) *httptest.Server {
	t.Helper()

	const (
		wantedID   = "dev1"
		wantedMAC  = "00:11:22:33:44:55"
		otherID    = "dev2"
		otherMAC   = "66:77:88:99:aa:bb"
		metaOK     = `"meta":{"rc":"ok"}`
		wantedJSON = `{"_id":"` + wantedID + `","mac":"` + wantedMAC + `","name":"wanted"}`
		otherJSON  = `{"_id":"` + otherID + `","mac":"` + otherMAC + `","name":"other"}`
	)

	listPath := "/proxy/network/api/s/" + site + "/stat/device"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == loginPathNew {
			w.Header().Set("X-Csrf-Token", "tok")
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, listPath) {
			w.WriteHeader(http.StatusOK)
			return
		}

		*gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		switch strings.TrimPrefix(r.URL.Path, listPath) {
		case "/" + wantedMAC:
			_, _ = w.Write([]byte(`{` + metaOK + `,"data":[` + wantedJSON + `]}`))
		case "/" + otherMAC:
			_, _ = w.Write([]byte(`{` + metaOK + `,"data":[` + otherJSON + `]}`))
		default:
			// Bare stat/device, and equally stat/device/ with nothing after
			// the slash: the whole site.
			_, _ = w.Write([]byte(`{` + metaOK + `,"data":[` + wantedJSON + `,` + otherJSON + `]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestRereadDeviceByMAC pins the normal path: a device carrying a MAC is
// re-read through the MAC-keyed endpoint.
func TestRereadDeviceByMAC(t *testing.T) {
	const site = "default"
	var gotPath string
	srv := deviceRereadTestServer(t, site, &gotPath)

	c, err := New(context.Background(), &Config{BaseURL: srv.URL, Username: "admin", Password: "admin"})
	if err != nil {
		t.Fatalf("client init: %v", err)
	}

	got, err := c.rereadDevice(context.Background(), site, &Device{ID: "dev1", MAC: "00:11:22:33:44:55"})
	if err != nil {
		t.Fatalf("re-read by MAC: %v", err)
	}
	if got.ID != "dev1" {
		t.Errorf("re-read the wrong device: got %q", got.ID)
	}
	if !strings.HasSuffix(gotPath, "/stat/device/00:11:22:33:44:55") {
		t.Errorf("a device with a MAC should be read through the MAC endpoint, not %q", gotPath)
	}
}

// TestRereadDeviceWithoutMAC is the regression this helper exists for.
//
// A masked write addresses the device through the URL, so the documented
// partial form &Device{ID: id, ...} carries no MAC. Re-reading that by MAC
// asks for stat/device/, which is the list: on this two-device site the old
// code got two rows back and reported NotFound for a write that had already
// succeeded.
func TestRereadDeviceWithoutMAC(t *testing.T) {
	const site = "default"
	var gotPath string
	srv := deviceRereadTestServer(t, site, &gotPath)

	c, err := New(context.Background(), &Config{BaseURL: srv.URL, Username: "admin", Password: "admin"})
	if err != nil {
		t.Fatalf("client init: %v", err)
	}

	got, err := c.rereadDevice(context.Background(), site, &Device{ID: "dev2"})
	if err != nil {
		t.Fatalf("re-read without a MAC failed, which is the bug: %v", err)
	}
	if got.ID != "dev2" {
		t.Errorf("filtered to the wrong device: got %q, want dev2", got.ID)
	}
	if strings.Contains(gotPath, "/stat/device/") {
		t.Errorf("with no MAC the re-read must list and filter on id, but it hit %q", gotPath)
	}
}
