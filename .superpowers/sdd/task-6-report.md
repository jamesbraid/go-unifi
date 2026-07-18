# Task 6 implementation report

## Docker media-type compatibility bugfix

Root cause: the real UniFi OS Server `image.tar` is an OCI image layout whose
linux/amd64 descriptor and manifest use Docker schema 2 media type
`application/vnd.docker.distribution.manifest.v2+json`, and whose compressed
layers use `application/vnd.docker.image.rootfs.diff.tar.gzip`. `ResolveImage`
and `FindFileInLayers` previously accepted only the corresponding OCI media
types, so resolution ended with `expected exactly one linux/amd64 OCI manifest,
found 0` despite the descriptor being otherwise valid.

RED:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestResolveImageAcceptsDocker|TestResolveImageRejectsDocker|TestResolveImageRejectsDescriptorMismatch' -count=1
--- FAIL: TestResolveImageAcceptsDockerSchema2ManifestAndNestedList
    Received unexpected error:
        OCI index: unsupported media type "application/vnd.docker.distribution.manifest.list.v2+json"
FAIL
```

GREEN:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestResolveImageAcceptsDocker|TestResolveImageRejectsDocker|TestResolveImageRejectsDescriptorMismatch|TestFindFileInLayers' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields
```

The implementation adds role-specific support for Docker manifest lists,
Docker schema 2 manifests, and Docker gzip layers. Docker schema 1 and unknown
media types remain unsupported. Existing descriptor size and digest validation
is unchanged and is exercised for Docker manifests by the new fixture cases.

Read-only validation against `/private/tmp/recheck/image.tar` advanced through
`ResolveImage`, confirming that its single linux/amd64 Docker schema 2 manifest
is now selected. `FindFileInLayers` then exposed a separate issue while scanning
an unrelated layer entry named
`lib/systemd/system/system-systemd\\x2dcryptsetup.slice`: the general archive
path validator rejects backslashes. That issue is deliberately not bundled
into this media-type compatibility commit.

Verification after the scoped implementation:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'Test(ImportOCI|ResolveImage|FindFileInLayers|ExtractUOSInstaller|SyntheticInstaller)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.307s

GOCACHE=/tmp/go-build-task6 go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.579s
ok github.com/ubiquiti-community/go-unifi/unifi 1.413s
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

GOCACHE=/tmp/go-build-task6 go vet ./...
git diff --check
```

## Singleton schema-enum scanner compatibility

Root cause: `PortConf.json` contains `op_mode: "switch"` in both the flattened
and raw schemas. This is a singleton lowercase enum validator, but
`schemaString` previously accepted type names, regex-shaped strings, and
pipe-delimited values only. Scanner errors also omitted the JSON key path, so
the real snapshot failure reported only `unexpected concrete scalar "switch"`.

RED:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestScanExtractedInputs(AcceptsSingletonSchemaEnums|ReportsRejectedSchemaJSONPath)' -count=1
--- FAIL: TestScanExtractedInputsAcceptsSingletonSchemaEnums
    unexpected concrete scalar "switch"
--- FAIL: TestScanExtractedInputsReportsRejectedSchemaJSONPath
    Error "unexpected concrete scalar \"literal-secret-value\"" does not contain "/outer/op_mode"
FAIL
```

Schema recursion now visits object keys in sorted order and reports RFC 6901
JSON Pointer paths for rejected leaves. After the unchanged credential and
high-entropy checks, `schemaString` accepts a singleton enum only when it
matches the explicit bounded grammar `[a-z][a-z0-9_-]{0,15}`. Existing concrete
string and high-entropy rejection fixtures remain green; in particular,
`literal-secret-value` remains rejected and is now reported at
`/outer/op_mode`.

Focused scanner tests and the direct real-snapshot scan pass:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestScanExtractedInputs' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.235s

GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run TestTask6RealSnapshotScan -count=1 -v
=== RUN   TestTask6RealSnapshotScan
--- PASS: TestTask6RealSnapshotScan (0.09s)
PASS
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.303s
```

The temporary real-snapshot test was removed before final verification and
commit. The full `cmd/fields/v10.4.57` snapshot scan has no next error.

Final verification:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestScanExtractedInputs' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.241s

GOCACHE=/tmp/go-build-task6 go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.394s
ok github.com/ubiquiti-community/go-unifi/unifi (cached)
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

GOCACHE=/tmp/go-build-task6 go vet ./...
git diff --check
```

The initial sandboxed full-suite run could not bind the tests' localhost
`httptest` servers. The same suite passed with the required localhost
permission; this was an environment restriction, not a product failure.

## POSIX layer-name compatibility bugfix

Root cause: `scanLayer` applied `cleanArchiveName` to every tar header even
though unrelated layer paths are never staged or written. That validator
intentionally rejects backslashes for host-path safety, but a backslash is an
ordinary byte in a POSIX filename. The real image consequently stopped at the
unrelated systemd-escaped name
`lib/systemd/system/system-systemd\\x2dcryptsetup.slice` before it reached
`ace.jar`.

RED:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestFindFileInLayers(AllowsLiteralBackslash|RejectsMalformedOrDuplicateLayerNames)' -count=1
--- FAIL: TestFindFileInLayersAllowsLiteralBackslashInUnrelatedPOSIXName
    invalid path "lib/systemd/system/system-systemd\\x2dcryptsetup.slice": path is not relative slash-separated
--- FAIL: TestFindFileInLayersRejectsMalformedOrDuplicateLayerNames/empty_slash_component
--- FAIL: TestFindFileInLayersRejectsMalformedOrDuplicateLayerNames/dot_slash_component
FAIL
```

GREEN uses a layer-only POSIX header validator. It permits literal backslashes
and the conventional single trailing slash on tar directory headers, but
rejects empty, absolute, NUL-containing, traversal, dot-component, redundant
slash, and duplicate names. The requested target still uses strict
`cleanArchiveName`; OCI and ZIP paths that can be staged or written are
unchanged. Layer header names are only compared with slash-delimited targets,
and extracted candidates are always written to `os.CreateTemp`, so a Windows
host cannot turn the accepted layer name into an output path.

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestFindFileInLayers' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.238s
```

A temporary, uncommitted test opened `/private/tmp/recheck/image.tar`, imported
the layout, resolved linux/amd64, and called `FindFileInLayers` for
`usr/lib/unifi/lib/ace.jar`. It extracted a non-empty `ace.jar` successfully:

```text
--- PASS: TestTask6RealImageTarValidation (9.91s)
PASS
ok github.com/ubiquiti-community/go-unifi/cmd/fields 10.124s
```

The temporary test containing the machine-local absolute path was removed
before verification and commit.

Final verification:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'Test(ImportOCI|ResolveImage|FindFileInLayers|ExtractUOSInstaller|SyntheticInstaller)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.306s

GOCACHE=/tmp/go-build-task6 go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.407s
ok github.com/ubiquiti-community/go-unifi/unifi (cached)
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

GOCACHE=/tmp/go-build-task6 go vet ./...
git diff --check
```
