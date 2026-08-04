package scout

// TargetProfile identifies one immutable disposable controller target.
type TargetProfile struct {
	Name                  string `json:"name"`
	Product               string `json:"product"`
	Version               string `json:"version"`
	Architecture          string `json:"architecture"`
	ImageIndexSHA256      string `json:"image_index_sha256"`
	ImageManifestSHA256   string `json:"image_manifest_sha256"`
	ControllerFingerprint string `json:"controller_fingerprint"`
}

// Scenario is one declared controller interaction. Scout accepts only
// read-only and disposable workflows.
type Scenario struct {
	ID       string `json:"id"`
	Mode     string `json:"mode"`
	Resource string `json:"resource"`
	Method   string `json:"method"`
	Path     string `json:"path"`
}

// Input contains the immutable structural and observed evidence for one run.
type Input struct {
	Target                TargetProfile
	Scenario              Scenario
	CaptureLockSHA256     string
	Specification         []byte
	ObservedResponse      []byte
	SecretCandidateFields []string
}

// Result contains value-free canonical evidence.
type Result struct {
	Catalog []byte
	Receipt []byte
}

type catalog struct {
	FormatVersion     int                `json:"format_version"`
	CatalogID         string             `json:"catalog_id"`
	Target            TargetProfile      `json:"target"`
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
	Target                     TargetProfile `json:"target"`
	Mode                       string        `json:"mode"`
	OperationDigest            string        `json:"operation_digest"`
	ObservedRecordCount        int           `json:"observed_record_count"`
	RedactedFieldCount         int           `json:"redacted_field_count"`
	CanonicalObservationSHA256 string        `json:"canonical_observation_sha256"`
	Result                     string        `json:"result"`
}
