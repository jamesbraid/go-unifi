//go:build integration

package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// harnessClient builds the real client against the disposable controller,
// so a test can drive a public method end to end rather than only the raw
// request it is expected to make.
func harnessClient(ctx context.Context, t *testing.T, c *controllertest.Controller) *ApiClient {
	t.Helper()
	client, err := New(ctx, &Config{
		BaseURL:       c.BaseURL,
		Username:      c.Username,
		Password:      c.Password,
		AllowInsecure: true,
	})
	if err != nil {
		t.Fatalf("client against the harness controller: %v", err)
	}
	return client
}

// TestIntegrationUpdateSite drives update-site end to end. From 2020 until
// this test existed the site commands posted to s/<site>/cmd/sitemgr -- no
// api/ prefix, unlike every other command in the package -- which resolves
// to /s/... on a classic controller and /proxy/network/s/... on UniFi OS,
// and neither is served. This is the first proof the path is right.
func TestIntegrationUpdateSite(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	client := harnessClient(ctx, t, c)

	before, err := client.GetSiteByName(ctx, c.Site)
	if err != nil {
		t.Fatalf("GetSiteByName: %v", err)
	}

	if _, err := client.UpdateSite(ctx, c.Site, "renamed-by-update-site"); err != nil {
		t.Fatalf("UpdateSite: %v", err)
	}

	after, err := client.GetSiteByName(ctx, c.Site)
	if err != nil {
		t.Fatalf("GetSiteByName after the write: %v", err)
	}
	if after.Description != "renamed-by-update-site" {
		t.Errorf("desc did not land: %q", after.Description)
	}
	if after.ID != before.ID || after.Name != before.Name {
		t.Errorf("update-site changed the site's identity: before %+v, after %+v", before, after)
	}
}
