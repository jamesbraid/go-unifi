package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
)

// fieldOverride is one [Resource.field.<wire>] entry from
// overrides/fields.toml.
type fieldOverride struct {
	Add           bool   `toml:"add"`
	Name          string `toml:"name"`
	Type          string `toml:"type"`
	Validation    string `toml:"validation"`
	OmitEmpty     *bool  `toml:"omitempty"`
	Pointer       *bool  `toml:"pointer"`
	Array         *bool  `toml:"array"`
	UnmarshalType string `toml:"unmarshal_type"`
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

	for jsonName, fo := range override.Field {
		key, exists := keysByJSON[jsonName]
		switch {
		case exists:
			f := base.Fields[key]
			if fo.Name != "" && fo.Name != f.FieldName {
				delete(base.Fields, key)
				f.FieldName = fo.Name
				base.Fields[fo.Name] = f
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
		case fo.Add:
			if fo.Name == "" || fo.Type == "" {
				return fmt.Errorf("%s field %q: add requires name and type", r.StructName, jsonName)
			}
			f := NewFieldInfo(
				fo.Name, jsonName, fo.Type, fo.Validation,
				fo.OmitEmpty != nil && *fo.OmitEmpty,
				fo.Array != nil && *fo.Array,
				fo.Pointer != nil && *fo.Pointer,
				fo.UnmarshalType,
			)
			base.Fields[f.FieldName] = f
		default:
			fmt.Printf("warning: override %s.%s matches no schema field (set add = true to create it)\n", r.StructName, jsonName)
		}
	}

	return nil
}
