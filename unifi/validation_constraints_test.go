package unifi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestFieldConstraintsMatchTheVariables checks the two shapes of the same
// facts against each other: the per-field variables and constants a caller
// names directly, and the FieldConstraints table a caller reaches by type
// and wire name.
//
// Both are written by one pass over one set of entries, so they agree by
// construction. That is exactly the kind of claim that stops being true
// quietly, and a consumer deriving a validator per attribute would inherit
// the disagreement with no way to see it.
//
// The declarations are read from the generated source rather than by
// reflection, because a package cannot enumerate its own top-level
// declarations at runtime, and naming all of them here would be the
// transcription this table exists to abolish. Each one already says which
// field it describes in its doc comment, which is the join.
func TestFieldConstraintsMatchTheVariables(t *testing.T) {
	compared := 0

	for _, decl := range parseGeneratedValidation(t) {
		typeName, wire, kind := decl.subject, decl.wire, decl.kind
		constraint, ok := FieldConstraints[typeName][wire]
		if !ok {
			t.Errorf("%s describes %s.%s, which FieldConstraints has no entry for", decl.names, typeName, wire)
			continue
		}
		compared++

		switch kind {
		case "values":
			want := constraintValues(constraint)
			if want == "" {
				t.Errorf("%s enumerates %s.%s, but FieldConstraints carries no values for it", decl.names, typeName, wire)
			} else if decl.values[0] != want {
				t.Errorf("%s = [%s], FieldConstraints says [%s]", decl.names, decl.values[0], want)
			}
		case "bounds":
			if !constraint.HasBounds {
				t.Errorf("%s bounds %s.%s, but FieldConstraints reports no bounds", decl.names, typeName, wire)
				continue
			}
			assertPair(t, decl, strconv.FormatInt(constraint.Min, 10), strconv.FormatInt(constraint.Max, 10))
		case "length":
			if !constraint.HasLength {
				t.Errorf("%s bounds the length of %s.%s, but FieldConstraints reports no length", decl.names, typeName, wire)
				continue
			}
			assertPair(t, decl, strconv.FormatInt(constraint.MinLength, 10), strconv.FormatInt(constraint.MaxLength, 10))
		}
	}

	// Every table entry that carries a fact must have produced a declaration.
	for typeName, byWire := range FieldConstraints {
		for wire, c := range byWire {
			if pattern, ok := FieldValidationPatterns[typeName][wire]; !ok || pattern != c.Pattern {
				t.Errorf("FieldValidationPatterns[%q][%q] = %q, FieldConstraints says %q", typeName, wire, pattern, c.Pattern)
			}
		}
	}

	if compared == 0 {
		t.Fatal("no declarations were compared; the source scan found nothing")
	}
	t.Logf("compared %d generated declarations against FieldConstraints", compared)
}

func assertPair(t *testing.T, decl validationDecl, wantLow, wantHigh string) {
	t.Helper()
	if len(decl.values) != 2 {
		t.Errorf("%s declares %d values, want 2", decl.names, len(decl.values))
		return
	}
	if decl.values[0] != wantLow || decl.values[1] != wantHigh {
		t.Errorf("%s = %s..%s, FieldConstraints says %s..%s",
			decl.names, decl.values[0], decl.values[1], wantLow, wantHigh)
	}
}

func constraintValues(c FieldConstraint) string {
	switch {
	case c.Values != nil:
		return strings.Join(c.Values, ",")
	case c.Int64Values != nil:
		parts := make([]string, len(c.Int64Values))
		for i, v := range c.Int64Values {
			parts[i] = strconv.FormatInt(v, 10)
		}
		return strings.Join(parts, ",")
	}
	return ""
}

// validationDecl is one generated declaration: which field it describes,
// which kind of fact it carries, and the literal values it declares.
type validationDecl struct {
	names   string
	subject string
	wire    string
	kind    string
	values  []string
}

// subjectRe pulls the field a generated doc comment describes out of its
// final sentence, as in "the controller accepts for Network.purpose".
var subjectRe = regexp.MustCompile(`for ([A-Za-z0-9_]+)\.([A-Za-z0-9_]+)\.$`)

func parseGeneratedValidation(t *testing.T) []validationDecl {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "validation.generated.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse the generated validation file: %v", err)
	}

	var out []validationDecl
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Doc == nil || (gen.Tok != token.VAR && gen.Tok != token.CONST) {
			continue
		}
		doc := strings.Join(strings.Fields(gen.Doc.Text()), " ")
		match := subjectRe.FindStringSubmatch(doc)
		if match == nil {
			continue
		}

		var kind string
		switch {
		case strings.Contains(doc, "character-count bounds"):
			kind = "length"
		case strings.Contains(doc, "inclusive bounds"):
			kind = "bounds"
		case strings.Contains(doc, "are the values"):
			kind = "values"
		default:
			continue
		}

		var names []string
		var values []string
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
				continue
			}
			names = append(names, value.Names[0].Name)
			values = append(values, renderValidationLiteral(t, value.Values[0]))
		}
		out = append(out, validationDecl{
			names:   strings.Join(names, "/"),
			subject: match[1],
			wire:    match[2],
			kind:    kind,
			values:  values,
		})
	}
	return out
}

// renderValidationLiteral flattens a generated right-hand side: a composite
// literal to its comma-joined elements, a basic literal to its text.
func renderValidationLiteral(t *testing.T, expr ast.Expr) string {
	t.Helper()
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			unquoted, err := strconv.Unquote(e.Value)
			if err != nil {
				t.Fatalf("unquote %s: %v", e.Value, err)
			}
			return unquoted
		}
		return e.Value
	case *ast.CompositeLit:
		parts := make([]string, len(e.Elts))
		for i, elt := range e.Elts {
			parts[i] = renderValidationLiteral(t, elt)
		}
		return strings.Join(parts, ",")
	case *ast.UnaryExpr:
		// A negative bound is a unary minus applied to a literal, not a
		// literal. Several are: an RSSI threshold and a DHCP time offset are
		// both negative, so dropping the sign here would compare -90 against
		// 90 and call them equal.
		return e.Op.String() + renderValidationLiteral(t, e.X)
	}
	return ""
}
