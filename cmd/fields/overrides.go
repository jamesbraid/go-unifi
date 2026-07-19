package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/BurntSushi/toml"
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
	Pointer       *bool  `toml:"pointer"`
	Array         *bool  `toml:"array"`
	UnmarshalType string `toml:"unmarshal_type"`
	UnmarshalFunc string `toml:"unmarshal_func"`
	Doc           string `toml:"doc"`
}

// resourceOverride is one [Resource] table from overrides/fields.toml.
type resourceOverride struct {
	Path  string                   `toml:"path"`
	Field map[string]fieldOverride `toml:"field"`
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
		if _, err := toml.DecodeFile(path, &overrides); err != nil {
			panic(fmt.Sprintf("unable to load %s: %v", path, err))
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
	override, ok := resourceOverrides()[r.StructName]
	if !ok {
		return nil
	}

	base := r.Types[r.StructName]
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
					return fmt.Errorf("%s field %q: rename target %q already exists", r.StructName, jsonName, fo.Name)
				}
				delete(base.Fields, key)
				f.FieldName = fo.Name
				base.Fields[fo.Name] = f
			}
			if fo.JSON != "" && fo.JSON != f.JSONName {
				// Retag the wire name (true-v2 objects use "id", not "_id").
				if !jsonNameRe.MatchString(fo.JSON) {
					return fmt.Errorf("%s field %q: unsafe json retag %q", r.StructName, jsonName, fo.JSON)
				}
				f.JSONName = fo.JSON
			}
			if fo.Type != "" {
				f.FieldType = fo.Type
			}
			if fo.OmitEmpty != nil {
				f.OmitEmpty = *fo.OmitEmpty
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
				f.CustomUnmarshalFunc = fo.UnmarshalFunc
			}
			if fo.Doc != "" {
				f.Doc = newlineRe.ReplaceAllString(fo.Doc, " ")
			}
		case fo.Add:
			if fo.Name == "" || fo.Type == "" {
				return fmt.Errorf("%s field %q: add requires name and type", r.StructName, jsonName)
			}
			if _, taken := base.Fields[fo.Name]; taken {
				return fmt.Errorf("%s field %q: add target %q already exists", r.StructName, jsonName, fo.Name)
			}
			f := NewFieldInfo(
				fo.Name, jsonName, fo.Type, fo.Validation,
				fo.OmitEmpty != nil && *fo.OmitEmpty,
				fo.Array != nil && *fo.Array,
				fo.Pointer != nil && *fo.Pointer,
				fo.UnmarshalType,
			)
			f.CustomUnmarshalFunc = fo.UnmarshalFunc
			f.Doc = newlineRe.ReplaceAllString(fo.Doc, " ")
			base.Fields[f.FieldName] = f
			keysByJSON[jsonName] = f.FieldName
		default:
			fmt.Printf("warning: override %s.%s matches no schema field (set add = true to create it)\n", r.StructName, jsonName)
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
			return fmt.Errorf("%s: overrides left duplicate field name %q (wire %q and %q)", r.StructName, f.FieldName, prev, f.JSONName)
		}
		seenName[f.FieldName] = f.JSONName
		if prev, dup := seenJSON[f.JSONName]; dup {
			return fmt.Errorf("%s: overrides left duplicate wire name %q (fields %q and %q)", r.StructName, f.JSONName, prev, f.FieldName)
		}
		seenJSON[f.JSONName] = f.FieldName
	}

	return nil
}
