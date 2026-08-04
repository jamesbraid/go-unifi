package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ubiquiti-community/go-unifi/internal/campaign"
)

const builderImage = "golang:1.26.5-bookworm@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651"

func TestRunWritesCandidateAttestationFromTrackedBaseline(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "attestation.json")
	ledger := filepath.Join(directory, "attempts.ndjson")
	var stderr bytes.Buffer
	exitCode := run(campaignArgs(output, ledger, "fixture-attempt-1"), &stderr)
	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, stderr = %s", exitCode, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Classification      string `json:"classification"`
		Promotion           string `json:"promotion"`
		ElapsedMilliseconds int64  `json:"elapsed_milliseconds"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Classification != "unchanged" || got.Promotion != "review_required" || got.ElapsedMilliseconds != 1200 {
		t.Fatalf("attestation = %#v", got)
	}
	events, err := campaign.ReadAttemptLedger(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].State != "started" || events[1].State != "succeeded" {
		t.Fatalf("attempt events = %#v", events)
	}
}

func TestRunRetainsFailedAttemptWithoutAttestation(t *testing.T) {
	directory := t.TempDir()
	badCandidate := filepath.Join(directory, "candidate.json")
	if err := os.WriteFile(badCandidate, []byte(`{"format_version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(directory, "attestation.json")
	ledger := filepath.Join(directory, "attempts.ndjson")
	args := campaignArgs(output, ledger, "failed-attempt-1")
	for index := range args {
		if args[index] == "../../catalogs/network-10.4.57/dns_record.catalog.json" {
			args[index] = badCandidate
		}
	}
	var stderr bytes.Buffer
	if exitCode := run(args, &stderr); exitCode == 0 {
		t.Fatalf("run() succeeded, stderr = %s", stderr.String())
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("failed campaign produced attestation: %v", err)
	}
	events, err := campaign.ReadAttemptLedger(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].State != "failed" || events[1].ErrorClass != "campaign_validation" {
		t.Fatalf("failed attempt events = %#v", events)
	}
}

func TestRunRejectsIdenticalBaselineAndCandidatePath(t *testing.T) {
	directory := t.TempDir()
	args := campaignArgs(filepath.Join(directory, "attestation.json"), filepath.Join(directory, "attempts.ndjson"), "same-path-attempt")
	for index := range args {
		if args[index] == "../../catalogs/network-10.4.57/dns_record.catalog.json" {
			args[index] = "../../catalogs/network-10.4.57/dns_record.admitted-catalog.json"
		}
	}
	var stderr bytes.Buffer
	exitCode := run(args, &stderr)
	if exitCode == 0 || !bytes.Contains(stderr.Bytes(), []byte("same file")) {
		t.Fatalf("run() exit code = %d, stderr = %s", exitCode, stderr.String())
	}
}

func TestRunRequiresBoundInputs(t *testing.T) {
	var stderr bytes.Buffer
	if exitCode := run(nil, &stderr); exitCode == 0 {
		t.Fatal("run() succeeded without bound inputs")
	}
}

func campaignArgs(output, ledger, attemptID string) []string {
	return []string{
		"-campaign-id", "network-10.4.57-dns-record",
		"-profile-class", "fresh_seeded",
		"-builder-image", builderImage,
		"-baseline", "../../catalogs/network-10.4.57/dns_record.admitted-catalog.json",
		"-admission-receipt", "../../catalogs/network-10.4.57/dns_record.admission-receipt.json",
		"-candidate", "../../catalogs/network-10.4.57/dns_record.catalog.json",
		"-receipt", "../../catalogs/network-10.4.57/dns_record.scenario-receipt.json",
		"-execution-receipt", "../../campaigns/fixtures/dns_record.execution-receipt.json",
		"-attempt-ledger", ledger,
		"-attempt-id", attemptID,
		"-elapsed-milliseconds", "1200",
		"-decision", "retain admitted DNS operation",
		"-reconfirmed-claim", "unifi.network.dns_record.field.enabled",
		"-output", output,
	}
}
