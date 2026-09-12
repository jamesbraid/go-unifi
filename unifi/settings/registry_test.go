package settings

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// EVERY SETTING TYPE MUST BE REGISTERED IN GetSettingKey, WITH ITS OWN KEY.
//
// GetSettingKey is a hand-written switch over every setting type and the only
// thing that turns a Go type into the wire key the controller stores it under.
// Its default arm returns an error, so a setting type nobody added to the
// switch cannot be written at all, and the failure arrives at runtime in the
// caller rather than here.
//
// Most of these types are generated, so a controller refresh adds new ones
// without touching the switch -- which is the drift this test exists for, and
// which it found: see unregisteredSettingTypes. It also catches the other
// half, two arms returning the same key, one copy-paste away in a switch this
// long and enough to make one setting overwrite another.
//
// Both halves are read from source, because Go cannot enumerate the types that
// satisfy an interface at runtime.
//
// This replaced four per-type round-trip tests. Each unmarshalled one fixture,
// read one or two scalars back, and checked that type's single switch arm;
// between them they covered four of the forty-nine, and since those types are
// plain structs with no custom (un)marshalling, the rest of what they asserted
// was encoding/json's own behaviour. igmp_snooping_test.go stays, because the
// type behind it has hand-written decode logic (querier_addresses.go) that a
// fixture is the right way to cover.

// unregisteredSettingTypes are setting types with no case in GetSettingKey.
// Both were found by this test when it was written. Registering one means
// naming the key the controller stores it under, and the generated code does
// not carry that key -- only the schema the generator read does. Until one is
// measured against a controller, a guessed key would write to an endpoint
// nobody has seen answer, so they stay listed here and stay unwritable.
var unregisteredSettingTypes = map[string]string{
	"SuperApplicationDegradation": "key not measured; generated 10.6.101, never registered",
	"TestAndCommit":               "key not measured; generated 10.6.101, never registered",
}

func TestEverySettingTypeIsRegistered(t *testing.T) {
	types, keys := parseSettingsRegistry(t)

	for _, name := range types {
		_, registered := keys[name]
		_, known := unregisteredSettingTypes[name]
		switch {
		case !registered && !known:
			t.Errorf("setting type %s embeds BaseSetting but has no case in GetSettingKey, so "+
				"GetSettingKey errors for it and the setting cannot be written. Add a case "+
				"returning the key the controller stores it under -- measured, not guessed -- "+
				"or list it in unregisteredSettingTypes with the reason.", name)
		case registered && known:
			t.Errorf("setting type %s is registered now; drop it from unregisteredSettingTypes", name)
		}
	}

	for name := range keys {
		if !slices.Contains(types, name) {
			t.Errorf("GetSettingKey has a case for %s, which is no longer a setting type here; drop the case", name)
		}
	}
	for name := range unregisteredSettingTypes {
		if !slices.Contains(types, name) {
			t.Errorf("unregisteredSettingTypes lists %s, which is no longer a setting type here; drop it", name)
		}
	}

	byKey := map[string][]string{}
	for name, key := range keys {
		if key == "" {
			t.Errorf("GetSettingKey returns an empty key for %s", name)
			continue
		}
		byKey[key] = append(byKey[key], name)
	}
	for key, names := range byKey {
		if len(names) > 1 {
			slices.Sort(names)
			t.Errorf("GetSettingKey returns %q for more than one type (%s); writing either would "+
				"overwrite the other's document", key, strings.Join(names, ", "))
		}
	}

	if len(types) < 40 {
		t.Fatalf("found only %d setting types; the parse missed some and this test checked less "+
			"than it claims", len(types))
	}
	t.Logf("checked %d setting types against %d registered keys", len(types), len(keys))
}

// TestGetSettingKeyUsesTheRawSettingsOwnKey pins the one arm that returns no
// literal. A RawSetting is whatever key it was decoded with, and an undecided
// one has to be an error rather than the empty string, which a caller would
// send as a path segment.
func TestGetSettingKeyUsesTheRawSettingsOwnKey(t *testing.T) {
	var raw RawSetting
	raw.SetKey("some_unmodelled_setting")
	if key, err := GetSettingKey(&raw); err != nil || key != "some_unmodelled_setting" {
		t.Errorf("GetSettingKey(RawSetting) = (%q, %v), want (some_unmodelled_setting, nil)", key, err)
	}
	if key, err := GetSettingKey(&RawSetting{}); err == nil {
		t.Errorf("GetSettingKey accepted a keyless RawSetting and returned %q", key)
	}
}

// parseSettingsRegistry returns every type in the package that embeds
// BaseSetting, and the key each case in GetSettingKey returns. RawSetting is
// excluded from both: it is the catch-all for an unmodelled setting and
// carries its key instead of being assigned one.
func parseSettingsRegistry(t *testing.T) (types []string, keys map[string]string) {
	t.Helper()

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse the settings package: %v", err)
	}
	pkg, ok := pkgs["settings"]
	if !ok {
		t.Fatal("no settings package found in .")
	}

	keys = map[string]string{}
	for _, file := range pkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeSpec:
				if embedsBaseSetting(node) && node.Name.Name != "RawSetting" {
					types = append(types, node.Name.Name)
				}
			case *ast.CaseClause:
				if name, key, ok := settingKeyCase(node); ok {
					keys[name] = key
				}
			}
			return true
		})
	}
	slices.Sort(types)
	return types, keys
}

func embedsBaseSetting(ts *ast.TypeSpec) bool {
	st, ok := ts.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return false
	}
	for _, field := range st.Fields.List {
		ident, ok := field.Type.(*ast.Ident)
		if len(field.Names) == 0 && ok && ident.Name == "BaseSetting" {
			return true
		}
	}
	return false
}

// settingKeyCase reads a `case *T: return "key", nil` clause.
func settingKeyCase(clause *ast.CaseClause) (name, key string, ok bool) {
	if len(clause.List) != 1 {
		return "", "", false
	}
	star, ok := clause.List[0].(*ast.StarExpr)
	if !ok {
		return "", "", false
	}
	ident, ok := star.X.(*ast.Ident)
	if !ok || ident.Name == "RawSetting" {
		return "", "", false
	}
	for _, stmt := range clause.Body {
		ret, isReturn := stmt.(*ast.ReturnStmt)
		if !isReturn || len(ret.Results) == 0 {
			continue
		}
		lit, isLit := ret.Results[0].(*ast.BasicLit)
		if !isLit || lit.Kind != token.STRING {
			continue
		}
		if unquoted, err := strconv.Unquote(lit.Value); err == nil {
			return ident.Name, unquoted, true
		}
	}
	return "", "", false
}
