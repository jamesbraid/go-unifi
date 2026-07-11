package settings

// Ipsec is the site-level IPsec setting (key "ipsec").
//
// Hand-maintained: the ace.jar field spec (9.5.21) does not define this
// setting, but newer controllers expose it at
// /api/s/<site>/{get,set}/setting/ipsec. It carries the IKEv2
// re-authentication behavior for site-to-site VPNs.
//
// All fields are simple scalars, so the default JSON (un)marshalling of the
// embedded BaseSetting plus these fields is sufficient — no custom
// UnmarshalJSON is needed.
type Ipsec struct {
	BaseSetting

	Ikev2ReauthenticationMethod string `json:"ikev2_reauthentication_method,omitempty"` // observed: make-before-break
}
