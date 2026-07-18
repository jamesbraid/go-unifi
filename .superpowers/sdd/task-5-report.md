# Task 5 implementation report

Baseline: `05284f644477f7da623e8a9049f3d7807d20c9e4`

## RED evidence

The first focused run failed at compile time because the new command contract did not exist yet:

```text
cmd/fields/input_scan_test.go:16:21: undefined: ScanExtractedInputs
cmd/fields/run_test.go:18:10: undefined: runWithDeps
cmd/fields/run_test.go:28:24: undefined: HashGeneratedFiles
cmd/fields/run_test.go:...: undefined: publishGeneratedTree
FAIL github.com/ubiquiti-community/go-unifi/cmd/fields [build failed]
```

Command: `go test ./cmd/fields -run 'Test(ScanExtractedInputs|Run|PublishGeneratedTree|SelectorFromValues)' -count=1`

## Implementation

- Added `Run(ctx,args,stdout,stderr)` backed by a private `flag.FlagSet` and immutable-by-convention injected `runDeps`.
- Added `selectorFromValues`, strict selector/generation parsing, selector-free terminal verification modes, and canonical-output enforcement for UOS provenance.
- Routed UOS selectors through resolve, verified materialization, bounded extraction, atomic local snapshot publication, input scanning, reviewed sensitivity, staged rendering, validation, and a generated-output transaction.
- Kept the local snapshot after later policy review failures while leaving committed outputs byte-for-byte unchanged.
- Changed legacy orchestration to extract from the already-materialized local Debian package rather than independently downloading it. Legacy output/spec flags remain available.
- Refactored generation into parse/apply/render phases. All resources are parsed before sensitivity is applied, and UOS processing errors now fail the run instead of printing and skipping files.
- Added strict extracted-input scanning for both flattened and raw pre-overlay schemas. It rejects credential signatures and unexpected concrete schema scalars; every optional metadata filename has an explicit top-level shape contract, with unknown files rejected.
- Added per-owned-file SHA-256 hashes to `SchemaSource`, deterministic first-path comparison, offline `-verify-committed`, and non-mutating local-snapshot `-verify-regeneration`.
- Added a generated-output transaction that owns only `*.generated.go`, `version.generated.go`, `specification.json`, and `schema-source.json`; it preserves hand-written Go, drops stale generated files, and stages provenance in the same rollback unit.

## GREEN evidence

Focused Task 5:

```text
$ go test ./cmd/fields -run TestRun -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.481s
```

Full verification:

```text
$ go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.013s
ok github.com/ubiquiti-community/go-unifi/unifi (cached)
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

$ go vet ./...
(no output)

$ git diff --check
(no output)
```

The full test run required the normal external sandbox profile because existing `httptest` tests bind loopback ports.

## Decisions and follow-ups

- UOS non-download-only generation requires `-generate-spec` and canonical `unifi` / `specification.json` destinations. Legacy/testing helpers retain custom destinations without publishing canonical provenance.
- Explicit/local installer sources leave OS version empty when it cannot be established; the bundled Network version remains authoritative for the snapshot and generated client.
- Metadata validators intentionally avoid entropy checks on schema regex strings. Task 6's real-release pilot should update a validator only when an observed optional metadata shape differs from its explicit fixture-backed contract.
- No Task 6 or Task 7 workflow files were changed.
