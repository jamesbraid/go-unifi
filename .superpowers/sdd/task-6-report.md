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

The initial sandboxed full-suite run could not bind the tests' localhost
`httptest` servers. The same suite passed with the required localhost
permission; this was an environment restriction, not a product failure.
