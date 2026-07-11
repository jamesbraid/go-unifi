package settings

// UsgGeo is the site-level geo IP filtering setting (key "usg_geo").
//
// Hand-maintained: the ace.jar field spec (9.5.21) does not define this
// setting, but controllers expose it at
// /api/s/<site>/{get,set}/setting/usg_geo. It carries the country-based
// traffic filter (allow/block lists by country code and direction).
//
// SettingUsgGeoIPFiltering keeps the "Setting" prefix to match the
// generator's naming for nested setting types (see
// SettingGlobalSwitchAclL3Isolation). All fields are simple scalars, so the
// default JSON (un)marshalling of the embedded BaseSetting plus these fields
// is sufficient — no custom UnmarshalJSON is needed.
type UsgGeo struct {
	BaseSetting

	IPFiltering *SettingUsgGeoIPFiltering `json:"ip_filtering,omitempty"`
}

// SettingUsgGeoIPFiltering is the nested ip_filtering object of UsgGeo.
type SettingUsgGeoIPFiltering struct {
	Action           string `json:"action,omitempty"`    // observed: block
	Countries        string `json:"countries,omitempty"` // comma-separated country codes
	Enabled          bool   `json:"enabled"`
	TrafficDirection string `json:"traffic_direction,omitempty"` // observed: both
}
