package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanonicalTreeDigestIsStableAndValidatesJSON(t *testing.T) {
	t.Parallel()

	a := map[string][]byte{
		"api/fields/Z.json": []byte("{\"b\":2,\"a\":1}"),
		"api/fields/A.json": []byte("{\r\n  \"n\": 1.0\r\n}"),
	}
	b := map[string][]byte{
		"api/fields/A.json": []byte("{\"n\":1.0}"),
		"api/fields/Z.json": []byte("{ \"a\" : 1, \"b\" : 2 }"),
	}
	digestA, err := CanonicalTreeDigest(a)
	if err != nil {
		t.Fatal(err)
	}
	digestB, err := CanonicalTreeDigest(b)
	if err != nil {
		t.Fatal(err)
	}
	if digestA != digestB {
		t.Fatalf("canonical digest changed with ordering/formatting: %s != %s", digestA, digestB)
	}
	if len(digestA) != 64 {
		t.Fatalf("digest length = %d, want 64", len(digestA))
	}

	if _, err := CanonicalTreeDigest(map[string][]byte{"x.json": []byte("{} trailing")}); err == nil {
		t.Fatal("expected trailing JSON to be rejected")
	}
}

func TestCanonicalTreeDigestFramesPathsAndNormalizesText(t *testing.T) {
	t.Parallel()

	one, err := CanonicalTreeDigest(map[string][]byte{"a": []byte("bc\r\nd")})
	if err != nil {
		t.Fatal(err)
	}
	two, err := CanonicalTreeDigest(map[string][]byte{"a": []byte("bc\nd")})
	if err != nil {
		t.Fatal(err)
	}
	if one != two {
		t.Fatalf("line ending normalization failed: %s != %s", one, two)
	}

	left, _ := CanonicalTreeDigest(map[string][]byte{"a": []byte("bc"), "d": nil})
	right, _ := CanonicalTreeDigest(map[string][]byte{"ab": []byte("c"), "d": nil})
	if left == right {
		t.Fatal("length-framed trees must not admit path/content concatenation collisions")
	}
}

func TestWriteSchemaSourceIsStableAtomicAndMinimal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "schema-source.json")
	created := time.Date(2026, 7, 17, 12, 34, 56, 987654321, time.FixedZone("offset", -7*60*60))
	source := SchemaSource{
		OSVersion: "5.1.21", NetworkVersion: "10.4.57", FirmwareID: "firmware-id",
		InstallerURL: "https://dl.ui.com/installer", InstallerSHA256: strings.Repeat("a", 64),
		SchemaDigest: strings.Repeat("b", 64), SensitivityDigest: strings.Repeat("c", 64),
		NoticeDigest: strings.Repeat("d", 64), GeneratedTreeDigest: strings.Repeat("e", 64),
		PolicyVersion: "policy-v1", InstallerSize: 123, Created: created,
	}
	if err := WriteSchemaSource(path, source); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteSchemaSource(path, source); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Fatal("schema source output is not byte-stable")
	}
	if strings.Contains(string(first), "generated_at") || strings.Contains(string(first), "local_path") || strings.Contains(string(first), "installer_md5") {
		t.Fatalf("committed source leaked forbidden fields: %s", first)
	}
	if !strings.Contains(string(first), `"created":"2026-07-17T19:34:56Z"`) || !strings.Contains(string(first), `"updated":null`) {
		t.Fatalf("timestamps were not normalized/null: %s", first)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o, want 644", info.Mode().Perm())
	}

	var decoded map[string]any
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["generated_tree_digest"]; !ok {
		t.Fatal("snake_case generated_tree_digest is missing")
	}
}

func TestWriteSchemaSourcePreservesExistingFileAfterInjectedRenameFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "schema-source.json")
	mustWriteTestFile(t, path, []byte("old"))
	ops := defaultSchemaSourceFileOps()
	ops.rename = func(string, string) error { return errors.New("injected rename failure") }
	if err := writeSchemaSource(path, SchemaSource{OSVersion: "new"}, ops); err == nil || !strings.Contains(err.Error(), "injected rename failure") {
		t.Fatalf("expected injected rename failure, got %v", err)
	}
	assertFileBody(t, path, []byte("old"))
	matches, err := filepath.Glob(filepath.Join(dir, ".schema-source.json.tmp-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary source files leaked: %v, %v", matches, err)
	}
}

func TestCanonicalJSONDoesNotEscapeHTMLOrAppendNewline(t *testing.T) {
	t.Parallel()

	got, err := canonicalJSON([]byte(`{"value":"<tag>&","number":1.00}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"number":1.00,"value":"<tag>&"}` {
		t.Fatalf("canonical JSON = %q", got)
	}
}

func TestCanonicalGeneratedTreeDigestSelectsOnlyOwnedOutputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteTestFile(t, filepath.Join(root, "unifi", "device.generated.go"), []byte("package unifi\r\n"))
	mustWriteTestFile(t, filepath.Join(root, "unifi", "version.generated.go"), []byte("package unifi\n"))
	mustWriteTestFile(t, filepath.Join(root, "unifi", "settings", "guest.generated.go"), []byte("package settings\n"))
	mustWriteTestFile(t, filepath.Join(root, "unifi", "handwritten.go"), []byte("ignored one"))
	mustWriteTestFile(t, filepath.Join(root, "unifi", "device_test.go"), []byte("ignored two"))
	mustWriteTestFile(t, filepath.Join(root, "specification.json"), []byte(`{"b":2,"a":1}`))

	first, err := GeneratedTreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteTestFile(t, filepath.Join(root, "unifi", "handwritten.go"), []byte("changed"))
	second, err := GeneratedTreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("hand-maintained Go changed generated-tree digest")
	}
	mustWriteTestFile(t, filepath.Join(root, "unifi", "settings", "guest.generated.go"), []byte("changed generated"))
	third, err := GeneratedTreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if third == second {
		t.Fatal("nested generated Go did not change generated-tree digest")
	}
}

func TestCanonicalGeneratedTreeDigestRejectsSymlinks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteTestFile(t, filepath.Join(root, "specification.json"), []byte(`{}`))
	mustWriteTestFile(t, filepath.Join(root, "target"), []byte("package unifi\n"))
	if err := os.MkdirAll(filepath.Join(root, "unifi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "target"), filepath.Join(root, "unifi", "bad.generated.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := GeneratedTreeDigest(root); err == nil {
		t.Fatal("expected generated symlink to be rejected")
	}
}

func mustWriteTestFile(t *testing.T, name string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, body, 0o644); err != nil {
		t.Fatal(err)
	}
}
