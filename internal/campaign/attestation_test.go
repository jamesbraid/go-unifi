package campaign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBuildAttestationCanonicalizesAdmittedAndCandidatePair(t *testing.T) {
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
	if got.FormatVersion != 1 || got.Classification != "unchanged" || got.Promotion != "review_required" || got.ProviderContractImpact != "none" || !got.CandidateOnly {
		t.Fatalf("attestation status = %#v", got)
	}
	if len(got.StructuralDiff) != 0 || len(got.ObservationDiff) != 0 || len(got.AffectedOperations) != 0 {
		t.Fatalf("equivalent campaign reported differences: %#v", got)
	}
	if len(got.ReconfirmedClaims) != 1 || got.ReconfirmedClaims[0] != "unifi.network.dns_record.field.enabled" {
		t.Fatalf("reconfirmed claims = %#v", got.ReconfirmedClaims)
	}
	if got.Inputs.BaselineCatalogSHA256 == got.Inputs.CandidateCatalogSHA256 {
		t.Fatalf("admitted baseline and candidate hashes are identical: %#v", got.Inputs)
	}
	if len(got.Attempts) != 1 || got.Attempts[0].ID != "fixture-attempt-1" {
		t.Fatalf("attempts = %#v", got.Attempts)
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
	if got.Classification != "capture_invalid" || got.Promotion != "review_required" || got.ProviderContractImpact != "review_required" {
		t.Fatalf("mutation status = %#v", got)
	}
	if len(got.StructuralDiff) != 1 || got.StructuralDiff[0] != "unifi.network.dns_record.field.vendor_flag" {
		t.Fatalf("structural diff = %#v", got.StructuralDiff)
	}
	if len(got.AffectedOperations) != 1 || got.AffectedOperations[0] != "199c002d9a1229aa7e43a94b12da6f33c7ce612ecb471eebcd6896d985b161f8" {
		t.Fatalf("affected operations = %#v", got.AffectedOperations)
	}
	if len(got.Conflicts) != 1 || got.Conflicts[0].Kind != "unknown_observed_field" || got.Conflicts[0].Field != "vendor_flag" {
		t.Fatalf("conflicts = %#v", got.Conflicts)
	}
}

func TestBuildAttestationClassifiesCompatibilityDiffs(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Input)
		wantClass string
	}{
		{
			name: "additive candidate",
			mutate: func(input *Input) {
				input.CandidateCatalog = addCatalogField(input.CandidateCatalog)
				input.ScenarioReceipt = bytes.Replace(input.ScenarioReceipt, []byte(`"observed_record_count": 1`), []byte(`"observed_record_count": 2`), 1)
			},
			wantClass: "additive_candidate",
		},
		{
			name: "changed admitted field",
			mutate: func(input *Input) {
				input.CandidateCatalog = bytes.Replace(input.CandidateCatalog, []byte(`"definition_sha256":"sha256:enabled"`), []byte(`"definition_sha256":"sha256:changed"`), 1)
			},
			wantClass: "breaking_suspect",
		},
		{
			name: "manual generated edit",
			mutate: func(input *Input) {
				input.ManualGeneratedFileEdits = true
			},
			wantClass: "generator_defect",
		},
		{
			name: "inconsistent generated records",
			mutate: func(input *Input) {
				input.CandidateCatalog = bytes.Replace(input.CandidateCatalog,
					[]byte(`[{"id":"unifi.network.dns_record.field.enabled","field":"enabled","json_type":"bool","present_count":1,"non_null_count":1}]`),
					[]byte(`[]`), 1)
			},
			wantClass: "generator_defect",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := testInput(testCatalog("candidate", false), testReceipt("candidate"))
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

func TestBuildAttestationRejectsSelfComparisonAndIdentityMismatches(t *testing.T) {
	tests := map[string]struct {
		mutate func(*Input)
		want   string
	}{
		"candidate used as baseline": {func(input *Input) {
			input.BaselineCatalog = testCatalog("candidate", false)
		}, "baseline catalog must be admitted"},
		"identical path": {func(input *Input) {
			input.CandidatePath = input.BaselinePath
		}, "paths must differ"},
		"catalog identity": {func(input *Input) {
			input.CandidateCatalog = bytes.Replace(input.CandidateCatalog, []byte(`unifi.network.dns_record@10.4.57`), []byte(`unifi.network.other@10.4.57`), 1)
		}, "catalog identities do not match"},
		"operation identity": {func(input *Input) {
			input.CandidateCatalog = bytes.Replace(input.CandidateCatalog, []byte(`199c002d9a1229aa7e43a94b12da6f33c7ce612ecb471eebcd6896d985b161f8`), []byte(`sha256:different`), 1)
		}, "operation identities do not match"},
		"normalized operation identity": {func(input *Input) {
			input.BaselineCatalog = bytes.Replace(input.BaselineCatalog, []byte(`199c002d9a1229aa7e43a94b12da6f33c7ce612ecb471eebcd6896d985b161f8`), []byte(`sha256:other-operation`), 1)
			input.CandidateCatalog = bytes.Replace(input.CandidateCatalog, []byte(`199c002d9a1229aa7e43a94b12da6f33c7ce612ecb471eebcd6896d985b161f8`), []byte(`sha256:other-operation`), 1)
			input.ScenarioReceipt = bytes.Replace(input.ScenarioReceipt, []byte(`199c002d9a1229aa7e43a94b12da6f33c7ce612ecb471eebcd6896d985b161f8`), []byte(`sha256:other-operation`), 1)
		}, "operation identity does not match normalized operation"},
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

func TestBuildAttestationRequiresScopedAttemptProvenance(t *testing.T) {
	input := testInput(testCatalog("candidate", false), testReceipt("candidate"))
	input.Attempts = []Attempt{
		{
			ID: "fixture-attempt-0", Outcome: "failed", ExecutionMode: "fixture",
			BuilderImageDigest: input.BuilderImageDigest, BuilderProvenance: "fixture",
			ControllerFingerprint: "sha256:fingerprint", ControllerProvenance: "fixture",
			RuntimeProvenance: "fixture",
		},
		input.Attempts[0],
	}
	output, err := BuildAttestation(input)
	if err != nil {
		t.Fatal(err)
	}
	var got attestationView
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Attempts) != 2 || got.Attempts[0].Outcome != "failed" || got.Attempts[1].Outcome != "completed" {
		t.Fatalf("attempt history = %#v", got.Attempts)
	}

	input.Attempts[0].ControllerProvenance = "provisioner"
	if _, err := BuildAttestation(input); err == nil || !strings.Contains(err.Error(), "fixture-scoped") {
		t.Fatalf("BuildAttestation() error = %v, want fixture provenance rejection", err)
	}
}

func TestBuildAttestationRejectsUnboundEvidence(t *testing.T) {
	tests := map[string]struct {
		mutate func(*Input)
		want   string
	}{
		"unpinned builder": {func(input *Input) { input.BuilderImageDigest = "golang:latest" }, "builder image digest"},
		"fixture receipt target claim": {func(input *Input) {
			input.ScenarioReceipt = bytes.Replace(input.ScenarioReceipt, []byte(`"execution_mode": "fixture"`), []byte(`"execution_mode": "fixture", "measured_target_fingerprint": "sha256:not-measured"`), 1)
		}, "fixture receipt target"},
		"receipt result mismatch": {func(input *Input) {
			input.ScenarioReceipt = bytes.Replace(input.ScenarioReceipt, []byte(`"verdict": "candidate"`), []byte(`"verdict": "blocked"`), 1)
		}, "scenario receipt result"},
		"live receipt missing observed instance": {func(input *Input) {
			input.ScenarioReceipt = bytes.Replace(input.ScenarioReceipt, []byte(`"execution_mode": "fixture"`), []byte(`"execution_mode": "live", "measured_target_fingerprint": "sha256:fingerprint", "controller_version": "10.4.57"`), 1)
		}, "observed instance identity"},
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

func TestBuildAttestationRequiresCompleteBindingsForFixtureAndLive(t *testing.T) {
	bindings := map[string]string{
		"scenario path":   "  \"scenario_path\": \"/v2/api/site/{site}/static-dns\",\n",
		"request shape":   "    \"method\": \"GET\",\n",
		"response digest": "  \"response_sha256\": \"sha256:response\",\n",
		"normalization":   "  \"normalization\": \"field_presence_and_json_type_v1\",\n",
		"redaction":       "  \"redaction\": \"drop_all_observed_values_v1\",\n",
		"cleanup":         "  \"cleanup\": \"not_required_read_only\",\n",
	}
	for _, executionMode := range []string{"fixture", "live"} {
		for name, binding := range bindings {
			t.Run(executionMode+"/"+name, func(t *testing.T) {
				receipt := testReceipt("candidate")
				if executionMode == "live" {
					receipt = bytes.Replace(receipt, []byte(`"execution_mode": "fixture"`), []byte(`"execution_mode": "live", "measured_target_fingerprint": "sha256:fingerprint", "controller_version": "10.4.57", "observed_instance_identity_sha256": "sha256:4444444444444444444444444444444444444444444444444444444444444444"`), 1)
				}
				mutated := bytes.Replace(receipt, []byte(binding), nil, 1)
				if bytes.Equal(mutated, receipt) {
					t.Fatalf("test did not remove %s", name)
				}
				_, err := BuildAttestation(testInput(testCatalog("candidate", false), mutated))
				if err == nil || !strings.Contains(err.Error(), "scenario receipt bindings are incomplete") {
					t.Fatalf("BuildAttestation() error = %v, want incomplete binding rejection", err)
				}
			})
		}
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
	Inputs struct {
		BaselineCatalogSHA256  string `json:"baseline_catalog_sha256"`
		CandidateCatalogSHA256 string `json:"candidate_catalog_sha256"`
	} `json:"inputs"`
	Attempts []Attempt `json:"attempts"`
}

func testInput(candidate, receipt []byte) Input {
	return Input{
		CampaignID: "network-10.4.57-dns-record", ProfileClass: "fresh_seeded",
		BuilderImageDigest: "golang:1.26.5@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BaselinePath:       "catalogs/admitted/dns_record.catalog.json",
		CandidatePath:      "catalogs/candidate/dns_record.catalog.json",
		BaselineCatalog:    testCatalog("admitted", false), CandidateCatalog: candidate, ScenarioReceipt: receipt,
		Elapsed: 3 * time.Second, HumanDecisions: []string{"retain admitted DNS operation"},
		AutomaticallyReconfirmedClaims: []string{"unifi.network.dns_record.field.enabled"},
		Attempts: []Attempt{{
			ID: "fixture-attempt-1", Outcome: "completed", ExecutionMode: "fixture",
			BuilderImageDigest: "golang:1.26.5@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			BuilderProvenance:  "fixture", ControllerFingerprint: "sha256:fingerprint",
			ControllerProvenance: "fixture", RuntimeProvenance: "fixture",
		}},
	}
}

func addCatalogField(catalog []byte) []byte {
	catalog = bytes.Replace(catalog,
		[]byte(`{"id":"unifi.network.dns_record.field.enabled","field":"enabled","type":"bool","definition_sha256":"sha256:enabled","secret_candidate":false}]`),
		[]byte(`{"id":"unifi.network.dns_record.field.enabled","field":"enabled","type":"bool","definition_sha256":"sha256:enabled","secret_candidate":false},{"id":"unifi.network.dns_record.field.ttl","field":"ttl","type":"number","definition_sha256":"sha256:ttl","secret_candidate":false}]`), 1)
	catalog = bytes.Replace(catalog,
		[]byte(`{"id":"unifi.network.dns_record.field.enabled","field":"enabled","json_type":"bool","present_count":1,"non_null_count":1}]`),
		[]byte(`{"id":"unifi.network.dns_record.field.enabled","field":"enabled","json_type":"bool","present_count":1,"non_null_count":1},{"id":"unifi.network.dns_record.field.ttl","field":"ttl","json_type":"number","present_count":1,"non_null_count":1}]`), 1)
	catalog = bytes.Replace(catalog,
		[]byte(`[{"id":"unifi.network.dns_record.field.enabled","state":"observed"}]`),
		[]byte(`[{"id":"unifi.network.dns_record.field.enabled","state":"observed"},{"id":"unifi.network.dns_record.field.ttl","state":"observed"}]`), 1)
	return catalog
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
    "structural_projection_sha256": "sha256:structural",
    "semantic_predecessor_sha256": "sha256:semantic-predecessor",
    "semantic_ids_sha256": "sha256:semantic-ids"
  },
  "structural_records": %s,
  "observed_records": %s,
  "conflicts": %s,
  "coverage": %s,
  "admission": {"state": %q, "operation_digest": "199c002d9a1229aa7e43a94b12da6f33c7ce612ecb471eebcd6896d985b161f8"},
  "tombstones": [],
  "migrations": []
}`, structural, observed, conflicts, coverage, admission))
}

func testReceipt(result string) []byte {
	return []byte(fmt.Sprintf(`{
  "format_version": 1,
  "scenario_id": "dns-record-list-v1",
  "scenario_path": "/v2/api/site/{site}/static-dns",
  "scenario_mode": "read_only",
  "request_shape": {
    "method": "GET",
    "path": "/v2/api/site/{site}/static-dns",
    "query": "none",
    "body": "none"
  },
  "execution_mode": "fixture",
  "operation_digest": "199c002d9a1229aa7e43a94b12da6f33c7ce612ecb471eebcd6896d985b161f8",
  "response_sha256": "sha256:response",
  "normalization": "field_presence_and_json_type_v1",
  "redaction": "drop_all_observed_values_v1",
  "cleanup": "not_required_read_only",
  "observed_record_count": 1,
  "redacted_field_count": 2,
  "canonical_observation_sha256": "sha256:observation",
  "verdict": %q
}`, result))
}
