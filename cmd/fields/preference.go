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
		for _, mode := range slices.Sorted(maps.Keys(prefs)) {
			owns := prefs[mode].Owns
			if len(owns) == 0 {
				// A measured empty set is a result: this mode carries the
				// enum and acts on nothing. Rendered explicitly so it reads
				// as measured rather than missing.
				fmt.Fprintf(&body, "\t\t%q: {},\n", mode)
				continue
			}
			fmt.Fprintf(&body, "\t\t%q: {\n", mode)
			for _, wire := range slices.Sorted(slices.Values(owns)) {
				fmt.Fprintf(&body, "\t\t\t%q,\n", wire)
			}
			body.WriteString("\t\t},\n")
		}
		body.WriteString("\t},\n")
	}

	src := fmt.Appendf(nil, `
// Generated code. DO NOT EDIT.

package unifi

// PreferenceOwnedFields records what each auto|manual mode field owns.
//
// A UniFi resource can carry a mode field -- setting_preference and its
// siblings -- that decides whether a block of its own fields is the caller's
// to set or the controller's. While the mode is "auto" the controller stores
// its own values over whatever the payload asked for, answers rc: ok, and
// reports nothing, so a caller learns from the next read or not at all.
//
// The outer key is the resource's schema name (settings keep their "Setting"
// prefix, so the site NTP document is "SettingNtp"). The inner key is the
// mode field's wire name, and the value is the wire names it owns. An empty
// list is a measured result, not a gap: that mode was probed and owns
// nothing.
//
// Measured against a live controller by TestIntegrationPreferenceOwnership
// and recorded in overrides/fields.toml; the build of each set is in the
// "measured" key beside it there.
var PreferenceOwnedFields = map[string]map[string][]string{
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
func describePreference(r *ResourceInfo, field *FieldInfo, attr *resource.Attribute) {
	if field == nil || attr == nil {
		return
	}
	prefs := resourceOverrides()[r.StructName].Preference
	if len(prefs) == 0 {
		return
	}

	if pref, ok := prefs[field.JSONName]; ok {
		setAttributeDescription(attr, modeDescription(r, pref))
		return
	}

	for _, mode := range slices.Sorted(maps.Keys(prefs)) {
		if slices.Contains(prefs[mode].Owns, field.JSONName) {
			name := specAttributeName(r, mode)
			setAttributeDescription(attr, fmt.Sprintf(
				"Ignored while %s is \"auto\": the controller stores its own value for this field, "+
					"answers rc: ok, and reports nothing. Set %s to \"manual\" to configure it.",
				name, name))
			return
		}
	}
}

// modeDescription renders the description for a mode field itself.
func modeDescription(r *ResourceInfo, pref fields.Preference) string {
	measured := pref.Measured
	if measured == "" {
		measured = "an unrecorded build"
	}
	if len(pref.Owns) == 0 {
		return fmt.Sprintf(
			"auto|manual. Measured against UniFi Network %s: this mode governs no fields, "+
				"so \"auto\" discards nothing.", measured)
	}
	owns := make([]string, 0, len(pref.Owns))
	for _, wire := range pref.Owns {
		owns = append(owns, specAttributeName(r, wire))
	}
	slices.Sort(owns)
	return fmt.Sprintf(
		"auto|manual. While \"auto\", the controller manages these attributes itself and overwrites "+
			"whatever is sent for them, without reporting it: %s. Measured against UniFi Network %s.",
		strings.Join(owns, ", "), measured)
}

// specAttributeName maps a wire name to the attribute name this spec emits
// for it.
//
// The two are not the same. Attribute names come from the Go field name via
// toTerraformName, which splits differently around digits and acronyms:
// ipv6_setting_preference is emitted as ipv_6_setting_preference. A
// description is read in provider documentation, next to the name a
// practitioner types, so it has to name the attribute rather than the wire
// field it came from. Falls back to the wire name if the field cannot be
// resolved, which is better than naming nothing.
func specAttributeName(r *ResourceInfo, wire string) string {
	base := r.Types[r.StructName]
	if base == nil {
		return wire
	}
	for _, f := range base.Fields {
		if f != nil && f.JSONName == wire {
			return toTerraformName(f.FieldName)
		}
	}
	return wire
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
