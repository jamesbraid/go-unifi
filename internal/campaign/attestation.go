// Package campaign builds candidate-only compatibility attestations from
// canonical scout evidence.
package campaign

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"
)

// Input binds one candidate scout run to its baseline and immutable builder.
type Input struct {
	CampaignID                     string
	ProfileClass                   string
	BuilderImageDigest             string
	BaselineCatalog                []byte
	CandidateCatalog               []byte
	ScenarioReceipt                []byte
	Elapsed                        time.Duration
	HumanDecisions                 []string
	ManualGeneratedFileEdits       bool
	AutomaticallyReconfirmedClaims []string
}

type targetProfile struct {
	Name                  string `json:"name"`
	Product               string `json:"product"`
	Version               string `json:"version"`
	Architecture          string `json:"architecture"`
	ImageIndexSHA256      string `json:"image_index_sha256"`
	ImageManifestSHA256   string `json:"image_manifest_sha256"`
	ControllerFingerprint string `json:"controller_fingerprint"`
}

type catalogDocument struct {
	FormatVersion     int                `json:"format_version"`
	CatalogID         string             `json:"catalog_id"`
	Target            targetProfile      `json:"target"`
	Sources           catalogSources     `json:"sources"`
	StructuralRecords []structuralRecord `json:"structural_records"`
	ObservedRecords   []observedRecord   `json:"observed_records"`
	Conflicts         []conflict         `json:"conflicts"`
	Coverage          []coverageRecord   `json:"coverage"`
	Admission         admission          `json:"admission"`
	Tombstones        []string           `json:"tombstones"`
	Migrations        []catalogMigration `json:"migrations"`
}

type catalogSources struct {
	CaptureLockSHA256   string `json:"capture_lock_sha256"`
	SpecificationSHA256 string `json:"specification_sha256"`
}

type structuralRecord struct {
	ID               string `json:"id"`
	Field            string `json:"field"`
	Type             string `json:"type"`
	DefinitionSHA256 string `json:"definition_sha256"`
	SecretCandidate  bool   `json:"secret_candidate"`
}

type observedRecord struct {
	ID           string `json:"id"`
	Field        string `json:"field"`
	JSONType     string `json:"json_type"`
	PresentCount int    `json:"present_count"`
	NonNullCount int    `json:"non_null_count"`
}

type conflict struct {
	Kind     string `json:"kind"`
	Field    string `json:"field"`
	Expected string `json:"expected,omitempty"`
	Observed string `json:"observed,omitempty"`
}

type coverageRecord struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type admission struct {
	State           string `json:"state"`
	OperationDigest string `json:"operation_digest"`
}

type catalogMigration struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
	Reason string `json:"reason"`
}

type scenarioReceipt struct {
	FormatVersion              int           `json:"format_version"`
	ScenarioID                 string        `json:"scenario_id"`
	Target                     targetProfile `json:"target"`
	Mode                       string        `json:"mode"`
	OperationDigest            string        `json:"operation_digest"`
	ObservedRecordCount        int           `json:"observed_record_count"`
	RedactedFieldCount         int           `json:"redacted_field_count"`
	CanonicalObservationSHA256 string        `json:"canonical_observation_sha256"`
	Result                     string        `json:"result"`
}

type attestation struct {
	FormatVersion                  int               `json:"format_version"`
	CampaignID                     string            `json:"campaign_id"`
	ProfileClass                   string            `json:"profile_class"`
	Classification                 string            `json:"classification"`
	Promotion                      string            `json:"promotion"`
	CandidateOnly                  bool              `json:"candidate_only"`
	Target                         targetProfile     `json:"target"`
	BuilderImageDigest             string            `json:"builder_image_digest"`
	Inputs                         attestationInputs `json:"inputs"`
	StructuralDiff                 []string          `json:"structural_diff"`
	ObservationDiff                []string          `json:"observation_diff"`
	Conflicts                      []conflict        `json:"conflicts"`
	ClaimCoverage                  []coverageRecord  `json:"claim_coverage"`
	AffectedAdmittedOperations     []string          `json:"affected_admitted_operations"`
	ProviderContractImpact         string            `json:"provider_contract_impact"`
	ElapsedMilliseconds            int64             `json:"elapsed_milliseconds"`
	HumanDecisions                 []string          `json:"human_decisions"`
	ManualGeneratedFileEdits       bool              `json:"manual_generated_file_edits"`
	AutomaticallyReconfirmedClaims []string          `json:"automatically_reconfirmed_claims"`
}

type attestationInputs struct {
	BaselineCatalogSHA256  string `json:"baseline_catalog_sha256"`
	CandidateCatalogSHA256 string `json:"candidate_catalog_sha256"`
	ScenarioReceiptSHA256  string `json:"scenario_receipt_sha256"`
	CaptureLockSHA256      string `json:"capture_lock_sha256"`
}

// BuildAttestation compares a candidate catalog with an admitted baseline.
// Its output always requires review and cannot promote provider behavior.
func BuildAttestation(input Input) ([]byte, error) {
	if strings.TrimSpace(input.CampaignID) == "" {
		return nil, fmt.Errorf("campaign ID is required")
	}
	if !validProfileClass(input.ProfileClass) {
		return nil, fmt.Errorf("unsupported profile class %q", input.ProfileClass)
	}
	if !pinnedImageDigest(input.BuilderImageDigest) {
		return nil, fmt.Errorf("builder image digest must end in @sha256:<64 lowercase hex>")
	}
	if input.Elapsed < 0 {
		return nil, fmt.Errorf("elapsed duration cannot be negative")
	}

	var baseline, candidate catalogDocument
	baselineCanonical, err := decodeCanonical(input.BaselineCatalog, &baseline)
	if err != nil {
		return nil, fmt.Errorf("baseline catalog: %w", err)
	}
	candidateCanonical, err := decodeCanonical(input.CandidateCatalog, &candidate)
	if err != nil {
		return nil, fmt.Errorf("candidate catalog: %w", err)
	}
	var receipt scenarioReceipt
	receiptCanonical, err := decodeCanonical(input.ScenarioReceipt, &receipt)
	if err != nil {
		return nil, fmt.Errorf("scenario receipt: %w", err)
	}
	if err := validateCatalog(baseline); err != nil {
		return nil, fmt.Errorf("baseline catalog: %w", err)
	}
	if err := validateCatalog(candidate); err != nil {
		return nil, fmt.Errorf("candidate catalog: %w", err)
	}
	if receipt.FormatVersion != 1 || receipt.ScenarioID == "" || receipt.Mode == "" {
		return nil, fmt.Errorf("scenario receipt identity is incomplete")
	}
	if !reflect.DeepEqual(receipt.Target, candidate.Target) {
		return nil, fmt.Errorf("scenario receipt target does not match candidate catalog")
	}
	if receipt.OperationDigest != candidate.Admission.OperationDigest {
		return nil, fmt.Errorf("scenario receipt operation does not match candidate catalog")
	}
	if receipt.Result != candidate.Admission.State {
		return nil, fmt.Errorf("scenario receipt result does not match candidate admission")
	}

	structuralDiff := recordDiff(baseline.StructuralRecords, candidate.StructuralRecords, func(record structuralRecord) string { return record.ID })
	observationDiff := recordDiff(baseline.ObservedRecords, candidate.ObservedRecords, func(record observedRecord) string { return record.ID })
	classification := "equivalent"
	providerImpact := "none"
	if len(structuralDiff) > 0 || len(observationDiff) > 0 {
		classification = "candidate_change"
		providerImpact = "review_required"
	}
	if candidate.Admission.State == "blocked" || len(candidate.Conflicts) > 0 {
		classification = "blocked"
		providerImpact = "review_required"
	}
	affectedOperations := []string{}
	if classification != "equivalent" {
		affectedOperations = append(affectedOperations, candidate.Admission.OperationDigest)
	}

	coverage := append([]coverageRecord(nil), candidate.Coverage...)
	sort.Slice(coverage, func(i, j int) bool { return coverage[i].ID < coverage[j].ID })
	decisions := sortedUnique(input.HumanDecisions)
	reconfirmed := sortedUnique(input.AutomaticallyReconfirmedClaims)
	for _, claim := range reconfirmed {
		if !coveredClaim(coverage, claim) {
			return nil, fmt.Errorf("automatically reconfirmed claim %q lacks observed coverage", claim)
		}
	}
	conflicts := make([]conflict, len(candidate.Conflicts))
	copy(conflicts, candidate.Conflicts)
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Field == conflicts[j].Field {
			return conflicts[i].Kind < conflicts[j].Kind
		}
		return conflicts[i].Field < conflicts[j].Field
	})

	return encodeCanonical(attestation{
		FormatVersion:      1,
		CampaignID:         input.CampaignID,
		ProfileClass:       input.ProfileClass,
		Classification:     classification,
		Promotion:          "review_required",
		CandidateOnly:      true,
		Target:             candidate.Target,
		BuilderImageDigest: input.BuilderImageDigest,
		Inputs: attestationInputs{
			BaselineCatalogSHA256:  digest(baselineCanonical),
			CandidateCatalogSHA256: digest(candidateCanonical),
			ScenarioReceiptSHA256:  digest(receiptCanonical),
			CaptureLockSHA256:      candidate.Sources.CaptureLockSHA256,
		},
		StructuralDiff:                 structuralDiff,
		ObservationDiff:                observationDiff,
		Conflicts:                      conflicts,
		ClaimCoverage:                  coverage,
		AffectedAdmittedOperations:     affectedOperations,
		ProviderContractImpact:         providerImpact,
		ElapsedMilliseconds:            input.Elapsed.Milliseconds(),
		HumanDecisions:                 decisions,
		ManualGeneratedFileEdits:       input.ManualGeneratedFileEdits,
		AutomaticallyReconfirmedClaims: reconfirmed,
	})
}

func validateCatalog(document catalogDocument) error {
	if document.FormatVersion != 1 || document.CatalogID == "" || document.Target.Name == "" ||
		document.Target.ImageIndexSHA256 == "" || document.Target.ImageManifestSHA256 == "" ||
		document.Sources.CaptureLockSHA256 == "" || document.Sources.SpecificationSHA256 == "" ||
		document.Admission.OperationDigest == "" {
		return fmt.Errorf("catalog identity and immutable inputs are required")
	}
	if document.Admission.State != "candidate" && document.Admission.State != "blocked" {
		return fmt.Errorf("unsupported admission state %q", document.Admission.State)
	}
	return nil
}

func validProfileClass(value string) bool {
	switch value {
	case "fresh_seeded", "persisted_single_hop", "long_lived_multi_hop":
		return true
	default:
		return false
	}
}

func pinnedImageDigest(value string) bool {
	_, digestValue, ok := strings.Cut(value, "@sha256:")
	if !ok || len(digestValue) != sha256.Size*2 || strings.ToLower(digestValue) != digestValue {
		return false
	}
	_, err := hex.DecodeString(digestValue)
	return err == nil
}

func coveredClaim(coverage []coverageRecord, claim string) bool {
	for _, item := range coverage {
		if item.ID == claim && item.State == "observed" {
			return true
		}
	}
	return false
}

func recordDiff[T any](baseline, candidate []T, id func(T) string) []string {
	baselineRecords := make(map[string][]byte, len(baseline))
	candidateRecords := make(map[string][]byte, len(candidate))
	for _, record := range baseline {
		baselineRecords[id(record)], _ = json.Marshal(record)
	}
	for _, record := range candidate {
		candidateRecords[id(record)], _ = json.Marshal(record)
	}
	changed := make(map[string]struct{})
	for recordID, baselineRecord := range baselineRecords {
		if candidateRecord, ok := candidateRecords[recordID]; !ok || !bytes.Equal(baselineRecord, candidateRecord) {
			changed[recordID] = struct{}{}
		}
	}
	for recordID, candidateRecord := range candidateRecords {
		if baselineRecord, ok := baselineRecords[recordID]; !ok || !bytes.Equal(baselineRecord, candidateRecord) {
			changed[recordID] = struct{}{}
		}
	}
	result := make([]string, 0, len(changed))
	for recordID := range changed {
		result = append(result, recordID)
	}
	sort.Strings(result)
	return result
}

func sortedUnique(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func decodeCanonical(document []byte, target any) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing JSON value")
		}
		return nil, err
	}
	return json.Marshal(target)
}

func encodeCanonical(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func digest(document []byte) string {
	sum := sha256.Sum256(document)
	return hex.EncodeToString(sum[:])
}
