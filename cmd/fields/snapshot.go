package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
)

type SnapshotOptions struct {
	Root          string
	CustomDir     string
	Source        InstallerSource
	Installer     *MaterializedInstaller
	Definitions   *ExtractedDefinitions
	PolicyVersion string
	fileOps       *snapshotFileOps
}

type snapshotFileOps struct {
	mkdirTemp func(string, string) (string, error)
	rename    func(string, string) error
	removeAll func(string) error
}

func defaultSnapshotFileOps() snapshotFileOps {
	return snapshotFileOps{mkdirTemp: os.MkdirTemp, rename: os.Rename, removeAll: os.RemoveAll}
}

func BuildSnapshot(ctx context.Context, options SnapshotOptions) (_ *LocalManifest, retErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if options.Root == "" {
		return nil, errors.New("snapshot root is empty")
	}
	if options.CustomDir == "" {
		return nil, errors.New("custom definitions directory is empty")
	}
	if options.Installer == nil {
		return nil, errors.New("materialized installer is nil")
	}
	if options.Definitions == nil {
		return nil, errors.New("extracted definitions are nil")
	}
	version, err := snapshotVersion(options.Definitions.NetworkVersion)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(options.Root, 0o755); err != nil {
		return nil, fmt.Errorf("create snapshot root: %w", err)
	}
	ops := defaultSnapshotFileOps()
	if options.fileOps != nil {
		ops = *options.fileOps
	}
	stage, err := ops.mkdirTemp(options.Root, "."+version+".staging-*")
	if err != nil {
		return nil, fmt.Errorf("create snapshot staging directory: %w", err)
	}
	if err := os.Chmod(stage, 0o755); err != nil {
		_ = ops.removeAll(stage)
		return nil, fmt.Errorf("chmod snapshot staging directory: %w", err)
	}
	removeStage := true
	defer func() {
		if removeStage {
			_ = ops.removeAll(stage)
		}
	}()

	manifest, err := materializeSnapshot(ctx, stage, options)
	if err != nil {
		return nil, err
	}
	if err := normalizeSnapshotModes(stage); err != nil {
		return nil, fmt.Errorf("normalize staged snapshot modes: %w", err)
	}
	if err := validateSnapshotTree(stage); err != nil {
		return nil, fmt.Errorf("validate staged snapshot: %w", err)
	}
	if err := syncSnapshotTree(stage); err != nil {
		return nil, fmt.Errorf("sync staged snapshot: %w", err)
	}

	final := filepath.Join(options.Root, version)
	backup, err := reserveBackupPath(options.Root, version, ops)
	if err != nil {
		return nil, err
	}
	hadCurrent := false
	if info, statErr := os.Lstat(final); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("existing snapshot is not a regular directory: %s", final)
		}
		if err := ops.rename(final, backup); err != nil {
			return nil, fmt.Errorf("move current snapshot %s to backup %s: %w", final, backup, err)
		}
		hadCurrent = true
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("stat current snapshot: %w", statErr)
	}

	if err := ops.rename(stage, final); err != nil {
		removeStage = false
		if hadCurrent {
			if restoreErr := ops.rename(backup, final); restoreErr != nil {
				return nil, fmt.Errorf("publish staged snapshot %s to %s: %v; restore backup %s to %s: %w", stage, final, err, backup, final, restoreErr)
			}
		}
		return nil, fmt.Errorf("publish staged snapshot %s to %s (backup %s): %w", stage, final, backup, err)
	}
	removeStage = false
	if err := syncDirectory(options.Root); err != nil {
		return nil, fmt.Errorf("sync snapshot parent after publication (final %s, backup %s): %w", final, backup, err)
	}
	if hadCurrent {
		if err := ops.removeAll(backup); err != nil {
			return nil, fmt.Errorf("remove successful snapshot backup %s: %w", backup, err)
		}
		if err := syncDirectory(options.Root); err != nil {
			return nil, fmt.Errorf("sync snapshot parent after backup removal: %w", err)
		}
	}
	return manifest, nil
}

func materializeSnapshot(ctx context.Context, stage string, options SnapshotOptions) (*LocalManifest, error) {
	definitions := options.Definitions
	upstreamBodies := make(map[string][]byte, len(definitions.Fields))
	artifacts := make(map[string]string, len(definitions.Fields)+len(definitions.Metadata)+len(definitions.Notices))
	flatNames := make(map[string]string)

	fieldNames := sortedArtifactNames(definitions.Fields)
	for _, name := range fieldNames {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := validateFieldName(name); err != nil {
			return nil, err
		}
		artifact := definitions.Fields[name]
		body, err := readArtifact(artifact)
		if err != nil {
			return nil, fmt.Errorf("read upstream field %s: %w", name, err)
		}
		if _, err := canonicalJSON(body); err != nil {
			return nil, fmt.Errorf("upstream field %s is invalid JSON: %w", name, err)
		}
		basename := path.Base(name)
		folded := strings.ToLower(basename)
		if previous, exists := flatNames[folded]; exists {
			return nil, fmt.Errorf("flattened field basename collision between %s and %s", previous, name)
		}
		flatNames[folded] = name
		upstreamBodies[name] = body
		if err := addArtifactHash(artifacts, name, artifact.SHA256); err != nil {
			return nil, err
		}
		if err := writeSnapshotFile(filepath.Join(stage, basename), body); err != nil {
			return nil, err
		}
		rawPath := filepath.Join(stage, "metadata", "raw-fields", filepath.FromSlash(name))
		if err := writeSnapshotFile(rawPath, body); err != nil {
			return nil, err
		}
	}

	setting, ok := upstreamBodies["api/fields/Setting.json"]
	if !ok {
		return nil, errors.New("required upstream api/fields/Setting.json is missing")
	}
	splitNames, err := splitSettings(stage, setting, flatNames)
	if err != nil {
		return nil, err
	}
	if err := overlayCustom(options.CustomDir, stage, flatNames, splitNames); err != nil {
		return nil, err
	}

	metadataBodies := make(map[string][]byte, len(definitions.Metadata))
	metadataNames := sortedArtifactNames(definitions.Metadata)
	metadataDestinations := make(map[string]snapshotDestination, len(metadataNames)+3)
	for _, reserved := range []string{"metadata/source.json", "metadata/raw-fields", "metadata/notices"} {
		if err := addSnapshotDestination(metadataDestinations, reserved, "generated "+reserved); err != nil {
			return nil, err
		}
	}
	for _, name := range metadataNames {
		if err := validateDigestPath(name); err != nil {
			return nil, fmt.Errorf("invalid metadata path: %w", err)
		}
		if err := addSnapshotDestination(metadataDestinations, path.Join("metadata", path.Base(name)), name); err != nil {
			return nil, err
		}
	}
	for _, name := range metadataNames {
		artifact := definitions.Metadata[name]
		body, err := readArtifact(artifact)
		if err != nil {
			return nil, fmt.Errorf("read metadata %s: %w", name, err)
		}
		basename := path.Base(name)
		if basename == "." || basename == "" {
			return nil, fmt.Errorf("invalid metadata name %q", name)
		}
		metadataBodies[basename] = body
		if err := addArtifactHash(artifacts, name, artifact.SHA256); err != nil {
			return nil, err
		}
		if err := writeSnapshotFile(filepath.Join(stage, "metadata", basename), body); err != nil {
			return nil, err
		}
	}
	sensitive, ok := metadataBodies["sensitive_metadata.json"]
	if !ok {
		return nil, errors.New("required sensitive_metadata.json is missing")
	}
	sensitivityDigest, err := CanonicalJSONDigest(sensitive)
	if err != nil {
		return nil, fmt.Errorf("canonicalize sensitive_metadata.json: %w", err)
	}

	noticeNames := sortedArtifactNames(definitions.Notices)
	noticeDestinations := make(map[string]snapshotDestination, len(noticeNames))
	for _, name := range noticeNames {
		if err := validateNoticeName(name); err != nil {
			return nil, err
		}
		if err := addSnapshotDestination(noticeDestinations, path.Join("metadata/notices", name), name); err != nil {
			return nil, err
		}
	}
	noticeBodies := make(map[string][]byte, len(definitions.Notices))
	for _, name := range noticeNames {
		artifact := definitions.Notices[name]
		body, err := readArtifact(artifact)
		if err != nil {
			return nil, fmt.Errorf("read notice %s: %w", name, err)
		}
		noticeBodies[name] = body
		if err := addArtifactHash(artifacts, name, artifact.SHA256); err != nil {
			return nil, err
		}
		if err := writeSnapshotFile(filepath.Join(stage, "metadata", "notices", filepath.FromSlash(name)), body); err != nil {
			return nil, err
		}
	}

	schemaDigest, err := CanonicalTreeDigest(upstreamBodies)
	if err != nil {
		return nil, fmt.Errorf("digest upstream schema tree: %w", err)
	}
	noticeDigest, err := CanonicalTreeDigest(noticeBodies)
	if err != nil {
		return nil, fmt.Errorf("digest notice tree: %w", err)
	}
	missing := sortedUnique(definitions.MissingOptional)
	manifest := &LocalManifest{
		OSVersion: options.Source.OSVersion, NetworkVersion: definitions.NetworkVersion,
		FirmwareID: options.Source.FirmwareID, InstallerURL: installerURL(options.Source.URL),
		InstallerSHA256: strings.ToLower(options.Installer.SHA256), InstallerMD5: strings.ToLower(options.Source.ExpectedMD5),
		Product: options.Source.Product, Platform: options.Source.Platform, Channel: options.Source.Channel,
		SchemaDigest: schemaDigest, SensitivityDigest: sensitivityDigest, NoticeDigest: noticeDigest,
		PolicyVersion: options.PolicyVersion, InstallerSize: options.Installer.Size,
		Created: options.Source.Created, Updated: options.Source.Updated,
		Artifacts: artifacts, MissingOptional: missing,
	}
	manifestBody, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal local snapshot manifest: %w", err)
	}
	if err := writeSnapshotFile(filepath.Join(stage, "metadata", "source.json"), manifestBody); err != nil {
		return nil, err
	}
	return manifest, nil
}

func snapshotVersion(networkVersion string) (string, error) {
	version := strings.TrimPrefix(networkVersion, "v")
	if version == "" || strings.ContainsAny(version, `/\\`) || version == "." || version == ".." {
		return "", fmt.Errorf("invalid Network version %q", networkVersion)
	}
	return "v" + version, nil
}

func validateFieldName(name string) error {
	if strings.Contains(name, "\\") || path.Clean(name) != name || !strings.HasPrefix(name, "api/fields/") || !strings.HasSuffix(name, ".json") || path.Base(name) == "." {
		return fmt.Errorf("invalid upstream field path %q", name)
	}
	return nil
}

func validateNoticeName(name string) error {
	if err := validateDigestPath(name); err != nil {
		return fmt.Errorf("invalid notice path: %w", err)
	}
	parts := strings.Split(name, "/")
	if len(parts) < 2 || parts[0] == "" {
		return fmt.Errorf("notice path %q is missing its source namespace", name)
	}
	return nil
}

type snapshotDestination struct {
	path, source string
}

func addSnapshotDestination(destinations map[string]snapshotDestination, destination, source string) error {
	normalized := path.Clean(destination)
	key := strings.ToLower(normalized)
	if previous, exists := destinations[key]; exists {
		return fmt.Errorf("snapshot destination collision between %q (%s) and %q (%s)", previous.source, previous.path, source, normalized)
	}
	destinations[key] = snapshotDestination{path: normalized, source: source}
	return nil
}

func splitSettings(stage string, body []byte, upstream map[string]string) (map[string]string, error) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	settings := make(map[string]any)
	if err := decoder.Decode(&settings); err != nil {
		return nil, fmt.Errorf("decode Setting.json: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("Setting.json contains trailing data")
	}
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		name := "Setting" + strcase.ToCamel(key) + ".json"
		folded := strings.ToLower(name)
		if previous, exists := upstream[folded]; exists {
			return nil, fmt.Errorf("split setting %q collides with upstream field %s", key, previous)
		}
		if previous, exists := result[folded]; exists {
			return nil, fmt.Errorf("split setting basename collision between %q and %q", previous, key)
		}
		encoded, err := json.MarshalIndent(settings[key], "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode setting %q: %w", key, err)
		}
		if err := writeSnapshotFile(filepath.Join(stage, name), encoded); err != nil {
			return nil, err
		}
		result[folded] = key
	}
	return result, nil
}

func overlayCustom(customDir, stage string, upstream, split map[string]string) error {
	entries, err := os.ReadDir(customDir)
	if err != nil {
		return fmt.Errorf("read custom definitions: %w", err)
	}
	seen := make(map[string]string, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat custom definition %s: %w", name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(name), ".json") {
			return fmt.Errorf("custom definition %s is not a direct-child regular JSON file", name)
		}
		if strings.EqualFold(name, "Setting.json") {
			return errors.New("custom Setting.json is forbidden")
		}
		folded := strings.ToLower(name)
		if previous, exists := seen[folded]; exists {
			return fmt.Errorf("custom definition basename collision between %s and %s", previous, name)
		}
		seen[folded] = name
		if setting, exists := split[folded]; exists {
			return fmt.Errorf("custom definition %s collides with split setting %q", name, setting)
		}
		body, err := os.ReadFile(filepath.Join(customDir, name))
		if err != nil {
			return fmt.Errorf("read custom definition %s: %w", name, err)
		}
		if _, err := canonicalJSON(body); err != nil {
			return fmt.Errorf("custom definition %s is invalid JSON: %w", name, err)
		}
		if previous, exists := upstream[folded]; exists && path.Base(previous) != name {
			return fmt.Errorf("custom definition %s has case-only upstream collision with %s", name, previous)
		}
		if err := writeSnapshotFile(filepath.Join(stage, name), body); err != nil {
			return err
		}
	}
	return nil
}

func readArtifact(artifact ExtractedArtifact) ([]byte, error) {
	info, err := os.Lstat(artifact.Path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("artifact is not a regular file")
	}
	return os.ReadFile(artifact.Path)
}

func writeSnapshotFile(name string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return fmt.Errorf("create snapshot directory for %s: %w", name, err)
	}
	if err := os.Chmod(filepath.Dir(name), 0o755); err != nil {
		return fmt.Errorf("chmod snapshot directory for %s: %w", name, err)
	}
	if err := os.WriteFile(name, body, 0o644); err != nil {
		return fmt.Errorf("write snapshot file %s: %w", name, err)
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return fmt.Errorf("chmod snapshot file %s: %w", name, err)
	}
	return nil
}

func validateSnapshotTree(root string) error {
	return filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is forbidden: %s", name)
		}
		if entry.IsDir() {
			if info.Mode().Perm() != 0o755 {
				return fmt.Errorf("directory mode for %s is %o, want 755", name, info.Mode().Perm())
			}
			return nil
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
			return fmt.Errorf("file %s is not regular mode 0644", name)
		}
		return nil
	})
}

func normalizeSnapshotModes(root string) error {
	return filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is forbidden: %s", name)
		}
		mode := os.FileMode(0o644)
		if entry.IsDir() {
			mode = 0o755
		}
		return os.Chmod(name, mode)
	})
}

func syncSnapshotTree(root string) error {
	directories := make([]string, 0)
	if err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			directories = append(directories, name)
			return nil
		}
		file, err := os.Open(name)
		if err != nil {
			return err
		}
		syncErr := file.Sync()
		closeErr := file.Close()
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	}); err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := syncDirectory(directories[index]); err != nil {
			return err
		}
	}
	return nil
}

func reserveBackupPath(root, version string, ops snapshotFileOps) (string, error) {
	backup, err := ops.mkdirTemp(root, "."+version+".backup-*")
	if err != nil {
		return "", fmt.Errorf("reserve snapshot backup path: %w", err)
	}
	if err := ops.removeAll(backup); err != nil {
		return "", fmt.Errorf("clear reserved snapshot backup path %s: %w", backup, err)
	}
	return backup, nil
}

func sortedArtifactNames(values map[string]ExtractedArtifact) []string {
	result := make([]string, 0, len(values))
	for name := range values {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func sortedUnique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func addArtifactHash(artifacts map[string]string, name, digest string) error {
	if _, exists := artifacts[name]; exists {
		return fmt.Errorf("duplicate artifact provenance key %q", name)
	}
	artifacts[name] = strings.ToLower(digest)
	return nil
}

func installerURL(value *url.URL) string {
	if value == nil {
		return ""
	}
	return value.String()
}
