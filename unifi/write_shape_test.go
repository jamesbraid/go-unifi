package unifi

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var updateWriteShape = flag.Bool("update-write-shape", false,
	"rewrite testdata/always_serialized_fields.txt from the current generated code")

// TestGeneratedWriteShape pins every field the client puts on the wire whether
// or not the caller set it.
//
// A field with no omitempty is asserted on every write. A caller that builds
// an object from partial input -- a Terraform provider filling a struct from
// a resource model, say -- therefore sends a zero value for everything it
// never mentioned: false for a bool, null for a slice, map or pointer.
//
// The 10.4.57 regeneration added roaming_assistant_na_enabled to WLAN. Before
// it, the key never reached the wire and the controller kept its own value.
// After it, every WLAN write asserted false, and roaming assistant switched
// off on WLANs nobody had touched. Nothing failed, because nothing watched
// the write shape.
//
// The file this compares against records what the wire looks like today
// rather than approving it, so the next regeneration that widens the shape
// has to say so out loud.
//
// Network is included even though its hand-written encoder emits a curated
// subset per purpose: the declaration is still what a caller sees, and
// TestNetworkEncoderCoversGeneratedFields watches the encoder separately.
func TestGeneratedWriteShape(t *testing.T) {
	current, err := alwaysSerializedFields()
	if err != nil {
		t.Fatalf("scan generated code: %v", err)
	}

	golden := filepath.Join("testdata", "always_serialized_fields.txt")

	if *updateWriteShape {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(strings.Join(current, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d fields)", golden, len(current))
		return
	}

	raw, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read %s: %v\n\nRun: go test ./unifi/ -run TestGeneratedWriteShape -update-write-shape", golden, err)
	}

	want := map[string]bool{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			want[line] = true
		}
	}
	have := map[string]bool{}
	for _, entry := range current {
		have[entry] = true
	}

	var added, removed []string
	for entry := range have {
		if !want[entry] {
			added = append(added, entry)
		}
	}
	for entry := range want {
		if !have[entry] {
			removed = append(removed, entry)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)

	if len(added) > 0 {
		t.Errorf("regeneration widened the write shape by %d field(s):\n  %s\n\n"+
			"Each of these is now sent as false on every write where the caller left it unset, and the "+
			"controller will store false. Decide which this is:\n"+
			"  - the caller should be able to leave it alone: give it omitempty = true in "+
			"overrides/fields.toml, so unset stays unset\n"+
			"  - asserting the zero value is correct: re-record with\n"+
			"      go test ./unifi/ -run TestGeneratedWriteShape -update-write-shape",
			len(added), strings.Join(added, "\n  "))
	}
	if len(removed) > 0 {
		t.Errorf("%d field(s) left the write shape:\n  %s\n\n"+
			"Re-record with -update-write-shape once the change is understood.",
			len(removed), strings.Join(removed, "\n  "))
	}
}

// alwaysSerializedFields returns "Struct.wire_name" for every field in the
// generated code that carries a json tag without omitempty and a type whose
// zero value still marshals.
//
// The generated files are parsed rather than reflected over because the set
// has to stay complete on its own. Reflection needs a list of types to walk,
// and that list would need updating at exactly the moment it is easiest to
// forget: when a regeneration introduces a resource.
func alwaysSerializedFields() ([]string, error) {
	var out []string

	for _, dir := range []string{".", "settings"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".generated.go") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", path, err)
			}

			ast.Inspect(file, func(n ast.Node) bool {
				spec, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}
				structType, ok := spec.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, field := range structType.Fields.List {
					wire, ok := alwaysSerializedField(field)
					if ok {
						out = append(out, spec.Name.Name+"."+wire)
					}
				}
				return true
			})
		}
	}

	sort.Strings(out)
	return out, nil
}

// alwaysSerializedField reports whether a struct field goes on the wire even
// when the caller never set it, and returns its wire name.
//
// Two shapes qualify, and the second is why this is not a bool-only check. A
// bool asserts false, which the controller stores. A slice, map or pointer
// asserts null, which the controller may reject outright: WLAN's
// schedule_with_duration did exactly that, and it made every WLAN
// read-modify-write fail until it was given omitempty.
func alwaysSerializedField(field *ast.Field) (string, bool) {
	if field.Tag == nil || !nullableOrBool(field.Type) {
		return "", false
	}

	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false
	}
	jsonTag := structTag(tag, "json")
	if jsonTag == "" {
		return "", false
	}

	name, opts, _ := strings.Cut(jsonTag, ",")
	if name == "" || name == "-" {
		return "", false
	}
	if slices.Contains(strings.Split(opts, ","), "omitempty") {
		return "", false
	}
	return name, true
}

// nullableOrBool reports whether a field type marshals to a fixed zero value
// when unset: false for a bool, null for a slice, map or pointer.
func nullableOrBool(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "bool"
	case *ast.ArrayType, *ast.MapType, *ast.StarExpr:
		return true
	}
	return false
}

// structTag pulls one key out of a struct tag. reflect.StructTag.Get would do
// this, but the tag here comes from source text rather than a live type.
func structTag(tag, key string) string {
	for part := range strings.FieldsSeq(tag) {
		name, value, found := strings.Cut(part, ":")
		if !found || name != key {
			continue
		}
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return ""
		}
		return unquoted
	}
	return ""
}
