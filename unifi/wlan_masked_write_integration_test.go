//go:build integration

// unifi/wlan_masked_write_integration_test.go
package unifi

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationMaskedWritePreservesUnnamedFields shows what the masked
// write buys, against the field that made it necessary.
//
// Both arms read a stored WLAN, rename it, and leave the struct holding a
// wrong value for roaming_assistant_na_enabled -- which is what happens to a
// caller that does not model a field. A Terraform provider converting through
// a resource model drops what its schema does not name, so the field reaches
// the client false. Writing that false is the 10.4.57 regression, where
// roaming assistant switched off on WLANs nobody had touched.
//
// The masked write names only the field it means, so the controller keeps its
// own value for the rest. The other arm records something found on the way:
// the unmasked round trip does not reach the controller's storage at all,
// because the payload is rejected.
func TestIntegrationMaskedWritePreservesUnnamedFields(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	deps := probeDeps{apGroupID: firstAPGroupID(ctx, t, s, c.Site)}

	// The harness exposes a raw session; this test needs the real client,
	// because what is under test is how the client encodes a write.
	api, err := New(ctx, &Config{
		BaseURL:       c.BaseURL,
		Username:      c.Username,
		Password:      c.Password,
		AllowInsecure: true,
	})
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	// prepare seeds a WLAN with roaming assistant on, reads it back through
	// the client, and drops the field the caller does not model.
	prepare := func(name string) (*WLAN, string) {
		payload := wlanBase(name, deps)
		payload["roaming_assistant_na_enabled"] = true
		body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/wlanconf", payload)
		if err != nil || status != 200 {
			t.Fatalf("seed rejected (HTTP %d): %v %v", status, body, err)
		}
		id, _ := firstData(t, body)["_id"].(string)
		if id == "" {
			t.Fatal("no _id on the created WLAN")
		}

		w, err := api.GetWLAN(ctx, c.Site, id)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		if !w.RoamingAssistantNaEnabled {
			t.Fatal("seed did not store roaming_assistant_na_enabled")
		}
		w.RoamingAssistantNaEnabled = false
		return w, id
	}

	stored := func(id string) map[string]any {
		fresh, status, err := s.GetJSON(ctx, "/api/s/"+c.Site+"/rest/wlanconf/"+id)
		if err != nil || status != 200 {
			t.Fatalf("re-read failed (HTTP %d): %v", status, err)
		}
		return firstData(t, fresh)
	}

	// A separate defect, found while building the arm above and worth
	// recording where someone will meet it: a WLAN read straight back
	// through the client cannot be written back. Nothing is mutated here
	// beyond a rename, and the controller rejects the payload outright.
	//
	// That matters for more than WLAN. Read-modify-write is the usual answer
	// to the write-shape problem -- read the object, change what you mean,
	// send it back, and fields you do not model round-trip untouched -- and
	// on this resource that answer is unavailable. Which fields of the
	// round trip the controller objects to is not established here.
	t.Run("a full round-trip write is rejected", func(t *testing.T) {
		w, id := prepare("mask-full")
		defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/wlanconf/"+id) //nolint:errcheck

		w.Name = "mask-full-renamed"
		_, err := api.UpdateWLAN(ctx, c.Site, w)
		if err == nil {
			t.Fatalf("a full round-trip write now succeeds. That is an improvement, but it " +
				"changes what the masked write is working around -- re-check this file and " +
				"TestGeneratedWriteShape.")
		}
		if !strings.Contains(err.Error(), "api.err.InvalidPayload") {
			t.Errorf("full round-trip write failed with %v, want api.err.InvalidPayload", err)
		}
	})

	t.Run("masked write leaves an unnamed field alone", func(t *testing.T) {
		w, id := prepare("mask-named")
		defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/wlanconf/"+id) //nolint:errcheck

		w.Name = "mask-named-renamed"
		if _, err := api.UpdateWLANFields(ctx, c.Site, w, "name"); err != nil {
			t.Fatalf("masked update: %v", err)
		}

		after := stored(id)
		if got := after["roaming_assistant_na_enabled"]; got != true {
			t.Errorf("roaming_assistant_na_enabled = %v, want true: the mask did not name this "+
				"field, so the controller should have kept its stored value", got)
		}
		// The rename still has to land, or preservation was achieved by
		// writing nothing at all.
		if got := after["name"]; got != "mask-named-renamed" {
			t.Errorf("name = %v, want mask-named-renamed: the masked write kept the unnamed "+
				"field but did not apply the named one", got)
		}
	})
}
