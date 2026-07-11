package unifi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// natTestServer stands up a new-style (UniFi OS) controller that serves the
// NAT rule collection at v2/.../nat as a bare JSON array. A per-id GET
// answers 405 + HTML like other v2 collection endpoints, so these tests lock
// GetNat to the list-and-filter read the generated client implements.
func natTestServer(t *testing.T, site, listJSON string) *httptest.Server {
	t.Helper()
	listPath := "/proxy/network/v2/api/site/" + site + "/nat"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == loginPathNew {
			w.Header().Set("X-Csrf-Token", "tok")
			w.WriteHeader(http.StatusOK)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == listPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(listJSON))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, listPath+"/"):
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte("<html><body>405 Not Allowed</body></html>"))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestGetNat_ReadsViaList pins the exported NAT read surface the Terraform
// provider consumes (ListNat/GetNat wrappers in nat.go) and proves GetNat
// resolves a rule by listing the bare-array collection and filtering by ID.
// It also exercises the tolerant numeric decode of rule_index.
func TestGetNat_ReadsViaList(t *testing.T) {
	const site = "default"
	srv := natTestServer(t, site, `[
		{"_id":"aaaa000000000000000000a1","type":"MASQUERADE","enabled":true,
		 "out_interface":"wan","description":"lan masquerade","rule_index":30001,
		 "setting_preference":"manual"},
		{"_id":"aaaa000000000000000000a2","type":"DNAT","enabled":false,
		 "in_interface":"wan","ip_address":"192.0.2.10"}
	]`)
	c := newAPGroupTestClient(t, srv)

	got, err := c.GetNat(context.Background(), site, "aaaa000000000000000000a1")
	if err != nil {
		t.Fatalf("GetNat errored (regressed to per-id GET?): %v", err)
	}
	if got.Type != "MASQUERADE" || !got.Enabled || got.OutInterface != "wan" {
		t.Errorf("rule = %+v", got)
	}
	if got.RuleIndex == nil || *got.RuleIndex != 30001 {
		t.Errorf("rule_index = %v, want 30001", got.RuleIndex)
	}
	if got.Description != "lan masquerade" || got.SettingPreference != "manual" {
		t.Errorf("rule = %+v", got)
	}
}

// TestGetNat_NotFound verifies a missing ID yields a typed NotFoundError so
// the Terraform resource's Read can drop the rule from state instead of
// erroring.
func TestGetNat_NotFound(t *testing.T) {
	const site = "default"
	srv := natTestServer(t, site, `[
		{"_id":"aaaa000000000000000000a1","type":"MASQUERADE","enabled":true}
	]`)
	c := newAPGroupTestClient(t, srv)

	_, err := c.GetNat(context.Background(), site, "does-not-exist")
	if !errors.As(err, new(*NotFoundError)) {
		t.Errorf("expected *NotFoundError for missing id, got %v", err)
	}
}
