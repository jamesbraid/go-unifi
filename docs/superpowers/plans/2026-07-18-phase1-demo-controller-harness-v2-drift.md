# Phase 1: Demo-Controller Harness + v2 Drift Probe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Boot a disposable UniFi controller (simulation mode) from Go tests and use it to detect drift between the live internal v2 API and the hand-written schemas in `overrides/resources/`.

**Architecture:** A new `internal/testenv` package starts `jacobalberty/unifi` via testcontainers-go (plain `GenericContainer`, NOT the compose module — the provider's compose approach drags in the whole `docker/compose` dependency and its bind-mount breaks under Podman; we embed the init script and copy it in via `ContainerFile` instead). Simulation mode (`is_simulation=true`) seeds an `admin`/`admin` account plus demo devices, so no setup wizard automation is needed. A small raw HTTP session (cookie auth, insecure TLS) fetches undecoded JSON — raw, because the point is seeing fields our structs *don't* know. The drift probe is a build-tagged (`integration`) test in `cmd/fields` comparing observed field sets against the override schemas.

**Tech Stack:** Go, testcontainers-go v0.35.0 (core module only), jacobalberty/unifi docker image, stdlib net/http.

## Global Constraints

- Commit style: kernel-style `subsystem: imperative summary` (no conventional-commit prefixes); wrap bodies at ~72 chars; end with `Co-Authored-By:` trailer matching the repo's existing commits.
- Integration tests MUST carry `//go:build integration` so `go test ./...` stays docker-free.
- Nothing extracted from Ubiquiti software may be committed (repo policy; see schemas/README.md).
- `gofmt`, `go vet ./...`, and `go test ./...` must pass after every task.
- The controller container is `jacobalberty/unifi` (default tag `v10.0.162`, override via `UNIFI_TEST_IMAGE`; exact-version installs via `UNIFI_TEST_PKGURL` pointing at a UniFi Network `.deb`).
- Demo-mode credentials are `admin`/`admin`, site `default`, classic API on container port 8443 (HTTPS, self-signed).

---

### Task 1: testenv session (raw authenticated client)

**Files:**
- Create: `internal/testenv/session.go`
- Test: `internal/testenv/session_test.go`

**Interfaces:**
- Consumes: nothing (stdlib only).
- Produces: `testenv.NewSession(baseURL string) *Session`, `(*Session).Login(ctx context.Context, username, password string) error`, `(*Session).GetJSON(ctx context.Context, path string) (any, int, error)` — returns decoded body (or nil on non-JSON), HTTP status, error. Used by Tasks 2–4 and by Phase 2.

- [ ] **Step 1: Write the failing tests**

```go
// internal/testenv/session_test.go
package testenv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeController mimics the classic controller's /api/login cookie flow.
func fakeController(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		var creds struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil || creds.Username != "admin" || creds.Password != "admin" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "unifises", Value: "fake-session"})
		w.Write([]byte(`{"meta":{"rc":"ok"}}`))
	})
	mux.HandleFunc("/v2/api/site/default/trafficroutes", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("unifises"); err != nil || c.Value != "fake-session" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`[{"_id":"x","enabled":true}]`))
	})
	return httptest.NewTLSServer(mux)
}

func TestSessionLoginAndGetJSON(t *testing.T) {
	srv := fakeController(t)
	defer srv.Close()

	s := NewSession(srv.URL)
	ctx := context.Background()

	if err := s.Login(ctx, "admin", "admin"); err != nil {
		t.Fatalf("login: %v", err)
	}

	body, status, err := s.GetJSON(ctx, "/v2/api/site/default/trafficroutes")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	items, ok := body.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("body = %#v, want 1-item array", body)
	}
}

func TestSessionLoginRejected(t *testing.T) {
	srv := fakeController(t)
	defer srv.Close()

	s := NewSession(srv.URL)
	if err := s.Login(context.Background(), "admin", "wrong"); err == nil {
		t.Fatal("expected login error")
	}
}

func TestSessionGetJSONNotFound(t *testing.T) {
	srv := fakeController(t)
	defer srv.Close()

	s := NewSession(srv.URL)
	_ = s.Login(context.Background(), "admin", "admin")
	_, status, err := s.GetJSON(context.Background(), "/v2/api/site/default/nope")
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/testenv/ -run TestSession -v`
Expected: FAIL — `undefined: NewSession`

- [ ] **Step 3: Write the implementation**

```go
// internal/testenv/session.go

// Package testenv boots a disposable UniFi Network controller in simulation
// mode and provides a raw HTTP session against it. The session deliberately
// returns undecoded JSON: its consumers are probes looking for fields the
// SDK's generated structs do NOT know about yet.
package testenv

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// Session is a cookie-authenticated raw client for a classic UniFi
// controller (self-signed TLS accepted).
type Session struct {
	baseURL string
	client  *http.Client
}

func NewSession(baseURL string) *Session {
	jar, _ := cookiejar.New(nil)
	return &Session{
		baseURL: baseURL,
		client: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // local throwaway controller
			},
		},
	}
}

// Login authenticates against the classic /api/login endpoint; the session
// cookie is kept in the jar.
func (s *Session) Login(ctx context.Context, username, password string) error {
	creds, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/login", bytes.NewReader(creds))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login returned HTTP %s", resp.Status)
	}
	return nil
}

// GetJSON fetches path and returns the decoded body (nil when the body is
// not JSON), the HTTP status code, and any transport error. Non-2xx statuses
// are not errors: probes need to distinguish 404 (endpoint absent in this
// controller version) from failure.
func (s *Session) GetJSON(ctx context.Context, path string) (any, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	var body any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, resp.StatusCode, nil
	}
	return body, resp.StatusCode, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/testenv/ -run TestSession -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Vet, build, commit**

```bash
gofmt -l internal/testenv && go vet ./internal/testenv/ && go build ./...
git add internal/testenv/session.go internal/testenv/session_test.go
git commit -m "testenv: add raw cookie-authenticated controller session

Returns undecoded JSON on purpose: consumers are drift probes looking
for fields the generated structs do not know about yet.

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 2: controller container harness

**Files:**
- Create: `internal/testenv/controller.go`
- Create: `internal/testenv/demo-mode` (init script, embedded)
- Test: `internal/testenv/controller_test.go` (unit: option/env resolution only)

**Interfaces:**
- Consumes: `NewSession`, `(*Session).Login` from Task 1.
- Produces: `testenv.Start(ctx context.Context, t *testing.T) *Controller` (skips the test when docker is unavailable; registers cleanup via `t.Cleanup`), `Controller{BaseURL, Username, Password, Site string}`, `(*Controller).NewSession(ctx, t) *Session` (logged-in session or fatal). Env overrides: `UNIFI_TEST_URL` (skip container entirely, use an existing controller), `UNIFI_TEST_IMAGE`, `UNIFI_TEST_PKGURL`. Used by Tasks 3–4 and Phases 2–3.

- [ ] **Step 1: Add the testcontainers dependency**

```bash
go get github.com/testcontainers/testcontainers-go@v0.35.0
go mod tidy
```

Expected: go.mod gains `github.com/testcontainers/testcontainers-go v0.35.0` (direct). Do NOT add the compose module.

- [ ] **Step 2: Create the demo-mode init script**

```sh
# internal/testenv/demo-mode
#!/bin/sh
# Runs inside jacobalberty/unifi at startup (via /unifi/init.d/). Simulation
# mode seeds an admin/admin account plus demo sites, devices, and networks —
# no setup wizard needed. Mirrors the script used by terraform-provider-unifi
# and ubiquiti-community/unifi-api.

write_config() {
  echo "${1}=${2}" >> /usr/lib/unifi/data/system.properties
}

write_config is_simulation true

# Seed enough demo devices for concurrent tests.
write_config demo.num_uap 3
write_config demo.num_ugw 1
write_config demo.num_usw 5
```

- [ ] **Step 3: Write the unit test for configuration resolution**

```go
// internal/testenv/controller_test.go
package testenv

import "testing"

func TestControllerConfigFromEnv(t *testing.T) {
	t.Setenv("UNIFI_TEST_IMAGE", "jacobalberty/unifi:v99")
	t.Setenv("UNIFI_TEST_PKGURL", "https://example.invalid/unifi.deb")

	cfg := configFromEnv()
	if cfg.Image != "jacobalberty/unifi:v99" {
		t.Errorf("Image = %q", cfg.Image)
	}
	if cfg.PkgURL != "https://example.invalid/unifi.deb" {
		t.Errorf("PkgURL = %q", cfg.PkgURL)
	}
}

func TestControllerConfigDefaults(t *testing.T) {
	t.Setenv("UNIFI_TEST_IMAGE", "")
	t.Setenv("UNIFI_TEST_PKGURL", "")

	cfg := configFromEnv()
	if cfg.Image != defaultImage {
		t.Errorf("Image = %q, want %q", cfg.Image, defaultImage)
	}
	if cfg.PkgURL != "" {
		t.Errorf("PkgURL = %q, want empty", cfg.PkgURL)
	}
}
```

- [ ] **Step 4: Run to verify failure**

Run: `go test ./internal/testenv/ -run TestControllerConfig -v`
Expected: FAIL — `undefined: configFromEnv`

- [ ] **Step 5: Write the harness**

```go
// internal/testenv/controller.go
package testenv

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

//go:embed demo-mode
var demoModeScript []byte

const (
	defaultImage = "jacobalberty/unifi:v10.0.162"
	// Simulation mode seeds this account (see demo-mode).
	demoUsername = "admin"
	demoPassword = "admin"
	demoSite     = "default"
)

type Controller struct {
	BaseURL  string
	Username string
	Password string
	Site     string
}

type config struct {
	Image  string
	PkgURL string
}

func configFromEnv() config {
	cfg := config{Image: defaultImage, PkgURL: os.Getenv("UNIFI_TEST_PKGURL")}
	if img := os.Getenv("UNIFI_TEST_IMAGE"); img != "" {
		cfg.Image = img
	}
	return cfg
}

// Start boots a disposable simulation-mode controller and returns its
// coordinates. It skips the test when docker is unavailable, honours
// UNIFI_TEST_URL to target an existing controller instead, and cleans the
// container up via t.Cleanup.
func Start(ctx context.Context, t *testing.T) *Controller {
	t.Helper()

	if url := os.Getenv("UNIFI_TEST_URL"); url != "" {
		return &Controller{BaseURL: strings.TrimRight(url, "/"), Username: demoUsername, Password: demoPassword, Site: demoSite}
	}

	cfg := configFromEnv()
	req := testcontainers.ContainerRequest{
		Image:        cfg.Image,
		ExposedPorts: []string{"8443/tcp"},
		Env: map[string]string{
			"UNIFI_STDOUT": "true",
			"TZ":           "Etc/UTC",
			"PKGURL":       cfg.PkgURL,
		},
		Files: []testcontainers.ContainerFile{{
			Reader:            strings.NewReader(string(demoModeScript)),
			ContainerFilePath: "/unifi/init.d/demo-mode",
			FileMode:          0o755,
		}},
		// TLS comes up before the app finishes booting; the real readiness
		// gate is the login poll below.
		WaitingFor: wait.ForListeningPort("8443/tcp").WithStartupTimeout(5 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("unable to start controller container (docker unavailable?): %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "8443/tcp")
	if err != nil {
		t.Fatalf("mapped port: %v", err)
	}

	c := &Controller{
		BaseURL:  fmt.Sprintf("https://%s:%s", host, port.Port()),
		Username: demoUsername,
		Password: demoPassword,
		Site:     demoSite,
	}

	// The seeded admin appears some time after the port opens; poll login.
	deadline := time.Now().Add(4 * time.Minute)
	for {
		s := NewSession(c.BaseURL)
		if err := s.Login(ctx, c.Username, c.Password); err == nil {
			return c
		} else if time.Now().After(deadline) {
			t.Fatalf("controller never became ready: %v", err)
		}
		time.Sleep(3 * time.Second)
	}
}

// NewSession returns a logged-in raw session against the controller.
func (c *Controller) NewSession(ctx context.Context, t *testing.T) *Session {
	t.Helper()
	s := NewSession(c.BaseURL)
	if err := s.Login(ctx, c.Username, c.Password); err != nil {
		t.Fatalf("login to %s: %v", c.BaseURL, err)
	}
	return s
}
```

- [ ] **Step 6: Run unit tests, vet, build**

Run: `go test ./internal/testenv/ -v && go vet ./... && go build ./...`
Expected: PASS (session + config tests; no container started).

- [ ] **Step 7: Commit**

```bash
git add internal/testenv/controller.go internal/testenv/demo-mode internal/testenv/controller_test.go go.mod go.sum
git commit -m "testenv: boot a disposable simulation-mode controller

Plain testcontainers GenericContainer rather than the compose module the
terraform provider uses: no docker/compose dependency tree, and the init
script is embedded and copied in via ContainerFile, which sidesteps the
bind-mount problems the provider hit under Podman. Simulation mode seeds
admin/admin plus demo devices, so readiness is a login poll.
UNIFI_TEST_URL targets an existing controller instead;
UNIFI_TEST_IMAGE/UNIFI_TEST_PKGURL pin the version.

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: integration smoke test

**Files:**
- Create: `internal/testenv/testenv_integration_test.go`

**Interfaces:**
- Consumes: `Start`, `(*Controller).NewSession`, `(*Session).GetJSON`.
- Produces: the `integration` build-tag convention every later integration test follows.

- [ ] **Step 1: Write the smoke test**

```go
//go:build integration

package testenv

import (
	"context"
	"testing"
	"time"
)

// TestIntegrationControllerBoots proves the harness end to end: container
// up, simulation admin seeded, classic API answering.
func TestIntegrationControllerBoots(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	c := Start(ctx, t)
	s := c.NewSession(ctx, t)

	body, status, err := s.GetJSON(ctx, "/api/s/"+c.Site+"/stat/sysinfo")
	if err != nil || status != 200 {
		t.Fatalf("sysinfo: status=%d err=%v", status, err)
	}
	t.Logf("sysinfo: %#v", body)

	wrapped, ok := body.(map[string]any)
	if !ok || wrapped["data"] == nil {
		t.Fatalf("unexpected sysinfo shape: %#v", body)
	}
}
```

- [ ] **Step 2: Verify it is excluded without the tag**

Run: `go test ./internal/testenv/ -run TestIntegration -v`
Expected: `no tests to run` (build tag keeps it out).

- [ ] **Step 3: Run it for real (requires docker/podman)**

Run: `go test -tags integration ./internal/testenv/ -run TestIntegrationControllerBoots -v -timeout 15m`
Expected: PASS after ~1–3 min of controller boot. If this environment has no docker, expected: SKIP with the "docker unavailable" message — in that case note it in the commit message and run it wherever docker exists before Phase 3.

- [ ] **Step 4: Commit**

```bash
git add internal/testenv/testenv_integration_test.go
git commit -m "testenv: add controller boot smoke test

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 4: v2 drift comparison logic

**Files:**
- Create: `cmd/fields/drift.go`
- Test: `cmd/fields/drift_test.go`

**Interfaces:**
- Consumes: nothing new (pure functions).
- Produces: `driftCompare(observed []map[string]any, schema map[string]any) driftResult` with `driftResult{LiveOnly, SchemaOnly []string}`; `driftIgnoredKeys` (the controller-envelope keys never present in schemas). Task 5 and Phase 3 consume these.

- [ ] **Step 1: Write the failing tests**

```go
// cmd/fields/drift_test.go
package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDriftCompare(t *testing.T) {
	schema := map[string]any{
		"name":        "",
		"enabled":     "true|false",
		"network_ids": []any{""},
	}
	observed := []map[string]any{
		{"_id": "a", "name": "x", "enabled": true, "origin_type": "zbf"},
		{"_id": "b", "name": "y", "site_id": "s", "sorting_weight": 1},
	}

	r := driftCompare(observed, schema)

	// _id/site_id are controller envelope, ignored; origin_type and
	// sorting_weight are genuine drift.
	require.Equal(t, []string{"origin_type", "sorting_weight"}, r.LiveOnly)
	// network_ids never appeared in live output: informational.
	require.Equal(t, []string{"network_ids"}, r.SchemaOnly)
}

func TestDriftCompareEmptyObserved(t *testing.T) {
	r := driftCompare(nil, map[string]any{"name": ""})
	require.Empty(t, r.LiveOnly)
	require.Equal(t, []string{"name"}, r.SchemaOnly)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./cmd/fields/ -run TestDriftCompare -v`
Expected: FAIL — `undefined: driftCompare`

- [ ] **Step 3: Implement**

```go
// cmd/fields/drift.go
package main

import "slices"

// driftIgnoredKeys are controller envelope fields that the schema files
// never carry (the generator adds them to every resource itself).
var driftIgnoredKeys = map[string]bool{
	"_id":            true,
	"site_id":        true,
	"attr_hidden":    true,
	"attr_hidden_id": true,
	"attr_no_delete": true,
	"attr_no_edit":   true,
}

type driftResult struct {
	// LiveOnly are fields the live controller emitted that the schema does
	// not define — real drift, the probe's reason to exist.
	LiveOnly []string
	// SchemaOnly are schema fields never observed live — usually just
	// absent-when-unset, so informational.
	SchemaOnly []string
}

// driftCompare unions the top-level keys of the observed objects and
// compares them with the schema definition's top-level keys.
func driftCompare(observed []map[string]any, schema map[string]any) driftResult {
	live := map[string]bool{}
	for _, item := range observed {
		for k := range item {
			if !driftIgnoredKeys[k] {
				live[k] = true
			}
		}
	}

	var r driftResult
	for k := range live {
		if _, ok := schema[k]; !ok {
			r.LiveOnly = append(r.LiveOnly, k)
		}
	}
	for k := range schema {
		if !live[k] {
			r.SchemaOnly = append(r.SchemaOnly, k)
		}
	}
	slices.Sort(r.LiveOnly)
	slices.Sort(r.SchemaOnly)
	return r
}
```

- [ ] **Step 4: Run tests, vet**

Run: `go test ./cmd/fields/ -run TestDriftCompare -v && go vet ./cmd/fields/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/fields/drift.go cmd/fields/drift_test.go
git commit -m "fields: add v2 schema drift comparison

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 5: v2 drift probe (integration test)

**Files:**
- Create: `cmd/fields/drift_integration_test.go`
- Modify: `schemas/README.md` (add a "Live verification" section at the end)

**Interfaces:**
- Consumes: `testenv.Start`, `(*Controller).NewSession`, `(*Session).GetJSON`, `driftCompare`, `driftResult`.
- Produces: `TestIntegrationV2Drift` — the check Phase 3 wires into CI.

- [ ] **Step 1: Verify the endpoint table against the SDK before writing it**

Run: `grep -n 'apiPath\|v2/api\|fmt.Sprintf' unifi/firewall_zone.go unifi/firewall_policy.go unifi/traffic_route.go unifi/nat.go unifi/dns_record.go unifi/ospf_router.go unifi/bgp_config.go | head -30`

Confirm each resource's list path. The table below is derived from each resource's `ResourcePath` (see `overrides/fields.toml`) with the classic-controller v2 prefix `/v2/api/site/{site}/…`; correct any path the grep contradicts before proceeding.

- [ ] **Step 2: Write the probe**

```go
//go:build integration

// cmd/fields/drift_integration_test.go
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
```

- [ ] **Step 3: Verify exclusion without the tag, then run live**

Run: `go test ./cmd/fields/ -run TestIntegrationV2Drift -v` → `no tests to run`.
Run: `go test -tags integration ./cmd/fields/ -run TestIntegrationV2Drift -v -timeout 20m`
Expected: subtests PASS, SKIP (empty/404), or FAIL with a concrete drift list. A FAIL here is the probe working: record the reported fields, add them to the corresponding `overrides/resources/*.json` with sensible validations (look at live values in the test log), regenerate (`go generate ./...`), and re-run until green. Commit any such schema updates separately as `overrides: <resource> catch up with live controller`.

- [ ] **Step 4: Document**

Append to `schemas/README.md`:

```markdown
## Live verification

`go test -tags integration ./internal/testenv/ ./cmd/fields/` boots a
disposable simulation-mode controller (jacobalberty/unifi via
testcontainers; `admin`/`admin`) and compares the hand-written v2 schemas
in `overrides/resources/` against what the live API serves. Pin the
controller build with `UNIFI_TEST_IMAGE` or `UNIFI_TEST_PKGURL` (a UniFi
Network .deb URL), or point `UNIFI_TEST_URL` at an existing controller.
```

- [ ] **Step 5: Full check and commit**

```bash
gofmt -l cmd/fields internal/testenv && go vet ./... && go test ./... && go build ./...
git add cmd/fields/drift_integration_test.go schemas/README.md
git commit -m "fields: probe live v2 endpoints for schema drift

The hand-written v2 definitions in overrides/resources/ had no drift
signal when Ubiquiti changes those endpoints. Boot the simulation-mode
controller and compare each definition's fields against what the live
API serves: live-only fields fail (upstream drift), schema-only fields
log (absent-when-unset is normal).

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Self-review notes

- Spec coverage: harness (T2), raw session (T1), smoke (T3), drift compare + probe (T4–T5), docs (T5). Escape hatches (`UNIFI_TEST_URL`, image/PKGURL pinning) in T2.
- Types consistent: `Controller`/`Session`/`driftCompare` signatures match across tasks.
- Known risks called out in-line: exact v2 paths verified in T5 Step 1; empty collections and 404s skip rather than fail; docker-less environments skip.
