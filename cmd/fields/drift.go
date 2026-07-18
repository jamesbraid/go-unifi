package main

import "slices"

// driftIgnoredKeys are controller envelope fields that the schema files
// never carry (the generator adds them to every resource itself). Derived
// from envelopeJSONKeys (main.go), the single source of truth shared with
// the generator's baseType fields.
var driftIgnoredKeys = func() map[string]bool {
	m := make(map[string]bool, len(envelopeJSONKeys))
	for _, k := range envelopeJSONKeys {
		m[k] = true
	}
	return m
}()

type driftResult struct {
	// LiveOnly are fields the live controller emitted that the schema does
	// not define — real drift, the probe's reason to exist.
	LiveOnly []string
	// SchemaOnly are schema fields never observed live — usually just
	// absent-when-unset, so informational.
	SchemaOnly []string
}

// driftCompare unions the top-level keys of the observed objects and
// compares them with the schema definition's top-level keys.
func driftCompare(observed []map[string]any, schema map[string]any) driftResult {
	live := map[string]bool{}
	for _, item := range observed {
		for k := range item {
			if !driftIgnoredKeys[k] {
				live[k] = true
			}
		}
	}

	var r driftResult
	for k := range live {
		if _, ok := schema[k]; !ok {
			r.LiveOnly = append(r.LiveOnly, k)
		}
	}
	for k := range schema {
		if !live[k] {
			r.SchemaOnly = append(r.SchemaOnly, k)
		}
	}
	slices.Sort(r.LiveOnly)
	slices.Sort(r.SchemaOnly)
	return r
}
