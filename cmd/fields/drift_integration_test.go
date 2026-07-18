//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/testenv"
)

// v2Probes maps each hand-written schema in overrides/resources/ to the
// live endpoint that serves it. list=false endpoints return a single object.
var v2Probes = []struct {
	schemaFile string
	path       string
	list       bool
}{
	{"FirewallZone.json", "/v2/api/site/%s/firewall/zone", true},
	{"FirewallPolicy.json", "/v2/api/site/%s/firewall-policies", true},
	{"TrafficRoute.json", "/v2/api/site/%s/trafficroutes", true},
	{"Nat.json", "/v2/api/site/%s/nat", true},
	{"DnsRecord.json", "/v2/api/site/%s/static-dns", true},
	{"OSPFRouter.json", "/v2/api/site/%s/ospf/router", true},
	{"BgpConfig.json", "/v2/api/site/%s/bgp/config", false},
}

// TestIntegrationV2Drift compares the hand-written v2 schemas against what a
// live controller actually serves. LiveOnly fields fail: they are upstream
// drift our definitions are missing. SchemaOnly fields only log: absent
// wire fields are normal for unset options.
func TestIntegrationV2Drift(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	c := testenv.Start(ctx, t)
	s := c.NewSession(ctx, t)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	resourcesDir := filepath.Join(findModuleRoot(wd), "overrides", "resources")

	for _, probe := range v2Probes {
		t.Run(probe.schemaFile, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(resourcesDir, probe.schemaFile))
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			var schema map[string]any
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatalf("schema parse: %v", err)
			}

			body, status, err := s.GetJSON(ctx, fmt.Sprintf(probe.path, c.Site))
			if err != nil {
				t.Fatalf("probe: %v", err)
			}
			if status == 404 {
				t.Skipf("endpoint absent on this controller version (404)")
			}
			if status != 200 {
				t.Fatalf("probe status = %d", status)
			}
			if body == nil {
				t.Fatalf("probe returned HTTP 200 with a non-JSON body — controller not serving the API?")
			}

			var observed []map[string]any
			switch v := body.(type) {
			case []any:
				for _, item := range v {
					if m, ok := item.(map[string]any); ok {
						observed = append(observed, m)
					}
				}
			case map[string]any:
				observed = append(observed, v)
			}
			if len(observed) == 0 {
				t.Skipf("no live objects to compare (empty collection)")
			}

			r := driftCompare(observed, schema)
			if len(r.SchemaOnly) > 0 {
				t.Logf("schema-only fields (unset live, informational): %v", r.SchemaOnly)
			}
			if len(r.LiveOnly) > 0 {
				t.Errorf("live controller emits fields missing from %s: %v — update overrides/resources/%s",
					probe.schemaFile, r.LiveOnly, probe.schemaFile)
			}
		})
	}
}
