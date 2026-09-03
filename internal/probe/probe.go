// Package probe holds the shared write-then-reread machinery the controller
// behaviour probes use. It exists because four integration tests -- the field
// probe, the preference-ownership sweep, the clearing-semantics probe and the
// round-trip discard check -- each grew their own copy of the same three
// steps: write a document, read back what the controller stored, and classify
// each field as kept, changed or dropped. This is that, once.
//
// It carries no controller-start or session logic; callers pass an already
// logged-in Session (from internal/controllertest). It is deliberately test-
// and capture-only: nothing in the shipped SDK imports it.
package probe

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Verdict is how the controller treated one field between what was asked and
// what it stored.
type Verdict int

const (
	// Kept: the stored value equals what was asked (JSON-normalized).
	Kept Verdict = iota
	// Changed: the field is present but the controller stored something else
	// -- a coercion, a floor, a normalization.
	Changed
	// Dropped: the field is absent from the stored document. The controller
	// accepted the write and did not persist it.
	Dropped
)

func (v Verdict) String() string {
	switch v {
	case Kept:
		return "KEPT"
	case Changed:
		return "CHANGED"
	case Dropped:
		return "DROPPED"
	default:
		return "?"
	}
}

// FieldResult is one field's verdict with a human-readable before/after.
type FieldResult struct {
	Wire    string
	Verdict Verdict
	Detail  string
}

// Classify compares what was asked against what the controller stored, one
// entry per asked field. A field kept as asked is omitted from the result:
// callers care about what moved.
//
// This is the merge of discardedFields (round-trip) and the PERSISTED/
// MUTATED/STRIPPED switch (field probe) -- the same computation both did.
func Classify(asked, stored map[string]any) []FieldResult {
	var out []FieldResult
	for wire, want := range asked {
		have, present := stored[wire]
		switch {
		case !present:
			out = append(out, FieldResult{wire, Dropped, describe(want, nil)})
		case JSONEqual(want, have):
			// kept -- omit
		default:
			out = append(out, FieldResult{wire, Changed, describe(want, have)})
		}
	}
	return out
}

// describe renders a before/after a reader can act on. The controller returns
// some numeric fields as JSON strings, so a raw "%v differs" can read as a
// bug in the comparison when only the representation differs; say so when the
// rendered forms match.
func describe(want, have any) string {
	w := fmt.Sprintf("%v", want)
	h := fmt.Sprintf("%v", have)
	if w == h {
		return fmt.Sprintf("asked %s (%T), stored the same value as %T", w, want, have)
	}
	return fmt.Sprintf("asked %s, stored %s", w, h)
}

// JSONEqual compares two values by round-tripping both through JSON and
// comparing with reflect.DeepEqual. A Go struct/slice never DeepEquals the
// map[string]any the controller hands back, and fmt formatting hides type and
// order differences the wire does not preserve; normalizing both sides to the
// JSON shape (map[string]any, []any, float64) compares what actually crossed
// the wire. int-vs-float differences become equal, which is the intended
// semantic: did the controller keep the value.
func JSONEqual(a, b any) bool {
	na, err := normalize(a)
	if err != nil {
		return false
	}
	nb, err := normalize(b)
	if err != nil {
		return false
	}
	return reflect.DeepEqual(na, nb)
}

func normalize(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	return out, json.Unmarshal(raw, &out)
}

// FirstData pulls the first object out of a decoded controller response,
// tolerating the v1 {"data":[...]} envelope, a bare object, and a bare array.
func FirstData(body any) map[string]any {
	switch v := body.(type) {
	case map[string]any:
		if data, ok := v["data"].([]any); ok {
			if len(data) > 0 {
				m, _ := data[0].(map[string]any)
				return m
			}
			return nil
		}
		return v
	case []any:
		if len(v) > 0 {
			m, _ := v[0].(map[string]any)
			return m
		}
	}
	return nil
}
