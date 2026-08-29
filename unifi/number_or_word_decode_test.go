package unifi

import (
	"encoding/json"
	"testing"

	"github.com/ubiquiti-community/go-unifi/unifi/settings"
)

// Some controller validators accept the same value written two ways: a bare
// number, or a word. Protocol is 47 or "gre"; guest access expires after a
// count of units, or "custom". The field is modelled as a string so the word
// survives, and the controller serialises the numeric form as a JSON number
// -- which does not decode into a Go string.
//
// A read of an object with such a field set then fails outright, which is how
// this surfaced: GetSetting for guest access could not decode a controller
// that had expire set (measured on 10.6.101).
//
// The decision is derived from the validator now rather than listed by hand
// (cmd/fields/number_or_word.go). These are the fields it currently covers,
// each asserted against both wire forms.
func TestNumberOrWordFieldsDecodeBothWireForms(t *testing.T) {
	cases := []struct {
		name   string
		number string
		word   string
		decode func(t *testing.T, body string) string
	}{
		{
			name: "FirewallPolicy.Protocol", number: `{"protocol":47}`, word: `{"protocol":"gre"}`,
			decode: func(t *testing.T, body string) string {
				var v FirewallPolicy
				mustUnmarshal(t, body, &v)
				return v.Protocol
			},
		},
		{
			name: "FirewallRule.Protocol", number: `{"protocol":47}`, word: `{"protocol":"gre"}`,
			decode: func(t *testing.T, body string) string {
				var v FirewallRule
				mustUnmarshal(t, body, &v)
				return v.Protocol
			},
		},
		{
			name: "FirewallRule.ProtocolV6", number: `{"protocol_v6":58}`, word: `{"protocol_v6":"icmpv6"}`,
			decode: func(t *testing.T, body string) string {
				var v FirewallRule
				mustUnmarshal(t, body, &v)
				return v.ProtocolV6
			},
		},
		{
			name: "DeviceQOSMatching.Protocol", number: `{"protocol":47}`, word: `{"protocol":"gre"}`,
			decode: func(t *testing.T, body string) string {
				var v DeviceQOSMatching
				mustUnmarshal(t, body, &v)
				return v.Protocol
			},
		},
		{
			name: "PortProfileQOSMatching.Protocol", number: `{"protocol":47}`, word: `{"protocol":"gre"}`,
			decode: func(t *testing.T, body string) string {
				var v PortProfileQOSMatching
				mustUnmarshal(t, body, &v)
				return v.Protocol
			},
		},
		{
			name: "settings.GuestAccess.Expire", number: `{"expire":480}`, word: `{"expire":"custom"}`,
			decode: func(t *testing.T, body string) string {
				var v settings.GuestAccess
				mustUnmarshal(t, body, &v)
				return v.Expire
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.decode(t, c.number); got == "" {
				t.Errorf("the numeric wire form decoded to an empty string; "+
					"the controller may send this field as a JSON number, so it has to "+
					"decode through types.Number (body %s)", c.number)
			}
			if got := c.decode(t, c.word); got == "" {
				t.Errorf("the word wire form decoded to an empty string (body %s)", c.word)
			}
		})
	}
}

func mustUnmarshal(t *testing.T, body string, dst any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), dst); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
}
