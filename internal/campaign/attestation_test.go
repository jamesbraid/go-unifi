package campaign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBuildAttestationCanonicalizesEquivalentCampaign(t *testing.T) {
	input := testInput(testCatalog("candidate", false), testReceipt("candidate"))
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
	if !bytes.Contains(first, []byte(`"conflicts": []`)) {
		t.Fatalf("equivalent attestation encoded empty conflicts as null: %s", first)
	}
	var got attestationView
	if err := json.Unmarshal(first, &got); err != nil {
		t.Fatal(err)
	}
	if got.FormatVersion != 1 || got.Classification != "equivalent" || got.Promotion != "review_required" || got.ProviderContractImpact != "none" || !got.CandidateOnly {
		t.Fatalf("attestation status = %#v", got)
	}
	if len(got.StructuralDiff) != 0 || len(got.ObservationDiff) != 0 || len(got.AffectedOperations) != 0 {
		t.Fatalf("equivalent campaign reported differences: %#v", got)
	}
	if len(got.ReconfirmedClaims) != 1 || got.ReconfirmedClaims[0] != "unifi.network.dns_record.field.enabled" {
		t.Fatalf("reconfirmed claims = %#v", got.ReconfirmedClaims)
	}
}

func TestBuildAttestationBlocksDeliberateStructuralMutation(t *testing.T) {
	output, err := BuildAttestation(testInput(testCatalog("blocked", true), testReceipt("blocked")))
	if err != nil {
		t.Fatal(err)
	}
	var got attestationView
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatal(err)
	}
	if got.Classification != "blocked" || got.Promotion != "review_required" || got.ProviderContractImpact != "review_required" {
		t.Fatalf("mutation status = %#v", got)
	}
	if len(got.StructuralDiff) != 1 || got.StructuralDiff[0] != "unifi.network.dns_record.field.vendor_flag" {
		t.Fatalf("structural diff = %#v", got.StructuralDiff)
	}
	if len(got.AffectedOperations) != 1 || got.AffectedOperations[0] != "sha256:operation" {
		t.Fatalf("affected operations = %#v", got.AffectedOperations)
	}
	if len(got.Conflicts) != 1 || got.Conflicts[0].Kind != "unknown_observed_field" || got.Conflicts[0].Field != "vendor_flag" {
		t.Fatalf("conflicts = %#v", got.Conflicts)
	}
}

func TestBuildAttestationRejectsUnboundEvidence(t *testing.T) {
	tests := map[string]struct {
		mutate func(*Input)
		want   string
	}{
		"unpinned builder": {func(input *Input) { input.BuilderImageDigest = "golang:latest" }, "builder image digest"},
		"receipt target mismatch": {func(input *Input) {
			input.ScenarioReceipt = bytes.ReplaceAll(input.ScenarioReceipt, []byte("10.4.57"), []byte("10.5.0"))
		}, "scenario receipt target"},
		"receipt result mismatch": {func(input *Input) {
			input.ScenarioReceipt = bytes.Replace(input.ScenarioReceipt, []byte(`"result": "candidate"`), []byte(`"result": "blocked"`), 1)
		}, "scenario receipt result"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			input := testInput(testCatalog("candidate", false), testReceipt("candidate"))
			test.mutate(&input)
			_, err := BuildAttestation(input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("BuildAttestation() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

type attestationView struct {
	FormatVersion          int      `json:"format_version"`
	Classification         string   `json:"classification"`
	Promotion              string   `json:"promotion"`
	ProviderContractImpact string   `json:"provider_contract_impact"`
	StructuralDiff         []string `json:"structural_diff"`
	ObservationDiff        []string `json:"observation_diff"`
	AffectedOperations     []string `json:"affected_admitted_operations"`
	ReconfirmedClaims      []string `json:"automatically_reconfirmed_claims"`
	CandidateOnly          bool     `json:"candidate_only"`
	Conflicts              []struct {
		Kind  string `json:"kind"`
		Field string `json:"field"`
	} `json:"conflicts"`
}

func testInput(candidate, receipt []byte) Input {
	return Input{
		CampaignID: "network-10.4.57-dns-record", ProfileClass: "fresh_seeded",
		BuilderImageDigest: "golang:1.26.5@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BaselineCatalog:    testCatalog("candidate", false), CandidateCatalog: candidate, ScenarioReceipt: receipt,
		Elapsed: 3 * time.Second, HumanDecisions: []string{"retain admitted DNS operation"},
		AutomaticallyReconfirmedClaims: []string{"unifi.network.dns_record.field.enabled"},
	}
}

func testCatalog(admission string, mutated bool) []byte {
	structural := `[{"id":"unifi.network.dns_record.field.enabled","field":"enabled","type":"bool","definition_sha256":"sha256:enabled","secret_candidate":false}]`
	observed := `[{"id":"unifi.network.dns_record.field.enabled","field":"enabled","json_type":"bool","present_count":1,"non_null_count":1}]`
	coverage := `[{"id":"unifi.network.dns_record.field.enabled","state":"observed"}]`
	conflicts := `[]`
	if mutated {
		structural = `[{"id":"unifi.network.dns_record.field.enabled","field":"enabled","type":"bool","definition_sha256":"sha256:enabled","secret_candidate":false},{"id":"unifi.network.dns_record.field.vendor_flag","field":"vendor_flag","type":"bool","definition_sha256":"sha256:vendor","secret_candidate":false}]`
		observed = `[{"id":"unifi.network.dns_record.field.enabled","field":"enabled","json_type":"bool","present_count":1,"non_null_count":1},{"id":"unifi.network.dns_record.field.vendor_flag","field":"vendor_flag","json_type":"bool","present_count":1,"non_null_count":1}]`
		coverage = `[{"id":"unifi.network.dns_record.field.enabled","state":"observed"},{"id":"unifi.network.dns_record.field.vendor_flag","state":"observed"}]`
		conflicts = `[{"kind":"unknown_observed_field","field":"vendor_flag"}]`
	}
	return []byte(fmt.Sprintf(`{
  "format_version": 1,
  "catalog_id": "unifi.network.dns_record@10.4.57",
  "target": {
    "name": "network-10.4.57-seeded",
    "product": "unifi-network",
    "version": "10.4.57",
    "architecture": "amd64",
    "image_index_sha256": "sha256:index",
    "image_manifest_sha256": "sha256:manifest",
    "controller_fingerprint": "sha256:fingerprint"
  },
  "sources": {
    "capture_lock_sha256": "sha256:lock",
    "specification_sha256": "sha256:specification"
  },
  "structural_records": %s,
  "observed_records": %s,
  "conflicts": %s,
  "coverage": %s,
  "admission": {"state": %q, "operation_digest": "sha256:operation"},
  "tombstones": [],
  "migrations": []
}`, structural, observed, conflicts, coverage, admission))
}

func testReceipt(result string) []byte {
	return []byte(fmt.Sprintf(`{
  "format_version": 1,
  "scenario_id": "dns-record-list-v1",
  "target": {
    "name": "network-10.4.57-seeded",
    "product": "unifi-network",
    "version": "10.4.57",
    "architecture": "amd64",
    "image_index_sha256": "sha256:index",
    "image_manifest_sha256": "sha256:manifest",
    "controller_fingerprint": "sha256:fingerprint"
  },
  "mode": "read_only",
  "operation_digest": "sha256:operation",
  "observed_record_count": 1,
  "redacted_field_count": 2,
  "canonical_observation_sha256": "sha256:observation",
  "result": %q
}`, result))
}
