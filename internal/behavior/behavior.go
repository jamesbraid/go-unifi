// Package behavior holds the measured-behaviour artifact: what the pinned
// controller actually does, recorded by probes at capture time and versioned
// beside the field definitions it complements.
//
// The field definitions say what the controller declares; this says what it
// was measured doing -- which fields a preference mode silently owns, which
// writes it accepts and discards, and whether an absent key and an empty
// string mean the same thing. Every entry here used to be a hand-pasted
// baseline inside an integration test, stamped with a controller version and
// forgotten until it went red. The probes write this file, the tests read it,
// and a controller bump re-measures it into a reviewable diff.
package behavior

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Artifact is the whole measured-behaviour record for one controller version.
// Maps are keyed for stable diffs; MarshalJSON is not needed because encoding/
// json sorts map keys, and the writer indents.
type Artifact struct {
	// ControllerVersion is the version the probes ran against, so a stale
	// artifact is obvious in the diff rather than silently trusted.
	ControllerVersion string `json:"controller_version"`

	// Ownership: per resource, per preference mode field, the wire names the
	// controller silently manages when that mode is set -- accepted on write,
	// stored as the controller's own value, never reported. Replaces the
	// hand-pasted "owns" blocks in overrides/fields.toml.
	Ownership map[string]map[string][]string `json:"ownership,omitempty"`

	// Discarded: per resource, wire names the controller accepts on create
	// and does not store -- the round-trip probe's finding. Replaces the
	// wantDiscarded baselines.
	Discarded map[string][]string `json:"discarded,omitempty"`

	// Empty: per resource, per field, how the controller treats an empty
	// string versus an absent key, in the clearing probe's vocabulary.
	Empty map[string]map[string]EmptySemantics `json:"empty,omitempty"`

	// Coercions: per resource, per field, a below-range value that was
	// written and the value the controller floored/clamped it to -- the
	// conntrack timeout floors and their kin.
	Coercions map[string]map[string]Coercion `json:"coercions,omitempty"`

	// Writes: per resource, the measured write contract -- the verb and path
	// the controller actually accepts, and which fields must be present on
	// create and on update. Replaces the codegen's guess-from-shape.
	Writes map[string]WriteContract `json:"writes,omitempty"`
}

// EmptySemantics records what a field did when written as "" and when its key
// was omitted (EMPTY-REJECTED / EMPTY-CLEARS / EMPTY-IGNORED, OMIT-CLEARS /
// OMIT-KEEPS / OMIT-REJECTED).
type EmptySemantics struct {
	Empty string `json:"empty"`
	Omit  string `json:"omit"`
}

// Coercion is a written value the controller refused to store verbatim, with
// what it stored instead.
type Coercion struct {
	Wrote  string `json:"wrote"`
	Stored string `json:"stored"`
}

// WriteContract is the measured verb/path and required-field sets for a
// resource's create and update.
type WriteContract struct {
	CreateVerb       string   `json:"create_verb"`
	CreatePath       string   `json:"create_path"`
	UpdateVerb       string   `json:"update_verb"`
	UpdatePath       string   `json:"update_path"`
	RequiredOnCreate []string `json:"required_on_create,omitempty"`
	RequiredOnUpdate []string `json:"required_on_update,omitempty"`
}

// Path is the artifact's location relative to the module root.
const Path = "schemas/behavior.json"

// Load reads the artifact from root. A missing file is not an error -- it
// returns a zero Artifact and false, so a consumer that predates the artifact
// degrades to "nothing measured" rather than failing.
func Load(root string) (Artifact, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, Path))
	if os.IsNotExist(err) {
		return Artifact{}, false, nil
	}
	if err != nil {
		return Artifact{}, false, err
	}
	var a Artifact
	if err := json.Unmarshal(raw, &a); err != nil {
		return Artifact{}, false, fmt.Errorf("parse %s: %w", Path, err)
	}
	return a, true, nil
}

// Write renders the artifact to root deterministically: sorted keys (via
// encoding/json), two-space indent, trailing newline, so a re-measure that
// found nothing new is a zero-line diff.
func Write(root string, a Artifact) error {
	// sort slice values so an unordered probe result still diffs stably.
	for _, byField := range a.Ownership {
		for k := range byField {
			sort.Strings(byField[k])
		}
	}
	for k := range a.Discarded {
		sort.Strings(a.Discarded[k])
	}
	for _, w := range a.Writes {
		sort.Strings(w.RequiredOnCreate)
		sort.Strings(w.RequiredOnUpdate)
	}
	raw, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(filepath.Join(root, Path), raw, 0o644)
}
