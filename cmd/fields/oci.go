package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	dockerMediaTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	dockerMediaTypeManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	dockerMediaTypeLayerGzip    = "application/vnd.docker.image.rootfs.diff.tar.gzip"
)

type OCILayout struct {
	Root   string
	limits ArchiveLimits
	blobs  map[digest.Digest]string
}

type ResolvedImage struct {
	Manifest v1.Manifest
	Blobs    map[digest.Digest]string
	root     string
}

func ImportOCI(r io.Reader, tempRoot string, limits ArchiveLimits) (_ *OCILayout, retErr error) {
	if err := limits.validate(); err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(tempRoot, "oci-layout-*")
	if err != nil {
		return nil, fmt.Errorf("create OCI layout: %w", err)
	}
	defer func() {
		if retErr != nil {
			_ = os.RemoveAll(root)
		}
	}()

	max, _ := limitPlusOne(limits.MaxImageTarBytes)
	lr := &io.LimitedReader{R: r, N: max}
	tr := tar.NewReader(lr)
	seen := make(map[string]struct{})
	blobs := make(map[digest.Digest]string)
	entries := 0
	for {
		h, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return nil, fmt.Errorf("read OCI image tar: %w", nextErr)
		}
		entries++
		if entries > limits.MaxEntries {
			return nil, fmt.Errorf("OCI image tar exceeds %d entries", limits.MaxEntries)
		}
		name, err := cleanArchiveName(h.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid OCI image tar path %q: %w", h.Name, err)
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("duplicate OCI image tar entry %q", name)
		}
		seen[name] = struct{}{}

		if h.Typeflag == tar.TypeDir {
			continue
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("unsupported OCI image tar entry type for %q", name)
		}
		selected := name == "oci-layout" || name == "index.json"
		var blobDigest digest.Digest
		if strings.HasPrefix(name, "blobs/") {
			parts := strings.Split(name, "/")
			if len(parts) != 3 {
				return nil, fmt.Errorf("invalid OCI blob path %q", name)
			}
			blobDigest = digest.Digest(parts[1] + ":" + parts[2])
			if err := blobDigest.Validate(); err != nil {
				return nil, fmt.Errorf("invalid OCI blob digest %q: %w", blobDigest, err)
			}
			selected = true
		}
		if !selected {
			continue
		}
		entryLimit := limits.MaxBlobBytes
		if name == "oci-layout" || name == "index.json" {
			entryLimit = maxOCIControlJSONBytes
		}
		if h.Size < 0 || h.Size > entryLimit {
			return nil, fmt.Errorf("OCI entry %q exceeds %d bytes", name, entryLimit)
		}
		dstPath := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
			return nil, fmt.Errorf("create OCI path: %w", err)
		}
		dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("create OCI entry %q: %w", name, err)
		}
		var verifier digest.Verifier
		writer := io.Writer(dst)
		if blobDigest != "" {
			verifier = blobDigest.Verifier()
			writer = io.MultiWriter(dst, verifier)
		}
		written, copyErr := copyBounded(writer, tr, entryLimit)
		closeErr := dst.Close()
		if copyErr != nil {
			return nil, fmt.Errorf("copy OCI entry %q: %w", name, copyErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close OCI entry %q: %w", name, closeErr)
		}
		if written != h.Size {
			return nil, fmt.Errorf("OCI entry %q size mismatch: header %d, copied %d", name, h.Size, written)
		}
		if verifier != nil && !verifier.Verified() {
			return nil, fmt.Errorf("OCI blob %s digest mismatch", blobDigest)
		}
		if blobDigest != "" {
			blobs[blobDigest] = dstPath
		}
	}
	if _, err := io.Copy(io.Discard, lr); err != nil {
		return nil, fmt.Errorf("drain OCI image tar: %w", err)
	}
	if lr.N == 0 {
		return nil, fmt.Errorf("OCI image tar exceeds %d bytes", limits.MaxImageTarBytes)
	}
	if err := validateLayoutHeader(filepath.Join(root, "oci-layout")); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(root, "index.json")); err != nil {
		return nil, fmt.Errorf("OCI index: %w", err)
	}
	return &OCILayout{Root: root, limits: limits, blobs: blobs}, nil
}

func ResolveImage(layout *OCILayout, platform v1.Platform) (*ResolvedImage, error) {
	if layout == nil {
		return nil, errors.New("OCI layout is nil")
	}
	if err := layout.limits.validate(); err != nil {
		return nil, err
	}
	rootBytes, err := readFileBounded(filepath.Join(layout.Root, "index.json"), maxOCIControlJSONBytes)
	if err != nil {
		return nil, fmt.Errorf("read OCI index: %w", err)
	}
	var root v1.Index
	if err := json.Unmarshal(rootBytes, &root); err != nil {
		return nil, fmt.Errorf("parse OCI index: %w", err)
	}
	if err := validateIndex(root); err != nil {
		return nil, fmt.Errorf("OCI index: %w", err)
	}

	var matches []v1.Manifest
	seenLeaves := make(map[digest.Digest]struct{})
	active := make(map[digest.Digest]struct{})
	var walk func([]v1.Descriptor, int) error
	walk = func(descs []v1.Descriptor, depth int) error {
		if depth > layout.limits.MaxIndexDepth {
			return fmt.Errorf("OCI nested index exceeds depth %d", layout.limits.MaxIndexDepth)
		}
		for _, desc := range descs {
			switch desc.MediaType {
			case v1.MediaTypeImageIndex, dockerMediaTypeManifestList:
				if _, ok := active[desc.Digest]; ok {
					return fmt.Errorf("OCI nested index cycle at %s", desc.Digest)
				}
				body, err := layout.readDescriptor(desc, maxOCIControlJSONBytes)
				if err != nil {
					return fmt.Errorf("read OCI nested index %s: %w", desc.Digest, err)
				}
				var nested v1.Index
				if err := json.Unmarshal(body, &nested); err != nil {
					return fmt.Errorf("parse OCI nested index %s: %w", desc.Digest, err)
				}
				if err := validateIndex(nested); err != nil {
					return fmt.Errorf("OCI nested index %s: %w", desc.Digest, err)
				}
				if nested.MediaType != "" && nested.MediaType != desc.MediaType {
					return fmt.Errorf("OCI nested index %s media type mismatch: descriptor %q, body %q", desc.Digest, desc.MediaType, nested.MediaType)
				}
				active[desc.Digest] = struct{}{}
				err = walk(nested.Manifests, depth+1)
				delete(active, desc.Digest)
				if err != nil {
					return err
				}
			case v1.MediaTypeImageManifest, dockerMediaTypeManifest:
				if desc.Platform == nil || desc.Platform.OS != platform.OS || desc.Platform.Architecture != platform.Architecture {
					continue
				}
				if _, ok := seenLeaves[desc.Digest]; ok {
					return fmt.Errorf("repeated matching OCI manifest %s", desc.Digest)
				}
				seenLeaves[desc.Digest] = struct{}{}
				body, err := layout.readDescriptor(desc, maxOCIControlJSONBytes)
				if err != nil {
					return fmt.Errorf("read OCI manifest %s: %w", desc.Digest, err)
				}
				var manifest v1.Manifest
				if err := json.Unmarshal(body, &manifest); err != nil {
					return fmt.Errorf("parse OCI manifest %s: %w", desc.Digest, err)
				}
				if manifest.SchemaVersion != 2 || manifest.MediaType != desc.MediaType {
					return fmt.Errorf("unsupported OCI manifest %s", desc.Digest)
				}
				matches = append(matches, manifest)
			default:
				// OCI indexes may contain safe auxiliary descriptors.
				continue
			}
		}
		return nil
	}
	if err := walk(root.Manifests, 0); err != nil {
		return nil, err
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("expected exactly one %s/%s OCI manifest, found %d", platform.OS, platform.Architecture, len(matches))
	}
	return &ResolvedImage{Manifest: matches[0], Blobs: layout.blobs, root: layout.Root}, nil
}

func FindFileInLayers(image *ResolvedImage, name string, limits ArchiveLimits) (*os.File, error) {
	if image == nil {
		return nil, errors.New("resolved image is nil")
	}
	if err := limits.validate(); err != nil {
		return nil, err
	}
	target, err := cleanArchiveName(name)
	if err != nil {
		return nil, fmt.Errorf("invalid layer target: %w", err)
	}
	for i := len(image.Manifest.Layers) - 1; i >= 0; i-- {
		desc := image.Manifest.Layers[i]
		if desc.MediaType != v1.MediaTypeImageLayer && !isGzipLayerMediaType(desc.MediaType) {
			return nil, fmt.Errorf("layer %s: unsupported media type %q", desc.Digest, desc.MediaType)
		}
		blob, err := openVerifiedBlob(image.Blobs, desc, limits.MaxBlobBytes)
		if err != nil {
			return nil, fmt.Errorf("layer %s: %w", desc.Digest, err)
		}
		var stream io.Reader = blob
		var gz *gzip.Reader
		if isGzipLayerMediaType(desc.MediaType) {
			gz, err = gzip.NewReader(blob)
			if err != nil {
				blob.Close()
				return nil, fmt.Errorf("layer %s gzip: %w", desc.Digest, err)
			}
			stream = gz
		}
		candidate, blocked, scanErr := scanLayer(stream, target, image.root, limits)
		if gz != nil {
			if err := gz.Close(); scanErr == nil && err != nil {
				scanErr = err
			}
		}
		if err := blob.Close(); scanErr == nil && err != nil {
			scanErr = err
		}
		if scanErr != nil {
			if candidate != nil {
				candidate.Close()
				_ = os.Remove(candidate.Name())
			}
			return nil, fmt.Errorf("layer %s: %w", desc.Digest, scanErr)
		}
		if candidate != nil {
			_, err := candidate.Seek(0, io.SeekStart)
			if err != nil {
				candidate.Close()
				return nil, err
			}
			return candidate, nil
		}
		if blocked {
			return nil, fmt.Errorf("%s: %w", target, os.ErrNotExist)
		}
	}
	return nil, fmt.Errorf("%s: %w", target, os.ErrNotExist)
}

func scanLayer(r io.Reader, target, tempRoot string, limits ArchiveLimits) (*os.File, bool, error) {
	max, _ := limitPlusOne(limits.MaxLayerBytes)
	lr := &io.LimitedReader{R: r, N: max}
	tr := tar.NewReader(lr)
	seen := make(map[string]struct{})
	entries := 0
	blocked := false
	var candidate *os.File
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return candidate, false, err
		}
		entries++
		if entries > limits.MaxEntries {
			return candidate, false, fmt.Errorf("layer exceeds %d entries", limits.MaxEntries)
		}
		entryName, err := cleanLayerHeaderName(h.Name, h.Typeflag == tar.TypeDir)
		if err != nil {
			return candidate, false, fmt.Errorf("invalid path %q: %w", h.Name, err)
		}
		if _, ok := seen[entryName]; ok {
			return candidate, false, fmt.Errorf("duplicate layer entry %q", entryName)
		}
		seen[entryName] = struct{}{}
		isTarget := entryName == target
		isTargetAncestor := strings.HasPrefix(target, entryName+"/")
		removed, isWhiteout := whiteoutTarget(entryName)
		if isTarget || isWhiteout {
			if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
				return candidate, false, fmt.Errorf("unsupported entry type for %q", entryName)
			}
		}
		if isTargetAncestor && h.Typeflag != tar.TypeDir && h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			return candidate, false, fmt.Errorf("unsupported entry type for target ancestor %q", entryName)
		}
		if isWhiteout && (removed == "" || target == removed || strings.HasPrefix(target, removed+"/")) {
			blocked = true
		}
		if isTarget {
			if h.Size < 0 || h.Size > limits.MaxJarBytes {
				return candidate, false, fmt.Errorf("target %q exceeds %d bytes", target, limits.MaxJarBytes)
			}
			candidate, err = os.CreateTemp(tempRoot, "layer-file-*")
			if err != nil {
				return nil, false, err
			}
			written, err := copyBounded(candidate, tr, limits.MaxJarBytes)
			if err != nil {
				return candidate, false, err
			}
			if written != h.Size {
				return candidate, false, fmt.Errorf("target %q size mismatch", target)
			}
		}
	}
	if _, err := io.Copy(io.Discard, lr); err != nil {
		return candidate, false, err
	}
	if lr.N == 0 {
		return candidate, false, fmt.Errorf("decompressed layer exceeds %d bytes", limits.MaxLayerBytes)
	}
	return candidate, blocked, nil
}

func whiteoutTarget(name string) (string, bool) {
	base := path.Base(name)
	dir := path.Dir(name)
	if base == ".wh..wh..opq" {
		if dir == "." {
			return "", true
		}
		return dir, true
	}
	if !strings.HasPrefix(base, ".wh.") {
		return "", false
	}
	removed := strings.TrimPrefix(base, ".wh.")
	if dir == "." {
		return removed, true
	}
	return path.Join(dir, removed), true
}

func (l *OCILayout) readDescriptor(desc v1.Descriptor, limit int64) ([]byte, error) {
	if desc.Data != nil {
		return nil, errors.New("embedded OCI descriptor data is unsupported")
	}
	if err := desc.Digest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid digest: %w", err)
	}
	if desc.Size < 0 || desc.Size > limit {
		return nil, fmt.Errorf("descriptor size %d exceeds %d", desc.Size, limit)
	}
	f, err := openVerifiedBlob(l.blobs, desc, limit)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func openVerifiedBlob(blobs map[digest.Digest]string, desc v1.Descriptor, limit int64) (*os.File, error) {
	if desc.Data != nil {
		return nil, errors.New("embedded OCI descriptor data is unsupported")
	}
	if err := desc.Digest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid descriptor digest: %w", err)
	}
	if desc.Size < 0 || desc.Size > limit {
		return nil, fmt.Errorf("descriptor size %d exceeds %d", desc.Size, limit)
	}
	blobPath, ok := blobs[desc.Digest]
	if !ok {
		return nil, fmt.Errorf("descriptor blob %s is missing", desc.Digest)
	}
	f, err := os.Open(blobPath)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if stat.Size() != desc.Size {
		f.Close()
		return nil, fmt.Errorf("descriptor size mismatch: expected %d, got %d", desc.Size, stat.Size())
	}
	verifier := desc.Digest.Verifier()
	if _, err := io.Copy(verifier, f); err != nil {
		f.Close()
		return nil, err
	}
	if !verifier.Verified() {
		f.Close()
		return nil, errors.New("descriptor digest mismatch")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func cleanArchiveName(name string) (string, error) {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
		return "", errors.New("path is not relative slash-separated")
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", errors.New("path contains traversal")
		}
	}
	cleaned := path.Clean(strings.TrimPrefix(name, "./"))
	if cleaned == "." || cleaned == "" || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("path is empty or escapes root")
	}
	return strings.TrimSuffix(cleaned, "/"), nil
}

// cleanLayerHeaderName validates a POSIX tar header used only for comparisons
// while scanning a container layer. A literal backslash is an ordinary POSIX
// filename byte; unlike cleanArchiveName, this name is never converted to a
// host path or used as a write destination.
func cleanLayerHeaderName(name string, directory bool) (string, error) {
	if name == "" || strings.HasPrefix(name, "/") || strings.ContainsRune(name, '\x00') {
		return "", errors.New("path is not a relative POSIX name")
	}
	if directory && strings.HasSuffix(name, "/") && !strings.HasSuffix(name, "//") {
		name = strings.TrimSuffix(name, "/")
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("path contains a non-canonical or unsafe slash component")
		}
	}
	if path.Clean(name) != name {
		return "", errors.New("path is not canonical")
	}
	return name, nil
}

func copyBounded(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	max, err := limitPlusOne(limit)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(dst, io.LimitReader(src, max))
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, fmt.Errorf("content exceeds %d bytes", limit)
	}
	return n, nil
}

func readFileBounded(name string, limit int64) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var buf strings.Builder
	_, err = copyBounded(&buf, f, limit)
	if err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

func validateLayoutHeader(name string) error {
	body, err := readFileBounded(name, maxOCIControlJSONBytes)
	if err != nil {
		return fmt.Errorf("OCI layout header: %w", err)
	}
	var header struct {
		ImageLayoutVersion string `json:"imageLayoutVersion"`
	}
	if err := json.Unmarshal(body, &header); err != nil {
		return fmt.Errorf("parse OCI layout header: %w", err)
	}
	if header.ImageLayoutVersion != "1.0.0" {
		return fmt.Errorf("unsupported OCI layout version %q", header.ImageLayoutVersion)
	}
	return nil
}

func validateIndex(index v1.Index) error {
	if index.SchemaVersion != 2 {
		return fmt.Errorf("unsupported schema version %d", index.SchemaVersion)
	}
	if index.MediaType != "" && index.MediaType != v1.MediaTypeImageIndex && index.MediaType != dockerMediaTypeManifestList {
		return fmt.Errorf("unsupported media type %q", index.MediaType)
	}
	return nil
}

func isGzipLayerMediaType(mediaType string) bool {
	return mediaType == v1.MediaTypeImageLayerGzip || mediaType == dockerMediaTypeLayerGzip
}
