package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// The wire floor is every field a write sends whether or not the caller set
// it. A field with no omitempty is asserted on every write, so a caller that
// filled a struct from partial input ships a zero value for everything it
// never mentioned -- false for a bool, "" for a string, 0 for a number, null
// for a slice, map or pointer -- and the controller stores that. The 10.4.57
// regeneration put roaming_assistant_na_enabled on WLAN's floor and switched
// roaming assistant off on WLANs nobody had touched.
//
// None of this is Go API: struct tags are invisible to the type checker, so
// the apidiff comparison cannot see the floor move. Both halves below are
// derived from the tree they describe, so the baseline is the previous
// release's own code rather than a recorded file somebody accepted.

// floorDirs are the generated packages, each with the qualifier its type names
// need. Dashboard and FieldConstraint are declared in both, so a bare
// "Dashboard.is_public" cannot say which package's field moved -- and the one
// that exists today is unifi's, with settings.Dashboard inert only by luck.
var floorDirs = []struct{ dir, prefix string }{
	{"unifi", ""},
	{"unifi/settings", "settings."},
}

// generatedFloor returns package-qualified "Type.wire_name" for every field in
// root's generated code carrying a json tag without omitempty.
//
// The files are parsed rather than reflected over so the set stays complete on
// its own. Reflection needs a list of types to walk, and that list would want
// updating at exactly the moment it is easiest to forget: when a regeneration
// introduces a resource.
func generatedFloor(root string) ([]string, error) {
	var out []string
	found := false
	for _, pkg := range floorDirs {
		entries, err := os.ReadDir(filepath.Join(root, pkg.dir))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".generated.go") {
				continue
			}
			found = true
			path := filepath.Join(root, pkg.dir, entry.Name())
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
					if wire, ok := alwaysSerialized(field); ok {
						out = append(out, pkg.prefix+spec.Name.Name+"."+wire)
					}
				}
				return true
			})
		}
	}
	if !found {
		return nil, fmt.Errorf("no generated code under %s", root)
	}
	sort.Strings(out)
	return out, nil
}

// alwaysSerialized reports whether a struct field goes on the wire even when
// the caller never set it, and returns its wire name.
//
// omitempty is the whole question. encoding/json emits a tagged field's zero
// value for every type there is, so the type narrows nothing: a bool asserts
// false and the controller stores it, and a slice, map or pointer asserts
// null, which the controller may reject outright -- WLAN's
// schedule_with_duration did exactly that, and it made every WLAN
// read-modify-write fail until it was given omitempty.
func alwaysSerialized(field *ast.Field) (string, bool) {
	if field.Tag == nil {
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

// purposeFloorProgram prints what each Network purpose encoder sends for an
// object the caller left alone, as "purpose wire_name" pairs.
//
// generatedFloor says nothing about Network. Its encoders are hand-written,
// dispatch on purpose, and apply their own emission rules on top of the
// generated tags, so a field's contract there can differ from the one the
// struct declares. That gap let a real change through: remote_vpn_subnets lost
// its omitempty in the site-to-site encoder, putting the key on every
// site-to-site write, and a tag-only view reported no wire change at all.
// Encoding a zero Network and looking is the only honest way to ask.
const purposeFloorProgram = `package main

import (
	"encoding/json"
	"fmt"

	"github.com/ubiquiti-community/go-unifi/unifi"
)

func main() {
	for _, purpose := range unifi.NetworkPurposes {
		raw, err := json.Marshal(&unifi.Network{Purpose: purpose})
		if err != nil {
			panic(err)
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(raw, &payload); err != nil {
			panic(err)
		}
		for wire := range payload {
			fmt.Printf("%s %s\n", purpose, wire)
		}
	}
}
`

// purposeFloor runs purposeFloorProgram against the module rooted at root.
//
// The program is written outside root, which go run still resolves against
// root's module, so measuring a tree never writes into it -- the working tree
// stays clean and the extracted baseline stays the bytes git gave us.
func purposeFloor(root string) ([]string, error) {
	prog, err := os.CreateTemp("", "go-unifi-purpose-floor-*.go")
	if err != nil {
		return nil, err
	}
	defer os.Remove(prog.Name())
	if _, err := prog.WriteString(purposeFloorProgram); err != nil {
		prog.Close()
		return nil, err
	}
	if err := prog.Close(); err != nil {
		return nil, err
	}

	measure := exec.Command("go", "run", prog.Name())
	measure.Dir = root
	var failure bytes.Buffer
	measure.Stderr = &failure
	out, err := measure.Output()
	if err != nil {
		return nil, fmt.Errorf("encode a Network per purpose in %s: %w: %s",
			root, err, strings.TrimSpace(failure.String()))
	}

	var lines []string
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	slices.Sort(lines)
	return lines, nil
}
