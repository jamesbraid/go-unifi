package main

import (
	"bytes"
	"fmt"
	"go/format"
	"maps"
	"slices"
	"sync"

	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

var (
	preferenceTablesOnce sync.Once
	preferenceTablesMap  map[string]map[string]fields.Preference
)

// preferenceTables lazily loads the measured ownership tables: the ownership
// and uos_pins sections of schemas/behavior.json (see fields.LoadPreferences).
func preferenceTables() map[string]map[string]fields.Preference {
	preferenceTablesOnce.Do(func() {
		tables, err := fields.LoadPreferences()
		if err != nil {
			panic(err)
		}
		preferenceTablesMap = tables
	})
	return preferenceTablesMap
}

// generatePreferenceFile renders the measured auto|manual ownership tables
// into a Go map consumers can read at runtime.
//
// The tables are the only record of which fields a controller takes over on
// "auto"; nothing in the schema describes it. Emitting them here means a
// consumer that has to decide the mode -- a Terraform provider settling the
// value at plan time, say -- can read the measured answer instead of keeping
// a hand-copied list that nothing checks.
func generatePreferenceFile(generated map[string]bool) ([]byte, error) {
	tables := preferenceTables()

	var body bytes.Buffer
	for _, resource := range slices.Sorted(maps.Keys(tables)) {
		prefs := tables[resource]
		if len(prefs) == 0 {
			continue
		}
		if !generated[resource] {
			return nil, fmt.Errorf(
				"ownership is recorded for %s, which this run did not generate; the resource left the "+
					"schema, so re-measure schemas/behavior.json (or drop the overrides/fields.toml entry) "+
					"or restore the resource", resource)
		}

		fmt.Fprintf(&body, "\t%q: {\n", resource)
		for _, key := range slices.Sorted(maps.Keys(prefs)) {
			container, mode := splitPreferenceKey(key)
			body.WriteString("\t\t{")
			if container != "" {
				fmt.Fprintf(&body, "Container: %q, ", container)
			}
			fmt.Fprintf(&body, "Mode: %q, Owns: []string{", mode)

			owns := prefs[key].Owns
			excludes := prefs[key].UOSExcludes
			if len(owns) == 0 {
				// A measured empty set is a result: this mode carries the
				// enum and acts on nothing. Rendered explicitly so it reads
				// as measured rather than missing.
				body.WriteString("}},\n")
				continue
			}
			body.WriteString("\n")
			for _, wire := range slices.Sorted(slices.Values(owns)) {
				fmt.Fprintf(&body, "\t\t\t%q,\n", wire)
			}
			body.WriteString("\t\t}")
			// Carried through rather than folded into Owns: a consumer
			// deciding what to send has to know which product it is talking
			// to, and publishing only the standalone answer would have it
			// describe UniFi OS wrongly with no way to tell.
			if len(excludes) > 0 {
				body.WriteString(", UOSExcludes: []string{\n")
				for _, wire := range slices.Sorted(slices.Values(excludes)) {
					fmt.Fprintf(&body, "\t\t\t%q,\n", wire)
				}
				body.WriteString("\t\t}")
			}
			body.WriteString("},\n")
		}
		body.WriteString("\t},\n")
	}

	src := fmt.Appendf(nil, `
// Generated code. DO NOT EDIT.

package unifi

import "slices"

// Preference is one auto|manual mode field and the fields it governs.
type Preference struct {
	// Container is the dotted wire path to the sub-object holding the mode,
	// empty when the mode sits on the resource itself. An array container
	// holds one mode per element, each governing that element.
	Container string

	// Mode is the mode field's wire name, relative to Container.
	Mode string

	// Owns lists the wire names the controller takes over while Mode is
	// "auto", relative to Container -- a mode governs its own object. An
	// empty list is a measured result, not a gap: that mode was probed and
	// owns nothing.
	//
	// This is the standalone UniFi Network answer. Inside UniFi OS, subtract
	// UOSExcludes -- or call OwnsOn, which does it for you.
	Owns []string

	// UOSExcludes lists entries of Owns that do NOT hold inside UniFi OS,
	// because the console owns the field outright and neither mode reaches
	// it. Always a subset of Owns.
	//
	// The Network version does not separate the two products: UniFi OS
	// bundles the same build and reports it, while pinning some fields the
	// standalone controller leaves to manual mode. A consumer that assumes
	// Owns holds everywhere will describe UniFi OS wrongly, so the
	// difference is published rather than folded away.
	UOSExcludes []string
}

// OwnsOn returns the wire names this mode owns on one product.
//
// uos selects the UniFi OS answer, which is Owns minus the fields the console
// pins. Everything else gets Owns unchanged.
func (p Preference) OwnsOn(uos bool) []string {
	if !uos || len(p.UOSExcludes) == 0 {
		return p.Owns
	}
	out := make([]string, 0, len(p.Owns))
	for _, wire := range p.Owns {
		if !slices.Contains(p.UOSExcludes, wire) {
			out = append(out, wire)
		}
	}
	return out
}

// PreferenceOwnedFields records what each auto|manual mode field owns.
//
// A UniFi resource can carry a mode field -- setting_preference and its
// siblings -- that decides whether a block of its own fields is the caller's
// to set or the controller's. While the mode is "auto" the controller stores
// its own values over whatever the payload asked for, answers rc: ok, and
// reports nothing, so a caller learns from the next read or not at all.
//
// The key is the resource's schema name; settings keep their "Setting"
// prefix, so the site NTP document is "SettingNtp".
//
// Measured against a live controller by TestIntegrationPreferenceOwnership
// and recorded in schemas/behavior.json, which also stamps the builds the
// two harnesses were measured on.
var PreferenceOwnedFields = map[string][]Preference{
%s}
`, body.String())

	formatted, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("unable to format the generated preference file: %w", err)
	}
	return formatted, nil
}
