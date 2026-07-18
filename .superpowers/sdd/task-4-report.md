# Task 4 report: sensitivity policy and Terraform specification codegen

## Status

Implemented the reviewed sensitivity policy mapper, raw/source identity tracking,
transactional `FieldInfo.Sensitive` updates, and leaf-level Terraform specification
propagation. The committed bootstrap policy contains version `1` and empty digest,
generated-secret, and non-generated-secret lists. No vendor metadata is committed.

During implementation, Task 6 preflight corrected the policy contract. Task 4 now
distinguishes:

- `secret_paths`, which must resolve to exactly one Terraform-emitted leaf; and
- `non_generated_secret_paths`, which must remain non-generated (including exact
  paths in absent collections).

A path changing between those states fails before any field flag is mutated.
Coverage exposes aggregate Generated/NonGenerated lists plus deterministic
secret/private by Generated/NonGenerated lists.

## RED evidence

Initial focused run after adding policy and specification tests:

```text
$ go test ./cmd/fields -run 'Test(ParseSensitivity|ApplySensitivity|SensitivityPolicy|ResourceSourceIdentity|SpecificationGenerator_.*Sensitive|Specification_JSONStructure)' -count=1
cmd/fields/sensitivity_test.go:27:71: undefined: SensitivityPolicy
cmd/fields/sensitivity_test.go:37:4: r.SourceFileBase undefined
cmd/fields/schema_test.go:162:13: privateKey.Sensitive undefined
FAIL github.com/ubiquiti-community/go-unifi/cmd/fields [build failed]
```

After the corrected non-generated-secret contract was received, its tests failed
before production changes:

```text
$ GOCACHE=/tmp/go-build-task4 go test ./cmd/fields -run 'TestApplySensitivity_NonGeneratedSecretPathsEnforceStatus|TestApplySensitivity_SettingExpansionAndSkippedTerraform|TestApplySensitivity_ClassifiesExactLeavesAndCoverage' -count=1
cmd/fields/sensitivity_test.go:40:105: unknown field NonGeneratedSecretPaths in struct literal of type SensitivityPolicy
cmd/fields/sensitivity_test.go:140:14: coverage.SecretGenerated undefined
cmd/fields/sensitivity_test.go:141:27: coverage.SecretNonGenerated undefined
FAIL github.com/ubiquiti-community/go-unifi/cmd/fields [build failed]
```

Independent review then found that an absent raw collection was accepted as
non-generated without checking a custom/generated resource with the same identity.
The regression test failed before the fix:

```text
$ GOCACHE=/tmp/go-build-task4 go test ./cmd/fields -run TestApplySensitivity_NonGeneratedSecretPathsEnforceStatus -count=1
Error: An error is expected but got nil.
FAIL github.com/ubiquiti-community/go-unifi/cmd/fields
```

The absent-collection branch now also resolves the generated identity and fails on
a leaf or ambiguity before recording the reviewed non-generated secret.

## GREEN evidence

Required focused verification:

```text
$ GOCACHE=/tmp/go-build-task4 go test ./cmd/fields -run 'Test(ParseSensitivity|ApplySensitivity|SensitivityPolicy|SpecificationGenerator_.*Sensitive|Specification_JSONStructure)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.206s
```

Full repository verification (run outside the filesystem/network sandbox because
existing `httptest` cases bind loopback ports):

```text
$ GOCACHE=/tmp/go-build-task4 go test ./... -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.050s
ok github.com/ubiquiti-community/go-unifi/unifi 1.409s
ok github.com/ubiquiti-community/go-unifi/unifi/settings 0.352s
```

`go vet ./...` and `git diff --check` also passed.

## Coverage and self-review

- Full real metadata shape is parsed strictly, including `min_field_size`,
  `default_names`, and all three sensitivity sections; the approved digest covers
  the entire canonical JSON document.
- Raw collections use a case-folded identity index with ambiguity rejection; raw
  field traversal remains exact and case-sensitive and unwraps one-element schema
  arrays during nested traversal.
- Split settings derive their original raw top-level key from unsplit `Setting.json`
  and carry it in `SourcePathPrefix`.
- Classified missing/skipped fields and system properties are recorded as
  non-generated. Explicit reviewed generated and non-generated secret additions may
  extend vendor metadata without substring inference.
- Generated traversal follows `JSONName`, custom/nested types, and only
  Terraform-emitted map entries; duplicate emitted JSON names fail.
- Sensitivity updates clear stale reachable flags and set exact leaves only after
  every parse, digest, raw, coverage, and generated-resolution check succeeds.
  Aliased `FieldInfo` pointers are deduplicated.
- All concrete resource and data-source attribute constructors propagate the
  current field's sensitivity pointer. Enclosing nested objects remain nil unless
  explicitly marked. Provider `password` and `api_key` remain sensitive.

## Concerns / follow-up

Task 4 deliberately does not wire parse/apply/write orchestration; Task 5 owns that
boundary. The bootstrap policy intentionally approves no real metadata digest and
contains no real secret paths; Task 6 must review the local 10.4.57 metadata and
populate both policy path lists.
