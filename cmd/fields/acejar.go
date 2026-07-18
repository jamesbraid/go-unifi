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

// readZipFile returns the full contents of a zip entry.
func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// findInternalJar reads internal-dependencies.jar out of ace.jar (a Spring
// Boot fat jar). Pass 1 matches the exact BOOT-INF path; pass 2 falls back
// to a name suffix in case the packaging moves the jar.
func findInternalJar(aceJarPath string) ([]byte, error) {
	zr, err := zip.OpenReader(aceJarPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open ace.jar: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name == "BOOT-INF/lib/internal-dependencies.jar" {
			return readZipFile(f)
		}
	}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "internal-dependencies.jar") {
			return readZipFile(f)
		}
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
	fieldDefs := 0
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
			return n, fmt.Errorf("unable to extract %s: %w", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return n, fmt.Errorf("unable to extract %s: %w", f.Name, err)
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return n, fmt.Errorf("unable to extract %s: %w", f.Name, err)
		}
		if err := os.WriteFile(dest, b, 0o644); err != nil {
			return n, fmt.Errorf("unable to extract %s: %w", f.Name, err)
		}
		n++
		if strings.HasPrefix(f.Name, "api/fields/") {
			fieldDefs++
		}
	}

	// A valid internal-dependencies.jar always carries field definitions; zero
	// means the packaging changed and we must not silently extract nothing.
	if fieldDefs == 0 {
		return 0, errors.New("no api/fields definitions found in internal-dependencies.jar")
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
		b, err := readZipFile(f)
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
