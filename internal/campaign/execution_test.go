package campaign

import (
	"encoding/json"
	"strings"
	"testing"
)

const testBuilderImage = "golang:1.26.5-bookworm@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651"

func TestValidateExecutionReceiptKeepsFixtureScopeExplicit(t *testing.T) {
	candidate := mustCatalogDocument(t, mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.catalog.json"))
	scenario := mustScenarioReceipt(t, mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.scenario-receipt.json"))
	receipt := mustReadTestFile(t, "../../campaigns/fixtures/dns_record.execution-receipt.json")

	canonical, mode, err := validateExecutionReceipt(receipt, nil, nil, testBuilderImage, candidate, scenario)
	if err != nil {
		t.Fatal(err)
	}
	if len(canonical) == 0 || mode != "fixture" {
		t.Fatalf("execution receipt mode = %q, canonical bytes = %d", mode, len(canonical))
	}
}

func TestValidateExecutionReceiptRejectsCallerAssertedRunnerLabel(t *testing.T) {
	candidate := mustCatalogDocument(t, mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.catalog.json"))
	scenario := mustScenarioReceipt(t, mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.scenario-receipt.json"))
	receiptBytes := mustReadTestFile(t, "../../campaigns/fixtures/dns_record.execution-receipt.json")
	var receipt executionReceipt
	if _, err := decodeCanonical(receiptBytes, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.Mode = "runner"
	receipt.BuilderProvenance = "runner"
	receipt.ChecksumSHA256 = ""
	receipt.ChecksumSHA256, _ = executionReceiptChecksum(receipt)
	forged, _ := encodeCanonical(receipt)

	if _, _, err := validateExecutionReceipt(forged, nil, nil, testBuilderImage, candidate, scenario); err == nil || !strings.Contains(err.Error(), "runner evidence") {
		t.Fatalf("validateExecutionReceipt() error = %v, want runner evidence rejection", err)
	}
}

func TestValidateExecutionReceiptBindsMeasuredRunnerEvidence(t *testing.T) {
	candidate := mustCatalogDocument(t, mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.catalog.json"))
	scenario := mustScenarioReceipt(t, mustReadTestFile(t, "../../catalogs/network-10.4.57/dns_record.scenario-receipt.json"))
	evidence := RunnerEvidence{
		WorkflowSHA256: strings.Repeat("b", 64), SourceRevision: strings.Repeat("a", 40),
		PipelineIdentity: "ubiquiti-community/go-unifi#123", BuilderImage: testBuilderImage,
		ObservedToolchain: "go1.26.5",
	}
	receipt, err := BuildRunnerExecutionReceipt(evidence, candidate.Target.ControllerFingerprint, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, mode, err := validateExecutionReceipt(receipt, &evidence, nil, testBuilderImage, candidate, scenario); err != nil || mode != "runner" {
		t.Fatalf("validateExecutionReceipt() mode = %q, error = %v", mode, err)
	}
	forgedMeasurement := evidence
	forgedMeasurement.BuilderImage = "review.invalid/fake@sha256:" + strings.Repeat("a", 64)
	if _, _, err := validateExecutionReceipt(receipt, &forgedMeasurement, nil, forgedMeasurement.BuilderImage, candidate, scenario); err == nil || !strings.Contains(err.Error(), "configured campaign builder") {
		t.Fatalf("validateExecutionReceipt() error = %v, want measured builder rejection", err)
	}
}

func mustCatalogDocument(t *testing.T, document []byte) catalogDocument {
	t.Helper()
	var result catalogDocument
	if _, err := decodeCanonical(document, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func mustScenarioReceipt(t *testing.T, document []byte) scenarioReceipt {
	t.Helper()
	var result scenarioReceipt
	if _, err := decodeCanonical(document, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func mustJSON[T any](t *testing.T, value T) []byte {
	t.Helper()
	result, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
