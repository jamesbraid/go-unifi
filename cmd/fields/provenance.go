package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const treeDigestVersion = "go-unifi-tree-digest-v1\x00"

type LocalManifest struct {
	OSVersion         string            `json:"os_version"`
	NetworkVersion    string            `json:"network_version"`
	FirmwareID        string            `json:"firmware_id"`
	InstallerURL      string            `json:"installer_url"`
	InstallerSHA256   string            `json:"installer_sha256"`
	InstallerMD5      string            `json:"installer_md5"`
	Product           string            `json:"product"`
	Platform          string            `json:"platform"`
	Channel           string            `json:"channel"`
	SchemaDigest      string            `json:"schema_digest"`
	SensitivityDigest string            `json:"sensitivity_digest"`
	NoticeDigest      string            `json:"notice_digest"`
	PolicyVersion     string            `json:"policy_version"`
	InstallerSize     int64             `json:"installer_size"`
	Created           time.Time         `json:"created"`
	Updated           time.Time         `json:"updated"`
	Artifacts         map[string]string `json:"artifacts"`
	MissingOptional   []string          `json:"missing_optional"`
}

type SchemaSource struct {
	OSVersion           string            `json:"os_version"`
	NetworkVersion      string            `json:"network_version"`
	FirmwareID          string            `json:"firmware_id"`
	InstallerURL        string            `json:"installer_url"`
	InstallerSHA256     string            `json:"installer_sha256"`
	SchemaDigest        string            `json:"schema_digest"`
	SensitivityDigest   string            `json:"sensitivity_digest"`
	NoticeDigest        string            `json:"notice_digest"`
	GeneratedTreeDigest string            `json:"generated_tree_digest"`
	PolicyVersion       string            `json:"policy_version"`
	InstallerSize       int64             `json:"installer_size"`
	Created             time.Time         `json:"created"`
	Updated             time.Time         `json:"updated"`
	GeneratedFiles      map[string]string `json:"generated_files"`
}

func (m LocalManifest) MarshalJSON() ([]byte, error) {
	type fields struct {
		OSVersion         string            `json:"os_version"`
		NetworkVersion    string            `json:"network_version"`
		FirmwareID        string            `json:"firmware_id"`
		InstallerURL      string            `json:"installer_url"`
		InstallerSHA256   string            `json:"installer_sha256"`
		InstallerMD5      string            `json:"installer_md5"`
		Product           string            `json:"product"`
		Platform          string            `json:"platform"`
		Channel           string            `json:"channel"`
		SchemaDigest      string            `json:"schema_digest"`
		SensitivityDigest string            `json:"sensitivity_digest"`
		NoticeDigest      string            `json:"notice_digest"`
		PolicyVersion     string            `json:"policy_version"`
		InstallerSize     int64             `json:"installer_size"`
		Created           *time.Time        `json:"created"`
		Updated           *time.Time        `json:"updated"`
		Artifacts         map[string]string `json:"artifacts"`
		MissingOptional   []string          `json:"missing_optional"`
	}
	return json.Marshal(fields{
		OSVersion: m.OSVersion, NetworkVersion: m.NetworkVersion, FirmwareID: m.FirmwareID,
		InstallerURL: m.InstallerURL, InstallerSHA256: m.InstallerSHA256, InstallerMD5: m.InstallerMD5,
		Product: m.Product, Platform: m.Platform, Channel: m.Channel, SchemaDigest: m.SchemaDigest,
		SensitivityDigest: m.SensitivityDigest, NoticeDigest: m.NoticeDigest, PolicyVersion: m.PolicyVersion,
		InstallerSize: m.InstallerSize, Created: normalizedTime(m.Created), Updated: normalizedTime(m.Updated),
		Artifacts: nonNilStringMap(m.Artifacts), MissingOptional: nonNilStrings(m.MissingOptional),
	})
}

func (s SchemaSource) MarshalJSON() ([]byte, error) {
	type fields struct {
		OSVersion           string            `json:"os_version"`
		NetworkVersion      string            `json:"network_version"`
		FirmwareID          string            `json:"firmware_id"`
		InstallerURL        string            `json:"installer_url"`
		InstallerSHA256     string            `json:"installer_sha256"`
		SchemaDigest        string            `json:"schema_digest"`
		SensitivityDigest   string            `json:"sensitivity_digest"`
		NoticeDigest        string            `json:"notice_digest"`
		GeneratedTreeDigest string            `json:"generated_tree_digest"`
		PolicyVersion       string            `json:"policy_version"`
		InstallerSize       int64             `json:"installer_size"`
		Created             *time.Time        `json:"created"`
		Updated             *time.Time        `json:"updated"`
		GeneratedFiles      map[string]string `json:"generated_files"`
	}
	return json.Marshal(fields{
		OSVersion: s.OSVersion, NetworkVersion: s.NetworkVersion, FirmwareID: s.FirmwareID,
		InstallerURL: s.InstallerURL, InstallerSHA256: s.InstallerSHA256, SchemaDigest: s.SchemaDigest,
		SensitivityDigest: s.SensitivityDigest, NoticeDigest: s.NoticeDigest,
		GeneratedTreeDigest: s.GeneratedTreeDigest, PolicyVersion: s.PolicyVersion,
		InstallerSize: s.InstallerSize, Created: normalizedTime(s.Created), Updated: normalizedTime(s.Updated),
		GeneratedFiles: nonNilStringMap(s.GeneratedFiles),
	})
}

func normalizedTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	normalized := value.UTC().Truncate(time.Second)
	return &normalized
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilStringMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	return values
}

// CanonicalTreeDigest computes a versioned, length-framed SHA-256 tree digest.
// Paths are sorted slash paths. JSON values are canonicalized; other files have
// CRLF and CR line endings normalized to LF.
func CanonicalTreeDigest(files map[string][]byte) (string, error) {
	paths := make([]string, 0, len(files))
	for name := range files {
		if err := validateDigestPath(name); err != nil {
			return "", err
		}
		paths = append(paths, name)
	}
	sort.Strings(paths)

	hasher := sha256.New()
	_, _ = io.WriteString(hasher, treeDigestVersion)
	writeFrameLength(hasher, uint64(len(paths)))
	for _, name := range paths {
		body := files[name]
		var canonical []byte
		var err error
		if strings.EqualFold(filepath.Ext(name), ".json") {
			canonical, err = canonicalJSON(body)
		} else {
			canonical = normalizeLineEndings(body)
		}
		if err != nil {
			return "", fmt.Errorf("canonicalize %s: %w", name, err)
		}
		writeFrameLength(hasher, uint64(len(name)))
		_, _ = io.WriteString(hasher, name)
		writeFrameLength(hasher, uint64(len(canonical)))
		_, _ = hasher.Write(canonical)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func CanonicalJSONDigest(body []byte) (string, error) {
	canonical, err := canonicalJSON(body)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalJSON(body []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("multiple JSON values")
		}
		return nil, fmt.Errorf("trailing JSON data: %w", err)
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}

func normalizeLineEndings(body []byte) []byte {
	body = bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(body, []byte("\r"), []byte("\n"))
}

func validateDigestPath(name string) error {
	if name == "" || strings.Contains(name, "\\") || filepath.IsAbs(name) || filepath.ToSlash(filepath.Clean(name)) != name || name == "." || strings.HasPrefix(name, "../") {
		return fmt.Errorf("invalid digest path %q", name)
	}
	return nil
}

func writeFrameLength(writer io.Writer, value uint64) {
	var frame [8]byte
	binary.BigEndian.PutUint64(frame[:], value)
	_, _ = writer.Write(frame[:])
}

func GeneratedTreeDigest(root string) (string, error) {
	files := make(map[string][]byte)
	unifiRoot := filepath.Join(root, "unifi")
	if err := filepath.WalkDir(unifiRoot, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("generated tree contains symlink %s", name)
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".generated.go") {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("generated output is not regular: %s", name)
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = body
		return nil
	}); err != nil {
		return "", fmt.Errorf("walk generated Go tree: %w", err)
	}

	specification := filepath.Join(root, "specification.json")
	info, err := os.Lstat(specification)
	if err != nil {
		return "", fmt.Errorf("stat specification.json: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("specification.json is not a regular file")
	}
	body, err := os.ReadFile(specification)
	if err != nil {
		return "", fmt.Errorf("read specification.json: %w", err)
	}
	files["specification.json"] = body
	return CanonicalTreeDigest(files)
}

func WriteSchemaSource(name string, source SchemaSource) error {
	return writeSchemaSource(name, source, defaultSchemaSourceFileOps())
}

type schemaSourceFileOps struct {
	createTemp func(string, string) (*os.File, error)
	rename     func(string, string) error
	remove     func(string) error
	syncDir    func(string) error
}

func defaultSchemaSourceFileOps() schemaSourceFileOps {
	return schemaSourceFileOps{
		createTemp: os.CreateTemp,
		rename:     os.Rename,
		remove:     os.Remove,
		syncDir:    syncDirectory,
	}
}

func writeSchemaSource(name string, source SchemaSource, ops schemaSourceFileOps) error {
	body, err := json.Marshal(source)
	if err != nil {
		return fmt.Errorf("marshal schema source: %w", err)
	}
	parent := filepath.Dir(name)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create schema source directory: %w", err)
	}
	temp, err := ops.createTemp(parent, "."+filepath.Base(name)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create schema source temporary file: %w", err)
	}
	tempName := temp.Name()
	keep := false
	defer func() {
		_ = temp.Close()
		if !keep {
			_ = ops.remove(tempName)
		}
	}()
	if err := temp.Chmod(0o644); err != nil {
		return fmt.Errorf("chmod schema source temporary file: %w", err)
	}
	if _, err := temp.Write(body); err != nil {
		return fmt.Errorf("write schema source temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync schema source temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close schema source temporary file: %w", err)
	}
	if err := ops.rename(tempName, name); err != nil {
		return fmt.Errorf("publish schema source: %w", err)
	}
	keep = true
	if err := ops.syncDir(parent); err != nil {
		return fmt.Errorf("sync schema source directory: %w", err)
	}
	return nil
}

func syncDirectory(name string) error {
	dir, err := os.Open(name)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
