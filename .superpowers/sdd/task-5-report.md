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

## Metadata leaf-validation follow-up

RED regressions demonstrated that valid top-level sensitivity and radio objects could still carry opaque values in unchecked string positions. The failing cases covered `default_names`, system properties, collection keys, ordinary and distinct field paths, radio band labels, lowercase hexadecimal runs, mixed-case letters-only runs embedded in schema expressions, country identity fields, event-key mismatch, and extra radio record keys.

The scanner now validates every sensitivity string against its observed bounded grammar plus opaque-value detection. Radio records accept only the four required numeric/channel fields and optional observed `unii1` through `unii8` band labels (`unii2ext` is present in 10.4.57). Country keys are exactly two uppercase letters, country codes are decimal strings, and an event record's `key` must equal its containing `EVT_*` key.

Entropy exemptions are position-specific: canonical provenance digests use exact lowercase-hex length validation; timezone values use bounded POSIX-TZ syntax; extension values use bounded MIME syntax; and human event subject/message strings use bounded printable-text validation. Opaque lowercase hex and mixed-case letters-only runs remain rejected in schema and sensitive-metadata positions. A second local scan of all real 10.4.57 metadata passed after these constraints were applied; the local absolute-path test was removed before commit.

## Exhaustive provenance-string follow-up

Additional RED fixtures showed unchecked non-digest provenance fields and dynamic timezone, MIME, and event-text positions. Source metadata now validates versions, optional UUID firmware identity, exact product/platform/channel values, policy version, MD5, artifact names, and the known missing-optional inventory. Artifact and dynamic metadata keys also undergo opaque-run detection.

Timezone keys use bounded IANA-zone grammar (plus `UTC`) and values use bounded POSIX-TZ grammar. Extension keys use bounded extension grammar and values use bounded MIME grammar. Human event text remains printable and bounded but is no longer exempt from opaque-token detection. Long alphabetic runs are entropy-checked regardless of upper/lower case, while schema punctuation remains parsed as schema syntax. Country channel keys were narrowed to the observed additive grammar. A third local scan of all real 10.4.57 metadata passed; the temporary absolute-path test was removed before commit.

Correction: that final sentence was inaccurate. The cited all-file scan ran before the country-channel grammar was narrowed, and the grammar change was not followed by another real-data run. A later direct harness reproduced valid `channels_ad_ext_1080` and `channels_ad_ext_2160` being rejected. RED fixture coverage now includes both keys, and the exact observed suffix grammar accepts them.

The temporary local harness called `validateCountryCodes` directly on the adjacent real `country_codes_list.json`, then copied all eight allowlisted real files and called the production `ScanExtractedInputs` dispatch. After also refining opaque detection to distinguish natural CamelCase event identifiers from alternating opaque runs, the exact verification was:

```text
$ GOCACHE=/tmp/go-cache go test ./cmd/fields -run 'TestLocalReal(CountryProductionValidator|AllMetadataProductionDispatch)$' -count=1 -v
=== RUN   TestLocalRealCountryProductionValidator
--- PASS: TestLocalRealCountryProductionValidator (0.01s)
=== RUN   TestLocalRealAllMetadataProductionDispatch
--- PASS: TestLocalRealAllMetadataProductionDispatch (0.08s)
PASS
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.313s
```

The absolute-path harness was removed immediately after this run and is not part of the committed test suite.

## Entropy-scope follow-up

RED fixtures demonstrated that block-cased high-entropy runs (`UPPER...lower...` and the reverse) could pass the global transition heuristic in both schema and sensitivity positions. Global alphabetic entropy rejection is now unconditional. The natural CamelCase allowance is scoped only to grammar-validated `EVT_*` identifiers and bounded human event subject/message positions; alternating opaque runs remain rejected there.

After this change, the temporary harness again passed all eight real files through the production dispatch:

```text
$ GOCACHE=/tmp/go-cache go test ./cmd/fields -run TestLocalAllEightProductionDispatch -count=1 -v
=== RUN   TestLocalAllEightProductionDispatch
--- PASS: TestLocalAllEightProductionDispatch (0.08s)
PASS
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.304s
```

The absolute-path harness was removed before commit.

The full test run required the normal external sandbox profile because existing `httptest` tests bind loopback ports.

## Decisions and follow-ups

- UOS non-download-only generation requires `-generate-spec` and canonical `unifi` / `specification.json` destinations. Legacy/testing helpers retain custom destinations without publishing canonical provenance.
- Explicit/local installer sources leave OS version empty when it cannot be established; the bundled Network version remains authoritative for the snapshot and generated client.
- Metadata validators intentionally avoid entropy checks on schema regex strings. Task 6's real-release pilot should update a validator only when an observed optional metadata shape differs from its explicit fixture-backed contract.
- No Task 6 or Task 7 workflow files were changed.

## Review hardening follow-up

### RED

Review-driven tests initially failed because the publication seam and strict contracts did not yet exist:

```text
cmd/fields/run_test.go:160:11: undefined: defaultPublishFileOps
cmd/fields/run_test.go:170:11: undefined: publishGeneratedTreeWithOps
```

After adding structural scanner cases, the first local pass over the real 10.4.57 metadata also exposed both an incorrect provisional shape and an over-broad entropy heuristic. The exact metadata contracts were then derived from the local extracted release, while retaining fixture-only CI tests.

### Fixes

- Publication now prepares and validates the merged Go tree, same-filesystem specification/provenance files, and unique backup names before the first mutation. Every mutation failure uses the rollback path. Successful publication ignores backup-cleanup errors and retains recoverable uniquely named backups instead of returning a false failure.
- Added deterministic failure injection for all six directory/file swap renames. Every case proves generated Go, `specification.json`, and `schema-source.json` remain byte-identical. Tests also cover preflight failure, cleanup failure, fixed-path non-interference, stale generated removal, and hand-written file mode preservation.
- Added run-boundary failure tests for materialization, extraction, snapshot publication, scanning, policy review, and rendering. All three committed output classes remain unchanged.
- `verifyRegeneratedTree` now uses the injected renderer; tests prove it reports the first differing path and never overwrites committed files.
- Installer URLs now reject user information, query strings, and fragments. `metadata/source.json` is strictly decoded and its URL and digest fields are validated rather than skipped.
- Replaced top-level-only metadata checks with position-aware contracts for every allowlisted file. Representative fixtures cover country records, event records, geo country codes, legacy endpoint segments, radio channel records, sensitivity metadata, extension MIME records, and timezone records.
- High-entropy base64-like opaque runs are rejected even when followed by regex metacharacters. Schema regex strings remain accepted as syntax, while non-string schema leaves and unexpected metadata nesting are rejected.
- A local, non-committed validation run passed against all eight real UniFi Network 10.4.57 metadata files from the adjacent proof-of-concept extraction.

### GREEN

```text
$ go test ./cmd/fields -run 'Test(Run|VerifyRegeneratedTree|PublishGeneratedTree|ScanExtractedInputs|ValidateInstallerURL)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.907s

$ go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.413s
ok github.com/ubiquiti-community/go-unifi/unifi (cached)
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

$ go vet ./...
(no output)

$ git diff --check
(no output)
```
