# Architecture

This SDK targets UniFi Network's internal APIs, which change without a
compatibility contract. Everything here exists to make that tractable: the
SDK is generated from one exact controller build, every input to that
generation is pinned by digest, and every claim about controller behavior
traces to a measurement against a live controller.

## Generation pipeline

```
controller artifact (.deb from Ubiquiti's CDN)
        │  cmd/schema-capture — retain by SHA-256, propose the lock
        ▼
schemas/capture.lock.json — artifact digest, extracted-schema digests,
        │                   generator-input digests
        ▼
        │  go generate ./...  (cmd/fields, then cmd/wirecontract; offline)
        ▼
unifi/*.generated.go
wirecontract/wire_contract.json — the published wire contract
schemas/GENERATED_SHA256 — digest of everything generated
```

`cmd/schema-capture` downloads or reads one controller artifact, stores it
in a content-addressed directory (`GO_UNIFI_CONTENT_STORE`), and writes
`schemas/capture.lock.json` naming its exact bytes. `cmd/fields` generates
only from those bytes: it refuses drifted or missing bytes, never
downloads, and never looks up "latest". A successful run re-stamps the
lock's input digests and `schemas/GENERATED_SHA256`, so `go generate`
leaves the tree consistent. A dependency bump touches nothing but
go.mod/go.sum; the `rebuild` workflow is what proves dep-caused drift.

`cmd/wirecontract` runs second, over the tree `cmd/fields` just wrote, and
records `wirecontract/wire_contract.json`: every wire object, its fields in
declaration order with their Go declarations and omitempty, the generated
constraint tables re-serialized, and the measured behaviour from
`schemas/behavior.json` rekeyed onto the same type keys. It runs second
because the constraint tables are read as Go values rather than re-derived,
which is what keeps the artifact and the tables consumers already use from
drifting apart. Package `wirecontract` embeds the result, so a consumer
imports it instead of walking this SDK's source.

The artifact comes from Ubiquiti's public CDN but cannot be redistributed,
so git carries the lock, not the bytes. Anyone can re-run capture against
the URL in `schemas/ARTIFACT` and verify the digest. CI verifies the lock
without the bytes (`-verify-lock-only`) and independently recomputes the
output digest (`cmd/rebuild-manifest`) against the accepted one. The
`rebuild` workflow proves the whole pipeline reproduces from public
inputs alone: on a stock runner, it fetches the locked artifact from its
public URL, regenerates from scratch, and diffs the result against the
committed tree.

## Overrides and measured ownership

The extracted schema says what fields exist. It does not say how the
controller treats them. `overrides/` carries every deviation from the
schema, each tied to evidence:

- `overrides/fields.toml` — per-field pins: serialization shape and fields
  upstream dropped that the controller still honors. Measured ownership
  lives in the `ownership` and `uos_pins` sections of
  `schemas/behavior.json`, written by integration tests that create the
  object twice and diff what comes back; the tests fail when the controller
  stops agreeing with the record.
- `overrides/resources/*.json` — hand-written schemas for v2 API resources
  the field spec does not describe. The drift probe
  (`go test -tags integration ./cmd/fields -run TestIntegrationV2Drift`)
  compares them against a live controller and fails on live-only fields.

Both are generator inputs, digested into the lock, and so is
`schemas/behavior.json`: a re-measure moves the lock's input digest
exactly like an override edit does, and lands with its regenerate. Its
`writes` section decides real client code — the verb a create issues, the
path it writes to, and which fields lose `omitempty` because the
controller refuses a create without them — so a measured endpoint that
does not follow the usual shape is corrected by re-measuring it, not by
hand-editing the generated file. See `overrides/README.md` and
COMPATIBILITY.md for the retention policy.

## CI model

Two CI systems, one per forge. A `.forgejo/workflows` directory would
replace `.github/workflows` rather than add to it, so instead every job
under `.github/workflows/` carries a `github.server_url` guard naming
the one forge it belongs to, and `.woodpecker/checks.yml` runs the same
gates on the canonical forge. Nothing here is portable to an arbitrary
Actions-compatible forge: each job names its own.

Every input the gates read is public: the controller CDN, the sim
controller images (`ghcr.io/jamesbraid/unifi-network`, built from
[jamesbraid/unifi-containers](https://github.com/jamesbraid/unifi-containers)),
and the device emulator
([jamesbraid/unifi-emu](https://github.com/jamesbraid/unifi-emu)). The sim
image's `admin`/`admin` credentials are part of the image contract, not
secrets, so any fork can run the gates. Two workflows do need a
credential: `capture`, whose PR-opening plumbing is GitHub-specific and
wants a schema-bot app or PAT so its PRs trigger CI, and `renovate`,
which fails loudly rather than quietly skipping when `RENOVATE_TOKEN` is
missing.

| Workflow | Trigger | Job |
| --- | --- | --- |
| `ci` | push, PR | unit tests, lock verification without the artifact |
| `integration` | PR, nightly, dispatch | sim controller + emulated devices, full suite |
| `capture` | manual dispatch | fetch artifact, propose lock, regenerate, open PR |
| `auto-release` | merge touching schemas/{VERSION,SOURCE,ARTIFACT} | apidiff, then tag the next minor |
| `release` | tag push | goreleaser release notes naming the controller train |
| `rebuild` | generator input or output changes, dispatch | from-scratch regenerate-and-diff on a stock runner |
| `renovate` | nightly, dispatch | Renovate against the canonical forge; forge-only |
| `.woodpecker/checks.yml` | PR, push to main, manual | the canonical forge's copy of the `ci` gates |

## Releasing for a new controller version

1. [unifi-containers](https://github.com/jamesbraid/unifi-containers)
   publishes the `<version>-sim` image (the one prerequisite outside this
   repo).
2. Run the **capture** workflow with the artifact URL from Ubiquiti's
   release notes. It proposes the new lock, regenerates, and opens the
   schema PR with an apidiff summary — the regeneration diff is the
   discovered v1 surface.
3. The **integration** suite and the drift probe
   (`go test -tags integration ./cmd/fields -run TestIntegrationV2Drift`)
   run against the new controller on the schema PR. Drift-probe failures
   are the discovered v2 fields: add them to `overrides/resources/` and
   regenerate.
4. Re-measure ownership wherever the controller changed: run the ownership
   integration test (`unifi/preference_ownership_integration_test.go`) with
   `BEHAVIOR_WRITE=1` on the standalone harness, then on the UniFi OS
   harness (`UNIFI_TEST_HARNESS=uos`) for the `uos_pins` half, and review
   the `schemas/behavior.json` diff it writes.
5. Merge. **auto-release** tags the next minor unless apidiff found a
   break, in which case the version bump is a human decision.

Every step is reproducible from public inputs: the same capture,
integration, and rebuild commands run from a laptop with nothing but this
repository, Docker, and the artifact URL.

## Versioning

Each release names the one controller train it was generated and tested
against. Semantic versioning tracks the Go API. Controller-forced breaking
changes ship in documented minor releases rather than a `/vN` bump per
train. Pin an exact version for module-strict guarantees. COMPATIBILITY.md
is the full policy.
