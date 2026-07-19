package settings

import (
	"bytes"
	"encoding/json"
)

// querierAddressesFromRaw decodes igmp_snooping querier_addresses elements
// tolerantly: 10.x controllers emit {mac, network_id, querier_address}
// objects, while pre-10.x controllers (and databases upgraded from them)
// carried plain address strings. A legacy string becomes an entry with only
// QuerierAddress set; elements that are neither shape are skipped rather
// than failing the whole settings read. Wired up via overrides/fields.toml.
func querierAddressesFromRaw(raw []json.RawMessage) []SettingIgmpSnoopingQuerierAddresses {
	if raw == nil {
		return nil
	}

	out := make([]SettingIgmpSnoopingQuerierAddresses, 0, len(raw))
	for _, elem := range raw {
		// json.Unmarshal(null, &string) succeeds without touching the
		// target; treat null elements as skippable, not as empty entries.
		if string(bytes.TrimSpace(elem)) == "null" {
			continue
		}

		var legacy string
		if err := json.Unmarshal(elem, &legacy); err == nil {
			out = append(out, SettingIgmpSnoopingQuerierAddresses{QuerierAddress: legacy})
			continue
		}

		var entry SettingIgmpSnoopingQuerierAddresses
		if err := json.Unmarshal(elem, &entry); err == nil {
			out = append(out, entry)
		}
	}
	return out
}
