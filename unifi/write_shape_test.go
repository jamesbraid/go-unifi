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
	"rewrite testdata/always_serialized_bools.txt from the current generated code")

// TestGeneratedWriteShape pins every bool the client puts on the wire whether
// or not the caller set it.
//
// A bool field with no omitempty is asserted on every write. A caller who
// builds an object from partial input -- a Terraform provider assembling a
// struct from a resource model, say -- therefore sends false for every toggle
// it never mentioned, and the controller stores false.
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
	current, err := alwaysSerializedBools()
	if err != nil {
		t.Fatalf("scan generated code: %v", err)
	}

	golden := filepath.Join("testdata", "always_serialized_bools.txt")

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
			"  - the caller should be able to leave it alone: give it pointer = true and omitempty = true "+
			"in overrides/fields.toml, so unset stays unset\n"+
			"  - asserting false is correct: re-record with\n"+
			"      go test ./unifi/ -run TestGeneratedWriteShape -update-write-shape",
			len(added), strings.Join(added, "\n  "))
	}
	if len(removed) > 0 {
		t.Errorf("%d field(s) left the write shape:\n  %s\n\n"+
			"Re-record with -update-write-shape once the change is understood.",
			len(removed), strings.Join(removed, "\n  "))
	}
}

// alwaysSerializedBools returns "Struct.wire_name" for every bool field in the
// generated code that carries a json tag without omitempty.
//
// The generated files are parsed rather than reflected over because the set
// has to stay complete on its own. Reflection needs a list of types to walk,
// and that list would need updating at exactly the moment it is easiest to
// forget: when a regeneration introduces a resource.
func alwaysSerializedBools() ([]string, error) {
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
					wire, ok := alwaysSerializedBool(field)
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

// alwaysSerializedBool reports whether a struct field is a plain bool with a
// json tag that has no omitempty, and returns its wire name.
func alwaysSerializedBool(field *ast.Field) (string, bool) {
	ident, ok := field.Type.(*ast.Ident)
	if !ok || ident.Name != "bool" || field.Tag == nil {
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
