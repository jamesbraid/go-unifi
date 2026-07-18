# Task 6 implementation report

## Docker media-type compatibility bugfix

Root cause: the real UniFi OS Server `image.tar` is an OCI image layout whose
linux/amd64 descriptor and manifest use Docker schema 2 media type
`application/vnd.docker.distribution.manifest.v2+json`, and whose compressed
layers use `application/vnd.docker.image.rootfs.diff.tar.gzip`. `ResolveImage`
and `FindFileInLayers` previously accepted only the corresponding OCI media
types, so resolution ended with `expected exactly one linux/amd64 OCI manifest,
found 0` despite the descriptor being otherwise valid.

RED:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestResolveImageAcceptsDocker|TestResolveImageRejectsDocker|TestResolveImageRejectsDescriptorMismatch' -count=1
--- FAIL: TestResolveImageAcceptsDockerSchema2ManifestAndNestedList
    Received unexpected error:
        OCI index: unsupported media type "application/vnd.docker.distribution.manifest.list.v2+json"
FAIL
```

GREEN:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestResolveImageAcceptsDocker|TestResolveImageRejectsDocker|TestResolveImageRejectsDescriptorMismatch|TestFindFileInLayers' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields
```

The implementation adds role-specific support for Docker manifest lists,
Docker schema 2 manifests, and Docker gzip layers. Docker schema 1 and unknown
media types remain unsupported. Existing descriptor size and digest validation
is unchanged and is exercised for Docker manifests by the new fixture cases.

Read-only validation against `/private/tmp/recheck/image.tar` advanced through
`ResolveImage`, confirming that its single linux/amd64 Docker schema 2 manifest
is now selected. `FindFileInLayers` then exposed a separate issue while scanning
an unrelated layer entry named
`lib/systemd/system/system-systemd\\x2dcryptsetup.slice`: the general archive
path validator rejects backslashes. That issue is deliberately not bundled
into this media-type compatibility commit.

Verification after the scoped implementation:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'Test(ImportOCI|ResolveImage|FindFileInLayers|ExtractUOSInstaller|SyntheticInstaller)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.307s

GOCACHE=/tmp/go-build-task6 go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.579s
ok github.com/ubiquiti-community/go-unifi/unifi 1.413s
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

GOCACHE=/tmp/go-build-task6 go vet ./...
git diff --check
```

## Reviewed nested dependency notice gate

The first local notice review counted 111 LICENSE/NOTICE entries, but that
side-channel inventory was incomplete under the final extractor contract. The
production matcher now inspects all 153 direct `ace.jar/BOOT-INF/lib/*.jar`
archives and accepts root or `META-INF` LICENSE, NOTICE, COPYING, COPYRIGHT, and
THIRD-PARTY families with exact, `.txt`, `.md`, dash, and underscore forms. It
explicitly excludes `.class`, `.properties`, `.bin`, and unrelated basename
lookalikes.

The final retained-ace harness used the production indexer, sequential JAR spool,
all-entry body/CRC validation, notice budget, artifact reader, and
`CanonicalTreeDigest`. It found 138 entries across 73 matching JARs, totaling
850,719 bytes: 77 LICENSE-family, 60 NOTICE-family, and one THIRDPARTY-family
entry. The shape breakdown is 54 exact basenames, 66 `.txt`, 16 `.md`, one dash
variant, and one exact THIRDPARTY name. The canonical reviewed digest is:

```text
70a014c0a8a3e9f3e91c48c6fb03811fbd15cbd8102a376e60dcc5253dc5a10f
```

The temporary absolute-path harness was deleted. No notice body, JAR, ZIP, tar,
or raw inventory was added to the repository. Notice bodies remain in the local,
gitignored snapshot and remain vendor-governed.

Extraction now bounds nested archives to 1,024, captured notices to 10,000
entries and 256 MiB aggregate, and each expanded dependency archive to the
existing 512 MiB JAR limit. Every nested entry is streamed to EOF before notice
filtering so CRC corruption in ignored entries also fails. Paths, entry types,
duplicates, archive namespace case-fold collisions, and notice destination
case-fold collisions fail deterministically. Each dependency spool is closed and
removed before processing the next archive.

The required, strictly validated and sorted `approved_notice_sha256` policy array
contains exactly the reviewed digest. Generation checks it after atomic snapshot
publication and input scan but before rendering tracked output. Both offline
verification modes check the committed provenance digest against the same policy.
An unknown digest leaves the complete new snapshot and prior tracked output in
place and returns a first-class `notice digest ... is not approved` error.

TDD first demonstrated missing nested limits/inventory and then showed that run,
committed verification, and regeneration verification incorrectly accepted an
unknown digest. Focused extraction, policy, run, verification, and documentation
tests now pass:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'Test(ExtractUOSInstaller|DependencyNoticePathFamilies|SensitivityPolicy|RequireApprovedNoticeDigest|RunUnapprovedNotice|VerificationModesReject|RunLocalInstaller|RunPolicyFailure|RunBoundary|SchemaGenerationDocumentation)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.911s

GOCACHE=/tmp/go-build-task6 go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.635s
ok github.com/ubiquiti-community/go-unifi/unifi (cached)
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

GOCACHE=/tmp/go-build-task6 go vet ./...
git diff --check
raw archive and binary diff checks
```

### Review hardening: snapshot binding and aggregate expansion

Review found that offline regeneration approved only the committed notice digest;
it did not bind that value to the selected snapshot's actual notice files. The new
snapshot provenance validator strictly decodes `metadata/source.json`, checks its
Network version, walks only regular non-symlink notice paths with canonical and
case-fold collision validation, and recomputes `CanonicalTreeDigest`. Rendering
requires equality among the actual tree, local manifest, approved policy, and
committed schema-source digest. A regression mutates both a local notice body and
its local manifest while leaving the old committed digest approved; regeneration
now fails on the unapproved actual digest without changing tracked output.

The prior 512 MiB limit applied independently to every nested JAR. A shared 2 GiB
expanded-byte budget now spans all direct dependency JARs while preserving that
per-JAR limit. Multiple individually valid archives that exceed the shared budget
fail. The final retained-ace harness streamed and CRC-validated all 153 archives
through the production path and measured 263,007,262 expanded bytes. It again found
138 notices totaling 850,719 bytes, and the canonical digest remained unchanged:

```text
70a014c0a8a3e9f3e91c48c6fb03811fbd15cbd8102a376e60dcc5253dc5a10f
```

Direct and nested capture now use the same reviewed family matcher. Exact names,
`.txt`, `.md`, and dash/underscore variants such as `LICENSE-2.0` are accepted;
`.class`, `.properties`, and `.bin` lookalikes remain excluded. The existing root
and `META-INF` path scope is unchanged.

Successful extraction is explicitly closed immediately after `BuildSnapshot`
returns, before scan, policy, or rendering. The original defer remains an
idempotent fallback for snapshot-construction failures. A scan seam proves the
extraction tree is already absent, and download-only continues from the durable
snapshot.

Verification:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'Test(SnapshotNotice|ValidateSnapshotNotice|AddSnapshotNotice|VerifyRegenerationRejectsDifferent|VerificationModesReject|ExtractUOSInstaller|DependencyNoticePath|RunCleansExtractedDefinitions|RunLocalInstaller|RunPolicyFailure|RunBoundary|SchemaGenerationDocumentation)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.041s

GOCACHE=/tmp/go-build-task6 go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 2.025s
ok github.com/ubiquiti-community/go-unifi/unifi (cached)
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

GOCACHE=/tmp/go-build-task6 go vet ./...
git diff --check
temporary-harness and raw-archive checks
```

### Closed notice suffix grammar

The first reviewed matcher excluded several named binary extensions but still
accepted arbitrary dash/underscore suffixes such as `LICENSE-secrets.json`. RED
tests captured JSON, executable, compressed-property, malformed separator,
non-numeric dotted-token, empty, and control-byte cases for both direct and nested
paths.

The shared matcher now accepts only an exact reviewed family, family plus `.txt`
or `.md`, or a dash/underscore sequence of non-empty ASCII alphanumeric tokens.
Dots are permitted only inside all-numeric version tokens, and the variant may end
in `.txt` or `.md`. This accepts observed forms plus `LICENSE-2.0`,
`LICENSE-APACHE-2.0.txt`, and `NOTICE-third-party`, while rejecting arbitrary file
extensions and malformed names.

The retained-ace production harness again reported 138 notices, 850,719 notice
bytes, 263,007,262 expanded dependency bytes, and the unchanged reviewed digest:

```text
70a014c0a8a3e9f3e91c48c6fb03811fbd15cbd8102a376e60dcc5253dc5a10f
```

The temporary absolute-path harness was removed before verification and commit.

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'Test(DependencyNoticePathFamiliesExcludeBinaryLookalikes|ExtractUOSInstallerInventoriesDirectDependencyNotices|ExtractUOSInstallerInventoriesNoticeBasenameSuffixes|ExtractUOSInstallerDependencyNoticeInventoryIsOrderIndependent)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.218s

GOCACHE=/tmp/go-build-task6 go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.845s
ok github.com/ubiquiti-community/go-unifi/unifi (cached)
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

GOCACHE=/tmp/go-build-task6 go vet ./...
git diff --check
```

## Extracted-definition lifecycle and snapshot semantics

The successful extractor path returned file-backed artifacts beneath a
`uos-extract-*` directory without returning ownership of that directory to the
caller. Consequently every successful run leaked its extraction tree, including
runs that later stopped at snapshot, scan, sensitivity-policy, or generation
boundaries.

RED run lifecycle tests asserted that no extraction tree remains after a complete
run or after each post-extraction failure boundary. They failed with the surviving
`uos-extract-*` paths. `ExtractedDefinitions` now owns that directory through
idempotent `Close` and `Cleanup` methods. `runUOS` defers `Close` immediately after
successful extraction, so all later returns clean the artifacts while leaving the
caller-owned installer and the atomically published snapshot untouched. Direct
extractor tests close successful results and prove both idempotency and installer
preservation.

The operator and design documentation now distinguishes the two publication
boundaries. A same-version snapshot atomically replaces its predecessor as soon as
snapshot construction succeeds and remains available if later scan, policy, or
generation work fails. Tracked generated output is independently staged and keeps
its prior state until final publication.

Verification:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'Test(ExtractUOSInstaller|SyntheticInstaller|RunLocalInstallerEndToEndIsDeterministic|RunPolicyFailureKeepsSnapshotAndPriorOutputs|RunBoundaryFailuresPreserveAllCommittedOutputs|SchemaGenerationDocumentationSafetyBoundaries)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.763s

GOCACHE=/tmp/go-build-task6 go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.524s
ok github.com/ubiquiti-community/go-unifi/unifi (cached)
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

GOCACHE=/tmp/go-build-task6 go vet ./...
git diff --check
```

## Policy and documentation verification

No snapshot-dependent or machine-absolute test remains in the tree. The
committed policy test is compact and repository-portable; it checks the pinned
digest, sorted and disjoint 28/36 lists, required examples, and explicit
private-visible exclusions. Documentation tests guard the redistribution,
state-security, incomplete-notice, verification, disk-space, and automation
boundaries. Existing provider schema tests verify both `password` and `api_key`
remain Sensitive.

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'Test(SensitivityPolicy_ApprovedUniFi10457|ApplySensitivity_ClassifiesExactLeavesAndCoverage|SpecificationGenerator_Generate_Provider|SchemaGenerationDocumentationSafetyBoundaries)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.212s

GOCACHE=/tmp/go-build-task6 go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.503s
ok github.com/ubiquiti-community/go-unifi/unifi (cached)
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

GOCACHE=/tmp/go-build-task6 go vet ./...
git diff --check
git ls-files | rg '(^|/)(v10\.4\.57|ace\.jar|internal-dependencies\.jar|image\.tar|sensitive_metadata\.json)'
# no output
```

## UniFi Network 10.4.57 sensitivity classification

The actual generator parse path loaded the raw schemas and canonical
`sensitive_metadata.json` from the local 10.4.57 snapshot. Its canonical digest
matches the reviewed value
`7b1dfe4989af062a1bb1be0c40ffed90192cb615dffb698de090fb82db5b298c`.

Before provider policy extensions, vendor metadata covers 39 generated and 43
non-generated paths. A semantic review also inspected credential-bearing raw
schema paths not present in that privacy metadata. The final exact policy adds
28 generated secrets and 36 non-generated secrets. The mapper accepted every
proposed path with no generated/non-generated deviations. Final coverage is:

- 51 generated paths: 28 secret, 23 private-visible;
- 63 non-generated paths: 36 secret, 27 private;
- 114 canonical reviewed paths in total.

Generated secrets:

```text
account.x_password
device.lte_password
device.lte_sim_pin
device.mbb_overrides.sim.current_apn.password
device.nut_server.password
device.x_baresip_password
dynamicdns.x_password
hotspotop.x_password
networkconf.wireguard_client_preshared_key
networkconf.x_auth_key
networkconf.x_ca_key
networkconf.x_ipsec_pre_shared_key
networkconf.x_openvpn_password
networkconf.x_openvpn_shared_secret_key
networkconf.x_pptpc_password
networkconf.x_server_key
networkconf.x_shared_client_key
networkconf.x_wan_password
networkconf.x_wireguard_private_key
radiusprofile.acct_servers.x_secret
radiusprofile.auth_servers.x_secret
radiusprofile.x_client_private_key
radiusprofile.x_client_private_key_password
wlanconf.private_preshared_keys.password
wlanconf.sae_psk.psk
wlanconf.x_iapp_key
wlanconf.x_passphrase
wlanconf.x_wep
```

Non-generated secrets:

```text
authenticationrequest.password
authenticationrequest.sso_token
authenticationrequest.ubic_2fa_token
device.x_authkey
device.x_ble_adopt_key
device.x_ble_auth_key
device.x_ssh_hostkey
device.x_vwirekey
setting.connectivity.x_mesh_psk
setting.element_adopt.x_element_psk
setting.guest_access.x_authorize_transactionkey
setting.guest_access.x_facebook_app_secret
setting.guest_access.x_google_client_secret
setting.guest_access.x_merchantwarrior_apikey
setting.guest_access.x_merchantwarrior_apipassphrase
setting.guest_access.x_password
setting.guest_access.x_paypal_password
setting.guest_access.x_quickpay_apikey
setting.guest_access.x_stripe_api_key
setting.guest_access.x_wechat_app_secret
setting.guest_access.x_wechat_secret_key
setting.mgmt.x_mgmt_key
setting.mgmt.x_ssh_md5passwd
setting.mgmt.x_ssh_password
setting.mgmt.x_ssh_sha512passwd
setting.radius.x_secret
setting.snmp.x_password
setting.super_cloudaccess.x_private_key
setting.super_mgmt.google_maps_api_key
setting.super_mgmt.x_ssh_password
setting.super_sdn.auth_token
setting.super_smtp.x_password
setting.x_api_token
setting.x_sso_token
setting.x_stunnel_key
teleport_token.secret_verifier_encoded
```

The review conservatively treats `device.x_ssh_hostkey` and the teleport secret
verifier as authentication material. It excludes `networkconf.x_dh_key` as
public DH parameters, and excludes public certificates/keys, names, emails,
usernames, hostnames, IPs, serial/SIM identifiers, and
`setting.super_cloudaccess.device_auth` pending evidence that it contains a
credential rather than an authentication setting. Required examples resolve as
expected: the WireGuard private key and RADIUS auth-server secret are generated
secrets; absent device/root-setting tokens are non-generated; `networkconf.name`
is visible. Provider `password` and `api_key` remain independently hand-coded
Sensitive attributes.

## Local license and notice inventory

Read-only review used `/private/tmp/recheck/layer/usr/lib/unifi/lib/ace.jar`,
the extracted `ace/BOOT-INF/lib/*.jar` set, and the extracted internal dependency
tree. All 153 nested dependency JARs passed `unzip -tqq`.

- direct matching entries in `ace.jar`: 0;
- direct matching entries in `internal-dependencies.jar`: 0;
- matching files in the extracted internal tree: 0;
- nested dependency JARs inspected: 153;
- nested JARs with matches: 61;
- matching nested entries: 111 (65 LICENSE-family, 46 NOTICE-family).

For each nested match, the local inventory records the JAR basename, entry name,
and SHA-256 of the streamed entry body. The sorted inventory's aggregate SHA-256
is `72c3399fadddb2fcb513b2ec6c4dfbd1cefee05e0fe465c1fe48be82b8fcc3d2`.
Representative entry hashes are:

```text
angus-activation-2.0.3.jar  META-INF/LICENSE.md  87eba02f8a415f1a25de6ca3ac6b5c77eb33f33f00ae5a5f9d6ee963147f7956
angus-activation-2.0.3.jar  META-INF/NOTICE.md   203414df1bdc467ff3d2c53e44ed7a7b9bcba90067bd2f3442b1d509a308b21e
commons-codec-1.18.0.jar    META-INF/LICENSE.txt cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30
commons-codec-1.18.0.jar    META-INF/NOTICE.txt  b64933ee1d36d14659156223a2604edadb60bffdd465d5368ff422d7689db5fb
spring-core-6.2.17.jar      META-INF/license.txt 955e3c35f8b685c46bf2d6e5d7eea44ce2f83aaef947a3571b9bddacca577d8a
spring-core-6.2.17.jar      META-INF/notice.txt  05de3a2336d1f58c0ae8d43237e050043dce669f985e108378b52fda10f5bd73
```

The snapshot has no extracted notice bodies because the extractor inventories
only direct relevant entries at its two reviewed JAR boundaries. Its
`NoticeDigest`
`2caaba0bb439038643b99decb6f1c5bcdd0179a1885190685e796ac0dbfaebe5`
therefore truthfully identifies an empty direct set. It is not evidence of a
complete nested dependency license inventory. Nested bodies remain local and
vendor-governed and are not committed.

## Metadata-backed absent non-generated secrets

Real sensitivity metadata classifies historical/device and root-setting secret
paths that are absent from the corresponding 10.4.57 raw schema. The policy
contract requires recording these as non-generated secrets, but the mapper
previously rejected any missing path when its raw collection still existed.

A RED fixture uses metadata-backed `device.x_authkey` with a real-shaped Device
schema that lacks the field. The mapper now permits an absent exact
`non_generated_secret_path` only when the same canonical path came from the
approved vendor sensitivity metadata. It still checks the generated resource
graph and fails if the path becomes generated. An absent typo such as
`device.x_authkey_typo` fails because it is not backed by the approved metadata.
This keeps historical secrets reviewable without turning the non-generated list
into an unchecked extension point.

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestApplySensitivity_(NonGeneratedSecretPathsEnforceStatus|AllowsMetadataBackedAbsentSecretInExistingRawCollection|FailuresAreTransactional)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.221s
```

## NetworkConf compatibility-field identity

The real generator parse path initially blocked sensitivity inventory because
Network 10.4.57 now supplies
`wireguard_interface_binding_mode_ip_version`, while `NewResource` still adds
the same JSON field for compatibility with older schemas. Global Go-name
cleanup rewrites upstream `IPVersion` to `Version`, leaving two map entries with
the same JSON name. Sensitivity resolution correctly rejected the ambiguity:

```text
duplicate JSONName "wireguard_interface_binding_mode_ip_version" while resolving networkconf.networkgroup
```

A minimal real-shaped generation fixture established RED with two matching
fields. The Network field processor now restores the established
`WireguardInterfaceBindingModeIPVersion` Go identity for the upstream field, so
normal map insertion replaces the compatibility fallback. The upstream JSON
tag, string type, pointer behavior, and validation remain authoritative, while
older schemas retain the fallback.

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run TestGenerateFromFieldsMergesUpstreamWireguardIPVersionCompatibilityField -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.214s
```

## Reviewed singleton-enum allowlist

Review found that the bounded lowercase grammar still admitted arbitrary short
concrete strings such as `password`, `letmein`, and `secret123`. Inventory of
the real raw field schemas found exactly three lowercase singleton enum
validators outside standard schema types: `static-route`, `switch`, and
`upgrade`.

RED fixtures confirmed that all three secret-like values passed under the
grammar. The broad pattern is now replaced by an exact reviewed allowlist for
the three observed enum validators, so any future singleton value fails closed
for review. Fixtures cover acceptance of all three values and rejection of all
three short secret-like strings with their RFC 6901 path.

The stricter direct real-snapshot scan then revealed one separate observed
literal category previously masked by the broad pattern:

```text
scan Setting.json: schema path /super_mgmt/default_site_device_auth_password_alert: unexpected concrete scalar "false"
```

Inventory found only this `"false"` boolean-literal validator (in its flattened
and raw representations), so `false` is accepted explicitly alongside standard
schema type tokens; `true` is not implicitly broadened. With that reviewed
literal added, focused scanner tests and the full real snapshot pass:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestScanExtractedInputs' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.244s

GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run TestTask6ReviewedSingletonRealSnapshot -count=1 -v
=== RUN   TestTask6ReviewedSingletonRealSnapshot
--- PASS: TestTask6ReviewedSingletonRealSnapshot (0.10s)
PASS
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.316s
```

The temporary real-snapshot harness was removed before final verification and
commit.

## Singleton schema-enum scanner compatibility

Root cause: `PortConf.json` contains `op_mode: "switch"` in both the flattened
and raw schemas. This is a singleton lowercase enum validator, but
`schemaString` previously accepted type names, regex-shaped strings, and
pipe-delimited values only. Scanner errors also omitted the JSON key path, so
the real snapshot failure reported only `unexpected concrete scalar "switch"`.

RED:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestScanExtractedInputs(AcceptsSingletonSchemaEnums|ReportsRejectedSchemaJSONPath)' -count=1
--- FAIL: TestScanExtractedInputsAcceptsSingletonSchemaEnums
    unexpected concrete scalar "switch"
--- FAIL: TestScanExtractedInputsReportsRejectedSchemaJSONPath
    Error "unexpected concrete scalar \"literal-secret-value\"" does not contain "/outer/op_mode"
FAIL
```

Schema recursion now visits object keys in sorted order and reports RFC 6901
JSON Pointer paths for rejected leaves. After the unchanged credential and
high-entropy checks, `schemaString` accepts a singleton enum only when it
matches the explicit bounded grammar `[a-z][a-z0-9_-]{0,15}`. Existing concrete
string and high-entropy rejection fixtures remain green; in particular,
`literal-secret-value` remains rejected and is now reported at
`/outer/op_mode`.

Focused scanner tests and the direct real-snapshot scan pass:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestScanExtractedInputs' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.235s

GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run TestTask6RealSnapshotScan -count=1 -v
=== RUN   TestTask6RealSnapshotScan
--- PASS: TestTask6RealSnapshotScan (0.09s)
PASS
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.303s
```

The temporary real-snapshot test was removed before final verification and
commit. The full `cmd/fields/v10.4.57` snapshot scan has no next error.

Final verification:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestScanExtractedInputs' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.241s

GOCACHE=/tmp/go-build-task6 go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.394s
ok github.com/ubiquiti-community/go-unifi/unifi (cached)
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

GOCACHE=/tmp/go-build-task6 go vet ./...
git diff --check
```

The initial sandboxed full-suite run could not bind the tests' localhost
`httptest` servers. The same suite passed with the required localhost
permission; this was an environment restriction, not a product failure.

## POSIX layer-name compatibility bugfix

Root cause: `scanLayer` applied `cleanArchiveName` to every tar header even
though unrelated layer paths are never staged or written. That validator
intentionally rejects backslashes for host-path safety, but a backslash is an
ordinary byte in a POSIX filename. The real image consequently stopped at the
unrelated systemd-escaped name
`lib/systemd/system/system-systemd\\x2dcryptsetup.slice` before it reached
`ace.jar`.

RED:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestFindFileInLayers(AllowsLiteralBackslash|RejectsMalformedOrDuplicateLayerNames)' -count=1
--- FAIL: TestFindFileInLayersAllowsLiteralBackslashInUnrelatedPOSIXName
    invalid path "lib/systemd/system/system-systemd\\x2dcryptsetup.slice": path is not relative slash-separated
--- FAIL: TestFindFileInLayersRejectsMalformedOrDuplicateLayerNames/empty_slash_component
--- FAIL: TestFindFileInLayersRejectsMalformedOrDuplicateLayerNames/dot_slash_component
FAIL
```

GREEN uses a layer-only POSIX header validator. It permits literal backslashes
and the conventional single trailing slash on tar directory headers, but
rejects empty, absolute, NUL-containing, traversal, dot-component, redundant
slash, and duplicate names. The requested target still uses strict
`cleanArchiveName`; OCI and ZIP paths that can be staged or written are
unchanged. Layer header names are only compared with slash-delimited targets,
and extracted candidates are always written to `os.CreateTemp`, so a Windows
host cannot turn the accepted layer name into an output path.

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'TestFindFileInLayers' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.238s
```

A temporary, uncommitted test opened `/private/tmp/recheck/image.tar`, imported
the layout, resolved linux/amd64, and called `FindFileInLayers` for
`usr/lib/unifi/lib/ace.jar`. It extracted a non-empty `ace.jar` successfully:

```text
--- PASS: TestTask6RealImageTarValidation (9.91s)
PASS
ok github.com/ubiquiti-community/go-unifi/cmd/fields 10.124s
```

The temporary test containing the machine-local absolute path was removed
before verification and commit.

Final verification:

```text
GOCACHE=/tmp/go-build-task6 go test ./cmd/fields -run 'Test(ImportOCI|ResolveImage|FindFileInLayers|ExtractUOSInstaller|SyntheticInstaller)' -count=1
ok github.com/ubiquiti-community/go-unifi/cmd/fields 0.306s

GOCACHE=/tmp/go-build-task6 go test ./...
ok github.com/ubiquiti-community/go-unifi/cmd/fields 1.407s
ok github.com/ubiquiti-community/go-unifi/unifi (cached)
ok github.com/ubiquiti-community/go-unifi/unifi/settings (cached)

GOCACHE=/tmp/go-build-task6 go vet ./...
git diff --check
```
