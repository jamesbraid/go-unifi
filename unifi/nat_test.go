package unifi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// natTestServer stands up a new-style (UniFi OS) controller that serves the
// NAT rule collection at v2/.../nat as a bare JSON array, and is method-aware
// so it can also exercise create/update/delete against the same collection:
//   - GET  {listPath}     -> the fixture array (list/get-via-list)
//   - POST {listPath}     -> echoes the decoded body back as the created rule
//     (assigning an ID if the caller didn't supply one), the way createNat's
//     single-object response decode expects
//   - PUT  {listPath}/id  -> echoes the decoded body back as the updated rule,
//     matching updateNat's single-object response decode
//   - DELETE {listPath}/id -> 200 with no body, matching deleteNat's nil
//     respBody decode
//   - other GETs under {listPath}/ -> 405 + HTML like other v2 collection
//     endpoints, so GetNat is locked to the list-and-filter read.
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
		case r.Method == http.MethodPost && r.URL.Path == listPath:
			var rule Nat
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &rule); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if rule.ID == "" {
				rule.ID = "created-id-0001"
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(rule)
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, listPath+"/"):
			id := strings.TrimPrefix(r.URL.Path, listPath+"/")
			var rule Nat
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &rule); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if rule.ID != id {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(rule)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, listPath+"/"):
			w.WriteHeader(http.StatusOK)
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
//
// It requests the *second* fixture element deliberately: a regression that
// unconditionally returns respBody[0] (ignoring the requested id) would still
// pass a test that only ever asked for the first id.
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

	got, err := c.GetNat(context.Background(), site, "aaaa000000000000000000a2")
	if err != nil {
		t.Fatalf("GetNat errored (regressed to per-id GET?): %v", err)
	}
	if got.ID != "aaaa000000000000000000a2" {
		t.Fatalf("id = %q, want aaaa000000000000000000a2 (regressed to respBody[0]?)", got.ID)
	}
	if got.Type != "DNAT" || got.Enabled || got.InInterface != "wan" || got.IPAddress != "192.0.2.10" {
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

// TestListNat_ReturnsAll pins ListNat: it must decode the full bare-array
// collection response, not just a single element or a filtered subset.
func TestListNat_ReturnsAll(t *testing.T) {
	const site = "default"
	srv := natTestServer(t, site, `[
		{"_id":"aaaa000000000000000000a1","type":"MASQUERADE","enabled":true,
		 "out_interface":"wan","rule_index":30001},
		{"_id":"aaaa000000000000000000a2","type":"DNAT","enabled":false,
		 "in_interface":"wan","ip_address":"192.0.2.10"}
	]`)
	c := newAPGroupTestClient(t, srv)

	got, err := c.ListNat(context.Background(), site)
	if err != nil {
		t.Fatalf("ListNat errored: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(ListNat) = %d, want 2: %+v", len(got), got)
	}
	if got[0].ID != "aaaa000000000000000000a1" || got[1].ID != "aaaa000000000000000000a2" {
		t.Errorf("ids = [%q, %q], want [aaaa...a1, aaaa...a2]", got[0].ID, got[1].ID)
	}
	if got[1].Type != "DNAT" || got[1].IPAddress != "192.0.2.10" {
		t.Errorf("rule[1] = %+v", got[1])
	}
}

// TestCreateNat_RoundTrip pins CreateNat: it must POST the rule to the bare
// collection path (no id suffix, matching createNat's fmt.Sprintf) and decode
// the single-object response body, not a wrapped/array shape.
func TestCreateNat_RoundTrip(t *testing.T) {
	const site = "default"
	srv := natTestServer(t, site, `[]`)
	c := newAPGroupTestClient(t, srv)

	in := &Nat{
		Type:         "SNAT",
		Enabled:      true,
		OutInterface: "wan",
		Description:  "outbound snat",
	}
	got, err := c.CreateNat(context.Background(), site, in)
	if err != nil {
		t.Fatalf("CreateNat errored: %v", err)
	}
	if got.ID == "" {
		t.Errorf("CreateNat did not return an assigned id: %+v", got)
	}
	if got.Type != "SNAT" || !got.Enabled || got.OutInterface != "wan" || got.Description != "outbound snat" {
		t.Errorf("created rule = %+v, want echo of input", got)
	}
}

// TestUpdateNat_RoundTrip pins UpdateNat: it must PUT to the per-id path
// (collection path + "/" + d.ID, matching updateNat's fmt.Sprintf) and decode
// the single-object response body.
func TestUpdateNat_RoundTrip(t *testing.T) {
	const site = "default"
	srv := natTestServer(t, site, `[]`)
	c := newAPGroupTestClient(t, srv)

	in := &Nat{
		ID:           "aaaa000000000000000000a1",
		Type:         "MASQUERADE",
		Enabled:      false,
		OutInterface: "wan2",
		Description:  "updated",
	}
	got, err := c.UpdateNat(context.Background(), site, in)
	if err != nil {
		t.Fatalf("UpdateNat errored: %v", err)
	}
	if got.ID != "aaaa000000000000000000a1" {
		t.Errorf("id = %q, want aaaa000000000000000000a1 (wrong path hit?)", got.ID)
	}
	if got.Enabled || got.OutInterface != "wan2" || got.Description != "updated" {
		t.Errorf("updated rule = %+v, want echo of input", got)
	}
}

// TestDeleteNat_Success pins DeleteNat: it must DELETE the per-id path and
// treat a no-body 200 response as success (matching deleteNat's nil
// respBody), rather than erroring on empty/undecodable body.
func TestDeleteNat_Success(t *testing.T) {
	const site = "default"
	srv := natTestServer(t, site, `[]`)
	c := newAPGroupTestClient(t, srv)

	if err := c.DeleteNat(context.Background(), site, "aaaa000000000000000000a1"); err != nil {
		t.Fatalf("DeleteNat errored: %v", err)
	}
}
