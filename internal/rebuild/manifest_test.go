package rebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, name, content string) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func completeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{
		"specification.json":            "spec\n",
		"schemas/capture.lock.json":     "lock\n",
		"schemas/VERSION":               "10.4.57\n",
		"schemas/SOURCE":                "source\n",
		"schemas/ARTIFACT":              "artifact\n",
		"unifi/a.generated.go":          "package unifi\n",
		"unifi/settings/b.generated.go": "package settings\n",
		"unifi/hand_written.go":         "ignored\n",
	} {
		writeFixture(t, root, name, content)
	}
	return root
}

func TestBuildManifestIsCanonicalAndIgnoresHandWrittenFiles(t *testing.T) {
	root := completeFixture(t)

	first, err := BuildManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Compare(first, second); err != nil {
		t.Fatalf("unchanged manifests differ: %v", err)
	}
	if len(first.Files) != 7 {
		t.Fatalf("manifest has %d files, want 7: %#v", len(first.Files), first.Files)
	}
	for _, file := range first.Files {
		if file.Path == "unifi/hand_written.go" {
			t.Fatal("manifest included hand-written Go")
		}
	}
}

func TestCompareRejectsIntentionalGeneratorNondeterminism(t *testing.T) {
	root := completeFixture(t)
	first, err := BuildManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "unifi/a.generated.go", "package unifi\n// nondeterministic value\n")
	second, err := BuildManifest(root)
	if err != nil {
		t.Fatal(err)
	}

	err = Compare(first, second)
	if err == nil || !strings.Contains(err.Error(), "nondeterministic generated output") || !strings.Contains(err.Error(), "unifi/a.generated.go") {
		t.Fatalf("Compare() error = %v, want named nondeterminism failure", err)
	}
}

func TestBuildManifestFailsWhenRequiredProvenanceIsMissing(t *testing.T) {
	root := completeFixture(t)
	if err := os.Remove(filepath.Join(root, "schemas", "SOURCE")); err != nil {
		t.Fatal(err)
	}

	_, err := BuildManifest(root)
	if err == nil || !strings.Contains(err.Error(), "schemas/SOURCE") {
		t.Fatalf("BuildManifest() error = %v, want missing provenance failure", err)
	}
}
