# Schema cache

`cmd/fields` generates the Go client and `specification.json` from the JSON
field definitions the UniFi Network application ships inside
`internal-dependencies.jar`. Those definitions are extracted **transiently at
build time** into this directory and are deliberately **not committed or
redistributed** — they originate from software proprietary to Ubiquiti Inc.
or its licensors, and this repository publishes only its own code and the
generated interoperable client. Only the two marker files and this README are
tracked.

## Layout

```
VERSION    UniFi Network application version of the local cache (tracked)
SOURCE     release the cache came from, "<product> <version>" (tracked) —
           generation skips the download when this matches the latest release
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

When the cache is current, runs only re-copy the overlay files — they never
delete. After *removing* a file from `overrides/resources/`, force a rebuild
so its stale copy leaves `fields/`: `rm schemas/SOURCE && go generate ./...`.

## Override layers

Schema adjustments are human-maintained under [overrides/](../overrides/)
(whole-resource definitions in `resources/`, declarative field pins and
compat fields in `fields.toml`), with conditional logic in
`cmd/fields/main.go` FieldProcessors and hand-written client code in
`unifi/`. See the comments at the top of `overrides/fields.toml` for the
selection rules; hand-written files must not re-declare generated types (the
generator fails with a collision error naming the offending file).

## Live verification

`go test -tags integration ./internal/testenv/ ./cmd/fields/` boots a
disposable simulation-mode controller (jacobalberty/unifi via
testcontainers; `admin`/`admin`) and compares the hand-written v2 schemas
in `overrides/resources/` against what the live API serves. Pin the
controller build with `UNIFI_TEST_IMAGE` or `UNIFI_TEST_PKGURL` (a UniFi
Network .deb URL), or point `UNIFI_TEST_URL` at an existing controller.
