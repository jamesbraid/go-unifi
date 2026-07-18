package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"
	assert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateFromFieldsGeneratesCurrentSettingRegistry(t *testing.T) {
	fieldsDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fieldsDir, "Setting.json"), []byte(`{"alpha":{},"new_feature":{}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(fieldsDir, "SettingAlpha.json"), []byte(`{}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(fieldsDir, "SettingNewFeature.json"), []byte(`{}`), 0o600))

	outDir := t.TempDir()
	err := generateFromFields(fieldsDir, outDir, version.Must(version.NewVersion("10.4.57")), false, filepath.Join(t.TempDir(), "spec.json"), io.Discard, nil)
	require.NoError(t, err)

	body, err := os.ReadFile(filepath.Join(outDir, "settings", "registry.generated.go"))
	require.NoError(t, err)
	generated := string(body)
	assert.Contains(t, generated, "case *Alpha:")
	assert.Contains(t, generated, `return "alpha", true`)
	assert.Contains(t, generated, "case *NewFeature:")
	assert.Contains(t, generated, `return "new_feature", true`)
	assert.False(t, strings.Contains(generated, "RawSetting"), "raw fallback remains hand-written")
}

func TestGenerateFromFieldsKeepsHandWrittenSettingImplementation(t *testing.T) {
	fieldsDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fieldsDir, "Setting.json"), []byte(`{"alpha":{}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(fieldsDir, "SettingAlpha.json"), []byte(`{"enabled":"true|false"}`), 0o600))

	outDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(outDir, "settings"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outDir, "settings", "alpha.go"), []byte("package settings\n\ntype Alpha struct { BaseSetting }\n"), 0o600))
	err := generateFromFields(fieldsDir, outDir, version.Must(version.NewVersion("10.4.57")), false, filepath.Join(t.TempDir(), "spec.json"), io.Discard, nil)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outDir, "settings", "alpha.generated.go"))
	assert.ErrorIs(t, err, os.ErrNotExist)
	registry, err := os.ReadFile(filepath.Join(outDir, "settings", "registry.generated.go"))
	require.NoError(t, err)
	assert.Contains(t, string(registry), "case *Alpha:")
}

func TestGenerateFromFieldsMergesUpstreamWireguardIPVersionCompatibilityField(t *testing.T) {
	fieldsDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fieldsDir, "Setting.json"), []byte(`{}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(fieldsDir, "NetworkConf.json"), []byte(`{"networkgroup":"LAN[2-8]?","wireguard_interface_binding_mode_ip_version":"^(v4|v6)$"}`), 0o600))

	err := generateFromFields(fieldsDir, t.TempDir(), version.Must(version.NewVersion("10.4.57")), false, filepath.Join(t.TempDir(), "spec.json"), io.Discard, func(resources []*ResourceInfo) error {
		for _, resource := range resources {
			if resource.StructName != "Network" {
				continue
			}
			var matches []*FieldInfo
			for _, field := range resource.Types[resource.StructName].Fields {
				if field != nil && field.JSONName == "wireguard_interface_binding_mode_ip_version" {
					matches = append(matches, field)
				}
			}
			require.Len(t, matches, 1)
			assert.Equal(t, "WireguardInterfaceBindingModeIPVersion", matches[0].FieldName)
			assert.Equal(t, "string", matches[0].FieldType)
			assert.True(t, matches[0].IsPointer)
		}
		return nil
	})
	require.NoError(t, err)
}

func TestGenerateFromFieldsPreservesReleasedNetworkFields(t *testing.T) {
	fieldsDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fieldsDir, "Setting.json"), []byte(`{}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(fieldsDir, "NetworkConf.json"), []byte(`{"dhcpd_dns_1":"","dhcpd_dns_2":""}`), 0o600))

	err := generateFromFields(fieldsDir, t.TempDir(), version.Must(version.NewVersion("10.4.57")), false, filepath.Join(t.TempDir(), "spec.json"), io.Discard, func(resources []*ResourceInfo) error {
		require.Len(t, resources, 1)
		fields := resources[0].Types["Network"].Fields
		assert.Contains(t, fields, "MdnsEnabled")
		assert.Equal(t, "mdns_enabled", fields["MdnsEnabled"].JSONName)
		assert.False(t, fields["DHCPDDNS1"].IsPointer)
		assert.False(t, fields["DHCPDDNS2"].IsPointer)
		return nil
	})
	require.NoError(t, err)
}

func TestFieldInfoFromValidation(t *testing.T) {
	for i, c := range []struct {
		expectedType      string
		expectedComment   string
		expectedOmitEmpty bool
		validation        any
	}{
		{"string", "", true, ""},
		{"string", "default|custom", true, "default|custom"},
		{"string", ".{0,32}", true, ".{0,32}"},
		{"string", "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$", false, "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$"},

		{"int64", "^([1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$", true, "^([1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$|^$"},
		{"int64", "", true, "^[0-9]*$"},

		{"float64", "", true, "[-+]?[0-9]*\\.?[0-9]+"},
		// this one is really an error as the . is not escaped
		{"float64", "", true, "^([-]?[\\d]+[.]?[\\d]*)$"},
		{"float64", "", true, "^([\\d]+[.]?[\\d]*)$"},

		{"bool", "", false, "false|true"},
		{"bool", "", false, "true|false"},
	} {
		t.Run(fmt.Sprintf("%d %s %s", i, c.expectedType, c.validation), func(t *testing.T) {
			resource := &ResourceInfo{
				StructName:     "TestType",
				Types:          make(map[string]*FieldInfo),
				FieldProcessor: func(name string, f *FieldInfo) error { return nil },
			}

			fieldInfo, err := resource.fieldInfoFromValidation("fieldName", c.validation)
			// actualType, actualComment, actualOmitEmpty, err := fieldInfoFromValidation(c.validation)
			if err != nil {
				t.Fatal(err)
			}
			if fieldInfo.FieldType != c.expectedType {
				t.Fatalf("expected type %q got %q", c.expectedType, fieldInfo.FieldType)
			}
			if fieldInfo.FieldValidation != c.expectedComment {
				t.Fatalf("expected comment %q got %q", c.expectedComment, fieldInfo.FieldValidation)
			}
			if fieldInfo.OmitEmpty != c.expectedOmitEmpty {
				t.Fatalf("expected omitempty %t got %t", c.expectedOmitEmpty, fieldInfo.OmitEmpty)
			}
		})
	}
}

func TestResourceTypes(t *testing.T) {
	testData := `
{
  "note": ".{0,1024}",
  "date": "^$|^(20[0-9]{2}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9])Z?$",
  "mac": "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$",
  "number": "\\d+",
  "boolean": "true|false",
	"nested_type": {
    "nested_field": "^$"
  },
  "nested_type_array": [{
    "nested_field": "^$"
  }]
}
	`
	expectedFields := map[string]*FieldInfo{
		"Note":    NewFieldInfo("Note", "note", "string", ".{0,1024}", true, false, false, ""),
		"Date":    NewFieldInfo("Date", "date", "string", "^$|^(20[0-9]{2}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9])Z?$", false, false, false, ""),
		"MAC":     NewFieldInfo("MAC", "mac", "string", "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$", true, false, false, ""),
		"Number":  NewFieldInfo("Number", "number", "int64", "", true, false, true, "types.Number"),
		"Boolean": NewFieldInfo("Boolean", "boolean", "bool", "", false, false, false, ""),
		"NestedType": {
			FieldName:       "NestedType",
			JSONName:        "nested_type",
			FieldType:       "StructNestedType",
			FieldValidation: "",
			OmitEmpty:       true,
			IsPointer:       true,
			IsArray:         false,
			Fields: map[string]*FieldInfo{
				"NestedFieldModified": NewFieldInfo("NestedFieldModified", "nested_field", "string", "^$", false, false, false, ""),
			},
		},
		"NestedTypeArray": {
			FieldName:       "NestedTypeArray",
			JSONName:        "nested_type_array",
			FieldType:       "StructNestedTypeArray",
			FieldValidation: "",
			OmitEmpty:       true,
			IsPointer:       false,
			IsArray:         true,
			Fields: map[string]*FieldInfo{
				"NestedFieldModified": NewFieldInfo("NestedFieldModified", "nested_field", "string", "^$", false, false, false, ""),
			},
		},
	}

	expectedStruct := map[string]*FieldInfo{
		"Struct": {
			FieldName:       "Struct",
			JSONName:        "path",
			FieldType:       "struct",
			FieldValidation: "",
			OmitEmpty:       false,
			IsArray:         false,
			Fields: map[string]*FieldInfo{
				"   ID":      NewFieldInfo("ID", "_id", "string", "", true, false, false, ""),
				"   SiteID":  NewFieldInfo("SiteID", "site_id", "string", "", true, false, false, ""),
				"   _Spacer": nil,
				"  Hidden":   NewFieldInfo("Hidden", "attr_hidden", "bool", "", true, false, false, ""),
				"  HiddenID": NewFieldInfo("HiddenID", "attr_hidden_id", "string", "", true, false, false, ""),
				"  NoDelete": NewFieldInfo("NoDelete", "attr_no_delete", "bool", "", true, false, false, ""),
				"  NoEdit":   NewFieldInfo("NoEdit", "attr_no_edit", "bool", "", true, false, false, ""),
				"  _Spacer":  nil,
				" _Spacer":   nil,
			},
		},
	}

	for k, v := range expectedFields {
		expectedStruct["Struct"].Fields[k] = v
	}

	expectation := &ResourceInfo{
		StructName:   "Struct",
		ResourcePath: "path",

		Types: map[string]*FieldInfo{
			"Struct":                expectedStruct["Struct"],
			"StructNestedType":      expectedStruct["Struct"].Fields["NestedType"],
			"StructNestedTypeArray": expectedStruct["Struct"].Fields["NestedTypeArray"],
		},

		FieldProcessor: func(name string, f *FieldInfo) error {
			if name == "NestedField" {
				f.FieldName = "NestedFieldModified"
			}
			return nil
		},
	}

	t.Run("structural test", func(t *testing.T) {
		resource := NewResource("Struct", "path")
		resource.FieldProcessor = expectation.FieldProcessor

		err := resource.processJSON(([]byte)(testData))

		assert.NoError(t, err, "No error processing JSON")
		assert.Equal(t, expectation.StructName, resource.StructName)
		assert.Equal(t, expectation.ResourcePath, resource.ResourcePath)
		assert.Equal(t, expectation.Types, resource.Types)
	})
}
