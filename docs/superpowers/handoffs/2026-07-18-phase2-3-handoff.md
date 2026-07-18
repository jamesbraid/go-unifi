# Handoff: phases 2/3 of controller testing (after phase 1 has landed)

**For:** a fresh agent executing phase 2 (encoder field verification) and/or
phase 3 (integration CI gate), once phase 1 (the `internal/testenv` harness +
v2 drift probe) is merged.

**Read first:** `2026-07-18-controller-testing-handoff.md` in this directory
— it carries the environment facts (simulation mode seeds `admin`/`admin`;
`PKGURL` pins exact `.deb` builds; classic API paths without
`/proxy/network`; deliberate deviations from the provider's compose wiring).
This document only adds what a post-phase-1 agent needs.

## Step minus one: wait until phase 1 has actually landed

Do NOT start phase 2/3 against unlanded interfaces. The landing signal is
the phase 1 code being present on the branch you'll build on:

```sh
git fetch origin
# landed when BOTH exist on the target branch (check controller-testing
# first, then main — phase 1 may merge either way):
git cat-file -e origin/controller-testing:internal/testenv/controller.go
git cat-file -e origin/controller-testing:internal/testenv/testenv_integration_test.go
```

Your worktree branches off `controller-testing` but was created BEFORE
phase 1 landed there — once the signal fires, rebase onto the updated ref
(`git fetch origin && git rebase origin/controller-testing`, or
`origin/main` if that's where it landed) before doing step zero.

If not landed yet: wait by polling, self-paced at ~20–30 minute intervals
(ScheduleWakeup / a self-paced loop — long intervals, this is a
hours-not-seconds wait). Each wake: fetch, re-check, report one line of
status. While waiting, the only useful work is reading the two plans and
this handoff — do not pre-implement against assumed interfaces, and do not
"help" phase 1 along in its branch. If phase 1 hasn't landed after ~24
hours of polling, stop and report instead of waiting forever — its branch
may have stalled or been renamed (in that case, say what you looked for
and where).

## Step zero: reconcile the plans with phase 1 as built

The phase 2/3 plans were written against phase 1's *planned* interfaces.
Before executing anything, verify what actually landed:

1. Read `internal/testenv/` — confirm the real signatures of `Start`,
   `Controller`, `NewSession`, `GetJSON` (plans assume:
   `Start(ctx, t) *Controller`, `Controller{BaseURL, Username, Password,
   Site}`, `GetJSON(ctx, path) (body any, status int, err error)` with
   non-2xx-is-not-an-error semantics). If phase 1 deviated, adapt the plan
   code to reality — reality wins, and note the deltas in your commits.
2. Run the unit suite (`go test ./internal/testenv/ ./cmd/fields/`) and,
   where docker exists, the integration suite once:
   `go test -tags integration ./internal/testenv/ ./cmd/fields/ -run TestIntegration -v -timeout 20m`.
   Record which drift-probe subtests SKIP (empty/404) — that is the
   empirical map of what the simulation controller serves, and phase 2's
   probe rides the same harness.
3. Check the drift probe's resource table covers all NINE files now in
   `overrides/resources/` (ApGroups and NetworkMembersGroup were imported
   after the plans were first drafted; the phase 1 plan's table was updated,
   but verify the implementation matched).

## Which phase first

They are independent of each other; both depend only on phase 1.
**Recommended: phase 3 first** — it is small, and once the `Integration`
check gates schema auto-merge, every later change (including phase 2's
encoder wiring) runs under it. Phase 2 is the longer burn-down and produces
its value incrementally.

## Phase 3 notes beyond the plan
(`docs/superpowers/plans/2026-07-18-phase3-integration-ci-gate.md`)

- The `schemas/ARTIFACT` marker task touches `buildSchemas` in
  `cmd/fields/main.go` — that function also invalidates markers before
  rewriting the cache; keep ARTIFACT in that invalidation list (the plan
  says so; don't lose it when reconciling).
- Current SOURCE format is `<product> <full-version> <firmware-record-id>`
  (e.g. `unifi-controller v10.4.57+atag-10.4.57-34628 86432683-...`); the
  ARTIFACT marker is separate and holds the bare download URL.
- Repo-settings flip (required checks `Test` + `Integration`, allow
  auto-merge) is manual and fork-admin only — surface it in the PR
  description; do not silently skip it, it is what makes the gate real.
- If the snapshot's artifact is ever a UniFi OS Server installer rather
  than a `.deb`, the pin step falls back to the image default — that
  fallback must stay visible in the job log, never silent.

## Phase 2 notes beyond the plan
(`docs/superpowers/plans/2026-07-18-phase2-encoder-field-verification.md`)

- The candidate list source of truth is the allowlist in
  `unifi/network_encode_coverage_test.go` — entries commented
  `TODO: possibly a real gap` (~39 at handoff time; recount, phase 1 or
  other work may have touched it). The plan's completeness test pins the
  table to that list bidirectionally.
- HARD RULE: the mutating probe runs only against the disposable container
  — it must skip when `UNIFI_TEST_URL` is set. Never mutate the homelab
  controller.
- Simulation-mode caveat for interpreting results: STRIPPED there is
  evidence, not proof — device-dependent fields may persist on real
  hardware. Record classifications in the allowlist comments with the
  probe date and controller version rather than deleting unverified
  entries.
- The value-flow test (`TestNetworkEncoderValueFlow`) will catch any
  cross-sourced field the moment you wire it — if it fires, you copied
  from the wrong struct field, not a test bug (this exact class of bug
  existed once: corporate `dhcp_relay_servers` sourced from
  `RemoteVPNSubnets`).

## Definition of done (either phase)

- `gofmt` clean, `go vet ./...`, `go test ./...` (docker-free) green.
- Integration suite green somewhere real (locally with docker, or the
  phase 3 CI job).
- yamllint zero errors on any touched workflow.
- Kernel-style commits; nothing extracted from Ubiquiti software committed.
- Update `2026-07-18-controller-testing-handoff.md`'s open-questions
  section with what the run answered (which endpoints simulation mode
  serves; whether PKGURL 10.4.57 installs cleanly).
