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
// bare JSON array, and is method-aware so it can also exercise
// update/delete against the same collection, mirroring natTestServer:
//   - GET    {listPath}     -> the fixture array (list/get-via-list)
//   - POST   {listPath}     -> echoes the decoded body back with an _id
//     assigned, the way the controller answers a create
//   - PUT    {listPath}/id  -> echoes the decoded body back as the updated
//     policy, matching updateContentFiltering's single-object response
//     decode; rejects a bare-collection PUT or an id mismatch
//   - DELETE {listPath}/id  -> 200 with no body, matching
//     deleteContentFiltering's nil respBody decode
//   - other GETs under {listPath}/ -> 405 + HTML like other v2 collection
//     endpoints, so GetContentFiltering is locked to the list-and-filter
//     read.
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
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, listPath+"/"):
			id := strings.TrimPrefix(r.URL.Path, listPath+"/")
			var policy ContentFiltering
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &policy); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if policy.ID != id {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(policy)
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

// TestGetContentFiltering_ReadsViaList proves GetContentFiltering resolves a
// policy by listing the bare-array collection and filtering by ID, and that
// the wire fields observed on a live controller decode.
//
// It requests the *second* fixture element deliberately: a regression that
// unconditionally returns respBody[0] (ignoring the requested id) would still
// pass a test that only ever asked for the first id.
func TestGetContentFiltering_ReadsViaList(t *testing.T) {
	const site = "default"
	srv := contentFilteringTestServer(t, site, `[
		{"_id":"cccc0000000000000000c001","name":"guest","enabled":false,
		 "categories":[],"client_macs":[],"network_ids":[],
		 "allow_list":[],"block_list":[],"safe_search":[],
		 "schedule":{"mode":"ALWAYS"}},
		{"_id":"cccc0000000000000000c002","name":"kids devices","enabled":true,
		 "categories":["FAMILY","ADVERTISEMENT"],
		 "client_macs":["aa:bb:cc:00:00:01"],"network_ids":[],
		 "allow_list":[],"block_list":["example.test"],
		 "safe_search":["GOOGLE","YOUTUBE","BING"],
		 "schedule":{"mode":"ALWAYS"}}
	]`, nil)
	c := newAPGroupTestClient(t, srv)

	got, err := c.GetContentFiltering(context.Background(), site, "cccc0000000000000000c002")
	if err != nil {
		t.Fatalf("GetContentFiltering errored (regressed to per-id GET?): %v", err)
	}
	if got.ID != "cccc0000000000000000c002" {
		t.Fatalf("id = %q, want cccc...c002 (regressed to respBody[0]?)", got.ID)
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

// TestListContentFiltering_ReturnsAll pins ListContentFiltering: it must
// decode the full bare-array collection response, in order, with every
// policy's fields intact — not just a single element or a filtered subset.
func TestListContentFiltering_ReturnsAll(t *testing.T) {
	const site = "default"
	srv := contentFilteringTestServer(t, site, `[
		{"_id":"cccc0000000000000000c001","name":"guest","enabled":false,
		 "categories":[],"client_macs":[],"network_ids":[],
		 "allow_list":[],"block_list":[],"safe_search":[],
		 "schedule":{"mode":"ALWAYS"}},
		{"_id":"cccc0000000000000000c002","name":"kids devices","enabled":true,
		 "categories":["FAMILY","ADVERTISEMENT"],
		 "client_macs":["aa:bb:cc:00:00:01"],"network_ids":[],
		 "allow_list":[],"block_list":["example.test"],
		 "safe_search":["GOOGLE","YOUTUBE","BING"],
		 "schedule":{"mode":"ALWAYS"}}
	]`, nil)
	c := newAPGroupTestClient(t, srv)

	got, err := c.ListContentFiltering(context.Background(), site)
	if err != nil {
		t.Fatalf("ListContentFiltering errored: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(ListContentFiltering) = %d, want 2: %+v", len(got), got)
	}
	if got[0].ID != "cccc0000000000000000c001" || got[1].ID != "cccc0000000000000000c002" {
		t.Errorf("ids = [%q, %q], want [cccc...c001, cccc...c002]", got[0].ID, got[1].ID)
	}
	if got[0].Name != "guest" || got[0].Enabled {
		t.Errorf("policy[0] = %+v", got[0])
	}
	if got[1].Name != "kids devices" || !got[1].Enabled {
		t.Errorf("policy[1] = %+v", got[1])
	}
	if len(got[1].Categories) != 2 || got[1].Categories[0] != "FAMILY" {
		t.Errorf("policy[1].categories = %v", got[1].Categories)
	}
	if len(got[1].BlockList) != 1 || got[1].BlockList[0] != "example.test" {
		t.Errorf("policy[1].block_list = %v", got[1].BlockList)
	}
}

// TestUpdateContentFiltering_RoundTrip pins UpdateContentFiltering: it must
// PUT to the per-id path (collection path + "/" + d.ID, matching
// updateContentFiltering's fmt.Sprintf) and decode the single-object
// response body. The test server rejects a bare-collection PUT or an id
// mismatch, so a regression to the wrong path or a dropped ID fails loudly
// instead of silently succeeding against the wrong endpoint.
func TestUpdateContentFiltering_RoundTrip(t *testing.T) {
	const site = "default"
	srv := contentFilteringTestServer(t, site, `[]`, nil)
	c := newAPGroupTestClient(t, srv)

	in := &ContentFiltering{
		ID:         "cccc0000000000000000c001",
		Name:       "kids devices updated",
		Enabled:    false,
		Categories: []string{"FAMILY"},
		ClientMACs: []string{"aa:bb:cc:00:00:01"},
		NetworkIDs: []string{},
		AllowList:  []string{},
		BlockList:  []string{"example.test"},
		SafeSearch: []string{"GOOGLE"},
		Schedule:   &ContentFilteringSchedule{Mode: "ALWAYS"},
	}
	got, err := c.UpdateContentFiltering(context.Background(), site, in)
	if err != nil {
		t.Fatalf("UpdateContentFiltering errored: %v", err)
	}
	if got.ID != "cccc0000000000000000c001" {
		t.Errorf("id = %q, want cccc...c001 (wrong path hit?)", got.ID)
	}
	if got.Name != "kids devices updated" || got.Enabled {
		t.Errorf("updated policy = %+v, want echo of input", got)
	}
	if len(got.BlockList) != 1 || got.BlockList[0] != "example.test" {
		t.Errorf("updated block_list = %v", got.BlockList)
	}
	if got.Schedule == nil || got.Schedule.Mode != "ALWAYS" {
		t.Errorf("updated schedule = %+v", got.Schedule)
	}
}

// TestDeleteContentFiltering_Success pins DeleteContentFiltering: it must
// DELETE the per-id path and treat a no-body 200 response as success
// (matching deleteContentFiltering's nil respBody), rather than erroring on
// an empty/undecodable body.
func TestDeleteContentFiltering_Success(t *testing.T) {
	const site = "default"
	srv := contentFilteringTestServer(t, site, `[]`, nil)
	c := newAPGroupTestClient(t, srv)

	if err := c.DeleteContentFiltering(context.Background(), site, "cccc0000000000000000c001"); err != nil {
		t.Fatalf("DeleteContentFiltering errored: %v", err)
	}
}
