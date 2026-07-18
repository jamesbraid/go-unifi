# UniFi OS Server Schema Fetcher Design

## Summary

Extend the existing Go schema fetcher and Internal API generator so it can rebuild
`go-unifi` from the API definitions bundled with current UniFi OS Server releases.
The new path reads the nested `internal-dependencies.jar` found in UniFi Network
10.x, makes supporting metadata such as `sensitive_metadata.json` available to
the generator, and leaves the existing Internal API code-generation architecture
intact. Secret-bearing attributes are marked sensitive in the generated Terraform
specification from the first release. Vendor installers, JARs, raw schemas, and raw
metadata remain untracked and are never redistributed by the project.

This change does not adopt or generate the Official UniFi OpenAPI surface. It does
not migrate the provider or SDK to filipowm's architecture.

## Goals

- Discover the latest or a pinned UniFi OS Server Linux x64 release through
  Ubiquiti's machine-readable firmware endpoint.
- Accept an explicit official installer URL or a previously downloaded local
  installer when automatic discovery is unavailable or undesirable.
- Extract the bundled UniFi Network version, `api/fields/*.json`, and supporting
  metadata from the newer nested archive layout.
- Publish inputs in the flat layout the existing generator consumes, with custom
  definitions overlaid exactly as they are today.
- Preserve every Ubiquiti sensitivity classification for coverage, while marking
  only explicitly reviewed secret-bearing attributes as Terraform-sensitive.
- Preserve reproducible generation from a pinned official installer and its
  committed provenance hashes, while keeping ordinary unit tests fully offline.
- Automate schema-update pull requests and patch releases for backward-compatible
  generated API changes.
- Fail safely: snapshot publication is atomic, later failures retain the complete
  new snapshot, and tracked release output remains last-known-good.

## Non-goals

- Generating clients from `integration.json` or any other Official API schema.
- Migrating to `filipowm/go-unifi` or `filipowm/terraform-provider-unifi`.
- Runtime request, response, or log redaction outside the generated Terraform
  specification.
- Scraping the Ubiquiti Community site or the client-rendered downloads page.
- Supporting macOS, Windows, or Linux ARM installers in the first implementation.
- Committing or releasing verbatim Ubiquiti schemas, metadata, installers, or JARs.

## Source Resolution

The existing controller-package commands remain valid:

```sh
go run ./cmd/fields -latest
go run ./cmd/fields 9.5.21
```

The following mutually exclusive source selectors are added:

```sh
go run ./cmd/fields -uos-latest
go run ./cmd/fields -uos-version 5.1.21
go run ./cmd/fields -installer /path/to/unifi-os-server
go run ./cmd/fields -installer-url https://fw-download.ubnt.com/data/unifi-os-server/...
```

Existing output and generation options, including `-download-only`, `-output-dir`,
and specification generation, apply to every source.

### Firmware resolver

Latest discovery uses:

```text
GET https://fw-update.ui.com/api/firmware-latest
  ?filter=eq~~product~~unifi-os-server
  &filter=eq~~platform~~linux-x64
  &filter=eq~~channel~~release
```

A pinned release adds `filter=eq~~version~~v<version>`. The resolver consumes
`_links.data.href`, never the temporary signed URL reached through the misleadingly
named `_links.upload` relation.

The endpoint is operated by Ubiquiti and is already the basis of the legacy
fetcher's release discovery, but its filter contract is undocumented. It therefore
lives behind a small resolver interface and is not the only supported input path.
Responses must contain exactly one entry matching the requested product, platform,
channel, and optional version. Empty, multiple, or mismatched results are errors.

Explicit installer URLs must use HTTPS and target `ui.com`, `ubnt.com`, or a
subdomain of either. Other sources must be downloaded separately and supplied with
`-installer`.

## Components

### Source resolver

Resolves a selector into an installer description containing the source kind,
UniFi OS Server version when known, URL or local path, expected byte length,
SHA-256 checksum, optional MD5 checksum, firmware record ID, and release timestamps.
Local installers may omit facts that cannot be established before extraction.

### Installer materializer

Returns a local seekable installer file. Remote installers are downloaded into a
temporary file with bounded HTTP timeouts. When the firmware API supplied size and
SHA-256 values, both are verified before extraction. A failed download or checksum
never reaches the extraction stage.

### UniFi OS Server extractor

Traverses the following archive chain:

```text
installer ELF
  -> appended ZIP
  -> image.tar
  -> OCI layer
  -> /usr/lib/unifi/lib/ace.jar
  -> BOOT-INF/lib/internal-dependencies.jar
```

The installer is kept on disk because ZIP directory access requires a seekable
reader. Go's `archive/zip` supports ZIP data prefixed by a self-extracting executable,
so the extractor opens the complete installer directly instead of searching for a
`PK` byte signature.

The `image.tar` entry is streamed into a bounded temporary OCI layout. The extractor
parses `index.json`, selects the Linux x64 manifest, verifies descriptor sizes and
digests, and processes layers in reverse overlay order while respecting OCI
whiteouts. This avoids selecting a stale or deleted `ace.jar` from an older layer.
`ace.jar` and the nested JAR are spooled to bounded temporary files so neither the
880 MB installer nor its image layers are loaded into memory as whole byte slices.

The extractor reads `BOOT-INF/classes/product.properties` from `ace.jar` and uses
its `version` value as the UniFi Network version. Output directories are keyed by
this Network version, such as `v10.4.57`, not by the enclosing OS Server version,
such as `v5.1.21`.

All archive entry names are treated as untrusted. Extraction rejects traversal,
absolute paths, links, unexpected entry types, oversized JARs, oversized JSON
entries, truncated archives, and decompression-limit violations.

### Snapshot builder

The builder stages a complete local version directory beside the final path. It:

1. Flattens `api/fields/*.json` into the existing version-directory layout.
2. Preserves the original `Setting.json` and creates the per-setting files using
   the existing splitting behavior.
3. Overlays `cmd/fields/custom/*.json` after upstream extraction.
4. Writes supporting artifacts beneath `metadata/`.
5. Creates a deterministic local extraction manifest.
6. Validates the staged tree.
7. Atomically renames the staged directory into place.

The old local snapshot remains untouched until the staged snapshot itself is
complete and atomically published. That publication is independent of later scan,
policy, and generation steps: a complete new snapshot remains when any of those
steps fails, and the previous version-directory snapshot has already been replaced.
Interrupted or failed extraction removes its temporary tree; successful extraction
is removed when the run returns after snapshot construction has consumed it. Version
directories remain covered by `cmd/fields/.gitignore` and are never included in
commits, Actions artifacts, GitHub releases, or Go module source archives.

### Existing generator

The generator continues reading top-level JSON files from the local, gitignored
`cmd/fields/v<network-version>/`. It ignores the `metadata/` directory naturally and
does not need an Official API pass. A local regenerate is offline after extraction;
CI reproduction may re-download the exact pinned installer from Ubiquiti and verify
it against the committed size and SHA-256.

Generated output is limited to facts necessary for interoperability: API and wire
names, primitive and container types, routes, required/optional behavior, and
functional validation constraints, including whether a Terraform attribute contains
a secret. It does not copy vendor prose, examples, messages, metadata ordering, or
unrelated internal datasets.

The generated-tree digest covers only files the generator owns and overwrites on
every run: `unifi/**/*.generated.go`, `unifi/version.generated.go`, and
`specification.json`. One-time `.go` implementation scaffolds are hand-maintained
after creation and are excluded from reproducibility comparisons.

### Sensitivity policy and specification generation

Ubiquiti's `sensitive_metadata.json` is a privacy and sanitization classification,
not a direct Terraform schema policy. It includes both secret-bearing values and
ordinary private metadata. Mapping every classified value to Terraform `Sensitive`
would hide routine values such as names and descriptions in plans and propagate
sensitivity into expressions and outputs.

The generator therefore uses two classifications:

- **Secret:** passwords, passphrases, private keys, pre-shared keys, authentication
  keys and tokens, RADIUS shared secrets, certificate private material, and PINs.
  The corresponding generated Terraform attribute has `Sensitive: true`.
- **Private metadata:** names, descriptions, email addresses, hostnames, usernames,
  serial numbers, and other identifiers that should be sanitized in diagnostics or
  fixtures but are useful in an ordinary Terraform plan. These remain visible in
  Terraform and are not marked `Sensitive`.

Classification is by exact collection and dotted JSON path, including paths such as
`radiusprofile.auth_servers.x_secret`; it is never inferred solely from a field-name
substring. Only the leaf attribute is marked sensitive, so a nested object remains
readable while the secret value is masked.

The repository commits a minimal provider-owned policy containing the approved
generated secret paths, reviewed non-generated secret paths, and the digest of each
reviewed canonical sensitivity dataset. It does not commit the raw Ubiquiti metadata
or enumerate private metadata. During local or CI generation, the policy mapper:

1. Parses every Ubiquiti classification and checks it against the raw upstream
   schema set before settings are split, collections are skipped, or custom files
   are overlaid.
2. Records whether each path maps to a generated field or to a classified
   non-generated collection; malformed and ambiguous paths are errors.
3. Applies `Sensitive: true` to exact generated-secret matches, at any nesting
   depth, and fails unless each lands on one generated leaf.
4. Requires every reviewed non-generated secret to remain non-generated. A path
   moving between generated and non-generated states blocks generation until the
   policy is reviewed, preventing silent loss of masking or stale classifications.
5. Treats the remaining classified paths as reviewed private metadata only when the
   canonical metadata digest has already been approved.
6. Fails generation when the sensitivity dataset digest is new, requiring review
   and a policy update before automated merge or release.

Provider credentials that are not sourced from Ubiquiti metadata, including the
existing provider `password` and `api_key` attributes, remain explicitly sensitive.
Terraform sensitivity masks values in CLI and UI output but does not remove or
encrypt them in state; the documentation continues to require protected state
storage.

## Go Library Selection

Use the Go standard library for all archive and compression boundaries:

- `archive/zip` for the prefixed installer ZIP, `ace.jar`, and the nested JAR.
- `archive/tar` for `image.tar` and OCI filesystem layers.
- `compress/gzip` for OCI gzip layers.
- `io`, `os`, and bounded temporary files for streaming and seekable intermediates.

Use `github.com/opencontainers/image-spec/specs-go/v1` for OCI index, manifest,
descriptor, platform, and media-type definitions, plus
`github.com/opencontainers/go-digest` for descriptor verification. Both dependencies
are Apache-2.0 licensed and remain separate works compatible with this MPL-2.0
project.

Do not add a full registry or container-runtime stack initially. The OCI resolver is
kept behind an internal interface so `go-containerregistry` can replace it later if
format or platform variability makes that worthwhile. Add a focused zstd decoder
only if a real release begins using OCI zstd layers.

## Local Extraction Layout

```text
cmd/fields/v10.4.57/
  Account.json
  Device.json
  NetworkConf.json
  Setting.json
  SettingAutoSpeedtest.json
  ...
  metadata/
    source.json
    sensitive_metadata.json
    legacy_endpoint_segments.json
    event_defs.json
    radio_specification.json
    country_codes_list.json
    geo_ip_country_codes_list.json
    timezones.json
    ssl-inspection-file-extension.json
```

This entire version directory is gitignored. `metadata/source.json` records only
stable source facts:

- UniFi OS Server version.
- Bundled UniFi Network version.
- Product, platform, channel, and firmware record ID.
- Installer URL, byte length, SHA-256, and optional MD5.
- Ubiquiti's release creation and update timestamps.
- SHA-256 hashes of extracted schemas and metadata artifacts.
- A digest of the canonical schema set used to decide whether generation changed.
- The canonical sensitivity-metadata digest and approved policy version.
- A digest of the relevant bundled third-party license and notice inventory.
- Names of known optional metadata artifacts absent from the source.

It contains no generation timestamp or machine-local path.

The repository separately commits `cmd/fields/schema-source.json`, a small
provenance record containing the selected OS Server and Network versions, official
installer URL, firmware ID, byte length, SHA-256, release timestamps, canonical
schema digest, and generated-tree digest. It contains no raw schema content or local
paths. It also records the sensitivity-metadata digest and policy version without
listing Ubiquiti's private-metadata paths. This file lets scheduled automation detect
a new installer without repeatedly downloading the current 880 MB artifact. It also
records the reviewed license/notice digest without copying the notices.

## Extraction Contract and Failure Behavior

The following are required:

- A readable `image.tar` payload.
- An OCI layer containing `usr/lib/unifi/lib/ace.jar`.
- `BOOT-INF/classes/product.properties` with a non-empty Network version.
- `BOOT-INF/lib/internal-dependencies.jar`.
- `api/fields/Setting.json` and at least one additional field definition.
- `sensitive_metadata.json`.

The other known metadata artifacts are retained when present. Their absence is
recorded in the manifest and reported as a warning but does not block generation.
Every sensitivity path must resolve uniquely, and the sensitivity-metadata digest
must be present in the approved policy. A new or malformed dataset blocks generation
rather than risking an unmasked secret.

Errors identify the failed boundary and source, for example firmware resolution,
download verification, appended ZIP, OCI image, layer, `ace.jar`, nested JAR,
required artifact validation, custom overlay, or atomic publication. Automation
must keep the last known-good tracked release when any boundary fails. A failure
before atomic snapshot publication leaves the prior snapshot intact; a later
failure leaves the newly published complete snapshot as the canonical review input.

## Automated Regeneration

The scheduled GitHub Actions workflow runs daily and also supports manual dispatch
with an optional pinned OS Server version or explicit installer URL.

1. Query the filtered firmware endpoint.
2. Compare the returned installer SHA-256 with committed
   `cmd/fields/schema-source.json`.
3. Stop without downloading when it is unchanged.
4. Download, verify, extract, and stage a changed installer.
5. Compare the canonical schema, sensitivity-metadata, and relevant third-party
   license/notice digests.
6. If the sensitivity-metadata digest is not in the approved policy or the notice
   digest has not been reviewed, stop with an actionable review error before
   changing the committed manifest or generated tree. A maintainer reviews the
   local inputs, updates the approvals, and reruns the workflow.
7. If only provenance changed, update the small committed manifest without
   releasing or publishing extracted artifacts.
8. If schemas changed, regenerate the Internal API code and Terraform specification.
9. Run formatting, full tests, lint, pinned-source regeneration, generated-tree
   digest verification, and exported API compatibility checks.
10. Open or update one stable schema-update pull request.

The workflow uses the same Go entry point as local development. There is no
workflow-only extractor. It deletes the downloaded installer and extracted tree at
the end and never uploads either as an Actions artifact.

Current advisory CI is insufficient for automation. `continue-on-error: true` is
removed from generation, test, and lint jobs, and those jobs become required branch
checks. Ordinary pull-request CI remains offline: it hashes the committed
generator-owned files and compares them with the committed generated-tree digest.
Full regenerate-and-compare runs only when a local snapshot exists, including the
scheduled updater after verified extraction.

### Authentication and merge

The updater uses a narrowly scoped GitHub App installation token with only the
repository permissions necessary to update a branch and pull request. This avoids
the manual workflow approval GitHub applies to pull requests created with the
ordinary `GITHUB_TOKEN`.

The schema-update pull request enables auto-merge only when all required checks pass
and the exported Go API comparison classifies the change as backward compatible.
Repeated scheduled runs update the same branch and pull request rather than opening
duplicates.

Unattended auto-merge and release additionally require an explicit repository policy
setting, such as `ALLOW_AUTOMATED_SCHEMA_RELEASES=true`. Without it, automation opens
or updates the verified pull request but leaves merge and release to a maintainer.

### Release policy

- Backward-compatible generated API change: when the repository policy setting is
  enabled, merge automatically and publish the next patch release; otherwise wait
  for a maintainer.
- Provenance or metadata-only change: merge without a Go module release.
- New sensitivity-metadata digest: publish no changes until a maintainer reviews the
  extracted metadata and updates the policy, even when the exported Go API would
  otherwise remain compatible.
- Removed exported symbol, changed exported type/signature, or ambiguous API change:
  leave the pull request open for manual review and an explicit version decision.
- Extraction or verification failure: publish nothing and surface a failed workflow
  with an actionable error.

After a compatible schema pull request merges, a trusted workflow creates the next
patch tag using the GitHub App token. The existing tag-triggered GoReleaser workflow
then publishes the release. Ordinary pushes to `main` do not create releases.

## Licensing and Redistribution Boundary

This design supports interoperability, not redistribution of Ubiquiti software or
content. Ubiquiti's current Terms and EULA restrict redistribution and reverse
engineering, while Canadian and US interoperability exceptions are narrow and do
not clearly authorize publishing complete vendor datasets. This is a risk-management
boundary, not a legal conclusion.

- Installers are downloaded only from Ubiquiti and are never mirrored.
- Installers, JARs, raw `api/fields` JSON, and ancillary metadata remain local or
  ephemeral and are never committed, uploaded, or released.
- `sensitive_metadata.json` is consumed locally. Only the minimal reviewed secret
  paths, approved dataset digests, and generated `Sensitive` flags necessary for
  provider behavior are committed; the source file is not published.
- Automated discovery runs at most daily, avoids a download when the checksum is
  unchanged, uses an identifying User-Agent with project contact information, and
  honors errors and rate limits.
- Generated output contains only the minimal functional interface facts necessary
  for an independent client to interoperate.
- A repository notice states that Ubiquiti artifacts remain Ubiquiti's, are not
  covered by the repository's MPL license, and the project is unofficial and not
  affiliated with or endorsed by Ubiquiti.
- The workflow scans extracted inputs for unexpected secrets or customer-derived
  data before generation and fails rather than publishing suspicious output.
- Third-party notices bundled in each installer are inspected before a newly
  discovered dataset is consumed by generation.

Public redistribution of raw definitions or metadata requires separate permission
or legal review. Maintainers may keep automatic publication disabled until they are
comfortable with the minimized-output boundary or have obtained advice or consent.

Relevant primary references include Ubiquiti's [general Terms](https://www.ui.com/legal/),
[software EULA](https://www.ui.com/eula/?direct=true), and
[UniFi OS Server terms](https://www.ui.com/legal/unifi-os-server/?direct=true), plus
the Canadian Copyright Act interoperability provisions in
[section 30.61](https://laws-lois.justice.gc.ca/eng/acts/C-42/section-30.61.html) and
[section 41.12](https://laws-lois.justice.gc.ca/eng/acts/C-42/section-41.12.html).
These links document the design inputs and are not a substitute for legal advice.

## Testing

Ordinary tests use small synthetic archives created in Go. No large proprietary
installer is committed to the repository.

### Resolver tests

- Latest and pinned release queries.
- URL encoding and exact repeated filters.
- Empty, multiple, and mismatched firmware results.
- HTTP status, malformed JSON, timeout, and cancellation failures.
- HTTPS and Ubiquiti-host validation for explicit URLs.

### Materializer tests

- Successful local and HTTP inputs.
- Expected-size mismatch.
- SHA-256 mismatch.
- Interrupted and truncated downloads.
- Temporary-file cleanup.

### Extractor tests

- A complete synthetic appended ZIP, `image.tar`, OCI layer, `ace.jar`, and nested
  `internal-dependencies.jar` chain.
- Compressed and uncompressed OCI layers encountered in arbitrary order.
- Missing image, layer, `ace.jar`, product version, nested JAR, fields, or sensitive
  metadata.
- Path traversal, links, unexpected entry types, and configured size limits.
- Correct Network-version discovery independent of the OS Server version.

### Snapshot and generation tests

- Flattened field definitions and split settings.
- Custom definitions override upstream definitions.
- Local metadata layout, stable local manifest hashes, and the minimized committed
  provenance manifest.
- Optional metadata omissions are recorded deterministically.
- Top-level and nested exact-path sensitivity mappings mark only secret leaves in
  the generated specification.
- Private metadata remains visible, while existing provider credentials remain
  sensitive.
- Malformed or ambiguous sensitivity paths fail; classified paths in skipped,
  split, or non-generated collections are recorded in coverage.
- Every generated-secret path must land on exactly one generated leaf, and every
  reviewed non-generated secret must remain non-generated.
- A new sensitivity-metadata digest fails until its policy is reviewed and approved.
- The metadata mapper covers every classified path without relying on substring
  inference.
- Atomic snapshot publication, including prior-snapshot restoration when the
  publication operation itself fails and retention of the new complete snapshot
  when a later boundary fails.
- End-to-end generation from a tiny synthetic installer.
- Two consecutive generations from the same pinned installer produce byte-identical
  generated output and matching digests.
- Exported API compatibility classification distinguishes compatible additions from
  breaking removals and type changes.

### Live verification

The 880 MB installer is not downloaded in ordinary pull-request CI. Before the
implementation lands, a manual real-data pilot processes UniFi OS Server 5.1.21,
verifies its published checksum and size, extracts Network 10.4.57, regenerates the
SDK, reviews the resulting exported API diff, and confirms that no raw vendor
artifacts are tracked or uploaded. The scheduled updater then serves as the
continuing live integration path.

## Rollout

1. Land the source resolver, materializer, extractor, snapshot builder, and offline
   tests without changing the scheduled release policy.
2. Run the 5.1.21 real-data pilot, review the 10.4.57 sensitivity classifications,
   and commit only the minimal secret policy, generated output, minimized provenance
   manifest, and notice; keep the raw extraction and metadata local.
3. Make CI failures blocking and add deterministic generation and API compatibility
   checks.
4. Enable scheduled schema-update pull requests.
5. Configure the GitHub App, required checks, and auto-merge.
6. After one successful unattended update cycle, explicitly decide whether to set
   `ALLOW_AUTOMATED_SCHEMA_RELEASES=true`; otherwise retain maintainer approval.
