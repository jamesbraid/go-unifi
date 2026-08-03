package main

import (
	"bytes"
	"fmt"
	"go/format"
	"maps"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-codegen-spec/resource"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

// generatePreferenceFile renders the measured auto|manual ownership tables
// from overrides/fields.toml into a Go map consumers can read at runtime.
//
// The tables are the only record of which fields a controller takes over on
// "auto"; nothing in the schema describes it. Emitting them here means a
// consumer that has to decide the mode -- a Terraform provider settling the
// value at plan time, say -- can read the measured answer instead of keeping
// a hand-copied list that nothing checks.
func generatePreferenceFile(generated map[string]bool) ([]byte, error) {
	overrides := resourceOverrides()

	var body bytes.Buffer
	for _, resource := range slices.Sorted(maps.Keys(overrides)) {
		prefs := overrides[resource].Preference
		if len(prefs) == 0 {
			continue
		}
		if !generated[resource] {
			return nil, fmt.Errorf(
				"overrides/fields.toml has a preference table for %s, which this run did not generate; "+
					"the resource left the schema, so remove the table or restore the resource", resource)
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
// and recorded in overrides/fields.toml; the build of each set is in the
// "measured" key beside it there.
var PreferenceOwnedFields = map[string][]Preference{
%s}
`, body.String())

	formatted, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("unable to format the generated preference file: %w", err)
	}
	return formatted, nil
}

// describePreference annotates a resource attribute that takes part in an
// auto|manual relationship.
//
// Ownership is invisible in the schema: the mode field is an ordinary
// two-value enum and the fields it governs look like any other. A consumer
// reading only the generated spec has no way to know that setting one of
// them under "auto" is accepted and then discarded, so the relationship is
// written into the description on both sides.
//
// Resources only. A data source cannot write, so the trap does not exist
// there, and describing it on both would double the spec diff for no gain.
func describePreference(r *ResourceInfo, container string, field *FieldInfo, attr *resource.Attribute) {
	if field == nil || attr == nil {
		return
	}
	prefs := resourceOverrides()[r.StructName].Preference
	if len(prefs) == 0 {
		return
	}

	// Keys are resolved in the object the attribute actually lives in, so a
	// mode nested in a sub-object never annotates a same-named field on the
	// resource, and vice versa.
	if pref, ok := prefs[joinContainer(container, field.JSONName)]; ok {
		setAttributeDescription(attr, modeDescription(pref))
		return
	}

	for _, key := range slices.Sorted(maps.Keys(prefs)) {
		modeContainer, mode := splitPreferenceKey(key)
		if modeContainer != container {
			continue
		}
		if slices.Contains(prefs[key].Owns, field.JSONName) {
			// mode is the wire name, which is also what the attribute is
			// called -- see modeDescription.
			name := mode
			setAttributeDescription(attr, fmt.Sprintf(
				"Ignored while %s is \"auto\": the controller stores its own value for this field, "+
					"answers rc: ok, and reports nothing. Set %s to \"manual\" to configure it.",
				name, name))
			return
		}
	}
}

// modeDescription renders the description for a mode field itself.
func modeDescription(pref fields.Preference) string {
	measured := pref.Measured
	if measured == "" {
		measured = "an unrecorded build"
	}
	if len(pref.Owns) == 0 {
		return fmt.Sprintf(
			"auto|manual. Measured against UniFi Network %s: this mode governs no fields, "+
				"so \"auto\" discards nothing.", measured)
	}
	// The wire name is the attribute name: buildResourceAttribute names
	// attributes from JSONName, precisely because deriving them from the Go
	// field name produced names no API user would recognise. A description
	// is read next to the name a practitioner types, so it has to use the
	// same one.
	owns := slices.Clone(pref.Owns)
	slices.Sort(owns)
	return fmt.Sprintf(
		"auto|manual. While \"auto\", the controller manages these attributes itself and overwrites "+
			"whatever is sent for them, without reporting it: %s. Measured against UniFi Network %s.",
		strings.Join(owns, ", "), measured)
}

// setAttributeDescription sets Description on whichever typed attribute the
// spec builder produced.
func setAttributeDescription(attr *resource.Attribute, description string) {
	switch {
	case attr.Bool != nil:
		attr.Bool.Description = &description
	case attr.Int64 != nil:
		attr.Int64.Description = &description
	case attr.Float64 != nil:
		attr.Float64.Description = &description
	case attr.String != nil:
		attr.String.Description = &description
	case attr.List != nil:
		attr.List.Description = &description
	case attr.ListNested != nil:
		attr.ListNested.Description = &description
	case attr.SingleNested != nil:
		attr.SingleNested.Description = &description
	}
}

// joinContainer appends a wire name to a dotted container path.
func joinContainer(container, wire string) string {
	if container == "" {
		return wire
	}
	return container + "." + wire
}
