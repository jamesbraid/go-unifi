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
	var stderr bytes.Buffer
	exitCode := run([]string{
		"-campaign-id", "network-10.4.57-dns-record",
		"-profile-class", "fresh_seeded",
		"-builder-image", "golang:1.26.5@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"-baseline", "../../catalogs/network-10.4.57/dns_record.catalog.json",
		"-candidate", "../../catalogs/network-10.4.57/dns_record.catalog.json",
		"-receipt", "../../catalogs/network-10.4.57/dns_record.scenario-receipt.json",
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
	if got.Classification != "equivalent" || got.Promotion != "review_required" || got.ElapsedMilliseconds != 1200 {
		t.Fatalf("attestation = %#v", got)
	}
}

func TestRunRequiresBoundInputs(t *testing.T) {
	var stderr bytes.Buffer
	if exitCode := run(nil, &stderr); exitCode == 0 {
		t.Fatal("run() succeeded without bound inputs")
	}
}
