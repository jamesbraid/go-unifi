package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	aceJarPath       = "usr/lib/unifi/lib/ace.jar"
	productPropsPath = "BOOT-INF/classes/product.properties"
	internalJarPath  = "BOOT-INF/lib/internal-dependencies.jar"
)

var metadataAllowlist = []string{
	"country_codes_list.json",
	"event_defs.json",
	"geo_ip_country_codes_list.json",
	"legacy_endpoint_segments.json",
	"radio_specification.json",
	"sensitive_metadata.json",
	"ssl-inspection-file-extension.json",
	"timezones.json",
}

type ExtractedArtifact struct {
	Name   string
	Path   string
	SHA256 string
	Size   int64
}

type ExtractedDefinitions struct {
	NetworkVersion  string
	Fields          map[string]ExtractedArtifact
	Metadata        map[string]ExtractedArtifact
	Notices         map[string]ExtractedArtifact
	MissingOptional []string
}

func ExtractUOSInstaller(ctx context.Context, installerPath, tempRoot string, limits ArchiveLimits) (_ *ExtractedDefinitions, retErr error) {
	if err := limits.validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	outRoot, err := os.MkdirTemp(tempRoot, "uos-extract-*")
	if err != nil {
		return nil, fmt.Errorf("create extraction directory: %w", err)
	}
	defer func() {
		if retErr != nil {
			_ = os.RemoveAll(outRoot)
		}
	}()

	installer, err := zip.OpenReader(installerPath)
	if err != nil {
		return nil, fmt.Errorf("open appended installer ZIP: %w", err)
	}
	defer installer.Close()
	installerEntries, err := indexZipFiles(ctx, installer.File, limits.MaxEntries)
	if err != nil {
		return nil, fmt.Errorf("installer ZIP: %w", err)
	}
	imageEntry, ok := installerEntries["image.tar"]
	if !ok {
		return nil, errors.New("installer ZIP: image.tar is missing")
	}
	if !imageEntry.Mode().IsRegular() {
		return nil, errors.New("installer ZIP: image.tar is not a regular file")
	}
	if imageEntry.UncompressedSize64 > uint64(limits.MaxImageTarBytes) {
		return nil, fmt.Errorf("installer ZIP: image.tar exceeds %d bytes", limits.MaxImageTarBytes)
	}
	imageReader, err := imageEntry.Open()
	if err != nil {
		return nil, fmt.Errorf("open installer image.tar: %w", err)
	}
	layout, importErr := ImportOCI(&contextReader{ctx: ctx, r: imageReader}, tempRoot, limits)
	closeErr := imageReader.Close()
	if importErr != nil {
		return nil, fmt.Errorf("import OCI image: %w", importErr)
	}
	if closeErr != nil {
		_ = os.RemoveAll(layout.Root)
		return nil, fmt.Errorf("close installer image.tar: %w", closeErr)
	}
	defer os.RemoveAll(layout.Root)

	image, err := ResolveImage(layout, v1.Platform{OS: "linux", Architecture: "amd64"})
	if err != nil {
		return nil, fmt.Errorf("resolve OCI image: %w", err)
	}
	ace, err := FindFileInLayers(image, aceJarPath, limits)
	if err != nil {
		return nil, fmt.Errorf("find ace.jar: %w", err)
	}
	acePath := ace.Name()
	defer func() { ace.Close(); _ = os.Remove(acePath) }()
	aceStat, err := ace.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat ace.jar: %w", err)
	}
	aceZip, err := zip.NewReader(ace, aceStat.Size())
	if err != nil {
		return nil, fmt.Errorf("open ace.jar: %w", err)
	}
	aceEntries, err := indexZipFiles(ctx, aceZip.File, limits.MaxEntries)
	if err != nil {
		return nil, fmt.Errorf("ace.jar: %w", err)
	}

	result := &ExtractedDefinitions{
		Fields: make(map[string]ExtractedArtifact), Metadata: make(map[string]ExtractedArtifact), Notices: make(map[string]ExtractedArtifact),
	}
	props, ok := aceEntries[productPropsPath]
	if !ok {
		return nil, fmt.Errorf("ace.jar: %s is missing", productPropsPath)
	}
	propsBody, err := readZipEntry(ctx, props, limits.MaxJSONBytes)
	if err != nil {
		return nil, fmt.Errorf("ace.jar %s: %w", productPropsPath, err)
	}
	result.NetworkVersion, err = parseNetworkVersion(propsBody)
	if err != nil {
		return nil, fmt.Errorf("ace.jar %s: %w", productPropsPath, err)
	}
	if err := extractNotices(ctx, aceEntries, "ace.jar", outRoot, limits, result.Notices); err != nil {
		return nil, err
	}

	internalEntry, ok := aceEntries[internalJarPath]
	if !ok {
		return nil, fmt.Errorf("ace.jar: %s is missing", internalJarPath)
	}
	internal, err := spoolZipEntry(ctx, internalEntry, outRoot, "internal-*.jar", limits.MaxJarBytes)
	if err != nil {
		return nil, fmt.Errorf("ace.jar %s: %w", internalJarPath, err)
	}
	internalPath := internal.Name()
	defer func() { internal.Close(); _ = os.Remove(internalPath) }()
	internalStat, err := internal.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat internal-dependencies.jar: %w", err)
	}
	internalZip, err := zip.NewReader(internal, internalStat.Size())
	if err != nil {
		return nil, fmt.Errorf("open internal-dependencies.jar: %w", err)
	}
	innerEntries, err := indexZipFiles(ctx, internalZip.File, limits.MaxEntries)
	if err != nil {
		return nil, fmt.Errorf("internal-dependencies.jar: %w", err)
	}

	metadataSet := make(map[string]struct{}, len(metadataAllowlist))
	for _, name := range metadataAllowlist {
		metadataSet[name] = struct{}{}
	}
	for name, entry := range innerEntries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch {
		case strings.HasPrefix(name, "api/fields/") && strings.HasSuffix(name, ".json"):
			artifact, err := extractZipArtifact(ctx, entry, name, outRoot, limits.MaxJSONBytes)
			if err != nil {
				return nil, fmt.Errorf("internal-dependencies.jar %s: %w", name, err)
			}
			result.Fields[name] = artifact
		case path.Dir(name) == ".":
			if _, ok := metadataSet[name]; ok {
				artifact, err := extractZipArtifact(ctx, entry, name, outRoot, limits.MaxJSONBytes)
				if err != nil {
					return nil, fmt.Errorf("internal-dependencies.jar %s: %w", name, err)
				}
				result.Metadata[name] = artifact
			}
		}
	}
	if err := extractNotices(ctx, innerEntries, "internal-dependencies.jar", outRoot, limits, result.Notices); err != nil {
		return nil, err
	}
	if _, ok := result.Fields["api/fields/Setting.json"]; !ok {
		return nil, errors.New("required artifact api/fields/Setting.json is missing")
	}
	if len(result.Fields) < 2 {
		return nil, errors.New("at least one field definition in addition to Setting.json is required")
	}
	if _, ok := result.Metadata["sensitive_metadata.json"]; !ok {
		return nil, errors.New("required artifact sensitive_metadata.json is missing")
	}
	for _, name := range metadataAllowlist {
		if name == "sensitive_metadata.json" {
			continue
		}
		if _, ok := result.Metadata[name]; !ok {
			result.MissingOptional = append(result.MissingOptional, name)
		}
	}
	sort.Strings(result.MissingOptional)
	return result, nil
}

func indexZipFiles(ctx context.Context, files []*zip.File, maxEntries int) (map[string]*zip.File, error) {
	if len(files) > maxEntries {
		return nil, fmt.Errorf("archive exceeds %d entries", maxEntries)
	}
	result := make(map[string]*zip.File, len(files))
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name, err := cleanArchiveName(file.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid path %q: %w", file.Name, err)
		}
		if _, exists := result[name]; exists {
			return nil, fmt.Errorf("duplicate entry %q", name)
		}
		mode := file.Mode()
		if !mode.IsRegular() && !mode.IsDir() {
			return nil, fmt.Errorf("unsupported entry type for %q", name)
		}
		result[name] = file
	}
	return result, nil
}

func parseNetworkVersion(body []byte) (string, error) {
	var version string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "version" {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return "", errors.New("version is empty")
		}
		if version != "" {
			return "", errors.New("version is duplicated")
		}
		version = value
	}
	if version == "" {
		return "", errors.New("version is missing")
	}
	return version, nil
}

func extractNotices(ctx context.Context, entries map[string]*zip.File, source, outRoot string, limits ArchiveLimits, notices map[string]ExtractedArtifact) error {
	for name, entry := range entries {
		if !isNoticePath(name) || entry.Mode().IsDir() {
			continue
		}
		key := source + "/" + name
		artifact, err := extractZipArtifact(ctx, entry, key, outRoot, limits.MaxJSONBytes)
		if err != nil {
			return fmt.Errorf("%s %s: %w", source, name, err)
		}
		notices[key] = artifact
	}
	return nil
}

func isNoticePath(name string) bool {
	dir := path.Dir(name)
	if dir != "." && dir != "META-INF" && !strings.HasPrefix(dir, "META-INF/") {
		return false
	}
	base := strings.ToUpper(path.Base(name))
	return base == "LICENSE" || strings.HasPrefix(base, "LICENSE.") || base == "NOTICE" || strings.HasPrefix(base, "NOTICE.")
}

func extractZipArtifact(ctx context.Context, entry *zip.File, name, outRoot string, limit int64) (ExtractedArtifact, error) {
	if !entry.Mode().IsRegular() {
		return ExtractedArtifact{}, errors.New("entry is not a regular file")
	}
	if entry.UncompressedSize64 > uint64(limit) {
		return ExtractedArtifact{}, fmt.Errorf("entry exceeds %d bytes", limit)
	}
	src, err := entry.Open()
	if err != nil {
		return ExtractedArtifact{}, err
	}
	defer src.Close()
	dst, err := os.CreateTemp(outRoot, "artifact-*")
	if err != nil {
		return ExtractedArtifact{}, err
	}
	dstPath := dst.Name()
	keep := false
	defer func() {
		dst.Close()
		if !keep {
			_ = os.Remove(dstPath)
		}
	}()
	hasher := sha256.New()
	n, err := copyBounded(io.MultiWriter(dst, hasher), &contextReader{ctx: ctx, r: src}, limit)
	if err != nil {
		return ExtractedArtifact{}, err
	}
	if n != int64(entry.UncompressedSize64) {
		return ExtractedArtifact{}, fmt.Errorf("entry size mismatch: expected %d, got %d", entry.UncompressedSize64, n)
	}
	if err := dst.Close(); err != nil {
		return ExtractedArtifact{}, err
	}
	keep = true
	return ExtractedArtifact{Name: name, Path: dstPath, SHA256: hex.EncodeToString(hasher.Sum(nil)), Size: n}, nil
}

func readZipEntry(ctx context.Context, entry *zip.File, limit int64) ([]byte, error) {
	if entry.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("entry exceeds %d bytes", limit)
	}
	r, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var buf strings.Builder
	n, err := copyBounded(&buf, &contextReader{ctx: ctx, r: r}, limit)
	if err != nil {
		return nil, err
	}
	if n != int64(entry.UncompressedSize64) {
		return nil, fmt.Errorf("entry size mismatch")
	}
	return []byte(buf.String()), nil
}

func spoolZipEntry(ctx context.Context, entry *zip.File, tempRoot, pattern string, limit int64) (*os.File, error) {
	if !entry.Mode().IsRegular() {
		return nil, errors.New("entry is not a regular file")
	}
	if entry.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("entry exceeds %d bytes", limit)
	}
	r, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	f, err := os.CreateTemp(tempRoot, pattern)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			f.Close()
			_ = os.Remove(f.Name())
		}
	}()
	n, err := copyBounded(f, &contextReader{ctx: ctx, r: r}, limit)
	if err != nil {
		return nil, err
	}
	if n != int64(entry.UncompressedSize64) {
		return nil, fmt.Errorf("entry size mismatch")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	keep = true
	return f, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	if err == nil {
		if ctxErr := r.ctx.Err(); ctxErr != nil {
			return n, ctxErr
		}
	}
	return n, err
}
