# Unifi Go SDK [![GoDoc](https://godoc.org/github.com/ubiquiti-community/go-unifi?status.svg)](https://godoc.org/github.com/ubiquiti-community/go-unifi)

This was written primarily for use in my [Terraform provider for Unifi](https://github.com/ubiquiti-community/terraform-provider-unifi).

## Versioning

Many of the naming adjustments are breaking changes, but to simplify things, treating naming errors as minor changes for the 1.0.0 version (probably should have just started at 0.1.0).

## Note on Code Generation

The data models and basic REST methods are generated from JSON field
definition files shipped inside the UniFi Network application
(`api/fields/*.json` in `internal-dependencies.jar`, bundled in `ace.jar`).

For UniFi Network 10 and later, `cmd/fields` downloads the UniFi OS Server
installer from Ubiquiti's firmware API, extracts `ace.jar` from the OCI image
inside, and pulls the definitions out. For Network 9 and earlier it can still
fetch the legacy Debian package instead.

To regenerate, run `go generate` inside the `unifi` directory. Source modes:

    fields -latest                # latest UniFi OS Server release
    fields -os-server 5.1.21      # a specific UniFi OS Server release
    fields -url <installer-url>   # direct installer URL
    fields -installer <path>      # local installer file
    fields 9.5.21                 # legacy deb, explicit Network version

Extracted fields are cached in `cmd/fields/v<network-version>/` (gitignored),
including a `metadata/` dir with upstream extras such as
`sensitive_metadata.json`, which drives `Sensitive` flags in
`specification.json`.
