package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
	required := []string{"protocol", "source_filter", "source.zone_id"}

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
		{
			// The artifact spells a nested field "source.zone_id"; the
			// processor sees the bare leaf.
			name:          "a dotted entry matches its leaf field",
			field:         NewFieldInfo("ZoneID", "zone_id", "string", "", true, false, false, ""),
			wantOmitEmpty: false,
		},
		{
			// Without omitempty a nil slice marshals as null, which no
			// probe has measured; a required array keeps its tag.
			name:          "a required array field keeps omitempty",
			field:         NewFieldInfo("Protocol", "protocol", "string", "", true, true, false, ""),
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
func createFunc(t *testing.T, code string, resource *ResourceInfo) string {
	t.Helper()
	marker := "func (c *ApiClient) " + resource.Method("Create") + "("
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
		code, err := resource.generateCode("network.generated.go")
		if err != nil {
			t.Fatal(err)
		}
		create := createFunc(t, code, resource)
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
		code, err := resource.generateCode("network.generated.go")
		if err != nil {
			t.Fatal(err)
		}
		create := createFunc(t, code, resource)
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

func TestCreatePathSegment(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"v2/api/site/{site}/content-filtering/create", "content-filtering/create"},
		{"v2/api/site/{site}/nat", "nat"},
		{"v2/api/site/{site}/ospf/router", "ospf/router"},
		{"api/s/{site}/rest/hotspotpackage", "hotspotpackage"},
		// Neither shape: an unmeasured resource, and a verdict the old
		// artifact recorded in the path's place. Both must change nothing.
		{"", ""},
		{"POST-REJECTED-405", ""},
		{"api/s/{site}/stat/device", ""},
	} {
		if got := createPathSegment(tc.path); got != tc.want {
			t.Errorf("createPathSegment(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// A create endpoint that is not the collection has to reach the generated
// client, or the SDK keeps writing to a path the controller answers 405 to
// -- which is exactly what shipped for content filtering.
func TestGeneratedCreatePath(t *testing.T) {
	t.Run("defaults to the resource path", func(t *testing.T) {
		resource := NewResource("ContentFiltering", "content-filtering")
		applyWriteContract(resource, behavior.WriteContract{})
		code, err := resource.generateCode("content_filtering.generated.go")
		if err != nil {
			t.Fatal(err)
		}
		create := createFunc(t, code, resource)
		if !strings.Contains(create, `v2/api/site/%s/content-filtering"`) {
			t.Errorf("create does not post to the collection by default:\n%s", create)
		}
	})

	t.Run("a measured sub-path moves the create", func(t *testing.T) {
		resource := NewResource("ContentFiltering", "content-filtering")
		applyWriteContract(resource, behavior.WriteContract{
			CreateVerb: "POST", CreatePath: "v2/api/site/{site}/content-filtering/create",
		})
		code, err := resource.generateCode("content_filtering.generated.go")
		if err != nil {
			t.Fatal(err)
		}
		create := createFunc(t, code, resource)
		if !strings.Contains(create, `v2/api/site/%s/content-filtering/create"`) {
			t.Errorf("create ignores the measured create path:\n%s", create)
		}
		// Only the create moves: update and delete stay on the collection's
		// own paths, which the same measurement recorded separately.
		if strings.Contains(code, `v2/api/site/%s/content-filtering/create/%s"`) {
			t.Error("the create sub-path leaked into a by-id path")
		}
	})
}

// retiredV2List is the literal slice IsV2 used to return, kept here word for
// word so TestIsV2MatchesRetiredList has something fixed to check the
// derivation against. It must never be updated to make a test pass -- if
// isV2 disagrees with it, either isV2 is wrong or a name here has actually
// changed API generation, and either way that is the finding, not a reason
// to edit this list.
var retiredV2List = []string{
	"APGroup",
	"BGPConfig",
	"ContentFiltering",
	"DNSRecord",
	"FirewallPolicy",
	"FirewallZone",
	"Nat",
	"NetworkMembersGroup",
	"OSPFRouter",
	"TrafficRoute",
}

// TestIsV2MatchesRetiredList is the proof that swapping the hardcoded slice
// for a measured derivation did not move any generated output: for every
// resource the real schemas/behavior.json artifact has a write contract
// for, plus every name the retired list named, the derivation must agree
// with what the literal slice used to answer. Today that is every name in
// both sets -- schemas/behavior.json now measures a create path for all ten
// retired names, and for none of them does it disagree.
//
// This is the one check in this file that reads the actual artifact
// checked into the repo rather than a synthetic one, because the claim
// being proven is specifically about today's real schemas/behavior.json.
func TestIsV2MatchesRetiredList(t *testing.T) {
	retired := make(map[string]bool, len(retiredV2List))
	for _, name := range retiredV2List {
		retired[name] = true
	}

	names := map[string]bool{}
	for name := range retired {
		names[name] = true
	}
	for name := range writeContracts() {
		names[name] = true
	}
	if len(names) == 0 {
		t.Fatal("no resources to check -- writeContracts() and retiredV2List are both empty")
	}

	for name := range names {
		t.Run(name, func(t *testing.T) {
			got := isV2(name, writeContracts()[name])
			if want := retired[name]; got != want {
				t.Errorf("isV2(%q) = %v, want %v (the retired list's answer)", name, got, want)
			}
		})
	}
}

// TestV2MustMeasureIsFullyMeasured proves the loud path in isV2 has nothing
// to catch today: every resource v2MustMeasure names has a measured create
// path in the real artifact, and it is a v2 path. If either goes false for
// some future name, isV2 itself would already panic during generation --
// this test just says so on its own terms, without needing `go generate` to
// find it first.
func TestV2MustMeasureIsFullyMeasured(t *testing.T) {
	contracts := writeContracts()
	for name := range v2MustMeasure {
		t.Run(name, func(t *testing.T) {
			w, ok := contracts[name]
			if !ok || w.CreatePath == "" {
				t.Fatalf("%s is in v2MustMeasure but schemas/behavior.json has no measured create path for it", name)
			}
			if !strings.HasPrefix(w.CreatePath, v2CreatePathPrefix) {
				t.Fatalf("%s is in v2MustMeasure but its measured create path %q is not v2", name, w.CreatePath)
			}
		})
	}
}

// TestIsV2AgreesWithMeasuredPath is the guard the hardcoded list never had:
// it walks every resource the real artifact has actually measured a create
// path for and checks isV2's answer against the path's own prefix,
// recomputed independently of isV2's internals. A future edit that makes
// isV2 stop reading the path correctly fails here even if
// TestIsV2MatchesRetiredList still passes, because that test's "no measured
// path" branches do not exercise this comparison.
func TestIsV2AgreesWithMeasuredPath(t *testing.T) {
	contracts := writeContracts()
	measuredAny := false
	for name, w := range contracts {
		if w.CreatePath == "" {
			continue
		}
		measuredAny = true
		t.Run(name, func(t *testing.T) {
			want := strings.HasPrefix(w.CreatePath, "v2/")
			var got bool
			require.NotPanics(t, func() {
				got = isV2(name, w)
			}, "a measured, recognized create path must not panic")
			if got != want {
				t.Errorf("isV2(%q) = %v for create_path %q, want %v", name, got, w.CreatePath, want)
			}
		})
	}
	if !measuredAny {
		t.Fatal("no resource in schemas/behavior.json has a measured create_path -- this test is not checking anything")
	}
}

// TestIsV2 exercises the derivation directly against synthetic contracts,
// independent of whatever schemas/behavior.json happens to hold today.
func TestIsV2(t *testing.T) {
	t.Run("a v2 measured path is v2", func(t *testing.T) {
		require.True(t, isV2("Whatever", behavior.WriteContract{CreatePath: "v2/api/site/{site}/whatever"}))
	})

	t.Run("a v1 REST measured path is not v2", func(t *testing.T) {
		require.False(t, isV2("Whatever", behavior.WriteContract{CreatePath: "api/s/{site}/rest/whatever"}))
	})

	t.Run("no measured path defaults to v1, same as not being in the old list", func(t *testing.T) {
		require.False(t, isV2("Network", behavior.WriteContract{}))
	})

	t.Run("a must-measure resource with no measured path panics rather than fall back to v1", func(t *testing.T) {
		require.Panics(t, func() {
			isV2("APGroup", behavior.WriteContract{})
		})
	})

	t.Run("a must-measure resource measuring out as v1 REST panics rather than flip in silence", func(t *testing.T) {
		require.Panics(t, func() {
			isV2("APGroup", behavior.WriteContract{CreatePath: "api/s/{site}/rest/apgroup"})
		})
	})

	t.Run("an unrecognized path shape panics rather than guess", func(t *testing.T) {
		require.Panics(t, func() {
			isV2("Whatever", behavior.WriteContract{CreatePath: "POST-REJECTED-405"})
		})
	})
}

// TestIsV2ReadsTheRealArtifact locks IsV2 (the template-facing method) to
// the writeContracts() global instead of some other, disconnected source --
// the exact wiring mistake that would leave the method still returning the
// pre-refactor answers for the wrong reason.
func TestIsV2ReadsTheRealArtifact(t *testing.T) {
	withWriteContracts(t, map[string]behavior.WriteContract{
		"Thing": {CreatePath: "v2/api/site/{site}/things"},
	}, func() {
		r := NewResource("Thing", "things")
		if !r.IsV2() {
			t.Error("IsV2() = false for a resource with a v2 measured create path")
		}
	})

	withWriteContracts(t, map[string]behavior.WriteContract{
		"Thing": {CreatePath: "api/s/{site}/rest/thing"},
	}, func() {
		r := NewResource("Thing", "thing")
		if r.IsV2() {
			t.Error("IsV2() = true for a resource with a v1 REST measured create path")
		}
	})
}

var errNotMeasured = errors.New("not measured")
