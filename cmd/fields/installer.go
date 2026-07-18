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
