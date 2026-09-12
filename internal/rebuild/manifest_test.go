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

func digest(t *testing.T, root string) string {
	t.Helper()
	value, err := OutputDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// The digest has to answer for generated output only, wherever it sits in the
// tree, and has to ignore the hand-written client files next to it -- those
// change on every ordinary commit, and covering them would make the recorded
// digest unmaintainable.
func TestOutputDigestCoversGeneratedOutputOnly(t *testing.T) {
	root := completeFixture(t)
	before := digest(t, root)

	writeFixture(t, root, "unifi/hand_written.go", "edited by hand\n")
	if after := digest(t, root); after != before {
		t.Fatalf("hand-written edit moved the digest: %s then %s", before, after)
	}

	writeFixture(t, root, "unifi/settings/b.generated.go", "package settings\n// drift\n")
	if after := digest(t, root); after == before {
		t.Fatal("a nested generated file changed without moving the digest")
	}
}

func TestOutputDigestFailsWhenRequiredProvenanceIsMissing(t *testing.T) {
	root := completeFixture(t)
	if err := os.Remove(filepath.Join(root, "schemas", "SOURCE")); err != nil {
		t.Fatal(err)
	}

	_, err := OutputDigest(root)
	if err == nil || !strings.Contains(err.Error(), "schemas/SOURCE") {
		t.Fatalf("OutputDigest() error = %v, want missing provenance failure", err)
	}
}
