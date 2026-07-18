package main

import "github.com/ubiquiti-community/go-unifi/internal/fields"

// AddReviewedLegacySchemas preserves Terraform provider APIs whose vendor
// definitions disappeared in UniFi Network 10.4.57. These are frozen from the
// last generated go-unifi 9.5.21 specification, not attributed to the current
// vendor schema digest. The generated-tree digest still binds their exact
// serialized representation. A live schema wins automatically if one of these
// names reappears in a future release.
func (g *SpecificationGenerator) AddReviewedLegacySchemas() {
	existing := make(map[string]struct{}, len(g.Resources))
	for _, resource := range g.Resources {
		existing[toTerraformName(resource.StructName)] = struct{}{}
	}
	for _, resource := range reviewedLegacySchemas() {
		if _, ok := existing[toTerraformName(resource.StructName)]; ok {
			continue
		}
		g.AddResource(resource)
	}
}

func reviewedLegacySchemas() []*ResourceInfo {
	return []*ResourceInfo{
		legacySchema("HeatMap", "heatmap", map[string]*FieldInfo{
			"Description": NewFieldInfo("Description", "description", fields.String, "", true, false, false, ""),
			"MapID":       NewFieldInfo("MapID", "map_id", fields.String, "", true, false, false, ""),
			"Name":        NewFieldInfo("Name", "name", fields.String, "", true, false, false, ""),
			"Type":        NewFieldInfo("Type", "type", fields.String, "", true, false, false, ""),
		}),
		legacySchema("HeatMapPoint", "heatmappoint", map[string]*FieldInfo{
			"DownloadSpeed": NewFieldInfo("DownloadSpeed", "download_speed", "float64", "", true, false, false, ""),
			"HeatmapID":     NewFieldInfo("HeatmapID", "heatmap_id", fields.String, "", true, false, false, ""),
			"UploadSpeed":   NewFieldInfo("UploadSpeed", "upload_speed", "float64", "", true, false, false, ""),
			"X":             NewFieldInfo("X", "x", "float64", "", true, false, false, ""),
			"Y":             NewFieldInfo("Y", "y", "float64", "", true, false, false, ""),
		}),
		legacySchema("Map", "map", map[string]*FieldInfo{
			"Lat":        NewFieldInfo("Lat", "lat", fields.String, "", true, false, false, ""),
			"Lng":        NewFieldInfo("Lng", "lng", fields.String, "", true, false, false, ""),
			"MapTypeID":  NewFieldInfo("MapTypeID", "mapTypeId", fields.String, "", true, false, false, ""),
			"Name":       NewFieldInfo("Name", "name", fields.String, "", true, false, false, ""),
			"OffsetLeft": NewFieldInfo("OffsetLeft", "offset_left", "float64", "", true, false, false, ""),
			"OffsetTop":  NewFieldInfo("OffsetTop", "offset_top", "float64", "", true, false, false, ""),
			"Opacity":    NewFieldInfo("Opacity", "opacity", "float64", "", true, false, false, ""),
			"Selected":   NewFieldInfo("Selected", "selected", fields.Bool, "", false, false, false, ""),
			"Tilt":       NewFieldInfo("Tilt", "tilt", fields.Int, "", true, false, true, ""),
			"Type":       NewFieldInfo("Type", "type", fields.String, "", true, false, false, ""),
			"Unit":       NewFieldInfo("Unit", "unit", fields.String, "", true, false, false, ""),
			"Upp":        NewFieldInfo("Upp", "upp", "float64", "", true, false, false, ""),
			"Zoom":       NewFieldInfo("Zoom", "zoom", fields.Int, "", true, false, true, ""),
		}),
		legacySchema("Tag", "tag", map[string]*FieldInfo{
			"MemberTable": NewFieldInfo("MemberTable", "member_table", fields.String, "", true, true, false, ""),
			"Name":        NewFieldInfo("Name", "name", fields.String, "", true, false, false, ""),
		}),
		legacySchema("VirtualDevice", "virtualdevice", map[string]*FieldInfo{
			"HeightInMeters": NewFieldInfo("HeightInMeters", "heightInMeters", "float64", "", true, false, false, ""),
			"Locked":         NewFieldInfo("Locked", "locked", fields.Bool, "", false, false, false, ""),
			"MapID":          NewFieldInfo("MapID", "map_id", fields.String, "", true, false, false, ""),
			"Type":           NewFieldInfo("Type", "type", fields.String, "", true, false, false, ""),
			"X":              NewFieldInfo("X", "x", fields.String, "", true, false, false, ""),
			"Y":              NewFieldInfo("Y", "y", fields.String, "", true, false, false, ""),
		}),
	}
}

func legacySchema(structName, resourcePath string, legacyFields map[string]*FieldInfo) *ResourceInfo {
	resource := NewResource(structName, resourcePath)
	for name, field := range legacyFields {
		resource.Types[structName].Fields[name] = field
	}
	return resource
}
