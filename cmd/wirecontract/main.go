// Command wirecontract records what the generated client puts on the wire into
// wirecontract/wire_contract.json.
//
// It runs as the second half of `go generate ./...`, after cmd/fields has
// rewritten unifi/: the directive lives in package wirecontract, which sorts
// after unifi, and this command imports the generated packages. Reading the
// finished tree is the point -- the constraint tables are published as the Go
// values the SDK enforces, not re-derived alongside them -- and the types come
// from the source rather than a hand-kept list, which would need updating at the
// moment it is easiest to forget: a new resource.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/capturelock"
	internalfields "github.com/ubiquiti-community/go-unifi/internal/fields"
	"github.com/ubiquiti-community/go-unifi/internal/rebuild"
	"github.com/ubiquiti-community/go-unifi/unifi"
	"github.com/ubiquiti-community/go-unifi/unifi/settings"
	"github.com/ubiquiti-community/go-unifi/wirecontract"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "wirecontract: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// go generate runs this from the declaring package's directory.
	root := internalfields.ModuleRoot()
	if root == "" {
		return fmt.Errorf("no go.mod above the working directory")
	}
	lock, err := capturelock.LoadFile(filepath.Join(root, "schemas", "capture.lock.json"))
	if err != nil {
		return err
	}
	contract := wirecontract.Contract{
		FormatVersion: wirecontract.FormatVersion,
		Provenance: wirecontract.Provenance{
			ControllerVersion: lock.Controller.NetworkVersion,
			ControllerBuild:   lock.Controller.Build,
			CaptureSHA256:     lock.Source.SHA256,
		},
		Types:       map[string]wirecontract.Type{},
		Constraints: map[string]json.RawMessage{},
	}
	// One place names the packages the artifact covers. Each contributes its
	// types and its constraint table, the latter published as its own JSON so the
	// rules consumers validate against are the bytes this SDK enforces.
	for pkg, table := range map[string]any{
		"unifi":          unifi.FieldConstraints,
		"unifi/settings": settings.FieldConstraints,
	} {
		if contract.Constraints[pkg], err = json.Marshal(table); err != nil {
			return err
		}
		if err := addPackage(&contract, root, pkg); err != nil {
			return fmt.Errorf("%s: %w", pkg, err)
		}
	}
	contract.Behavior, contract.Provenance.BehaviorControllerVersion, err = projectBehavior(root, &contract)
	if err != nil {
		return err
	}

	// encoding/json sorts map keys, so a run that found nothing new diffs clean.
	encoded, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(wirecontract.Path))
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	// cmd/fields stamped the generated-output digest before this file reached its
	// final form, and the digest covers it, so re-stamp it here.
	digest, err := rebuild.OutputDigest(root)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "schemas", "GENERATED_SHA256"), []byte(digest+"\n"), 0o644)
}

// segmentPattern matches the collection a request path the generated client
// builds addresses: the site placeholder marks where the prefix ends, and the
// collection runs to the id placeholder. It can be more than one segment --
// firewall zones live on firewall/zone.
var segmentPattern = regexp.MustCompile(
	`"(?:api/s/%s/(?:rest|stat)/|v2/api/site/%s/)([a-z0-9-]+(?:/[a-z0-9-]+)*)`)

// addPackage adds every exported struct in one package directory that carries a
// json tag.
//
// Being tagged is what makes a type a wire object; being generated is neither
// required nor sufficient. A hand-written type the generated code embeds is on
// the wire just as much -- every settings document sends BaseSetting.key -- while
// a generated struct with no tagged field sends nothing.
func addPackage(contract *wirecontract.Contract, root, pkg string) error {
	dir := filepath.Join(root, filepath.FromSlash(pkg))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}
		generated := strings.HasSuffix(name, ".generated.go")
		declared := wireTypes(file, pkg, generated)
		if generated && len(declared) > 0 {
			// The collection segments a generated file addresses belong to the
			// resource it declares, which is the type with the shortest name:
			// the generator names a nested type by concatenating its parent and
			// the field.
			resource := 0
			for i := range declared {
				if len(declared[i].GoName) < len(declared[resource].GoName) {
					resource = i
				}
			}
			for _, match := range segmentPattern.FindAllSubmatch(src, -1) {
				declared[resource].APIPaths = append(declared[resource].APIPaths, string(match[1]))
			}
			slices.Sort(declared[resource].APIPaths)
			declared[resource].APIPaths = slices.Compact(declared[resource].APIPaths)
		}
		for _, declaredType := range declared {
			contract.Types[pkg+"."+declaredType.GoName] = declaredType
		}
	}
	return nil
}

// wireTypes returns the tagged structs one file declares, fields in declaration
// order.
func wireTypes(file *ast.File, pkg string, generated bool) []wirecontract.Type {
	var out []wirecontract.Type
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		structType, ok := spec.Type.(*ast.StructType)
		// An unexported type is transport plumbing -- the response envelope and
		// its kin -- which a consumer cannot name and does not send.
		if !ok || !spec.Name.IsExported() {
			return true
		}
		declared := wirecontract.Type{Package: pkg, GoName: spec.Name.Name, Generated: generated}
		for _, field := range structType.Fields.List {
			goType := types.ExprString(field.Type)
			if len(field.Names) == 0 {
				// An embedded type's own fields are on the wire too.
				declared.Embeds = append(declared.Embeds, goType)
				continue
			}
			// The tag comes from source text rather than a live type, so
			// reflect.StructTag parses it once the literal is unquoted.
			raw := ""
			if field.Tag != nil {
				raw, _ = strconv.Unquote(field.Tag.Value)
			}
			tag, tagged := reflect.StructTag(raw).Lookup("json")
			wire, opts, _ := strings.Cut(tag, ",")
			if !tagged || wire == "" || wire == "-" {
				continue
			}
			omitEmpty := slices.Contains(strings.Split(opts, ","), "omitempty")
			for _, ident := range field.Names {
				declared.Fields = append(declared.Fields, wirecontract.Field{
					Wire:      wire,
					GoName:    ident.Name,
					GoType:    goType,
					OmitEmpty: omitEmpty,
				})
			}
		}
		if len(declared.Fields) == 0 {
			return true
		}
		if generated {
			declared.SchemaResource = schemaResource(pkg, declared)
		}
		out = append(out, declared)
		return true
	})
	return out
}

// schemaResource is the key the constraint tables and the probes use for a type:
// the controller's own resource name, which for a settings document is the Go
// type plus the "Setting" prefix the settings package drops -- the site NTP
// document is type settings.Ntp and resource SettingNtp. A document is exactly
// the type that embeds BaseSetting, which is how the generator decides to drop it.
func schemaResource(pkg string, declared wirecontract.Type) string {
	if pkg == "unifi/settings" && slices.Contains(declared.Embeds, "BaseSetting") {
		return "Setting" + declared.GoName
	}
	return declared.GoName
}

// projectBehavior rekeys schemas/behavior.json onto the artifact's type keys,
// carrying each section's value verbatim, and returns the controller version the
// probes ran against.
//
// Most sections are keyed on the resource name and the clearing results on the
// controller's collection segment, because each probe recorded what it had in
// hand. Both are translated here rather than rekeyed in the file: re-recording
// needs a live controller, and hand-editing a measured artifact is the one thing
// this tree does not do. A key no type claims fails the run for the same reason
// -- the alternative is dropping a measured result quietly.
func projectBehavior(
	root string,
	contract *wirecontract.Contract,
) (map[string]map[string]json.RawMessage, string, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(behavior.Path)))
	if err != nil {
		return nil, "", err
	}
	var file map[string]json.RawMessage
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, "", fmt.Errorf("parse %s: %w", behavior.Path, err)
	}
	var controller string
	if err := json.Unmarshal(file["controller_version"], &controller); err != nil {
		return nil, "", fmt.Errorf("%s records no controller version: %w", behavior.Path, err)
	}

	// Both key spaces share one index: a resource name is a Go type name and a
	// collection segment is lowercase, so they cannot collide, and a section that
	// keys on either needs no special case. Two types claiming one key does
	// collide, and fails the run -- data keyed on it could not be placed.
	index := map[string]string{}
	for _, typeKey := range slices.Sorted(maps.Keys(contract.Types)) {
		declared := contract.Types[typeKey]
		for _, key := range slices.Concat([]string{declared.SchemaResource}, declared.APIPaths) {
			if key == "" {
				continue
			}
			if existing, ok := index[key]; ok {
				return nil, "", fmt.Errorf("%q names both %s and %s; measured data keyed on it cannot be placed",
					key, existing, typeKey)
			}
			index[key] = typeKey
		}
	}

	out := map[string]map[string]json.RawMessage{}
	for _, section := range slices.Sorted(maps.Keys(file)) {
		if section == "controller_version" {
			continue // in the provenance block instead
		}
		var byKey map[string]json.RawMessage
		if err := json.Unmarshal(file[section], &byKey); err != nil {
			return nil, "", fmt.Errorf("%s section %s is not keyed by resource: %w", behavior.Path, section, err)
		}
		for _, key := range slices.Sorted(maps.Keys(byKey)) {
			typeKey, ok := index[key]
			if !ok {
				return nil, "", fmt.Errorf("%s section %s names %q, which no type in the artifact claims; "+
					"a probe that records a key this tree cannot place has to be re-run, not re-keyed by hand",
					behavior.Path, section, key)
			}
			if out[typeKey] == nil {
				out[typeKey] = map[string]json.RawMessage{}
			}
			out[typeKey][section] = byKey[key]
		}
	}
	return out, controller, nil
}
