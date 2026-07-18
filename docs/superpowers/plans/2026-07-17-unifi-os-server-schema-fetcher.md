# UniFi OS Server Schema Fetcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a UniFi OS Server installer download/extract path to `cmd/fields` (keeping the legacy deb path), feed `sensitive_metadata.json` into `specification.json` codegen, and fully automate regeneration + release in GitHub Actions.

**Architecture:** Port the Python PoC's ELF→zip→OCI-image→ace.jar→internal-dependencies.jar chain to Go inside `cmd/fields`, using `go-containerregistry`'s `layout` package for OCI parsing and stdlib `archive/zip`/`archive/tar` elsewhere (Go's zip reader handles the ELF-prepended zip natively via `baseOffset`). Sensitive metadata flags spec attributes via a dot-path walker with an explicit display allowlist. Automation: cron workflow with a cheap version pre-check → regenerate → GitHub-App-token PR → auto-merge → tag-on-merge → existing goreleaser.

**Tech Stack:** Go 1.25, `github.com/google/go-containerregistry` v0.21.7 (new dep), stdlib archives, GitHub Actions (`actions/create-github-app-token` v2.2.2, `peter-evans/create-pull-request` v8.1.1).

**Spec:** `docs/superpowers/specs/2026-07-17-unifi-os-server-schema-fetcher-design.md` (read it first for policy rationale, especially the sensitive-marking allowlist).

**House rules for the executor:**
- Kernel-style commit messages (`fields: ...`, `ci: ...`), lowercase, no conventional-commit prefixes, no trailing period.
- NEVER `git push`, never create PRs. Commits only; report at the end.
- Run `go build ./... && go test ./cmd/fields/ ./unifi/` after every task — the tree must stay green at every commit.
- Test helpers shared across the new test files live in `cmd/fields/testhelpers_test.go` (created in Task 2, extended later).

---

## File map

| File | Responsibility |
|---|---|
| `cmd/fields/fwupdate.go` | fw-update API types/constants (+ `Sha256Checksum`, `FileSize`, OS Server consts, new API endpoint vars) |
| `cmd/fields/version.go` | OS Server release lookups (latest + specific version) |
| `cmd/fields/osserver_test.go` | tests for the lookups |
| `cmd/fields/oci.go` | installer zip → `image.tar` → OCI layout dir → `ace.jar` |
| `cmd/fields/acejar.go` | ace.jar → internal-dependencies.jar → defs layout + Network version |
| `cmd/fields/installer.go` | download (sha256), pipeline orchestration, `source.json`, fields-dir cache |
| `cmd/fields/extract.go` | deb extraction (trimmed) + shared `postProcessFieldsDir` |
| `cmd/fields/sensitive.go` | sensitive metadata types, allowlist, dot-path field marker |
| `cmd/fields/main.go` | CLI modes, wiring, `FieldInfo.Sensitive`, version.generated.go |
| `cmd/fields/schema.go` | apply `Sensitive` to generated spec attributes |
| `cmd/fields/testhelpers_test.go` | synthetic installer/jar/OCI fixtures |
| `unifi/unifi.go` | go:generate directive (+ `-generate-spec`) |
| `.github/workflows/generate.yaml` | cron → pre-check → generate → PR → auto-merge |
| `.github/workflows/release-on-merge.yaml` | tag on schema-touching merge (new) |
| `.github/workflows/ci.yaml` | real test gate; skip generate on `auto/*` |
| `README.md` | updated codegen docs |

---

### Task 1: OS Server release lookups (`fwupdate.go`, `version.go`)

**Files:**
- Modify: `cmd/fields/fwupdate.go`
- Modify: `cmd/fields/version.go` (full rewrite)
- Test: `cmd/fields/osserver_test.go` (new)

Do NOT delete `latestUnifiVersion`/`firmwareUpdateApi`/`maxVersion` yet — main.go still uses them until Task 6. Only add.

- [ ] **Step 1: Write the failing tests**

Create `cmd/fields/osserver_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func osServerFixture(t *testing.T, entries []firmwareUpdateApiResponseEmbeddedFirmware) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		resp, err := json.Marshal(firmwareUpdateApiResponse{
			Embedded: firmwareUpdateApiResponseEmbedded{Firmware: entries},
		})
		require.NoError(t, err)
		_, err = rw.Write(resp)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	return server
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func mustVersion(t *testing.T, v string) *version.Version {
	t.Helper()
	ver, err := version.NewVersion(v)
	require.NoError(t, err)
	return ver
}

func TestLatestOsServerRelease(t *testing.T) {
	server := osServerFixture(t, []firmwareUpdateApiResponseEmbeddedFirmware{
		{
			Channel: releaseChannel, Platform: "linux-arm64", Product: osServerProduct,
			Version:        mustVersion(t, "5.1.21"),
			Sha256Checksum: "arm64sha",
			Links: firmwareUpdateApiResponseEmbeddedFirmwareLinks{
				Data: firmwareUpdateApiResponseEmbeddedFirmwareDataLink{
					Href: mustURL(t, "https://fw-download.ubnt.com/arm64"),
				},
			},
		},
		{
			Channel: releaseChannel, Platform: osServerPlatform, Product: osServerProduct,
			Version:        mustVersion(t, "5.1.21"),
			Sha256Checksum: "x64sha",
			Links: firmwareUpdateApiResponseEmbeddedFirmwareLinks{
				Data: firmwareUpdateApiResponseEmbeddedFirmwareDataLink{
					Href: mustURL(t, "https://fw-download.ubnt.com/x64"),
				},
			},
		},
	})

	old := firmwareLatestApi
	firmwareLatestApi = server.URL
	t.Cleanup(func() { firmwareLatestApi = old })

	rel, err := latestOsServerRelease()
	require.NoError(t, err)
	assert.Equal(t, "5.1.21", rel.Version.String())
	assert.Equal(t, "https://fw-download.ubnt.com/x64", rel.URL.String())
	assert.Equal(t, "x64sha", rel.SHA256)
}

func TestFindOsServerRelease(t *testing.T) {
	server := osServerFixture(t, []firmwareUpdateApiResponseEmbeddedFirmware{
		{
			Channel: releaseChannel, Platform: osServerPlatform, Product: osServerProduct,
			Version: mustVersion(t, "5.1.21"),
			Links: firmwareUpdateApiResponseEmbeddedFirmwareLinks{
				Data: firmwareUpdateApiResponseEmbeddedFirmwareDataLink{
					Href: mustURL(t, "https://fw-download.ubnt.com/new"),
				},
			},
		},
		{
			Channel: releaseChannel, Platform: osServerPlatform, Product: osServerProduct,
			Version: mustVersion(t, "5.0.8"),
			Links: firmwareUpdateApiResponseEmbeddedFirmwareLinks{
				Data: firmwareUpdateApiResponseEmbeddedFirmwareDataLink{
					Href: mustURL(t, "https://fw-download.ubnt.com/old"),
				},
			},
		},
	})

	old := firmwareApi
	firmwareApi = server.URL
	t.Cleanup(func() { firmwareApi = old })

	rel, err := findOsServerRelease("5.0.8")
	require.NoError(t, err)
	assert.Equal(t, "https://fw-download.ubnt.com/old", rel.URL.String())

	_, err = findOsServerRelease("9.9.9")
	assert.ErrorContains(t, err, "9.9.9")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/fields/ -run 'OsServer' -v`
Expected: FAIL — `undefined: latestOsServerRelease` (compile error)

- [ ] **Step 3: Implement**

In `cmd/fields/fwupdate.go`, add endpoint vars and constants (keep the existing `firmwareUpdateApi` var and `maxVersion` for now):

```go
var firmwareLatestApi = "https://fw-update.ubnt.com/api/firmware-latest"
var firmwareApi = "https://fw-update.ubnt.com/api/firmware"
```

Add to the const block:

```go
	osServerProduct  = "unifi-os-server"
	osServerPlatform = "linux-x64"
```

Add two fields to `firmwareUpdateApiResponseEmbeddedFirmware`:

```go
	Sha256Checksum string `json:"sha256_checksum"`
	FileSize       int64  `json:"file_size"`
```

Replace the body of `cmd/fields/version.go` with (keeping the package/imports; `latestUnifiVersion` moves here unchanged from the old file — do not lose it):

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-version"
)

// osServerRelease describes a UniFi OS Server download published on the
// firmware update API.
type osServerRelease struct {
	Version *version.Version
	URL     *url.URL
	SHA256  string
}

func fetchFirmware(u *url.URL) (*firmwareUpdateApiResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var respData firmwareUpdateApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, err
	}
	return &respData, nil
}

// latestOsServerRelease returns the newest UniFi OS Server release for
// linux-x64 on the release channel.
func latestOsServerRelease() (*osServerRelease, error) {
	u, err := url.Parse(firmwareLatestApi)
	if err != nil {
		return nil, err
	}

	query := u.Query()
	query.Add("filter", firmwareUpdateApiFilter("eq", "channel", releaseChannel))
	query.Add("filter", firmwareUpdateApiFilter("eq", "product", osServerProduct))
	u.RawQuery = query.Encode()

	respData, err := fetchFirmware(u)
	if err != nil {
		return nil, err
	}

	return pickOsServerRelease(respData, nil)
}

// findOsServerRelease returns a specific UniFi OS Server release for
// linux-x64 (e.g. "5.1.21").
func findOsServerRelease(want string) (*osServerRelease, error) {
	wantV, err := version.NewVersion(want)
	if err != nil {
		return nil, fmt.Errorf("invalid unifi-os-server version %q: %w", want, err)
	}

	u, err := url.Parse(firmwareApi)
	if err != nil {
		return nil, err
	}

	query := u.Query()
	query.Add("filter", firmwareUpdateApiFilter("eq", "channel", releaseChannel))
	query.Add("filter", firmwareUpdateApiFilter("eq", "product", osServerProduct))
	query.Add("filter", firmwareUpdateApiFilter("eq", "platform", osServerPlatform))
	u.RawQuery = query.Encode()

	respData, err := fetchFirmware(u)
	if err != nil {
		return nil, err
	}

	return pickOsServerRelease(respData, wantV)
}

func pickOsServerRelease(respData *firmwareUpdateApiResponse, want *version.Version) (*osServerRelease, error) {
	for _, firmware := range respData.Embedded.Firmware {
		if firmware.Platform != osServerPlatform || firmware.Version == nil {
			continue
		}
		if want != nil && !firmware.Version.Equal(want) {
			continue
		}
		return &osServerRelease{
			Version: firmware.Version,
			URL:     firmware.Links.Data.Href,
			SHA256:  firmware.Sha256Checksum,
		}, nil
	}
	if want != nil {
		return nil, fmt.Errorf("unifi-os-server %s not found on firmware API", want)
	}
	return nil, fmt.Errorf("no unifi-os-server linux-x64 release found on firmware API")
}
```

Also keep `latestUnifiVersion` in this file (copy it over from the old version.go verbatim — it is deleted later, in Task 6).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/fields/ -run 'OsServer' -v && go build ./...`
Expected: PASS, build OK

- [ ] **Step 5: Commit**

```bash
git add cmd/fields/fwupdate.go cmd/fields/version.go cmd/fields/osserver_test.go
git commit -m "fields: add unifi-os-server release lookups"
```

---

### Task 2: OCI extraction (`oci.go`)

**Files:**
- Create: `cmd/fields/oci.go`
- Test: `cmd/fields/oci_test.go` (new), `cmd/fields/testhelpers_test.go` (new)
- Modify: `go.mod`/`go.sum` (new dep)

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/google/go-containerregistry@v0.21.7`
Expected: added to go.mod

- [ ] **Step 2: Write the failing tests**

Create `cmd/fields/testhelpers_test.go`:

```go
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"
)

// buildTestZip returns zip bytes containing a single file.
func buildTestZip(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// buildTestAceJar returns ace.jar bytes: an internal-dependencies.jar (with
// the given files) plus product.properties carrying the given version.
func buildTestAceJar(t *testing.T, files map[string]string, networkVersion string) []byte {
	t.Helper()

	var ibuf bytes.Buffer
	izw := zip.NewWriter(&ibuf)
	for name, content := range files {
		w, err := izw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, izw.Close())

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("BOOT-INF/lib/internal-dependencies.jar")
	require.NoError(t, err)
	_, err = w.Write(internal)
	require.NoError(t, err)
	pw, err := zw.Create("BOOT-INF/classes/product.properties")
	require.NoError(t, err)
	_, err = pw.Write([]byte("product=UniFi\nversion=" + networkVersion + "\n"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// writeTestLayout writes an OCI image layout whose single layer contains the
// given files, returning the layout dir.
func writeTestLayout(t *testing.T, files map[string][]byte) string {
	t.Helper()

	var layerBuf bytes.Buffer
	tw := tar.NewWriter(&layerBuf)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(layerBuf.Bytes())), nil
	})
	require.NoError(t, err)

	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	dir := t.TempDir()
	p, err := layout.Write(dir, empty.Index)
	require.NoError(t, err)
	require.NoError(t, p.AppendImage(img))
	return dir
}

// tarDir returns dir's contents as tar bytes (paths relative to dir).
func tarDir(t *testing.T, dir string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		require.NoError(t, err)
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     rel,
			Mode:     0o644,
			Size:     int64(len(b)),
			Typeflag: tar.TypeReg,
		}))
		_, err = tw.Write(b)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

// writeTestInstaller writes a fake self-extracting installer (shell stub +
// appended zip holding image.tar) and returns its path.
func writeTestInstaller(t *testing.T, imageTar []byte) string {
	t.Helper()
	installer := filepath.Join(t.TempDir(), "installer")
	content := append([]byte("#!/bin/sh\n# fake self-extract stub\nexit 0\n"), buildTestZip(t, "image.tar", imageTar)...)
	require.NoError(t, os.WriteFile(installer, content, 0o755))
	return installer
}
```

Create `cmd/fields/oci_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractImageTar(t *testing.T) {
	installer := writeTestInstaller(t, []byte("fake-tar-content"))
	dir := t.TempDir()

	require.NoError(t, extractImageTar(installer, dir))

	b, err := os.ReadFile(filepath.Join(dir, "image.tar"))
	require.NoError(t, err)
	assert.Equal(t, "fake-tar-content", string(b))
}

func TestExtractImageTarMissing(t *testing.T) {
	installer := writeTestInstaller(t, []byte("x"))
	// overwrite with a zip that has no image.tar
	require.NoError(t, os.WriteFile(installer,
		append([]byte("#!/bin/sh\n"), buildTestZip(t, "other.bin", []byte("x"))...), 0o755))

	err := extractImageTar(installer, t.TempDir())
	assert.ErrorContains(t, err, "image.tar")
}

func TestUntarLayoutAndFindAceJar(t *testing.T) {
	aceJar := buildTestAceJar(t, map[string]string{
		"api/fields/NetworkConf.json": `{"name":".{1,128}"}`,
	}, "10.4.57")

	layoutDir := writeTestLayout(t, map[string][]byte{
		"usr/lib/unifi/lib/ace.jar": aceJar,
		"usr/bin/bash":              []byte("not-a-jar"),
	})

	// round-trip through image.tar + untarLayout, like the real pipeline
	imageTar := tarDir(t, layoutDir)
	untarDir := t.TempDir()
	imageTarPath := filepath.Join(t.TempDir(), "image.tar")
	require.NoError(t, os.WriteFile(imageTarPath, imageTar, 0o644))
	require.NoError(t, untarLayout(imageTarPath, untarDir))

	got, err := findAceJar(untarDir, t.TempDir())
	require.NoError(t, err)

	b, err := os.ReadFile(got)
	require.NoError(t, err)
	assert.Equal(t, aceJar, b)
}

func TestFindAceJarMissing(t *testing.T) {
	layoutDir := writeTestLayout(t, map[string][]byte{
		"usr/bin/bash": []byte("nope"),
	})
	_, err := findAceJar(layoutDir, t.TempDir())
	assert.ErrorContains(t, err, "ace.jar")
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./cmd/fields/ -run 'ImageTar|Layout|AceJar' -v`
Expected: FAIL — `undefined: extractImageTar` etc.

- [ ] **Step 4: Implement `cmd/fields/oci.go`**

```go
package main

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
)

const aceJarLayerPath = "usr/lib/unifi/lib/ace.jar"

// extractImageTar extracts image.tar from the zip archive appended to the
// UniFi OS Server installer ELF. Go's archive/zip locates the central
// directory via the end record (baseOffset), so the prepended ELF stub needs
// no special handling.
func extractImageTar(installerPath, dir string) error {
	f, err := os.Open(installerPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return fmt.Errorf("unable to open installer zip payload: %w", err)
	}

	for _, zf := range zr.File {
		if zf.Name != "image.tar" {
			continue
		}
		src, err := zf.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.Create(filepath.Join(dir, "image.tar"))
		if err != nil {
			return err
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("unable to extract image.tar: %w", err)
		}
		return nil
	}

	return errors.New("image.tar not found in installer zip payload")
}

// untarLayout extracts an OCI image layout tar into dir.
func untarLayout(tarPath, dir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		clean := filepath.Clean(header.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("unsafe path in image.tar: %q", header.Name)
		}
		dest := filepath.Join(dir, clean)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			out, err := os.Create(dest)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
}

// findAceJar walks the OCI image layout at layoutDir and extracts ace.jar
// into workDir, returning the temp file path.
func findAceJar(layoutDir, workDir string) (string, error) {
	ii, err := layout.ImageIndexFromPath(layoutDir)
	if err != nil {
		return "", fmt.Errorf("unable to read OCI image layout: %w", err)
	}

	manifest, err := ii.IndexManifest()
	if err != nil {
		return "", err
	}

	for _, m := range manifest.Manifests {
		img, err := ii.Image(m.Digest)
		if err != nil {
			return "", err
		}

		layers, err := img.Layers()
		if err != nil {
			return "", err
		}

		for _, layer := range layers {
			path, err := extractAceJarFromLayer(layer, workDir)
			if err != nil {
				return "", err
			}
			if path != "" {
				return path, nil
			}
		}
	}

	return "", errors.New("ace.jar not found in any image layer")
}

func extractAceJarFromLayer(layer v1.Layer, workDir string) (string, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if strings.TrimPrefix(header.Name, "./") != aceJarLayerPath {
			continue
		}

		aceJar, err := os.CreateTemp(workDir, "ace-*.jar")
		if err != nil {
			return "", err
		}
		defer aceJar.Close()

		if _, err := io.Copy(aceJar, tr); err != nil {
			return "", err
		}
		return aceJar.Name(), nil
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./cmd/fields/ -run 'ImageTar|Layout|AceJar' -v && go build ./...`
Expected: PASS (4 tests), build OK

- [ ] **Step 6: Commit**

```bash
git add cmd/fields/oci.go cmd/fields/oci_test.go cmd/fields/testhelpers_test.go go.mod go.sum
git commit -m "fields: extract image.tar and ace.jar from os-server installer"
```

---

### Task 3: ace.jar processing (`acejar.go`)

**Files:**
- Create: `cmd/fields/acejar.go`
- Test: `cmd/fields/acejar_test.go` (new)

- [ ] **Step 1: Write the failing tests**

Create `cmd/fields/acejar_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, content, 0o644))
	return p
}

func TestFindInternalJar(t *testing.T) {
	aceJar := writeTempFile(t, "ace.jar", buildTestAceJar(t, map[string]string{
		"api/fields/NetworkConf.json": `{}`,
	}, "10.4.57"))

	internal, err := findInternalJar(aceJar)
	require.NoError(t, err)
	assert.Contains(t, string(internal), "PK")
}

func TestReadNetworkVersion(t *testing.T) {
	aceJar := writeTempFile(t, "ace.jar", buildTestAceJar(t, nil, "10.4.57"))

	v, err := readNetworkVersion(aceJar)
	require.NoError(t, err)
	assert.Equal(t, "10.4.57", v)
}

func TestExtractDefs(t *testing.T) {
	aceJar := writeTempFile(t, "ace.jar", buildTestAceJar(t, map[string]string{
		"api/fields/NetworkConf.json":    `{"name":".{1,128}"}`,
		"api/fields/WlanConf.json":       `{"ssid":".{1,32}"}`,
		"sensitive_metadata.json":        `{"sensitive_db_fields_by_collection":{}}`,
		"timezones.json":                 `[]`,
		"com/ubnt/SomeClass.class":       "CAFEBABE",
		"META-INF/MANIFEST.MF":           "Manifest-Version: 1.0",
	}, "10.4.57"))

	internal, err := findInternalJar(aceJar)
	require.NoError(t, err)

	fieldsDir := t.TempDir()
	n, err := extractDefs(internal, fieldsDir)
	require.NoError(t, err)
	assert.Equal(t, 4, n)

	// api/fields flattened into root
	b, err := os.ReadFile(filepath.Join(fieldsDir, "NetworkConf.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":".{1,128}"}`, string(b))
	assert.FileExists(t, filepath.Join(fieldsDir, "WlanConf.json"))

	// top-level metadata into metadata/
	assert.FileExists(t, filepath.Join(fieldsDir, "metadata", "sensitive_metadata.json"))
	assert.FileExists(t, filepath.Join(fieldsDir, "metadata", "timezones.json"))

	// class files and manifests ignored
	assert.NoFileExists(t, filepath.Join(fieldsDir, "MANIFEST.MF"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/fields/ -run 'InternalJar|NetworkVersion|ExtractDefs' -v`
Expected: FAIL — `undefined: findInternalJar` etc.

- [ ] **Step 3: Implement `cmd/fields/acejar.go`**

```go
package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// keepTopLevelJSON are the top-level JSON definition files extracted from
// internal-dependencies.jar into <fieldsDir>/metadata/.
var keepTopLevelJSON = []string{
	"legacy_endpoint_segments.json",
	"event_defs.json",
	"sensitive_metadata.json",
	"radio_specification.json",
	"country_codes_list.json",
	"geo_ip_country_codes_list.json",
	"timezones.json",
	"ssl-inspection-file-extension.json",
}

// findInternalJar reads internal-dependencies.jar out of ace.jar (a Spring
// Boot fat jar).
func findInternalJar(aceJarPath string) ([]byte, error) {
	zr, err := zip.OpenReader(aceJarPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open ace.jar: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != "BOOT-INF/lib/internal-dependencies.jar" &&
			!strings.Contains(f.Name, "internal-dependencies") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}

	return nil, errors.New("internal-dependencies.jar not found in ace.jar")
}

// extractDefs writes api/fields/*.json flattened into fieldsDir and the
// keepTopLevelJSON files into fieldsDir/metadata/. Returns the file count.
func extractDefs(internalJar []byte, fieldsDir string) (int, error) {
	zr, err := zip.NewReader(bytes.NewReader(internalJar), int64(len(internalJar)))
	if err != nil {
		return 0, fmt.Errorf("unable to open internal-dependencies.jar: %w", err)
	}

	n := 0
	for _, f := range zr.File {
		var dest string
		switch {
		case strings.HasPrefix(f.Name, "api/fields/") && path.Ext(f.Name) == ".json":
			dest = filepath.Join(fieldsDir, path.Base(f.Name))
		case slices.Contains(keepTopLevelJSON, f.Name):
			dest = filepath.Join(fieldsDir, "metadata", f.Name)
		default:
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return n, err
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return n, err
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return n, err
		}
		if err := os.WriteFile(dest, b, 0o644); err != nil {
			return n, err
		}
		n++
	}

	return n, nil
}

var productVersionRe = regexp.MustCompile(`(?m)^version=(.+)$`)

// readNetworkVersion reads the UniFi Network version from product.properties
// in ace.jar.
func readNetworkVersion(aceJarPath string) (string, error) {
	zr, err := zip.OpenReader(aceJarPath)
	if err != nil {
		return "", fmt.Errorf("unable to open ace.jar: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != "BOOT-INF/classes/product.properties" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		m := productVersionRe.FindSubmatch(b)
		if m == nil {
			return "", errors.New("version= not found in product.properties")
		}
		return strings.TrimSpace(string(m[1])), nil
	}

	return "", errors.New("product.properties not found in ace.jar")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/fields/ -run 'InternalJar|NetworkVersion|ExtractDefs' -v && go build ./...`
Expected: PASS, build OK

- [ ] **Step 5: Commit**

```bash
git add cmd/fields/acejar.go cmd/fields/acejar_test.go
git commit -m "fields: extract api defs from internal-dependencies.jar"
```

---

### Task 4: Installer orchestration (`installer.go`)

**Files:**
- Create: `cmd/fields/installer.go`
- Test: `cmd/fields/installer_test.go` (new)

- [ ] **Step 1: Write the failing tests**

Create `cmd/fields/installer_test.go`:

```go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadInstaller(t *testing.T) {
	payload := []byte("fake installer bytes")
	sum := sha256.Sum256(payload)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, _ = rw.Write(payload)
	}))
	t.Cleanup(server.Close)

	path, err := downloadInstaller(server.URL, hex.EncodeToString(sum[:]), t.TempDir())
	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestDownloadInstallerHashMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, _ = rw.Write([]byte("payload"))
	}))
	t.Cleanup(server.Close)

	_, err := downloadInstaller(server.URL, "0000000000000000000000000000000000000000000000000000000000000000", t.TempDir())
	assert.ErrorContains(t, err, "sha256 mismatch")
}

func TestExtractInstallerDefsEndToEnd(t *testing.T) {
	aceJar := buildTestAceJar(t, map[string]string{
		"api/fields/NetworkConf.json": `{"name":".{1,128}"}`,
		"sensitive_metadata.json":     `{"sensitive_db_fields_by_collection":{}}`,
	}, "10.4.57")
	layoutDir := writeTestLayout(t, map[string][]byte{
		"usr/lib/unifi/lib/ace.jar": aceJar,
	})
	installer := writeTestInstaller(t, tarDir(t, layoutDir))

	staging, networkVersion, err := extractInstallerDefs(installer, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "10.4.57", networkVersion)
	assert.FileExists(t, filepath.Join(staging, "NetworkConf.json"))
	assert.FileExists(t, filepath.Join(staging, "metadata", "sensitive_metadata.json"))
}

func TestPublishAndFindCachedFieldsDir(t *testing.T) {
	base := t.TempDir()
	staging := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(staging, "NetworkConf.json"), []byte(`{}`), 0o644))

	fieldsDir, err := publishFieldsDir(staging, base, "10.4.57", sourceInfo{
		OsServerVersion: "5.1.21",
		NetworkVersion:  "10.4.57",
		URL:             "https://example/installer",
		SHA256:          "abc",
	})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(base, "v10.4.57"), fieldsDir)
	assert.FileExists(t, filepath.Join(fieldsDir, "NetworkConf.json"))
	assert.FileExists(t, filepath.Join(fieldsDir, "source.json"))

	assert.Equal(t, fieldsDir, findCachedFieldsDir(base, "5.1.21", ""))
	assert.Equal(t, fieldsDir, findCachedFieldsDir(base, "", "https://example/installer"))
	assert.Empty(t, findCachedFieldsDir(base, "9.9.9", ""))
	assert.Empty(t, findCachedFieldsDir(base, "", "https://example/other"))
}

func TestParseOsServerVersionFromName(t *testing.T) {
	assert.Equal(t, "5.1.21", parseOsServerVersionFromName("f5e2-linux-x64-5.1.21-a400c9c6-8328-4634-b223-ebfcf742720a.21-x64"))
	assert.Equal(t, "5.0.8", parseOsServerVersionFromName("162a-linux-arm64-5.0.8-c2775845.8-arm64"))
	assert.Empty(t, parseOsServerVersionFromName("installer"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/fields/ -run 'DownloadInstaller|ExtractInstaller|PublishAndFind|ParseOsServer' -v`
Expected: FAIL — `undefined: downloadInstaller` etc.

- [ ] **Step 3: Implement `cmd/fields/installer.go`**

```go
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// sourceInfo records where a fields dir came from. Written to
// <fieldsDir>/source.json and used for cache lookups.
type sourceInfo struct {
	OsServerVersion string `json:"os_server_version,omitempty"`
	NetworkVersion  string `json:"network_version"`
	URL             string `json:"url,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
}

// downloadInstaller fetches rawURL into workDir, verifying sha256Hex when
// non-empty, and returns the temp file path.
func downloadInstaller(rawURL, sha256Hex, workDir string) (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unable to download installer: %s", resp.Status)
	}

	dst, err := os.CreateTemp(workDir, "installer-*")
	if err != nil {
		return "", err
	}
	defer dst.Close()

	h := sha256.New()
	if _, err := io.Copy(dst, io.TeeReader(resp.Body, h)); err != nil {
		return "", err
	}

	if sha256Hex != "" {
		if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, sha256Hex) {
			return "", fmt.Errorf("installer sha256 mismatch: got %s, want %s", got, sha256Hex)
		}
	}

	return dst.Name(), nil
}

// extractInstallerDefs runs the full installer pipeline (zip payload →
// image.tar → OCI layout → ace.jar → internal-dependencies.jar) into a
// staging dir, returning (stagingDir, networkVersion, error).
func extractInstallerDefs(installerPath, workDir string) (string, string, error) {
	if err := extractImageTar(installerPath, workDir); err != nil {
		return "", "", err
	}

	layoutDir := filepath.Join(workDir, "image")
	if err := untarLayout(filepath.Join(workDir, "image.tar"), layoutDir); err != nil {
		return "", "", err
	}

	aceJar, err := findAceJar(layoutDir, workDir)
	if err != nil {
		return "", "", err
	}

	networkVersion, err := readNetworkVersion(aceJar)
	if err != nil {
		return "", "", err
	}

	internal, err := findInternalJar(aceJar)
	if err != nil {
		return "", "", err
	}

	staging, err := os.MkdirTemp(workDir, "staging-fields-")
	if err != nil {
		return "", "", err
	}

	n, err := extractDefs(internal, staging)
	if err != nil {
		return "", "", err
	}
	fmt.Printf("extracted %d definition files (network %s)\n", n, networkVersion)

	return staging, networkVersion, nil
}

// publishFieldsDir moves the staging dir to versionBaseDir/v<networkVersion>
// (replacing any existing one) and writes source.json.
func publishFieldsDir(staging, versionBaseDir, networkVersion string, info sourceInfo) (string, error) {
	fieldsDir := filepath.Join(versionBaseDir, fmt.Sprintf("v%s", networkVersion))
	if err := os.RemoveAll(fieldsDir); err != nil {
		return "", err
	}
	if err := os.Rename(staging, fieldsDir); err != nil {
		return "", fmt.Errorf("unable to publish fields dir: %w", err)
	}

	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(fieldsDir, "source.json"), b, 0o644); err != nil {
		return "", err
	}

	return fieldsDir, nil
}

// findCachedFieldsDir returns the existing v*/ dir under versionBaseDir whose
// source.json matches osServerVersion or rawURL (first non-empty match), or
// "" if none.
func findCachedFieldsDir(versionBaseDir, osServerVersion, rawURL string) string {
	entries, err := os.ReadDir(versionBaseDir)
	if err != nil {
		return ""
	}

	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "v") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(versionBaseDir, e.Name(), "source.json"))
		if err != nil {
			continue
		}
		var info sourceInfo
		if json.Unmarshal(b, &info) != nil {
			continue
		}
		if osServerVersion != "" && info.OsServerVersion == osServerVersion {
			return filepath.Join(versionBaseDir, e.Name())
		}
		if rawURL != "" && info.URL == rawURL {
			return filepath.Join(versionBaseDir, e.Name())
		}
	}

	return ""
}

var osServerNameRe = regexp.MustCompile(`linux-(?:x64|arm64)-([0-9]+(?:\.[0-9]+)*)-`)

// parseOsServerVersionFromName extracts the OS Server version from an
// installer filename, e.g. "f5e2-linux-x64-5.1.21-a400c9c6.21-x64" →
// "5.1.21". Returns "" when the name doesn't match.
func parseOsServerVersionFromName(name string) string {
	m := osServerNameRe.FindStringSubmatch(name)
	if m == nil {
		return ""
	}
	return m[1]
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/fields/ -run 'DownloadInstaller|ExtractInstaller|PublishAndFind|ParseOsServer' -v && go build ./...`
Expected: PASS (5 tests), build OK

- [ ] **Step 5: Commit**

```bash
git add cmd/fields/installer.go cmd/fields/installer_test.go
git commit -m "fields: orchestrate installer download and fields extraction"
```

---

### Task 5: Shared post-processing (`extract.go` refactor)

**Files:**
- Modify: `cmd/fields/extract.go`
- Modify: `cmd/fields/main.go` (only the download/cache block calls)
- Test: `cmd/fields/extract_test.go` (new)

Background: `extractJSON` currently (a) pulls `api/fields/*.json` out of ace.jar, (b) splits `Setting.json` into `Setting*.json`, (c) copies `custom/*.json` — and then main.go calls `copyCustom` a second time. Split (a) [deb-specific] from (b)+(c) [shared], so the installer path can reuse (b)+(c).

- [ ] **Step 1: Write the failing test**

Create `cmd/fields/extract_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostProcessFieldsDir(t *testing.T) {
	fieldsDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(fieldsDir, "Setting.json"),
		[]byte(`{"usg":{"mdns_enabled":"true|false"},"radius":{"enabled":"true|false"}}`),
		0o644,
	))

	require.NoError(t, postProcessFieldsDir(fieldsDir))

	b, err := os.ReadFile(filepath.Join(fieldsDir, "SettingUsg.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"mdns_enabled":"true|false"}`, string(b))
	assert.FileExists(t, filepath.Join(fieldsDir, "SettingRadius.json"))

	// custom files copied from cmd/fields/custom
	assert.FileExists(t, filepath.Join(fieldsDir, "DnsRecord.json"))
	assert.FileExists(t, filepath.Join(fieldsDir, "FirewallPolicy.json"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/fields/ -run 'PostProcess' -v`
Expected: FAIL — `undefined: postProcessFieldsDir`

- [ ] **Step 3: Implement the split**

In `cmd/fields/extract.go`: keep `downloadJar` and `copyCustom` and `findModuleRoot` as-is. Replace `extractJSON` so it only does the ace.jar `api/fields` extraction (delete the `Setting.json` handling block and the trailing custom-copy block — the one reading `srcDir/custom` inline). Then add:

```go
// postProcessFieldsDir applies the shared post-extraction steps used by both
// the deb and installer paths: split Setting.json into per-key Setting*.json
// files and copy the hand-written custom/*.json definitions.
func postProcessFieldsDir(fieldsDir string) error {
	settingsData, err := os.ReadFile(filepath.Join(fieldsDir, "Setting.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("unable to open settings file: %w", err)
	}
	if err == nil {
		var settings map[string]any
		if err := json.Unmarshal(settingsData, &settings); err != nil {
			return fmt.Errorf("unable to unmarshal settings: %w", err)
		}

		for k, v := range settings {
			fileName := fmt.Sprintf("Setting%s.json", strcase.ToCamel(k))

			data, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return fmt.Errorf("unable to marshal setting %q: %w", k, err)
			}

			if err := os.WriteFile(filepath.Join(fieldsDir, fileName), data, 0o644); err != nil {
				return fmt.Errorf("unable to write new settings file: %w", err)
			}
		}
	}

	if err := copyCustom(fieldsDir); err != nil {
		return err
	}

	return nil
}
```

In `cmd/fields/main.go`, find the block:

```go
		err = extractJSON(jarFile, fieldsDir)
		if err != nil {
			panic(err)
		}

		// defer func() {
		// ...
		// }()

		err = copyCustom(fieldsDir)
		if err != nil {
			panic(err)
		}

		fieldsInfo, err = os.Stat(fieldsDir)
		if err != nil {
			panic(err)
		}
	}
```

Replace with (drop the dead defer comment and the duplicate copyCustom call; add the postProcess call after the cache block):

```go
		err = extractJSON(jarFile, fieldsDir)
		if err != nil {
			panic(err)
		}

		fieldsInfo, err = os.Stat(fieldsDir)
		if err != nil {
			panic(err)
		}
	}

	err = postProcessFieldsDir(fieldsDir)
	if err != nil {
		panic(err)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/fields/ && go build ./...`
Expected: all PASS (postProcess runs against cwd inside the repo — `findModuleRoot` locates `cmd/fields/custom` during tests)

- [ ] **Step 5: Commit**

```bash
git add cmd/fields/extract.go cmd/fields/main.go cmd/fields/extract_test.go
git commit -m "fields: share settings split and custom defs across fetch paths"
```

---

### Task 6: CLI wiring (`main.go`) + version constants

**Files:**
- Modify: `cmd/fields/main.go`
- Modify: `cmd/fields/version.go` (delete `latestUnifiVersion`)
- Modify: `cmd/fields/fwupdate.go` (delete `firmwareUpdateApi` var + `maxVersion` const)
- Delete: `cmd/fields/version_test.go` (tests the deleted function)
- Test: `cmd/fields/main_test.go` (add mode tests)

- [ ] **Step 1: Write the failing tests**

Append to `cmd/fields/main_test.go`:

```go
func TestResolveMode(t *testing.T) {
	cases := []struct {
		name      string
		pos       string
		latest    bool
		osServer  string
		rawURL    string
		installer string
		want      sourceMode
		wantErr   bool
	}{
		{"deb", "9.5.21", false, "", "", "", modeDeb, false},
		{"latest", "", true, "", "", "", modeInstallerLatest, false},
		{"os-server", "", false, "5.1.21", "", "", modeInstallerVersion, false},
		{"url", "", false, "", "https://x/y", "", modeInstallerURL, false},
		{"installer", "", false, "", "", "/tmp/i", modeInstallerLocal, false},
		{"none", "", false, "", "", "", 0, true},
		{"version and latest", "9.5.21", true, "", "", "", 0, true},
		{"url and installer", "", false, "", "https://x/y", "/tmp/i", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveMode(tc.pos, tc.latest, tc.osServer, tc.rawURL, tc.installer)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/fields/ -run 'ResolveMode' -v`
Expected: FAIL — `undefined: resolveMode`

- [ ] **Step 3: Implement the wiring**

In `cmd/fields/main.go`:

1. Add the mode type + resolver (top level, after the imports):

```go
type sourceMode int

const (
	modeDeb sourceMode = iota
	modeInstallerLatest
	modeInstallerVersion
	modeInstallerURL
	modeInstallerLocal
)

func resolveMode(positional string, latest bool, osServer, rawURL, installer string) (sourceMode, error) {
	n := 0
	for _, set := range []bool{positional != "", latest, osServer != "", rawURL != "", installer != ""} {
		if set {
			n++
		}
	}
	if n != 1 {
		return 0, fmt.Errorf("specify exactly one of: version, -latest, -os-server, -url, -installer")
	}
	switch {
	case positional != "":
		return modeDeb, nil
	case latest:
		return modeInstallerLatest, nil
	case osServer != "":
		return modeInstallerVersion, nil
	case rawURL != "":
		return modeInstallerURL, nil
	default:
		return modeInstallerLocal, nil
	}
}
```

2. Add the new flags in `main()` next to the existing ones:

```go
	osServerFlag := flag.String(
		"os-server",
		"",
		"Fetch a specific UniFi OS Server release (installer path), e.g. 5.1.21",
	)
	urlFlag := flag.String(
		"url",
		"",
		"Direct UniFi OS Server installer URL (installer path)",
	)
	installerFlag := flag.String(
		"installer",
		"",
		"Path to a local UniFi OS Server installer (no download)",
	)
```

3. Replace the old validation block:

```go
	specifiedVersion := flag.Arg(0)
	if specifiedVersion != "" && *useLatestVersion {
		fmt.Print("error: cannot specify version with latest\n\n")
		usage()
		os.Exit(1)
	} else if specifiedVersion == "" && !*useLatestVersion {
		fmt.Print("error: must specify version or latest\n\n")
		usage()
		os.Exit(1)
	}
```

and the old version/URL block:

```go
	var unifiVersion *version.Version
	var unifiDownloadUrl *url.URL
	var err error

	if *useLatestVersion {
		unifiVersion, unifiDownloadUrl, err = latestUnifiVersion()
		if err != nil {
			panic(err)
		}
	} else {
		unifiVersion, err = version.NewVersion(specifiedVersion)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		unifiDownloadUrl, err = url.Parse(fmt.Sprintf("https://dl.ui.com/unifi/%s/unifi_sysvinit_all.deb", unifiVersion))
		if err != nil {
			panic(err)
		}
	}
```

with:

```go
	mode, err := resolveMode(flag.Arg(0), *useLatestVersion, *osServerFlag, *urlFlag, *installerFlag)
	if err != nil {
		fmt.Printf("error: %s\n\n", err)
		usage()
		os.Exit(1)
	}

	var unifiVersion *version.Version
	osServerVersion := ""
```

4. Replace the fieldsDir download block (from `wd, err := os.Getwd()` down to the `if !fieldsInfo.IsDir()` check) with:

```go
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Unable to get the current filename")
	}

	versionBaseDir := filepath.Dir(filename)
	outDir := filepath.Join(wd, *outputDirFlag)

	var fieldsDir string

	switch mode {
	case modeDeb:
		unifiVersion, err = version.NewVersion(flag.Arg(0))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		debURL, err := url.Parse(fmt.Sprintf("https://dl.ui.com/unifi/%s/unifi_sysvinit_all.deb", unifiVersion))
		if err != nil {
			panic(err)
		}

		fieldsDir = filepath.Join(versionBaseDir, fmt.Sprintf("v%s", unifiVersion))
		if _, err := os.Stat(fieldsDir); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				panic(err)
			}

			if err := os.MkdirAll(fieldsDir, 0o755); err != nil {
				panic(err)
			}

			jarFile, err := downloadJar(debURL, fieldsDir)
			if err != nil {
				panic(err)
			}

			if err := extractJSON(jarFile, fieldsDir); err != nil {
				panic(err)
			}
		}

	case modeInstallerLatest, modeInstallerVersion, modeInstallerURL, modeInstallerLocal:
		// The work dir lives under versionBaseDir so publishFieldsDir's
		// os.Rename never crosses filesystems (EXDEV on tmpfs /tmp,
		// redirected $TMPDIR, or a repo on a separate mount).
		workDir, err := os.MkdirTemp(versionBaseDir, ".unifi-fields-")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(workDir)

		var rel *osServerRelease
		switch mode {
		case modeInstallerLatest:
			rel, err = latestOsServerRelease()
		case modeInstallerVersion:
			rel, err = findOsServerRelease(*osServerFlag)
		}
		if err != nil {
			panic(err)
		}

		installerPath := *installerFlag
		dlURL, dlSHA := *urlFlag, ""
		if rel != nil {
			osServerVersion = rel.Version.String()
			dlURL, dlSHA = rel.URL.String(), rel.SHA256
			if cached := findCachedFieldsDir(versionBaseDir, osServerVersion, ""); cached != "" {
				fmt.Printf("reusing cached %s\n", cached)
				fieldsDir = cached
				break
			}
			fmt.Printf("fetching UniFi OS Server %s: %s\n", osServerVersion, dlURL)
		}

		if mode == modeInstallerLocal {
			osServerVersion = parseOsServerVersionFromName(filepath.Base(installerPath))
		}
		if mode == modeInstallerURL {
			if cached := findCachedFieldsDir(versionBaseDir, "", dlURL); cached != "" {
				fmt.Printf("reusing cached %s\n", cached)
				fieldsDir = cached
				break
			}
		}

		if installerPath == "" {
			installerPath, err = downloadInstaller(dlURL, dlSHA, workDir)
			if err != nil {
				panic(err)
			}
		}

		staging, networkVersion, err := extractInstallerDefs(installerPath, workDir)
		if err != nil {
			panic(err)
		}

		unifiVersion, err = version.NewVersion(networkVersion)
		if err != nil {
			panic(fmt.Errorf("unable to parse network version %q: %w", networkVersion, err))
		}

		fieldsDir, err = publishFieldsDir(staging, versionBaseDir, networkVersion, sourceInfo{
			OsServerVersion: osServerVersion,
			NetworkVersion:  networkVersion,
			URL:             dlURL,
			SHA256:          dlSHA,
		})
		if err != nil {
			panic(err)
		}
	}

	if fieldsDir == "" {
		panic("no fields dir resolved")
	}

	fieldsInfo, err := os.Stat(fieldsDir)
	if err != nil {
		panic(err)
	}
	if !fieldsInfo.IsDir() {
		panic("version info isn't a directory")
	}

	err = postProcessFieldsDir(fieldsDir)
	if err != nil {
		panic(err)
	}
```

Note: the `break` inside `if rel != nil` / `if mode == modeInstallerURL` breaks the `switch`, skipping straight to the shared `postProcessFieldsDir` — `unifiVersion` stays nil on the cache-hit path, so read the version back for the version file: replace the version-file write (step 5 below) accordingly.

5. Replace the version-file block:

```go
	// Write version file.
	versionGo := fmt.Appendf(nil, `
// Generated code. DO NOT EDIT.

package unifi

const UnifiVersion = %q
`, unifiVersion)
```

with:

```go
	// Write version file. On a fields-dir cache hit, unifiVersion is nil;
	// recover both versions from source.json.
	if unifiVersion == nil {
		b, err := os.ReadFile(filepath.Join(fieldsDir, "source.json"))
		if err != nil {
			panic(fmt.Errorf("fields dir cache hit without version: %w", err))
		}
		var info sourceInfo
		if err := json.Unmarshal(b, &info); err != nil {
			panic(err)
		}
		unifiVersion, err = version.NewVersion(info.NetworkVersion)
		if err != nil {
			panic(err)
		}
		osServerVersion = info.OsServerVersion
	}

	versionGo := fmt.Appendf(nil, `
// Generated code. DO NOT EDIT.

package unifi

const UnifiVersion = %q

const UnifiOsServerVersion = %q
`, unifiVersion, osServerVersion)
```

(`encoding/json` is already imported in main.go.)

6. Exclude `source.json` from codegen. In the `for _, fieldsFile := range fieldsFiles` loop, extend the skip switch:

```go
		switch name {
		case "AuthenticationRequest.json", "Setting.json", "Wall.json", "source.json":
			continue
		}
```

7. Delete `latestUnifiVersion` from `cmd/fields/version.go`, delete the `firmwareUpdateApi` var and `maxVersion` const from `cmd/fields/fwupdate.go`, and delete `cmd/fields/version_test.go`:

```bash
git rm cmd/fields/version_test.go
```

8. Update `usage()`'s first line to mention the installer modes:

```go
func usage() {
	fmt.Printf("Usage: %s [OPTIONS] [version]\n", path.Base(os.Args[0]))
	fmt.Println("Sources (exactly one): version (deb), -latest, -os-server, -url, -installer")
	flag.PrintDefaults()
}
```

9. Add the work-dir pattern to `cmd/fields/.gitignore` (a SIGKILLed run could leave one behind):

```
/.unifi-fields-*/
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./... && go test ./cmd/fields/`
Expected: build OK, all PASS

- [ ] **Step 5: Smoke-test the CLI locally (no network)**

Run: `go run ./cmd/fields -installer /does/not/exist -download-only`
Expected: fails with a file-not-found error (proves flag parsing + installer path wiring)

Run: `go run ./cmd/fields -latest 9.5.21`
Expected: `error: specify exactly one of: ...` (mutual exclusion)

- [ ] **Step 6: Commit**

```bash
git add cmd/fields/main.go cmd/fields/main_test.go cmd/fields/version.go cmd/fields/fwupdate.go
git commit -m "fields: add installer source modes to the cli"
```

---

### Task 7: Sensitive metadata marking (`sensitive.go`)

**Files:**
- Create: `cmd/fields/sensitive.go`
- Modify: `cmd/fields/main.go` (`FieldInfo` struct + marking call in the codegen loop)
- Test: `cmd/fields/sensitive_test.go` (new)

- [ ] **Step 1: Write the failing tests**

Create `cmd/fields/sensitive_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

func TestLoadSensitiveMetadataAbsent(t *testing.T) {
	meta, err := loadSensitiveMetadata(t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, meta)
}

func TestCollectionForResource(t *testing.T) {
	assert.Equal(t, "networkconf", collectionForResource("NetworkConf.json", "Network"))
	assert.Equal(t, "wlanconf", collectionForResource("WlanConf.json", "WLAN"))
	assert.Equal(t, "setting", collectionForResource("SettingUsg.json", "SettingUsg"))
	assert.Equal(t, "user", collectionForResource("User.json", "Client"))
	assert.Equal(t, "usergroup", collectionForResource("UserGroup.json", "ClientGroup"))
	assert.Equal(t, "site", collectionForResource("Site.json", "Site"))
}

func TestMarkResource(t *testing.T) {
	r := NewResource("RadiusProfile", "radiusprofile")
	base := r.Types["RadiusProfile"]

	auth := NewFieldInfo("AuthServers", "auth_servers", "RadiusProfileAuthServers", "", true, false, false, "")
	auth.Fields = map[string]*FieldInfo{
		"IP":      NewFieldInfo("IP", "ip", fields.String, "", true, false, false, ""),
		"XSecret": NewFieldInfo("XSecret", "x_secret", fields.String, "", true, false, false, ""),
	}
	base.Fields["AuthServers"] = auth
	base.Fields["XPassphrase"] = NewFieldInfo("XPassphrase", "x_passphrase", fields.String, "", true, false, false, "")
	base.Fields["Name"] = NewFieldInfo("Name", "name", fields.String, "", true, false, false, "")

	meta := &sensitiveMetadata{ByCollection: map[string][]string{
		"radiusprofile": {"name", "auth_servers.x_secret", "x_passphrase", "bogus.path"},
	}}

	meta.markResource(r, "radiusprofile")

	assert.False(t, base.Fields["Name"].Sensitive, "name is allowlisted")
	assert.True(t, base.Fields["XPassphrase"].Sensitive)
	assert.True(t, auth.Fields["XSecret"].Sensitive, "nested leaf")
	assert.False(t, auth.Fields["IP"].Sensitive)
	// bogus.path: logged and skipped, no panic
}

func TestMarkResourceUnknownCollection(t *testing.T) {
	r := NewResource("DnsRecord", "static-dns")
	meta := &sensitiveMetadata{ByCollection: map[string][]string{}}
	meta.markResource(r, "dnsrecord") // must not panic
}

func TestLoadSensitiveMetadata(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "metadata"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "metadata", "sensitive_metadata.json"),
		[]byte(`{"sensitive_db_fields_by_collection":{"wlanconf":["x_passphrase"]}}`),
		0o644,
	))

	meta, err := loadSensitiveMetadata(dir)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, []string{"x_passphrase"}, meta.ByCollection["wlanconf"])
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/fields/ -run 'Sensitive|MarkResource|CollectionFor' -v`
Expected: FAIL — `undefined: loadSensitiveMetadata` etc.

- [ ] **Step 3: Implement `cmd/fields/sensitive.go`**

```go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sensitiveMetadata mirrors the parts of upstream's sensitive_metadata.json
// we use. Ignored keys: min_field_size, default_names,
// sensitive_system_properties, sensitive_distinct_db_fields_by_collection.
type sensitiveMetadata struct {
	ByCollection map[string][]string `json:"sensitive_db_fields_by_collection"`
}

// sensitiveDisplayFields are identifiers/display metadata that UniFi flags
// sensitive for PII-redaction reasons. Marking them Sensitive in Terraform
// would hide resource names from plan output with no security benefit, so
// they are allowlisted out. Everything else upstream lists is treated as a
// secret (fail-safe: new upstream fields default to sensitive).
//
// If a field ever needs per-collection granularity, keys become
// "collection.field".
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

// loadSensitiveMetadata reads <fieldsDir>/metadata/sensitive_metadata.json.
// Returns (nil, nil) when absent (deb-sourced fields dirs).
func loadSensitiveMetadata(fieldsDir string) (*sensitiveMetadata, error) {
	b, err := os.ReadFile(filepath.Join(fieldsDir, "metadata", "sensitive_metadata.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var meta sensitiveMetadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, fmt.Errorf("unable to parse sensitive_metadata.json: %w", err)
	}
	return &meta, nil
}

// collectionForResource maps a resource to its UniFi DB collection: the
// lowercase basename of the source fields file, with exceptions for the
// resources whose struct names were cleaned up.
func collectionForResource(sourceFile, structName string) string {
	if strings.HasPrefix(structName, "Setting") {
		return "setting"
	}
	switch structName {
	case "Client":
		return "user"
	case "ClientGroup":
		return "usergroup"
	}
	return strings.ToLower(strings.TrimSuffix(sourceFile, ".json"))
}

// markResource flags the resource's fields listed for collection. Paths may
// be dot-separated ("auth_servers.x_secret") and are walked by JSON name.
// Drift (unknown collections/paths) is logged and skipped, never fatal.
func (m *sensitiveMetadata) markResource(r *ResourceInfo, collection string) {
	paths, ok := m.ByCollection[collection]
	if !ok {
		fmt.Printf("sensitive metadata: no entry for collection %q\n", collection)
		return
	}

	root := r.Types[r.StructName]
	for _, p := range paths {
		leaf := p
		if i := strings.LastIndex(p, "."); i >= 0 {
			leaf = p[i+1:]
		}
		if sensitiveDisplayFields[leaf] {
			continue
		}
		f := findFieldByJSONPath(root, p)
		if f == nil {
			fmt.Printf("sensitive metadata: %s.%s not found in schema\n", collection, p)
			continue
		}
		f.Sensitive = true
	}
}

// findFieldByJSONPath walks a dot-separated path of JSON field names from
// root, returning the leaf field or nil when any segment is missing.
func findFieldByJSONPath(root *FieldInfo, path string) *FieldInfo {
	cur := root
	for _, seg := range strings.Split(path, ".") {
		if cur == nil {
			return nil
		}
		var next *FieldInfo
		for _, f := range cur.Fields {
			if f != nil && f.JSONName == seg {
				next = f
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}
```

In `cmd/fields/main.go`, add the field to `FieldInfo`:

```go
type FieldInfo struct {
	FieldName           string
	JSONName            string
	FieldType           string
	IsPointer           bool
	FieldValidation     string
	OmitEmpty           bool
	IsArray             bool
	Sensitive           bool
	Fields              map[string]*FieldInfo
	CustomUnmarshalType string
	CustomUnmarshalFunc string
}
```

In `main()`, after `fieldsFiles` is read (before the codegen loop), load the metadata:

```go
	sensitiveMeta, err := loadSensitiveMetadata(fieldsDir)
	if err != nil {
		panic(err)
	}
```

In the codegen loop, after `err = resource.processJSON(b)`'s error check, add:

```go
		if sensitiveMeta != nil {
			collection := collectionForResource(fieldsFile.Name(), resource.StructName)
			sensitiveMeta.markResource(resource, collection)
		}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/fields/ -run 'Sensitive|MarkResource|CollectionFor' -v && go build ./...`
Expected: PASS (5 tests), build OK

- [ ] **Step 5: Commit**

```bash
git add cmd/fields/sensitive.go cmd/fields/sensitive_test.go cmd/fields/main.go
git commit -m "fields: flag sensitive fields from upstream metadata"
```

---

### Task 8: Sensitive flags in the spec (`schema.go`)

**Files:**
- Modify: `cmd/fields/schema.go`
- Test: `cmd/fields/schema_test.go` (append)

- [ ] **Step 1: Write the failing tests**

Append to `cmd/fields/schema_test.go`:

```go
func TestSpecificationGenerator_SensitiveAttributes(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")

	resource := NewResource("RadiusProfile", "radiusprofile")
	base := resource.Types["RadiusProfile"]
	auth := NewFieldInfo("AuthServers", "auth_servers", "RadiusProfileAuthServers", "", true, false, false, "")
	auth.Fields = map[string]*FieldInfo{
		"XSecret": NewFieldInfo("XSecret", "x_secret", "string", "", true, false, false, ""),
	}
	auth.Fields["XSecret"].Sensitive = true
	base.Fields["AuthServers"] = auth
	plain := NewFieldInfo("Name", "name", "string", "", false, false, false, "")
	base.Fields["Name"] = plain
	secret := NewFieldInfo("XPassword", "x_password", "string", "", true, false, false, "")
	secret.Sensitive = true
	base.Fields["XPassword"] = secret

	gen.AddResource(resource)
	spec := gen.Generate()

	require.Len(t, spec.Resources, 1)
	attrs := spec.Resources[0].Schema.Attributes

	find := func(name string) *struct{ found, sensitive bool } {
		for _, a := range attrs {
			if a.Name == name {
				s := false
				switch {
				case a.String != nil && a.String.Sensitive != nil:
					s = *a.String.Sensitive
				}
				return &struct{ found, sensitive bool }{true, s}
			}
		}
		return &struct{ found, sensitive bool }{}
	}

	assert.True(t, find("x_password").sensitive, "top-level sensitive")
	assert.False(t, find("name").sensitive, "not sensitive")

	// nested: auth_servers is a list_nested with x_secret inside
	for _, a := range attrs {
		if a.Name != "auth_servers" {
			continue
		}
		require.NotNil(t, a.ListNested)
		found := false
		for _, na := range a.ListNested.NestedObject.Attributes {
			if na.Name == "x_secret" {
				found = true
				require.NotNil(t, na.String)
				require.NotNil(t, na.String.Sensitive)
				assert.True(t, *na.String.Sensitive, "nested sensitive")
			}
		}
		assert.True(t, found, "x_secret nested attr present")
	}
}
```

Note: attribute names come from `toTerraformName` = `strings.ToLower(strcase.ToSnake(name))`, so `XPassword` → `x_password`, `AuthServers` → `auth_servers`, `XSecret` → `x_secret` — the names asserted above are correct as written.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/fields/ -run 'SensitiveAttributes' -v`
Expected: FAIL — `x_password` attr has nil Sensitive

- [ ] **Step 3: Implement in `cmd/fields/schema.go`**

Rename `fieldToResourceAttribute` → `buildResourceAttribute` and `fieldToDataSourceAttribute` → `buildDataSourceAttribute` (bodies unchanged; keep the recursive calls to `generateNested*Attributes` as-is — those call the *wrappers* below, so nested fields get marked too). Wait — careful: `generateNestedResourceAttributes` calls `g.fieldToResourceAttribute`. After the rename, it must call the wrapper. Keep its body calling `g.fieldToResourceAttribute` (the wrapper) — do NOT rename those call sites.

Add the wrappers and marking helpers:

```go
// fieldToResourceAttribute converts a FieldInfo to a resource.Attribute,
// applying the sensitive flag to whatever attribute type was built.
func (g *SpecificationGenerator) fieldToResourceAttribute(r *ResourceInfo, field *FieldInfo) *resource.Attribute {
	attr := g.buildResourceAttribute(r, field)
	if attr != nil && field.Sensitive {
		markResourceAttributeSensitive(attr)
	}
	return attr
}

// fieldToDataSourceAttribute converts a FieldInfo to a datasource.Attribute,
// applying the sensitive flag to whatever attribute type was built.
func (g *SpecificationGenerator) fieldToDataSourceAttribute(r *ResourceInfo, field *FieldInfo) *datasource.Attribute {
	attr := g.buildDataSourceAttribute(r, field)
	if attr != nil && field.Sensitive {
		markDataSourceAttributeSensitive(attr)
	}
	return attr
}

func markResourceAttributeSensitive(attr *resource.Attribute) {
	switch {
	case attr.Bool != nil:
		attr.Bool.Sensitive = ptr(true)
	case attr.Float64 != nil:
		attr.Float64.Sensitive = ptr(true)
	case attr.Int64 != nil:
		attr.Int64.Sensitive = ptr(true)
	case attr.List != nil:
		attr.List.Sensitive = ptr(true)
	case attr.ListNested != nil:
		attr.ListNested.Sensitive = ptr(true)
	case attr.Map != nil:
		attr.Map.Sensitive = ptr(true)
	case attr.MapNested != nil:
		attr.MapNested.Sensitive = ptr(true)
	case attr.Number != nil:
		attr.Number.Sensitive = ptr(true)
	case attr.Object != nil:
		attr.Object.Sensitive = ptr(true)
	case attr.Set != nil:
		attr.Set.Sensitive = ptr(true)
	case attr.SetNested != nil:
		attr.SetNested.Sensitive = ptr(true)
	case attr.SingleNested != nil:
		attr.SingleNested.Sensitive = ptr(true)
	case attr.String != nil:
		attr.String.Sensitive = ptr(true)
	}
}

func markDataSourceAttributeSensitive(attr *datasource.Attribute) {
	switch {
	case attr.Bool != nil:
		attr.Bool.Sensitive = ptr(true)
	case attr.Float64 != nil:
		attr.Float64.Sensitive = ptr(true)
	case attr.Int64 != nil:
		attr.Int64.Sensitive = ptr(true)
	case attr.List != nil:
		attr.List.Sensitive = ptr(true)
	case attr.ListNested != nil:
		attr.ListNested.Sensitive = ptr(true)
	case attr.Map != nil:
		attr.Map.Sensitive = ptr(true)
	case attr.MapNested != nil:
		attr.MapNested.Sensitive = ptr(true)
	case attr.Number != nil:
		attr.Number.Sensitive = ptr(true)
	case attr.Object != nil:
		attr.Object.Sensitive = ptr(true)
	case attr.Set != nil:
		attr.Set.Sensitive = ptr(true)
	case attr.SetNested != nil:
		attr.SetNested.Sensitive = ptr(true)
	case attr.SingleNested != nil:
		attr.SingleNested.Sensitive = ptr(true)
	case attr.String != nil:
		attr.String.Sensitive = ptr(true)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/fields/ && go build ./...`
Expected: all PASS (existing schema tests unaffected)

- [ ] **Step 5: Commit**

```bash
git add cmd/fields/schema.go cmd/fields/schema_test.go
git commit -m "fields: emit sensitive attributes in the tf provider spec"
```

---

### Task 9: Real regeneration at Network 10.4.57

**Files:**
- Modify: `unifi/unifi.go` (go:generate directive)
- Regenerate: `unifi/*.generated.go`, `unifi/version.generated.go`, `specification.json`
- Possibly modify: `cmd/fields/main.go` FieldProcessors if 10.4.57 defs break codegen

This is the integration task. Expect fallout; fix it properly, don't paper over it.

- [ ] **Step 1: Update the go:generate directive**

In `unifi/unifi.go`, change:

```go
//go:generate go run ../cmd/fields/ -output-dir=../unifi/ -latest
```

to:

```go
//go:generate go run ../cmd/fields/ -output-dir=../unifi/ -latest -generate-spec -spec-output ../specification.json
```

- [ ] **Step 2: Extract from the real installer (local file, no 880MB download)**

Run:

```bash
go run ./cmd/fields -installer ~/Downloads/f5e2-linux-x64-5.1.21-a400c9c6-8328-4634-b223-ebfcf742720a.21-x64 -download-only
ls cmd/fields/v10.4.57/ | head -40
```

Expected: `NetworkConf.json`, `WlanConf.json`, …, `metadata/`, `source.json`; "extracted N definition files (network 10.4.57)" in output.

- [ ] **Step 3: Verify extraction against the PoC oracle (byte-for-byte)**

Run:

```bash
POC=/Users/jamesb/emdash/worktrees/terraform-provider-unifi/emdash/extract-find-api-json-definitions-gyr8e/schemas/unifi-network/10.4.57
for f in "$POC"/api/fields/*.json; do
  diff -q "$f" "cmd/fields/v10.4.57/$(basename "$f")" || echo "MISSING/DIFF: $(basename "$f")"
done
for f in "$POC"/*.json; do
  diff -q "$f" "cmd/fields/v10.4.57/metadata/$(basename "$f")" || echo "MISSING/DIFF: $(basename "$f")"
done
```

Expected: no output (all identical). If a diff appears, find out why before continuing — the PoC is the validated reference.

- [ ] **Step 4: Full regeneration**

Run: `go generate ./...`
Expected: completes; `unifi/*.generated.go`, `unifi/version.generated.go`, `specification.json` rewritten for 10.4.57. (`-latest` hits the API for the version lookup but reuses the cached fields dir — no re-download.)

- [ ] **Step 5: Build + test, fix fallout until green**

Run: `go build ./... && go test ./...`

If the build breaks: 10.4.57 defs added/removed/changed fields. Fixes belong in `FieldProcessor` hooks or `NewResource` special cases in `cmd/fields/main.go` — follow the existing documented examples there (FirewallZone, FirewallPolicy, Network). Never edit generated files by hand. Re-run `go generate ./...` after each fix. Iterate until green.

- [ ] **Step 6: Verify sensitive flags landed in the spec**

Run:

```bash
grep -c '"sensitive": true' specification.json
python3 - << 'EOF'
import json
d = json.load(open('specification.json'))
for r in d['resources']:
    if r['name'] in ('wlan', 'network', 'radius_profile'):
        sens = [a['name'] for a in r['schema']['attributes']
                if a.get('string', {}).get('sensitive')
                or a.get('bool', {}).get('sensitive')]
        print(r['name'], sens)
EOF
```

Expected: count well above the previous 2; `wlan` includes `x_passphrase` and `x_wep` but NOT `name`; `network` includes `x_wireguard_private_key`, `x_ipsec_pre_shared_key`, etc. but NOT `name`/`domain_name`.

- [ ] **Step 7: Commit the tool changes and the regeneration separately**

```bash
git add unifi/unifi.go
git commit -m "unifi: generate the tf spec with go generate"

git add unifi/ specification.json
git commit -m "unifi: regenerate at unifi network 10.4.57"
```

(If FieldProcessor fixes were needed: `git add cmd/fields/main.go` into a commit like `fields: handle 10.4.57 schema drift`.)

---

### Task 10: GitHub Actions automation

**Files:**
- Modify: `.github/workflows/generate.yaml` (full rewrite)
- Create: `.github/workflows/release-on-merge.yaml`
- Modify: `.github/workflows/ci.yaml`

Prerequisites the user must do once (call these out in the final report; workflows fail without them): create a GitHub App with `contents: write` + `pull_requests: write` on this repo, install it, add `APP_ID` + `APP_PRIVATE_KEY` secrets, enable "Allow auto-merge" in repo settings.

- [ ] **Step 1: Rewrite `.github/workflows/generate.yaml`**

```yaml
---
name: Schema Generation

on:
  schedule:
    - cron: 0 0 * * *
  workflow_dispatch: {}

permissions:
  contents: read

concurrency:
  group: schema-generation
  cancel-in-progress: false

jobs:
  fields:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0

      - name: Check latest UniFi OS Server release
        id: check
        run: |
          set -euo pipefail
          latest="$(curl -sf 'https://fw-update.ubnt.com/api/firmware-latest?filter=eq~~channel~~release&filter=eq~~product~~unifi-os-server' | jq -r '[._embedded.firmware[] | select(.platform == "linux-x64")] | .[0].version | ltrimstr("v")')"
          current="$(grep -oE 'UnifiOsServerVersion = "[^"]*"' unifi/version.generated.go | cut -d'"' -f2 || true)"
          echo "latest=${latest}" >> "$GITHUB_OUTPUT"
          if [ "${latest}" = "${current}" ]; then
            echo "uptodate=true" >> "$GITHUB_OUTPUT"
          fi
          echo "latest UniFi OS Server: ${latest}; generated from: ${current:-none}"

      - name: Setup Go
        if: steps.check.outputs.uptodate != 'true'
        uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0
        with:
          go-version-file: 'go.mod'

      - name: Generate
        if: steps.check.outputs.uptodate != 'true'
        run: go generate ./...

      - name: Create GitHub App token
        if: steps.check.outputs.uptodate != 'true'
        id: app-token
        uses: actions/create-github-app-token@fee1f7d63c2ff003460e3d139729b119787bc349 # v2.2.2
        with:
          app-id: ${{ secrets.APP_ID }}
          private-key: ${{ secrets.APP_PRIVATE_KEY }}

      - name: Create pull request
        if: steps.check.outputs.uptodate != 'true'
        id: cpr
        uses: peter-evans/create-pull-request@5f6978faf089d4d20b00c7766989d076bb2fc7f1 # v8.1.1
        with:
          token: ${{ steps.app-token.outputs.token }}
          delete-branch: true
          branch: auto/unifi-${{ steps.check.outputs.latest }}
          title: "Update to UniFi OS Server ${{ steps.check.outputs.latest }}"
          body: |
            Automated regeneration from UniFi OS Server ${{ steps.check.outputs.latest }}.

            Produced by the schema-generation workflow (`go generate ./...`).
          labels: automation

      - name: Enable auto-merge
        if: steps.cpr.outputs.pull-request-number != ''
        run: gh pr merge --auto --squash "${{ steps.cpr.outputs.pull-request-number }}"
        env:
          GH_TOKEN: ${{ steps.app-token.outputs.token }}
```

- [ ] **Step 2: Create `.github/workflows/release-on-merge.yaml`**

```yaml
---
name: Release On Schema Update

on:
  push:
    branches:
      - 'main'

permissions:
  contents: read

concurrency:
  group: schema-release
  cancel-in-progress: false

jobs:
  tag:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
        with:
          fetch-depth: 0

      - name: Check for generated schema changes
        id: changed
        run: |
          set -euo pipefail
          if git diff --name-only HEAD~1 HEAD | grep -qE '^unifi/.*\.generated\.go$|^specification\.json$'; then
            echo "changed=true" >> "$GITHUB_OUTPUT"
          else
            echo "no generated schema changes; skipping"
          fi

      - name: Create GitHub App token
        if: steps.changed.outputs.changed == 'true'
        id: app-token
        uses: actions/create-github-app-token@fee1f7d63c2ff003460e3d139729b119787bc349 # v2.2.2
        with:
          app-id: ${{ secrets.APP_ID }}
          private-key: ${{ secrets.APP_PRIVATE_KEY }}

      - name: Tag next minor release
        if: steps.changed.outputs.changed == 'true'
        env:
          GH_TOKEN: ${{ steps.app-token.outputs.token }}
        run: |
          set -euo pipefail
          latest="$(git tag -l 'v*' --sort=-v:refname | head -1)"
          latest="${latest:-v0.0.0}"
          next="$(echo "${latest}" | awk -F. '{printf "v%d.%d.0", substr($1, 2), $2 + 1}')"
          git tag "${next}"
          git push "https://x-access-token:${GH_TOKEN}@github.com/${{ github.repository }}.git" "${next}"
          echo "tagged ${next} (was ${latest})"
```

The tag push triggers the existing `release.yaml` (goreleaser) — unchanged.

- [ ] **Step 3: Harden `.github/workflows/ci.yaml`**

In the `generate` job, add a job-level `if` so automated PRs skip the redundant 880MB regeneration. Change:

```yaml
jobs:
  generate:
    runs-on: ubuntu-latest
    steps:
```

to:

```yaml
jobs:
  generate:
    runs-on: ubuntu-latest
    if: github.event_name != 'pull_request' || !startsWith(github.head_ref, 'auto/')
    steps:
```

In the `test` job, delete the `continue-on-error: true` line under the "Run tests" step so tests are a real merge gate:

```yaml
      - name: Run tests
        run: go test ./...
```

- [ ] **Step 4: Validate workflow syntax**

Run: `python3 -c "import yaml,glob; [yaml.safe_load(open(f)) for f in glob.glob('.github/workflows/*.yaml')]; print('ok')"`
Expected: `ok` (and no stray tab characters — YAML forbids them)

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/generate.yaml .github/workflows/release-on-merge.yaml .github/workflows/ci.yaml
git commit -m "ci: automate schema regeneration and releases"
```

---

### Task 11: Docs + final validation

**Files:**
- Modify: `README.md`
- Modify: `go.mod` (tidy)

- [ ] **Step 1: Update the README codegen note**

Replace the "Note on Code Generation" section with:

```markdown
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
```

- [ ] **Step 2: Tidy and format**

Run: `go mod tidy && gofmt -l cmd/ unifi/ && go vet ./...`
Expected: no output from gofmt; vet clean

- [ ] **Step 3: Full validation**

Run: `go build ./... && go test ./...`
Expected: everything PASS

- [ ] **Step 4: Commit**

```bash
git add README.md go.mod go.sum
git commit -m "fields: document the os-server schema fetcher"
```

- [ ] **Step 5: Final report**

Report: commit list (`git log --oneline main..HEAD`), test results, the regenerated version (10.4.57 / OS Server 5.1.21), and the manual prerequisites for the automation (GitHub App + secrets + allow auto-merge). Do NOT push.
