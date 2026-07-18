# Handoff: live controller testing for go-unifi

**For:** a fresh session/worktree implementing the demo-controller "live
oracle" (harness, drift probe, field verification, CI gate).

**Branch strategy:** branch `controller-testing` forks from
`emdash/schema-fetcher-fable-xl0cf` (this work builds on that branch's
`overrides/` layout, markers, and coverage tests — do NOT fork from main).
If the schema-fetcher branch merges first, rebase onto main afterwards.

## What to build (three plans, execute in order)

1. `docs/superpowers/plans/2026-07-18-phase1-demo-controller-harness-v2-drift.md`
   — `internal/testenv` harness (testcontainers, simulation mode) + v2
   schema drift probe. Independent value: the drift signal for the
   hand-written `overrides/resources/*.json`.
2. `docs/superpowers/plans/2026-07-18-phase2-encoder-field-verification.md`
   — raw CRUD probe classifying the ~39 unverified encoder allowlist
   fields, then wiring the persisted ones. Requires phase 1.
3. `docs/superpowers/plans/2026-07-18-phase3-integration-ci-gate.md`
   — `schemas/ARTIFACT` marker, `integration.yaml` workflow, required-check
   gating of schema auto-merge. Requires phase 1; independent of phase 2.

Use superpowers:subagent-driven-development (or executing-plans) per the
plan headers.

## Facts established this session (trust these; re-verify only if stale)

- **Simulation mode**: `is_simulation=true` in `system.properties` (via an
  init script in `/unifi/init.d/`) makes the controller seed an
  `admin`/`admin` account, site `default`, and demo devices — no setup
  wizard. Source of truth: `ubiquiti-community/unifi-api`
  `scripts/init.d/demo-mode` and `scripts/crawler/setup.mjs` (which
  documents the seeding), mirrored from
  `terraform-provider-unifi/unifi/provider_test.go`.
- **Image**: `jacobalberty/unifi` — versioned tags reach `v10.0.162`
  (10.4.x not tagged yet); env `PKGURL` installs an arbitrary UniFi Network
  `.deb` at container start (that's how we pin the exact schema version;
  the `.deb` URL for the current snapshot is recorded by the phase 3
  ARTIFACT marker, and today is
  `https://fw-download.ubnt.com/data/unifi-controller/fa30-debian-10.4.57-86432683-a50a-4fd9-8e7b-21180c41611b.deb`).
- **Classic controller API** (this image is NOT UniFi OS): base
  `https://host:8443`, login `POST /api/login` `{"username","password"}`
  with cookie session, v1 REST at `/api/s/<site>/rest/...`, v2 at
  `/v2/api/site/<site>/...`, self-signed TLS. (A UniFi-OS console like the
  homelab UDM prefixes `/proxy/network` and logs in via
  `/api/auth/login` — the harness does not need that, but
  `UNIFI_TEST_URL` users might.)
- **Provider wiring** (inspiration, deliberately not copied): it uses the
  testcontainers *compose module* (drags in docker/compose v2), bind-mounts
  the init script (their own comment records Podman breakage), disables
  Ryuk, polls login (`waitForUniFiAPI`), then exports `UNIFI_*` env vars.
  Keep: login-poll readiness, admin/admin, skip-container escape hatch.
  Improve: plain `GenericContainer`, embedded init script via
  `ContainerFile`, return a struct instead of env mutation.
- **Homelab alternative**: read-only checks can run against the UDM Pro Max
  (`oxcart.local`, API key in 1Password `op://Homelab/unifi.api-token...`,
  `/proxy/network` paths) — used earlier to verify `querier_addresses`
  object shape and `vpn_binding_mode`/`mss_clamp` placement. NEVER run
  mutating probes there; phase 2 skips when `UNIFI_TEST_URL` is set.

## Context from the parent branch you'll rely on

- `overrides/resources/*.json` — 7 hand-written v2 schemas (the drift
  probe's subject). `overrides/fields.toml` — declarative field overrides.
- `unifi/network_encode_coverage_test.go` — presence allowlist whose
  `TODO: possibly a real gap` entries (~39) are phase 2's target;
  `TestNetworkEncoderValueFlow` guards against cross-sourced values.
- `schemas/VERSION|SOURCE` — tracked markers; SOURCE format is
  `<product> <full-version> <firmware-record-id>`. Phase 3 adds ARTIFACT.
- Repo policy: nothing extracted from Ubiquiti software is committed;
  kernel-style commit messages; integration tests behind
  `//go:build integration`.

## Phase 4 (exploratory, not yet planned): crawler-assisted v2 schema population

`ubiquiti-community/unifi-api` is NOT a better source for the v2 JSON
definitions themselves — its `cmd/fields/custom/` carries the same
hand-rolled files as our `overrides/resources/` (shared lineage), plus three
we ship as hand-written Go instead (`ApGroups.json`,
`NetworkMembersGroup.json`, `PowerSupervisor.json` — evaluating those as
overrides to replace `unifi/ap_group.go` etc. is a quick win worth its own
small task). What IS leverageable is `scripts/crawler/`: a Playwright-style
crawler that drives the controller SPA exhaustively in simulation mode
(MUTATE mode creates/updates/deletes through the UI, so payloads are always
*valid* — the part our raw probes must otherwise guess) and exports a HAR of
every XHR plus a deduplicated `api-endpoints.json`. Feeding those HAR
request/response bodies into a derivation step could mechanically populate
and refresh `overrides/resources/*.json` field sets (types and enums by
sampling; validation regexes still need hands or frontend-bundle mining —
the settings chunks carry zod form schemas, the richest-but-brittle
validation source, proven when verifying `querier_addresses`).

Sequencing: do this only after phase 1 (the drift probe tells you *when*
population is needed) and consider a cheap intermediate first — extending
the drift probe with a suggest mode that emits proposed JSON entries for
LiveOnly fields with validation inferred from observed values. Run the
crawler as-is from their repo against our harness rather than porting it;
upstream fixes there (same org).

## Open questions the implementation will answer empirically

- Which v2 endpoints the simulation controller actually serves (expect
  zones/policies present via ZBF defaults; trafficroutes/NAT possibly
  empty → probe skips; OSPF/BGP may 404 without a routing-capable
  gateway).
- Whether PKGURL-installing the 10.4.57 deb works in the image's current
  base (if mongo/java constraints bite, fall back to the newest working
  tag and record it).
- Exact v2 list paths (phase 1 task verifies against the SDK's own client
  code before enabling each probe).
