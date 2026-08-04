package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunWritesCandidateAttestation(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "attestation.json")
	candidate := "../../catalogs/network-10.4.57/dns_record.catalog.json"
	candidateBytes, err := os.ReadFile(candidate)
	if err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(directory, "dns_record.admitted.catalog.json")
	admittedBytes := bytes.Replace(candidateBytes, []byte(`"state": "candidate"`), []byte(`"state": "admitted"`), 1)
	if err := os.WriteFile(baseline, admittedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	exitCode := run([]string{
		"-campaign-id", "network-10.4.57-dns-record",
		"-profile-class", "fresh_seeded",
		"-builder-image", "golang:1.26.5@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"-baseline", baseline,
		"-candidate", candidate,
		"-receipt", "../../catalogs/network-10.4.57/dns_record.scenario-receipt.json",
		"-attempt-json", `{"id":"fixture-attempt-1","outcome":"completed","execution_mode":"fixture","builder_image_digest":"golang:1.26.5@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","builder_provenance":"fixture","controller_fingerprint":"sha256:5a9624d7671e1cead43755f3f2bdb37d852460fe58e75b184af85eb67635751a","controller_provenance":"fixture","runtime_provenance":"fixture"}`,
		"-elapsed-milliseconds", "1200",
		"-decision", "retain admitted DNS operation",
		"-reconfirmed-claim", "unifi.network.dns_record.field.enabled",
		"-output", output,
	}, &stderr)
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
}

func TestRunRejectsIdenticalBaselineAndCandidatePath(t *testing.T) {
	path := "../../catalogs/network-10.4.57/dns_record.catalog.json"
	var stderr bytes.Buffer
	exitCode := run([]string{
		"-campaign-id", "network-10.4.57-dns-record",
		"-profile-class", "fresh_seeded",
		"-builder-image", "golang:1.26.5@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"-baseline", path,
		"-candidate", path,
		"-receipt", "../../catalogs/network-10.4.57/dns_record.scenario-receipt.json",
		"-attempt-json", `{"id":"fixture-attempt-1","outcome":"completed","execution_mode":"fixture","builder_image_digest":"golang:1.26.5@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","builder_provenance":"fixture","controller_fingerprint":"sha256:5a9624d7671e1cead43755f3f2bdb37d852460fe58e75b184af85eb67635751a","controller_provenance":"fixture","runtime_provenance":"fixture"}`,
		"-output", filepath.Join(t.TempDir(), "attestation.json"),
	}, &stderr)
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
