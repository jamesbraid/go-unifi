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
)

var ignoredControllerFields = map[string]struct{}{
	"_id":            {},
	"site_id":        {},
	"attr_hidden":    {},
	"attr_hidden_id": {},
	"attr_no_delete": {},
	"attr_no_edit":   {},
}

// BuildDNSCatalog joins the generated DNS structure with a declared,
// sanitized observation. Observed values never enter either output.
func BuildDNSCatalog(input Input) (Result, error) {
	if input.Target.Name == "" || input.Target.Product == "" || input.Target.Version == "" ||
		input.Target.Architecture == "" || input.Target.ImageIndexSHA256 == "" ||
		input.Target.ImageManifestSHA256 == "" || input.Target.ControllerFingerprint == "" {
		return Result{}, fmt.Errorf("immutable target digest and identity are required")
	}
	if input.Scenario.Mode != "read_only" && input.Scenario.Mode != "disposable" {
		return Result{}, fmt.Errorf("unsupported scenario mode %q", input.Scenario.Mode)
	}
	if input.Scenario.ID == "" || input.Scenario.Resource != "dns_record" ||
		input.Scenario.Method == "" || input.Scenario.Path == "" {
		return Result{}, fmt.Errorf("incomplete DNS scenario")
	}
	if input.CaptureLockSHA256 == "" {
		return Result{}, fmt.Errorf("capture lock digest is required")
	}

	fields, err := dnsStructuralFields(input.Specification)
	if err != nil {
		return Result{}, err
	}
	records, err := observedObjects(input.ObservedResponse)
	if err != nil {
		return Result{}, err
	}
	secretCandidates := make(map[string]struct{}, len(input.SecretCandidateFields))
	for _, name := range input.SecretCandidateFields {
		secretCandidates[name] = struct{}{}
	}

	structuralNames := make([]string, 0, len(fields))
	structuralByName := make(map[string]structuralRecord, len(fields))
	for name, field := range fields {
		id := catalogFieldID(name)
		_, secretCandidate := secretCandidates[name]
		record := structuralRecord{
			ID:               id,
			Field:            name,
			Type:             field.Type,
			DefinitionSHA256: digest(field.Definition),
			SecretCandidate:  secretCandidate,
		}
		structuralNames = append(structuralNames, name)
		structuralByName[name] = record
	}
	sort.Strings(structuralNames)

	observed := make(map[string]*observedRecord, len(fields))
	for _, name := range structuralNames {
		observed[name] = &observedRecord{
			ID:       catalogFieldID(name),
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
			if nonNull && !compatibleType(field.Type, observedType) {
				conflicts = append(conflicts, conflict{
					Kind:     "type_mismatch",
					Field:    name,
					Expected: field.Type,
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

	operationBytes, err := encodeCanonical(input.Scenario)
	if err != nil {
		return Result{}, fmt.Errorf("encode scenario: %w", err)
	}
	operationDigest := digest(operationBytes)
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
			CaptureLockSHA256:   input.CaptureLockSHA256,
			SpecificationSHA256: digest(input.Specification),
		},
		StructuralRecords: structuralRecords,
		ObservedRecords:   observedRecords,
		Conflicts:         conflicts,
		Coverage:          coverage,
		Admission: admission{
			State:           admissionState,
			OperationDigest: operationDigest,
		},
		Tombstones: []string{},
		Migrations: []catalogMigration{},
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode catalog: %w", err)
	}
	receiptBytes, err := encodeCanonical(scenarioReceipt{
		FormatVersion:              1,
		ScenarioID:                 input.Scenario.ID,
		Target:                     input.Target,
		Mode:                       input.Scenario.Mode,
		OperationDigest:            operationDigest,
		ObservedRecordCount:        len(records),
		RedactedFieldCount:         redactedFields,
		CanonicalObservationSHA256: digest(observationBytes),
		Result:                     admissionState,
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode scenario receipt: %w", err)
	}
	return Result{Catalog: catalogBytes, Receipt: receiptBytes}, nil
}

type structuralField struct {
	Type       string
	Definition []byte
}

func dnsStructuralFields(document []byte) (map[string]structuralField, error) {
	var specification struct {
		Resources []struct {
			Name   string `json:"name"`
			Schema struct {
				Attributes []map[string]json.RawMessage `json:"attributes"`
			} `json:"schema"`
		} `json:"resources"`
	}
	if err := decodeSingleJSON(document, &specification); err != nil {
		return nil, fmt.Errorf("decode specification: %w", err)
	}
	for _, resource := range specification.Resources {
		if resource.Name != "dns_record" {
			continue
		}
		fields := make(map[string]structuralField, len(resource.Schema.Attributes))
		for _, attribute := range resource.Schema.Attributes {
			var name string
			if err := json.Unmarshal(attribute["name"], &name); err != nil || name == "" {
				return nil, fmt.Errorf("DNS attribute has no name")
			}
			for _, attributeType := range []string{"bool", "string", "int64", "float64", "number", "list", "set", "map", "object"} {
				definition, ok := attribute[attributeType]
				if !ok {
					continue
				}
				canonicalDefinition, err := canonicalJSON(definition)
				if err != nil {
					return nil, fmt.Errorf("DNS attribute %q: %w", name, err)
				}
				fields[name] = structuralField{Type: attributeType, Definition: canonicalDefinition}
				break
			}
			if _, ok := fields[name]; !ok {
				return nil, fmt.Errorf("DNS attribute %q has no supported type", name)
			}
		}
		if len(fields) == 0 {
			return nil, fmt.Errorf("DNS resource has no structural fields")
		}
		return fields, nil
	}
	return nil, fmt.Errorf("specification has no dns_record resource")
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

func decodeSingleJSON(document []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func canonicalJSON(document []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
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

func compatibleType(structuralType, observedType string) bool {
	switch structuralType {
	case "bool":
		return observedType == "bool"
	case "string":
		return observedType == "string"
	case "int64", "float64", "number":
		return observedType == "number"
	case "list", "set":
		return observedType == "array"
	case "map", "object":
		return observedType == "object"
	default:
		return false
	}
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

func catalogFieldID(field string) string {
	return "unifi.network.dns_record.field." + field
}

func digest(document []byte) string {
	sum := sha256.Sum256(document)
	return hex.EncodeToString(sum[:])
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
