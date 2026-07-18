# UniFi OS Server Schema Fetcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the existing Internal API Go client and Terraform specification from a pinned or newly discovered UniFi OS Server installer, including reviewed leaf-level secret sensitivity, without publishing Ubiquiti's raw artifacts.

**Architecture:** Keep `cmd/fields` as the generator and add a second source pipeline: resolve and materialize an installer, traverse its prefixed ZIP and OCI image into the nested JAR, then atomically publish a gitignored local snapshot in the existing flat layout. A committed provenance manifest and minimal sensitivity policy make regeneration deterministic; automation may merge compatible updates, but unknown sensitivity metadata and breaking API changes stop for review.

**Tech Stack:** Go 1.25; Go archive, compression, I/O, HTTP, and crypto packages; `github.com/opencontainers/image-spec/specs-go/v1`; `github.com/opencontainers/go-digest`; HashiCorp Terraform plugin codegen specification; GitHub Actions and GoReleaser.

## Global Constraints

- Generate only the Internal API; never consume `integration.json` or generate the Official API.
- Keep all legacy Debian selectors working and add UOS latest, exact version, official URL, and local installer selectors.
- Support only UniFi OS Server `linux-x64` release installers initially.
- Permit explicit remote URLs only over HTTPS on `ui.com`, `ubnt.com`, or their subdomains.
- Verify API-provided size and SHA-256 before extraction.
- Keep installers, OCI blobs, JARs, raw fields, and raw metadata local or ephemeral and out of Git and Actions artifacts.
- Bound every archive boundary; reject traversal, links, duplicates, unexpected types, bad digests, and decompression-limit violations.
- Match sensitivity by exact collection and dotted JSON path; mark only secret leaves, never private metadata containers.
- An unknown sensitivity-metadata digest blocks generation before committed output changes.
- Ordinary tests are offline and use synthetic archives only.
- Automated compatible releases require all checks and `ALLOW_AUTOMATED_SCHEMA_RELEASES=true`.

---

### Task 1: Source resolution and verified materialization

**Files:**
- Create: `cmd/fields/source.go`
- Create: `cmd/fields/source_test.go`
- Create: `cmd/fields/materialize.go`
- Create: `cmd/fields/materialize_test.go`
- Modify: `cmd/fields/fwupdate.go`
- Modify: `cmd/fields/main.go`

**Interfaces:**
- Produces: `SourceSelector`, `InstallerSource`, `ParseSourceSelector`, `ResolveInstaller`, and `MaterializeInstaller`.
- Consumes: the existing firmware response model and legacy version resolver.

- [ ] **Step 1: Write failing selector and resolver tests**

Cover mutual exclusion among positional legacy version, `-latest`, `-uos-latest`, `-uos-version`, `-installer`, and `-installer-url`. With an `httptest.Server` response fixture containing the endpoint's size, SHA-256, and MD5 field names, assert repeated filters for product `unifi-os-server`, platform `linux-x64`, channel `release`, and optional version `v5.1.21`; require one exact result and `_links.data.href`. Test HTTPS host validation against valid Ubiquiti hosts, HTTP, deceptive suffixes, and unrelated hosts. Migrate the legacy resolver onto the injected endpoint rather than mutating the package-level `firmwareUpdateApi` in tests.

- [ ] **Step 2: Add the stable source types**

```go
type SourceKind string

const (
	SourceLegacyLatest SourceKind = "legacy-latest"
	SourceLegacyVersion SourceKind = "legacy-version"
	SourceUOSLatest SourceKind = "uos-latest"
	SourceUOSVersion SourceKind = "uos-version"
	SourceInstallerURL SourceKind = "installer-url"
	SourceInstallerFile SourceKind = "installer-file"
)

type SourceSelector struct { Kind SourceKind; Value string }

type InstallerSource struct {
	Kind SourceKind
	OSVersion, FirmwareID, Product, Platform, Channel string
	URL *url.URL
	LocalPath string
	ExpectedSize int64
	ExpectedSHA256, ExpectedMD5 string
	Created, Updated time.Time
}

func ParseSourceSelector(args []string) (SourceSelector, error)
func ResolveInstaller(ctx context.Context, client *http.Client, endpoint string, selector SourceSelector) (InstallerSource, error)
```

Change the legacy default endpoint from `fw-update.ubnt.com` to the design's `https://fw-update.ui.com/api/firmware-latest`, and keep it behind the `endpoint` argument for tests. Extend the firmware JSON model with the real size/checksum fields and reject incomplete or mismatched records. Resolver and materializer requests use one identifying User-Agent containing the project URL and contact address documented in `docs/schema-generation.md`.

- [ ] **Step 3: Write failing materializer tests**

Test local input, successful verified HTTP download, non-2xx, timeout, cancellation, declared-size mismatch, SHA-256 mismatch, partial-file cleanup, and `Close`. Assert the returned path remains seekable until closed and local inputs are never deleted.

- [ ] **Step 4: Implement bounded materialization**

```go
type MaterializedInstaller struct { Path string; Size int64; SHA256 string; temporary bool }
func MaterializeInstaller(ctx context.Context, client *http.Client, src InstallerSource, tempRoot string) (*MaterializedInstaller, error)
func (m *MaterializedInstaller) Close() error
```

Stream remote data through `io.MultiWriter(file, sha256.New())`, verify bytes and digest before returning, and remove partial temporary files on every failure. Hash local inputs without claiming an expected value was vendor-verified.

- [ ] **Step 5: Verify and commit**

Run: `go test ./cmd/fields -run 'Test(ParseSourceSelector|ResolveInstaller|ValidateInstallerURL|MaterializeInstaller|LatestUnifiVersion)' -count=1`

Expected: PASS.

Commit: `feat(fields): resolve and verify UniFi OS installers`

### Task 2: Safe OCI and nested archive extraction

**Files:**
- Create: `cmd/fields/archive_limits.go`
- Create: `cmd/fields/oci.go`
- Create: `cmd/fields/oci_test.go`
- Create: `cmd/fields/uos_extract.go`
- Create: `cmd/fields/uos_extract_test.go`
- Create: `cmd/fields/archive_fixture_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Consumes: a materialized installer path.
- Produces: `ExtractUOSInstaller` and `ExtractedDefinitions`.

- [ ] **Step 1: Add the two focused OCI dependencies**

Run: `go get github.com/opencontainers/image-spec@v1.1.1 github.com/opencontainers/go-digest@v1.0.0`

Expected: both become direct requirements; no registry or container-runtime stack is added.

- [ ] **Step 2: Build failing synthetic OCI tests**

Build tiny OCI layouts in test temporary directories. Cover Linux amd64 selection, nested indexes, gzip and uncompressed layers, descriptor size/digest mismatch, unsupported media types, traversal, links, duplicate entries, entry/byte limits, newest-layer selection, regular and opaque whiteouts.

- [ ] **Step 3: Implement bounded OCI import and lookup**

```go
type ArchiveLimits struct {
	MaxImageTarBytes, MaxBlobBytes, MaxLayerBytes, MaxJarBytes, MaxJSONBytes int64
	MaxEntries, MaxIndexDepth int
}

type OCILayout struct { Root string }
type ResolvedImage struct {
	Manifest v1.Manifest
	Blobs map[digest.Digest]string
}

func ImportOCI(r io.Reader, tempRoot string, limits ArchiveLimits) (*OCILayout, error)
func ResolveImage(layout *OCILayout, platform v1.Platform) (*ResolvedImage, error)
func FindFileInLayers(image *ResolvedImage, name string, limits ArchiveLimits) (*os.File, error)
```

Stage only `oci-layout`, `index.json`, and regular `blobs/<algorithm>/<digest>` entries. Validate paths before joining, count with `io.LimitedReader(max+1)`, verify every followed descriptor, select exactly one Linux amd64 manifest, and traverse layers newest-first with OCI whiteout semantics. Reject zstd explicitly until a real release requires it.

- [ ] **Step 4: Build a complete failing installer fixture**

Create an ELF-prefixed ZIP with `image.tar`; its OCI layer contains `usr/lib/unifi/lib/ace.jar`; `ace.jar` contains `BOOT-INF/classes/product.properties` and `BOOT-INF/lib/internal-dependencies.jar`; the nested JAR contains `api/fields/Setting.json`, a second field file, and `sensitive_metadata.json`.

- [ ] **Step 5: Implement nested extraction**

```go
type ExtractedArtifact struct { Name, Path, SHA256 string; Size int64 }
type ExtractedDefinitions struct {
	NetworkVersion string
	Fields, Metadata, Notices map[string]ExtractedArtifact
	MissingOptional []string
}

func ExtractUOSInstaller(ctx context.Context, installerPath, tempRoot string, limits ArchiveLimits) (*ExtractedDefinitions, error)
```

Open the complete installer with `zip.NewReader`, stream `image.tar` through the OCI reader, spool both JAR boundaries into bounded temporary files, parse the Network version, and copy only recursive `api/fields/*.json` plus the design's metadata allowlist. Inventory relevant `LICENSE*`, `NOTICE*`, and `META-INF` license/notice entries from both JARs into the local `Notices` map for review and hashing. Require `Setting.json`, another field file, and `sensitive_metadata.json`.

- [ ] **Step 6: Verify and commit**

Run: `go test ./cmd/fields -run 'Test(ImportOCI|ResolveImage|FindFileInLayers|ExtractUOSInstaller|SyntheticInstaller)' -count=1`

Expected: PASS, including all malicious archive fixtures.

Commit: `feat(fields): extract definitions from UniFi OS images`

### Task 3: Atomic snapshots and deterministic provenance

**Files:**
- Create: `cmd/fields/snapshot.go`
- Create: `cmd/fields/snapshot_test.go`
- Create: `cmd/fields/provenance.go`
- Create: `cmd/fields/provenance_test.go`
- Modify: `cmd/fields/.gitignore`

**Interfaces:**
- Consumes: resolved source, materialized installer, and extracted definitions.
- Produces: `BuildSnapshot`, `LocalManifest`, `SchemaSource`, and canonical digests.

- [ ] **Step 1: Write failing snapshot tests**

Test flattening `api/fields`, preserving and splitting `Setting.json`, custom definition precedence, metadata placement, stable hashes independent of map/filesystem order, deterministic missing-optionals, absence of timestamps/local paths, atomic replacement, and preservation of an existing snapshot after injected failures.

- [ ] **Step 2: Implement deterministic models**

```go
type LocalManifest struct {
	OSVersion, NetworkVersion, FirmwareID, InstallerURL, InstallerSHA256 string
	Product, Platform, Channel, SchemaDigest, SensitivityDigest, NoticeDigest, PolicyVersion string
	InstallerSize int64
	Created, Updated time.Time
	Artifacts map[string]string
	MissingOptional []string
}

type SchemaSource struct {
	OSVersion, NetworkVersion, FirmwareID, InstallerURL, InstallerSHA256 string
	SchemaDigest, SensitivityDigest, NoticeDigest, GeneratedTreeDigest, PolicyVersion string
	InstallerSize int64
	Created, Updated time.Time
}
```

Canonicalization sorts paths and object keys and normalizes line endings. Hash upstream schemas before custom overlays and sensitivity metadata separately. `GeneratedTreeDigest` covers only files overwritten on every run: `unifi/**/*.generated.go`, `unifi/version.generated.go`, and `specification.json`; it excludes one-time `.go` implementation scaffolds that become hand-maintained.

- [ ] **Step 3: Implement staged publication**

Build a sibling temporary directory, copy artifacts with mode `0644`, split settings using existing behavior, overlay `cmd/fields/custom`, validate, and write `metadata/source.json`. Publish with an explicit recoverable swap: rename current to a unique sibling backup, rename staged to final, restore the backup if the second rename fails, then remove the backup only after success. Failure-injection tests must prove restoration. Expand ignore patterns for staging, backup, installer, image, JAR, and raw metadata paths.

- [ ] **Step 4: Verify and commit**

Run: `go test ./cmd/fields -run 'Test(BuildSnapshot|Canonical|WriteSchemaSource)' -count=1`

Run: `git check-ignore cmd/fields/v10.4.57/metadata/sensitive_metadata.json`

Expected: tests PASS and the raw metadata path is ignored.

Commit: `feat(fields): publish deterministic local snapshots`

### Task 4: Sensitivity policy and Terraform specification codegen

**Files:**
- Create: `cmd/fields/sensitivity.go`
- Create: `cmd/fields/sensitivity_test.go`
- Create: `cmd/fields/sensitive-policy.json`
- Modify: `cmd/fields/main.go`
- Modify: `cmd/fields/schema.go`
- Modify: `cmd/fields/schema_test.go`

**Interfaces:**
- Consumes: local `sensitive_metadata.json`, the raw pre-overlay schema set, and parsed `ResourceInfo` trees.
- Produces: `FieldInfo.Sensitive` and leaf-level `sensitive: true` in the Terraform specification.

- [ ] **Step 1: Write failing policy tests**

Cover `networkconf.x_wireguard_private_key`, nested `radiusprofile.auth_servers.x_secret`, private `networkconf.name`, a `setting.*` path, and a path from a collection replaced by a custom overlay. Cover scalar traversal, ambiguous collection identity, invalid/new digest, obsolete secret-policy path, and no substring fallback for an unlisted `backup_password_hint`. A valid classified field in a skipped or non-generated collection is recorded rather than rejected; a secret-policy path that cannot land on a generated leaf fails.

- [ ] **Step 2: Add the minimal policy model**

```go
type SensitivityPolicy struct {
	Version string `json:"version"`
	ApprovedMetadataSHA256 []string `json:"approved_metadata_sha256"`
	SecretPaths []string `json:"secret_paths"`
	NonGeneratedSecretPaths []string `json:"non_generated_secret_paths"`
}

type SensitiveMetadata struct {
	SystemProperties []string `json:"sensitive_system_properties"`
	DBFields map[string][]string `json:"sensitive_db_fields_by_collection"`
	DistinctDBFields map[string]string `json:"sensitive_distinct_db_fields_by_collection"`
}

type SensitivityCoverage struct {
	Generated, NonGenerated []string
}

type RawSchemaIndex map[string]json.RawMessage

func LoadSensitivityPolicy(path string) (SensitivityPolicy, error)
func ParseSensitiveMetadata(data []byte) (SensitiveMetadata, error)
func ApplySensitivity(resources []*ResourceInfo, raw RawSchemaIndex, metadata []byte, policy SensitivityPolicy) (SensitivityCoverage, error)
```

Add `SourceFileBase string` to `ResourceInfo` when it is created from each upstream JSON filename, before replacements or route overrides. The committed policy contains only reviewed metadata digests, exact generated secret paths, and exact non-generated secret paths. A new digest fails before fields mutate. Match collections case-insensitively against `SourceFileBase`, inspect raw schemas before `Setting.json` is split or custom files replace fields, and traverse generated children by `JSONName`. Record paths in skipped/non-generated collections and reject malformed or ambiguous paths. Every `secret_paths` entry must land on exactly one Terraform-emitted leaf; every `non_generated_secret_paths` entry must remain non-generated. A path moving between those states fails until reviewed. Set `FieldInfo.Sensitive` only on terminal generated-secret leaves.

- [ ] **Step 3: Write failing specification tests**

Mark top-level private-key and nested RADIUS-secret `FieldInfo` leaves. Assert resource and data-source leaf attributes are sensitive; `name`, sibling `ip`, and enclosing nested objects are not. Retain checks for provider `password` and `api_key`.

- [ ] **Step 4: Propagate sensitivity through attribute constructors**

Add `Sensitive bool` to `FieldInfo`. In every primitive, collection, single-nested, and list-nested constructor used by resource and data-source generation, set the current field's `Sensitive` pointer only when true. Use one helper:

```go
func sensitivePtr(field *FieldInfo) *bool {
	if field == nil || !field.Sensitive { return nil }
	return ptr(true)
}
```

- [ ] **Step 5: Verify and commit**

Run: `go test ./cmd/fields -run 'Test(ParseSensitivity|ApplySensitivity|SensitivityPolicy|SpecificationGenerator_.*Sensitive|Specification_JSONStructure)' -count=1`

Expected: PASS; generated nested objects remain readable while secret leaves are masked.

Commit: `feat(fields): generate reviewed sensitive attributes`

### Task 5: End-to-end command and reproducibility

**Files:**
- Create: `cmd/fields/run.go`
- Create: `cmd/fields/run_test.go`
- Create: `cmd/fields/input_scan.go`
- Create: `cmd/fields/input_scan_test.go`
- Modify: `cmd/fields/main.go`
- Modify: `cmd/fields/extract.go`

**Interfaces:**
- Consumes: Tasks 1-4.
- Produces: one testable `Run` entry point for local use and CI.

- [ ] **Step 1: Write failing command tests**

With the synthetic installer, cover all four UOS selectors plus `-download-only`, `-output-dir`, `-generate-spec`, and `-spec-output`. Two identical runs must produce byte-identical generator-owned files and digests; exclude hand-maintained implementation scaffolds. Inject failures at resolve, download, extraction, policy, snapshot, and generation boundaries and assert prior output remains unchanged.

- [ ] **Step 2: Extract orchestration from `main`**

```go
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error

func main() {
	if err := Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "fields:", err)
		os.Exit(1)
	}
}
```

Keep legacy extraction intact, route UOS inputs through the new pipeline, load/apply sensitivity before spec generation, and update `schema-source.json` only after generated output validates. Replace orchestration panics with boundary-specific wrapped errors.

- [ ] **Step 3: Add input scanning and two verification modes**

Implement `ScanExtractedInputs` with fixtures that reject PEM private-key blocks, JWT-shaped tokens, common cloud credential formats, high-entropy values in unexpected JSON value positions, and JSON structures outside the allowlisted schema/metadata shapes. Run it before generation and fail without publishing output.

Implement `-verify-committed` for fresh offline checkouts: hash only committed generator-owned files and compare with `GeneratedTreeDigest` in `schema-source.json`. Implement `-verify-regeneration` for a machine with the local snapshot: regenerate those owned files into a temporary tree and compare bytes/digest without overwriting the working tree. Both modes report the first differing path.

- [ ] **Step 4: Verify and commit**

Run: `go test ./cmd/fields -run TestRun -count=1`

Run: `go test ./... && go vet ./... && git diff --check`

Expected: all commands PASS.

Commit: `feat(fields): regenerate from UniFi OS end to end`

### Task 6: Notices, operator documentation, and 5.1.21 pilot

**Files:**
- Create: `LICENSES/README.md`
- Create: `docs/schema-generation.md`
- Modify: `README.md`
- Modify: `cmd/fields/sensitive-policy.json`
- Create: `cmd/fields/schema-source.json`
- Modify: `specification.json`
- Modify: generated files under `unifi/`

**Interfaces:**
- Consumes: the completed local pipeline and the official OS Server 5.1.21 installer.
- Produces: reviewed 10.4.57 output, the initial generated-tree digest, and operating guidance without raw artifacts.

- [ ] **Step 1: Add redistribution and affiliation notices**

State that Ubiquiti installers and extracted artifacts remain Ubiquiti's, are not covered by this repository's MPL-2.0 license, are downloaded from Ubiquiti and not redistributed, and this project is unofficial and not affiliated with or endorsed by Ubiquiti.

- [ ] **Step 2: Document generation and state sensitivity**

Document selectors, pinned/offline regeneration, disk and cleanup behavior, sensitivity review, archive limits, failure recovery, provenance verification, input scanning, third-party-notice review, and automation controls. Explain that Terraform sensitivity masks display but does not encrypt or remove values from state.

- [ ] **Step 3: Run the real pilot**

Run: `go run ./cmd/fields -installer-url https://fw-download.ubnt.com/data/unifi-os-server/f5e2-linux-x64-5.1.21-a400c9c6-8328-4634-b223-ebfcf742720a.21-x64 -output-dir unifi -generate-spec -spec-output specification.json`

Expected: verified OS Server 5.1.21, extracted Network 10.4.57, then an intentional policy failure until its sensitivity digest is reviewed. Capture the live firmware JSON field names for size and checksums, compare them with Task 1's fixture, and correct the resolver model before trusting verification. Inspect relevant bundled license and notice entries and approve their hashes before consuming a newly discovered dataset.

- [ ] **Step 4: Review and approve sensitivity**

Classify every local metadata path as generated secret, non-generated secret, or private metadata; commit the canonical digest and both exact secret lists, then rerun. Verify `networkconf.x_wireguard_private_key`, `radiusprofile.auth_servers.x_secret`, provider `password`, and provider `api_key` are sensitive while `networkconf.name` remains visible. Verify absent setting, device, and token secrets are recorded as non-generated rather than mislabeled private.

- [ ] **Step 5: Prove raw artifacts are absent and output is reproducible**

Run: `git ls-files | rg '(^|/)(v10\.4\.57|ace\.jar|internal-dependencies\.jar|image\.tar|sensitive_metadata\.json)'`

Expected: no output.

Run in a clean checkout: `go test ./... && go vet ./... && go run ./cmd/fields -verify-committed && git diff --check`

Run twice with the local pinned snapshot: `go run ./cmd/fields -verify-regeneration`

Expected: PASS both times and the second pinned generation changes no tracked files.

- [ ] **Step 6: Commit**

Commit: `feat: regenerate for UniFi Network 10.4.57`

### Task 7: Blocking CI, compatibility gate, and release automation

**Files:**
- Modify: `.github/workflows/ci.yaml`
- Create: `.github/workflows/schema-update.yaml`
- Create: `.github/workflows/schema-release.yaml`
- Delete: `.github/workflows/generate.yaml`
- Create: `cmd/apicompat/main.go`
- Create: `cmd/apicompat/main_test.go`
- Create: `cmd/fields/workflow_test.go`

**Interfaces:**
- Consumes: the end-to-end command, generated tree, and GitHub App token.
- Produces: blocking CI, one stable update PR, compatible/breaking/ambiguous classification, and guarded patch tags.

- [ ] **Step 1: Write failing workflow and API compatibility tests**

Inspect both `.yaml` and `.yml`, including `dependabot.yml`. Assert workflows contain no `continue-on-error`, PR CI is offline and runs committed-tree verification, tests, lint, and diff checks, scheduled/manual update accepts pinned version/URL inputs, and no artifact-upload action exists. Review existing Dependabot auto-approval/merge permissions against the new required checks. With temporary Go modules, assert exported additions are compatible, removals/signature changes are breaking, and load failures are ambiguous.

- [ ] **Step 2: Harden pull-request CI**

Remove advisory error handling, pin lint versions, add `go run ./cmd/fields -verify-committed`, and require `git diff --exit-code`. Keep this workflow offline; it never needs the gitignored schema snapshot.

- [ ] **Step 3: Implement deterministic API classification**

Use `golang.org/x/tools/go/packages` to canonicalize exported packages, declarations, named underlying types, struct fields, interfaces, functions, methods, parameters, and results. Exit `0` for compatible additions, `2` for breaking changes, and `3` for ambiguous/load failures; emit a stable Markdown summary.

- [ ] **Step 4: Replace the scheduled workflow**

Use the same `cmd/fields` entry point, compare installer SHA-256 before download, run full `-verify-regeneration` while the verified snapshot exists, clean temporary inputs under `if: always()`, and create/update `automation/unifi-schema-update` using a narrowly scoped GitHub App token. Unknown sensitivity digests, changed relevant third-party notice/license hashes, and suspicious input-scan results fail before committed changes. Breaking/ambiguous changes stay open. Never upload extracted artifacts.

- [ ] **Step 5: Add guarded auto-merge and tagging**

Auto-merge only when API status is compatible, required checks pass, sensitivity policy is unchanged, and `ALLOW_AUTOMATED_SCHEMA_RELEASES=true`. Provenance-only changes merge without release. The trusted tag workflow additionally requires a merged PR from `automation/unifi-schema-update`, recorded compatibility status `compatible`, the repository variable still equal to `true`, and the expected manifest digest. It increments the latest semantic patch tag and pushes that explicit tag with the GitHub App token; do not replace it with `GITHUB_TOKEN`, whose tag push would not trigger the existing release workflow.

- [ ] **Step 6: Verify and commit**

Run: `go test ./cmd/apicompat ./cmd/fields -count=1`

Run: `actionlint .github/workflows/*.yaml .github/workflows/*.yml && git diff --check`

Expected: PASS with no workflow warnings.

Commit: `ci: automate compatible UniFi schema releases`

### Task 8: Manual automation dry run

**Files:**
- Modify only files whose behavior fails the dry run.

**Interfaces:**
- Consumes: configured GitHub App secrets, required checks, and repository variable.
- Produces: one verified update PR without an unintended release.

- [ ] **Step 1: Configure controls**

Install the narrowly scoped GitHub App, add its ID/private key Actions secrets, make CI/generation/compatibility checks required, and leave `ALLOW_AUTOMATED_SCHEMA_RELEASES` unset.

- [ ] **Step 2: Dispatch pinned OS Server 5.1.21**

Confirm the workflow reuses its stable branch, uploads no raw artifact, reproduces committed digests, and creates either no diff or a provenance-only PR.

- [ ] **Step 3: Exercise safety gates**

In a temporary branch, confirm unknown sensitivity digest, breaking API fixture, checksum mismatch, and a failed required check each prevent merge and tagging.

- [ ] **Step 4: Choose the release setting**

After one successful unattended cycle, either set `ALLOW_AUTOMATED_SCHEMA_RELEASES=true` or retain maintainer approval. Record the workflow URL and digests in the PR, never raw metadata or signed URLs.
