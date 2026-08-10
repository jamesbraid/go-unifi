package scout

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const (
	testExtractionSHA  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testStructuralSHA  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testSensitivitySHA = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	testDNSDocument    = "DnsRecord.json"
	testDNSDocumentSHA = "1111111111111111111111111111111111111111111111111111111111111111"
	testOtherDocument  = "FirewallPolicy.json"
)

func TestBuildDNSCatalogIsCanonicalAndValueFree(t *testing.T) {
	first := testInput(t, `[
  {"_id":"record-a","enabled":true,"key":"alpha.example.invalid","port":53,"priority":10,"record_type":"A","ttl":300,"value":"192.0.2.10","weight":1},
  {"_id":"record-b","enabled":false,"key":"beta.example.invalid","record_type":"TXT","ttl":60,"value":"private fixture text"}
]`)
	second := testInput(t, `[
  {"_id":"different-b","enabled":true,"key":"changed.example.invalid","record_type":"TXT","ttl":120,"value":"different text"},
  {"_id":"different-a","enabled":false,"key":"other.example.invalid","port":443,"priority":20,"record_type":"AAAA","ttl":600,"value":"2001:db8::10","weight":2}
]`)

	firstResult, err := BuildDNSCatalog(first)
	if err != nil {
		t.Fatal(err)
	}
	secondResult, err := BuildDNSCatalog(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstResult.Catalog, secondResult.Catalog) {
		t.Fatalf("canonical catalogs differ:\nfirst: %s\nsecond: %s", firstResult.Catalog, secondResult.Catalog)
	}
	for _, secret := range []string{"record-a", "alpha.example.invalid", "192.0.2.10", "private fixture text"} {
		if bytes.Contains(firstResult.Catalog, []byte(secret)) || bytes.Contains(firstResult.Receipt, []byte(secret)) {
			t.Fatalf("output retained observed value %q", secret)
		}
	}

	var catalog struct {
		StructuralRecords []struct {
			ID string `json:"id"`
		} `json:"structural_records"`
		ObservedRecords []struct {
			ID string `json:"id"`
		} `json:"observed_records"`
		Admission struct {
			State string `json:"state"`
		} `json:"admission"`
	}
	if err := json.Unmarshal(firstResult.Catalog, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.StructuralRecords) != 8 || len(catalog.ObservedRecords) != 8 {
		t.Fatalf("record counts = %d structural, %d observed; want 8 and 8", len(catalog.StructuralRecords), len(catalog.ObservedRecords))
	}
	if catalog.StructuralRecords[0].ID != "unifi.network.dns_record.field.enabled" {
		t.Fatalf("first catalog ID = %q", catalog.StructuralRecords[0].ID)
	}
	if catalog.Admission.State != "candidate" {
		t.Fatalf("admission state = %q, want candidate", catalog.Admission.State)
	}
}

func TestBuildDNSCatalogKeepsLiveUnknownFieldConflictSeparate(t *testing.T) {
	input := testInput(t, `[{"enabled":true,"key":"fixture.example.invalid","port":53,"priority":10,"record_type":"A","ttl":300,"value":"192.0.2.20","new_field":true,"weight":1}]`)
	makeLiveInput(t, &input)
	result, err := BuildDNSCatalog(input)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Conflicts []struct {
			Kind  string `json:"kind"`
			Field string `json:"field"`
		} `json:"conflicts"`
		Admission struct {
			State string `json:"state"`
		} `json:"admission"`
	}
	if err := json.Unmarshal(result.Catalog, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Conflicts) != 1 || catalog.Conflicts[0].Kind != "unknown_observed_field" || catalog.Conflicts[0].Field != "new_field" {
		t.Fatalf("conflicts = %#v", catalog.Conflicts)
	}
	if catalog.Admission.State != "blocked" {
		t.Fatalf("admission state = %q, want blocked", catalog.Admission.State)
	}
}

func TestBuildDNSCatalogUsesExplicitSemanticIDAndSensitivityHint(t *testing.T) {
	input := testInput(t, `[{"enabled":true,"key":"fixture.example.invalid","port":53,"priority":10,"record_type":"A","ttl":300,"answer":"192.0.2.20","weight":1}]`)
	input.StructuralProjection = []byte(strings.ReplaceAll(string(input.StructuralProjection),
		`{"wire_name":"value","json_type":"string","secret_candidate":false}`,
		`{"wire_name":"answer","json_type":"string","secret_candidate":true}`))
	input.SemanticIDs = []byte(strings.ReplaceAll(string(input.SemanticIDs), `"wire_name":"value"`, `"wire_name":"answer"`))
	input.LockedSources.StructuralProjectionSHA256 = mustCanonicalDigest(t, input.StructuralProjection)

	result, err := BuildDNSCatalog(input)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		StructuralRecords []struct {
			ID              string `json:"id"`
			Field           string `json:"field"`
			SecretCandidate bool   `json:"secret_candidate"`
		} `json:"structural_records"`
	}
	if err := json.Unmarshal(result.Catalog, &catalog); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, record := range catalog.StructuralRecords {
		if record.Field != "answer" {
			continue
		}
		found = true
		if record.ID != "unifi.network.dns_record.field.value" {
			t.Fatalf("answer ID = %q; semantic ID was derived from mutable wire name", record.ID)
		}
		if !record.SecretCandidate {
			t.Fatal("locked sensitivity hint was not retained as secret_candidate")
		}
	}
	if !found {
		t.Fatal("answer structural record not found")
	}
	for _, policy := range []string{"required", "optional", "computed", "validator", "sensitive"} {
		if bytes.Contains(result.Catalog, []byte(policy)) {
			t.Fatalf("catalog retained Terraform policy marker %q", policy)
		}
	}
}

func TestBuildDNSCatalogRejectsProjectionSensitivityChangeUnderSameLock(t *testing.T) {
	input := testInput(t, `[]`)
	input.StructuralProjection = []byte(strings.Replace(
		string(input.StructuralProjection),
		`"wire_name":"value","json_type":"string","secret_candidate":false`,
		`"wire_name":"value","json_type":"string","secret_candidate":true`,
		1,
	))

	_, err := BuildDNSCatalog(input)
	if err == nil || !strings.Contains(err.Error(), "structural projection digest") {
		t.Fatalf("BuildDNSCatalog() error = %v, want locked projection digest rejection", err)
	}
}

func TestBuildDNSCatalogRejectsCandidateRewrittenSemanticHistory(t *testing.T) {
	input := testInput(t, `[]`)
	input.SemanticIDs = []byte(strings.ReplaceAll(
		string(input.SemanticIDs),
		"unifi.network.dns_record.field.enabled",
		"unifi.network.dns_record.field.enabled_rewritten",
	))

	_, err := BuildDNSCatalog(input)
	if err == nil || !strings.Contains(err.Error(), "predecessor semantic ID") {
		t.Fatalf("BuildDNSCatalog() error = %v, want immutable predecessor rejection", err)
	}
}

func TestBuildDNSCatalogRejectsStrandedPriorSemanticID(t *testing.T) {
	input := testInput(t, `[]`)
	input.SemanticIDs = []byte(strings.Replace(string(input.SemanticIDs),
		`"id":"unifi.network.dns_record.field.value"`,
		`"id":"unifi.network.dns_record.field.answer"`, 1))
	_, err := BuildDNSCatalog(input)
	if err == nil || !strings.Contains(err.Error(), "strands predecessor semantic ID") {
		t.Fatalf("BuildDNSCatalog() error = %v, want stranded semantic ID rejection", err)
	}
}

func TestBuildDNSCatalogRequiresReviewedSemanticMigration(t *testing.T) {
	input := testInput(t, `[]`)
	input.SemanticIDs = []byte(strings.Replace(string(input.SemanticIDs),
		`"id":"unifi.network.dns_record.field.value"`,
		`"id":"unifi.network.dns_record.field.answer"`, 1))
	input.SemanticIDs = []byte(strings.Replace(string(input.SemanticIDs),
		`"migrations":[]`,
		`"migrations":[{"from_id":"unifi.network.dns_record.field.value","to_id":"unifi.network.dns_record.field.answer","reason":"semantic meaning changed","reviewed":false}]`, 1))
	_, err := BuildDNSCatalog(input)
	if err == nil || !strings.Contains(err.Error(), "must be reviewed") {
		t.Fatalf("BuildDNSCatalog() error = %v, want unreviewed migration rejection", err)
	}

	input.SemanticIDs = []byte(strings.Replace(string(input.SemanticIDs), `"reviewed":false`, `"reviewed":true`, 1))
	result, err := BuildDNSCatalog(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(result.Catalog, []byte(`"from_id": "unifi.network.dns_record.field.value"`)) {
		t.Fatalf("reviewed migration missing from catalog: %s", result.Catalog)
	}
}

func TestBuildDNSCatalogBindsFixtureReceiptWithoutLiveTargetClaim(t *testing.T) {
	input := testInput(t, `[{"enabled":true,"key":"fixture.example.invalid","port":53,"priority":10,"record_type":"A","ttl":300,"value":"192.0.2.20","weight":1}]`)
	result, err := BuildDNSCatalog(input)
	if err != nil {
		t.Fatal(err)
	}
	var receipt map[string]json.RawMessage
	if err := json.Unmarshal(result.Receipt, &receipt); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"target", "measured_target_fingerprint"} {
		if _, exists := receipt[forbidden]; exists {
			t.Fatalf("fixture receipt claims %s: %s", forbidden, result.Receipt)
		}
	}
	assertReceiptString(t, receipt, "scenario_path", input.Scenario.Path)
	assertReceiptString(t, receipt, "execution_mode", "fixture")
	assertReceiptString(t, receipt, "operation_digest", "199c002d9a1229aa7e43a94b12da6f33c7ce612ecb471eebcd6896d985b161f8")
	assertReceiptString(t, receipt, "response_sha256", digest(input.ObservedResponse))
	assertReceiptString(t, receipt, "normalization", "field_presence_and_json_type_v1")
	assertReceiptString(t, receipt, "redaction", "drop_all_observed_values_v1")
	assertReceiptString(t, receipt, "cleanup", "not_required_read_only")
	assertReceiptString(t, receipt, "verdict", "candidate")
	var requestShape struct {
		Method string `json:"method"`
		Path   string `json:"path"`
		Query  string `json:"query"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal(receipt["request_shape"], &requestShape); err != nil {
		t.Fatal(err)
	}
	if requestShape.Method != "GET" || requestShape.Path != input.Scenario.Path || requestShape.Query != "none" || requestShape.Body != "none" {
		t.Fatalf("request shape = %#v", requestShape)
	}
}

func TestBuildDNSCatalogRequiresMeasuredFingerprintOnlyForLiveExecution(t *testing.T) {
	input := testInput(t, `[]`)
	receipt := testTargetReceipt(input.Target)
	input.TargetReceipt = &receipt
	_, err := BuildDNSCatalog(input)
	if err == nil || !strings.Contains(err.Error(), "fixture execution cannot claim a measured target receipt") {
		t.Fatalf("BuildDNSCatalog() error = %v, want fixture target claim rejection", err)
	}

	input.ExecutionMode = "live"
	input.TargetReceipt = nil
	_, err = BuildDNSCatalog(input)
	if err == nil || !strings.Contains(err.Error(), "live execution requires a measured target receipt") {
		t.Fatalf("BuildDNSCatalog() error = %v, want missing live fingerprint rejection", err)
	}

	makeLiveInput(t, &input)
	result, err := BuildDNSCatalog(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(result.Receipt, []byte(`"measured_target_fingerprint": "`+input.Target.ControllerFingerprint+`"`)) {
		t.Fatalf("live receipt did not bind measured fingerprint: %s", result.Receipt)
	}
}

func TestBuildDNSCatalogRejectsCopiedLiveFingerprint(t *testing.T) {
	input := testInput(t, `[]`)
	input.ExecutionMode = "live"

	_, err := BuildDNSCatalog(input)
	if err == nil || !strings.Contains(err.Error(), "measured target receipt") {
		t.Fatalf("BuildDNSCatalog() error = %v, want independently measured target rejection", err)
	}
}

func TestBuildDNSCatalogRejectsObservedControllerVersionMismatch(t *testing.T) {
	input := testInput(t, `[]`)
	makeLiveInput(t, &input)
	input.ControllerVersion = "10.4.58"

	_, err := BuildDNSCatalog(input)
	if err == nil || !strings.Contains(err.Error(), "does not match target profile and capture lock") {
		t.Fatalf("BuildDNSCatalog() error = %v, want observed version mismatch", err)
	}
}

func TestBuildDNSCatalogFailsClosed(t *testing.T) {
	tests := map[string]struct {
		mutate func(*Input)
		want   string
	}{
		"malformed observation": {
			mutate: func(input *Input) { input.ObservedResponse = []byte(`{"not":"an array"}`) },
			want:   "observation must be an array",
		},
		"unsafe field": {
			mutate: func(input *Input) {
				input.ObservedResponse = []byte(`[{"key":"fixture","x_password":"do-not-retain"}]`)
			},
			want: "unsafe observed field",
		},
		"mutable target": {
			mutate: func(input *Input) { input.Target.ImageManifestSHA256 = "" },
			want:   "immutable target digest",
		},
		"undeclared workflow": {
			mutate: func(input *Input) { input.Scenario.Mode = "production" },
			want:   "unsupported scenario mode",
		},
		"source document moved": {
			mutate: func(input *Input) {
				input.FieldDocumentDigests[testDNSDocument] = strings.Repeat("d", 64)
			},
			want: "field document " + testDNSDocument + " digest is",
		},
		"source document absent from the lock": {
			mutate: func(input *Input) {
				delete(input.FieldDocumentDigests, testDNSDocument)
			},
			want: "is absent from the locked structural snapshot",
		},
		"lock records no per-document digests": {
			mutate: func(input *Input) { input.FieldDocumentDigests = nil },
			want:   "capture lock does not record per-document field digests",
		},
		"sensitivity lock mismatch": {
			mutate: func(input *Input) { input.LockedSources.SensitivitySHA256 = strings.Repeat("d", 64) },
			want:   "sensitivity snapshot digest",
		},
		"extraction rules lock mismatch": {
			mutate: func(input *Input) { input.LockedSources.ExtractionRulesSHA256 = strings.Repeat("d", 64) },
			want:   "extraction rules digest",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			input := testInput(t, `[]`)
			test.mutate(&input)
			_, err := BuildDNSCatalog(input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("BuildDNSCatalog() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

// TestBuildDNSCatalogSurvivesAnUnrelatedSurfaceMoving is the regression this
// per-document pin exists for.
//
// The projection used to pin snapshots.structural_sha256, which digests the
// whole schemas/fields tree after the overrides are written. Commit 7d15987
// added two fields to overrides/resources/FirewallPolicy.json, that digest
// moved, and dns_record's projection went stale over a change to a surface it
// does not describe -- with no way to re-pin it that a reviewer could tell
// apart from a real dns_record change.
func TestBuildDNSCatalogSurvivesAnUnrelatedSurfaceMoving(t *testing.T) {
	input := testInput(t, `[]`)
	// Exactly what an override to another resource does: the tree digest
	// moves, that resource's own entry moves, dns_record's does not.
	input.LockedSources.StructuralSHA256 = strings.Repeat("d", 64)
	input.FieldDocumentDigests[testOtherDocument] = strings.Repeat("e", 64)

	if _, err := BuildDNSCatalog(input); err != nil {
		t.Fatalf("BuildDNSCatalog() rejected a projection whose own source document did not move: %v", err)
	}
}

func assertReceiptString(t *testing.T, receipt map[string]json.RawMessage, field, want string) {
	t.Helper()
	var got string
	if err := json.Unmarshal(receipt[field], &got); err != nil {
		t.Fatalf("decode receipt %s: %v", field, err)
	}
	if got != want {
		t.Fatalf("receipt %s = %q, want %q", field, got, want)
	}
}

func testInput(t *testing.T, observation string) Input {
	t.Helper()
	structural := []byte(`{
  "format_version":1,
  "resource":"dns_record",
  "source":{"field_document":"` + testDNSDocument + `","field_document_sha256":"` + testDNSDocumentSHA + `","sensitivity_sha256":"` + testSensitivitySHA + `"},
  "fields":[
    {"wire_name":"enabled","json_type":"bool","secret_candidate":false},
    {"wire_name":"key","json_type":"string","secret_candidate":false},
    {"wire_name":"port","json_type":"number","secret_candidate":false},
    {"wire_name":"priority","json_type":"number","secret_candidate":false},
    {"wire_name":"record_type","json_type":"string","secret_candidate":false},
    {"wire_name":"ttl","json_type":"number","secret_candidate":false},
    {"wire_name":"value","json_type":"string","secret_candidate":false},
    {"wire_name":"weight","json_type":"number","secret_candidate":false}
  ]
}`)
	semanticIDs := []byte(`{
  "format_version":1,
  "resource":"dns_record",
  "extraction_rules_sha256":"` + testExtractionSHA + `",
  "fields":[
    {"wire_name":"enabled","id":"unifi.network.dns_record.field.enabled"},
    {"wire_name":"key","id":"unifi.network.dns_record.field.key"},
    {"wire_name":"port","id":"unifi.network.dns_record.field.port"},
    {"wire_name":"priority","id":"unifi.network.dns_record.field.priority"},
    {"wire_name":"record_type","id":"unifi.network.dns_record.field.record_type"},
    {"wire_name":"ttl","id":"unifi.network.dns_record.field.ttl"},
    {"wire_name":"value","id":"unifi.network.dns_record.field.value"},
    {"wire_name":"weight","id":"unifi.network.dns_record.field.weight"}
  ],
  "tombstones":[],
  "migrations":[]
}`)
	semanticPredecessor := []byte(`{
  "format_version":1,
  "resource":"dns_record",
  "fields":[
    {"wire_name":"enabled","id":"unifi.network.dns_record.field.enabled"},
    {"wire_name":"key","id":"unifi.network.dns_record.field.key"},
    {"wire_name":"port","id":"unifi.network.dns_record.field.port"},
    {"wire_name":"priority","id":"unifi.network.dns_record.field.priority"},
    {"wire_name":"record_type","id":"unifi.network.dns_record.field.record_type"},
    {"wire_name":"ttl","id":"unifi.network.dns_record.field.ttl"},
    {"wire_name":"value","id":"unifi.network.dns_record.field.value"},
    {"wire_name":"weight","id":"unifi.network.dns_record.field.weight"}
  ]
}`)
	return Input{
		Target: TargetProfile{
			Name:                  "network-10.4.57-seeded",
			Product:               "unifi-network",
			Version:               "10.4.57",
			Architecture:          "amd64",
			ImageIndexSHA256:      "sha256:" + strings.Repeat("1", 64),
			ImageManifestSHA256:   "sha256:" + strings.Repeat("2", 64),
			ControllerFingerprint: "sha256:" + strings.Repeat("3", 64),
		},
		Scenario: Scenario{
			ID:       "dns-record-list-v1",
			Mode:     "read_only",
			Resource: "dns_record",
			Method:   "GET",
			Path:     "/v2/api/site/{site}/static-dns",
		},
		ExecutionMode: "fixture",
		LockedSources: LockedSources{
			CaptureLockSHA256:          strings.Repeat("d", 64),
			ControllerNetworkVersion:   "10.4.57",
			ExtractionRulesSHA256:      testExtractionSHA,
			StructuralSHA256:           testStructuralSHA,
			SensitivitySHA256:          testSensitivitySHA,
			StructuralProjectionSHA256: mustCanonicalDigest(t, structural),
			SemanticPredecessorSHA256:  mustCanonicalDigest(t, semanticPredecessor),
		},
		FieldDocumentDigests: FieldDocumentDigests{
			testDNSDocument:   testDNSDocumentSHA,
			testOtherDocument: strings.Repeat("9", 64),
		},
		StructuralProjection: structural,
		SemanticPredecessor:  semanticPredecessor,
		SemanticIDs:          semanticIDs,
		ObservedResponse:     []byte(observation),
	}
}

func makeLiveInput(t *testing.T, input *Input) {
	t.Helper()
	receipt := testTargetReceipt(input.Target)
	fingerprint, err := ProvisionerReceiptFingerprint(receipt)
	if err != nil {
		t.Fatal(err)
	}
	input.Target.ControllerFingerprint = fingerprint
	input.TargetReceipt = &receipt
	input.ControllerVersion = input.Target.Version
	input.ObservedInstanceIdentitySHA256 = receipt.InstanceIdentitySHA256
	input.ExecutionMode = "live"
}

func testTargetReceipt(target TargetProfile) ProvisionerTargetReceipt {
	return ProvisionerTargetReceipt{
		FormatVersion:          1,
		ProfileName:            target.Name,
		Product:                target.Product,
		Version:                target.Version,
		Architecture:           target.Architecture,
		ImageIndexSHA256:       target.ImageIndexSHA256,
		ImageManifestSHA256:    target.ImageManifestSHA256,
		InstanceIdentitySHA256: "sha256:" + strings.Repeat("4", 64),
	}
}

func TestProvisionerReceiptFingerprintIsStableAcrossFreshInstances(t *testing.T) {
	var target TargetProfile
	profile, err := os.ReadFile("../../scout/profiles/network-10.4.57-seeded.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(profile, &target); err != nil {
		t.Fatal(err)
	}
	first := testTargetReceipt(target)
	second := first
	second.InstanceIdentitySHA256 = "sha256:" + strings.Repeat("5", 64)
	firstFingerprint, err := ProvisionerReceiptFingerprint(first)
	if err != nil {
		t.Fatal(err)
	}
	secondFingerprint, err := ProvisionerReceiptFingerprint(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstFingerprint != secondFingerprint {
		t.Fatalf("fresh instance changed stable profile fingerprint: %q != %q", firstFingerprint, secondFingerprint)
	}
	if firstFingerprint != target.ControllerFingerprint {
		t.Fatalf("stable receipt fingerprint = %q, locked profile = %q, receipt = %#v", firstFingerprint, target.ControllerFingerprint, first)
	}
}

func mustCanonicalDigest(t *testing.T, document []byte) string {
	t.Helper()
	digest, err := canonicalDocumentDigest(document)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
