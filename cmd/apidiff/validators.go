package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The generated validator tables are a published surface: consumers build
// their own validation from them, and at least one downstream provider
// derives plan-time rules directly. A pattern that changes can start refusing
// a value it used to accept, which breaks that consumer -- and apidiff sees
// none of it, because the Go API and the always-serialized field set are both
// unchanged when only a pattern string moves.
//
// v1.111.0 is the case in point: `ax.25` gained the escape its dot always
// needed, which narrowed FirewallPolicy.protocol from accepting `axX25` to
// refusing it. Correct, and invisible to every other guard here.
var validatorTablePaths = []string{
	"unifi/validation.generated.go",
	"unifi/settings/validation.generated.go",
}

// validatorPatterns reads FieldValidationPatterns out of the generated tables
// under root, keyed "Type.wire_name".
//
// FieldConstraints in the same file carries the same patterns again. Reading
// only the one table keeps a single edit from being reported twice.
func validatorPatterns(root string) (map[string]string, bool, error) {
	out := map[string]string{}
	found := false
	for _, rel := range validatorTablePaths {
		path := filepath.Join(root, rel)
		src, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, false, err
		}
		found = true
		file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
		if err != nil {
			return nil, false, fmt.Errorf("parse %s: %w", rel, err)
		}
		prefix := ""
		if strings.Contains(rel, "/settings/") {
			prefix = "settings."
		}
		collectPatterns(file, prefix, out)
	}
	return out, found, nil
}

func collectPatterns(file *ast.File, prefix string, out map[string]string) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || value.Names[0].Name != "FieldValidationPatterns" {
				continue
			}
			if len(value.Values) != 1 {
				continue
			}
			byType, ok := value.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, typeEntry := range byType.Elts {
				pair, ok := typeEntry.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				typeName, ok := stringLit(pair.Key)
				if !ok {
					continue
				}
				byWire, ok := pair.Value.(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, wireEntry := range byWire.Elts {
					wirePair, ok := wireEntry.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					wire, wireOK := stringLit(wirePair.Key)
					pattern, patternOK := stringLit(wirePair.Value)
					if wireOK && patternOK {
						out[prefix+typeName+"."+wire] = pattern
					}
				}
			}
		}
	}
}

func stringLit(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

// validatorSurfaceDelta compares the published validators in baseTree against
// the working tree.
func validatorSurfaceDelta(baseTree string) (changed, added, removed []string, err error) {
	baseTable, baseFound, err := validatorPatterns(baseTree)
	if err != nil {
		return nil, nil, nil, err
	}
	currentTable, _, err := validatorPatterns(".")
	if err != nil {
		return nil, nil, nil, err
	}
	if !baseFound {
		// The baseline predates the published tables; there is nothing to
		// compare against, which is not the same as nothing having changed.
		return nil, nil, nil, nil
	}
	for key, basePattern := range baseTable {
		currentPattern, still := currentTable[key]
		switch {
		case !still:
			removed = append(removed, key)
		case currentPattern != basePattern:
			changed = append(changed, fmt.Sprintf("%s\n    was %s\n    now %s", key, basePattern, currentPattern))
		}
	}
	for key := range currentTable {
		if _, had := baseTable[key]; !had {
			added = append(added, key)
		}
	}
	sort.Strings(changed)
	sort.Strings(added)
	sort.Strings(removed)
	return changed, added, removed, nil
}

func validatorSection(base string, changed, added, removed []string) string {
	if len(changed) == 0 && len(added) == 0 && len(removed) == 0 {
		return fmt.Sprintf("No validator changes against `%s`: every published pattern is unchanged.", orNone(base))
	}
	var section strings.Builder
	fmt.Fprintf(&section, "**Validator surface** vs `%s` (patterns consumers derive their own validation from):\n\n```\n", base)
	for _, line := range changed {
		fmt.Fprintf(&section, "~ %s\n", line)
	}
	for _, key := range added {
		fmt.Fprintf(&section, "+ %s\n", key)
	}
	for _, key := range removed {
		fmt.Fprintf(&section, "- %s\n", key)
	}
	section.WriteString("```\n")
	if len(changed) > 0 {
		section.WriteString("~ a changed pattern may refuse a value it used to accept. " +
			"Which direction it moved is not decided here -- compare them and say so in the " +
			"release notes, because a consumer deriving this validator inherits the change.\n")
	}
	if len(removed) > 0 {
		section.WriteString("- a removed pattern leaves a consumer with no rule where it had one\n")
	}
	return strings.TrimRight(section.String(), "\n")
}
