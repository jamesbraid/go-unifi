package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ubiquiti-community/go-unifi/internal/scout"
)

func TestObserveLiveKeepsUnknownDNSMemberAndBlocksAdmission(t *testing.T) {
	const response = `[{"_id":"record","enabled":true,"key":"fixture.example.invalid","port":53,"priority":10,"record_type":"A","ttl":300,"value":"192.0.2.40","vendor_flag":true,"weight":1}]`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			writer.WriteHeader(http.StatusOK)
		case "/api/auth/login":
			writer.Header().Set("X-Csrf-Token", "test-token")
			writer.WriteHeader(http.StatusOK)
		case "/proxy/network/status":
			_, _ = writer.Write([]byte(`{"meta":{"server_version":"10.4.57"}}`))
		case "/proxy/network/v2/api/site/default/static-dns":
			_, _ = writer.Write([]byte(response))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("UNIFI_API", server.URL)
	t.Setenv("UNIFI_USERNAME", "admin")
	t.Setenv("UNIFI_PASSWORD", "admin")

	observed, err := observeLive(context.Background(), scout.Scenario{
		ID:       "dns-record-list-v1",
		Mode:     "read_only",
		Resource: "dns_record",
		Method:   "GET",
		Path:     "/v2/api/site/{site}/static-dns",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(observed, []byte(`"vendor_flag"`)) {
		t.Fatalf("live observation dropped unknown field: %s", observed)
	}

	input := commandTestInput(observed)
	input.ExecutionMode = "live"
	input.MeasuredTargetFingerprint = "sha256:measured-target"
	result, err := scout.BuildDNSCatalog(input)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Admission struct {
			State string `json:"state"`
		} `json:"admission"`
	}
	if err := json.Unmarshal(result.Catalog, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Admission.State != "blocked" {
		t.Fatalf("admission state = %q, want blocked", catalog.Admission.State)
	}
}

func TestRunBuildsOfflineDNSCatalog(t *testing.T) {
	directory := t.TempDir()
	write := func(name, contents string) string {
		t.Helper()
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	target := write("target.json", `{
  "name":"network-10.4.57-seeded",
  "product":"unifi-network",
  "version":"10.4.57",
  "architecture":"amd64",
  "image_index_sha256":"sha256:index",
  "image_manifest_sha256":"sha256:manifest",
  "controller_fingerprint":"network-10.4.57"
}`)
	scenario := write("scenario.json", `{
  "id":"dns-record-list-v1",
  "mode":"read_only",
  "resource":"dns_record",
  "method":"GET",
  "path":"/v2/api/site/{site}/static-dns"
}`)
	structural := write("structural.json", commandStructuralProjection())
	semanticIDs := write("semantic-ids.json", commandSemanticIDs())
	captureLock := write("capture.lock.json", `{
  "format_version":1,
  "controller":{"product":"unifi-controller","build":"test-build","network_version":"10.4.57"},
  "source":{"location":"https://downloads.example.invalid/controller.deb","media_type":"application/vnd.debian.binary-package","byte_size":1,"sha256":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},
  "inputs":{"extraction_rules_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","generator_inputs_sha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
  "snapshots":{"structural_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","sensitivity_sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
  "captured_at":"2026-08-04T00:00:00Z"
}`)
	response := write("response.json", `[{
  "_id":"must-not-survive",
  "enabled":true,
  "key":"fixture.example.invalid",
  "port":53,
  "priority":10,
  "record_type":"A",
  "ttl":300,
  "value":"192.0.2.30",
  "weight":1
}]`)
	catalogPath := filepath.Join(directory, "catalog.json")
	receiptPath := filepath.Join(directory, "receipt.json")
	var stderr bytes.Buffer
	exitCode := run([]string{
		"-target-profile", target,
		"-scenario", scenario,
		"-structural", structural,
		"-semantic-ids", semanticIDs,
		"-capture-lock", captureLock,
		"-response", response,
		"-catalog-output", catalogPath,
		"-receipt-output", receiptPath,
	}, &stderr)
	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, stderr = %s", exitCode, stderr.String())
	}
	for _, path := range []string{catalogPath, receiptPath} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(contents) {
			t.Fatalf("%s is not JSON", path)
		}
		if bytes.Contains(contents, []byte("must-not-survive")) || bytes.Contains(contents, []byte("192.0.2.30")) {
			t.Fatalf("%s retained an observed value", path)
		}
	}
	receipt, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receiptFields map[string]json.RawMessage
	if err := json.Unmarshal(receipt, &receiptFields); err != nil {
		t.Fatal(err)
	}
	if string(receiptFields["execution_mode"]) != `"fixture"` {
		t.Fatalf("offline execution mode = %s, want fixture", receiptFields["execution_mode"])
	}
	if _, exists := receiptFields["target"]; exists {
		t.Fatalf("fixture receipt claimed a live target: %s", receipt)
	}
}

func TestRunRequiresDeclaredInputs(t *testing.T) {
	var stderr bytes.Buffer
	if exitCode := run(nil, &stderr); exitCode == 0 {
		t.Fatal("run() succeeded without inputs")
	}
}

func commandTestInput(observed []byte) scout.Input {
	return scout.Input{
		Target: scout.TargetProfile{
			Name:                  "network-10.4.57-seeded",
			Product:               "unifi-network",
			Version:               "10.4.57",
			Architecture:          "amd64",
			ImageIndexSHA256:      "sha256:index",
			ImageManifestSHA256:   "sha256:manifest",
			ControllerFingerprint: "sha256:declared-target",
		},
		Scenario: scout.Scenario{
			ID:       "dns-record-list-v1",
			Mode:     "read_only",
			Resource: "dns_record",
			Method:   "GET",
			Path:     "/v2/api/site/{site}/static-dns",
		},
		ExecutionMode: "fixture",
		LockedSources: scout.LockedSources{
			CaptureLockSHA256:     strings.Repeat("d", 64),
			ExtractionRulesSHA256: strings.Repeat("a", 64),
			StructuralSHA256:      strings.Repeat("b", 64),
			SensitivitySHA256:     strings.Repeat("c", 64),
		},
		StructuralProjection: []byte(commandStructuralProjection()),
		SemanticIDs:          []byte(commandSemanticIDs()),
		ObservedResponse:     observed,
	}
}

func commandStructuralProjection() string {
	return `{
  "format_version":1,
  "resource":"dns_record",
  "source":{"structural_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","sensitivity_sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
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
}`
}

func commandSemanticIDs() string {
	return `{
  "format_version":1,
  "resource":"dns_record",
  "extraction_rules_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
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
  "prior_ids":["unifi.network.dns_record.field.enabled","unifi.network.dns_record.field.key","unifi.network.dns_record.field.port","unifi.network.dns_record.field.priority","unifi.network.dns_record.field.record_type","unifi.network.dns_record.field.ttl","unifi.network.dns_record.field.value","unifi.network.dns_record.field.weight"],
  "tombstones":[],
  "migrations":[]
}`
}
