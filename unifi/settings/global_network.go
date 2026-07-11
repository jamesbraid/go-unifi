package settings

// GlobalNetwork is the site-level global network setting (key
// "global_network").
//
// Hand-maintained: the ace.jar field spec (9.5.21) does not define this
// setting, but newer controllers expose it at
// /api/s/<site>/{get,set}/setting/global_network. It carries the site-wide
// default security posture used by zone-based firewalling.
//
// All fields are simple scalars, so the default JSON (un)marshalling of the
// embedded BaseSetting plus these fields is sufficient — no custom
// UnmarshalJSON is needed.
type GlobalNetwork struct {
	BaseSetting

	DefaultSecurityPosture string `json:"default_security_posture,omitempty"` // observed: ALLOW_ALL
}
