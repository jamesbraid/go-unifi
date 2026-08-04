package campaign

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestValidateAdmissionReceiptRequiresPinnedTrackedArtifact(t *testing.T) {
	baseline := mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.admitted-catalog.json")
	receiptBytes := mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.admission-receipt.json")
	var baselineDocument catalogDocument
	baselineCanonical, err := decodeCanonical(baseline, &baselineDocument)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAdmissionReceipt(baselineCanonical, baselineDocument, receiptBytes); err != nil {
		t.Fatal(err)
	}
	var trackedReceipt admissionReceipt
	if _, err := decodeCanonical(receiptBytes, &trackedReceipt); err != nil {
		t.Fatal(err)
	}
	if digest(baselineCanonical) != trackedReceipt.BaselineCatalogSHA256 {
		t.Fatalf("tracked baseline digest = %q, receipt = %q", digest(baselineCanonical), trackedReceipt.BaselineCatalogSHA256)
	}
	if trackedReceipt.Provenance.SourceCommit != trustedDNSAdmissionSourceCommit {
		t.Fatalf("admission source commit = %q, trusted = %q", trackedReceipt.Provenance.SourceCommit, trustedDNSAdmissionSourceCommit)
	}
	wrongSource := trackedReceipt
	wrongSource.Provenance.SourceCommit = strings.Repeat("a", 40)
	wrongSource.ChecksumSHA256 = ""
	wrongSource.ChecksumSHA256, err = admissionReceiptChecksum(wrongSource)
	if err != nil {
		t.Fatal(err)
	}
	wrongSourceBytes, _ := encodeCanonical(wrongSource)
	if err := validateAdmissionReceipt(baselineCanonical, baselineDocument, wrongSourceBytes); err == nil || !strings.Contains(err.Error(), "trusted admission") {
		t.Fatalf("validateAdmissionReceipt() error = %v, want source commit rejection", err)
	}

	var fake admissionReceipt
	if _, err := decodeCanonical(receiptBytes, &fake); err != nil {
		t.Fatal(err)
	}
	fake.Decision = "self-admit mutable candidate"
	fake.ChecksumSHA256 = ""
	fake.ChecksumSHA256, err = admissionReceiptChecksum(fake)
	if err != nil {
		t.Fatal(err)
	}
	fakeBytes, _ := encodeCanonical(fake)
	if err := validateAdmissionReceipt(baselineCanonical, baselineDocument, fakeBytes); err == nil || !strings.Contains(err.Error(), "trusted admission") {
		t.Fatalf("validateAdmissionReceipt() error = %v, want trusted admission rejection", err)
	}

	relabelledDocument := baselineDocument
	relabelledDocument.Admission.State = "candidate"
	relabelled, _ := encodeCanonical(relabelledDocument)
	if err := validateAdmissionReceipt(relabelled, relabelledDocument, receiptBytes); err == nil || !strings.Contains(err.Error(), "baseline digest") {
		t.Fatalf("validateAdmissionReceipt() error = %v, want baseline digest rejection", err)
	}
}

func TestBuildAttestationCanonicalizesTrackedAdmittedAndCandidatePair(t *testing.T) {
	input := testInput(t)
	first, err := BuildAttestation(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildAttestation(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("equivalent attestations differ:\nfirst: %s\nsecond: %s", first, second)
	}
	var got attestationView
	if err := json.Unmarshal(first, &got); err != nil {
		t.Fatal(err)
	}
	if got.FormatVersion != 1 || got.Classification != "unchanged" || got.Promotion != "review_required" ||
		got.ProviderContractImpact != "none" || !got.CandidateOnly || got.ExecutionMode != "fixture" {
		t.Fatalf("attestation status = %#v", got)
	}
	if len(got.StructuralDiff) != 0 || len(got.ObservationDiff) != 0 || len(got.CoverageDiff) != 0 || len(got.SemanticHistoryDiff) != 0 {
		t.Fatalf("equivalent campaign reported differences: %#v", got)
	}
	if got.Inputs.BaselineCatalogSHA256 == got.Inputs.CandidateCatalogSHA256 || got.Inputs.AdmissionReceiptSHA256 == "" || got.Inputs.ExecutionReceiptSHA256 == "" {
		t.Fatalf("attestation inputs = %#v", got.Inputs)
	}
}

func TestBuildAttestationClassifiesCompatibilityDiffs(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Input)
		wantClass string
	}{
		{"additive candidate", func(input *Input) {
			candidate := mustCatalogDocument(t, input.CandidateCatalog)
			candidate.StructuralRecords = append(candidate.StructuralRecords, structuralRecord{ID: "unifi.network.dns_record.field.vendor_flag", Field: "vendor_flag", Type: "bool", DefinitionSHA256: "sha256:vendor"})
			candidate.ObservedRecords = append(candidate.ObservedRecords, observedRecord{ID: "unifi.network.dns_record.field.vendor_flag", Field: "vendor_flag", JSONType: "bool", PresentCount: 1, NonNullCount: 1})
			candidate.Coverage = append(candidate.Coverage, coverageRecord{ID: "unifi.network.dns_record.field.vendor_flag", State: "observed"})
			input.CandidateCatalog, _ = encodeCanonical(candidate)
			receipt := mustScenarioReceipt(t, input.ScenarioReceipt)
			receipt.ObservedRecordCount++
			input.ScenarioReceipt, _ = encodeCanonical(receipt)
		}, "additive_candidate"},
		{"changed admitted field", func(input *Input) {
			candidate := mustCatalogDocument(t, input.CandidateCatalog)
			candidate.StructuralRecords[0].DefinitionSHA256 = "sha256:changed"
			input.CandidateCatalog, _ = encodeCanonical(candidate)
		}, "breaking_suspect"},
		{"manual generated edit", func(input *Input) { input.ManualGeneratedFileEdits = true }, "generator_defect"},
		{"inconsistent generated records", func(input *Input) {
			candidate := mustCatalogDocument(t, input.CandidateCatalog)
			candidate.ObservedRecords = candidate.ObservedRecords[1:]
			input.CandidateCatalog, _ = encodeCanonical(candidate)
		}, "generator_defect"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := testInput(t)
			input.AutomaticallyReconfirmedClaims = nil
			test.mutate(&input)
			output, err := BuildAttestation(input)
			if err != nil {
				t.Fatal(err)
			}
			var got attestationView
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatal(err)
			}
			if got.Classification != test.wantClass {
				t.Fatalf("classification = %q, want %q", got.Classification, test.wantClass)
			}
		})
	}
}

func TestBuildAttestationTreatsCoverageAndSemanticHistoryAsCompatibilityEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*catalogDocument)
		want   string
	}{
		{"coverage loss", func(candidate *catalogDocument) { candidate.Coverage[0].State = "not_observed" }, "breaking_suspect"},
		{"tombstone change", func(candidate *catalogDocument) {
			candidate.Tombstones = append(candidate.Tombstones, "unifi.network.dns_record.field.retired")
		}, "breaking_suspect"},
		{"migration change", func(candidate *catalogDocument) {
			candidate.Migrations = append(candidate.Migrations, catalogMigration{FromID: "old", ToID: "new", Reason: "reviewed rename", Reviewed: true})
		}, "breaking_suspect"},
		{"invalid coverage vocabulary", func(candidate *catalogDocument) { candidate.Coverage[0].State = "uncovered" }, "generator_defect"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := testInput(t)
			input.AutomaticallyReconfirmedClaims = nil
			candidate := mustCatalogDocument(t, input.CandidateCatalog)
			test.mutate(&candidate)
			input.CandidateCatalog, _ = encodeCanonical(candidate)
			output, err := BuildAttestation(input)
			if err != nil {
				t.Fatal(err)
			}
			var got attestationView
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatal(err)
			}
			if got.Classification != test.want || got.ProviderContractImpact != "review_required" {
				t.Fatalf("classification = %q, impact = %q", got.Classification, got.ProviderContractImpact)
			}
		})
	}
}

func TestBuildAttestationBlocksConflictedCapture(t *testing.T) {
	input := testInput(t)
	candidate := mustCatalogDocument(t, input.CandidateCatalog)
	candidate.Admission.State = "blocked"
	candidate.Conflicts = append(candidate.Conflicts, conflict{Kind: "unknown_observed_field", Field: "vendor_flag"})
	input.CandidateCatalog, _ = encodeCanonical(candidate)
	receipt := mustScenarioReceipt(t, input.ScenarioReceipt)
	receipt.Verdict = "blocked"
	input.ScenarioReceipt, _ = encodeCanonical(receipt)
	output, err := BuildAttestation(input)
	if err != nil {
		t.Fatal(err)
	}
	var got attestationView
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatal(err)
	}
	if got.Classification != "capture_invalid" || len(got.Conflicts) != 1 {
		t.Fatalf("blocked capture = %#v", got)
	}
}

func TestBuildAttestationRejectsSelfComparisonAndIdentityMismatches(t *testing.T) {
	tests := map[string]struct {
		mutate func(*Input)
		want   string
	}{
		"candidate used as baseline": {func(input *Input) { input.BaselineCatalog = input.CandidateCatalog }, "baseline catalog must be admitted"},
		"identical path":             {func(input *Input) { input.CandidatePath = input.BaselinePath }, "paths must differ"},
		"catalog identity": {func(input *Input) {
			candidate := mustCatalogDocument(t, input.CandidateCatalog)
			candidate.CatalogID = "unifi.network.other@10.4.57"
			input.CandidateCatalog, _ = encodeCanonical(candidate)
		}, "catalog identities do not match"},
		"operation identity": {func(input *Input) {
			candidate := mustCatalogDocument(t, input.CandidateCatalog)
			candidate.Admission.OperationDigest = strings.Repeat("a", 64)
			input.CandidateCatalog, _ = encodeCanonical(candidate)
		}, "operation identities do not match"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			input := testInput(t)
			test.mutate(&input)
			_, err := BuildAttestation(input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("BuildAttestation() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestBuildAttestationRejectsIncompleteScenarioBindings(t *testing.T) {
	input := testInput(t)
	receipt := mustScenarioReceipt(t, input.ScenarioReceipt)
	receipt.Normalization = ""
	input.ScenarioReceipt, _ = encodeCanonical(receipt)
	if _, err := BuildAttestation(input); err == nil || !strings.Contains(err.Error(), "scenario receipt bindings are incomplete") {
		t.Fatalf("BuildAttestation() error = %v", err)
	}
}

type attestationView struct {
	FormatVersion          int               `json:"format_version"`
	Classification         string            `json:"classification"`
	Promotion              string            `json:"promotion"`
	ProviderContractImpact string            `json:"provider_contract_impact"`
	ExecutionMode          string            `json:"execution_mode"`
	StructuralDiff         []string          `json:"structural_diff"`
	ObservationDiff        []string          `json:"observation_diff"`
	CoverageDiff           []string          `json:"coverage_diff"`
	SemanticHistoryDiff    []string          `json:"semantic_history_diff"`
	CandidateOnly          bool              `json:"candidate_only"`
	Conflicts              []conflict        `json:"conflicts"`
	Inputs                 attestationInputs `json:"inputs"`
}

func testInput(t *testing.T) Input {
	t.Helper()
	return Input{
		CampaignID: "network-10.4.57-dns-record", ProfileClass: "fresh_seeded",
		BuilderImageDigest: testBuilderImage,
		BaselinePath:       "catalogs/network-10.4.57/dns_record.admitted-catalog.json",
		CandidatePath:      "catalogs/network-10.4.57/dns_record.catalog.json",
		BaselineCatalog:    mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.admitted-catalog.json"),
		AdmissionReceipt:   mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.admission-receipt.json"),
		CandidateCatalog:   mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.catalog.json"),
		ScenarioReceipt:    mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.scenario-receipt.json"),
		ExecutionReceipt:   mustReadTestFile(t, "../../campaigns/fixtures/dns_record.execution-receipt.json"),
		Elapsed:            3 * time.Second, HumanDecisions: []string{"retain admitted DNS operation"},
		AutomaticallyReconfirmedClaims: []string{"unifi.network.dns_record.field.enabled"},
	}
}

func mustReadTestFile(t *testing.T, path string) []byte {
	t.Helper()
	document, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return document
}
