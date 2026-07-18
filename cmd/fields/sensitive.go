package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sensitiveMetadata mirrors the parts of upstream's sensitive_metadata.json
// we use. Ignored keys: min_field_size, default_names,
// sensitive_system_properties, sensitive_distinct_db_fields_by_collection.
type sensitiveMetadata struct {
	ByCollection map[string][]string `json:"sensitive_db_fields_by_collection"`
}

// sensitiveDisplayFields are identifiers/display metadata that UniFi flags
// sensitive for PII-redaction reasons. Marking them Sensitive in Terraform
// would hide resource names from plan output with no security benefit, so
// they are allowlisted out. Everything else upstream lists is treated as a
// secret (fail-safe: new upstream fields default to sensitive).
//
// If a field ever needs per-collection granularity, keys become
// "collection.field".
var sensitiveDisplayFields = map[string]bool{
	// names & descriptions
	"name": true, "desc": true, "hostname": true, "host_name": true,
	"domain_name": true, "networkgroup": true,
	// identity / PII
	"email": true, "first_name": true, "last_name": true,
	"ubic_name": true, "ubic_uuid": true,
	"anonymous_id": true, "anonymous_device_id": true, "serial": true,
	// usernames & endpoints (the secrets are the passwords, not these)
	"login": true, "wan_username": true, "openvpn_username": true,
	"x_ssh_username": true, "lte_username": true,
	"management_ip": true, "management_peer_ip": true,
	"ipsec_key_exchange": true,
	// device radio identifiers
	"lte_imei": true, "lte_iccid": true, "lte_apn": true,
	"lte_networkoperator": true,
}

// loadSensitiveMetadata reads <fieldsDir>/metadata/sensitive_metadata.json.
// Returns (nil, nil) when absent (deb-sourced fields dirs).
func loadSensitiveMetadata(fieldsDir string) (*sensitiveMetadata, error) {
	b, err := os.ReadFile(filepath.Join(fieldsDir, "metadata", "sensitive_metadata.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var meta sensitiveMetadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, fmt.Errorf("unable to parse sensitive_metadata.json: %w", err)
	}
	return &meta, nil
}

// collectionForResource maps a resource to its UniFi DB collection: the
// lowercase basename of the source fields file, with exceptions for the
// resources whose struct names were cleaned up.
func collectionForResource(sourceFile, structName string) string {
	if strings.HasPrefix(structName, "Setting") {
		return "setting"
	}
	switch structName {
	case "Client":
		return "user"
	case "ClientGroup":
		return "usergroup"
	}
	return strings.ToLower(strings.TrimSuffix(sourceFile, ".json"))
}

// markResource flags the resource's fields listed for collection. Paths may
// be dot-separated ("auth_servers.x_secret") and are walked by JSON name.
// Drift (unknown collections/paths) is logged and skipped, never fatal.
func (m *sensitiveMetadata) markResource(r *ResourceInfo, collection string) {
	paths, ok := m.ByCollection[collection]
	if !ok {
		fmt.Printf("sensitive metadata: no entry for collection %q\n", collection)
		return
	}

	root := r.Types[r.StructName]
	for _, p := range paths {
		leaf := p
		if i := strings.LastIndex(p, "."); i >= 0 {
			leaf = p[i+1:]
		}
		if sensitiveDisplayFields[leaf] {
			continue
		}
		f := findFieldByJSONPath(root, p)
		if f == nil {
			fmt.Printf("sensitive metadata: %s.%s not found in schema\n", collection, p)
			continue
		}
		f.Sensitive = true
	}
}

// findFieldByJSONPath walks a dot-separated path of JSON field names from
// root, returning the leaf field or nil when any segment is missing.
func findFieldByJSONPath(root *FieldInfo, path string) *FieldInfo {
	cur := root
	for _, seg := range strings.Split(path, ".") {
		if cur == nil {
			return nil
		}
		var next *FieldInfo
		for _, f := range cur.Fields {
			if f != nil && f.JSONName == seg {
				next = f
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}
