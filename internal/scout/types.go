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

// ProvisionerTargetReceipt is the independently measured runtime identity for
// one disposable controller. InstanceIdentitySHA256 uses
// InstanceIdentitySHA256() over the exact controller UUID bytes.
type ProvisionerTargetReceipt struct {
	FormatVersion          int    `json:"format_version"`
	ProfileName            string `json:"profile_name"`
	Product                string `json:"product"`
	Version                string `json:"version"`
	Architecture           string `json:"architecture"`
	ImageIndexSHA256       string `json:"image_index_sha256"`
	ImageManifestSHA256    string `json:"image_manifest_sha256"`
	InstanceIdentitySHA256 string `json:"instance_identity_sha256"`
}

// Scenario is one declared controller interaction. Scout accepts only the
// read-only DNS list workflow.
type Scenario struct {
	ID       string `json:"id"`
	Mode     string `json:"mode"`
	Resource string `json:"resource"`
	Method   string `json:"method"`
	Path     string `json:"path"`
}

// LockedSources carries the digests read from the validated capture lock.
type LockedSources struct {
	CaptureLockSHA256          string
	ControllerNetworkVersion   string
	ExtractionRulesSHA256      string
	StructuralSHA256           string
	SensitivitySHA256          string
	StructuralProjectionSHA256 string
	SemanticPredecessorSHA256  string
}

// Input contains the immutable structural and raw observed evidence for one
// DNS run. ExecutionMode is either fixture or live; only a live run may carry
// a provisioner receipt, controller version, and hashed controller identity
// learned by the API client.
type Input struct {
	Target                         TargetProfile
	TargetReceipt                  *ProvisionerTargetReceipt
	ControllerVersion              string
	ObservedInstanceIdentitySHA256 string
	Scenario                       Scenario
	ExecutionMode                  string
	LockedSources                  LockedSources
	StructuralProjection           []byte
	SemanticPredecessor            []byte
	SemanticIDs                    []byte
	ObservedResponse               []byte
}

// Result contains value-free canonical evidence.
type Result struct {
	Catalog []byte
	Receipt []byte
}

type structuralProjection struct {
	FormatVersion int                         `json:"format_version"`
	Resource      string                      `json:"resource"`
	Source        structuralSource            `json:"source"`
	Fields        []structuralProjectionField `json:"fields"`
}

type structuralSource struct {
	StructuralSHA256  string `json:"structural_sha256"`
	SensitivitySHA256 string `json:"sensitivity_sha256"`
}

type structuralProjectionField struct {
	WireName        string `json:"wire_name"`
	JSONType        string `json:"json_type"`
	SecretCandidate bool   `json:"secret_candidate"`
}

type semanticRegistry struct {
	FormatVersion         int                 `json:"format_version"`
	Resource              string              `json:"resource"`
	ExtractionRulesSHA256 string              `json:"extraction_rules_sha256"`
	Fields                []semanticField     `json:"fields"`
	Tombstones            []semanticTombstone `json:"tombstones"`
	Migrations            []catalogMigration  `json:"migrations"`
}

type semanticPredecessorRegistry struct {
	FormatVersion int             `json:"format_version"`
	Resource      string          `json:"resource"`
	Fields        []semanticField `json:"fields"`
}

type semanticField struct {
	WireName string `json:"wire_name"`
	ID       string `json:"id"`
}

type semanticTombstone struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
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
	CaptureLockSHA256          string `json:"capture_lock_sha256"`
	StructuralProjectionSHA256 string `json:"structural_projection_sha256"`
	SemanticPredecessorSHA256  string `json:"semantic_predecessor_sha256"`
	SemanticIDsSHA256          string `json:"semantic_ids_sha256"`
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
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	Reason   string `json:"reason"`
	Reviewed bool   `json:"reviewed"`
}

type requestShape struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Query  string `json:"query"`
	Body   string `json:"body"`
}

type scenarioReceipt struct {
	FormatVersion                  int          `json:"format_version"`
	ScenarioID                     string       `json:"scenario_id"`
	ScenarioPath                   string       `json:"scenario_path"`
	ScenarioMode                   string       `json:"scenario_mode"`
	RequestShape                   requestShape `json:"request_shape"`
	ExecutionMode                  string       `json:"execution_mode"`
	MeasuredTargetFingerprint      string       `json:"measured_target_fingerprint,omitempty"`
	ControllerVersion              string       `json:"controller_version,omitempty"`
	ObservedInstanceIdentitySHA256 string       `json:"observed_instance_identity_sha256,omitempty"`
	OperationDigest                string       `json:"operation_digest"`
	ResponseSHA256                 string       `json:"response_sha256"`
	Normalization                  string       `json:"normalization"`
	Redaction                      string       `json:"redaction"`
	Cleanup                        string       `json:"cleanup"`
	ObservedRecordCount            int          `json:"observed_record_count"`
	RedactedFieldCount             int          `json:"redacted_field_count"`
	CanonicalObservationSHA256     string       `json:"canonical_observation_sha256"`
	Verdict                        string       `json:"verdict"`
}
