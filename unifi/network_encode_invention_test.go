package unifi

import (
	"encoding/json"
	"sort"
	"testing"
)

// networkEncoderJustifiedNonZero lists wire names the encoder is allowed to
// emit with a non-zero value even though the caller set nothing, keyed by
// purpose. Everything else must come out at its zero value or not at all.
//
// Keep this list short and each entry justified. An entry here is the encoder
// deciding something on the caller's behalf, which is exactly the class of bug
// that made a guarded corporate network un-creatable (dhcpguard_enabled sent
// without dhcpd_ip_1) and silently discarded six advanced fields
// (setting_preference forced to "auto").
var networkEncoderJustifiedNonZero = map[string]map[string]string{
	PurposeCorporate: {
		"purpose": "identity: the field MarshalJSON dispatches on",
	},
	PurposeGuest: {
		"purpose": "identity: the field MarshalJSON dispatches on",
	},
	PurposeVLANOnly: {
		"purpose": "identity: the field MarshalJSON dispatches on",
		// Measured on 10.4.57: a corporate or guest network created without
		// networkgroup comes back with "LAN" anyway, so the default was
		// dropped there. A vlan-only network created without it comes back
		// with no networkgroup at all, so dropping it here would change what
		// is stored rather than restate it. Whether that matters on an
		// L2-only network is untested, and not worth finding out on a guess.
		"networkgroup": "controller does not default networkgroup for vlan-only, unlike corporate and guest",
	},
	PurposeWAN: {
		"purpose": "identity: the field MarshalJSON dispatches on",
	},
	PurposeSiteVPN: {
		"purpose": "identity: the field MarshalJSON dispatches on",
	},
	PurposeVPNClient: {
		"purpose": "identity: the field MarshalJSON dispatches on",
	},
	PurposeUserVPN: {
		"purpose": "identity: the field MarshalJSON dispatches on",
	},
}

// isZeroJSON reports whether v is the JSON rendering of a Go zero value:
// null, false, "", 0, an empty array or an empty object. Emitting one of
// these for a field the caller never set is fine -- it says nothing the zero
// value did not already say. Emitting anything else is the encoder inventing.
func isZeroJSON(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case bool:
		return !t
	case string:
		return t == ""
	case float64:
		return t == 0
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}

// TestNetworkEncoderInventsNothing marshals an otherwise-zero Network for
// every purpose and fails on any field the encoder filled in by itself.
//
// The existing coverage tests cannot catch this. TestNetworkEncoderValueFlow
// populates every field non-zero, so a valueOrDefault(n.X, "...") default
// never fires -- it only fires when the caller left the field nil, which is
// the case that ships broken. This test is that case.
func TestNetworkEncoderInventsNothing(t *testing.T) {
	for _, purpose := range networkEncoderPurposes {
		t.Run(purpose, func(t *testing.T) {
			data, err := json.Marshal(&Network{Purpose: purpose})
			if err != nil {
				t.Fatalf("marshal %s: %v", purpose, err)
			}
			var got map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal %s: %v", purpose, err)
			}

			justified := networkEncoderJustifiedNonZero[purpose]
			invented := []string{}
			for k, v := range got {
				if isZeroJSON(v) {
					continue
				}
				if _, ok := justified[k]; ok {
					continue
				}
				invented = append(invented, k)
			}
			sort.Strings(invented)

			for _, k := range invented {
				t.Errorf("%s: encoder invented %s = %v; the caller set nothing.\n"+
					"Pass the field through instead, or add it to networkEncoderJustifiedNonZero "+
					"with a reason measured against a live controller.", purpose, k, got[k])
			}

			// A justified entry that stops being emitted is stale bookkeeping.
			for k := range justified {
				if _, ok := got[k]; !ok {
					t.Errorf("%s: %q is listed in networkEncoderJustifiedNonZero but is no longer emitted; drop it", purpose, k)
				}
			}
		})
	}
}
