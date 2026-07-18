package main

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-codegen-spec/datasource"
	"github.com/hashicorp/terraform-plugin-codegen-spec/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecificationGenerator_Generate_EmptyProvider(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")
	spec := gen.Generate()

	assert.Equal(t, SpecVersion, spec.Version)
	assert.NotNil(t, spec.Provider)
	assert.Equal(t, "unifi", spec.Provider.Name)
	assert.NotNil(t, spec.Provider.Schema)
	assert.Len(t, spec.DataSources, 0)
	assert.Len(t, spec.Resources, 0)
}

func TestSpecificationGenerator_Generate_ProviderAttributes(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")
	spec := gen.Generate()

	require.NotNil(t, spec.Provider.Schema)
	attrs := spec.Provider.Schema.Attributes

	// Check that we have the expected provider attributes
	attrNames := make(map[string]bool)
	for _, attr := range attrs {
		attrNames[attr.Name] = true
	}

	assert.True(t, attrNames["username"])
	assert.True(t, attrNames["password"])
	assert.True(t, attrNames["api_url"])
	assert.True(t, attrNames["site"])
	assert.True(t, attrNames["allow_insecure"])
	password := attrs[slices.IndexFunc(attrs, func(attr provider.Attribute) bool { return attr.Name == "password" })]
	apiKey := attrs[slices.IndexFunc(attrs, func(attr provider.Attribute) bool { return attr.Name == "api_key" })]
	require.NotNil(t, password.String)
	require.NotNil(t, apiKey.String)
	assert.Equal(t, ptr(true), password.String.Sensitive)
	assert.Equal(t, ptr(true), apiKey.String.Sensitive)
}

func TestSpecificationGenerator_Generate_SimpleResource(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")

	// Create a simple resource
	resource := NewResource("Network", "network")
	resource.Types["Network"].Fields["Name"] = NewFieldInfo("Name", "name", "string", "", false, false, false, "")
	resource.Types["Network"].Fields["Purpose"] = NewFieldInfo("Purpose", "purpose", "string", "", true, false, false, "")
	resource.Types["Network"].Fields["Enabled"] = NewFieldInfo("Enabled", "enabled", "bool", "", false, false, false, "")
	resource.Types["Network"].Fields["VLANID"] = NewFieldInfo("VLANID", "vlan_id", "int64", "", true, false, false, "")

	gen.AddResource(resource)
	spec := gen.Generate()

	// Check data sources
	require.Len(t, spec.DataSources, 1)
	ds := spec.DataSources[0]
	assert.Equal(t, "network", ds.Name)
	require.NotNil(t, ds.Schema)

	// Check resources
	require.Len(t, spec.Resources, 1)
	res := spec.Resources[0]
	assert.Equal(t, "network", res.Name)
	require.NotNil(t, res.Schema)

	// Verify attributes exist
	dsAttrNames := make(map[string]bool)
	for _, attr := range ds.Schema.Attributes {
		dsAttrNames[attr.Name] = true
	}
	assert.True(t, dsAttrNames["name"])
	assert.True(t, dsAttrNames["purpose"])
	assert.True(t, dsAttrNames["enabled"])
	// Note: VLANID converts to vlan_id or vlanid depending on the library
	assert.True(t, dsAttrNames["vlan_id"] || dsAttrNames["vlanid"])
}

func TestSpecificationGenerator_Generate_ArrayAttribute(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")

	// Create a resource with an array attribute
	resource := NewResource("FirewallGroup", "firewallgroup")
	resource.Types["FirewallGroup"].Fields["Members"] = NewFieldInfo("Members", "members", "string", "", true, true, false, "")

	gen.AddResource(resource)
	spec := gen.Generate()

	require.Len(t, spec.Resources, 1)
	res := spec.Resources[0]

	i := slices.IndexFunc(res.Schema.Attributes, findMembers)

	require.GreaterOrEqual(t, i, 0)
}

func TestSpecificationGenerator_Generate_NestedAttribute(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")

	// Create a resource with a nested attribute
	resource := NewResource("Device", "device")
	nestedField := NewFieldInfo("ConfigNetwork", "config_network", "DeviceConfigNetwork", "", true, false, false, "")
	nestedField.Fields = map[string]*FieldInfo{
		"IP":      NewFieldInfo("IP", "ip", "string", "", true, false, false, ""),
		"Gateway": NewFieldInfo("Gateway", "gateway", "string", "", true, false, false, ""),
	}
	resource.Types["Device"].Fields["ConfigNetwork"] = nestedField
	resource.Types["DeviceConfigNetwork"] = nestedField

	gen.AddResource(resource)
	spec := gen.Generate()

	require.Len(t, spec.Resources, 1)
	res := spec.Resources[0]

	// Find the config_network attribute
	i := slices.IndexFunc(res.Schema.Attributes, findConfigNetwork)

	require.GreaterOrEqual(t, i, 0)
	configNetworkAttr := &res.Schema.Attributes[i]

	require.NotNil(t, configNetworkAttr)
	require.NotNil(t, configNetworkAttr.SingleNested)
	require.Len(t, configNetworkAttr.SingleNested.Attributes, 2)
}

func TestSpecificationGenerator_Generate_NestedArrayAttribute(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")

	// Create a resource with a nested array attribute
	resource := NewResource("WLAN", "wlan")
	nestedField := NewFieldInfo("Schedules", "schedules", "WLANSchedule", "", true, true, false, "")
	nestedField.Fields = map[string]*FieldInfo{
		"Start": NewFieldInfo("Start", "start", "string", "", true, false, false, ""),
		"End":   NewFieldInfo("End", "end", "string", "", true, false, false, ""),
	}
	resource.Types["WLAN"].Fields["Schedules"] = nestedField
	resource.Types["WLANSchedule"] = nestedField

	gen.AddResource(resource)
	spec := gen.Generate()

	require.Len(t, spec.Resources, 1)
	res := spec.Resources[0]

	i := slices.IndexFunc(res.Schema.Attributes, findAttr("schedules"))
	require.GreaterOrEqual(t, i, 0)
	schedulesAttr := &res.Schema.Attributes[i]

	require.NotNil(t, schedulesAttr)
	require.NotNil(t, schedulesAttr.ListNested)
	require.Len(t, schedulesAttr.ListNested.NestedObject.Attributes, 2)
}

func TestSpecificationGenerator_Generate_SensitiveLeaves(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")
	resource := NewResource("RADIUSProfile", "radiusprofile")
	privateKey := NewFieldInfo("PrivateKey", "x_private_key", "string", "", true, false, false, "")
	privateKey.Sensitive = true
	name := NewFieldInfo("Name", "name", "string", "", false, false, false, "")
	authServers := NewFieldInfo("AuthServers", "auth_servers", "RADIUSProfileAuthServers", "", true, true, false, "")
	secret := NewFieldInfo("Secret", "x_secret", "string", "", true, false, false, "")
	secret.Sensitive = true
	authServers.Fields = map[string]*FieldInfo{
		"IP":     NewFieldInfo("IP", "ip", "string", "", true, false, false, ""),
		"Secret": secret,
	}
	resource.Types[resource.StructName].Fields["PrivateKey"] = privateKey
	resource.Types[resource.StructName].Fields["Name"] = name
	resource.Types[resource.StructName].Fields["AuthServers"] = authServers
	resource.Types[authServers.FieldType] = authServers
	gen.AddResource(resource)

	generated := gen.Generate()
	require.Len(t, generated.Resources, 1)
	require.Len(t, generated.DataSources, 1)

	resourceAttrs := generated.Resources[0].Schema.Attributes
	resourceKey := resourceAttrs[slices.IndexFunc(resourceAttrs, findAttr("private_key"))]
	require.NotNil(t, resourceKey.String)
	assert.Equal(t, ptr(true), resourceKey.String.Sensitive)
	resourceName := resourceAttrs[slices.IndexFunc(resourceAttrs, findAttr("name"))]
	require.NotNil(t, resourceName.String)
	assert.Nil(t, resourceName.String.Sensitive)
	resourceAuth := resourceAttrs[slices.IndexFunc(resourceAttrs, findAttr("auth_servers"))]
	require.NotNil(t, resourceAuth.ListNested)
	assert.Nil(t, resourceAuth.ListNested.Sensitive)
	resourceNested := resourceAuth.ListNested.NestedObject.Attributes
	resourceSecret := resourceNested[slices.IndexFunc(resourceNested, findAttr("secret"))]
	resourceIP := resourceNested[slices.IndexFunc(resourceNested, findAttr("ip"))]
	assert.Equal(t, ptr(true), resourceSecret.String.Sensitive)
	assert.Nil(t, resourceIP.String.Sensitive)

	dsAttrs := generated.DataSources[0].Schema.Attributes
	dsKey := dsAttrs[slices.IndexFunc(dsAttrs, func(attr datasource.Attribute) bool { return attr.Name == "private_key" })]
	require.NotNil(t, dsKey.String)
	assert.Equal(t, ptr(true), dsKey.String.Sensitive)
	dsAuth := dsAttrs[slices.IndexFunc(dsAttrs, func(attr datasource.Attribute) bool { return attr.Name == "auth_servers" })]
	require.NotNil(t, dsAuth.ListNested)
	assert.Nil(t, dsAuth.ListNested.Sensitive)
	dsNested := dsAuth.ListNested.NestedObject.Attributes
	dsSecret := dsNested[slices.IndexFunc(dsNested, func(attr datasource.Attribute) bool { return attr.Name == "secret" })]
	dsIP := dsNested[slices.IndexFunc(dsNested, func(attr datasource.Attribute) bool { return attr.Name == "ip" })]
	assert.Equal(t, ptr(true), dsSecret.String.Sensitive)
	assert.Nil(t, dsIP.String.Sensitive)
}

func TestSpecificationGenerator_Generate_SkipsSettings(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")

	// Create a regular resource
	resource := NewResource("Network", "network")
	gen.AddResource(resource)

	// Create a setting resource
	setting := NewResource("SettingGlobalAp", "setting_global_ap")
	gen.AddResource(setting)

	spec := gen.Generate()

	// Should only have the non-setting resource
	assert.Len(t, spec.DataSources, 1)
	assert.Len(t, spec.Resources, 1)
	assert.Equal(t, "network", spec.DataSources[0].Name)
	assert.Equal(t, "network", spec.Resources[0].Name)
}

func TestAssociatedExternalType_Formatting(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")

	resource := NewResource("Network", "network")

	// Test basic string field - primitives should return nil
	stringField := NewFieldInfo("Name", "name", "string", "", false, false, false, "")
	extType := gen.buildAssociatedExternalType(resource, stringField)
	assert.Nil(t, extType, "primitive types should not have associated external type")

	// Test pointer field with OmitEmpty - pointer to primitive should return nil
	ptrField := NewFieldInfo("Description", "description", "string", "", true, false, true, "")
	extType = gen.buildAssociatedExternalType(resource, ptrField)
	assert.Nil(t, extType, "pointer to primitive should not have associated external type")

	// Test array field - array of primitives should return nil
	arrayField := NewFieldInfo("Members", "members", "string", "", true, true, false, "")
	extType = gen.buildAssociatedExternalType(resource, arrayField)
	assert.Nil(t, extType, "array of primitives should not have associated external type")

	// Test custom type
	customField := NewFieldInfo("Config", "config", "CustomType", "", false, false, false, "")
	extType = gen.buildAssociatedExternalType(resource, customField)
	require.NotNil(t, extType)
	assert.Equal(t, GoUnifiImportPath, extType.Import.Path)
	assert.Equal(t, "CustomType", extType.Type)

	// Test pointer to custom type
	ptrCustomField := NewFieldInfo("Settings", "settings", "Settings", "", true, false, true, "")
	extType = gen.buildAssociatedExternalType(resource, ptrCustomField)
	require.NotNil(t, extType)
	assert.Equal(t, "*Settings", extType.Type)
}

func TestToTerraformName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Network", "network"},
		{"FirewallGroup", "firewall_group"},
		{"WLAN", "wlan"},
		{"DNSRecord", "dns_record"},
		{"BGPConfig", "bgp_config"},
		{"PortProfile", "port_profile"},
		{"ClientGroup", "client_group"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := toTerraformName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSpecificationGenerator_Generate_DetermineComputedOptionalRequired(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")

	tests := []struct {
		name     string
		field    *FieldInfo
		expected string
	}{
		{
			name:     "ID field is computed",
			field:    NewFieldInfo("ID", "_id", "string", "", true, false, false, ""),
			expected: "computed",
		},
		{
			name:     "SiteID field is computed",
			field:    NewFieldInfo("SiteID", "site_id", "string", "", true, false, false, ""),
			expected: "computed",
		},
		{
			name:     "Hidden field is computed",
			field:    NewFieldInfo("Hidden", "attr_hidden", "bool", "", true, false, false, ""),
			expected: "computed",
		},
		{
			name:     "Field with OmitEmpty is computed_optional",
			field:    NewFieldInfo("Description", "description", "string", "", true, false, false, ""),
			expected: "computed_optional",
		},
		{
			name:     "Field without OmitEmpty is optional",
			field:    NewFieldInfo("Name", "name", "string", "", false, false, false, ""),
			expected: "optional",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := gen.determineComputedOptionalRequired(tc.field)
			assert.Equal(t, tc.expected, string(result))
		})
	}
}

func TestSpecificationGenerator_Generate_ValidJSON(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")

	// Add a simple resource
	resource := NewResource("Network", "network")
	resource.Types["Network"].Fields["Name"] = NewFieldInfo("Name", "name", "string", "", false, false, false, "")
	gen.AddResource(resource)

	spec := gen.Generate()

	// Ensure it can be marshaled to valid JSON
	data, err := json.MarshalIndent(spec, "", "  ")
	require.NoError(t, err)
	require.NotEmpty(t, data)

	err = spec.Validate(t.Context())
	require.NoError(t, err)
}

func TestSpecificationGenerator_Generate_SortedOutput(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")

	// Add resources in non-alphabetical order
	gen.AddResource(NewResource("WLAN", "wlan"))
	gen.AddResource(NewResource("Account", "account"))
	gen.AddResource(NewResource("Network", "network"))

	spec := gen.Generate()

	// Verify data sources are sorted
	require.Len(t, spec.DataSources, 3)
	assert.Equal(t, "account", spec.DataSources[0].Name)
	assert.Equal(t, "network", spec.DataSources[1].Name)
	assert.Equal(t, "wlan", spec.DataSources[2].Name)

	// Verify resources are sorted
	require.Len(t, spec.Resources, 3)
	assert.Equal(t, "account", spec.Resources[0].Name)
	assert.Equal(t, "network", spec.Resources[1].Name)
	assert.Equal(t, "wlan", spec.Resources[2].Name)
}

func TestSpecification_JSONStructure(t *testing.T) {
	gen := NewSpecificationGenerator("unifi")

	// Create a comprehensive resource
	resource := NewResource("Network", "network")
	resource.Types["Network"].Fields["Name"] = NewFieldInfo("Name", "name", "string", "", false, false, false, "")
	resource.Types["Network"].Fields["Enabled"] = NewFieldInfo("Enabled", "enabled", "bool", "", false, false, false, "")
	resource.Types["Network"].Fields["VLANID"] = NewFieldInfo("VLANID", "vlan_id", "int64", "", true, false, false, "")
	resource.Types["Network"].Fields["Speed"] = NewFieldInfo("Speed", "speed", "float64", "", true, false, false, "")

	gen.AddResource(resource)
	spec := gen.Generate()

	// Marshal to JSON and verify structure
	data, err := json.MarshalIndent(spec, "", "  ")
	require.NoError(t, err)

	// Parse as generic map to check structure
	var jsonMap map[string]any
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	// Verify top-level keys
	assert.Contains(t, jsonMap, "version")
	assert.Contains(t, jsonMap, "provider")
	assert.Contains(t, jsonMap, "datasources")
	assert.Contains(t, jsonMap, "resources")

	// Verify version
	assert.Equal(t, "0.1", jsonMap["version"])

	// Verify provider structure
	provider := jsonMap["provider"].(map[string]any)
	assert.Equal(t, "unifi", provider["name"])
	assert.Contains(t, provider, "schema")

	// Verify datasources is an array
	datasources := jsonMap["datasources"].([]any)
	assert.Len(t, datasources, 1)

	// Verify resources is an array
	resources := jsonMap["resources"].([]any)
	assert.Len(t, resources, 1)
}
