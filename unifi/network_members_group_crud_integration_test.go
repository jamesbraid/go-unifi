//go:build integration

// unifi/network_members_group_crud_integration_test.go
package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationNetworkMembersGroupCRUD exercises every verb against a live
// controller, because four of the five used to be pointed at the wrong path.
//
// The collection is asymmetric, measured on 10.4.57: only the list is served
// by the plural path, and create, read-one, update and delete are served by
// the singular one. The client used the plural path throughout, so Create
// answered 405 and everything except List answered 404 -- the resource was
// effectively unusable, and nothing exercised it.
//
// A resource whose whole client surface is broken is exactly what an
// end-to-end test catches and a unit test cannot, so this walks the lot.
func TestIntegrationNetworkMembersGroupCRUD(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)

	api, err := New(ctx, &Config{
		BaseURL:       c.BaseURL,
		Username:      c.Username,
		Password:      c.Password,
		AllowInsecure: true,
	})
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	// type is an enum of USERS|CLIENTS. Sending anything else is answered
	// with the accepted values, which is how the enum was established.
	created, err := api.CreateNetworkMembersGroup(ctx, c.Site, &NetworkMembersGroup{
		Name: "crud-probe", Type: "CLIENTS",
	})
	if err != nil {
		t.Fatalf("create: %v\n\nThe create path is the singular one; a 405 here means it went "+
			"back to the plural collection.", err)
	}
	if created.ID == "" {
		t.Fatal("create returned no id")
	}
	defer api.DeleteNetworkMembersGroup(context.WithoutCancel(ctx), c.Site, created.ID) //nolint:errcheck

	// Members was left nil on the way in, so this also covers the nil-as-empty
	// marshalling: a null there is rejected with "members: must not be null".
	if created.Members == nil {
		t.Error("members came back nil; the controller stores an empty list")
	}

	got, err := api.GetNetworkMembersGroup(ctx, c.Site, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "crud-probe" {
		t.Errorf("get returned name %q, want crud-probe", got.Name)
	}

	listed, err := api.ListNetworkMembersGroups(ctx, c.Site)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, g := range listed {
		if g.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("the created group is not in the list of %d; list and the other verbs "+
			"disagree about which collection they address", len(listed))
	}

	got.Name = "crud-probe-renamed"
	updated, err := api.UpdateNetworkMembersGroup(ctx, c.Site, got)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "crud-probe-renamed" {
		t.Errorf("update returned name %q, want crud-probe-renamed", updated.Name)
	}

	if err := api.DeleteNetworkMembersGroup(ctx, c.Site, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := api.GetNetworkMembersGroup(ctx, c.Site, created.ID); err == nil {
		t.Error("the group is still readable after a delete")
	}
}
