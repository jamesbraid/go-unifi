// Package wirecontract publishes what this client puts on the wire: per type and
// field, the wire name, the Go declaration and whether an unset value is omitted,
// plus the controller's validation rules and the behaviour measured against the
// pinned controller.
//
// None of it is visible to the Go type checker. Struct tags are not API, so a
// field that loses omitempty still compiles and simply starts asserting a zero
// value on every write, which the controller stores. A consumer that models a
// resource from partial input -- a Terraform provider filling a struct from a
// plan -- needs the whole set to know which keys its writes assert.
//
// Types and Behavior are keyed package-qualified ("unifi.Network",
// "unifi/settings.Dashboard"): three type names are declared in both packages and
// each pair is a different wire object. Constraints and Behavior are keyed on a
// type's SchemaResource instead, which is what the controller calls the resource.
//
// The artifact is regenerated from the finished tree and never edited.
package wirecontract

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:generate go run ../cmd/wirecontract/

// FormatVersion is the layout version of the embedded artifact. A reader that
// finds a version it does not know is reading a different shape.
const FormatVersion = 1

// Path is the artifact's location relative to the module root.
const Path = "wirecontract/wire_contract.json"

// JSON is the artifact as published, for a consumer that would rather read it
// than import these types.
//
//go:embed wire_contract.json
var JSON []byte

// Contract is the whole artifact.
//
// Constraints holds each package's generated FieldConstraints table as that
// table's own JSON -- resource, then wire name, then the rule -- re-serialized
// rather than recomputed, so what is published and what the SDK enforces cannot
// drift; unmarshal an entry into map[string]map[string]unifi.FieldConstraint, the
// type the SDK exports for exactly this. Behavior holds a type's probe results
// under the section names schemas/behavior.json uses, each section's value
// carried verbatim.
type Contract struct {
	FormatVersion int                                   `json:"format_version"`
	Provenance    Provenance                            `json:"provenance"`
	Types         map[string]Type                       `json:"types"`
	Constraints   map[string]json.RawMessage            `json:"constraints"`
	Behavior      map[string]map[string]json.RawMessage `json:"behavior,omitempty"`
}

// Provenance names the controller the surface came from and, separately, the one
// the behaviour was measured on: a tree re-captured without a re-probe is visibly
// mismatched rather than silently trusted.
type Provenance struct {
	ControllerVersion         string `json:"controller_version"`
	ControllerBuild           string `json:"controller_build"`
	CaptureSHA256             string `json:"capture_sha256"`
	BehaviorControllerVersion string `json:"behavior_controller_version"`
}

// Type is one wire object. Generated is false for a hand-written type the
// generated code embeds or points at, whose fields are on the wire just the same.
// APIPaths are the collection segments the client addresses for the resource,
// under api/s/<site>/rest|stat/ or v2/api/site/<site>/. Embeds names same-package
// types whose fields are part of this object.
type Type struct {
	Package        string   `json:"package"`
	GoName         string   `json:"go_name"`
	Generated      bool     `json:"generated"`
	SchemaResource string   `json:"schema_resource,omitempty"`
	APIPaths       []string `json:"api_paths,omitempty"`
	Embeds         []string `json:"embeds,omitempty"`
	Fields         []Field  `json:"fields"`
}

// Field is one tagged struct field, in declaration order.
//
// GoType is the declaration verbatim ("*int64", "[]DevicePortTable"), and the
// pointers are the point: *int64 and int64 differ in whether the caller can leave
// the field unset, and **E, []E and []*E are three shapes for a nested object.
// OmitEmpty is always present, because absent would be indistinguishable from a
// deliberate false -- and false is the case that matters, since the key is then on
// every write whether the caller set it or not.
type Field struct {
	Wire      string `json:"wire"`
	GoName    string `json:"go_name"`
	GoType    string `json:"go_type"`
	OmitEmpty bool   `json:"omit_empty"`
}

// Load parses the embedded artifact.
func Load() (*Contract, error) {
	var contract Contract
	if err := json.Unmarshal(JSON, &contract); err != nil {
		return nil, fmt.Errorf("parse %s: %w", Path, err)
	}
	return &contract, nil
}
