package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
)

// The artifact does not exist until the probes write one, so the committed
// output cannot exercise any of this: with no schemas/behavior.json every
// path below is a no-op and regeneration is a zero-line diff. These tests
// are the only thing standing between the rules and silent deletion --
// dropping the wrapper, the verb override, or the outermost ordering leaves
// the whole suite green otherwise, because nothing measured is baked into
// the files on disk yet.

func TestWithRequiredOnCreate(t *testing.T) {
	required := []string{"protocol", "source_filter"}

	cases := []struct {
		name          string
		field         *FieldInfo
		wantOmitEmpty bool
	}{
		{
			name:          "a required field loses omitempty",
			field:         NewFieldInfo("Protocol", "protocol", "string", "", true, false, false, ""),
			wantOmitEmpty: false,
		},
		{
			// The template couples the pointer to omitempty, so flipping a
			// pointer field turns *T into T -- a breaking Go API change the
			// measurement does not require. Required pointer fields stay in
			// the artifact for consumers; their tags are left alone and the
			// controller's own rejection enforces them at runtime.
			name:          "a required pointer field keeps omitempty and its pointer",
			field:         NewFieldInfo("SourceFilter", "source_filter", "NatSourceFilter", "", true, false, true, ""),
			wantOmitEmpty: true,
		},
		{
			name:          "an unlisted field keeps its omitempty",
			field:         NewFieldInfo("Description", "description", "string", "", true, false, false, ""),
			wantOmitEmpty: true,
		},
		{
			name: "matching is on the wire name, not the Go name",
			// The Go name IS in the required list here; the wire name is
			// not. Matching on the Go name would flip this field.
			field:         NewFieldInfo("protocol", "proto_col", "string", "", true, false, false, ""),
			wantOmitEmpty: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			processor := withRequiredOnCreate(required, nil)
			if err := processor(c.field.FieldName, c.field); err != nil {
				t.Fatalf("processor: %v", err)
			}
			if c.field.OmitEmpty != c.wantOmitEmpty {
				t.Errorf("OmitEmpty = %v, want %v", c.field.OmitEmpty, c.wantOmitEmpty)
			}
		})
	}
}

// The measurement outranks a hand-written processor: the rule runs after and
// undoes an omitempty the processor asserted. Running it first would let the
// guess win over the measurement.
func TestWithRequiredOnCreateRunsAfterTheResourceProcessor(t *testing.T) {
	t.Run("overrides an omitempty the processor set", func(t *testing.T) {
		field := NewFieldInfo("Protocol", "protocol", "string", "", false, false, false, "")
		processor := withRequiredOnCreate([]string{"protocol"}, func(_ string, f *FieldInfo) error {
			f.OmitEmpty = true
			return nil
		})
		if err := processor(field.FieldName, field); err != nil {
			t.Fatal(err)
		}
		if field.OmitEmpty {
			t.Error("OmitEmpty = true; the measured contract lost to the hand-written processor")
		}
	})

	t.Run("an error from the processor stops the rule", func(t *testing.T) {
		field := NewFieldInfo("Protocol", "protocol", "string", "", true, false, false, "")
		processor := withRequiredOnCreate([]string{"protocol"}, func(_ string, _ *FieldInfo) error {
			return errNotMeasured
		})
		if err := processor(field.FieldName, field); !errors.Is(err, errNotMeasured) {
			t.Fatalf("err = %v, want errNotMeasured", err)
		}
		if !field.OmitEmpty {
			t.Error("the rule ran despite the processor failing")
		}
	})
}

func TestApplyWriteContract(t *testing.T) {
	t.Run("a zero contract changes nothing", func(t *testing.T) {
		resource := NewResource("Network", "networkconf")
		before := resource.CreateMethod
		applyWriteContract(resource, behavior.WriteContract{})
		if resource.CreateMethod != before {
			t.Errorf("CreateMethod = %q, want %q", resource.CreateMethod, before)
		}
		field := NewFieldInfo("Name", "name", "string", "", true, false, false, "")
		if err := resource.FieldProcessor(field.FieldName, field); err != nil {
			t.Fatal(err)
		}
		if !field.OmitEmpty {
			t.Error("an unmeasured resource's field lost its omitempty")
		}
	})

	t.Run("a measured PUT create overrides the verb", func(t *testing.T) {
		resource := NewResource("Network", "networkconf")
		applyWriteContract(resource, behavior.WriteContract{CreateVerb: "PUT"})
		if resource.CreateMethod != "PUT" {
			t.Errorf("CreateMethod = %q, want PUT", resource.CreateMethod)
		}
	})

	t.Run("any other measured verb leaves the POST default", func(t *testing.T) {
		for _, verb := range []string{"", "POST", "PATCH"} {
			resource := NewResource("Network", "networkconf")
			applyWriteContract(resource, behavior.WriteContract{CreateVerb: verb})
			if resource.CreateMethod != "POST" {
				t.Errorf("CreateVerb %q: CreateMethod = %q, want POST", verb, resource.CreateMethod)
			}
		}
	})

	t.Run("required-on-create installs the field rule", func(t *testing.T) {
		resource := NewResource("Nat", "nat")
		applyWriteContract(resource, behavior.WriteContract{
			RequiredOnCreate: []string{"protocol"},
		})
		field := NewFieldInfo("Protocol", "protocol", "string", "", true, false, false, "")
		if err := resource.FieldProcessor(field.FieldName, field); err != nil {
			t.Fatal(err)
		}
		if field.OmitEmpty {
			t.Error("a measured required-on-create field kept its omitempty")
		}
	})
}

// createFunc cuts the create method out of generated code, so an assertion
// about its verb cannot be satisfied by the PUT that update always issues.
func createFunc(t *testing.T, code string, structName string) string {
	t.Helper()
	marker := "func (c *ApiClient) create" + structName + "("
	start := strings.Index(code, marker)
	if start < 0 {
		t.Fatalf("generated code has no %s", marker)
	}
	rest := code[start+len(marker):]
	end := strings.Index(rest, "\nfunc ")
	if end < 0 {
		end = len(rest)
	}
	return rest[:end]
}

// The template must consume CreateMethod: defaulting to POST and switching
// to PUT only when the measured contract says so. Hardcoding either verb
// back into the template fails one of these.
func TestGeneratedCreateVerb(t *testing.T) {
	t.Run("defaults to POST", func(t *testing.T) {
		resource := NewResource("Network", "networkconf")
		code, err := resource.generateCode(false)
		if err != nil {
			t.Fatal(err)
		}
		create := createFunc(t, code, "Network")
		if !strings.Contains(create, "http.MethodPost") {
			t.Error("create does not issue POST by default")
		}
		if strings.Contains(create, "http.MethodPut") {
			t.Error("create issues PUT without a measured contract")
		}
	})

	t.Run("a measured PUT contract switches the verb", func(t *testing.T) {
		resource := NewResource("Network", "networkconf")
		resource.CreateMethod = "PUT"
		code, err := resource.generateCode(false)
		if err != nil {
			t.Fatal(err)
		}
		create := createFunc(t, code, "Network")
		if !strings.Contains(create, "http.MethodPut") {
			t.Error("create ignores the measured PUT contract")
		}
		if strings.Contains(create, "http.MethodPost") {
			t.Error("create still issues POST despite the measured contract")
		}
	})
}

// A repo without schemas/behavior.json -- this one, today -- must generate
// exactly what it generated before the artifact existed: default verbs,
// untouched tags, and a coercion map that is present but empty.
func TestMissingArtifactDegradesToNoOp(t *testing.T) {
	measured, found, err := behavior.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if found {
		t.Fatal("found reported true for a missing artifact")
	}

	resource := NewResource("Network", "networkconf")
	applyWriteContract(resource, measured.Writes[resource.StructName])
	if resource.CreateMethod != "POST" {
		t.Errorf("CreateMethod = %q, want POST", resource.CreateMethod)
	}
	field := NewFieldInfo("Name", "name", "string", "", true, false, false, "")
	if err := resource.FieldProcessor(field.FieldName, field); err != nil {
		t.Fatal(err)
	}
	if !field.OmitEmpty {
		t.Error("a field lost its omitempty with nothing measured")
	}

	rendered, err := renderCoercionsFile("unifi", measured.Coercions)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered), "var FieldCoercionFloors = map[string]map[string]string{}") {
		t.Errorf("empty artifact did not render an empty map:\n%s", rendered)
	}
}

func TestRenderCoercionsFile(t *testing.T) {
	coercions := map[string]map[string]behavior.Coercion{
		"SettingUsg": {
			"other_timeout": {Wrote: "0", Stored: "600"},
			"icmp_timeout":  {Wrote: "1", Stored: "30"},
		},
		"Network": {
			"dhcpd_leasetime": {Wrote: "1", Stored: "120"},
		},
	}

	rendered, err := renderCoercionsFile("unifi", coercions)
	if err != nil {
		t.Fatal(err)
	}
	got := string(rendered)

	// The floor is what the controller stored, not what the probe wrote.
	for _, want := range []string{
		"// Code generated by go-unifi. DO NOT EDIT.",
		"package unifi",
		`"SettingUsg": {`,
		`"icmp_timeout":  "30",`,
		`"other_timeout": "600",`,
		`"dhcpd_leasetime": "120",`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered file is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"1"`) || strings.Contains(got, `"0"`) {
		t.Error("rendered file carries a written probe value where a stored floor belongs")
	}

	// Map iteration order must not leak into the output: a re-measure that
	// found nothing new has to be a zero-line diff.
	for range 5 {
		again, err := renderCoercionsFile("unifi", coercions)
		if err != nil {
			t.Fatal(err)
		}
		if string(again) != got {
			t.Fatal("renderCoercionsFile is not deterministic")
		}
	}
}

var errNotMeasured = errors.New("not measured")
