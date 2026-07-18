# Task 2 report: safe OCI and nested archive extraction

## Scope

Implemented only Task 2 from `docs/superpowers/plans/2026-07-18-unifi-os-schema-fetcher.md`:

- bounded OCI image-tar import;
- nested OCI index resolution for one explicit `linux/amd64` leaf manifest;
- descriptor size and digest verification before parsing or decompression;
- gzip and uncompressed layer lookup with reverse overlay and OCI whiteouts;
- prefixed installer ZIP, `ace.jar`, and `internal-dependencies.jar` extraction;
- selected field, metadata, and notice artifact hashing;
- synthetic and malicious archive fixtures;
- direct OCI image-spec and go-digest dependencies.

No command wiring, snapshot publication, sensitivity policy, or generator behavior from later tasks was added.

## RED evidence

### OCI API and behavior

After creating `archive_fixture_test.go` and `oci_test.go`, the focused test command was run before production code existed:

```text
$ go test ./cmd/fields -run 'Test(ImportOCI|ResolveImage|FindFileInLayers)' -count=1
cmd/fields/oci_test.go:20:17: undefined: ImportOCI
cmd/fields/oci_test.go:20:116: undefined: DefaultArchiveLimits
cmd/fields/oci_test.go:31:11: undefined: ArchiveLimits
cmd/fields/oci_test.go:50:16: undefined: ResolveImage
FAIL github.com/ubiquiti-community/go-unifi/cmd/fields [build failed]
```

This established the initial RED state for the planned interfaces.

### Nested installer extraction

After adding the complete synthetic installer tests, the focused command failed before `uos_extract.go` existed:

```text
$ go test ./cmd/fields -run 'Test(ExtractUOSInstaller|SyntheticInstaller)' -count=1
cmd/fields/uos_extract_test.go:17:17: undefined: ExtractUOSInstaller
cmd/fields/uos_extract_test.go:35:41: undefined: ExtractedArtifact
FAIL github.com/ubiquiti-community/go-unifi/cmd/fields [build failed]
```

### Root opaque-whiteout regression

Self-review added a root opaque-whiteout case. It failed against the first implementation because `.wh..wh..opq` at the layer root did not block lower descendants:

```text
--- FAIL: TestFindFileInLayersCompressionOverlayAndWhiteouts/root_opaque_whiteout
    Error: An error is expected but got nil.
```

The blocker predicate was corrected so a root opaque marker hides every lower path while a same-layer replacement still wins.

## GREEN evidence

### OCI phase

```text
$ go test ./cmd/fields -run 'Test(ImportOCI|ResolveImage|FindFileInLayers)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields
```

### Installer phase

```text
$ GOCACHE=/tmp/go-cache go test ./cmd/fields -run 'Test(ExtractUOSInstaller|SyntheticInstaller)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields
```

### Focused final verification

```text
$ GOCACHE=/tmp/go-cache go vet ./cmd/fields
$ GOCACHE=/tmp/go-cache go test ./cmd/fields -run 'Test(ImportOCI|ResolveImage|FindFileInLayers|ExtractUOSInstaller|SyntheticInstaller)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields
$ git diff --check
```

### Repository-wide verification

The first sandboxed `go test ./...` attempt reached existing `httptest` suites but
could not bind localhost (`listen tcp6 [::1]:0: bind: operation not permitted`).
It was rerun with the required localhost permission and passed:

```text
$ GOCACHE=/tmp/go-cache go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields
?  github.com/ubiquiti-community/go-unifi/internal/fields [no test files]
ok github.com/ubiquiti-community/go-unifi/unifi
ok github.com/ubiquiti-community/go-unifi/unifi/settings
?  github.com/ubiquiti-community/go-unifi/unifi/types [no test files]
```

## Implementation details

### Limits and ownership

`DefaultArchiveLimits` uses the resolved limits:

- image tar: 2 GiB;
- staged blob: 2 GiB;
- decompressed layer: 4 GiB;
- each JAR: 512 MiB;
- selected JSON, properties, or notice: 32 MiB;
- entries per archive: 500,000;
- nested index depth: 8;
- private OCI control JSON cap: 8 MiB.

Every field must be positive. `ImportOCI` and `ExtractUOSInstaller` create private subdirectories under the caller-owned temporary root and remove them on failure. Successful extracted-artifact paths remain valid until the caller removes that root.

### OCI import and resolution

- Only `oci-layout`, `index.json`, and regular digest-addressed blobs are staged.
- Safe unrelated regular layout extensions are ignored.
- Traversal, absolute/backslash paths, normalized duplicates, links, special entries, entry limits, byte limits, malformed blob names, and blob-content digest mismatch are rejected.
- Root and nested control documents are bounded.
- Every followed index/manifest/layer descriptor is validated by raw size and digest before parsing/decompression.
- Nested indexes require an explicit matching platform on the leaf manifest descriptor; there is no platform inheritance.
- Repeated matching leaf digests and multiple matching manifests are rejected as ambiguous.
- Safe unknown auxiliary index descriptor types are ignored.

### Layer lookup

- Standard OCI uncompressed tar and gzip media types are accepted.
- Zstd, nondistributable, and all other layer media types produce boundary-specific unsupported-media errors.
- Layers are scanned newest first.
- All names, normalized duplicates, and entry counts are validated.
- Unrelated valid rootfs directories, links, and special entries are ignored because they are not materialized.
- Links or special entries at the requested target or a whiteout marker are rejected.
- Regular and opaque whiteouts block the exact target and all relevant descendants in lower layers, independent of tar order.
- Same-layer additions survive same-layer whiteouts, as required by OCI semantics.
- Gzip/tar streams are drained to EOF to surface trailer/checksum errors and enforce decompressed-byte limits.

### Nested extraction

- The complete ELF-prefixed installer is opened directly with `archive/zip`.
- Exactly one regular `image.tar` is required.
- `ace.jar` is found only at `usr/lib/unifi/lib/ace.jar` through resolved layer semantics.
- `product.properties` and `BOOT-INF/lib/internal-dependencies.jar` use exact paths with no fuzzy fallback.
- A single non-empty `version=` property is required.
- Fields retain full `api/fields/...` map keys.
- Metadata uses top-level filenames and the fixed allowlist.
- Notice keys are prefixed with `ace.jar/` or `internal-dependencies.jar/` and include case-insensitive `LICENSE`, `LICENSE.*`, `NOTICE`, or `NOTICE.*` basenames at root or under `META-INF`.
- Every returned artifact records its selected name, temporary path, size, and lowercase SHA-256.
- `Setting.json`, another field definition, and `sensitive_metadata.json` are required; missing optional metadata is sorted.
- Context cancellation is checked between entries and during copies.

## Test coverage

Synthetic tests cover:

- prefixed installer ZIP through the full nested chain;
- nested indexes and explicit `linux/amd64` selection;
- gzip and uncompressed layers;
- manifest and layer descriptor size/digest mismatches;
- zstd/unsupported layers;
- traversal, links, normalized duplicates, entry limits, image/layer/JAR/JSON byte limits;
- newest-layer selection;
- regular, ancestor, opaque, and root opaque whiteouts;
- same-layer addition versus opaque whiteout;
- required version, nested JAR, field, and sensitivity metadata failures;
- selected metadata, notices, hashes, optional omissions, and cancellation.

## Self-review notes

- Dependency graph contains only the requested direct OCI modules; no registry or runtime stack was introduced.
- Existing legacy Debian extraction and `main()` wiring are unchanged.
- Temporary layout, selected JAR, nested JAR, candidate, and partial artifact cleanup paths were checked.
- The package cache under the user Library directory was sandbox-blocked once; verification uses `GOCACHE=/tmp/go-cache`, with no source impact.

## Review fixes

### RED: target-ancestor links and special entries

Review identified that `scanLayer` rejected unsafe types only at the exact target
and whiteout paths. Tests added a same-layer ancestor symlink plus target, a newer
ancestor hardlink over a lower target, and a newer ancestor character-device entry
over a lower target. All three initially exposed the target and failed with:

```text
--- FAIL: TestFindFileInLayersRejectsLinkOrSpecialTargetAncestors
    --- FAIL: .../same_layer_ancestor_symlink
        Error: An error is expected but got nil.
    --- FAIL: .../newer_layer_ancestor_hardlink
        Error: An error is expected but got nil.
    --- FAIL: .../newer_layer_ancestor_special
        Error: An error is expected but got nil.
```

`scanLayer` now recognizes every ancestor of the requested target and rejects link
or special archive types there, while continuing to allow unrelated valid rootfs
entries and real ancestor directories.

### RED: notice basename suffixes

Review also identified that notice selection accepted only exact basenames or dot
suffixes. Fixtures added root and `META-INF` entries from both JARs using hyphen and
underscore suffixes. All four were initially absent:

```text
--- FAIL: TestExtractUOSInstallerInventoriesNoticeBasenameSuffixes
    map does not contain "ace.jar/LICENSE-APACHE"
    map does not contain "ace.jar/META-INF/notice_third-party"
    map does not contain "internal-dependencies.jar/NOTICE-third-party"
    map does not contain "internal-dependencies.jar/META-INF/LICENSE_BSD"
```

`isNoticePath` now accepts any case-insensitive basename beginning with `LICENSE`
or `NOTICE`, while retaining the root-or-`META-INF` location restriction.

### GREEN: review cases

```text
$ GOCACHE=/tmp/go-cache go test ./cmd/fields -run 'TestFindFileInLayersRejectsLinkOrSpecialTargetAncestors|TestExtractUOSInstallerInventoriesNoticeBasenameSuffixes' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields
```

### Review-fix final verification

```text
$ GOCACHE=/tmp/go-cache go test ./cmd/fields -run 'Test(ImportOCI|ResolveImage|FindFileInLayers|ExtractUOSInstaller|SyntheticInstaller)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields

$ GOCACHE=/tmp/go-cache go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields
?  github.com/ubiquiti-community/go-unifi/internal/fields [no test files]
ok github.com/ubiquiti-community/go-unifi/unifi
ok github.com/ubiquiti-community/go-unifi/unifi/settings
?  github.com/ubiquiti-community/go-unifi/unifi/types [no test files]

$ git diff --check
```
