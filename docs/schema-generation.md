# UniFi schema generation

This project generates its Internal API models from the field definitions
bundled with UniFi Network. It does not consume or generate Ubiquiti's Official
UniFi API/OpenAPI surface.

## Source selection

Run the generator from the repository root with exactly one source selector:

```sh
# Machine-readable Ubiquiti firmware discovery
go run ./cmd/fields -uos-latest -generate-spec
go run ./cmd/fields -uos-version 5.1.21 -generate-spec

# Explicit official URL or a separately downloaded installer
go run ./cmd/fields -installer-url https://fw-download.ubnt.com/data/unifi-os-server/... -generate-spec
go run ./cmd/fields -installer /path/to/unifi-os-server -generate-spec

# Existing legacy controller-package selectors remain available
go run ./cmd/fields -latest
go run ./cmd/fields 9.5.21
```

`-uos-version` is the preferred reproducible online selector. Discovery must
return exactly one `unifi-os-server`, `linux-x64`, release-channel record, and
the downloader verifies the advertised size and SHA-256 before extraction. An
explicit URL is restricted to HTTPS Ubiquiti domains. A local `-installer`
cannot establish facts that are absent from the file itself, so preserve the
original URL and checksums separately when auditability matters.

Use `-download-only` with any UniFi OS selector to extract and scan a local,
gitignored snapshot without generating tracked output. After successful
extraction, regeneration from that pinned snapshot is offline:

```sh
go run ./cmd/fields -verify-regeneration
```

UniFi OS publication always owns the canonical `unifi/` generated files and
`specification.json`; custom output roots are rejected for provenance safety.

## Space, limits, and cleanup

Allow at least 8-10 GiB of free disk space. The approximately 880 MiB installer
is expanded through an image tar, verified OCI blobs, compressed layers,
`ace.jar`, and `internal-dependencies.jar`; temporary copies can overlap.

Default hard limits are:

- image tar and individual OCI blob: 2 GiB each;
- decompressed layer: 4 GiB;
- extracted JAR target: 512 MiB;
- individual JSON artifact: 32 MiB;
- archive entries: 500,000;
- nested OCI index depth: 8;
- OCI control JSON: 8 MiB.

Downloads, extraction trees, and generated staging trees use temporary files
and are removed on success or failure. The completed `cmd/fields/v<network>`
snapshot is retained locally for offline verification and is gitignored. A
failed build never replaces the last complete snapshot; a failed generation
never publishes a partial tracked tree. If a temporary file was removed or a
download was interrupted, rerun the same pinned selector. That redownload is
intentional: partial downloads are never trusted or resumed as verified input.

## Validation and review gates

Every archive boundary rejects traversal, absolute paths, unsafe entry types,
duplicates, descriptor digest or size mismatches, decompression-limit
violations, and ambiguous platform manifests. Extracted JSON is scanned before
code generation. The scanner enforces known metadata shapes, rejects credential
material and suspicious opaque values, and reports schema failures with their
JSON Pointer path.

Ubiquiti's sensitivity metadata is a privacy/sanitization inventory, not a
Terraform policy. The committed `cmd/fields/sensitive-policy.json` approves one
canonical metadata digest and exact reviewed paths:

- generated secrets such as passwords, PINs, private keys, pre-shared keys,
  tokens, RADIUS secrets, and authentication material become Terraform
  `Sensitive` leaf attributes;
- genuine secrets that the generator skips or that exist only in vendor
  metadata are recorded as non-generated secrets and must remain non-generated;
- names, email addresses, usernames, hostnames, IPs, serial/SIM identifiers,
  public certificates, and public keys remain visible private metadata.

The current review marks 28 generated paths and 36 non-generated paths as
secret. It deliberately leaves values such as `networkconf.name`, public
certificate fields, and `networkconf.x_dh_key` visible. The hand-written
provider `password` and `api_key` attributes are independently marked
Sensitive. Terraform sensitivity masks values in CLI and UI display only: it
does not encrypt, redact, or remove them from state. Protect remote state,
backups, logs, and access to state just as you would any other credential store.

A new metadata digest, a secret changing generated status, a new extracted
input shape, or a changed notice digest stops generation for maintainer review.

## Notices and redistribution boundary

The extractor records a digest for the relevant license/notice entries it
captures directly from `ace.jar` and `internal-dependencies.jar`. For the
reviewed 10.4.57 snapshot, that direct set is empty and its `NoticeDigest` is
`2caaba0bb439038643b99decb6f1c5bcdd0179a1885190685e796ac0dbfaebe5`.
This truthfully describes the extractor's reviewed direct set; it is not proof
of a complete license inventory for every dependency nested under
`ace.jar/BOOT-INF/lib`.

The local review separately checked all 153 nested dependency JARs. Sixty-one
JARs contained 111 matching entries (65 LICENSE-family and 46 NOTICE-family
entries). Their sorted `jar name + entry name + content SHA-256` inventory has
SHA-256 `72c3399fadddb2fcb513b2ec6c4dfbd1cefee05e0fe465c1fe48be82b8fcc3d2`.
Those bodies and the raw inventory remain local and vendor-governed; they are
not committed or redistributed. See [the licensing boundary](../LICENSES/README.md).

## Verification and recovery

The verification modes are selector-free and never publish output:

```sh
# Fresh checkout: compare committed generated files with schema-source.json
go run ./cmd/fields -verify-committed

# Machine with the matching local snapshot: regenerate into a temporary tree
go run ./cmd/fields -verify-regeneration
```

Both modes report the first differing path. For a reviewed update, run the
local regeneration check twice, `go test ./...`, `go vet ./...`, and
`git diff --check`. If review fails, keep the prior tracked generated tree and
the last complete snapshot. Re-run from the pinned version or verified local
installer after correcting policy; do not copy a partially generated staging
tree into place.

Scheduled update, automatic merge, and automatic release controls remain
disabled until the blocking CI, compatibility classification, GitHub App, and
repository release opt-in have been reviewed and configured. Local generation
does not authorize an automated release.
