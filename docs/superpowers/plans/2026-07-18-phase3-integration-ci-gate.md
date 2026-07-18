# Phase 3: Integration CI Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run the Phase 1 drift probe and integration smoke suite in CI — pinned to the exact controller build the schemas came from — and make it a gate for auto-merged schema updates.

**Architecture:** The generator records the downloaded artifact URL in a new tracked marker `schemas/ARTIFACT`. A new `integration.yaml` workflow (pull_request with path filter + nightly schedule) boots the container on the runner's docker, passing `UNIFI_TEST_PKGURL=$(cat schemas/ARTIFACT)` when the artifact is a `.deb` (UOS-installer artifacts can't install into the image, so those runs fall back to the default image tag). Branch protection then adds the job to the required checks, which is what actually gates `gh pr merge --auto`.

**Tech Stack:** GitHub Actions (ubuntu-latest ships docker), `go test -tags integration`, Phase 1 harness.

**Prerequisite:** Phase 1 merged. (Phase 2 is independent; its mutating probe stays manual/nightly-only and is NOT part of the PR gate.)

## Global Constraints

- Commit style: kernel-style `subsystem: imperative summary`; `Co-Authored-By:` trailer as in existing commits.
- Workflows pin action SHAs with a trailing `# vX.Y.Z` comment; `uvx --from yamllint yamllint -c .yamllint .github/workflows/` must report zero errors.
- Jobs get explicit `timeout-minutes` (controller boot is minutes; a hung container must not burn a 6h runner).
- The PR gate must be advisory-fail (visible red check) rather than silently skipped when docker/image problems occur — a skip that looks green would defeat the gate.

---

### Task 1: record the artifact URL as a tracked marker

**Files:**
- Modify: `cmd/fields/main.go` (`buildSchemas` writes the marker; `snapshotCurrent` unaffected)
- Modify: `cmd/fields/extract_test.go` (assert marker written)
- Modify: `schemas/README.md` (document ARTIFACT in the Layout block)

**Interfaces:**
- Consumes: `buildSchemas`'s `downloadURL *url.URL` / `localFile` parameters, `writeMarker(dir, name, value string) error`.
- Produces: tracked file `schemas/ARTIFACT` containing the download URL (or `local <basename>` for `-file` runs). Task 2's workflow reads it.

- [ ] **Step 1: Write the failing test**

Add to `cmd/fields/extract_test.go` (unit-level, using the existing fixture helpers — extend `runExtraction`'s schemasDir usage or add a focused test):

```go
func TestBuildSchemasWritesArtifactMarker(t *testing.T) {
	restoreMin := minFieldFiles
	minFieldFiles = 1
	t.Cleanup(func() { minFieldFiles = restoreMin })

	deb := buildDeb(t, map[string][]byte{
		"./usr/lib/unifi/lib/ace.jar": buildZip(t, func() map[string][]byte {
			m := defsJarFiles()
			m["product.properties"] = []byte(productProperties)
			return m
		}()),
	})

	schemasDir := t.TempDir()
	customDir := filepath.Join(t.TempDir(), "custom")
	require.NoError(t, os.MkdirAll(customDir, 0o755))

	_, err := buildSchemas(schemasDir,
		filepath.Join(schemasDir, "fields"),
		filepath.Join(schemasDir, "metadata"),
		customDir,
		writeTempArtifact(t, deb), nil, nil)
	require.NoError(t, err)

	marker, err := os.ReadFile(filepath.Join(schemasDir, "ARTIFACT"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(marker), "local "), "marker = %q", marker)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./cmd/fields/ -run TestBuildSchemasWritesArtifactMarker -v`
Expected: FAIL — no `schemas/ARTIFACT` file.

- [ ] **Step 3: Implement**

In `buildSchemas` (cmd/fields/main.go), next to the VERSION marker write:

```go
	artifact := "local " + filepath.Base(artifactPath)
	if downloadURL != nil {
		artifact = downloadURL.String()
	}
	if err := writeMarker(schemasDir, "ARTIFACT", artifact); err != nil {
		return nil, err
	}
```

Also add `"ARTIFACT"` to the marker-invalidation loop at the top of the extraction block (`for _, marker := range []string{"VERSION", "SOURCE", "ARTIFACT"}`).

Document in `schemas/README.md`'s Layout block:

```
ARTIFACT   URL of the controller artifact the cache came from (tracked);
           CI uses it to boot the matching controller build
```

- [ ] **Step 4: Run tests, regenerate, commit**

```bash
go test ./cmd/fields/ -v -run 'TestBuildSchemas|TestExtract' && go vet ./cmd/fields/
go generate ./...   # rebuilds because ARTIFACT is missing -> marker mismatch is intentional here
git status --short  # expect: new schemas/ARTIFACT (tracked), nothing else
git add cmd/fields/main.go cmd/fields/extract_test.go schemas/README.md schemas/ARTIFACT
git commit -m "fields: record the source artifact URL as a tracked marker

CI needs to boot the exact controller build the schemas came from; the
download URL is the missing fact. Written alongside VERSION/SOURCE and
invalidated with them.

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

Note: `go generate` will re-download once (marker invalidation includes ARTIFACT). That is expected and proves the path; if the environment cannot download, run `go run ./cmd/fields -file <artifact> -output-dir=unifi` instead and note the `local ...` marker will be replaced by the nightly run's URL form.

---

### Task 2: integration workflow

**Files:**
- Create: `.github/workflows/integration.yaml`

**Interfaces:**
- Consumes: `schemas/ARTIFACT` (Task 1), `TestIntegrationControllerBoots` + `TestIntegrationV2Drift` (Phase 1), env contract `UNIFI_TEST_PKGURL`.
- Produces: a check named `Integration` for branch protection (Task 3).

- [ ] **Step 1: Write the workflow**

```yaml
---
name: Integration

# Boots a disposable simulation-mode controller (jacobalberty/unifi) and
# runs the integration-tagged suite: harness smoke test + v2 schema drift
# probe. PRs touching schema inputs/outputs get it as a gate; the nightly
# run catches upstream drift and image rot independent of PRs.
on:
  pull_request:
    paths:
      - 'schemas/**'
      - 'overrides/**'
      - 'unifi/**'
      - 'cmd/fields/**'
      - 'internal/testenv/**'
      - '.github/workflows/integration.yaml'
  schedule:
    - cron: 30 1 * * *
  workflow_dispatch: {}

permissions:
  contents: read

concurrency:
  group: integration-${{ github.ref }}
  cancel-in-progress: true

jobs:
  integration:
    name: Integration
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - name: Checkout
        uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0

      - name: Setup Go
        uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0
        with:
          go-version-file: 'go.mod'

      # Boot the exact build the schemas came from when it is a .deb; the
      # UniFi OS Server installer cannot be installed into this image, so
      # those artifacts fall back to the image's default controller.
      - name: Pin controller build
        id: pin
        run: |
          artifact="$(cat schemas/ARTIFACT 2>/dev/null || true)"
          case "${artifact}" in
            https://*.deb)
              echo "pkgurl=${artifact}" >> "$GITHUB_OUTPUT"
              echo "Pinning controller to ${artifact}"
              ;;
            *)
              echo "pkgurl=" >> "$GITHUB_OUTPUT"
              echo "No pinnable .deb artifact (${artifact:-none}); using image default"
              ;;
          esac

      - name: Run integration suite
        env:
          UNIFI_TEST_PKGURL: ${{ steps.pin.outputs.pkgurl }}
        run: >-
          go test -tags integration -timeout 25m -v
          ./internal/testenv/ ./cmd/fields/
          -run 'TestIntegration'
```

- [ ] **Step 2: Lint**

Run: `uvx --from yamllint yamllint -c .yamllint .github/workflows/integration.yaml`
Expected: zero errors (comment-spacing warnings matching existing files are acceptable).

- [ ] **Step 3: Commit, push the branch, verify the run**

```bash
git add .github/workflows/integration.yaml
git commit -m "ci: run the controller integration suite on schema PRs

Boots the simulation-mode controller pinned to schemas/ARTIFACT when it
is a .deb, and runs the harness smoke test plus the v2 drift probe.
Nightly schedule catches upstream drift independent of PRs.

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

After pushing, open a draft PR (or use workflow_dispatch) and confirm the `Integration` check runs green end-to-end on GitHub's runners before Task 3. Docker on ubuntu-latest needs no setup. If the controller image pull or boot is flaky, fix the harness timeout/wait rather than papering over with retries.

- [ ] **Step 4: Confirm the drift probe's failure mode in CI**

Temporarily (in the PR only) remove one field from `overrides/resources/TrafficRoute.json`, push, and confirm `Integration` goes red with the LiveOnly message; revert the field and confirm green. This proves the gate detects drift in the environment where it matters. Do not merge the temporary commit — rebase it away.

---

### Task 3: make it gate auto-merge

**Files:**
- Modify: `.github/workflows/generate.yaml` (comment only — behavior comes from branch protection)
- Modify: `docs/superpowers/specs/2026-07-17-schema-fetcher-uos-design.md` (Automation section)

**Interfaces:**
- Consumes: the `Integration` check (Task 2); the repo's existing auto-merge flow.
- Produces: documented gate: schema auto-merge waits for Test AND Integration.

- [ ] **Step 1: Update the generate.yaml settings comment**

Extend the `SCHEMA_UPDATE_TOKEN` comment block's NOTE to name both required checks:

```yaml
      # NOTE: auto-merge only waits for checks that branch protection marks
      # required (the Test and Integration jobs); without that protection
      # rule it merges immediately, ungated. Breaking API changes skip
      # auto-merge entirely (see the Flag breaking change step below).
```

- [ ] **Step 2: Update the design doc's Automation bullet**

In the design doc's Automation section, extend the repo-settings line to:

```markdown
- Full hands-off needs repo settings: `SCHEMA_UPDATE_TOKEN` (PAT/App,
  contents+pull-requests write), allow auto-merge, and required checks
  `Test` + `Integration` on main.
```

- [ ] **Step 3: Flip the repo settings (manual, repo admin)**

On the fork: Settings → Branches → protection rule for `main` → require status checks `Test` and `Integration`; Settings → General → allow auto-merge. Record in the PR description that this was done (workflow files cannot do it).

- [ ] **Step 4: Lint, commit**

```bash
uvx --from yamllint yamllint -c .yamllint .github/workflows/
git add .github/workflows/generate.yaml docs/superpowers/specs/2026-07-17-schema-fetcher-uos-design.md
git commit -m "ci: document the integration check as an auto-merge gate

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Self-review notes

- The gate is fail-visible: probe subtests use `t.Skip` only for genuinely absent data (404/empty), while docker/image failures fail the job — combined with the required-check setting, a broken harness blocks auto-merge rather than letting it through.
- PKGURL pinning degrades gracefully to the default image for UOS-installer artifacts, with the choice logged in the job output.
- Phase 2's mutating probe is intentionally NOT in the PR gate (runtime and flakiness budget); if wanted later, add `./unifi/ -run TestIntegrationNetworkFieldProbe` to the nightly schedule only.
