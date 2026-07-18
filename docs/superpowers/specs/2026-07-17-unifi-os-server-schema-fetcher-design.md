# UniFi OS Server schema fetcher — design

Date: 2026-07-17
Status: approved

## Problem

`cmd/fields` generates the `unifi` package from JSON field definitions shipped
with the UniFi Network application. Its only download source is the legacy
`unifi-controller` Debian package, capped at `<10.0.0`. Ubiquiti no longer
publishes Network ≥10 as a deb; it ships inside the **UniFi OS Server**
installer (a self-extracting ELF with an appended zip containing an OCI image).
The field definitions moved too: they now live in
`BOOT-INF/lib/internal-dependencies.jar` inside the Spring Boot fat jar
`ace.jar`, alongside extra top-level metadata files (`sensitive_metadata.json`
etc.) that the current extractor ignores.

A working proof of concept exists (Python,
`terraform-provider-unifi/.../scripts/extract_unifi_api_defs.py`). This design
ports it into `cmd/fields` in Go, wires the extra metadata into
`specification.json` generation, and automates regeneration + release in
GitHub Actions.

Verified facts (checked against the live fw-update API, 2026-07-17):

- Product `unifi-os-server`, platform `linux-x64`, channel `release` is
  published on `fw-update.ubnt.com`; latest at time of writing is v5.1.21,
  bundling **UniFi Network 10.4.57** (`product.properties` in `ace.jar`).
- `GET /api/firmware-latest` (filtered) returns the current release;
  `GET /api/firmware` (filtered by product+platform) returns version history,
  so explicit-version lookups work without a hardcoded URL.
- The API response carries `sha256_checksum` and `file_size` for the download.
- The `api/fields/*.json` validator format is unchanged from the deb era, and
  `Setting.json` still exists and still needs splitting.
- v2-only resources (`FirewallPolicy`, `Nat`, `FirewallZone`, `OSPFRouter`,
  `DnsRecord`, `BgpConfig`, `TrafficRoute`) remain absent from upstream defs;
  `cmd/fields/custom/*.json` stays.

## Decisions

- **Approach C:** keep the existing deb path for explicit old versions; add the
  installer path alongside. The deb-specific `latestUnifiVersion()` lookup
  becomes dead code and is deleted; `downloadJar`/`extractJSON` stay.
- **`-latest` switches to the installer path.** Deb latest is frozen <10, so
  "latest" only has meaning on the OS Server line.
- **Version semantics:** `UnifiVersion` remains the *Network* version
  (e.g. 10.4.57), read from `product.properties`. A new
  `UnifiOsServerVersion` const records the installer version (empty for
  deb-sourced runs).
- **Sensitive marking feeds the TF spec codegen** (`specification.json`), with
  an explicit allowlist policy (below). Go structs are untouched.
- **Full automation:** cron → regenerate → PR → auto-merge on green CI →
  auto-tag → goreleaser. Uses a GitHub App token.

## CLI surface

Four mutually exclusive source modes:

```
fields <network-version>        # existing deb path, unchanged
fields -latest                  # latest unifi-os-server release (installer path)
fields -os-server 5.1.21        # specific OS Server release via /api/firmware
fields -url <url>               # direct installer URL
fields -installer <path>        # local installer file, no download
```

Existing flags unchanged: `-output-dir`, `-download-only`, `-generate-spec`,
`-spec-output`. The `go:generate` directive in `unifi/unifi.go` gains
`-generate-spec -spec-output ../specification.json` so one run regenerates code
and spec (today the spec is a separate manual step).

## Installer extraction pipeline

New file `cmd/fields/installer.go`. Streaming throughout; nothing installer- or
image-sized is held in memory. Peak temp disk ~4 GB (installer + extracted
image + ace.jar); fine locally and on `ubuntu-latest` runners.

1. **Resolve to a local file.** URLs download to a temp file; when the source
   is the fw-update API, verify `sha256_checksum`.
2. **Find the appended zip.** Scan the head of the file in overlapping 64 KB
   chunks for `PK\x03\x04`; open with `archive/zip` over an `io.SectionReader`.
3. **Extract `image.tar`** from the zip to a temp dir.
4. **Parse the OCI layout with
   `github.com/google/go-containerregistry/pkg/v1/layout`:**
   `layout.ImageIndexFromPath` → index → image → `Layers()` →
   `layer.Uncompressed()` per layer. This replaces the PoC's brute-force
   blob scanning and gzip sniffing with spec-correct parsing, and handles
   zstd layers transparently (stdlib has no zstd). Walk each layer's tar
   stream (stdlib `archive/tar`) for `usr/lib/unifi/lib/ace.jar`.
5. **Spool `ace.jar` (~116 MB) to a temp file**, open with `archive/zip`, pull
   `BOOT-INF/lib/internal-dependencies.jar` (fallback: any entry whose name
   contains `internal-dependencies`).
6. **Extract defs:**
   - `api/fields/*.json` → **flattened** into a staging dir root (the layout
     `main.go` already consumes).
   - Extra top-level JSONs → `metadata/` subdir, so `main.go`'s flat `*.json`
     scan ignores them: `sensitive_metadata.json`, `event_defs.json`,
     `legacy_endpoint_segments.json`, `radio_specification.json`,
     `country_codes_list.json`, `geo_ip_country_codes_list.json`,
     `timezones.json`, `ssl-inspection-file-extension.json`.
7. **Read Network version** from `BOOT-INF/classes/product.properties`
   (`version=…`).
8. **Publish the fields dir.** Staging dir is renamed to
   `cmd/fields/v<network-version>` only on success; staging is deleted on any
   failure so a partial cache is never reused. Write `source.json` into the
   fields dir: `{os_server_version, network_version, url, sha256}`.
9. **Cache lookup** for `-latest`/`-os-server`: query the API (cheap), then
   scan existing `v*/source.json` for a matching `os_server_version`. Hit →
   reuse, no 880 MB download. Miss → full pipeline.

After extraction both source modes converge on the existing shared
post-processing (`Setting.json` split, `custom/*.json` copy) and codegen.

Dependency note: `go-containerregistry` lands in the root `go.mod`. The repo
already keeps tool deps there (`terraform-plugin-codegen-spec`, sprig), so this
follows the existing pattern.

## Extra metadata files

All eight extra files are extracted to `<fieldsDir>/metadata/`. Only
`sensitive_metadata.json` drives codegen in this iteration; the rest are
available for future use (event defs, endpoint segments, etc.).

## Sensitive marking

### Policy

A field is marked `Sensitive` in `specification.json` iff:

1. upstream lists it in `sensitive_db_fields_by_collection[collection]`, **and**
2. it is not in the explicit display allowlist below.

Fail-safe direction: anything new upstream adds is treated as a secret until a
human adds it to the allowlist. Over-hiding is a UX bug; under-hiding is a
credential leak.

The allowlist is the entire policy surface, one commented map in
`cmd/fields/sensitive.go`:

```go
// UniFi's sensitive_db_fields mixes credential protection with PII
// redaction. These are identifiers/display metadata in Terraform terms;
// marking them Sensitive would hide resource names from plan output
// with no security benefit. Everything else upstream lists is treated
// as a secret.
var sensitiveDisplayFields = map[string]bool{
	// names & descriptions
	"name": true, "desc": true, "hostname": true, "host_name": true,
	"domain_name": true, "networkgroup": true,
	// identity / PII
	"email": true, "first_name": true, "last_name": true,
	"ubic_name": true, "ubic_uuid": true,
	"anonymous_id": true, "anonymous_device_id": true, "serial": true,
	// usernames & endpoints (the secrets are the passwords, not these)
	"login": true, "wan_username": true, "openvpn_username": true,
	"x_ssh_username": true, "lte_username": true,
	"management_ip": true, "management_peer_ip": true,
	"ipsec_key_exchange": true,
	// device radio identifiers
	"lte_imei": true, "lte_iccid": true, "lte_apn": true,
	"lte_networkoperator": true,
}
```

Escape hatch: keys are bare field names today; if a collision ever needs
per-collection granularity, keys become `"collection.field"`. Not built now.

### Nested fields

Upstream uses dot-paths for nesting: `radiusprofile` lists
`auth_servers.x_secret` (`auth_servers` is an array of objects → a
`ListNestedAttribute` in the spec). Implementation: after `processJSON` builds
the `FieldInfo` tree, walk each dot-path by JSON name — descending through
arrays and objects — and set `Sensitive = true` on the leaf `FieldInfo`.
Generic recursion; nothing assumes depth 2.

`schema.go` sets `Sensitive: true` on generated attributes wherever
`field.Sensitive` is set — resource and data-source attribute paths, any
nesting depth.

### Collection mapping

Collection key = lowercase basename of the source fields file
(`NetworkConf.json` → `networkconf`). Exceptions: all `Setting*` resources →
`setting`; `Site` → `site`; aliases `Client` → `user`, `ClientGroup` →
`usergroup`.

### Drift tolerance

Structural mismatches never fail codegen; they log an audit line and continue:

- collection with no generated resource (`admin`, `rogue`, `teleport_*`,
  `ssl_inspection_certificate`),
- field path not present in our schema (e.g. `networkconf.networkgroup`),
- `metadata/sensitive_metadata.json` absent (deb-sourced fields dirs).

Ignored metadata keys: `min_field_size`, `default_names`,
`sensitive_system_properties`, `sensitive_distinct_db_fields_by_collection`.

Scope boundary: marking applies to spec generation only. No String()-redaction
or struct tag changes in the client (deliberate non-goal).

## Automation (GitHub Actions)

### `generate.yaml` (modified)

1. Cron daily + `workflow_dispatch`.
2. **Pre-check (cheap):** query fw-update API for latest
   `unifi-os-server`/`linux-x64` version; compare to `UnifiOsServerVersion` in
   the committed `unifi/version.generated.go`. Equal → exit 0. This avoids an
   880 MB download on days nothing changed.
3. `go generate ./...` (full pipeline: download, extract, codegen, spec).
4. Create a **GitHub App token** (`actions/create-github-app-token`) and use it
   for the PR. A `GITHUB_TOKEN`-created PR never triggers CI, which would make
   gated auto-merge impossible.
5. `peter-evans/create-pull-request` → branch `auto/unifi-<network-version>`,
   title/body carry OS Server + Network versions.
6. `gh pr merge --auto --squash`.

### `release-on-merge.yaml` (new)

On push to `main`: if the merged commit touched `unifi/*.generated.go` or
`specification.json` → bump minor from the latest `v*` tag → push the tag with
the App token (so the push triggers workflows) → existing `release.yaml`
(goreleaser) fires unchanged. A `concurrency` guard prevents tag races.

### `ci.yaml` (hardened)

- **Remove `continue-on-error: true` from the test job.** Today every check is
  advisory; with auto-merge that means red PRs merge. Tests must be a real
  gate. Lint/generate may stay advisory.
- Skip the `generate` job on `auto/*` PR branches — re-running `go generate`
  there is a second 880 MB download for zero information (the PR content was
  produced by the same tool minutes earlier).

### Manual prerequisites (repo owner)

- Create a GitHub App with `contents: write` + `pull_requests: write` on this
  repo; install it; add `APP_ID` and `APP_PRIVATE_KEY` secrets.
- Enable **Allow auto-merge** in repo settings.

## Error handling

- SHA-256 mismatch on API-sourced downloads → hard fail.
- Each pipeline stage fails with context (`ace.jar not found in any image
  layer`, `internal-dependencies.jar not in ace.jar`, …).
- Partial extraction never reaches the final fields dir (staging + rename).
- Sensitive-metadata drift logs and continues (above).
- Existing behavior unchanged: version fields dir present → skip download.

## Testing

- **Synthetic installer fixture (no network):** build a mini-installer in test
  code — dummy ELF bytes + appended zip containing a small OCI-layout
  `image.tar` with a tiny `ace.jar` → assert end-to-end extraction: right
  JSONs out, version read from `product.properties`, `api/fields` flat,
  extras under `metadata/`. Plus zip-offset edge cases.
- **Sensitive marking:** fixture metadata + a RadiusProfile-like resource →
  nested leaf flagged through a list-nested attribute; allowlisted fields not
  flagged; unknown collection / unknown path segment log-and-skip; `Setting*`
  mapping.
- Existing tests (`version_test`, `main_test`, `validator_test`,
  `schema_test`) keep passing; deb path behavior unchanged.
- **Real-world verification before PR:** run
  `-installer <path> -download-only` against the actual 5.1.21 installer and
  diff the extracted defs byte-for-byte against the PoC's
  `schemas/unifi-network/10.4.57/` output (already on disk).

## Non-goals

- Deleting the deb download/extract path (approach A — rejected for now).
- Codegen from the other metadata files (event defs, radio specs, …).
- Separate Go module for the codegen tool.
- arm64/macOS/Windows installer variants (same defs; x64 only).
- Auto-merging anything other than regeneration PRs.
