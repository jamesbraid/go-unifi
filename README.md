# Unifi Go SDK [![GoDoc](https://godoc.org/github.com/ubiquiti-community/go-unifi?status.svg)](https://godoc.org/github.com/ubiquiti-community/go-unifi)

Built primarily for my [Terraform provider for Unifi](https://github.com/ubiquiti-community/terraform-provider-unifi).

## Versioning

Each release names the UniFi Network version it was generated and tested
against. Semantic versioning tracks the Go API, with one deviation:
controller-forced breaking changes ship in documented minor releases rather
than a `/vN` bump per controller train. Pin an exact version for module-strict
guarantees.

See [COMPATIBILITY.md](COMPATIBILITY.md) for the full policy: how upstream schema drift is absorbed, and why older SDK tags stay the way to target older controllers.

## Note on Code Generation

The generator builds data models and basic REST methods from the JSON field
definitions in `internal-dependencies.jar`. These definitions contain field
names plus regex and enum validation. They extract into a gitignored cache
under [schemas/](schemas/). Controller bytes and extracted definitions stay
outside Git.

[`schemas/capture.lock.json`](schemas/capture.lock.json) identifies the exact
controller artifact and expected extracted snapshots. Put the retained artifact
in a restricted content-addressed store and set `GO_UNIFI_CONTENT_STORE`. Then
run `go generate ./...`. Generation verifies every digest and fails if the
retained bytes are missing or changed. It never looks up "latest," downloads a
replacement, or updates the lock.

Maintainers capture a new artifact separately with `cmd/schema-capture`. That
command stores the bytes by SHA-256 and proposes a new lock. Ordinary generation
remains offline. See [schemas/README.md](schemas/README.md) for the capture and
rebuild procedures.

This code generation is kind of gross. I wanted to use the Java classes in the
jar like scala2go, but the jar is obfuscated. I couldn't find that information
anywhere else. The web UI may have it, but not in a practically extractable
form. I still plan to dig through the bits later.

## Testing

`go test ./...` runs the unit and schema tests. It needs no controller or Docker.

`go test -tags integration ./...` boots a real controller. The
`internal/controllertest` harness starts a disposable simulation-mode controller
from the published `ghcr.io/jamesbraid/unifi-network` image. The drift probe in
`cmd/fields` compares the hand-written v2 schemas with the live API. Set
`UNIFI_TEST_URL` to reuse an existing controller, or `UNIFI_TEST_IMAGE` to pin a
build. CI runs this gate on every schema change.
