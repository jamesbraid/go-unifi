package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func SnapshotNoticeDigest(snapshot string) (LocalManifest, string, error) {
	body, err := os.ReadFile(filepath.Join(snapshot, "metadata", "source.json"))
	if err != nil {
		return LocalManifest{}, "", fmt.Errorf("read local snapshot manifest: %w", err)
	}
	var manifest LocalManifest
	if err := decodeStrictJSON(body, &manifest); err != nil {
		return LocalManifest{}, "", fmt.Errorf("parse local snapshot manifest: %w", err)
	}
	if !validSHA256(manifest.NoticeDigest) {
		return LocalManifest{}, "", fmt.Errorf("invalid local snapshot notice digest %q", manifest.NoticeDigest)
	}

	root := filepath.Join(snapshot, "metadata", "notices")
	files := make(map[string][]byte)
	foldedPaths := make(map[string]string)
	var total int64
	limits := DefaultArchiveLimits()
	err = filepath.WalkDir(root, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if name == root && errors.Is(walkErr, os.ErrNotExist) {
				return filepath.SkipDir
			}
			return walkErr
		}
		if name == root {
			if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
				return errors.New("snapshot notices root is not a regular directory")
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("snapshot notice path is a symlink: %s", name)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("snapshot notice path is not regular: %s", name)
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if err := validateDigestPath(rel); err != nil {
			return fmt.Errorf("invalid snapshot notice path %q: %w", rel, err)
		}
		if err := addSnapshotNoticePath(foldedPaths, rel); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if len(files) >= limits.MaxNoticeEntries {
			return fmt.Errorf("snapshot notice tree exceeds %d entries", limits.MaxNoticeEntries)
		}
		limit := limits.MaxNoticeBytes
		if total >= limit || info.Size() > limit-total {
			return fmt.Errorf("snapshot notice tree exceeds %d bytes", limit)
		}
		body, err := readFileBounded(name, limit-total)
		if err != nil {
			return err
		}
		total += int64(len(body))
		files[rel] = body
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return LocalManifest{}, "", fmt.Errorf("read snapshot notices: %w", err)
	}
	digest, err := CanonicalTreeDigest(files)
	if err != nil {
		return LocalManifest{}, "", fmt.Errorf("digest snapshot notices: %w", err)
	}
	return manifest, digest, nil
}

func addSnapshotNoticePath(paths map[string]string, name string) error {
	folded := strings.ToLower(name)
	if previous, exists := paths[folded]; exists {
		return fmt.Errorf("snapshot notice case-fold collision: %q and %q", previous, name)
	}
	paths[folded] = name
	return nil
}

func ValidateSnapshotNoticeProvenance(snapshot, expectedNetworkVersion, committedNoticeDigest string, policy SensitivityPolicy) (string, error) {
	manifest, actualDigest, err := SnapshotNoticeDigest(snapshot)
	if err != nil {
		return "", err
	}
	if manifest.NetworkVersion != expectedNetworkVersion {
		return "", fmt.Errorf("local snapshot Network version %q does not match committed version %q", manifest.NetworkVersion, expectedNetworkVersion)
	}
	if actualDigest != manifest.NoticeDigest {
		return "", fmt.Errorf("local snapshot notice digest differs from actual tree: manifest %s, actual %s", manifest.NoticeDigest, actualDigest)
	}
	if err := RequireApprovedNoticeDigest(policy, actualDigest); err != nil {
		return "", err
	}
	if actualDigest != committedNoticeDigest {
		return "", fmt.Errorf("local snapshot notice digest %s differs from committed schema-source digest %s", actualDigest, committedNoticeDigest)
	}
	return actualDigest, nil
}
