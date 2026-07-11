package settings

// ProviderCapabilities is the site-level ISP capability setting (key
// "provider_capabilities"): the advertised download/upload capacity of the
// internet connection, used by the controller for utilization displays and
// Smart Queues sizing.
//
// Hand-maintained: the ace.jar field spec (9.5.21) does not define this
// setting, but controllers expose it at
// /api/s/<site>/{get,set}/setting/provider_capabilities.
//
// The controller returns download/upload as JSON numbers (kbps; observed
// 1000000 on a 1 Gbps WAN), so plain int64 fields with the default JSON
// (un)marshalling are sufficient — no custom UnmarshalJSON is needed.
type ProviderCapabilities struct {
	BaseSetting

	Download int64 `json:"download,omitempty"`
	Upload   int64 `json:"upload,omitempty"`
}
