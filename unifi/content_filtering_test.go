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

// contentFilteringTestServer stands up a new-style (UniFi OS) controller
// serving the content-filtering collection at v2/.../content-filtering as a
// bare JSON array. A per-id GET answers 405 + HTML like other v2 collection
// endpoints, locking GetContentFiltering to the list-and-filter read. POST to
// the collection echoes the body back with an _id assigned, the way the
// controller answers a create.
func contentFilteringTestServer(t *testing.T, site, listJSON string, gotCreate *ContentFiltering) *httptest.Server {
	t.Helper()
	listPath := "/proxy/network/v2/api/site/" + site + "/content-filtering"
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
		case r.Method == http.MethodPost && r.URL.Path == listPath:
			body, _ := io.ReadAll(r.Body)
			if gotCreate != nil {
				if err := json.Unmarshal(body, gotCreate); err != nil {
					t.Errorf("create body did not decode: %v", err)
				}
			}
			var echo map[string]any
			_ = json.Unmarshal(body, &echo)
			echo["_id"] = "cccc0000000000000000c001"
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(echo)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestGetContentFiltering_ReadsViaList proves GetContentFiltering resolves a
// policy by listing the bare-array collection and filtering by ID, and that
// the wire fields observed on a live controller decode.
func TestGetContentFiltering_ReadsViaList(t *testing.T) {
	const site = "default"
	srv := contentFilteringTestServer(t, site, `[
		{"_id":"cccc0000000000000000c001","name":"kids devices","enabled":true,
		 "categories":["FAMILY","ADVERTISEMENT"],
		 "client_macs":["aa:bb:cc:00:00:01"],"network_ids":[],
		 "allow_list":[],"block_list":["example.test"],
		 "safe_search":["GOOGLE","YOUTUBE","BING"],
		 "schedule":{"mode":"ALWAYS"}},
		{"_id":"cccc0000000000000000c002","name":"guest","enabled":false,
		 "categories":[],"client_macs":[],"network_ids":[],
		 "allow_list":[],"block_list":[],"safe_search":[],
		 "schedule":{"mode":"ALWAYS"}}
	]`, nil)
	c := newAPGroupTestClient(t, srv)

	got, err := c.GetContentFiltering(context.Background(), site, "cccc0000000000000000c001")
	if err != nil {
		t.Fatalf("GetContentFiltering errored (regressed to per-id GET?): %v", err)
	}
	if got.Name != "kids devices" || !got.Enabled {
		t.Errorf("policy = %+v", got)
	}
	if len(got.Categories) != 2 || got.Categories[0] != "FAMILY" {
		t.Errorf("categories = %v", got.Categories)
	}
	if len(got.ClientMACs) != 1 || got.ClientMACs[0] != "aa:bb:cc:00:00:01" {
		t.Errorf("client_macs = %v", got.ClientMACs)
	}
	if len(got.BlockList) != 1 || got.BlockList[0] != "example.test" {
		t.Errorf("block_list = %v", got.BlockList)
	}
	if len(got.SafeSearch) != 3 || got.SafeSearch[2] != "BING" {
		t.Errorf("safe_search = %v", got.SafeSearch)
	}
	if got.Schedule == nil || got.Schedule.Mode != "ALWAYS" {
		t.Errorf("schedule = %+v", got.Schedule)
	}
}

// TestGetContentFiltering_NotFound verifies a missing ID yields a typed
// NotFoundError so the Terraform resource's Read can drop the policy from
// state instead of erroring.
func TestGetContentFiltering_NotFound(t *testing.T) {
	const site = "default"
	srv := contentFilteringTestServer(t, site,
		`[{"_id":"cccc0000000000000000c001","name":"kids devices","enabled":true,
		   "categories":[],"client_macs":[],"network_ids":[],
		   "allow_list":[],"block_list":[],"safe_search":[],
		   "schedule":{"mode":"ALWAYS"}}]`, nil)
	c := newAPGroupTestClient(t, srv)

	_, err := c.GetContentFiltering(context.Background(), site, "does-not-exist")
	if !errors.As(err, new(*NotFoundError)) {
		t.Errorf("expected *NotFoundError for missing id, got %v", err)
	}
}

// TestCreateContentFiltering_PostsFullArrays proves the create body always
// serializes the list fields (empty ones included) — omitting always-present
// arrays on a v2 POST is the failure mode seen on firewall zones — and that
// the created policy round-trips.
func TestCreateContentFiltering_PostsFullArrays(t *testing.T) {
	const site = "default"
	var posted ContentFiltering
	srv := contentFilteringTestServer(t, site, `[]`, &posted)
	c := newAPGroupTestClient(t, srv)

	created, err := c.CreateContentFiltering(context.Background(), site, &ContentFiltering{
		Name:       "kids devices",
		Enabled:    true,
		Categories: []string{"FAMILY"},
		ClientMACs: []string{"aa:bb:cc:00:00:01"},
		NetworkIDs: []string{},
		AllowList:  []string{},
		BlockList:  []string{},
		SafeSearch: []string{"GOOGLE"},
		Schedule:   &ContentFilteringSchedule{Mode: "ALWAYS"},
	})
	if err != nil {
		t.Fatalf("CreateContentFiltering: %v", err)
	}
	if created.ID != "cccc0000000000000000c001" {
		t.Errorf("created id = %q", created.ID)
	}
	if posted.Name != "kids devices" || len(posted.Categories) != 1 {
		t.Errorf("posted = %+v", posted)
	}
	// Empty slices must have been present in the body (not dropped by
	// omitempty): after decoding they are non-nil empty slices.
	if posted.NetworkIDs == nil || posted.AllowList == nil || posted.BlockList == nil {
		t.Errorf("empty arrays were omitted from the create body: %+v", posted)
	}
	if posted.Schedule == nil || posted.Schedule.Mode != "ALWAYS" {
		t.Errorf("schedule = %+v", posted.Schedule)
	}
}
