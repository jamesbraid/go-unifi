# Unifi Go SDK [![GoDoc](https://godoc.org/github.com/ubiquiti-community/go-unifi?status.svg)](https://godoc.org/github.com/ubiquiti-community/go-unifi)

This was written primarily for use in my [Terraform provider for Unifi](https://github.com/ubiquiti-community/terraform-provider-unifi).

This project is not affiliated with or endorsed by Ubiquiti Inc. UniFi and
Ubiquiti are trademarks of Ubiquiti Inc. No Ubiquiti-owned files are
redistributed: the controller's API field definitions are extracted
transiently at build time (see [schemas/](schemas/)) solely to generate this
interoperable client, and only the project's own code and generated output
are published.

## Versioning

Many of the naming adjustments are breaking changes, but to simplify things, treating naming errors as minor changes for the 1.0.0 version (probably should have just started at 0.1.0).

See [COMPATIBILITY.md](COMPATIBILITY.md) for the full policy: one tested controller train per release, how upstream schema drift is absorbed, and why controller-forced breaking changes ship in documented minor releases.

## Note on Code Generation

The data models and basic REST methods are generated from the JSON field definitions the controller ships in `internal-dependencies.jar` (field names plus regex/enum validation). The definitions are extracted transiently into a gitignored cache under [schemas/](schemas/); only the controller version markers are tracked, and the extracted files are never committed.

`go generate ./...` refreshes everything: it checks the Ubiquiti firmware update API for the latest release, downloads it if the local cache is stale (UniFi Network `.deb` preferred, UniFi OS Server installer as fallback), re-extracts the schemas, and regenerates the Go code and `specification.json`. To generate from a manually downloaded artifact instead, run `go run ./cmd/fields -file <deb-or-installer> -output-dir=unifi`.

A nightly workflow performs the same check and opens an auto-merging PR when Ubiquiti publishes a new controller version; merging it tags and publishes a release automatically.

This code generation is kind of gross, I wanted to switch to using the java classes in the jar like scala2go but the jar is obfuscated and I couldn't find a way to extract that information from anywhere else. Maybe it exists somewhere in the web UI, but I was unable to find it in there in a way that was extractable in a practical way.

Still planning to dig through the bits some more later on.
