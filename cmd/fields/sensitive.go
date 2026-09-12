package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// sensitiveIndex maps a controller collection name (lowercased schema file
// base name, e.g. "wlanconf") to the set of wire field leaf names UniFi
// lists in sensitive_metadata.json.
type sensitiveIndex map[string]map[string]bool

// loadSensitiveMetadata builds a sensitiveIndex from the controller's
// sensitive_metadata.json. A missing file yields a nil index, and the
// redaction list the generator emits from it is then empty -- the client
// falls back to its substring guesses, which is the state this replaced.
func loadSensitiveMetadata(path string) (sensitiveIndex, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Values are usually lists of field names, but single-field entries ship
	// as a bare string (e.g. "rogue": "essid" in the distinct section).
	var meta struct {
		ByCollection         map[string]any `json:"sensitive_db_fields_by_collection"`
		DistinctByCollection map[string]any `json:"sensitive_distinct_db_fields_by_collection"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, fmt.Errorf("unable to parse sensitive metadata: %w", err)
	}

	index := make(sensitiveIndex)
	addLeaf := func(collection string, field string) {
		leaves := index[collection]
		if leaves == nil {
			leaves = make(map[string]bool)
			index[collection] = leaves
		}
		// Nested entries are dotted paths (auth_servers.x_secret);
		// FieldInfo carries leaf wire names, so index the leaf.
		parts := strings.Split(field, ".")
		leaves[parts[len(parts)-1]] = true
	}

	for _, byCollection := range []map[string]any{meta.ByCollection, meta.DistinctByCollection} {
		for collection, value := range byCollection {
			switch entry := value.(type) {
			case string:
				addLeaf(collection, entry)
			case []any:
				for _, field := range entry {
					name, ok := field.(string)
					if !ok {
						return nil, fmt.Errorf("unexpected sensitive metadata entry %v for %s", field, collection)
					}
					addLeaf(collection, name)
				}
			default:
				return nil, fmt.Errorf("unexpected sensitive metadata shape %T for %s", value, collection)
			}
		}
	}

	return index, nil
}

// secretNameRe separates secret material from the anonymization-only entries
// in sensitive_metadata.json (name, hostname, serial, usernames,
// certificates, ...). ipsec_key_exchange is a protocol setting, which is why
// the key match is suffix-anchored.
var secretNameRe = regexp.MustCompile(`(?i)passw|passphrase|secret|token|psk|sim_pin|private_key|auth_?key|_key$`)
