# Schema fetcher update for UniFi Network 10.x / UniFi OS Server

Design as built (revised 2026-07-18 after two licensing reviews and a full
PR review; the original draft committed extracted schema files — that was
reversed, see Licensing posture).

## Problem

`cmd/fields` was pinned below 10.0.0 (`maxVersion` filter) because UniFi
Network 10.x repackaged the controller: the API field definitions moved out
of `ace.jar`'s root into `internal-dependencies.jar`. The repo was stuck
generating from 9.5.21 while 10.4.57 was current. The pipeline also needed
to become safely automatable: nightly regeneration, gated auto-merge, and
automatic releases.

## Findings (verified 2026-07-17)

Three packaging layouts exist for the same app:

| Source | Layout | Where the JSON defs live |
|---|---|---|
| deb ≤ 9.x | fat-ish ace.jar | `api/fields/*.json` at ace.jar root |
| deb 10.x (still published, 130 MB) | thin `ace.jar` launcher | separate `usr/lib/unifi/lib/internal/internal-dependencies.jar` |
| UniFi OS Server installer (~880 MB) | ELF stub + appended zip → `image.tar` (OCI) → layer → Spring Boot fat `ace.jar` | `BOOT-INF/lib/internal-dependencies.jar` |

- `https://fw-update.ubnt.com/api/firmware-latest` lists **both**
  `product=unifi-controller platform=debian` and
  `product=unifi-os-server platform=linux-x64`; both carried Network
  10.4.57 at review time, extracted byte-identical.
- The Network app version is in `product.properties` (thin jar root, or
  `BOOT-INF/classes/product.properties` in the fat jar).
- Legacy explicit-version URL `https://dl.ui.com/unifi/<v>/unifi_sysvinit_all.deb`
  still works for 10.x.

## Design

### Extraction

One source-agnostic pipeline in `cmd/fields`; input is a deb or a UniFi OS
Server installer (downloaded, or local via `-file`), sniffed from magic
bytes. Everything streams through temp files; jar finds accumulate across
OCI layers. Hardening for unattended use: schema text is sanitized before
templating (wire names constrained, line breaks stripped — schema text
becomes Go source CI compiles), near-empty extractions are rejected, the
firmware API's status is checked, HTTP is retried, markers are invalidated
before the cache is rewritten so a failed extraction can't be mistaken for
a current one.

### Transient schema cache (nothing of Ubiquiti's committed)

Extraction lands in a **gitignored** cache:

```
schemas/
  VERSION    Network app version of the cache        (tracked)
  SOURCE     release identity incl. build metadata + firmware record id
             (tracked; drives skip logic, CI cache keys, auto-release)
  fields/    extracted validators + Setting splits + overlay copies
             + .extracted manifest                    (gitignored)
  metadata/  sensitive_metadata.json                  (gitignored)
```

Only the generator's own outputs (Go code, `specification.json`) and the
tiny factual markers are committed. CI restores the cache via actions/cache
keyed on SOURCE (restore/save split so a fresh extraction saves under the
new key). Generated files whose schema disappears upstream are deleted at
generation time, with an actionable error when hand-written companions
would be orphaned.

### Override layers

1. `overrides/resources/*.json` — project-authored whole-resource schemas
   for v2 endpoints the jar doesn't describe; synced into the cache with
   stale-overlay removal (via the `.extracted` manifest).
2. `overrides/fields.toml` — declarative REST paths, per-field shape pins,
   renames, and `add = true` compat fields; applied after the
   FieldProcessor, add-if-missing so upstream's definition wins.
3. `FieldProcessor` cases in `cmd/fields/main.go` — conditional logic only.
4. Hand-written `.go` files in `unifi/` — client methods and custom
   encoders; a go/parser-based guard fails generation when a schema update
   starts defining a type a hand-written file already declares.

The hand-written Network encoder is held to the generated struct by two
reflection tests: coverage (every generated wire name emitted or
allowlisted) and value-flow (each emitted key must carry its own field's
value).

### Sensitive fields

The spec marks attributes sensitive from the union of the `x_` wire-prefix
convention and `sensitive_metadata.json` entries filtered to secret-looking
names — the metadata is an anonymization list, so names/hostnames/usernames
stay visible while real secrets (`lte_password`, `lte_sim_pin`, ...) are
caught even without the prefix.

## Automation

- **generate.yaml** (nightly): `-print-latest` vs the SOURCE marker; exits
  early when current. On change: regenerate, apidiff vs the latest tag
  (baseline via `git archive` — gorelease can't resolve this fork's tags),
  open a PR. Non-breaking → auto-merge; breaking → PR labelled
  `breaking-schema-change` with the diff in the body, no auto-merge.
- **auto-release.yaml**: on pushes touching the markers, tags the next
  minor (concurrency-grouped, tag-push retry) and calls release.yaml via
  `workflow_call`; refuses to tag when the commit is API-breaking (a major
  needs a manual /v2 decision).
- **ci.yaml**: Test is blocking; the generated-drift check is blocking
  unless a new upstream release appeared mid-PR.
- Full hands-off needs repo settings: `SCHEMA_UPDATE_TOKEN` (PAT/App,
  contents+pull-requests write), allow auto-merge, and required checks
  `Test` + `Integration` on main.
  - Caveat: `integration.yaml` is path-filtered; a required check that a
    PR's paths never trigger hangs forever on "Expected — Integration".
    The filter covers go.mod/go.sum and all workflow files, so
    dependabot PRs are safe, but docs-only PRs are not on their own —
    `integration-noop.yaml` covers them: its `paths-ignore` is the exact
    inverse of `integration.yaml`'s `paths` filter, and its job's
    display name is also `Integration`, so exactly one of the two
    workflows always reports that check name on any given PR. Keep the
    two filters in sync when editing either — a same-name job that
    stops running on some PR shape reopens the original hang.

## Licensing posture

Nothing extracted from Ubiquiti's software is committed or redistributed —
the repo publishes only its own code and generated interoperable client
(the *Google v. Oracle* interop pattern, matching upstream paultyng's
six-year precedent). The extracted set is limited to what the generator
consumes; expressive content (event message templates) is never extracted.
The README carries a nominative-use trademark disclaimer. Residual accepted
risks: nightly CI downloads Ubiquiti artifacts (contract formation for a
headless download is doubtful; flip to workflow_dispatch + `-file` for
maximal caution), and validation regexes survive in generated code (low
risk, functionally useful).

## Out of scope

- Full OCI manifest/whiteout semantics and EOCD-based zip location in the
  installer scanner (current images are single-rootfs-layer; failures are
  loud).
- v2 endpoint drift detection (overrides/resources/*.json are maintained by
  hand).
- The 39 Network encoder allowlist entries marked as possible pre-existing
  gaps (fields the encoder has never sent) — each needs a live-controller
  check before wiring.
