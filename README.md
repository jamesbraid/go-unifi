# Unifi Go SDK [![GoDoc](https://godoc.org/github.com/ubiquiti-community/go-unifi?status.svg)](https://godoc.org/github.com/ubiquiti-community/go-unifi)

This was written primarily for use in my [Terraform provider for Unifi](https://github.com/ubiquiti-community/terraform-provider-unifi).

## Versioning

Many of the naming adjustments are breaking changes, but to simplify things, treating naming errors as minor changes for the 1.0.0 version (probably should have just started at 0.1.0).

## Note on Code Generation

The data models and basic REST methods are generated from JSON field definitions
bundled with UniFi Network. Current UniFi OS Server releases, pinned/offline
regeneration, validation gates, sensitivity review, and recovery procedures are
documented in [docs/schema-generation.md](docs/schema-generation.md).

This project remains focused on the Internal API used by the existing SDK and
Terraform provider; it does not switch to the Official UniFi API.

`go-unifi` is independent and unofficial, and is not affiliated with or endorsed
by Ubiquiti. Installers and extracted vendor materials are downloaded by the user,
remain vendor-governed, and are not redistributed. See
[LICENSES/README.md](LICENSES/README.md) for the licensing boundary.

Terraform `Sensitive` attributes are masked in display, but their values still
exist in state. Use protected state storage and access controls.
