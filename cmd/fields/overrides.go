package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

// fieldOverride is one [Resource.field.<wire>] entry from
// overrides/fields.toml.
type fieldOverride struct {
	Add           bool   `toml:"add"`
	Remove        bool   `toml:"remove"`
	Name          string `toml:"name"`
	JSON          string `toml:"json"`
	Type          string `toml:"type"`
	Validation    string `toml:"validation"`
	OmitEmpty     *bool  `toml:"omitempty"`
	ReadOnly      bool   `toml:"readonly"`
	Pointer       *bool  `toml:"pointer"`
	Array         *bool  `toml:"array"`
	UnmarshalType string `toml:"unmarshal_type"`
	UnmarshalFunc string `toml:"unmarshal_func"`
	Doc           string `toml:"doc"`
	NilAsEmpty    bool   `toml:"nil_as_empty"`
}

// resourceOverride is one [Resource] table from overrides/fields.toml.
//
// Preference entries are typed in internal/fields because the integration
// test reads them back out of the same file; see fields.Preference.
type resourceOverride struct {
	Path string `toml:"path"`
	// ListPath overrides the collection path for the list call alone. The
	// v2 network members group needs it: every other verb is served by the
	// singular path, and only the list is served by the plural one.
	ListPath   string                       `toml:"list_path"`
	Field      map[string]fieldOverride     `toml:"field"`
	Preference map[string]fields.Preference `toml:"preference"`
}

var (
	resourceOverridesOnce sync.Once
	resourceOverridesMap  map[string]resourceOverride
)

// resourceOverrides lazily loads overrides/fields.toml from the module root.
func resourceOverrides() map[string]resourceOverride {
	resourceOverridesOnce.Do(func() {
		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		root := findModuleRoot(wd)
		if root == "" {
			panic("unable to locate the module root (go.mod) for overrides/fields.toml")
		}

		path := filepath.Join(root, "overrides", "fields.toml")
		overrides := map[string]resourceOverride{}
		md, err := toml.DecodeFile(path, &overrides)
		if err != nil {
			panic(fmt.Sprintf("unable to load %s: %v", path, err))
		}
		// TOML ignores a key no struct field claims, so a misspelling is
		// accepted in silence and the property it meant to set stays at its
		// zero value: "onws" leaves owns empty, and an empty owns set is a
		// measured result here, so the mode publishes as owning nothing.
		// That is the exact silent failure these tables were written to
		// document, reproduced in the file that documents it.
		//
		// Nothing in the file is legitimately undecoded, so any leftover is
		// a typo.
		if undecoded := md.Undecoded(); len(undecoded) > 0 {
			keys := make([]string, 0, len(undecoded))
			for _, key := range undecoded {
				keys = append(keys, key.String())
			}
			slices.Sort(keys)
			panic(fmt.Sprintf("%s has %d key(s) nothing reads: %s.\n\nEach is a misspelling or a "+
				"property this tool does not support. A key nothing reads sets nothing, and TOML "+
				"will not complain about it.", path, len(keys), strings.Join(keys, ", ")))
		}
		resourceOverridesMap = overrides
	})
	return resourceOverridesMap
}

// applyOverrides applies the resource's overrides/fields.toml entries to its
// top-level struct fields. It runs after schema parsing and the resource's
// FieldProcessor, so declared overrides always win; a field the schema also
// defines keeps its schema validation and only has the explicitly-set
// properties overridden, while an absent field is created when add = true
// (the compat-field case).
func (r *ResourceInfo) applyOverrides() error {
	// Every type this resource declares, not only the top-level one. A table
	// named for a nested struct addresses that struct: [FirewallPolicySchedule.
	// field.time_all_day] reaches the schedule, which nothing could previously
	// do. Before this, a table naming a nested type decoded happily -- the
	// overrides map accepts any key -- and was then never looked up, so it was
	// ignored in silence, which is the failure mode these tables exist to
	// document.
	//
	// Sorted so two types claiming the same override apply in a fixed order
	// rather than whichever way the map iterated.
	for _, typeName := range slices.Sorted(maps.Keys(r.Types)) {
		override, ok := resourceOverrides()[typeName]
		if !ok {
			continue
		}
		if err := r.applyOverrideToType(typeName, override); err != nil {
			return err
		}
	}
	return nil
}

func (r *ResourceInfo) applyOverrideToType(typeName string, override resourceOverride) error {
	base := r.Types[typeName]
	keysByJSON := map[string]string{}
	for key, f := range base.Fields {
		if f != nil {
			keysByJSON[f.JSONName] = key
		}
	}

	// Deterministic application: removals first, then everything else in
	// wire-name order — map iteration order must never decide the outcome
	// of a remove/rename interaction.
	names := slices.Sorted(maps.Keys(override.Field))
	for _, jsonName := range names {
		fo := override.Field[jsonName]
		if !fo.Remove {
			continue
		}
		if key, exists := keysByJSON[jsonName]; exists {
			// Envelope fields the resource's wire format doesn't carry
			// (true-v2 objects lack _id/site_id/attr_*).
			delete(base.Fields, key)
			delete(keysByJSON, jsonName)
		}
	}

	for _, jsonName := range names {
		fo := override.Field[jsonName]
		if fo.Remove {
			continue
		}
		key, exists := keysByJSON[jsonName]
		switch {
		case exists:
			f := base.Fields[key]
			if fo.Name != "" && fo.Name != f.FieldName {
				if _, taken := base.Fields[fo.Name]; taken {
					return fmt.Errorf("%s field %q: rename target %q already exists", typeName, jsonName, fo.Name)
				}
				delete(base.Fields, key)
				f.FieldName = fo.Name
				base.Fields[fo.Name] = f
			}
			if fo.JSON != "" && fo.JSON != f.JSONName {
				// Retag the wire name (true-v2 objects use "id", not "_id").
				if !jsonNameRe.MatchString(fo.JSON) {
					return fmt.Errorf("%s field %q: unsafe json retag %q", typeName, jsonName, fo.JSON)
				}
				f.JSONName = fo.JSON
			}
			if fo.Type != "" {
				f.FieldType = fo.Type
			}
			if fo.OmitEmpty != nil {
				f.OmitEmpty = *fo.OmitEmpty
			}
			if fo.ReadOnly {
				f.ReadOnly = true
			}
			if fo.Pointer != nil {
				f.IsPointer = *fo.Pointer
			}
			if fo.Array != nil {
				f.IsArray = *fo.Array
			}
			if fo.UnmarshalType != "" {
				f.CustomUnmarshalType = fo.UnmarshalType
			}
			if fo.UnmarshalFunc != "" {
				f.ReadOnly = fo.ReadOnly
				f.CustomUnmarshalFunc = fo.UnmarshalFunc
			}
			if fo.Doc != "" {
				f.Doc = newlineRe.ReplaceAllString(fo.Doc, " ")
			}
			if fo.NilAsEmpty {
				f.NilAsEmpty = true
			}
		case fo.Add:
			if fo.Name == "" || fo.Type == "" {
				return fmt.Errorf("%s field %q: add requires name and type", typeName, jsonName)
			}
			if _, taken := base.Fields[fo.Name]; taken {
				return fmt.Errorf("%s field %q: add target %q already exists", typeName, jsonName, fo.Name)
			}
			f := NewFieldInfo(
				fo.Name, jsonName, fo.Type, fo.Validation,
				fo.OmitEmpty != nil && *fo.OmitEmpty,
				fo.Array != nil && *fo.Array,
				fo.Pointer != nil && *fo.Pointer,
				fo.UnmarshalType,
			)
			f.ReadOnly = fo.ReadOnly
			f.CustomUnmarshalFunc = fo.UnmarshalFunc
			f.Doc = newlineRe.ReplaceAllString(fo.Doc, " ")
			base.Fields[f.FieldName] = f
			keysByJSON[jsonName] = f.FieldName
		default:
			fmt.Printf("warning: override %s.%s matches no schema field (set add = true to create it)\n", typeName, jsonName)
		}
	}

	// Final uniqueness validation: overrides must never leave two fields
	// sharing a Go name or a wire name (encoding/json silently drops
	// same-depth duplicate tags).
	seenName := map[string]string{}
	seenJSON := map[string]string{}
	for _, f := range base.Fields {
		if f == nil {
			continue
		}
		if prev, dup := seenName[f.FieldName]; dup {
			return fmt.Errorf("%s: overrides left duplicate field name %q (wire %q and %q)", typeName, f.FieldName, prev, f.JSONName)
		}
		seenName[f.FieldName] = f.JSONName
		if prev, dup := seenJSON[f.JSONName]; dup {
			return fmt.Errorf("%s: overrides left duplicate wire name %q (fields %q and %q)", typeName, f.JSONName, prev, f.FieldName)
		}
		seenJSON[f.JSONName] = f.FieldName
	}

	return r.validatePreferences(override)
}

// validatePreferences checks that every wire name in a [Resource.preference.*]
// table resolves to a field the generator emits.
//
// A misspelled entry would otherwise own nothing and say nothing -- the same
// silent failure these tables exist to document, reproduced in the tool that
// documents it. Fields are matched after removals and adds, so the names must
// match the struct the generator actually emits.
//
// A key may name a mode nested inside a sub-object, as
// "port_overrides.setting_preference". Everything in owns is then resolved in
// that same sub-object: a mode governs its own object, and for an array
// container there is no single path to a sibling anyway, because ownership
// holds per element.
func (r *ResourceInfo) validatePreferences(override resourceOverride) error {
	for _, key := range slices.Sorted(maps.Keys(override.Preference)) {
		container, mode := splitPreferenceKey(key)

		scope, err := r.resolveContainer(container)
		if err != nil {
			return fmt.Errorf("%s preference %q: %w", r.StructName, key, err)
		}

		if _, ok := scope[mode]; !ok {
			return fmt.Errorf("%s preference %q: no such field on %s",
				r.StructName, key, containerLabel(r.StructName, container))
		}
		for _, owned := range override.Preference[key].Owns {
			if strings.Contains(owned, ".") {
				return fmt.Errorf("%s preference %q: owns %q, but owned names are relative to %s "+
					"-- a mode governs its own object",
					r.StructName, key, owned, containerLabel(r.StructName, container))
			}
			if _, ok := scope[owned]; !ok {
				return fmt.Errorf("%s preference %q: owns %q, which is not a field on %s",
					r.StructName, key, owned, containerLabel(r.StructName, container))
			}
			if owned == mode {
				return fmt.Errorf("%s preference %q: lists itself in owns", r.StructName, key)
			}
		}

		// An ownership set is a measurement, and a measurement with no build
		// behind it cannot be told from a stale one -- which is the whole
		// job of the key. Nothing else catches its absence: a misspelled
		// "measured" decodes to empty without error, and the generated file
		// does not carry the value, so the provenance can vanish without
		// changing a single byte of output.
		if strings.TrimSpace(override.Preference[key].Measured) == "" {
			return fmt.Errorf("%s preference %q: no measured build. Record the controller version "+
				"the set was measured against (measured = \"10.4.57\"); an ownership set with no "+
				"provenance reads as current forever", r.StructName, key)
		}

		for _, excluded := range override.Preference[key].UOSExcludes {
			if !slices.Contains(override.Preference[key].Owns, excluded) {
				return fmt.Errorf("%s preference %q: uos_excludes names %q, which is not in owns. "+
					"An exclusion says the console takes a field the mode owns elsewhere, so one "+
					"that names an unowned field describes nothing",
					r.StructName, key, excluded)
			}
		}
	}
	return nil
}

// splitPreferenceKey splits a preference key into its container path and the
// mode's own wire name. A key with no dots has an empty container, meaning
// the mode sits on the resource itself.
func splitPreferenceKey(key string) (container, mode string) {
	idx := strings.LastIndex(key, ".")
	if idx < 0 {
		return "", key
	}
	return key[:idx], key[idx+1:]
}

// resolveContainer walks a dotted container path and returns the wire names
// declared at the end of it, keyed by wire name.
//
// An array descends to its element type: IsArray changes how many of the
// object there are, not where its fields live.
func (r *ResourceInfo) resolveContainer(container string) (map[string]*FieldInfo, error) {
	current := r.Types[r.StructName]
	if current == nil {
		return nil, fmt.Errorf("resource has no base type")
	}

	walked := ""
	if container != "" {
		for segment := range strings.SplitSeq(container, ".") {
			byWire := wireIndex(current)
			next, ok := byWire[segment]
			if !ok {
				return nil, fmt.Errorf("no field %q on %s", segment, containerLabel(r.StructName, walked))
			}
			resolved := r.nestedType(next)
			if resolved == nil {
				return nil, fmt.Errorf("field %q on %s carries no sub-object to nest into",
					segment, containerLabel(r.StructName, walked))
			}
			current = resolved
			if walked == "" {
				walked = segment
			} else {
				walked += "." + segment
			}
		}
	}

	return wireIndex(current), nil
}

// nestedType returns the type a field nests into, whether it carries its
// fields inline or names a type declared alongside the resource.
func (r *ResourceInfo) nestedType(field *FieldInfo) *FieldInfo {
	if len(field.Fields) > 0 {
		return field
	}
	if named, ok := r.Types[field.FieldType]; ok && named != nil && len(named.Fields) > 0 {
		return named
	}
	return nil
}

// wireIndex keys a type's fields by wire name.
func wireIndex(t *FieldInfo) map[string]*FieldInfo {
	out := make(map[string]*FieldInfo, len(t.Fields))
	for _, f := range t.Fields {
		if f != nil {
			out[f.JSONName] = f
		}
	}
	return out
}

// containerLabel names the object a path resolved to, for error messages.
func containerLabel(structName, container string) string {
	if container == "" {
		return "the resource"
	}
	return structName + "." + container
}
