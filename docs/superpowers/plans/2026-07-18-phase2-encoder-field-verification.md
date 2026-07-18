# Phase 2: Encoder Field Verification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Empirically classify the ~39 Network wire fields the hand-written encoder has never sent (the `TODO: possibly a real gap` allowlist entries in `unifi/network_encode_coverage_test.go`), then wire the fields the controller actually accepts into the encoder.

**Architecture:** Extend the Phase 1 `testenv.Session` with raw write methods, then drive a disposable simulation-mode controller with **raw** requests (raw is the point: the SDK encoder would strip exactly the fields under test). For each candidate field: create a minimal network of the right purpose with the field set, read it back, classify as PERSISTED / STRIPPED / REJECTED, delete the network. The output report drives mechanical encoder wiring, with the Phase-1-era coverage test enforcing allowlist removal.

**Tech Stack:** Go, `internal/testenv` (Phase 1), jacobalberty/unifi simulation controller.

**Prerequisite:** Phase 1 merged (`internal/testenv` with `Start`, `Controller{BaseURL,Username,Password,Site}`, `(*Controller).NewSession`, `(*Session).GetJSON(ctx, path) (any, int, error)`).

## Global Constraints

- Commit style: kernel-style `subsystem: imperative summary`; `Co-Authored-By:` trailer as in existing commits.
- Integration tests carry `//go:build integration`; plain `go test ./...` stays docker-free.
- Raw requests only for the probe — never the SDK's `Network` marshaller (it drops the fields under test).
- Mutations happen ONLY against the disposable container, never `UNIFI_TEST_URL`-supplied controllers: the probe must `t.Skip` when `UNIFI_TEST_URL` is set.
- Every encoder change must keep `go test ./unifi/...` green, including `TestNetworkEncoderCoversGeneratedFields` and `TestNetworkEncoderValueFlow`.

---

### Task 1: session write methods

**Files:**
- Modify: `internal/testenv/session.go`
- Test: `internal/testenv/session_test.go` (extend)

**Interfaces:**
- Consumes: `Session` from Phase 1.
- Produces: `(*Session).PostJSON(ctx, path string, body any) (any, int, error)` and `(*Session).DeleteJSON(ctx, path string) (any, int, error)` — same return convention as `GetJSON` (decoded body or nil, status, transport error; non-2xx is NOT an error).

- [ ] **Step 1: Extend the fake controller and write failing tests**

Add to `fakeController` in `internal/testenv/session_test.go`:

```go
	mux.HandleFunc("/api/s/default/rest/networkconf", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("unifises"); err != nil || c.Value != "fake-session" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["_id"] = "new-id"
			resp := map[string]any{"meta": map[string]any{"rc": "ok"}, "data": []any{body}}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/s/default/rest/networkconf/new-id", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.Write([]byte(`{"meta":{"rc":"ok"},"data":[]}`))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
```

New tests:

```go
func TestSessionPostAndDeleteJSON(t *testing.T) {
	srv := fakeController(t)
	defer srv.Close()
	ctx := context.Background()

	s := NewSession(srv.URL)
	if err := s.Login(ctx, "admin", "admin"); err != nil {
		t.Fatal(err)
	}

	body, status, err := s.PostJSON(ctx, "/api/s/default/rest/networkconf", map[string]any{"name": "probe"})
	if err != nil || status != 200 {
		t.Fatalf("post: status=%d err=%v", status, err)
	}
	wrapped := body.(map[string]any)
	created := wrapped["data"].([]any)[0].(map[string]any)
	if created["_id"] != "new-id" || created["name"] != "probe" {
		t.Fatalf("created = %#v", created)
	}

	_, status, err = s.DeleteJSON(ctx, "/api/s/default/rest/networkconf/new-id")
	if err != nil || status != 200 {
		t.Fatalf("delete: status=%d err=%v", status, err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/testenv/ -run TestSessionPostAndDeleteJSON -v`
Expected: FAIL — `undefined: (*Session).PostJSON` (compile error).

- [ ] **Step 3: Implement**

Add to `internal/testenv/session.go`:

```go
// PostJSON sends a JSON body; same return convention as GetJSON (non-2xx
// statuses are results, not errors — probes classify them).
func (s *Session) PostJSON(ctx context.Context, path string, body any) (any, int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	return s.do(ctx, http.MethodPost, path, payload)
}

// DeleteJSON deletes path; same return convention as GetJSON.
func (s *Session) DeleteJSON(ctx context.Context, path string) (any, int, error) {
	return s.do(ctx, http.MethodDelete, path, nil)
}

func (s *Session) do(ctx context.Context, method, path string, payload []byte) (any, int, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, resp.StatusCode, nil
	}
	return decoded, resp.StatusCode, nil
}
```

Also refactor `GetJSON` to call `s.do(ctx, http.MethodGet, path, nil)` (delete its duplicated body; the Phase 1 tests must still pass).

- [ ] **Step 4: Run all session tests, vet, commit**

```bash
go test ./internal/testenv/ -v && go vet ./internal/testenv/
git add internal/testenv/session.go internal/testenv/session_test.go
git commit -m "testenv: add raw write methods to the controller session

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 2: candidate field table

**Files:**
- Create: `unifi/network_field_candidates_test.go` (plain test file, no build tag — the table is compile-checked everywhere)

**Interfaces:**
- Consumes: the `TODO: possibly a real gap` entries in `unifi/network_encode_coverage_test.go` (read that file; the allowlist entries carry the wire names).
- Produces: `networkFieldCandidates []fieldCandidate` with `fieldCandidate{Wire string; Purpose string; Value any; Prereq map[string]any}` — consumed by Task 3.

- [ ] **Step 1: Build the table from the coverage allowlist**

Open `unifi/network_encode_coverage_test.go`, collect every allowlist entry whose comment contains `TODO: possibly a real gap`, and write one `fieldCandidate` per wire name into the new file. Choose `Purpose` from the field's domain and `Value` from the generated struct's type and validation comment in `unifi/network.generated.go` (grep the wire name for its tag line). Prereqs carry whatever sibling fields the controller needs to accept the candidate (e.g. WAN candidates need `wan_type`). The full mechanical pattern, applied to every entry:

```go
// unifi/network_field_candidates_test.go
package unifi

// fieldCandidate is one encoder-allowlist wire field to verify against a
// live controller: create a network of Purpose with Value (plus Prereq
// siblings), read back, and see whether the controller kept it.
type fieldCandidate struct {
	Wire    string
	Purpose string
	Value   any
	Prereq  map[string]any
}

// networkFieldCandidates lists the coverage-test allowlist entries marked
// "TODO: possibly a real gap". Values follow the generated struct's type and
// validation comment for each wire name.
var networkFieldCandidates = []fieldCandidate{
	{Wire: "interface_mtu", Purpose: PurposeCorporate, Value: 1400, Prereq: map[string]any{"interface_mtu_enabled": true}},
	{Wire: "interface_mtu_enabled", Purpose: PurposeCorporate, Value: true, Prereq: nil},
	{Wire: "firewall_zone_id", Purpose: PurposeCorporate, Value: "@zone", Prereq: nil}, // "@zone": resolved live to an existing zone id
	{Wire: "wan_ip", Purpose: PurposeWAN, Value: "192.0.2.10", Prereq: map[string]any{"wan_type": "static", "wan_netmask": "255.255.255.0", "wan_gateway": "192.0.2.1"}},
	// ... one entry per remaining TODO field, same shape ...
}
```

Every TODO wire name from the coverage file MUST appear exactly once (the Task 3 probe cross-checks this and fails otherwise — that is the completeness guarantee, so finishing this step means: run the Task 3 Step 1 unit test).

- [ ] **Step 2: Compile check**

Run: `go vet ./unifi/ && go test ./unifi/ -run TestNetworkEncoder -v`
Expected: compiles; existing coverage tests still PASS (the new file only adds a table).

- [ ] **Step 3: Commit**

```bash
git add unifi/network_field_candidates_test.go
git commit -m "unifi: table the unverified encoder allowlist fields

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: live classification probe

**Files:**
- Create: `unifi/network_field_probe_integration_test.go`
- Test (unit part): completeness check comparing the candidate table against the coverage allowlist TODOs (lives in `unifi/network_field_candidates_test.go`)

**Interfaces:**
- Consumes: `testenv.Start`, `(*Session).PostJSON/GetJSON/DeleteJSON`, `networkFieldCandidates`.
- Produces: `TestIntegrationNetworkFieldProbe` emitting a classification report; its output drives Task 4.

- [ ] **Step 1: Add the completeness unit test**

In `unifi/network_field_candidates_test.go`:

```go
func TestFieldCandidatesCoverAllTODOs(t *testing.T) {
	// networkEncoderPresenceAllowlistTODOs must be exported from the
	// coverage test file as a []string of the TODO wire names (extract the
	// TODO entries into that named slice while doing this task).
	want := map[string]bool{}
	for _, w := range networkEncoderPresenceAllowlistTODOs {
		want[w] = true
	}
	got := map[string]bool{}
	for _, c := range networkFieldCandidates {
		if got[c.Wire] {
			t.Errorf("duplicate candidate %q", c.Wire)
		}
		got[c.Wire] = true
	}
	for w := range want {
		if !got[w] {
			t.Errorf("allowlist TODO %q has no candidate entry", w)
		}
	}
	for w := range got {
		if !want[w] {
			t.Errorf("candidate %q is not an allowlist TODO (already wired or stale?)", w)
		}
	}
}
```

This requires a small refactor in `network_encode_coverage_test.go`: pull the TODO wire names into `var networkEncoderPresenceAllowlistTODOs = []string{...}` and build the existing allowlist from it plus the justified-permanent entries (behavior unchanged; coverage tests must still pass).

- [ ] **Step 2: Write the probe**

```go
//go:build integration

// unifi/network_field_probe_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/testenv"
)

// TestIntegrationNetworkFieldProbe classifies every candidate field by
// creating a throwaway network with the field set (raw request — the SDK
// encoder would strip it), reading it back, and comparing.
//
//	PERSISTED: controller stored and returned the value -> wire it (Task 4)
//	STRIPPED:  create succeeded, field absent/zero on read-back
//	REJECTED:  create failed with the field present
func TestIntegrationNetworkFieldProbe(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := testenv.Start(ctx, t)
	s := c.NewSession(ctx, t)

	// Resolve "@zone" placeholders to a real zone id once.
	zoneID := firstZoneID(ctx, t, s, c.Site)

	base := func(purpose string, n int) map[string]any {
		m := map[string]any{
			"name":    fmt.Sprintf("probe-%d", n),
			"purpose": purpose,
			"enabled": true,
		}
		if purpose == PurposeCorporate || purpose == PurposeGuest {
			m["ip_subnet"] = fmt.Sprintf("10.99.%d.1/24", n%250)
			m["vlan_enabled"] = true
			m["vlan"] = 100 + n
		}
		return m
	}

	for i, cand := range networkFieldCandidates {
		t.Run(cand.Wire, func(t *testing.T) {
			payload := base(cand.Purpose, i)
			for k, v := range cand.Prereq {
				payload[k] = v
			}
			value := cand.Value
			if value == "@zone" {
				value = zoneID
			}
			payload[cand.Wire] = value

			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", payload)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}
			if status != 200 {
				t.Logf("REJECTED %s (HTTP %d): %v", cand.Wire, status, body)
				return
			}

			created := firstData(t, body)
			id, _ := created["_id"].(string)
			defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck

			got, ok := created[cand.Wire]
			// Some fields only appear on read-back; check GET too.
			if !ok {
				fresh, status, _ := s.GetJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id)
				if status == 200 {
					if m := firstData(t, fresh); m != nil {
						got, ok = m[cand.Wire]
					}
				}
			}

			if ok && fmt.Sprintf("%v", got) == fmt.Sprintf("%v", value) {
				t.Logf("PERSISTED %s = %v", cand.Wire, got)
			} else if ok {
				t.Logf("MUTATED %s: sent %v, got %v", cand.Wire, value, got)
			} else {
				t.Logf("STRIPPED %s", cand.Wire)
			}
		})
	}
}

// firstData unwraps {"meta":..., "data":[obj,...]} envelopes; v2-style bare
// objects/arrays pass through.
func firstData(t *testing.T, body any) map[string]any {
	t.Helper()
	switch v := body.(type) {
	case map[string]any:
		if data, ok := v["data"].([]any); ok && len(data) > 0 {
			if m, ok := data[0].(map[string]any); ok {
				return m
			}
			return nil
		}
		return v
	case []any:
		if len(v) > 0 {
			if m, ok := v[0].(map[string]any); ok {
				return m
			}
		}
	}
	return nil
}

func firstZoneID(ctx context.Context, t *testing.T, s *testenv.Session, site string) string {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/v2/api/site/"+site+"/firewall/zone")
	if err != nil || status != 200 {
		t.Logf("no firewall zones available (status %d, %v); @zone candidates will be REJECTED", status, err)
		return "000000000000000000000000"
	}
	if items, ok := body.([]any); ok && len(items) > 0 {
		if m, ok := items[0].(map[string]any); ok {
			if id, ok := m["_id"].(string); ok {
				return id
			}
		}
	}
	return "000000000000000000000000"
}
```

- [ ] **Step 3: Run unit + integration**

Run: `go test ./unifi/ -run TestFieldCandidatesCoverAllTODOs -v` → PASS.
Run: `go test -tags integration ./unifi/ -run TestIntegrationNetworkFieldProbe -v -timeout 30m 2>&1 | tee /tmp/field-probe.log`
Expected: one subtest per candidate, each logging PERSISTED / MUTATED / STRIPPED / REJECTED. Save the log — it is Task 4's input.

- [ ] **Step 4: Commit**

```bash
git add unifi/network_field_probe_integration_test.go unifi/network_field_candidates_test.go unifi/network_encode_coverage_test.go
git commit -m "unifi: classify unverified encoder fields against a live controller

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 4: wire the PERSISTED fields into the encoder

**Files:**
- Modify: `unifi/network_encode.go`
- Modify: `unifi/network_encode_coverage_test.go` (remove wired entries from the TODO slice)
- Modify: `unifi/network_encode_test.go` (round-trip coverage per wired group)

**Interfaces:**
- Consumes: the probe log classification from Task 3; the purpose marshal structs in `network_encode.go`.
- Produces: encoder emitting every PERSISTED field; shrunken allowlist.

- [ ] **Step 1: For each PERSISTED field, add it to the right purpose struct**

Worked example (repeat the identical pattern per field, batching fields of the same purpose into one edit): probe says `PERSISTED interface_mtu` on corporate. In `network_encode.go`, find the corporate marshal struct, add next to its closest siblings:

```go
		InterfaceMtu        *int64 `json:"interface_mtu,omitempty"`
		InterfaceMtuEnabled bool   `json:"interface_mtu_enabled"`
```

and in the corresponding literal:

```go
		InterfaceMtu:        n.InterfaceMtu,
		InterfaceMtuEnabled: n.InterfaceMtuEnabled,
```

(Field names, pointer-ness, and omitempty must match `network.generated.go` exactly — the value-flow test fails on any cross-sourcing mistake.)

- [ ] **Step 2: Remove each wired wire name from `networkEncoderPresenceAllowlistTODOs`**

The coverage test now REQUIRES the encoder to emit it; leaving a wired name in the allowlist fails the staleness check, so this step is self-verifying.

- [ ] **Step 3: Add round-trip tests per wired group**

Follow the existing pattern (`TestMarshalNetworkWANMssClamp` in `network_encode_test.go`): marshal a Network with the fields set, assert emitted values; marshal unset, assert omitted.

- [ ] **Step 4: Full verification**

Run: `go test ./unifi/... -v -run 'TestNetworkEncoder|TestMarshalNetwork'` → all PASS.
Run: `go build ./... && go vet ./... && go test ./...` → PASS.

- [ ] **Step 5: Commit (one commit per purpose-group batch)**

```bash
git add unifi/network_encode.go unifi/network_encode_test.go unifi/network_encode_coverage_test.go
git commit -m "unifi: serialize live-verified <group> network fields

Verified against a simulation-mode 10.x controller: the controller
persists these on <purpose> networks (see the field probe). STRIPPED and
REJECTED fields stay allowlisted with their classification recorded.

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

- [ ] **Step 6: Record STRIPPED/REJECTED outcomes**

For fields that did not persist, update their allowlist comments from `TODO: possibly a real gap` to the measured outcome (e.g. `probe 2026-07: STRIPPED by 10.0 sim controller`), so the list stops looking actionable when it isn't. Commit with the Task 5 batch or separately as `unifi: record field probe outcomes in the encoder allowlist`.

---

## Self-review notes

- Simulation-mode caveat is inherent: STRIPPED there might persist on real hardware for device-dependent fields; the allowlist comments record the evidence base, and `UNIFI_TEST_PKGURL` lets the probe re-run on newer controller builds.
- The completeness unit test pins the candidate table to the allowlist TODOs in both directions, so neither can drift silently.
- Mutating probes are container-only by construction (`UNIFI_TEST_URL` skip).
