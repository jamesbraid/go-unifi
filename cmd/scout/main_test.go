package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ubiquiti-community/go-unifi/internal/scout"
)

// commandDNSDocumentSHA stands in for schemas/fields/DnsRecord.json's digest.
// It is deliberately not the structural snapshot digest these fixtures also
// carry: the projection pins its own source document, and the two values
// moving independently is the property under test.
const commandDNSDocumentSHA = "1111111111111111111111111111111111111111111111111111111111111111"

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
			_, _ = writer.Write([]byte(`{"meta":{"server_version":"10.4.57","uuid":"44444444-4444-4444-8444-444444444444"}}`))
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

	observation, err := observeLive(context.Background(), scout.Scenario{
		ID:       "dns-record-list-v1",
		Mode:     "read_only",
		Resource: "dns_record",
		Method:   "GET",
		Path:     "/v2/api/site/{site}/static-dns",
	})
	if err != nil {
		t.Fatal(err)
	}
	if observation.ControllerVersion != "10.4.57" {
		t.Fatalf("observed controller version = %q, want 10.4.57", observation.ControllerVersion)
	}
	if !bytes.Contains(observation.Response, []byte(`"vendor_flag"`)) {
		t.Fatalf("live observation dropped unknown field: %s", observation.Response)
	}

	input := commandTestInput(t, observation.Response)
	makeCommandLiveInput(t, &input, observation.ControllerVersion)
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

func TestRunBindsLiveObservationToControllerInstance(t *testing.T) {
	const (
		provisionedUUID = "11111111-1111-4111-8111-111111111111"
		otherUUID       = "22222222-2222-4222-8222-222222222222"
	)
	for _, test := range []struct {
		name         string
		observedUUID string
		wantExit     int
	}{
		{name: "matching instance", observedUUID: provisionedUUID, wantExit: 0},
		{name: "same version different instance", observedUUID: otherUUID, wantExit: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			const response = `[{"enabled":true,"key":"fixture.example.invalid","port":53,"priority":10,"record_type":"A","ttl":300,"value":"192.0.2.40","weight":1}]`
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/":
					writer.WriteHeader(http.StatusOK)
				case "/api/auth/login":
					writer.Header().Set("X-Csrf-Token", "test-token")
					writer.WriteHeader(http.StatusOK)
				case "/proxy/network/status":
					_, _ = fmt.Fprintf(writer, `{"meta":{"server_version":"10.4.57","uuid":%q}}`, test.observedUUID)
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

			directory := t.TempDir()
			write := func(name string, contents []byte) string {
				t.Helper()
				path := filepath.Join(directory, name)
				if err := os.WriteFile(path, contents, 0o600); err != nil {
					t.Fatal(err)
				}
				return path
			}
			input := commandTestInput(t, []byte(response))
			targetReceipt := commandTargetReceipt(input.Target, provisionedUUID)
			fingerprint, err := scout.ProvisionerReceiptFingerprint(targetReceipt)
			if err != nil {
				t.Fatal(err)
			}
			input.Target.ControllerFingerprint = fingerprint
			targetJSON, err := json.Marshal(input.Target)
			if err != nil {
				t.Fatal(err)
			}
			targetReceiptJSON, err := json.Marshal(targetReceipt)
			if err != nil {
				t.Fatal(err)
			}
			captureLock := fmt.Sprintf(`{
  "format_version":1,
  "controller":{"product":"unifi-controller","build":"test-build","network_version":"10.4.57"},
  "source":{"location":"https://downloads.example.invalid/controller.deb","media_type":"application/vnd.debian.binary-package","byte_size":1,"sha256":"%s"},
  "inputs":{"extraction_rules_sha256":"%s","generator_inputs_sha256":"%s"},
  "snapshots":{"structural_sha256":"%s","sensitivity_sha256":"%s","field_documents":{"DnsRecord.json":"`+commandDNSDocumentSHA+`"}},
  "scout":{"dns_structural_projection_sha256":"%s","dns_semantic_predecessor_sha256":"%s"},
  "captured_at":"2026-08-04T00:00:00Z"
}`,
				strings.Repeat("f", 64), strings.Repeat("a", 64), strings.Repeat("e", 64),
				strings.Repeat("b", 64), strings.Repeat("c", 64),
				commandCanonicalDigest(t, commandStructuralProjection()), commandCanonicalDigest(t, commandSemanticPredecessor()),
			)
			catalogPath := filepath.Join(directory, "catalog.json")
			receiptPath := filepath.Join(directory, "receipt.json")
			var stderr bytes.Buffer
			exitCode := run([]string{
				"-target-profile", write("target.json", targetJSON),
				"-target-receipt", write("target-receipt.json", targetReceiptJSON),
				"-scenario", write("scenario.json", []byte(`{"id":"dns-record-list-v1","mode":"read_only","resource":"dns_record","method":"GET","path":"/v2/api/site/{site}/static-dns"}`)),
				"-structural", write("structural.json", []byte(commandStructuralProjection())),
				"-semantic-predecessor", write("semantic-predecessor.json", []byte(commandSemanticPredecessor())),
				"-semantic-ids", write("semantic-ids.json", []byte(commandSemanticIDs())),
				"-capture-lock", write("capture.lock.json", []byte(captureLock)),
				"-catalog-output", catalogPath,
				"-receipt-output", receiptPath,
			}, &stderr)
			if exitCode != test.wantExit {
				t.Fatalf("run() exit code = %d, want %d; stderr = %s", exitCode, test.wantExit, stderr.String())
			}
			if test.wantExit != 0 {
				if !strings.Contains(stderr.String(), "instance identity") {
					t.Fatalf("run() stderr = %q, want instance identity mismatch", stderr.String())
				}
				return
			}
			receipt, err := os.ReadFile(receiptPath)
			if err != nil {
				t.Fatal(err)
			}
			catalog, err := os.ReadFile(catalogPath)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(receipt, []byte(provisionedUUID)) || bytes.Contains(catalog, []byte(provisionedUUID)) {
				t.Fatalf("evidence exposed raw controller UUID: catalog=%s receipt=%s", catalog, receipt)
			}
			if !bytes.Contains(receipt, []byte(commandInstanceIdentitySHA256(provisionedUUID))) {
				t.Fatalf("receipt omitted hashed controller identity: %s", receipt)
			}
		})
	}
}

func TestObserveLiveRejectsSysinfoFallbackWithoutControllerUUID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			writer.WriteHeader(http.StatusOK)
		case "/api/auth/login":
			writer.Header().Set("X-Csrf-Token", "test-token")
			writer.WriteHeader(http.StatusOK)
		case "/proxy/network/status":
			_, _ = writer.Write([]byte(`{"meta":{}}`))
		case "/proxy/network/api/s/default/stat/sysinfo":
			_, _ = writer.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"version":"10.4.57"}]}`))
		case "/proxy/network/v2/api/site/default/static-dns":
			_, _ = writer.Write([]byte(`[]`))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("UNIFI_API", server.URL)
	t.Setenv("UNIFI_USERNAME", "admin")
	t.Setenv("UNIFI_PASSWORD", "admin")

	_, err := observeLive(context.Background(), scout.Scenario{
		ID:       "dns-record-list-v1",
		Mode:     "read_only",
		Resource: "dns_record",
		Method:   "GET",
		Path:     "/v2/api/site/{site}/static-dns",
	})
	if err == nil || !strings.Contains(err.Error(), "controller UUID") {
		t.Fatalf("observeLive() error = %v, want missing controller UUID rejection", err)
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
	semanticPredecessor := write("semantic-predecessor.json", commandSemanticPredecessor())
	semanticIDs := write("semantic-ids.json", commandSemanticIDs())
	captureLock := write("capture.lock.json", fmt.Sprintf(`{
  "format_version":1,
  "controller":{"product":"unifi-controller","build":"test-build","network_version":"10.4.57"},
  "source":{"location":"https://downloads.example.invalid/controller.deb","media_type":"application/vnd.debian.binary-package","byte_size":1,"sha256":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},
  "inputs":{"extraction_rules_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","generator_inputs_sha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
  "snapshots":{"structural_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","sensitivity_sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","field_documents":{"DnsRecord.json":"`+commandDNSDocumentSHA+`"}},
	"scout":{"dns_structural_projection_sha256":"%s","dns_semantic_predecessor_sha256":"%s"},
  "captured_at":"2026-08-04T00:00:00Z"
}`, commandCanonicalDigest(t, commandStructuralProjection()), commandCanonicalDigest(t, commandSemanticPredecessor())))
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
		"-semantic-predecessor", semanticPredecessor,
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

func commandTestInput(t *testing.T, observed []byte) scout.Input {
	t.Helper()
	structural := commandStructuralProjection()
	predecessor := commandSemanticPredecessor()
	return scout.Input{
		Target: scout.TargetProfile{
			Name:                  "network-10.4.57-seeded",
			Product:               "unifi-network",
			Version:               "10.4.57",
			Architecture:          "amd64",
			ImageIndexSHA256:      "sha256:" + strings.Repeat("1", 64),
			ImageManifestSHA256:   "sha256:" + strings.Repeat("2", 64),
			ControllerFingerprint: "sha256:" + strings.Repeat("3", 64),
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
			CaptureLockSHA256:          strings.Repeat("d", 64),
			ControllerNetworkVersion:   "10.4.57",
			ExtractionRulesSHA256:      strings.Repeat("a", 64),
			StructuralSHA256:           strings.Repeat("b", 64),
			SensitivitySHA256:          strings.Repeat("c", 64),
			StructuralProjectionSHA256: commandCanonicalDigest(t, structural),
			SemanticPredecessorSHA256:  commandCanonicalDigest(t, predecessor),
		},
		FieldDocumentDigests: scout.FieldDocumentDigests{"DnsRecord.json": commandDNSDocumentSHA},
		StructuralProjection: []byte(structural),
		SemanticPredecessor:  []byte(predecessor),
		SemanticIDs:          []byte(commandSemanticIDs()),
		ObservedResponse:     observed,
	}
}

func makeCommandLiveInput(t *testing.T, input *scout.Input, controllerVersion string) {
	t.Helper()
	receipt := commandTargetReceipt(input.Target, "44444444-4444-4444-8444-444444444444")
	fingerprint, err := scout.ProvisionerReceiptFingerprint(receipt)
	if err != nil {
		t.Fatal(err)
	}
	input.Target.ControllerFingerprint = fingerprint
	input.TargetReceipt = &receipt
	input.ControllerVersion = controllerVersion
	input.ObservedInstanceIdentitySHA256 = receipt.InstanceIdentitySHA256
	input.ExecutionMode = "live"
}

func commandTargetReceipt(target scout.TargetProfile, controllerUUID string) scout.ProvisionerTargetReceipt {
	return scout.ProvisionerTargetReceipt{
		FormatVersion:          1,
		ProfileName:            target.Name,
		Product:                target.Product,
		Version:                target.Version,
		Architecture:           target.Architecture,
		ImageIndexSHA256:       target.ImageIndexSHA256,
		ImageManifestSHA256:    target.ImageManifestSHA256,
		InstanceIdentitySHA256: commandInstanceIdentitySHA256(controllerUUID),
	}
}

func commandInstanceIdentitySHA256(controllerUUID string) string {
	sum := sha256.Sum256([]byte(controllerUUID))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func commandStructuralProjection() string {
	return `{
  "format_version":1,
  "resource":"dns_record",
  "source":{"field_document":"DnsRecord.json","field_document_sha256":"` + commandDNSDocumentSHA + `","sensitivity_sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
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
  "tombstones":[],
  "migrations":[]
}`
}

func commandSemanticPredecessor() string {
	return `{
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
}`
}

func commandCanonicalDigest(t *testing.T, document string) string {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(document), &value); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}
