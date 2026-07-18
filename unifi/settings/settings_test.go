package settings

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestAllGeneratedSettingsHaveKeys guards against a generated setting type
// missing its case in GetSettingKey: it scans the package's *.generated.go
// files for structs embedding BaseSetting and asserts each has a
// corresponding `case *<Name>:` in settings.go.
func TestAllGeneratedSettingsHaveKeys(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)

	generated, err := filepath.Glob(filepath.Join(dir, "*.generated.go"))
	if err != nil {
		t.Fatalf("glob generated files: %v", err)
	}
	if len(generated) == 0 {
		t.Fatal("no *.generated.go files found")
	}

	structRe := regexp.MustCompile(`type (\w+) struct \{\s*BaseSetting\b`)
	var names []string
	for _, f := range generated {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range structRe.FindAllStringSubmatch(string(src), -1) {
			names = append(names, m[1])
		}
	}
	if len(names) == 0 {
		t.Fatal("no structs embedding BaseSetting found in generated files")
	}

	settingsSrc, err := os.ReadFile(filepath.Join(dir, "settings.go"))
	if err != nil {
		t.Fatalf("read settings.go: %v", err)
	}
	caseRe := regexp.MustCompile(`case \*(\w+):`)
	registered := map[string]bool{}
	for _, m := range caseRe.FindAllStringSubmatch(string(settingsSrc), -1) {
		registered[m[1]] = true
	}

	sort.Strings(names)
	var missing []string
	for _, name := range names {
		if !registered[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("generated setting types missing a GetSettingKey case: %s", strings.Join(missing, ", "))
	}
}
