package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeTable = `package unifi

var FieldValidationPatterns = map[string]map[string]string{
	"FirewallPolicy": {
		"protocol": "all|ax.25|tcp",
		"kept":     "same",
		"dropped":  "gone-next",
	},
}

var FieldConstraints = map[string]map[string]any{
	"FirewallPolicy": {
		"protocol": {Pattern: "all|ax.25|tcp"},
	},
}
`

func writeTable(t *testing.T, root, rel, src string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestValidatorPatternsReadsTheRealTable is the positive control. Every other
// test here builds its own fixture, so a reader that silently returned nothing
// against the real generated files would still pass them -- and apidiff would
// report "no validator changes" forever, which is indistinguishable from a
// clean release.
func TestValidatorPatternsReadsTheRealTable(t *testing.T) {
	patterns, found, err := validatorPatterns("../..")
	if err != nil {
		t.Fatalf("validatorPatterns: %v", err)
	}
	if !found {
		t.Fatal("no validator tables found in the repository")
	}
	if len(patterns) < 100 {
		t.Fatalf("read %d patterns from the real tables; the reader is not working", len(patterns))
	}
	if _, ok := patterns["FirewallPolicy.protocol"]; !ok {
		t.Error("FirewallPolicy.protocol missing; keys are not Type.wire_name")
	}
	if _, ok := patterns["settings.SettingGuestAccess.expire"]; !ok {
		t.Error("settings.SettingGuestAccess.expire missing; the settings table is not being read")
	}
}

// FieldConstraints carries the same patterns, so reading both tables would
// report one edit twice.
func TestValidatorPatternsReadsOneTableOnly(t *testing.T) {
	base := t.TempDir()
	writeTable(t, base, "unifi/validation.generated.go", fakeTable)
	patterns, _, err := validatorPatterns(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 3 {
		t.Fatalf("got %d patterns, want 3: %v", len(patterns), patterns)
	}
}

func TestValidatorSurfaceDeltaReportsEachDirection(t *testing.T) {
	base := t.TempDir()
	writeTable(t, base, "unifi/validation.generated.go", fakeTable)

	current := t.TempDir()
	writeTable(t, current, "unifi/validation.generated.go", strings.NewReplacer(
		`"protocol": "all|ax.25|tcp"`, `"protocol": "all|ax\\.25|tcp"`,
		`"dropped":  "gone-next",`, `"appeared": "brand-new",`,
	).Replace(fakeTable))

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(current); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	changed, added, removed, err := validatorSurfaceDelta(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || !strings.Contains(changed[0], "FirewallPolicy.protocol") {
		t.Errorf("changed = %v, want the protocol pattern", changed)
	}
	if len(changed) == 1 {
		if !strings.Contains(changed[0], `was all|ax.25|tcp`) || !strings.Contains(changed[0], `now all|ax\.25|tcp`) {
			t.Errorf("changed entry does not show both sides:\n%s", changed[0])
		}
	}
	if len(added) != 1 || added[0] != "FirewallPolicy.appeared" {
		t.Errorf("added = %v, want FirewallPolicy.appeared", added)
	}
	if len(removed) != 1 || removed[0] != "FirewallPolicy.dropped" {
		t.Errorf("removed = %v, want FirewallPolicy.dropped", removed)
	}
}

// A baseline predating the published tables cannot be compared against, which
// is not the same as nothing having changed.
func TestValidatorSurfaceDeltaIsSilentWithoutABaseline(t *testing.T) {
	changed, added, removed, err := validatorSurfaceDelta(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(changed)+len(added)+len(removed) != 0 {
		t.Errorf("expected nothing from an empty baseline, got %v %v %v", changed, added, removed)
	}
}

func TestValidatorSectionRendering(t *testing.T) {
	clean := validatorSection("v1.110.0", nil, nil, nil)
	if !strings.Contains(clean, "No validator changes") {
		t.Errorf("clean section = %q", clean)
	}
	section := validatorSection("v1.110.0", []string{"A.b\n    was x\n    now y"}, nil, nil)
	for _, want := range []string{"Validator surface", "A.b", "refuse a value it used to accept"} {
		if !strings.Contains(section, want) {
			t.Errorf("section missing %q in:\n%s", want, section)
		}
	}
}
