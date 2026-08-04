package capturelock

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validLock(data []byte) Lock {
	digest := sha256.Sum256(data)
	hexDigest := hex.EncodeToString(digest[:])

	return Lock{
		FormatVersion: FormatVersion,
		Controller: Controller{
			Product:        "unifi-controller",
			Build:          "v10.4.57+atag-10.4.57-34628 build-record",
			NetworkVersion: "10.4.57",
		},
		Source: Source{
			Location:            "https://downloads.example.invalid/unifi.deb",
			MediaType:           "application/vnd.debian.binary-package",
			ByteSize:            int64(len(data)),
			SHA256:              hexDigest,
			ContentStoreLocator: filepath.ToSlash(filepath.Join("sha256", hexDigest)),
		},
		Inputs: Inputs{
			ExtractionRulesSHA256: strings.Repeat("a", 64),
			GeneratorInputsSHA256: strings.Repeat("b", 64),
		},
		Snapshots: Snapshots{
			StructuralSHA256:  strings.Repeat("c", 64),
			SensitivitySHA256: strings.Repeat("d", 64),
		},
		CapturedAt: "2026-08-03T12:34:56Z",
	}
}

func TestLoadFileRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.lock.json")
	err := os.WriteFile(path, []byte(`{
		"format_version": 1,
		"unexpected": true
	}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("LoadFile() error = %v, want unknown-field rejection", err)
	}
}

func TestDraftLockAllowsOnlyInspectionOutputsToBeMissing(t *testing.T) {
	lock := validLock([]byte("controller artifact"))
	lock.Controller.NetworkVersion = ""
	lock.Snapshots = Snapshots{}
	path := filepath.Join(t.TempDir(), "capture.draft.json")

	if err := WriteDraftFile(path, lock); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDraftFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != lock {
		t.Fatalf("LoadDraftFile() = %#v, want %#v", got, lock)
	}
	if _, err := LoadFile(path); err == nil || !strings.Contains(err.Error(), "controller.network_version") {
		t.Fatalf("LoadFile() error = %v, want incomplete-lock rejection", err)
	}

	lock.Source.Location = ""
	if err := WriteDraftFile(path, lock); err == nil || !strings.Contains(err.Error(), "source.location") {
		t.Fatalf("WriteDraftFile() error = %v, want source validation", err)
	}
}

func TestValidateRequiresCompleteLock(t *testing.T) {
	data := []byte("controller artifact")

	tests := []struct {
		name string
		edit func(*Lock)
		want string
	}{
		{name: "format", edit: func(l *Lock) { l.FormatVersion = 2 }, want: "format_version"},
		{name: "product", edit: func(l *Lock) { l.Controller.Product = "" }, want: "controller.product"},
		{name: "build", edit: func(l *Lock) { l.Controller.Build = "" }, want: "controller.build"},
		{name: "network", edit: func(l *Lock) { l.Controller.NetworkVersion = "" }, want: "controller.network_version"},
		{name: "location", edit: func(l *Lock) { l.Source.Location = "" }, want: "source.location"},
		{name: "media type", edit: func(l *Lock) { l.Source.MediaType = "" }, want: "source.media_type"},
		{name: "size", edit: func(l *Lock) { l.Source.ByteSize = 0 }, want: "source.byte_size"},
		{name: "source digest", edit: func(l *Lock) { l.Source.SHA256 = "ABC" }, want: "source.sha256"},
		{name: "extraction digest", edit: func(l *Lock) { l.Inputs.ExtractionRulesSHA256 = "" }, want: "inputs.extraction_rules_sha256"},
		{name: "generator digest", edit: func(l *Lock) { l.Inputs.GeneratorInputsSHA256 = "" }, want: "inputs.generator_inputs_sha256"},
		{name: "structural digest", edit: func(l *Lock) { l.Snapshots.StructuralSHA256 = "" }, want: "snapshots.structural_sha256"},
		{name: "sensitivity digest", edit: func(l *Lock) { l.Snapshots.SensitivitySHA256 = "" }, want: "snapshots.sensitivity_sha256"},
		{name: "capture time", edit: func(l *Lock) { l.CapturedAt = "yesterday" }, want: "captured_at"},
		{name: "absolute locator", edit: func(l *Lock) { l.Source.ContentStoreLocator = "/tmp/artifact" }, want: "content_store_locator"},
		{name: "traversal locator", edit: func(l *Lock) { l.Source.ContentStoreLocator = "../artifact" }, want: "content_store_locator"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lock := validLock(data)
			test.edit(&lock)
			err := lock.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestResolveArtifactVerifiesContentAddressedBytes(t *testing.T) {
	data := []byte("controller artifact")
	lock := validLock(data)
	store := t.TempDir()
	artifact := filepath.Join(store, filepath.FromSlash(lock.Source.ContentStoreLocator))
	if err := os.MkdirAll(filepath.Dir(artifact), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveArtifact(store, lock)
	if err != nil {
		t.Fatal(err)
	}
	if got != artifact {
		t.Fatalf("ResolveArtifact() = %q, want %q", got, artifact)
	}

	if err := os.WriteFile(artifact, bytes.Repeat([]byte("x"), len(data)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ResolveArtifact(store, lock)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("ResolveArtifact() error = %v, want digest mismatch", err)
	}

	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	_, err = ResolveArtifact(store, lock)
	if err == nil || !strings.Contains(err.Error(), "retained artifact") {
		t.Fatalf("ResolveArtifact() error = %v, want missing artifact", err)
	}
}

func TestResolveArtifactAcceptsValidatedCaptureDraft(t *testing.T) {
	data := []byte("controller artifact")
	lock := validLock(data)
	lock.Controller.NetworkVersion = ""
	lock.Snapshots = Snapshots{}
	store := t.TempDir()
	artifact := filepath.Join(store, filepath.FromSlash(lock.Source.ContentStoreLocator))
	if err := os.MkdirAll(filepath.Dir(artifact), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveArtifact(store, lock)
	if err != nil {
		t.Fatal(err)
	}
	if got != artifact {
		t.Fatalf("ResolveArtifact() = %q, want %q", got, artifact)
	}
}

func TestWriteFileIsCanonicalAndRoundTrips(t *testing.T) {
	data := []byte("controller artifact")
	lock := validLock(data)
	path := filepath.Join(t.TempDir(), "capture.lock.json")

	if err := WriteFile(path, lock); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || first[len(first)-1] != '\n' {
		t.Fatalf("lock is not newline terminated: %q", first)
	}
	if err := WriteFile(path, lock); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("identical locks did not encode byte-identically")
	}

	got, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != lock {
		t.Fatalf("LoadFile() = %#v, want %#v", got, lock)
	}
}

func TestInspectionFileIsStrictAndCanonical(t *testing.T) {
	inspection := Inspection{
		NetworkVersion: "10.4.57",
		Snapshots: Snapshots{
			StructuralSHA256:  strings.Repeat("c", 64),
			SensitivitySHA256: strings.Repeat("d", 64),
		},
	}
	path := filepath.Join(t.TempDir(), "inspection.json")

	if err := WriteInspectionFile(path, inspection); err != nil {
		t.Fatal(err)
	}
	got, err := LoadInspectionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != inspection {
		t.Fatalf("LoadInspectionFile() = %#v, want %#v", got, inspection)
	}

	if err := os.WriteFile(path, []byte(`{"network_version":"10.4.57","snapshots":{},"extra":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadInspectionFile(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("LoadInspectionFile() error = %v, want strict decoding", err)
	}
}

func TestWriteCompatibilityProjectionsUsesOnlyLock(t *testing.T) {
	lock := validLock([]byte("controller artifact"))
	dir := t.TempDir()

	if err := WriteCompatibilityProjections(dir, lock); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"VERSION":  "10.4.57\n",
		"SOURCE":   "unifi-controller v10.4.57+atag-10.4.57-34628 build-record\n",
		"ARTIFACT": "https://downloads.example.invalid/unifi.deb\n",
	}
	for name, expected := range want {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != expected {
			t.Errorf("%s = %q, want %q", name, got, expected)
		}
	}
}

func TestDigestTreeIsStableAndDetectsContentChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.json"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "a.json"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := DigestTree(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := DigestTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("DigestTree() changed without input change: %q != %q", first, second)
	}

	if err := os.WriteFile(filepath.Join(root, "nested", "a.json"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := DigestTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("DigestTree() ignored a content change")
	}
}

func TestStoreArtifactUsesDigestPathAndRejectsCorruptExistingContent(t *testing.T) {
	data := []byte("controller artifact")
	source := filepath.Join(t.TempDir(), "controller.deb")
	if err := os.WriteFile(source, data, 0o644); err != nil {
		t.Fatal(err)
	}
	store := t.TempDir()

	stored, err := StoreArtifact(source, store)
	if err != nil {
		t.Fatal(err)
	}
	wantDigest := sha256.Sum256(data)
	wantHex := hex.EncodeToString(wantDigest[:])
	if stored.SHA256 != wantHex || stored.ByteSize != int64(len(data)) {
		t.Fatalf("StoreArtifact() = %#v", stored)
	}
	if stored.Locator != "sha256/"+wantHex {
		t.Fatalf("locator = %q, want sha256/<digest>", stored.Locator)
	}
	info, err := os.Stat(stored.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("stored artifact mode = %o, want no group/other access", info.Mode().Perm())
	}

	again, err := StoreArtifact(source, store)
	if err != nil {
		t.Fatal(err)
	}
	if again != stored {
		t.Fatalf("idempotent StoreArtifact() = %#v, want %#v", again, stored)
	}

	if err := os.WriteFile(stored.Path, bytes.Repeat([]byte("x"), len(data)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = StoreArtifact(source, store)
	if err == nil || !strings.Contains(err.Error(), "existing content-store object") {
		t.Fatalf("StoreArtifact() error = %v, want corrupt-object rejection", err)
	}
}

func TestComputeInputDigestsSeparatesExtractionFromGeneration(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"cmd/fields/extract.go":            "extract-v1",
		"cmd/fields/main.go":               "generate-v1",
		"cmd/fields/main_test.go":          "ignored-test-v1",
		"cmd/fields/api.go.tmpl":           "template-v1",
		"internal/fields/fields.go":        "fields-v1",
		"internal/capturelock/lock.go":     "lock-v1",
		"overrides/resources/Network.json": "override-v1",
		"overrides/fields.toml":            "fields-override-v1",
		"go.mod":                           "module example.invalid/test",
		"go.sum":                           "sum-v1",
	}
	for name, content := range files {
		filename := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	first, err := ComputeInputDigests(root)
	if err != nil {
		t.Fatal(err)
	}
	if !validSHA256(first.ExtractionRulesSHA256) || !validSHA256(first.GeneratorInputsSHA256) {
		t.Fatalf("ComputeInputDigests() = %#v", first)
	}

	if err := os.WriteFile(filepath.Join(root, "cmd/fields/main_test.go"), []byte("ignored-test-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterTest, err := ComputeInputDigests(root)
	if err != nil {
		t.Fatal(err)
	}
	if afterTest != first {
		t.Fatalf("test-only edit changed input digests: %#v != %#v", afterTest, first)
	}

	if err := os.WriteFile(filepath.Join(root, "cmd/fields/main.go"), []byte("generate-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterGenerator, err := ComputeInputDigests(root)
	if err != nil {
		t.Fatal(err)
	}
	if afterGenerator.ExtractionRulesSHA256 != first.ExtractionRulesSHA256 {
		t.Fatal("generator-only edit changed extraction-rules digest")
	}
	if afterGenerator.GeneratorInputsSHA256 == first.GeneratorInputsSHA256 {
		t.Fatal("generator-only edit did not change generator-input digest")
	}

	if err := os.WriteFile(filepath.Join(root, "cmd/fields/extract.go"), []byte("extract-v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterExtraction, err := ComputeInputDigests(root)
	if err != nil {
		t.Fatal(err)
	}
	if afterExtraction.ExtractionRulesSHA256 == afterGenerator.ExtractionRulesSHA256 {
		t.Fatal("extractor edit did not change extraction-rules digest")
	}
	if afterExtraction.GeneratorInputsSHA256 == afterGenerator.GeneratorInputsSHA256 {
		t.Fatal("extractor edit did not change generator-input digest")
	}
}
