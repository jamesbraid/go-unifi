# Schema cache

`cmd/fields` generates the Go client and `specification.json` from the JSON
field definitions the UniFi Network application ships inside
`internal-dependencies.jar`. Those definitions are extracted **transiently at
build time** into this directory and are deliberately **not committed or
redistributed** — they originate from software proprietary to Ubiquiti Inc.
or its licensors, and this repository publishes only its own code and the
generated interoperable client. Only the three marker files and this README
are tracked.

## Layout

```
VERSION    UniFi Network application version of the local cache (tracked)
SOURCE     release identity the cache came from — "<product> <full-version>
           <firmware-record-id>" (tracked); generation skips the download
           when this matches the latest release
ARTIFACT   URL of the controller artifact the cache came from (tracked);
           a provenance record of the release the cache was extracted from
fields/    extracted per-entity field validators + exploded Setting*.json +
           the overlay copies from overrides/resources/   (gitignored)
metadata/  extracted sensitive_metadata.json, which feeds the specification's
           sensitive flags                                 (gitignored)
```

## Regenerating

```sh
go generate ./...                    # latest release (deb preferred, UniFi OS
                                     # Server installer fallback)
go run ./cmd/fields -file <artifact> -output-dir=unifi   # from a local download
```

`-file` accepts either a UniFi Network `.deb` or a UniFi OS Server
self-extracting installer; the format and Network version are detected from
the artifact. CI caches `fields/` and `metadata/` keyed on `SOURCE`, so only
runs that encounter a new release download anything.

When the cache is current, runs re-sync the overlay files: overrides are
re-copied and, using the `.extracted` manifest, copies of since-deleted
override files are removed. Caches predating the manifest are treated as
invalid and rebuilt.

## Override layers

Schema adjustments are human-maintained under [overrides/](../overrides/)
(whole-resource definitions in `resources/`, declarative field pins and
compat fields in `fields.toml`), with conditional logic in
`cmd/fields/main.go` FieldProcessors and hand-written client code in
`unifi/`. See the comments at the top of `overrides/fields.toml` for the
selection rules; hand-written files must not re-declare generated types (the
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
