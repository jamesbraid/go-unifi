package scout

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ubiquiti-community/go-unifi/control/dnsrecord"
)

const (
	dnsScenarioPath     = "/v2/api/site/{site}/static-dns"
	normalizationPolicy = "field_presence_and_json_type_v1"
	redactionPolicy     = "drop_all_observed_values_v1"
	cleanupPolicy       = "not_required_read_only"
)

var ignoredControllerFields = map[string]struct{}{
	"_id":            {},
	"site_id":        {},
	"attr_hidden":    {},
	"attr_hidden_id": {},
	"attr_no_delete": {},
	"attr_no_edit":   {},
}

// BuildDNSCatalog joins a policy-free DNS structural projection with one raw
// controller response. Observed values never enter either output.
func BuildDNSCatalog(input Input) (Result, error) {
	if input.Target.Name == "" || input.Target.Product == "" || input.Target.Version == "" ||
		input.Target.Architecture == "" || input.Target.ImageIndexSHA256 == "" ||
		input.Target.ImageManifestSHA256 == "" || input.Target.ControllerFingerprint == "" {
		return Result{}, fmt.Errorf("immutable target digest and identity are required")
	}
	if input.Scenario.Mode != "read_only" {
		return Result{}, fmt.Errorf("unsupported scenario mode %q", input.Scenario.Mode)
	}
	if input.Scenario.ID == "" || input.Scenario.Resource != "dns_record" ||
		input.Scenario.Method != "GET" || input.Scenario.Path != dnsScenarioPath {
		return Result{}, fmt.Errorf("incomplete or unsupported DNS scenario")
	}
	if input.LockedSources.CaptureLockSHA256 == "" || input.LockedSources.ExtractionRulesSHA256 == "" ||
		input.LockedSources.ControllerNetworkVersion == "" || input.LockedSources.StructuralSHA256 == "" ||
		input.LockedSources.SensitivitySHA256 == "" || input.LockedSources.StructuralProjectionSHA256 == "" ||
		input.LockedSources.SemanticPredecessorSHA256 == "" {
		return Result{}, fmt.Errorf("complete locked source digests are required")
	}
	if input.Target.Version != input.LockedSources.ControllerNetworkVersion {
		return Result{}, fmt.Errorf("target version does not match capture lock Network version")
	}
	measuredTargetFingerprint := ""
	switch input.ExecutionMode {
	case "fixture":
		if input.TargetReceipt != nil || input.ControllerVersion != "" || input.ObservedInstanceIdentitySHA256 != "" {
			return Result{}, fmt.Errorf("fixture execution cannot claim a measured target receipt or observed controller identity")
		}
	case "live":
		if input.TargetReceipt == nil {
			return Result{}, fmt.Errorf("live execution requires a measured target receipt")
		}
		if input.ControllerVersion == "" {
			return Result{}, fmt.Errorf("live execution requires the observed controller version")
		}
		if !validPrefixedSHA256(input.ObservedInstanceIdentitySHA256) {
			return Result{}, fmt.Errorf("live execution requires an observed instance identity SHA-256")
		}
		if input.ControllerVersion != input.Target.Version || input.ControllerVersion != input.LockedSources.ControllerNetworkVersion {
			return Result{}, fmt.Errorf("observed controller version %q does not match target profile and capture lock", input.ControllerVersion)
		}
		if err := targetReceiptMatchesProfile(*input.TargetReceipt, input.Target); err != nil {
			return Result{}, err
		}
		if input.ObservedInstanceIdentitySHA256 != input.TargetReceipt.InstanceIdentitySHA256 {
			return Result{}, fmt.Errorf("observed controller instance identity does not match measured target receipt")
		}
		var err error
		measuredTargetFingerprint, err = ProvisionerReceiptFingerprint(*input.TargetReceipt)
		if err != nil {
			return Result{}, err
		}
		if measuredTargetFingerprint != input.Target.ControllerFingerprint {
			return Result{}, fmt.Errorf("measured target receipt fingerprint does not match target profile")
		}
	default:
		return Result{}, fmt.Errorf("unsupported execution mode %q", input.ExecutionMode)
	}

	fields, err := loadStructuralProjection(input)
	if err != nil {
		return Result{}, err
	}
	semanticIDs, tombstones, migrations, err := loadSemanticRegistry(input, fields)
	if err != nil {
		return Result{}, err
	}
	semanticIDsDigest, err := canonicalDocumentDigest(input.SemanticIDs)
	if err != nil {
		return Result{}, fmt.Errorf("canonicalize semantic IDs: %w", err)
	}
	records, err := observedObjects(input.ObservedResponse)
	if err != nil {
		return Result{}, err
	}

	structuralNames := make([]string, 0, len(fields))
	structuralByName := make(map[string]structuralRecord, len(fields))
	for name, field := range fields {
		definition, err := json.Marshal(field)
		if err != nil {
			return Result{}, fmt.Errorf("encode structural field %q: %w", name, err)
		}
		structuralNames = append(structuralNames, name)
		structuralByName[name] = structuralRecord{
			ID:               semanticIDs[name],
			Field:            name,
			Type:             field.JSONType,
			DefinitionSHA256: digest(definition),
			SecretCandidate:  field.SecretCandidate,
		}
	}
	sort.Strings(structuralNames)

	observed := make(map[string]*observedRecord, len(fields))
	for _, name := range structuralNames {
		observed[name] = &observedRecord{
			ID:       semanticIDs[name],
			Field:    name,
			JSONType: "unobserved",
		}
	}
	conflicts := make([]conflict, 0)
	redactedFields := 0
	for _, object := range records {
		for name, raw := range object {
			if _, ignored := ignoredControllerFields[name]; ignored {
				redactedFields++
				continue
			}
			field, known := fields[name]
			if !known {
				if unsafeFieldName(name) {
					return Result{}, fmt.Errorf("unsafe observed field %q", name)
				}
				conflicts = append(conflicts, conflict{Kind: "unknown_observed_field", Field: name})
				redactedFields++
				continue
			}
			entry := observed[name]
			entry.PresentCount++
			observedType, nonNull, err := jsonValueType(raw)
			if err != nil {
				return Result{}, fmt.Errorf("field %q: %w", name, err)
			}
			if nonNull {
				entry.NonNullCount++
			}
			if entry.JSONType == "unobserved" || entry.JSONType == "null" {
				entry.JSONType = observedType
			} else if observedType != "null" && observedType != entry.JSONType {
				entry.JSONType = "mixed"
			}
			if nonNull && !compatibleType(field.JSONType, observedType) {
				conflicts = append(conflicts, conflict{
					Kind:     "type_mismatch",
					Field:    name,
					Expected: field.JSONType,
					Observed: observedType,
				})
			}
		}
	}

	structuralRecords := make([]structuralRecord, 0, len(structuralNames))
	observedRecords := make([]observedRecord, 0, len(structuralNames))
	coverage := make([]coverageRecord, 0, len(structuralNames))
	complete := true
	for _, name := range structuralNames {
		structuralRecords = append(structuralRecords, structuralByName[name])
		entry := *observed[name]
		observedRecords = append(observedRecords, entry)
		state := "observed"
		if entry.PresentCount == 0 {
			state = "not_observed"
			complete = false
		}
		coverage = append(coverage, coverageRecord{ID: entry.ID, State: state})
	}
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Field == conflicts[j].Field {
			return conflicts[i].Kind < conflicts[j].Kind
		}
		return conflicts[i].Field < conflicts[j].Field
	})

	operationDigest := dnsrecord.OperationDigest()
	admissionState := "candidate"
	if len(conflicts) > 0 || !complete {
		admissionState = "blocked"
	}
	observationBytes, err := encodeCanonical(observedRecords)
	if err != nil {
		return Result{}, fmt.Errorf("encode observations: %w", err)
	}

	catalogBytes, err := encodeCanonical(catalog{
		FormatVersion: 1,
		CatalogID:     fmt.Sprintf("unifi.network.dns_record@%s", input.Target.Version),
		Target:        input.Target,
		Sources: catalogSources{
			CaptureLockSHA256:          input.LockedSources.CaptureLockSHA256,
			StructuralProjectionSHA256: input.LockedSources.StructuralProjectionSHA256,
			SemanticPredecessorSHA256:  input.LockedSources.SemanticPredecessorSHA256,
			SemanticIDsSHA256:          semanticIDsDigest,
		},
		StructuralRecords: structuralRecords,
		ObservedRecords:   observedRecords,
		Conflicts:         conflicts,
		Coverage:          coverage,
		Admission: admission{
			State:           admissionState,
			OperationDigest: operationDigest,
		},
		Tombstones: tombstones,
		Migrations: migrations,
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode catalog: %w", err)
	}
	receiptBytes, err := encodeCanonical(scenarioReceipt{
		FormatVersion:                  1,
		ScenarioID:                     input.Scenario.ID,
		ScenarioPath:                   input.Scenario.Path,
		ScenarioMode:                   input.Scenario.Mode,
		RequestShape:                   requestShape{Method: input.Scenario.Method, Path: input.Scenario.Path, Query: "none", Body: "none"},
		ExecutionMode:                  input.ExecutionMode,
		MeasuredTargetFingerprint:      measuredTargetFingerprint,
		ControllerVersion:              input.ControllerVersion,
		ObservedInstanceIdentitySHA256: input.ObservedInstanceIdentitySHA256,
		OperationDigest:                operationDigest,
		ResponseSHA256:                 digest(input.ObservedResponse),
		Normalization:                  normalizationPolicy,
		Redaction:                      redactionPolicy,
		Cleanup:                        cleanupPolicy,
		ObservedRecordCount:            len(records),
		RedactedFieldCount:             redactedFields,
		CanonicalObservationSHA256:     digest(observationBytes),
		Verdict:                        admissionState,
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode scenario receipt: %w", err)
	}
	return Result{Catalog: catalogBytes, Receipt: receiptBytes}, nil
}

func loadStructuralProjection(input Input) (map[string]structuralProjectionField, error) {
	var projection structuralProjection
	if err := decodeStrictSingleJSON(input.StructuralProjection, &projection); err != nil {
		return nil, fmt.Errorf("decode structural projection: %w", err)
	}
	if projection.FormatVersion != 1 || projection.Resource != "dns_record" {
		return nil, fmt.Errorf("unsupported DNS structural projection")
	}
	if projection.Source.StructuralSHA256 != input.LockedSources.StructuralSHA256 {
		return nil, fmt.Errorf("structural snapshot digest does not match capture lock")
	}
	if projection.Source.SensitivitySHA256 != input.LockedSources.SensitivitySHA256 {
		return nil, fmt.Errorf("sensitivity snapshot digest does not match capture lock")
	}
	projectionDigest, err := canonicalDocumentDigest(input.StructuralProjection)
	if err != nil {
		return nil, fmt.Errorf("canonicalize structural projection: %w", err)
	}
	if projectionDigest != input.LockedSources.StructuralProjectionSHA256 {
		return nil, fmt.Errorf("structural projection digest does not match capture lock")
	}
	if len(projection.Fields) == 0 {
		return nil, fmt.Errorf("DNS structural projection has no fields")
	}
	fields := make(map[string]structuralProjectionField, len(projection.Fields))
	for _, field := range projection.Fields {
		if field.WireName == "" || !supportedJSONType(field.JSONType) {
			return nil, fmt.Errorf("invalid DNS structural field %q", field.WireName)
		}
		if _, exists := fields[field.WireName]; exists {
			return nil, fmt.Errorf("duplicate DNS structural field %q", field.WireName)
		}
		fields[field.WireName] = field
	}
	return fields, nil
}

func loadSemanticRegistry(input Input, fields map[string]structuralProjectionField) (map[string]string, []string, []catalogMigration, error) {
	priorIDs, err := loadSemanticPredecessor(input)
	if err != nil {
		return nil, nil, nil, err
	}
	var registry semanticRegistry
	if err := decodeStrictSingleJSON(input.SemanticIDs, &registry); err != nil {
		return nil, nil, nil, fmt.Errorf("decode semantic IDs: %w", err)
	}
	if registry.FormatVersion != 1 || registry.Resource != "dns_record" {
		return nil, nil, nil, fmt.Errorf("unsupported DNS semantic ID registry")
	}
	if registry.ExtractionRulesSHA256 != input.LockedSources.ExtractionRulesSHA256 {
		return nil, nil, nil, fmt.Errorf("extraction rules digest does not match capture lock")
	}
	if registry.Tombstones == nil || registry.Migrations == nil {
		return nil, nil, nil, fmt.Errorf("semantic ID registry must declare tombstones and migrations")
	}

	semanticIDs := make(map[string]string, len(registry.Fields))
	activeIDs := make(map[string]struct{}, len(registry.Fields))
	for _, field := range registry.Fields {
		if field.WireName == "" || !strings.HasPrefix(field.ID, "unifi.network.dns_record.") {
			return nil, nil, nil, fmt.Errorf("invalid semantic ID for field %q", field.WireName)
		}
		if _, exists := semanticIDs[field.WireName]; exists {
			return nil, nil, nil, fmt.Errorf("duplicate semantic mapping for field %q", field.WireName)
		}
		if _, exists := activeIDs[field.ID]; exists {
			return nil, nil, nil, fmt.Errorf("duplicate active semantic ID %q", field.ID)
		}
		if _, exists := fields[field.WireName]; !exists {
			return nil, nil, nil, fmt.Errorf("semantic field %q is absent from structural projection", field.WireName)
		}
		semanticIDs[field.WireName] = field.ID
		activeIDs[field.ID] = struct{}{}
	}
	for name := range fields {
		if _, exists := semanticIDs[name]; !exists {
			return nil, nil, nil, fmt.Errorf("structural field %q has no semantic ID", name)
		}
	}

	tombstoned := make(map[string]struct{}, len(registry.Tombstones))
	tombstones := make([]string, 0, len(registry.Tombstones))
	for _, tombstone := range registry.Tombstones {
		if tombstone.ID == "" || strings.TrimSpace(tombstone.Reason) == "" {
			return nil, nil, nil, fmt.Errorf("semantic tombstone requires ID and reason")
		}
		if _, existed := priorIDs[tombstone.ID]; !existed {
			return nil, nil, nil, fmt.Errorf("tombstone %q was not a prior semantic ID", tombstone.ID)
		}
		if _, active := activeIDs[tombstone.ID]; active {
			return nil, nil, nil, fmt.Errorf("active semantic ID %q cannot be tombstoned", tombstone.ID)
		}
		if _, duplicate := tombstoned[tombstone.ID]; duplicate {
			return nil, nil, nil, fmt.Errorf("duplicate semantic tombstone %q", tombstone.ID)
		}
		tombstoned[tombstone.ID] = struct{}{}
		tombstones = append(tombstones, tombstone.ID)
	}
	sort.Strings(tombstones)

	migrated := make(map[string]struct{}, len(registry.Migrations))
	migrations := make([]catalogMigration, len(registry.Migrations))
	copy(migrations, registry.Migrations)
	for _, migration := range migrations {
		if migration.FromID == "" || migration.ToID == "" || strings.TrimSpace(migration.Reason) == "" {
			return nil, nil, nil, fmt.Errorf("semantic migration requires from_id, to_id, and reason")
		}
		if !migration.Reviewed {
			return nil, nil, nil, fmt.Errorf("semantic migration from %q must be reviewed", migration.FromID)
		}
		if _, existed := priorIDs[migration.FromID]; !existed {
			return nil, nil, nil, fmt.Errorf("migration source %q was not a prior semantic ID", migration.FromID)
		}
		if _, active := activeIDs[migration.ToID]; !active {
			return nil, nil, nil, fmt.Errorf("migration target %q is not an active semantic ID", migration.ToID)
		}
		if _, duplicate := migrated[migration.FromID]; duplicate {
			return nil, nil, nil, fmt.Errorf("duplicate semantic migration from %q", migration.FromID)
		}
		migrated[migration.FromID] = struct{}{}
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].FromID < migrations[j].FromID })

	for id := range priorIDs {
		if _, active := activeIDs[id]; active {
			continue
		}
		if _, removed := tombstoned[id]; removed {
			continue
		}
		if _, moved := migrated[id]; moved {
			continue
		}
		return nil, nil, nil, fmt.Errorf("semantic registry strands predecessor semantic ID %q", id)
	}
	return semanticIDs, tombstones, migrations, nil
}

func loadSemanticPredecessor(input Input) (map[string]struct{}, error) {
	var predecessor semanticPredecessorRegistry
	if err := decodeStrictSingleJSON(input.SemanticPredecessor, &predecessor); err != nil {
		return nil, fmt.Errorf("decode semantic predecessor: %w", err)
	}
	if predecessor.FormatVersion != 1 || predecessor.Resource != "dns_record" || len(predecessor.Fields) == 0 {
		return nil, fmt.Errorf("unsupported DNS semantic predecessor registry")
	}
	documentDigest, err := canonicalDocumentDigest(input.SemanticPredecessor)
	if err != nil {
		return nil, fmt.Errorf("canonicalize semantic predecessor: %w", err)
	}
	if documentDigest != input.LockedSources.SemanticPredecessorSHA256 {
		return nil, fmt.Errorf("semantic predecessor digest does not match capture lock")
	}
	ids := make(map[string]struct{}, len(predecessor.Fields))
	wires := make(map[string]struct{}, len(predecessor.Fields))
	for _, field := range predecessor.Fields {
		if field.WireName == "" || !strings.HasPrefix(field.ID, "unifi.network.dns_record.") {
			return nil, fmt.Errorf("invalid predecessor semantic ID for field %q", field.WireName)
		}
		if _, duplicate := wires[field.WireName]; duplicate {
			return nil, fmt.Errorf("duplicate predecessor semantic field %q", field.WireName)
		}
		if _, duplicate := ids[field.ID]; duplicate {
			return nil, fmt.Errorf("duplicate predecessor semantic ID %q", field.ID)
		}
		wires[field.WireName] = struct{}{}
		ids[field.ID] = struct{}{}
	}
	return ids, nil
}

func observedObjects(document []byte) ([]map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(document)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, fmt.Errorf("observation must be an array")
	}
	var records []map[string]json.RawMessage
	if err := decodeSingleJSON(document, &records); err != nil {
		return nil, fmt.Errorf("decode observation: %w", err)
	}
	if records == nil {
		return nil, fmt.Errorf("observation must be an array")
	}
	return records, nil
}

func decodeStrictSingleJSON(document []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func decodeSingleJSON(document []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func jsonValueType(raw []byte) (string, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", false, err
	}
	switch value.(type) {
	case nil:
		return "null", false, nil
	case bool:
		return "bool", true, nil
	case string:
		return "string", true, nil
	case json.Number:
		return "number", true, nil
	case []any:
		return "array", true, nil
	case map[string]any:
		return "object", true, nil
	default:
		return "", false, fmt.Errorf("unsupported JSON value")
	}
}

func supportedJSONType(value string) bool {
	switch value {
	case "bool", "string", "number", "array", "object":
		return true
	default:
		return false
	}
}

func compatibleType(structuralType, observedType string) bool {
	return structuralType == observedType
}

func unsafeFieldName(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range []string{"password", "secret", "token", "private_key", "authkey"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func digest(document []byte) string {
	sum := sha256.Sum256(document)
	return hex.EncodeToString(sum[:])
}

func canonicalDocumentDigest(document []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return digest(bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})), nil
}

// ProvisionerReceiptFingerprint derives the runtime identity used by the
// target profile. No caller-supplied fingerprint is accepted in the receipt.
func ProvisionerReceiptFingerprint(receipt ProvisionerTargetReceipt) (string, error) {
	if receipt.FormatVersion != 1 {
		return "", fmt.Errorf("measured target receipt format_version must be 1")
	}
	for name, value := range map[string]string{
		"profile_name": receipt.ProfileName,
		"product":      receipt.Product,
		"version":      receipt.Version,
		"architecture": receipt.Architecture,
	} {
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("measured target receipt %s is required", name)
		}
	}
	for name, value := range map[string]string{
		"image_index_sha256":       receipt.ImageIndexSHA256,
		"image_manifest_sha256":    receipt.ImageManifestSHA256,
		"instance_identity_sha256": receipt.InstanceIdentitySHA256,
	} {
		if !validPrefixedSHA256(value) {
			return "", fmt.Errorf("measured target receipt %s must be sha256:<64 lowercase hex>", name)
		}
	}
	document, err := json.Marshal(receipt)
	if err != nil {
		return "", err
	}
	canonicalDigest, err := canonicalDocumentDigest(document)
	if err != nil {
		return "", err
	}
	return "sha256:" + canonicalDigest, nil
}

// InstanceIdentitySHA256 hashes the exact UTF-8 controller UUID reported by
// the status endpoint. Provisioners use the same function for target receipts.
func InstanceIdentitySHA256(controllerUUID string) (string, error) {
	if controllerUUID == "" {
		return "", fmt.Errorf("controller UUID is required")
	}
	sum := sha256.Sum256([]byte(controllerUUID))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func targetReceiptMatchesProfile(receipt ProvisionerTargetReceipt, target TargetProfile) error {
	if receipt.ProfileName != target.Name || receipt.Product != target.Product || receipt.Version != target.Version ||
		receipt.Architecture != target.Architecture || receipt.ImageIndexSHA256 != target.ImageIndexSHA256 ||
		receipt.ImageManifestSHA256 != target.ImageManifestSHA256 {
		return fmt.Errorf("measured target receipt does not match target profile")
	}
	return nil
}

func validPrefixedSHA256(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && len(decoded) == sha256.Size && strings.ToLower(value) == value
}

func encodeCanonical(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
