package main

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ubiquiti-community/go-unifi/wirecontract"
)

// TestAddPackageReadsTheRealTree is the positive control. A walker that silently
// found nothing would still write a well-formed artifact, and every check a
// consumer builds on it would pass against an empty one.
func TestAddPackageReadsTheRealTree(t *testing.T) {
	contract := wirecontract.Contract{Types: map[string]wirecontract.Type{}}
	for _, pkg := range []string{"unifi", "unifi/settings"} {
		if err := addPackage(&contract, "../..", pkg); err != nil {
			t.Fatalf("addPackage(%s): %v", pkg, err)
		}
	}
	if len(contract.Types) < 100 {
		t.Fatalf("scanned %d types; the walker is not reading the tree", len(contract.Types))
	}
	if network := contract.Types["unifi.Network"]; len(network.Fields) < 50 ||
		!slices.Contains(network.APIPaths, "networkconf") {
		t.Errorf("unifi.Network has %d fields and paths %q", len(network.Fields), network.APIPaths)
	}
	// Three type names are declared in both packages, which is why keys carry one:
	// unifi.Dashboard is a site dashboard document, unifi/settings.Dashboard is the
	// dashboard setting.
	root, setting := contract.Types["unifi.Dashboard"], contract.Types["unifi/settings.Dashboard"]
	if root.SchemaResource != "Dashboard" || setting.SchemaResource != "SettingDashboard" {
		t.Errorf("the two Dashboards resolve to %q and %q", root.SchemaResource, setting.SchemaResource)
	}
	// Hand-written, embedded by every settings document, and on every settings
	// write -- the case a scan of the generated files alone misses.
	base := contract.Types["unifi/settings.BaseSetting"]
	key := slices.IndexFunc(base.Fields, func(f wirecontract.Field) bool { return f.Wire == "key" })
	if key < 0 || base.Fields[key].OmitEmpty {
		t.Errorf("settings.BaseSetting.key = %+v, want recorded with omit_empty false", base.Fields)
	}
}

// TestGoTypeIsRecordedVerbatim pins what a consumer reads pointer-ness off.
// *int64 and int64 differ in whether the caller can leave a field unset, and
// **Elem, []Elem and []*Elem are three different shapes for a nested object.
func TestGoTypeIsRecordedVerbatim(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := strings.ReplaceAll(`package pkg

type Shapes struct {
	Scalar   int64   'json:"scalar"'
	Pointer  *int64  'json:"pointer,omitempty"'
	Double   **Elem  'json:"double"'
	List     []Elem  'json:"list"'
	Pointers []*Elem 'json:"pointers"'
}
`, "'", "`")
	if err := os.WriteFile(filepath.Join(root, "pkg", "shapes.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	contract := wirecontract.Contract{Types: map[string]wirecontract.Type{}}
	if err := addPackage(&contract, root, "pkg"); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, field := range contract.Types["pkg.Shapes"].Fields {
		got[field.Wire] = field.GoType
	}
	want := map[string]string{
		"scalar": "int64", "pointer": "*int64", "double": "**Elem",
		"list": "[]Elem", "pointers": "[]*Elem",
	}
	if !maps.Equal(got, want) {
		t.Errorf("go types = %v, want %v", got, want)
	}
}
