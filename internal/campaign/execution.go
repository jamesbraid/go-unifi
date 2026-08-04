package campaign

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/ubiquiti-community/go-unifi/internal/scout"
)

const fixtureExecutionReceiptID = "unifi.network.dns_record.fixture-execution.1"

var sourceRevisionPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// RunnerEvidence is measured by cmd/campaign from the checked workflow, git,
// the CI environment, and the running Go toolchain. It is not decoded from the
// caller's execution receipt.
type RunnerEvidence struct {
	WorkflowSHA256    string
	SourceRevision    string
	PipelineIdentity  string
	BuilderImage      string
	ObservedToolchain string
}

// MeasureRunnerEvidence reads facts from the running checkout and CI process.
// None of these values are accepted from an execution receipt.
func MeasureRunnerEvidence(workflowPath, configuredBuilder string) (RunnerEvidence, error) {
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		return RunnerEvidence{}, fmt.Errorf("read workflow: %w", err)
	}
	command := exec.Command("git", "rev-parse", "HEAD")
	revisionBytes, err := command.Output()
	if err != nil {
		return RunnerEvidence{}, fmt.Errorf("measure source revision: %w", err)
	}
	revision := strings.TrimSpace(string(revisionBytes))
	repository := strings.TrimSpace(os.Getenv("CI_REPO"))
	pipelineNumber := strings.TrimSpace(os.Getenv("CI_PIPELINE_NUMBER"))
	if repository == "" || pipelineNumber == "" {
		return RunnerEvidence{}, fmt.Errorf("CI_REPO and CI_PIPELINE_NUMBER are required")
	}
	pipelineIdentity := repository + "#" + pipelineNumber
	observedBuilder := strings.TrimSpace(os.Getenv("CAMPAIGN_BUILDER_IMAGE"))
	if observedBuilder == "" || observedBuilder != configuredBuilder {
		return RunnerEvidence{}, fmt.Errorf("CAMPAIGN_BUILDER_IMAGE does not match configured campaign builder")
	}
	evidence := RunnerEvidence{
		WorkflowSHA256: digest(workflow), SourceRevision: revision, PipelineIdentity: pipelineIdentity,
		BuilderImage: observedBuilder, ObservedToolchain: runtime.Version(),
	}
	if !validSHA256(evidence.WorkflowSHA256) || !sourceRevisionPattern.MatchString(evidence.SourceRevision) {
		return RunnerEvidence{}, fmt.Errorf("measured workflow or source revision is invalid")
	}
	return evidence, nil
}

// BuildRunnerExecutionReceipt creates a checksum-bound receipt from runner
// evidence measured by MeasureRunnerEvidence. A provisioner receipt switches
// controller/runtime scope from fixture to live.
func BuildRunnerExecutionReceipt(evidence RunnerEvidence, controllerFingerprint string, provisionerDocument []byte) ([]byte, error) {
	receipt := executionReceipt{
		FormatVersion: 1, ReceiptID: "unifi.network.dns_record.runner-execution.1", Mode: "runner",
		WorkflowSHA256: evidence.WorkflowSHA256, SourceRevision: evidence.SourceRevision,
		PipelineIdentity: evidence.PipelineIdentity, BuilderImageDigest: evidence.BuilderImage,
		BuilderProvenance: "runner_observed", ObservedToolchain: evidence.ObservedToolchain,
		ControllerFingerprint: controllerFingerprint, ControllerProvenance: "fixture", RuntimeProvenance: "fixture",
	}
	if len(provisionerDocument) != 0 {
		var provisioner scout.ProvisionerTargetReceipt
		canonical, err := decodeCanonical(provisionerDocument, &provisioner)
		if err != nil {
			return nil, fmt.Errorf("provisioner receipt: %w", err)
		}
		fingerprint, err := scout.ProvisionerReceiptFingerprint(provisioner)
		if err != nil {
			return nil, err
		}
		receipt.ControllerFingerprint = fingerprint
		receipt.ControllerProvenance = "provisioner"
		receipt.RuntimeIdentitySHA256 = provisioner.InstanceIdentitySHA256
		receipt.RuntimeProvenance = "provisioner"
		receipt.ProvisionerReceiptSHA256 = digest(canonical)
	}
	if !pinnedImageDigest(receipt.BuilderImageDigest) || !validSHA256(receipt.WorkflowSHA256) ||
		!sourceRevisionPattern.MatchString(receipt.SourceRevision) || strings.TrimSpace(receipt.PipelineIdentity) == "" ||
		strings.TrimSpace(receipt.ObservedToolchain) == "" || strings.TrimSpace(receipt.ControllerFingerprint) == "" {
		return nil, fmt.Errorf("runner evidence bindings are incomplete")
	}
	checksum, err := executionReceiptChecksum(receipt)
	if err != nil {
		return nil, err
	}
	receipt.ChecksumSHA256 = checksum
	return encodeCanonical(receipt)
}

type executionReceipt struct {
	FormatVersion            int    `json:"format_version"`
	ReceiptID                string `json:"receipt_id"`
	Mode                     string `json:"mode"`
	WorkflowSHA256           string `json:"workflow_sha256,omitempty"`
	SourceRevision           string `json:"source_revision,omitempty"`
	PipelineIdentity         string `json:"pipeline_identity,omitempty"`
	BuilderImageDigest       string `json:"builder_image_digest"`
	BuilderProvenance        string `json:"builder_provenance"`
	ObservedToolchain        string `json:"observed_toolchain,omitempty"`
	ControllerFingerprint    string `json:"controller_fingerprint"`
	ControllerProvenance     string `json:"controller_provenance"`
	RuntimeIdentitySHA256    string `json:"runtime_identity_sha256,omitempty"`
	RuntimeProvenance        string `json:"runtime_provenance"`
	ProvisionerReceiptSHA256 string `json:"provisioner_receipt_sha256,omitempty"`
	ChecksumSHA256           string `json:"checksum_sha256"`
}

type executionReceiptPayload struct {
	FormatVersion            int    `json:"format_version"`
	ReceiptID                string `json:"receipt_id"`
	Mode                     string `json:"mode"`
	WorkflowSHA256           string `json:"workflow_sha256,omitempty"`
	SourceRevision           string `json:"source_revision,omitempty"`
	PipelineIdentity         string `json:"pipeline_identity,omitempty"`
	BuilderImageDigest       string `json:"builder_image_digest"`
	BuilderProvenance        string `json:"builder_provenance"`
	ObservedToolchain        string `json:"observed_toolchain,omitempty"`
	ControllerFingerprint    string `json:"controller_fingerprint"`
	ControllerProvenance     string `json:"controller_provenance"`
	RuntimeIdentitySHA256    string `json:"runtime_identity_sha256,omitempty"`
	RuntimeProvenance        string `json:"runtime_provenance"`
	ProvisionerReceiptSHA256 string `json:"provisioner_receipt_sha256,omitempty"`
}

func validateExecutionReceipt(document []byte, runner *RunnerEvidence, provisionerDocument []byte, expectedBuilder string, candidate catalogDocument, scenario scenarioReceipt) ([]byte, string, error) {
	var receipt executionReceipt
	canonical, err := decodeCanonical(document, &receipt)
	if err != nil {
		return nil, "", fmt.Errorf("execution receipt: %w", err)
	}
	if receipt.FormatVersion != 1 || strings.TrimSpace(receipt.ReceiptID) == "" {
		return nil, "", fmt.Errorf("execution receipt identity is incomplete")
	}
	checksum, err := executionReceiptChecksum(receipt)
	if err != nil {
		return nil, "", err
	}
	if receipt.ChecksumSHA256 != checksum {
		return nil, "", fmt.Errorf("execution receipt checksum does not match its canonical payload")
	}
	if receipt.BuilderImageDigest != expectedBuilder || !pinnedImageDigest(receipt.BuilderImageDigest) {
		return nil, "", fmt.Errorf("execution receipt builder image does not match configured campaign builder")
	}
	if receipt.ControllerFingerprint != candidate.Target.ControllerFingerprint {
		return nil, "", fmt.Errorf("execution receipt controller fingerprint does not match candidate")
	}

	switch receipt.Mode {
	case "fixture":
		if receipt.ReceiptID != fixtureExecutionReceiptID || receipt.BuilderProvenance != "fixture_bundle" ||
			receipt.ControllerProvenance != "fixture" || receipt.RuntimeProvenance != "fixture" ||
			receipt.WorkflowSHA256 != "" || receipt.SourceRevision != "" || receipt.PipelineIdentity != "" ||
			receipt.ObservedToolchain != "" || receipt.RuntimeIdentitySHA256 != "" || receipt.ProvisionerReceiptSHA256 != "" ||
			runner != nil || len(provisionerDocument) != 0 || scenario.ExecutionMode != "fixture" {
			return nil, "", fmt.Errorf("fixture execution receipt must remain fixture-scoped")
		}
	case "runner":
		if runner == nil {
			return nil, "", fmt.Errorf("runner evidence measured by cmd/campaign is required")
		}
		if receipt.BuilderProvenance != "runner_observed" || receipt.WorkflowSHA256 != runner.WorkflowSHA256 ||
			receipt.SourceRevision != runner.SourceRevision || receipt.PipelineIdentity != runner.PipelineIdentity ||
			receipt.BuilderImageDigest != runner.BuilderImage || receipt.ObservedToolchain != runner.ObservedToolchain {
			return nil, "", fmt.Errorf("execution receipt does not match runner evidence")
		}
		if !validSHA256(receipt.WorkflowSHA256) || !sourceRevisionPattern.MatchString(receipt.SourceRevision) ||
			strings.TrimSpace(receipt.PipelineIdentity) == "" || strings.TrimSpace(receipt.ObservedToolchain) == "" {
			return nil, "", fmt.Errorf("runner evidence bindings are incomplete")
		}
		if scenario.ExecutionMode == "fixture" {
			if receipt.ControllerProvenance != "fixture" || receipt.RuntimeProvenance != "fixture" ||
				receipt.RuntimeIdentitySHA256 != "" || receipt.ProvisionerReceiptSHA256 != "" || len(provisionerDocument) != 0 {
				return nil, "", fmt.Errorf("runner fixture execution must keep controller and runtime fixture-scoped")
			}
		} else if err := validateLiveExecution(receipt, provisionerDocument, candidate, scenario); err != nil {
			return nil, "", err
		}
	default:
		return nil, "", fmt.Errorf("unsupported execution receipt mode %q", receipt.Mode)
	}
	return canonical, receipt.Mode, nil
}

func validateLiveExecution(receipt executionReceipt, provisionerDocument []byte, candidate catalogDocument, scenario scenarioReceipt) error {
	if scenario.ExecutionMode != "live" || receipt.ControllerProvenance != "provisioner" || receipt.RuntimeProvenance != "provisioner" {
		return fmt.Errorf("live execution requires provisioner-scoped controller and runtime evidence")
	}
	var provisioner scout.ProvisionerTargetReceipt
	canonical, err := decodeCanonical(provisionerDocument, &provisioner)
	if err != nil {
		return fmt.Errorf("provisioner receipt: %w", err)
	}
	fingerprint, err := scout.ProvisionerReceiptFingerprint(provisioner)
	if err != nil {
		return fmt.Errorf("provisioner receipt: %w", err)
	}
	if digest(canonical) != receipt.ProvisionerReceiptSHA256 || fingerprint != candidate.Target.ControllerFingerprint ||
		fingerprint != scenario.MeasuredTargetFingerprint || provisioner.InstanceIdentitySHA256 != receipt.RuntimeIdentitySHA256 ||
		provisioner.InstanceIdentitySHA256 != scenario.ObservedInstanceIdentitySHA256 || provisioner.Version != candidate.Target.Version ||
		provisioner.ImageIndexSHA256 != candidate.Target.ImageIndexSHA256 || provisioner.ImageManifestSHA256 != candidate.Target.ImageManifestSHA256 {
		return fmt.Errorf("live execution receipt does not match independently measured provisioner and scout evidence")
	}
	return nil
}

func executionReceiptChecksum(receipt executionReceipt) (string, error) {
	payload := executionReceiptPayload{
		FormatVersion: receipt.FormatVersion, ReceiptID: receipt.ReceiptID, Mode: receipt.Mode,
		WorkflowSHA256: receipt.WorkflowSHA256, SourceRevision: receipt.SourceRevision, PipelineIdentity: receipt.PipelineIdentity,
		BuilderImageDigest: receipt.BuilderImageDigest, BuilderProvenance: receipt.BuilderProvenance,
		ObservedToolchain: receipt.ObservedToolchain, ControllerFingerprint: receipt.ControllerFingerprint,
		ControllerProvenance: receipt.ControllerProvenance, RuntimeIdentitySHA256: receipt.RuntimeIdentitySHA256,
		RuntimeProvenance: receipt.RuntimeProvenance, ProvisionerReceiptSHA256: receipt.ProvisionerReceiptSHA256,
	}
	document, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return digest(document), nil
}

func validSHA256(value string) bool {
	return len(value) == 64 && strings.ToLower(value) == value && sourceRevisionPattern.MatchString(value)
}
