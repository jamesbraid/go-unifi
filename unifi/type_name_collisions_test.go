package unifi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A type name that exists in both unifi and unifi/settings is a trap for any
// consumer that looks types up by bare name: it gets a real struct back, just
// not the one it asked for, and nothing about the result says so.
//
// terraform-provider-unifi hit exactly this on Dashboard -- its shape loader
// resolved the root package's type and built a descriptor for a settings
// section out of an unrelated struct. The fix belongs downstream (resolve
// package-qualified names), but the collision set is ours to publish, and a
// new one arriving unannounced is what makes it dangerous.
//
// Setting is the sharpest of the three: a struct here, an interface there.
var knownTypeNameCollisions = []string{
	"Dashboard",
	"FieldConstraint",
	"Setting",
}

// exportedTypeNames reads the exported type names declared in dir. It walks
// the files rather than using parser.ParseDir, which is deprecated and offers
// package association this does not need.
func exportedTypeNames(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	names := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.IsExported() {
					names[ts.Name.Name] = true
				}
			}
		}
	}
	return names
}

func TestTypeNameCollisionsArePinned(t *testing.T) {
	root := exportedTypeNames(t, ".")
	settings := exportedTypeNames(t, "settings")

	// Positive control: a parser that returned nothing would report a clean
	// collision set, which is indistinguishable from no collisions.
	if len(root) < 50 || len(settings) < 20 {
		t.Fatalf("read %d root and %d settings types; the parser is not working",
			len(root), len(settings))
	}

	var found []string
	for name := range root {
		if settings[name] {
			found = append(found, name)
		}
	}
	sort.Strings(found)

	known := map[string]bool{}
	for _, name := range knownTypeNameCollisions {
		known[name] = true
	}

	for _, name := range found {
		if !known[name] {
			t.Errorf("%s now exists in both unifi and unifi/settings and is not pinned\n\n"+
				"A consumer resolving types by bare name will silently get one of the two. "+
				"Add it here and tell the consumers.", name)
		}
	}
	for _, name := range knownTypeNameCollisions {
		if !contains(found, name) {
			t.Errorf("%s is pinned as a collision but no longer collides; the entry is stale", name)
		}
	}
	t.Logf("%d exported types in unifi, %d in settings, %d names in both",
		len(root), len(settings), len(found))
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
