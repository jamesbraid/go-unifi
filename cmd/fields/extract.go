package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/go-version"
	"github.com/iancoleman/strcase"
	"pault.ag/go/debian/deb"
)

const (
	aceJarPath      = "usr/lib/unifi/lib/ace.jar"
	internalJarPath = "usr/lib/unifi/lib/internal/internal-dependencies.jar"
	bootInfJarPath  = "BOOT-INF/lib/internal-dependencies.jar"
)

// metadataFiles are the definitions shipped at the root of
// internal-dependencies.jar that we extract alongside the api/fields
// schemas, into the gitignored local cache. The set is strictly limited to
// files the generator consumes: the jar's other data files (event message
// templates, country and timezone lists, radio specifications, ...) serve
// no client function here and are not extracted. Nothing extracted is
// committed or redistributed (see schemas/README.md).
var metadataFiles = []string{
	// Feeds the sensitive flags in the generated Terraform specification.
	"sensitive_metadata.json",
}

// minLayerSize skips OCI blobs that are too small to contain the controller
// rootfs. Variable so tests can use small fixtures.
var minLayerSize = int64(10_000_000)

// minFieldFiles is the smallest plausible number of api/fields definitions in
// a real controller build. Variable so tests can use small fixtures.
var minFieldFiles = 15

// artifacts are the files of interest pulled out of a controller
// distribution, whatever its packaging:
//
//   - deb <= 9.x: ace.jar carries api/fields/*.json at its root
//   - deb 10.x: ace.jar is a thin launcher; a standalone
//     internal-dependencies.jar carries the definitions
//   - UniFi OS Server installer: ELF stub + appended zip -> image.tar (OCI)
//     -> rootfs layer -> Spring Boot fat ace.jar with the definitions in
//     BOOT-INF/lib/internal-dependencies.jar
type artifacts struct {
	aceJar      string
	internalJar string
}

// httpClient retries transient failures; controller artifacts are large and
// the nightly workflow should not fail on a single connection reset. The
// overall timeout bounds a stalled connection (retryablehttp defaults to
// none) while leaving ample room for the ~880MB installer on slow links.
func httpClient() *http.Client {
	rc := retryablehttp.NewClient()
	rc.Logger = nil
	rc.HTTPClient.Timeout = 15 * time.Minute
	return rc.StandardClient()
}

func downloadArtifact(url *url.URL, outputDir string) (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url.String(), nil)
	if err != nil {
		return "", fmt.Errorf("unable to build download request: %w", err)
	}

	resp, err := httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unable to download %s: HTTP %s", url, resp.Status)
	}

	name := path.Base(url.Path)
	if name == "" || name == "/" || name == "." || name == ".." {
		name = "unifi-artifact"
	}

	dest := filepath.Join(outputDir, name)
	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("unable to create download file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return "", fmt.Errorf("unable to write download file: %w", err)
	}
	// A failed flush on close would silently truncate the artifact.
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("unable to write download file: %w", err)
	}

	return dest, nil
}

// extractArtifacts sniffs the packaging format of a controller distribution
// and pulls ace.jar (and a standalone internal-dependencies.jar, when
// present) into workDir.
func extractArtifacts(artifactPath, workDir string) (*artifacts, error) {
	f, err := os.Open(artifactPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	magic := make([]byte, 8)
	if _, err := io.ReadFull(f, magic); err != nil {
		return nil, fmt.Errorf("unable to read artifact magic: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	if bytes.HasPrefix(magic, []byte("!<arch>\n")) {
		return extractFromDeb(f, workDir)
	}

	return extractFromInstaller(f, workDir)
}

// extractFromDeb walks a debian package's data archive and captures the
// controller jars.
//
// It deliberately uses deb.LoadAr rather than deb.Load: only the data
// archive matters here (the application version comes from ace.jar's
// product.properties, not the package control paragraph), and Load would
// additionally require and validate a control member we never look at.
// ArEntry.Tarfile applies the library's decompressor table, so gz, bz2,
// xz, lzma, zstd, and uncompressed data archives are all handled.
func extractFromDeb(f *os.File, workDir string) (*artifacts, error) {
	arReader, err := deb.LoadAr(f)
	if err != nil {
		return nil, fmt.Errorf("unable to open deb: %w", err)
	}

	var dataTar *tar.Reader
	for {
		member, err := arReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("in ar next: %w", err)
		}
		if !strings.HasPrefix(member.Name, "data.tar") || !member.IsTarfile() {
			continue
		}

		var closer io.Closer
		dataTar, closer, err = member.Tarfile()
		if err != nil {
			return nil, fmt.Errorf("unable to open %s: %w", member.Name, err)
		}
		defer closer.Close()
		break
	}
	if dataTar == nil {
		return nil, errors.New("unable to find .deb data file")
	}

	found := &artifacts{}
	if err := captureTarMembers(dataTar, workDir, found); err != nil {
		return nil, err
	}
	if found.aceJar == "" {
		return nil, errors.New("unable to find ace.jar in deb")
	}

	return found, nil
}

// extractFromInstaller walks a UniFi OS Server self-extracting installer: an
// ELF stub with an appended zip containing an OCI image tar whose rootfs
// layer carries the controller jars.
func extractFromInstaller(f *os.File, workDir string) (*artifacts, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// archive/zip natively supports zip payloads with prepended data (the
	// ELF stub) since Go 1.20: it locates the end-of-central-directory
	// record from the tail and applies the stub length as a base offset.
	// Verified against the real UniFi OS Server installer.
	zr, err := zip.NewReader(f, stat.Size())
	if err != nil {
		return nil, fmt.Errorf("unable to open installer payload zip: %w", err)
	}

	var imageTar *zip.File
	for _, zf := range zr.File {
		if path.Base(zf.Name) == "image.tar" {
			imageTar = zf
			break
		}
	}
	if imageTar == nil {
		return nil, errors.New("image.tar not found in installer payload")
	}

	src, err := imageTar.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// OCI images store layers as tar blobs under blobs/sha256/<hash>. Scan
	// every plausibly-sized layer for the controller files rather than
	// parsing the manifest, so we don't have to know which layer they are in.
	// Finds accumulate across layers in case the jars are split between them.
	found := &artifacts{}
	imageReader := tar.NewReader(src)
	for !found.complete() {
		header, err := imageReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("in image tar next: %w", err)
		}
		if header.Typeflag != tar.TypeReg || header.Size < minLayerSize {
			continue
		}

		layer := bufio.NewReaderSize(imageReader, 1<<20)
		head, err := layer.Peek(2)
		if err != nil {
			continue
		}

		var layerReader io.Reader = layer
		if bytes.Equal(head, []byte{0x1f, 0x8b}) {
			gz, err := gzip.NewReader(layer)
			if err != nil {
				continue
			}
			layerReader = gz
		}

		// Errors mean the blob is not a (complete) tar layer; keep whatever
		// was captured before the error and move on to the next blob.
		_ = captureTarMembers(tar.NewReader(layerReader), workDir, found)
	}

	if found.aceJar == "" && found.internalJar == "" {
		return nil, errors.New("ace.jar not found in any installer image layer")
	}

	return found, nil
}

func (a *artifacts) complete() bool {
	return a.aceJar != "" && a.internalJar != ""
}

// captureTarMembers streams a tar archive, writing any controller jars it
// contains into workDir and recording them in found. It stops early once
// every jar of interest has been captured, and otherwise reads to the end of
// the archive so member order does not matter.
func captureTarMembers(tr *tar.Reader, workDir string, found *artifacts) error {
	for !found.complete() {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("in tar next: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		var dest *string
		switch {
		case strings.HasSuffix(header.Name, aceJarPath):
			dest = &found.aceJar
		case strings.HasSuffix(header.Name, internalJarPath):
			dest = &found.internalJar
		default:
			continue
		}

		out, err := os.Create(filepath.Join(workDir, path.Base(header.Name)))
		if err != nil {
			return fmt.Errorf("unable to create %s: %w", path.Base(header.Name), err)
		}
		_, err = io.Copy(out, tr)
		if closeErr := out.Close(); err == nil {
			// A failed flush on close would silently truncate the jar.
			err = closeErr
		}
		if err != nil {
			return fmt.Errorf("unable to write %s: %w", path.Base(header.Name), err)
		}
		*dest = out.Name()
	}

	return nil
}

// resolveDefsJar returns the path of the jar holding the api/fields
// definitions, extracting it from a fat ace.jar when necessary.
func resolveDefsJar(a *artifacts, workDir string) (string, error) {
	if a.internalJar != "" {
		return a.internalJar, nil
	}
	if a.aceJar == "" {
		return "", errors.New("no ace.jar available")
	}

	zr, err := zip.OpenReader(a.aceJar)
	if err != nil {
		return "", fmt.Errorf("unable to open ace.jar: %w", err)
	}
	defer zr.Close()

	for _, zf := range zr.File {
		if strings.HasPrefix(zf.Name, "api/fields/") {
			// Pre-10.x layout: the definitions live in ace.jar itself.
			return a.aceJar, nil
		}
	}

	for _, zf := range zr.File {
		if zf.Name != bootInfJarPath {
			continue
		}

		dest := filepath.Join(workDir, "internal-dependencies.jar")
		if err := writeZipEntry(zf, dest); err != nil {
			return "", fmt.Errorf("unable to extract %s: %w", bootInfJarPath, err)
		}
		return dest, nil
	}

	return "", errors.New("unable to locate api field definitions in ace.jar")
}

var productVersionRe = regexp.MustCompile(`(?m)^version=(.+)$`)

// readNetworkVersion reads the UniFi Network application version from
// product.properties inside ace.jar (at the root of the thin launcher jar,
// under BOOT-INF/classes in the Spring Boot fat jar).
func readNetworkVersion(aceJar string) (*version.Version, error) {
	if aceJar == "" {
		return nil, errors.New("no ace.jar available")
	}

	zr, err := zip.OpenReader(aceJar)
	if err != nil {
		return nil, fmt.Errorf("unable to open ace.jar: %w", err)
	}
	defer zr.Close()

	for _, name := range []string{"product.properties", "BOOT-INF/classes/product.properties"} {
		props, err := fs.ReadFile(&zr.Reader, name)
		if err != nil {
			continue
		}

		m := productVersionRe.FindSubmatch(props)
		if m == nil {
			return nil, fmt.Errorf("no version property in %s", name)
		}
		return version.NewVersion(strings.TrimSpace(string(m[1])))
	}

	return nil, errors.New("product.properties not found in ace.jar")
}

// extractSchemas writes the api/fields definitions into fieldsDir and the
// informational metadata files into metadataDir, then splits the composite
// Setting.json and overlays the custom definitions from customDir.
func extractSchemas(defsJar, fieldsDir, metadataDir, customDir string) error {
	zr, err := zip.OpenReader(defsJar)
	if err != nil {
		return fmt.Errorf("unable to open definitions jar: %w", err)
	}
	defer zr.Close()

	for _, dir := range []string{fieldsDir, metadataDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	fieldCount := 0
	for _, zf := range zr.File {
		var dest string
		switch {
		case strings.HasPrefix(zf.Name, "api/fields/") && path.Ext(zf.Name) == ".json":
			dest = filepath.Join(fieldsDir, path.Base(zf.Name))
			fieldCount++
		case slices.Contains(metadataFiles, zf.Name):
			dest = filepath.Join(metadataDir, zf.Name)
		default:
			continue
		}

		if err := writeZipEntry(zf, dest); err != nil {
			return err
		}
	}

	// Refuse to bless a near-empty snapshot: a wrong or truncated jar must
	// fail loudly, not produce a "current" snapshot that generates nothing.
	// Real controllers ship ~28 field files; the test fixtures ship 2.
	if fieldCount < minFieldFiles {
		return fmt.Errorf("only %d api/fields definitions found in %s; wrong or corrupt definitions jar", fieldCount, defsJar)
	}
	for _, name := range metadataFiles {
		if _, err := os.Stat(filepath.Join(metadataDir, name)); err != nil {
			fmt.Printf("warning: %s not found in definitions jar\n", name)
		}
	}

	if err := splitSettings(fieldsDir); err != nil {
		return err
	}

	if err := writeExtractedManifest(fieldsDir); err != nil {
		return err
	}

	return syncCustom(customDir, fieldsDir)
}

// extractedManifest records which files in the fields cache came from the
// controller artifact (including the Setting splits), so syncCustom can tell
// overlay leftovers apart from extracted definitions.
const extractedManifest = ".extracted"

func writeExtractedManifest(fieldsDir string) error {
	entries, err := os.ReadDir(fieldsDir)
	if err != nil {
		return err
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && path.Ext(entry.Name()) == ".json" {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)

	return os.WriteFile(filepath.Join(fieldsDir, extractedManifest), []byte(strings.Join(names, "\n")+"\n"), 0o644)
}

func writeZipEntry(zf *zip.File, dest string) error {
	src, err := zf.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return fmt.Errorf("unable to write %s: %w", dest, err)
	}
	// A failed flush on close would silently truncate the file.
	if err := out.Close(); err != nil {
		return fmt.Errorf("unable to write %s: %w", dest, err)
	}

	return nil
}

// splitSettings explodes the composite Setting.json into one
// Setting<Key>.json file per settings section.
func splitSettings(fieldsDir string) error {
	settingsData, err := os.ReadFile(filepath.Join(fieldsDir, "Setting.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to open settings file: %w", err)
	}

	var settings map[string]any
	err = json.Unmarshal(settingsData, &settings)
	if err != nil {
		return fmt.Errorf("unable to unmarshal settings: %w", err)
	}

	for k, v := range settings {
		fileName := fmt.Sprintf("Setting%s.json", strcase.ToCamel(k))

		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Errorf("unable to marshal setting %q: %w", k, err)
		}

		err = os.WriteFile(filepath.Join(fieldsDir, fileName), data, 0o644)
		if err != nil {
			return fmt.Errorf("unable to write new settings file: %w", err)
		}
	}

	return nil
}

// syncCustom overlays the manual schema definitions from customDir onto the
// extracted fields, and removes overlay leftovers: cached files that are
// neither in the extraction manifest nor in customDir any more (i.e. a
// deleted override). Runs on both the fresh-extraction and cache-hit paths.
func syncCustom(customDir, fieldsDir string) error {
	files, err := os.ReadDir(customDir)
	if err != nil {
		return fmt.Errorf("unable to read custom directory: %w", err)
	}

	keep := map[string]bool{}
	manifest, err := os.ReadFile(filepath.Join(fieldsDir, extractedManifest))
	hasManifest := err == nil
	if hasManifest {
		for name := range strings.SplitSeq(strings.TrimSpace(string(manifest)), "\n") {
			keep[name] = true
		}
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		keep[file.Name()] = true

		data, err := os.ReadFile(filepath.Join(customDir, file.Name()))
		if err != nil {
			return fmt.Errorf("unable to read custom file: %w", err)
		}
		if err := os.WriteFile(filepath.Join(fieldsDir, file.Name()), data, 0o644); err != nil {
			return fmt.Errorf("unable to write custom file: %w", err)
		}
	}

	// Without a manifest (pre-manifest cache) there is no way to tell
	// extracted files from stale overlays, so only clean when one exists.
	if hasManifest {
		entries, err := os.ReadDir(fieldsDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || path.Ext(name) != ".json" || keep[name] {
				continue
			}
			fmt.Printf("removing stale overlay %s\n", name)
			if err := os.Remove(filepath.Join(fieldsDir, name)); err != nil {
				return err
			}
		}
	}

	return nil
}

func findModuleRoot(dir string) string {
	if dir == "" {
		panic("dir not set")
	}
	dir = filepath.Clean(dir)
	// Look for enclosing go.mod.
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
		d := filepath.Dir(dir)
		if d == dir {
			break
		}
		dir = d
	}
	return ""
}
