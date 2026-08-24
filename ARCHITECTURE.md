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
        │  go generate ./...  (cmd/fields, offline)
        ▼
unifi/*.generated.go + specification.json
        │  cmd/rebuild-manifest
        ▼
schemas/GENERATED_SHA256 — digest of everything generated
```

`cmd/schema-capture` downloads or reads one controller artifact, stores it
in a content-addressed directory (`GO_UNIFI_CONTENT_STORE`), and writes
`schemas/capture.lock.json` naming its exact bytes. `cmd/fields` generates
only from a verified lock: it re-hashes the artifact, the extraction rules,
`overrides/`, and itself, and refuses to run on any drift. Generation never
downloads and never looks up "latest".

The artifact comes from Ubiquiti's public CDN but cannot be redistributed,
so git carries the lock, not the bytes. Anyone can re-run capture against
the URL in `schemas/ARTIFACT` and verify the digest. CI verifies the lock
without the bytes (`-verify-lock-only`) and checks the committed
generated output against its accepted digest. The `rebuild` workflow
proves the whole pipeline reproduces from public inputs alone: on a stock
runner, it fetches the locked artifact from its public URL, regenerates
from scratch, and diffs the result against `schemas/GENERATED_SHA256` and
the working tree.

## Overrides and measured ownership

The extracted schema says what fields exist. It does not say how the
controller treats them. `overrides/` carries every deviation from the
schema, each tied to evidence:

- `overrides/fields.toml` — per-field pins: serialization shape, ownership
  (`owns = [...]` with the build it was measured against), fields upstream
  dropped that the controller still honors. Ownership entries are produced
  by an integration test that writes the object twice and diffs what comes
  back. The test renders the exact TOML block to paste, and fails when the
  controller stops agreeing with a pin.
- `overrides/resources/*.json` — hand-written schemas for v2 API resources
  the field spec does not describe. The drift probe
  (`go test -tags integration ./cmd/fields -run TestIntegrationV2Drift`)
  compares them against a live controller and fails on live-only fields.

Both are generator inputs, digested into the lock. See
`overrides/README.md` and COMPATIBILITY.md for the retention policy.

## CI model

One workflow tree in `.github/workflows/`, no required secrets, every
input public: the controller CDN, the sim controller images
(`ghcr.io/jamesbraid/unifi-network`, built from
[jamesbraid/unifi-containers](https://github.com/jamesbraid/unifi-containers)),
and the device emulator
([jamesbraid/unifi-emu](https://github.com/jamesbraid/unifi-emu)). The sim
image's `admin`/`admin` credentials are part of the image contract, not
secrets. Any fork can run the whole pipeline, and the same files run on
any Actions-compatible forge. The PR-opening plumbing (capture) is
GitHub-specific and wants a schema-bot app or PAT so its PRs trigger CI;
the `github.server_url` guards pin tag and release publishing to the
public publish point.

| Workflow | Trigger | Job |
| --- | --- | --- |
| `ci` | push, PR | unit tests, lock verification without the artifact |
| `integration` | PR, nightly, dispatch | sim controller + emulated devices, full suite |
| `capture` | manual dispatch | fetch artifact, propose lock, regenerate, open PR |
| `auto-release` | merge touching schemas/{VERSION,SOURCE,ARTIFACT} | apidiff, then tag the next minor |
| `release` | tag push | goreleaser release notes naming the controller train |
| `rebuild` | generator input or output changes, dispatch | from-scratch regenerate-and-diff on a stock runner |
| `dependabot` | dependabot PRs | auto-approve and auto-merge pinned bumps |

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
4. Re-measure ownership pins wherever the controller changed. The
   ownership integration test
   (`unifi/preference_ownership_integration_test.go`) renders the
   replacement TOML to paste.
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
