package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestBuildSnapshotFlattensSplitsOverlaysAndRecordsProvenance(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	custom := filepath.Join(root, "custom")
	mustWriteTestFile(t, filepath.Join(custom, "Device.json"), []byte(`{"custom":true}`))
	setting := []byte(`{"auto_speedtest":{"enabled":true},"nested_setting":{"count":1.0}}`)
	device := []byte("{\r\n \"upstream\": true\r\n}")
	sensitive := []byte(`{"z":2,"a":1}`)
	defs := snapshotDefinitions(t, root, setting, device, sensitive)
	defs.MissingOptional = []string{"z.json", "a.json", "z.json"}

	src := InstallerSource{
		OSVersion: "5.1.21", FirmwareID: "fw", Product: "unifi-os-server", Platform: "linux-x64", Channel: "release",
		ExpectedMD5: strings.Repeat("f", 32), ExpectedSize: 999, ExpectedSHA256: strings.Repeat("e", 64),
		Created: time.Date(2026, 7, 17, 12, 0, 0, 123, time.FixedZone("offset", 2*60*60)),
	}
	installer := &MaterializedInstaller{Path: filepath.Join(root, "machine-local-installer"), Size: 321, SHA256: strings.Repeat("1", 64)}
	manifest, err := BuildSnapshot(context.Background(), SnapshotOptions{
		Root: root, CustomDir: custom, Source: src, Installer: installer, Definitions: defs, PolicyVersion: "policy-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	final := filepath.Join(root, "v10.4.57")
	assertFileBody(t, filepath.Join(final, "Setting.json"), setting)
	assertFileBody(t, filepath.Join(final, "SettingAutoSpeedtest.json"), []byte("{\n  \"enabled\": true\n}"))
	assertFileBody(t, filepath.Join(final, "SettingNestedSetting.json"), []byte("{\n  \"count\": 1\n}"))
	assertFileBody(t, filepath.Join(final, "Device.json"), []byte(`{"custom":true}`))
	assertFileBody(t, filepath.Join(final, "metadata", "raw-fields", "api", "fields", "nested", "Device.json"), device)
	assertFileBody(t, filepath.Join(final, "metadata", "sensitive_metadata.json"), sensitive)
	assertFileBody(t, filepath.Join(final, "metadata", "notices", "ace.jar", "META-INF", "NOTICE.txt"), []byte("notice\r\ntext"))

	if manifest.InstallerSize != 321 || manifest.InstallerSHA256 != strings.Repeat("1", 64) {
		t.Fatalf("materialized installer facts did not win: %#v", manifest)
	}
	if manifest.InstallerMD5 != strings.Repeat("f", 32) {
		t.Fatalf("installer MD5 missing: %#v", manifest)
	}
	if got, want := manifest.MissingOptional, []string{"a.json", "z.json"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("missing optional = %v, want %v", got, want)
	}
	if manifest.Artifacts["api/fields/nested/Device.json"] != rawSHA(device) {
		t.Fatalf("raw Task 2 artifact hash was not retained: %#v", manifest.Artifacts)
	}
	if manifest.SchemaDigest == "" || manifest.SensitivityDigest == "" || manifest.NoticeDigest == "" {
		t.Fatalf("canonical digests missing: %#v", manifest)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(final, "metadata", "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifestBytes), root) || strings.Contains(string(manifestBytes), "generated_at") {
		t.Fatalf("manifest leaked local path/time: %s", manifestBytes)
	}
	if !strings.Contains(string(manifestBytes), `"missing_optional":["a.json","z.json"]`) {
		t.Fatalf("missing optionals not deterministic/non-null: %s", manifestBytes)
	}
	if !strings.Contains(string(manifestBytes), `"created":"2026-07-17T10:00:00Z"`) || !strings.Contains(string(manifestBytes), `"updated":null`) {
		t.Fatalf("release timestamps wrong: %s", manifestBytes)
	}
	assertTreeModes(t, final)
}

func TestBuildSnapshotDigestsIgnoreInputMapOrderAndOverlay(t *testing.T) {
	t.Parallel()

	build := func(name string, reverse bool, customBody []byte) LocalManifest {
		root := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		custom := filepath.Join(root, "custom")
		mustWriteTestFile(t, filepath.Join(custom, "Device.json"), customBody)
		setting := []byte(`{"a":{"x":1}}`)
		device := []byte(`{"b":2,"a":1}`)
		defs := snapshotDefinitions(t, root, setting, device, []byte(`{"secret":1}`))
		if reverse {
			defs.Fields = map[string]ExtractedArtifact{
				"api/fields/nested/Device.json": defs.Fields["api/fields/nested/Device.json"],
				"api/fields/Setting.json":       defs.Fields["api/fields/Setting.json"],
			}
		}
		manifest, err := BuildSnapshot(context.Background(), SnapshotOptions{Root: root, CustomDir: custom, Installer: &MaterializedInstaller{Size: 1, SHA256: strings.Repeat("a", 64)}, Definitions: defs})
		if err != nil {
			t.Fatal(err)
		}
		return *manifest
	}
	a := build("one", false, []byte(`{"overlay":1}`))
	b := build("two", true, []byte(`{"different_overlay":2}`))
	if a.SchemaDigest != b.SchemaDigest || a.SensitivityDigest != b.SensitivityDigest || a.NoticeDigest != b.NoticeDigest {
		t.Fatalf("pre-overlay canonical digests differ:\n%#v\n%#v", a, b)
	}
}

func TestBuildSnapshotRejectsFlattenAndSplitCollisions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields map[string][]byte
	}{
		{"exact basenames", map[string][]byte{"api/fields/a/Device.json": []byte(`{}`), "api/fields/b/Device.json": []byte(`{}`)}},
		{"case folded basenames", map[string][]byte{"api/fields/a/Device.json": []byte(`{}`), "api/fields/b/device.json": []byte(`{}`)}},
		{"split collision", map[string][]byte{"api/fields/Setting.json": []byte(`{"auto_speedtest":{}}`), "api/fields/SettingAutoSpeedtest.json": []byte(`{}`)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			defs := definitionsFromBodies(t, root, tc.fields, map[string][]byte{"sensitive_metadata.json": []byte(`{}`)}, nil)
			_, err := BuildSnapshot(context.Background(), SnapshotOptions{Root: root, CustomDir: emptyCustom(t, root), Installer: &MaterializedInstaller{}, Definitions: defs})
			if err == nil {
				t.Fatal("expected collision to fail")
			}
		})
	}
}

func TestBuildSnapshotRejectsUnsafeMetadataNames(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	defs := definitionsFromBodies(t, root,
		map[string][]byte{"api/fields/Setting.json": []byte(`{"a":{}}`), "api/fields/Device.json": []byte(`{}`)},
		map[string][]byte{"nested\\sensitive_metadata.json": []byte(`{}`)}, nil)
	if _, err := BuildSnapshot(context.Background(), SnapshotOptions{Root: root, CustomDir: emptyCustom(t, root), Installer: &MaterializedInstaller{}, Definitions: defs}); err == nil {
		t.Fatal("expected backslash metadata path to fail")
	}
}

func TestBuildSnapshotRejectsUnsafeCustomOverlays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, file string
		body       []byte
	}{
		{"setting", "Setting.json", []byte(`{}`)},
		{"non json", "notes.txt", []byte("no")},
		{"split collision", "SettingAutoSpeedtest.json", []byte(`{}`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			custom := filepath.Join(root, "custom")
			mustWriteTestFile(t, filepath.Join(custom, tc.file), tc.body)
			defs := snapshotDefinitions(t, root, []byte(`{"auto_speedtest":{}}`), []byte(`{}`), []byte(`{}`))
			if _, err := BuildSnapshot(context.Background(), SnapshotOptions{Root: root, CustomDir: custom, Installer: &MaterializedInstaller{}, Definitions: defs}); err == nil {
				t.Fatal("expected unsafe custom overlay to fail")
			}
		})
	}
}

func TestBuildSnapshotRejectsCaseOnlyCustomOverlay(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	custom := filepath.Join(root, "custom")
	mustWriteTestFile(t, filepath.Join(custom, "device.json"), []byte(`{}`))
	defs := snapshotDefinitions(t, root, []byte(`{"auto_speedtest":{}}`), []byte(`{}`), []byte(`{}`))
	if _, err := BuildSnapshot(context.Background(), SnapshotOptions{Root: root, CustomDir: custom, Installer: &MaterializedInstaller{}, Definitions: defs}); err == nil {
		t.Fatal("expected case-only custom overlay to fail")
	}
}

func TestBuildSnapshotAtomicallyReplacesExistingSnapshot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	final := filepath.Join(root, "v10.4.57")
	mustWriteTestFile(t, filepath.Join(final, "old.txt"), []byte("old"))
	defs := snapshotDefinitions(t, root, []byte(`{"a":{}}`), []byte(`{"new":true}`), []byte(`{}`))
	if _, err := BuildSnapshot(context.Background(), SnapshotOptions{Root: root, CustomDir: emptyCustom(t, root), Installer: &MaterializedInstaller{}, Definitions: defs}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(final, "old.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old snapshot survived replacement: %v", err)
	}
	assertFileBody(t, filepath.Join(final, "Device.json"), []byte(`{"new":true}`))
}

func TestBuildSnapshotRestoresExistingSnapshotAfterInjectedPublishFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	final := filepath.Join(root, "v10.4.57")
	mustWriteTestFile(t, filepath.Join(final, "sentinel"), []byte("old"))
	defs := snapshotDefinitions(t, root, []byte(`{"a":{}}`), []byte(`{}`), []byte(`{}`))
	ops := defaultSnapshotFileOps()
	renames := 0
	realRename := ops.rename
	ops.rename = func(old, new string) error {
		renames++
		if renames == 2 {
			return errors.New("injected stage rename failure")
		}
		return realRename(old, new)
	}
	_, err := BuildSnapshot(context.Background(), SnapshotOptions{Root: root, CustomDir: emptyCustom(t, root), Installer: &MaterializedInstaller{}, Definitions: defs, fileOps: &ops})
	if err == nil || !strings.Contains(err.Error(), "injected stage rename failure") {
		t.Fatalf("expected injected publication failure, got %v", err)
	}
	assertFileBody(t, filepath.Join(final, "sentinel"), []byte("old"))
}

func TestBuildSnapshotReportsCompoundRestoreFailureAndRetainsBackup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	final := filepath.Join(root, "v10.4.57")
	mustWriteTestFile(t, filepath.Join(final, "sentinel"), []byte("old"))
	defs := snapshotDefinitions(t, root, []byte(`{"a":{}}`), []byte(`{}`), []byte(`{}`))
	ops := defaultSnapshotFileOps()
	renames := 0
	realRename := ops.rename
	ops.rename = func(old, new string) error {
		renames++
		if renames == 2 {
			return errors.New("publish failed")
		}
		if renames == 3 {
			return errors.New("restore failed")
		}
		return realRename(old, new)
	}
	_, err := BuildSnapshot(context.Background(), SnapshotOptions{Root: root, CustomDir: emptyCustom(t, root), Installer: &MaterializedInstaller{}, Definitions: defs, fileOps: &ops})
	if err == nil || !strings.Contains(err.Error(), "publish failed") || !strings.Contains(err.Error(), "restore failed") || !strings.Contains(err.Error(), ".backup-") || !strings.Contains(err.Error(), ".staging-") {
		t.Fatalf("expected compound path-rich failure, got %v", err)
	}
	matches, globErr := filepath.Glob(filepath.Join(root, ".v10.4.57.backup-*"))
	if globErr != nil || len(matches) != 1 {
		t.Fatalf("backup not retained: matches=%v err=%v", matches, globErr)
	}
	assertFileBody(t, filepath.Join(matches[0], "sentinel"), []byte("old"))
}

func snapshotDefinitions(t *testing.T, root string, setting, device, sensitive []byte) *ExtractedDefinitions {
	t.Helper()
	return definitionsFromBodies(t, root,
		map[string][]byte{"api/fields/Setting.json": setting, "api/fields/nested/Device.json": device},
		map[string][]byte{"sensitive_metadata.json": sensitive},
		map[string][]byte{"ace.jar/META-INF/NOTICE.txt": []byte("notice\r\ntext")},
	)
}

func definitionsFromBodies(t *testing.T, root string, fields, metadata, notices map[string][]byte) *ExtractedDefinitions {
	t.Helper()
	defs := &ExtractedDefinitions{NetworkVersion: "10.4.57", Fields: map[string]ExtractedArtifact{}, Metadata: map[string]ExtractedArtifact{}, Notices: map[string]ExtractedArtifact{}}
	add := func(dst map[string]ExtractedArtifact, kind, name string, body []byte) {
		file := filepath.Join(root, "artifacts", kind, strings.ReplaceAll(name, "/", "_"))
		mustWriteTestFile(t, file, body)
		dst[name] = ExtractedArtifact{Name: name, Path: file, Size: int64(len(body)), SHA256: rawSHA(body)}
	}
	keys := func(m map[string][]byte) []string {
		result := make([]string, 0, len(m))
		for key := range m {
			result = append(result, key)
		}
		sort.Strings(result)
		return result
	}
	for _, name := range keys(fields) {
		add(defs.Fields, "fields", name, fields[name])
	}
	for _, name := range keys(metadata) {
		add(defs.Metadata, "metadata", name, metadata[name])
	}
	for _, name := range keys(notices) {
		add(defs.Notices, "notices", name, notices[name])
	}
	return defs
}

func emptyCustom(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "empty-custom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func rawSHA(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func assertFileBody(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func assertTreeModes(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		want := os.FileMode(0o644)
		if entry.IsDir() {
			want = 0o755
		}
		if info.Mode().Perm() != want {
			return errors.New(path + " has mode " + info.Mode().Perm().String())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func decodeManifest(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
