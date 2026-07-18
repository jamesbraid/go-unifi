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
	"sync"

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
	extractionRoot  string
	cleanupOnce     sync.Once
	cleanupErr      error
}

// Close removes the temporary files owned by the extracted definitions. It is
// safe to call more than once. The source installer is owned by the caller and
// is never removed here.
func (d *ExtractedDefinitions) Close() error {
	if d == nil {
		return nil
	}
	d.cleanupOnce.Do(func() {
		if d.extractionRoot != "" {
			d.cleanupErr = os.RemoveAll(d.extractionRoot)
		}
	})
	return d.cleanupErr
}

// Cleanup is an alias for Close.
func (d *ExtractedDefinitions) Cleanup() error {
	return d.Close()
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
		Fields: make(map[string]ExtractedArtifact), Metadata: make(map[string]ExtractedArtifact), Notices: make(map[string]ExtractedArtifact), extractionRoot: outRoot,
	}
	noticeInventory := newNoticeInventory(outRoot, limits, result.Notices)
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
	if err := noticeInventory.extract(ctx, aceEntries, "ace.jar", isNoticePath); err != nil {
		return nil, err
	}
	if err := extractDependencyNotices(ctx, aceEntries, outRoot, limits, noticeInventory); err != nil {
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
	if err := noticeInventory.extract(ctx, innerEntries, "internal-dependencies.jar", isNoticePath); err != nil {
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

type noticeInventory struct {
	outRoot      string
	limits       ArchiveLimits
	notices      map[string]ExtractedArtifact
	destinations map[string]string
	entries      int
	bytes        int64
}

func newNoticeInventory(outRoot string, limits ArchiveLimits, notices map[string]ExtractedArtifact) *noticeInventory {
	return &noticeInventory{outRoot: outRoot, limits: limits, notices: notices, destinations: make(map[string]string)}
}

func (inventory *noticeInventory) extract(ctx context.Context, entries map[string]*zip.File, source string, matches func(string) bool) error {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := entries[name]
		if !matches(name) || entry.Mode().IsDir() {
			continue
		}
		key := source + "/" + name
		folded := strings.ToLower(key)
		if previous, exists := inventory.destinations[folded]; exists {
			return fmt.Errorf("notice destination case-fold collision: %q and %q", previous, key)
		}
		if inventory.entries >= inventory.limits.MaxNoticeEntries {
			return fmt.Errorf("notice inventory exceeds %d entries", inventory.limits.MaxNoticeEntries)
		}
		remaining := inventory.limits.MaxNoticeBytes - inventory.bytes
		if remaining <= 0 || entry.UncompressedSize64 > uint64(remaining) {
			return fmt.Errorf("notice inventory exceeds %d bytes", inventory.limits.MaxNoticeBytes)
		}
		artifact, err := extractZipArtifact(ctx, entry, key, inventory.outRoot, remaining)
		if err != nil {
			return fmt.Errorf("%s %s: %w", source, name, err)
		}
		inventory.destinations[folded] = key
		inventory.notices[key] = artifact
		inventory.entries++
		inventory.bytes += artifact.Size
	}
	return nil
}

func extractDependencyNotices(ctx context.Context, aceEntries map[string]*zip.File, outRoot string, limits ArchiveLimits, inventory *noticeInventory) error {
	jarNames := make([]string, 0)
	jarDestinations := make(map[string]string)
	for name, entry := range aceEntries {
		if path.Dir(name) != "BOOT-INF/lib" || !strings.EqualFold(path.Ext(name), ".jar") || entry.Mode().IsDir() {
			continue
		}
		folded := strings.ToLower(name)
		if previous, exists := jarDestinations[folded]; exists {
			return fmt.Errorf("dependency JAR namespace case-fold collision: %q and %q", previous, name)
		}
		jarDestinations[folded] = name
		jarNames = append(jarNames, name)
	}
	sort.Strings(jarNames)
	if len(jarNames) > limits.MaxNestedArchives {
		return fmt.Errorf("dependency JAR inventory exceeds %d archives", limits.MaxNestedArchives)
	}
	for _, name := range jarNames {
		if err := extractDependencyJarNotices(ctx, aceEntries[name], name, outRoot, limits, inventory); err != nil {
			return err
		}
	}
	return nil
}

func extractDependencyJarNotices(ctx context.Context, entry *zip.File, jarName, outRoot string, limits ArchiveLimits, inventory *noticeInventory) error {
	spooled, err := spoolZipEntry(ctx, entry, outRoot, "dependency-*.jar", limits.MaxJarBytes)
	if err != nil {
		return fmt.Errorf("ace.jar %s: %w", jarName, err)
	}
	spoolPath := spooled.Name()
	defer func() {
		_ = spooled.Close()
		_ = os.Remove(spoolPath)
	}()
	stat, err := spooled.Stat()
	if err != nil {
		return fmt.Errorf("stat dependency JAR %s: %w", jarName, err)
	}
	archive, err := zip.NewReader(spooled, stat.Size())
	if err != nil {
		return fmt.Errorf("open dependency JAR %s: %w", jarName, err)
	}
	entries, err := indexZipFiles(ctx, archive.File, limits.MaxEntries)
	if err != nil {
		return fmt.Errorf("dependency JAR %s: %w", jarName, err)
	}
	if err := validateZipEntryBodies(ctx, entries, limits.MaxJarBytes); err != nil {
		return fmt.Errorf("dependency JAR %s: %w", jarName, err)
	}
	if err := inventory.extract(ctx, entries, "ace.jar/"+jarName, isDependencyNoticePath); err != nil {
		return fmt.Errorf("dependency JAR %s: %w", jarName, err)
	}
	return nil
}

func validateZipEntryBodies(ctx context.Context, entries map[string]*zip.File, maxBytes int64) error {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	var total int64
	for _, name := range names {
		entry := entries[name]
		if entry.Mode().IsDir() {
			continue
		}
		remaining := maxBytes - total
		if remaining <= 0 || entry.UncompressedSize64 > uint64(remaining) {
			return fmt.Errorf("expanded archive exceeds %d bytes", maxBytes)
		}
		reader, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open %s: %w", name, err)
		}
		n, copyErr := copyBounded(io.Discard, &contextReader{ctx: ctx, r: reader}, remaining)
		closeErr := reader.Close()
		if copyErr != nil {
			return fmt.Errorf("validate %s: %w", name, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s: %w", name, closeErr)
		}
		if n != int64(entry.UncompressedSize64) {
			return fmt.Errorf("validate %s: entry size mismatch", name)
		}
		total += n
	}
	return nil
}

func isNoticePath(name string) bool {
	dir := path.Dir(name)
	if dir != "." && dir != "META-INF" && !strings.HasPrefix(dir, "META-INF/") {
		return false
	}
	base := strings.ToUpper(path.Base(name))
	return strings.HasPrefix(base, "LICENSE") || strings.HasPrefix(base, "NOTICE")
}

func isDependencyNoticePath(name string) bool {
	dir := path.Dir(name)
	if dir != "." && dir != "META-INF" && !strings.HasPrefix(dir, "META-INF/") {
		return false
	}
	return hasNoticeBasename(name, []string{"LICENSE", "NOTICE", "COPYING", "COPYRIGHT", "THIRD-PARTY", "THIRD_PARTY", "THIRDPARTY"})
}

func hasNoticeBasename(name string, families []string) bool {
	base := strings.ToUpper(path.Base(name))
	for _, family := range families {
		if base == family {
			return true
		}
		if !strings.HasPrefix(base, family) || len(base) == len(family) {
			continue
		}
		remainder := base[len(family):]
		if remainder == ".TXT" || remainder == ".MD" {
			return true
		}
		if remainder[0] == '-' || remainder[0] == '_' {
			extension := strings.ToUpper(path.Ext(remainder))
			if extension == "" || extension == ".TXT" || extension == ".MD" {
				return true
			}
		}
	}
	return false
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
