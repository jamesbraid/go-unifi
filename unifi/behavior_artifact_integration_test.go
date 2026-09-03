//go:build integration

// unifi/behavior_artifact_integration_test.go
package unifi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

// The probes read schemas/behavior.json as their baseline and, when asked,
// re-measure it. Two modes:
//
//   - default: the artifact, when present, supersedes the hand-pasted
//     baselines in the test files; a probe that measures something else goes
//     red and says to re-measure deliberately.
//   - BEHAVIOR_WRITE=1: the probe records what it measured into the artifact
//     (load-modify-write, so sections other probes own survive) for review
//     as a diff.
//
// With no artifact present both modes degrade to the in-file baselines, so a
// checkout that predates the artifact behaves exactly as before.

// behaviorWriteEnabled reports whether this run records measurements into
// schemas/behavior.json rather than only comparing against it.
func behaviorWriteEnabled() bool { return os.Getenv("BEHAVIOR_WRITE") == "1" }

// loadBehaviorArtifact reads schemas/behavior.json from the module root. A
// missing file is not an error: the probes fall back to their in-file
// baselines.
func loadBehaviorArtifact(t *testing.T) (behavior.Artifact, bool) {
	t.Helper()
	root := fields.ModuleRoot()
	if root == "" {
		return behavior.Artifact{}, false
	}
	a, found, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	return a, found
}

// updateBehaviorArtifact applies modify to the current artifact and writes it
// back, stamping the controller version from schemas/VERSION. Load-modify-
// write so each probe touches only its own section.
func updateBehaviorArtifact(t *testing.T, modify func(*behavior.Artifact)) {
	t.Helper()
	root := fields.ModuleRoot()
	if root == "" {
		t.Fatalf("BEHAVIOR_WRITE=1 but no go.mod above the working directory")
	}
	a, _, err := behavior.Load(root)
	if err != nil {
		t.Fatalf("load %s: %v", behavior.Path, err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "schemas", "VERSION"))
	if err != nil {
		t.Fatalf("read schemas/VERSION: %v", err)
	}
	a.ControllerVersion = strings.TrimSpace(string(raw))
	modify(&a)
	if err := behavior.Write(root, a); err != nil {
		t.Fatalf("write %s: %v", behavior.Path, err)
	}
}
