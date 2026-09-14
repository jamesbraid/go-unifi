package main

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"sync"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

// The measured-behaviour artifact (schemas/behavior.json) is a generator
// input like the validators and sensitive_metadata.json already are: the
// probes record what the pinned controller actually does, and the generator
// turns that record into code. A missing artifact means nothing has been
// measured yet, and every consumer here degrades to a no-op -- the same
// posture loadSensitiveMetadata takes for its file.

// applyWriteContract turns one resource's measured write contract into
// generator decisions. A zero contract -- no artifact, or a resource the
// probes have not measured -- changes nothing.
func applyWriteContract(r *ResourceInfo, w behavior.WriteContract) {
	if len(w.RequiredOnCreate) > 0 {
		r.FieldProcessor = withRequiredOnCreate(w.RequiredOnCreate, r.FieldProcessor)
	}
	// Only PUT is a measured deviation the template knows how to emit;
	// POST (or an empty verb) is the default already.
	if w.CreateVerb == "PUT" {
		r.CreateMethod = "PUT"
	}
	if segment := createPathSegment(w.CreatePath); segment != "" {
		r.CreateResourcePath = segment
	}
}

// createPathSegment reduces a measured create path to the part the template
// renders after the site, so a resource whose create endpoint is not its
// collection is generated against what the probe measured rather than
// against the collection-POST convention the other resources happen to
// share. Content filtering is that resource: the controller maps the
// collection for GET alone and serves creates from a /create sub-path.
//
// Both wire prefixes are recognised because the artifact records the whole
// path a probe used, and v1 and v2 resources spell it differently. A path
// in neither shape returns "" and changes nothing: the artifact is a
// measurement, and a shape this cannot read is one nobody has taught the
// template to emit.
func createPathSegment(path string) string {
	for _, prefix := range []string{"v2/api/site/{site}/", "api/s/{site}/rest/"} {
		if rest, ok := strings.CutPrefix(path, prefix); ok {
			return rest
		}
	}
	return ""
}

var (
	writeContractsOnce sync.Once
	writeContractsMap  map[string]behavior.WriteContract
)

// writeContracts lazily loads the writes section of schemas/behavior.json,
// the same way preferenceTables loads the ownership sections: found via
// fields.ModuleRoot, cached for the process. A missing artifact degrades to
// an empty map, same as everywhere else the artifact is read.
func writeContracts() map[string]behavior.WriteContract {
	writeContractsOnce.Do(func() {
		root := fields.ModuleRoot()
		if root == "" {
			panic("unable to locate the module root (go.mod) for schemas/behavior.json")
		}
		artifact, _, err := behavior.Load(root)
		if err != nil {
			panic(err)
		}
		writeContractsMap = artifact.Writes
	})
	return writeContractsMap
}

// v2 and v1 REST create paths are spelled with these prefixes; see the
// create_path values in schemas/behavior.json.
const (
	v2CreatePathPrefix     = "v2/"
	v1RESTCreatePathPrefix = "api/s/"
)

// v2MustMeasure names every StructName the hardcoded slice IsV2 used to
// return true for, literally, before this file replaced it. isV2 does not
// consult this map to decide its answer -- the measured create path always
// does that -- it exists only so a regression is loud: if any of these ten
// ever shows up with no measured create path, or with one that has moved to
// v1 REST, that is a capture that stopped covering it, an artifact rebuilt
// without a batch, or a rename, and generating the wrong client for it in
// silence is worse than a generator that refuses to run and says which
// resource broke.
//
// Confirmed against schemas/behavior.json: all ten measure with a "v2/"
// create path today, and no other resource in the artifact does. This map
// is expected to stay exactly this size; growing it back toward "every v2
// resource" would just be the old hardcoded list wearing a panic.
var v2MustMeasure = map[string]bool{
	"APGroup":             true,
	"BGPConfig":           true,
	"ContentFiltering":    true,
	"DNSRecord":           true,
	"FirewallPolicy":      true,
	"FirewallZone":        true,
	"Nat":                 true,
	"NetworkMembersGroup": true,
	"OSPFRouter":          true,
	"TrafficRoute":        true,
}

// isV2 reports whether structName's generated client targets the v2 API
// surface, decided entirely by the measured create path: a "v2/" path is
// v2, an "api/s/" path is v1 REST, and a resource with no measured contract
// at all is an ordinary v1 resource nobody has ever flagged otherwise --
// the same answer "not in the list" always meant.
//
// The one case that must never resolve that way in silence is one of the
// ten resources v2MustMeasure names: those used to be true unconditionally,
// and a derivation that quietly let one fall back to v1 because its
// measurement went missing would generate the wrong client without saying
// so. isV2 panics naming the resource instead, and the same for a measured
// path that matches neither known shape -- guessing there is exactly the
// failure mode this artifact exists to replace.
func isV2(structName string, w behavior.WriteContract) bool {
	switch {
	case strings.HasPrefix(w.CreatePath, v2CreatePathPrefix):
		return true
	case strings.HasPrefix(w.CreatePath, v1RESTCreatePathPrefix):
		if v2MustMeasure[structName] {
			panic(fmt.Sprintf(
				"%s is required to measure as v2 but its create path %q is v1 REST -- "+
					"either it really moved to v1 REST (update v2MustMeasure to say so) "+
					"or the measurement regressed",
				structName, w.CreatePath))
		}
		return false
	case w.CreatePath == "":
		if v2MustMeasure[structName] {
			panic(fmt.Sprintf(
				"%s is required to measure as v2 but has no measured create path in "+
					"schemas/behavior.json -- measure it before generating, do not let it "+
					"fall back to v1 in silence",
				structName))
		}
		return false
	default:
		panic(fmt.Sprintf(
			"%s has a measured create path %q that is neither v2 (%q) nor v1 REST (%q) -- "+
				"teach isV2 the new shape instead of guessing",
			structName, w.CreatePath, v2CreatePathPrefix, v1RESTCreatePathPrefix))
	}
}

// withRequiredOnCreate drops the omitempty tag from the fields the
// controller was measured to require on create, so their zero values reach
// the wire instead of vanishing from the request.
//
// It wraps outermost and matches on the wire name, because the measurement
// is authoritative: a hand-written processor's omitempty guess retires the
// day the probe records the field as required.
//
// A dotted artifact entry ("source.zone_id") matches by its leaf, because
// the processor sees each field without its parent. Leaf matching flips
// every same-named field in the resource, which is exact today: the
// measured entries name all of them.
func withRequiredOnCreate(required []string, next func(string, *FieldInfo) error) func(string, *FieldInfo) error {
	names := make(map[string]bool, len(required))
	for _, n := range required {
		if i := strings.LastIndex(n, "."); i >= 0 {
			n = n[i+1:]
		}
		names[n] = true
	}
	return func(name string, f *FieldInfo) error {
		if next != nil {
			if err := next(name, f); err != nil {
				return err
			}
		}
		// Pointer fields are left alone: the template couples the pointer
		// to omitempty, so flipping one turns *T into T -- a breaking Go
		// API change the measurement does not require. The artifact still
		// records the field as required; consumers derive requiredness
		// from it, and an SDK caller omitting a required pointer gets the
		// controller's own rejection, same as today.
		//
		// Array fields are left alone too: without omitempty a nil slice
		// marshals as null, a wire shape no probe has measured.
		if names[f.JSONName] && !f.IsPointer && !f.IsArray {
			f.OmitEmpty = false
		}
		return nil
	}
}

// renderCoercionsFile emits the measured coercion floors as consumable Go,
// so a caller can know before writing that the controller will silently
// rewrite a value -- the conntrack timeout floors and their kin. An empty
// artifact still emits the file, with an empty map, so the output set is
// stable whether or not anything has been measured.
func renderCoercionsFile(pkg string, coercions map[string]map[string]behavior.Coercion) ([]byte, error) {
	resources := make([]string, 0, len(coercions))
	for r := range coercions {
		resources = append(resources, r)
	}
	sort.Strings(resources)

	var b bytes.Buffer
	fmt.Fprintf(&b, `// Code generated by go-unifi. DO NOT EDIT.

package %s

// FieldCoercionFloors records, per resource and wire field name, the value
// the controller stored when a probe wrote a below-range value -- measured
// floors and clamps from schemas/behavior.json, not guesses. A field listed
// here is one the controller rewrites in silence: what was written is not
// what a read returns, and nothing in the response says so.
var FieldCoercionFloors = map[string]map[string]string{`, pkg)
	if len(resources) == 0 {
		b.WriteString("}\n")
		return format.Source(b.Bytes())
	}
	b.WriteString("\n")
	for _, r := range resources {
		fields := make([]string, 0, len(coercions[r]))
		for f := range coercions[r] {
			fields = append(fields, f)
		}
		sort.Strings(fields)
		fmt.Fprintf(&b, "\t%q: {\n", r)
		for _, f := range fields {
			fmt.Fprintf(&b, "\t\t%q: %q,\n", f, coercions[r][f].Stored)
		}
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}
