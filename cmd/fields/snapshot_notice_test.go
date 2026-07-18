package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSnapshotNoticeProvenanceBindsActualTree(t *testing.T) {
	snapshot := t.TempDir()
	notices := filepath.Join(snapshot, "metadata", "notices")
	require.NoError(t, os.MkdirAll(filepath.Join(notices, "ace.jar"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(notices, "ace.jar", "LICENSE"), []byte("new terms"), 0o644))
	localDigest, err := CanonicalTreeDigest(map[string][]byte{"ace.jar/LICENSE": []byte("new terms")})
	require.NoError(t, err)
	writeSnapshotManifest(t, snapshot, LocalManifest{NetworkVersion: "10.4.57", NoticeDigest: localDigest})
	oldDigest := strings.Repeat("a", 64)
	policy := SensitivityPolicy{ApprovedNoticeSHA256: []string{oldDigest}}

	_, err = ValidateSnapshotNoticeProvenance(snapshot, "10.4.57", oldDigest, policy)
	require.ErrorContains(t, err, localDigest)
	require.ErrorContains(t, err, "not approved")
}

func TestValidateSnapshotNoticeProvenanceAllowsCanonicalEmptyMissingTree(t *testing.T) {
	snapshot := t.TempDir()
	emptyDigest, err := CanonicalTreeDigest(map[string][]byte{})
	require.NoError(t, err)
	writeSnapshotManifest(t, snapshot, LocalManifest{NetworkVersion: "10.4.57", NoticeDigest: emptyDigest})
	policy := SensitivityPolicy{ApprovedNoticeSHA256: []string{emptyDigest}}
	got, err := ValidateSnapshotNoticeProvenance(snapshot, "10.4.57", emptyDigest, policy)
	require.NoError(t, err)
	require.Equal(t, emptyDigest, got)
}

func TestSnapshotNoticeDigestRejectsUnsafeTreeAndStrictManifest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{"symlink", func(t *testing.T, root string) {
			target := filepath.Join(t.TempDir(), "target")
			require.NoError(t, os.WriteFile(target, []byte("terms"), 0o644))
			require.NoError(t, os.Symlink(target, filepath.Join(root, "LICENSE")))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := t.TempDir()
			notices := filepath.Join(snapshot, "metadata", "notices")
			require.NoError(t, os.MkdirAll(notices, 0o755))
			tc.setup(t, notices)
			writeSnapshotManifest(t, snapshot, LocalManifest{NetworkVersion: "10.4.57", NoticeDigest: strings.Repeat("a", 64)})
			_, _, err := SnapshotNoticeDigest(snapshot)
			require.Error(t, err)
		})
	}

	snapshot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(snapshot, "metadata"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(snapshot, "metadata", "source.json"), []byte(`{"network_version":"10.4.57","notice_digest":"`+strings.Repeat("a", 64)+`","unknown":true}`), 0o644))
	_, _, err := SnapshotNoticeDigest(snapshot)
	require.ErrorContains(t, err, "unknown")
}

func TestAddSnapshotNoticePathRejectsCaseFoldCollision(t *testing.T) {
	paths := map[string]string{}
	require.NoError(t, addSnapshotNoticePath(paths, "Ace.jar/LICENSE"))
	require.ErrorContains(t, addSnapshotNoticePath(paths, "ace.jar/license"), "case-fold collision")
}

func writeSnapshotManifest(t *testing.T, snapshot string, manifest LocalManifest) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(snapshot, "metadata"), 0o755))
	body, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(snapshot, "metadata", "source.json"), body, 0o644))
}
