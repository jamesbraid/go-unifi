package main

import (
	"context"
	"flag"
	"fmt"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

type status string

const (
	compatible status = "compatible"
	breaking   status = "breaking"
	ambiguous  status = "ambiguous"
)

type apiEntry struct {
	Display string
	Value   string
}

type change struct {
	Kind   string
	Symbol string
}

type report struct {
	Status  status
	Changes []change
	Errors  []string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("apicompat", flag.ContinueOnError)
	flags.SetOutput(stderr)
	base := flags.String("base", "", "base Go module directory")
	candidate := flags.String("candidate", "", "candidate Go module directory")
	markdown := flags.String("markdown", "", "optional path for the Markdown report")
	if err := flags.Parse(args); err != nil {
		return 3
	}
	if *base == "" || *candidate == "" {
		fmt.Fprintln(stderr, "-base and -candidate are required")
		return 3
	}

	result := classify(context.Background(), *base, *candidate)
	body := result.Markdown()
	if _, err := io.WriteString(stdout, body); err != nil {
		fmt.Fprintf(stderr, "write report: %v\n", err)
		return 3
	}
	if *markdown != "" {
		if err := os.WriteFile(*markdown, []byte(body), 0o644); err != nil {
			fmt.Fprintf(stderr, "write Markdown report: %v\n", err)
			return 3
		}
	}

	switch result.Status {
	case compatible:
		return 0
	case breaking:
		return 2
	default:
		return 3
	}
}

func classify(ctx context.Context, baseDir, candidateDir string) report {
	base, baseErrors := loadAPI(ctx, baseDir)
	candidate, candidateErrors := loadAPI(ctx, candidateDir)
	if len(baseErrors)+len(candidateErrors) != 0 {
		errors := make([]string, 0, len(baseErrors)+len(candidateErrors))
		for _, message := range baseErrors {
			errors = append(errors, "base: "+message)
		}
		for _, message := range candidateErrors {
			errors = append(errors, "candidate: "+message)
		}
		sort.Strings(errors)
		return report{Status: ambiguous, Errors: errors}
	}

	var changes []change
	keys := make([]string, 0, len(base))
	for key := range base {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		before := base[key]
		after, ok := candidate[key]
		switch {
		case !ok:
			changes = append(changes, change{Kind: "Removed", Symbol: before.Display})
		case before.Value != after.Value:
			changes = append(changes, change{Kind: "Changed", Symbol: before.Display})
		}
	}
	if len(changes) != 0 {
		return report{Status: breaking, Changes: changes}
	}
	return report{Status: compatible}
}

func (r report) Markdown() string {
	var body strings.Builder
	body.WriteString("# Go API compatibility\n\nStatus: **")
	body.WriteString(string(r.Status))
	body.WriteString("**\n")
	if len(r.Changes) != 0 {
		body.WriteString("\n## Breaking changes\n\n")
		for _, item := range r.Changes {
			fmt.Fprintf(&body, "- %s `%s`\n", item.Kind, item.Symbol)
		}
	}
	if len(r.Errors) != 0 {
		body.WriteString("\n## Load errors\n\n")
		for _, message := range r.Errors {
			fmt.Fprintf(&body, "- `%s`\n", strings.ReplaceAll(message, "`", "'"))
		}
	}
	return body.String()
}

func loadAPI(ctx context.Context, dir string) (map[string]apiEntry, []string) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, []string{err.Error()}
	}
	if _, err := os.ReadFile(filepath.Join(absolute, "go.mod")); err != nil {
		return nil, []string{normalizeLoadError(absolute, "read go.mod: "+err.Error())}
	}
	if _, err := os.Stat(filepath.Join(absolute, "unifi")); errorsIsNotExist(err) {
		return map[string]apiEntry{}, nil
	} else if err != nil {
		return nil, []string{normalizeLoadError(absolute, "inspect unifi packages: "+err.Error())}
	}
	config := &packages.Config{
		Context: ctx,
		Dir:     absolute,
		Env:     fixedGoEnv(),
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo |
			packages.NeedModule,
	}
	loaded, err := packages.Load(config, "./unifi/...")
	if err != nil {
		return nil, []string{normalizeLoadError(absolute, err.Error())}
	}
	var loadErrors []string
	for _, pkg := range loaded {
		for _, pkgErr := range pkg.Errors {
			loadErrors = append(loadErrors, normalizeLoadError(absolute, pkgErr.Error()))
		}
		if pkg.Types == nil {
			loadErrors = append(loadErrors, pkg.PkgPath+": type information unavailable")
		}
	}
	if len(loaded) == 0 {
		return map[string]apiEntry{}, nil
	}
	if len(loadErrors) != 0 {
		sort.Strings(loadErrors)
		return nil, compactStrings(loadErrors)
	}

	sort.Slice(loaded, func(i, j int) bool { return loaded[i].PkgPath < loaded[j].PkgPath })
	api := make(map[string]apiEntry)
	for _, pkg := range loaded {
		if errors := addPackageAPI(api, pkg.Types); len(errors) != 0 {
			loadErrors = append(loadErrors, errors...)
		}
	}
	if len(loadErrors) != 0 {
		sort.Strings(loadErrors)
		return nil, compactStrings(loadErrors)
	}
	return api, nil
}

func fixedGoEnv() []string {
	env := make([]string, 0, len(os.Environ())+4)
	for _, setting := range os.Environ() {
		name, _, _ := strings.Cut(setting, "=")
		switch name {
		case "GOOS", "GOARCH", "CGO_ENABLED", "GOWORK", "GOFLAGS", "GOEXPERIMENT", "GODEBUG":
			continue
		}
		env = append(env, setting)
	}
	return append(env,
		"GOOS=linux",
		"GOARCH=amd64",
		"CGO_ENABLED=0",
		"GOWORK=off",
		"GOFLAGS=",
		"GOEXPERIMENT=",
		"GODEBUG=",
	)
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}

func normalizeLoadError(root, message string) string {
	message = filepath.ToSlash(message)
	root = filepath.ToSlash(root)
	message = strings.ReplaceAll(message, root+"/", "")
	return strings.TrimSpace(message)
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := values[:0]
	for index, value := range values {
		if index == 0 || value != values[index-1] {
			result = append(result, value)
		}
	}
	return result
}

func addPackageAPI(api map[string]apiEntry, pkg *types.Package) []string {
	packageKey := "package " + pkg.Path()
	api[packageKey] = apiEntry{
		Display: packageKey,
		Value:   "name " + pkg.Name(),
	}

	scope := pkg.Scope()
	names := scope.Names()
	sort.Strings(names)
	var errors []string
	reachable := newReachableTypes(pkg)
	for _, name := range names {
		if !tokenExported(name) {
			continue
		}
		object := scope.Lookup(name)
		symbol := pkg.Path() + "." + name
		entry := apiEntry{Display: symbol}
		switch object := object.(type) {
		case *types.Const:
			entry.Value = "const " + typeString(object.Type()) + " = " + object.Val().ExactString()
		case *types.Var:
			entry.Value = "var " + typeString(object.Type())
		case *types.Func:
			entry.Value = "func " + signatureString(object.Type().(*types.Signature))
		case *types.TypeName:
			entry.Value = typeDeclaration(object)
		default:
			errors = append(errors, "unsupported exported declaration "+symbol)
			continue
		}
		if previous, exists := api[symbol]; exists && previous.Value != entry.Value {
			errors = append(errors, "conflicting exported declaration "+symbol)
			continue
		}
		api[symbol] = entry
		reachable.visit(object.Type())
	}
	for _, named := range reachable.sortedNamed() {
		typeName := named.Obj()
		symbol := pkg.Path() + "." + typeName.Name()
		if !typeName.Exported() {
			api["reachable "+symbol] = apiEntry{
				Display: symbol,
				Value:   "reachable " + typeDeclaration(typeName),
			}
		}
		addMethodSets(api, symbol, named)
	}
	return errors
}

type reachableTypes struct {
	pkg   *types.Package
	named map[*types.Named]bool
	seen  map[types.Type]bool
}

func newReachableTypes(pkg *types.Package) *reachableTypes {
	return &reachableTypes{pkg: pkg, named: make(map[*types.Named]bool), seen: make(map[types.Type]bool)}
}

func (r *reachableTypes) visit(value types.Type) {
	if value == nil || r.seen[value] {
		return
	}
	r.seen[value] = true
	switch value := value.(type) {
	case *types.Alias:
		r.visit(value.Rhs())
	case *types.Array:
		r.visit(value.Elem())
	case *types.Chan:
		r.visit(value.Elem())
	case *types.Interface:
		value.Complete()
		for index := 0; index < value.NumMethods(); index++ {
			r.visit(value.Method(index).Type())
		}
		for index := 0; index < value.NumEmbeddeds(); index++ {
			r.visit(value.EmbeddedType(index))
		}
	case *types.Map:
		r.visit(value.Key())
		r.visit(value.Elem())
	case *types.Named:
		for index := 0; index < value.TypeArgs().Len(); index++ {
			r.visit(value.TypeArgs().At(index))
		}
		if value.Obj().Pkg() != r.pkg {
			return
		}
		definition := value.Origin()
		r.named[definition] = true
		r.visit(definition.Underlying())
		for index := 0; index < definition.NumMethods(); index++ {
			method := definition.Method(index)
			if method.Exported() {
				r.visit(method.Type())
			}
		}
	case *types.Pointer:
		r.visit(value.Elem())
	case *types.Signature:
		r.visit(value.Params())
		r.visit(value.Results())
		visitTypeParameters(r, value.RecvTypeParams())
		visitTypeParameters(r, value.TypeParams())
	case *types.Slice:
		r.visit(value.Elem())
	case *types.Struct:
		for index := 0; index < value.NumFields(); index++ {
			field := value.Field(index)
			if field.Exported() || field.Embedded() {
				r.visit(field.Type())
			}
		}
	case *types.Tuple:
		for index := 0; index < value.Len(); index++ {
			r.visit(value.At(index).Type())
		}
	case *types.TypeParam:
		r.visit(value.Constraint())
	case *types.Union:
		for index := 0; index < value.Len(); index++ {
			r.visit(value.Term(index).Type())
		}
	}
}

func visitTypeParameters(reachable *reachableTypes, parameters *types.TypeParamList) {
	if parameters == nil {
		return
	}
	for index := 0; index < parameters.Len(); index++ {
		reachable.visit(parameters.At(index))
	}
}

func (r *reachableTypes) sortedNamed() []*types.Named {
	named := make([]*types.Named, 0, len(r.named))
	for value := range r.named {
		named = append(named, value)
	}
	sort.Slice(named, func(i, j int) bool {
		return named[i].Obj().Name() < named[j].Obj().Name()
	})
	return named
}

func tokenExported(name string) bool {
	return unicode.IsUpper([]rune(name)[0])
}

func typeDeclaration(object *types.TypeName) string {
	if object.IsAlias() {
		if alias, ok := object.Type().(*types.Alias); ok {
			parameters := alias.TypeParams()
			context := newTypeContext(parameters)
			return "alias" + typeParameters(parameters, context) + " = " + canonicalType(alias.Rhs(), context)
		}
		return "alias = " + typeString(types.Unalias(object.Type()))
	}
	named, ok := object.Type().(*types.Named)
	if !ok {
		return "defined " + typeString(object.Type())
	}
	parameters := named.TypeParams()
	context := newTypeContext(parameters)
	return "defined" + typeParameters(parameters, context) + " " + underlyingString(named.Underlying(), context)
}

type typeContext map[*types.TypeParam]string

func newTypeContext(lists ...*types.TypeParamList) typeContext {
	context := make(typeContext)
	for _, parameters := range lists {
		if parameters == nil {
			continue
		}
		for index := 0; index < parameters.Len(); index++ {
			parameter := parameters.At(index)
			context[parameter] = fmt.Sprintf("T%d", len(context))
		}
	}
	return context
}

func extendTypeContext(parent typeContext, lists ...*types.TypeParamList) typeContext {
	context := make(typeContext, len(parent))
	for parameter, name := range parent {
		context[parameter] = name
	}
	for _, parameters := range lists {
		if parameters == nil {
			continue
		}
		for index := 0; index < parameters.Len(); index++ {
			parameter := parameters.At(index)
			if _, exists := context[parameter]; !exists {
				context[parameter] = fmt.Sprintf("T%d", len(context))
			}
		}
	}
	return context
}

func typeParameters(parameters *types.TypeParamList, context typeContext) string {
	if parameters == nil || parameters.Len() == 0 {
		return ""
	}
	parts := make([]string, parameters.Len())
	for index := 0; index < parameters.Len(); index++ {
		parameter := parameters.At(index)
		parts[index] = context[parameter] + " " + canonicalType(parameter.Constraint(), context)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func underlyingString(value types.Type, context typeContext) string {
	switch value := value.(type) {
	case *types.Struct:
		fields := make([]string, 0, value.NumFields())
		for index := 0; index < value.NumFields(); index++ {
			field := value.Field(index)
			if !field.Exported() && !field.Embedded() {
				continue
			}
			part := field.Name() + " " + canonicalType(field.Type(), context)
			if field.Embedded() {
				part = "embedded " + part
			}
			if tag := value.Tag(index); tag != "" {
				part += " " + strconv.Quote(tag)
			}
			fields = append(fields, part)
		}
		return "struct{" + strings.Join(fields, "; ") + "}; comparable=" + strconv.FormatBool(types.Comparable(value))
	case *types.Interface:
		value.Complete()
		methods := make([]string, value.NumMethods())
		for index := 0; index < value.NumMethods(); index++ {
			method := value.Method(index)
			name := method.Name()
			if !method.Exported() && method.Pkg() != nil {
				name = method.Pkg().Path() + "." + name
			}
			methods[index] = name + " " + signatureStringWithContext(method.Type().(*types.Signature), context)
		}
		sort.Strings(methods)
		terms := interfaceTerms(value, context)
		return "interface{methods:[" + strings.Join(methods, "; ") + "]; comparable:" + strconv.FormatBool(value.IsComparable()) + "; terms:[" + strings.Join(terms, "; ") + "]}"
	default:
		return canonicalType(value, context)
	}
}

func interfaceTerms(value *types.Interface, context typeContext) []string {
	terms := make(map[string]bool)
	var collect func(types.Type)
	collect = func(embedded types.Type) {
		embedded = types.Unalias(embedded)
		switch embedded := embedded.(type) {
		case *types.Interface:
			for index := 0; index < embedded.NumEmbeddeds(); index++ {
				collect(embedded.EmbeddedType(index))
			}
		case *types.Named:
			if nested, ok := embedded.Underlying().(*types.Interface); ok {
				collect(nested)
				return
			}
			terms[canonicalType(embedded, context)] = true
		case *types.Union:
			terms[canonicalType(embedded, context)] = true
		default:
			terms[canonicalType(embedded, context)] = true
		}
	}
	for index := 0; index < value.NumEmbeddeds(); index++ {
		collect(value.EmbeddedType(index))
	}
	result := make([]string, 0, len(terms))
	for term := range terms {
		result = append(result, term)
	}
	sort.Strings(result)
	return result
}

func addMethodSets(api map[string]apiEntry, symbol string, typ types.Type) {
	addMethodSet(api, symbol, "value", types.NewMethodSet(typ))
	if _, pointer := typ.(*types.Pointer); !pointer {
		addMethodSet(api, symbol, "pointer", types.NewMethodSet(types.NewPointer(typ)))
	}
}

func addMethodSet(api map[string]apiEntry, symbol, receiver string, set *types.MethodSet) {
	for index := 0; index < set.Len(); index++ {
		selection := set.At(index)
		method, ok := selection.Obj().(*types.Func)
		if !ok || !method.Exported() {
			continue
		}
		key := "method " + receiver + " " + symbol + "." + method.Name()
		api[key] = apiEntry{
			Display: symbol + "." + method.Name() + " [" + receiver + "]",
			Value:   signatureString(method.Type().(*types.Signature)),
		}
	}
}

func signatureString(signature *types.Signature) string {
	return signatureStringWithContext(signature, nil)
}

func signatureStringWithContext(signature *types.Signature, parent typeContext) string {
	context := extendTypeContext(parent, signature.RecvTypeParams(), signature.TypeParams())
	var body strings.Builder
	body.WriteString(typeParameters(signature.TypeParams(), context))
	body.WriteByte('(')
	body.WriteString(tupleString(signature.Params(), signature.Variadic(), context))
	body.WriteString(")->(")
	body.WriteString(tupleString(signature.Results(), false, context))
	body.WriteByte(')')
	return body.String()
}

func tupleString(tuple *types.Tuple, variadic bool, context typeContext) string {
	if tuple == nil || tuple.Len() == 0 {
		return ""
	}
	values := make([]string, tuple.Len())
	for index := 0; index < tuple.Len(); index++ {
		value := tuple.At(index).Type()
		if variadic && index == tuple.Len()-1 {
			if slice, ok := value.(*types.Slice); ok {
				values[index] = "..." + canonicalType(slice.Elem(), context)
				continue
			}
		}
		values[index] = canonicalType(value, context)
	}
	return strings.Join(values, ", ")
}

func typeString(value types.Type) string {
	return canonicalType(value, nil)
}

func canonicalType(value types.Type, context typeContext) string {
	switch value := value.(type) {
	case *types.Alias:
		return canonicalType(value.Rhs(), context)
	case *types.Array:
		return fmt.Sprintf("[%d]%s", value.Len(), canonicalType(value.Elem(), context))
	case *types.Basic:
		if value.Kind() == types.UnsafePointer {
			return "unsafe.Pointer"
		}
		return value.Name()
	case *types.Chan:
		prefix := "chan "
		switch value.Dir() {
		case types.SendOnly:
			prefix = "chan<- "
		case types.RecvOnly:
			prefix = "<-chan "
		}
		return prefix + canonicalType(value.Elem(), context)
	case *types.Interface, *types.Struct:
		return underlyingString(value, context)
	case *types.Map:
		return "map[" + canonicalType(value.Key(), context) + "]" + canonicalType(value.Elem(), context)
	case *types.Named:
		name := value.Obj().Name()
		if pkg := value.Obj().Pkg(); pkg != nil {
			name = pkg.Path() + "." + name
		}
		if arguments := value.TypeArgs(); arguments != nil && arguments.Len() != 0 {
			parts := make([]string, arguments.Len())
			for index := 0; index < arguments.Len(); index++ {
				parts[index] = canonicalType(arguments.At(index), context)
			}
			name += "[" + strings.Join(parts, ", ") + "]"
		}
		return name
	case *types.Pointer:
		return "*" + canonicalType(value.Elem(), context)
	case *types.Signature:
		return "func" + signatureStringWithContext(value, context)
	case *types.Slice:
		return "[]" + canonicalType(value.Elem(), context)
	case *types.Tuple:
		return "(" + tupleString(value, false, context) + ")"
	case *types.TypeParam:
		if name, ok := context[value]; ok {
			return name
		}
		return value.Obj().Name()
	case *types.Union:
		terms := make([]string, value.Len())
		for index := 0; index < value.Len(); index++ {
			term := value.Term(index)
			prefix := ""
			if term.Tilde() {
				prefix = "~"
			}
			terms[index] = prefix + canonicalType(term.Type(), context)
		}
		sort.Strings(terms)
		return strings.Join(terms, " | ")
	}
	return types.TypeString(value, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Path()
	})
}
