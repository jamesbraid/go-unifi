package main

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecificationGenerator_Generate_EmptyProvider(t *testing.T) {
	gen := NewSpecificationGenerator("unifi", nil)
	spec := gen.Generate()

	assert.Equal(t, SpecVersion, spec.Version)
	assert.NotNil(t, spec.Provider)
	assert.Equal(t, "unifi", spec.Provider.Name)
	assert.NotNil(t, spec.Provider.Schema)
	assert.Len(t, spec.DataSources, 0)
	assert.Len(t, spec.Resources, 0)
}

func TestSpecificationGenerator_Generate_ProviderAttributes(t *testing.T) {
	gen := NewSpecificationGenerator("unifi", nil)
	spec := gen.Generate()

	require.NotNil(t, spec.Provider.Schema)
	attrs := spec.Provider.Schema.Attributes

	attrNames := make(map[string]bool)
	for _, attr := range attrs {
		attrNames[attr.Name] = true
	}

	assert.True(t, attrNames["username"])
	assert.True(t, attrNames["password"])
	assert.True(t, attrNames["api_url"])
	assert.True(t, attrNames["site"])
	assert.True(t, attrNames["allow_insecure"])
}

func TestSpecificationGenerator_Generate_SimpleResource(t *testing.T) {
	gen := NewSpecificationGenerator("unifi", nil)

	resource := NewResource("Network", "network")
	resource.Types["Network"].Fields["Name"] = NewFieldInfo("Name", "name", "string", "", false, false, false, "")
	resource.Types["Network"].Fields["Purpose"] = NewFieldInfo("Purpose", "purpose", "string", "", true, false, false, "")
	resource.Types["Network"].Fields["Enabled"] = NewFieldInfo("Enabled", "enabled", "bool", "", false, false, false, "")
	resource.Types["Network"].Fields["VLANID"] = NewFieldInfo("VLANID", "vlan_id", "int64", "", true, false, false, "")

	gen.AddResource(resource)
	spec := gen.Generate()

	require.Len(t, spec.DataSources, 1)
	ds := spec.DataSources[0]
	assert.Equal(t, "network", ds.Name)
	require.NotNil(t, ds.Schema)

	require.Len(t, spec.Resources, 1)
	res := spec.Resources[0]
	assert.Equal(t, "network", res.Name)
	require.NotNil(t, res.Schema)

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
	gen := NewSpecificationGenerator("unifi", nil)

	resource := NewResource("FirewallGroup", "firewallgroup")
	resource.Types["FirewallGroup"].Fields["Members"] = NewFieldInfo("Members", "members", "string", "", true, true, false, "")

	gen.AddResource(resource)
	spec := gen.Generate()

	require.Len(t, spec.Resources, 1)
	res := spec.Resources[0]

	i := slices.IndexFunc(res.Schema.Attributes, findAttr("members"))

	require.GreaterOrEqual(t, i, 0)
}

func TestSpecificationGenerator_Generate_NestedAttribute(t *testing.T) {
	gen := NewSpecificationGenerator("unifi", nil)

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

	i := slices.IndexFunc(res.Schema.Attributes, findAttr("config_network"))

	require.GreaterOrEqual(t, i, 0)
	configNetworkAttr := &res.Schema.Attributes[i]

	require.NotNil(t, configNetworkAttr)
	require.NotNil(t, configNetworkAttr.SingleNested)
	require.Len(t, configNetworkAttr.SingleNested.Attributes, 2)
}

func TestSpecificationGenerator_Generate_NestedArrayAttribute(t *testing.T) {
	gen := NewSpecificationGenerator("unifi", nil)

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

func TestSpecificationGenerator_Generate_SkipsSettings(t *testing.T) {
	gen := NewSpecificationGenerator("unifi", nil)

	resource := NewResource("Network", "network")
	gen.AddResource(resource)

	setting := NewResource("SettingGlobalAp", "setting_global_ap")
	gen.AddResource(setting)

	spec := gen.Generate()

	assert.Len(t, spec.DataSources, 1)
	assert.Len(t, spec.Resources, 1)
	assert.Equal(t, "network", spec.DataSources[0].Name)
	assert.Equal(t, "network", spec.Resources[0].Name)
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
	gen := NewSpecificationGenerator("unifi", nil)

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
