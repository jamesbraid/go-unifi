package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

// markResource flags the resource's fields listed for collection, returning
// the number of fields marked and the listed paths not found in the schema.
// An unknown collection is not drift: it just means the resource has no
// sensitive fields, so it returns (0, nil).
func (m *sensitiveMetadata) markResource(r *ResourceInfo, collection string) (int, []string) {
	paths, ok := m.ByCollection[collection]
	if !ok {
		return 0, nil
	}

	root := r.Types[r.StructName]
	marked := 0
	var missed []string
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
			missed = append(missed, p)
			continue
		}
		f.Sensitive = true
		marked++
	}
	return marked, missed
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

// sensitiveAudit aggregates markResource results across a codegen run so the
// operator gets one compact audit block instead of per-resource log spam.
type sensitiveAudit struct {
	consumed   map[string]bool
	resources  map[string]int            // collection -> resources recorded
	missCounts map[string]map[string]int // collection -> path -> resources missing it
	marked     int
}

func newSensitiveAudit() *sensitiveAudit {
	return &sensitiveAudit{
		consumed:   map[string]bool{},
		resources:  map[string]int{},
		missCounts: map[string]map[string]int{},
	}
}

// record marks one resource's sensitive fields and tracks the collection,
// the marked count, and any missed (listed-but-not-found) paths.
func (a *sensitiveAudit) record(m *sensitiveMetadata, r *ResourceInfo, collection string) {
	marked, missed := m.markResource(r, collection)
	a.consumed[collection] = true
	a.resources[collection]++
	a.marked += marked
	for _, p := range missed {
		if a.missCounts[collection] == nil {
			a.missCounts[collection] = map[string]int{}
		}
		a.missCounts[collection][p]++
	}
}

// lines returns the audit report lines (sorted for determinism):
//   - upstream collections with no generated resource (reverse audit)
//   - listed paths not found in any schema, deduped across resources
//   - a summary line: "sensitive metadata: marked N fields across M collections"
//
// A miss is only emitted when every resource in the collection missed the
// path: with ~43 Setting* resources sharing the "setting" collection, a path
// present in some schemas but not others is per-resource noise, while a path
// missing from all of them is real drift.
func (a *sensitiveAudit) lines(m *sensitiveMetadata) []string {
	var out []string

	var unconsumed []string
	for c := range m.ByCollection {
		if !a.consumed[c] {
			unconsumed = append(unconsumed, c)
		}
	}
	slices.Sort(unconsumed)
	for _, c := range unconsumed {
		out = append(out, fmt.Sprintf("sensitive metadata: collection %q has no generated resource", c))
	}

	var missLines []string
	for c, paths := range a.missCounts {
		for p, n := range paths {
			if n == a.resources[c] {
				missLines = append(missLines, fmt.Sprintf("sensitive metadata: %s.%s not found in any schema", c, p))
			}
		}
	}
	slices.Sort(missLines)
	out = append(out, missLines...)

	collections := 0
	for c := range a.consumed {
		if _, ok := m.ByCollection[c]; ok {
			collections++
		}
	}
	out = append(out, fmt.Sprintf("sensitive metadata: marked %d fields across %d collections", a.marked, collections))
	return out
}

// print writes lines() to stdout.
func (a *sensitiveAudit) print(m *sensitiveMetadata) {
	for _, l := range a.lines(m) {
		fmt.Println(l)
	}
}
