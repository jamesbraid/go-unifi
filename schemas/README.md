# Schema cache

`cmd/fields` generates the Go client and `specification.json` from the JSON
field definitions the UniFi Network application ships inside
`internal-dependencies.jar`. The controller artifact and extracted definitions
are not committed or redistributed. A restricted content-addressed store holds
the retained artifact. This directory contains only permitted generated output,
digests, and provenance.

## Layout

```
capture.lock.json  sole structural-source lock (tracked)
VERSION            Network version projection of capture.lock.json (tracked)
SOURCE             product/build projection of capture.lock.json (tracked)
ARTIFACT           source-location projection of capture.lock.json (tracked)
GENERATED_SHA256   complete generated-output digest (tracked)
fields/            extracted structural snapshot plus overlays (gitignored)
metadata/          extracted sensitivity input (gitignored)
```

Do not edit `VERSION`, `SOURCE`, or `ARTIFACT`. Generation rewrites them from
the lock. `specification.json` remains a bootstrap and golden artifact. It does
not define Terraform policy.

`GENERATED_SHA256` covers generated Go, `specification.json`, the lock, and the
three compatibility projections. It does not hash itself.

## Regenerating

Set the restricted store path and generate from the lock:

```sh
export GO_UNIFI_CONTENT_STORE=/restricted/go-unifi/artifacts
go generate ./...
```

`cmd/fields` verifies the artifact size and SHA-256, generator-input digests,
Network version, structural snapshot, and sensitivity snapshot before it
replaces the local cache. A missing object, corrupt lock, changed generator, or
snapshot mismatch stops the run.

To propose a lock for a new artifact:

```sh
go run ./cmd/schema-capture \
  -file=/restricted/incoming/unifi.deb \
  -source-location=https://downloads.example.invalid/unifi.deb \
  -media-type=application/vnd.debian.binary-package \
  -product=unifi-controller \
  -build='v10.4.57+build-record' \
  -network-version=10.4.57 \
  -content-store="$GO_UNIFI_CONTENT_STORE"
```

Use `-url` instead of `-file` when capture should fetch an explicitly selected
artifact. Capture never discovers or blesses a replacement during ordinary
generation. Review the proposed lock and generated API changes together.

The lock records an operator-pinned checksum. Signing-key trust and key
rotation are release-workflow concerns, not part of this initial contract.

For the two-run, network-disabled Linux/amd64 rebuild, see
[`build/m0/README.md`](../build/m0/README.md).

## Override layers

Schema adjustments are human-maintained under [overrides/](../overrides/)
(whole-resource definitions in `resources/`, declarative field pins and
compat fields in `fields.toml`), with conditional logic in
`cmd/fields/main.go` FieldProcessors and hand-written client code in
`unifi/`. See the comments at the top of `overrides/fields.toml` for the
selection rules. Hand-written files must not re-declare generated types (the
generator fails with a collision error naming the offending file).

## Live verification

`go test -tags integration ./internal/controllertest/ ./cmd/fields/` boots a
disposable simulation-mode controller and compares the hand-written v2
schemas in `overrides/resources/` against what the live API serves. The
default image is already the published simulation image of the current
schema build (`admin`/`admin`, no setup wizard), so a bare run tests the
right version with no setup.

To pin explicitly, or test another build:

```sh
UNIFI_TEST_IMAGE="ghcr.io/jamesbraid/unifi-network:$(cat schemas/VERSION)-sim" \
  UNIFI_TEST_EXPECT_VERSION="$(cat schemas/VERSION)" \
  go test -tags integration ./internal/controllertest/ ./cmd/fields/
```

Images are published by github.com/jamesbraid/unifi-containers.
`UNIFI_TEST_EXPECT_VERSION` makes the smoke test fail unless the booted
controller reports exactly that version — this is how CI verifies the pin
actually took. Or point `UNIFI_TEST_URL` at an existing controller —
targets used this way must accept the demo `admin`/`admin` credentials.
