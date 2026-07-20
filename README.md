# Unifi Go SDK [![GoDoc](https://godoc.org/github.com/ubiquiti-community/go-unifi?status.svg)](https://godoc.org/github.com/ubiquiti-community/go-unifi)

Built primarily for my [Terraform provider for Unifi](https://github.com/ubiquiti-community/terraform-provider-unifi).

## Versioning

Each release names the one UniFi Network version it was generated and tested against. Semantic versioning tracks the Go API, with one deviation: controller-forced breaking changes ship in documented minor releases rather than a `/vN` bump per controller train. Pin an exact version for module-strict guarantees.

See [COMPATIBILITY.md](COMPATIBILITY.md) for the full policy: how upstream schema drift is absorbed, and why older SDK tags stay the way to target older controllers.

## Note on Code Generation

The generator builds the data models and basic REST methods from the JSON field definitions the controller ships in `internal-dependencies.jar` (field names plus regex and enum validation). The definitions extract into a gitignored cache under [schemas/](schemas/); only the controller version markers are tracked, never the extracted files.

`go generate ./...` refreshes everything: it checks the Ubiquiti firmware update API for the latest release, downloads it when the local cache is stale (UniFi Network `.deb` preferred, UniFi OS Server installer as fallback), re-extracts the schemas, and regenerates the Go code and `specification.json`. To generate from a manually downloaded artifact instead, run `go run ./cmd/fields -file <deb-or-installer> -output-dir=unifi`.

A nightly workflow runs the same check and opens an auto-merging PR when Ubiquiti publishes a new controller version. The tests and the live-controller integration gate must pass first, and a breaking change waits for a manual tag.

This code generation is kind of gross. I wanted to use the java classes in the jar like scala2go, but the jar is obfuscated and I couldn't find that information anywhere else — maybe the web UI has it, but not in any practically extractable form. Still planning to dig through the bits some more later on.

## Testing

`go test ./...` runs the unit and schema tests; no controller or Docker needed.

`go test -tags integration ./...` boots a real controller and tests against it. The `internal/controllertest` harness starts a disposable simulation-mode controller from the published `ghcr.io/jamesbraid/unifi-network` image, and the drift probe in `cmd/fields` compares the hand-written v2 schemas against what the live API serves. Set `UNIFI_TEST_URL` to reuse an existing controller instead of a container, or `UNIFI_TEST_IMAGE` to pin a build. CI runs this gate on every schema change.
