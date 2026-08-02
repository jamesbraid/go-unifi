package unifi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGetClientByMACIgnoresFormatting proves the lookup matches on the
// address rather than on how it was written.
//
// The comparison used to be a plain d.MAC == mac against the caller's raw
// argument, so a caller holding AA:BB:CC:DD:EE:FF or aa-bb-cc-dd-ee-ff got
// NotFoundError for a client that was sitting right there. Nothing in the
// signature says the argument has to be lowercase and colon-separated, and
// the controller's own validator accepts either case.
func TestGetClientByMACIgnoresFormatting(t *testing.T) {
	const stored = "aa:bb:cc:dd:ee:ff"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"_id":"c1","mac":"` + stored + `"}]}`))
	}))
	defer srv.Close()

	c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, arg := range []string{
		"aa:bb:cc:dd:ee:ff",
		"AA:BB:CC:DD:EE:FF",
		"aa-bb-cc-dd-ee-ff",
		"AA-BB-CC-DD-EE-FF",
		"aabb.ccdd.eeff",
		"AABBCCDDEEFF",
	} {
		t.Run(arg, func(t *testing.T) {
			got, err := c.GetClientByMAC(context.Background(), "default", arg)
			if err != nil {
				t.Fatalf("GetClientByMAC(%q): %v", arg, err)
			}
			if got.MAC != stored {
				t.Errorf("MAC = %q, want %q", got.MAC, stored)
			}
		})
	}
}

// TestGetClientByMACStillMisses checks the normalisation did not turn the
// lookup into something that matches too much.
func TestGetClientByMACStillMisses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"_id":"c1","mac":"aa:bb:cc:dd:ee:ff"}]}`))
	}))
	defer srv.Close()

	c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := c.GetClientByMAC(context.Background(), "default", "11:22:33:44:55:66"); err == nil {
		t.Error("expected NotFoundError for a MAC that is not present")
	}
}

// TestDeviceCommandsNormalizeMAC checks the MAC a command carries is
// normalised, so the controller is asked about the device the caller meant.
func TestDeviceCommandsNormalizeMAC(t *testing.T) {
	tests := []struct {
		name string
		call func(c *ApiClient) error
		key  string
	}{
		{
			name: "AdoptDevice",
			call: func(c *ApiClient) error {
				return c.AdoptDevice(context.Background(), "default", "AA-BB-CC-DD-EE-FF")
			},
			key: "mac",
		},
		{
			name: "ForgetDevice",
			call: func(c *ApiClient) error {
				return c.ForgetDevice(context.Background(), "default", "AA-BB-CC-DD-EE-FF")
			},
			key: "macs",
		},
		{
			name: "BlockClientByMAC",
			call: func(c *ApiClient) error {
				return c.BlockClientByMAC(context.Background(), "default", "AA-BB-CC-DD-EE-FF")
			},
			key: "mac",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var sent map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if handleNewStyleSetup(w, r) {
					return
				}
				if body, err := io.ReadAll(r.Body); err == nil && len(body) > 0 {
					_ = json.Unmarshal(body, &sent)
				}
				w.Header().Set("Content-Type", "application/json")
				// One element so the *ByMAC helpers see their expected count.
				_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"_id":"x"}]}`))
			}))
			defer srv.Close()

			c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if err := tc.call(c); err != nil {
				t.Fatalf("call: %v", err)
			}

			raw, _ := json.Marshal(sent[tc.key])
			if got := string(raw); !strings.Contains(got, "aa:bb:cc:dd:ee:ff") {
				t.Errorf("%s sent %s = %s, want the normalised aa:bb:cc:dd:ee:ff", tc.name, tc.key, got)
			}
		})
	}
}

// TestAssignDeviceTagNormalizesPathMAC covers the one that interpolates the
// MAC into the URL rather than a body.
func TestAssignDeviceTagNormalizesPathMAC(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		if strings.Contains(r.URL.Path, "device-tag-assignment") {
			path = r.URL.Path
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.AssignDeviceTag(context.Background(), "default", "AA-BB-CC-DD-EE-FF", nil, nil); err != nil {
		t.Fatalf("AssignDeviceTag: %v", err)
	}
	if !strings.HasSuffix(path, "/aa:bb:cc:dd:ee:ff") {
		t.Errorf("request path = %q, want it to end with the normalised MAC", path)
	}
}
