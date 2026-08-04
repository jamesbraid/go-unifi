package scout

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildDNSCatalogIsCanonicalAndValueFree(t *testing.T) {
	first := testInput(t, `[
  {"_id":"record-a","enabled":true,"key":"alpha.example.invalid","port":53,"priority":10,"record_type":"A","ttl":300,"value":"192.0.2.10","weight":1},
  {"_id":"record-b","enabled":false,"key":"beta.example.invalid","record_type":"TXT","ttl":60,"value":"private fixture text"}
]`)
	second := testInput(t, `[
  {"_id":"different-b","enabled":true,"key":"changed.example.invalid","record_type":"TXT","ttl":120,"value":"different text"},
  {"_id":"different-a","enabled":false,"key":"other.example.invalid","port":443,"priority":20,"record_type":"AAAA","ttl":600,"value":"2001:db8::10","weight":2}
]`)

	firstResult, err := BuildDNSCatalog(first)
	if err != nil {
		t.Fatal(err)
	}
	secondResult, err := BuildDNSCatalog(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstResult.Catalog, secondResult.Catalog) {
		t.Fatalf("canonical catalogs differ:\nfirst: %s\nsecond: %s", firstResult.Catalog, secondResult.Catalog)
	}
	for _, secret := range []string{"record-a", "alpha.example.invalid", "192.0.2.10", "private fixture text"} {
		if bytes.Contains(firstResult.Catalog, []byte(secret)) || bytes.Contains(firstResult.Receipt, []byte(secret)) {
			t.Fatalf("output retained observed value %q", secret)
		}
	}

	var catalog struct {
		StructuralRecords []struct {
			ID string `json:"id"`
		} `json:"structural_records"`
		ObservedRecords []struct {
			ID string `json:"id"`
		} `json:"observed_records"`
		Admission struct {
			State string `json:"state"`
		} `json:"admission"`
	}
	if err := json.Unmarshal(firstResult.Catalog, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.StructuralRecords) != 8 || len(catalog.ObservedRecords) != 8 {
		t.Fatalf("record counts = %d structural, %d observed; want 8 and 8", len(catalog.StructuralRecords), len(catalog.ObservedRecords))
	}
	if catalog.StructuralRecords[0].ID != "unifi.network.dns_record.field.enabled" {
		t.Fatalf("first catalog ID = %q", catalog.StructuralRecords[0].ID)
	}
	if catalog.Admission.State != "candidate" {
		t.Fatalf("admission state = %q, want candidate", catalog.Admission.State)
	}
}

func TestBuildDNSCatalogKeepsConflictsSeparate(t *testing.T) {
	input := testInput(t, `[{"enabled":true,"key":"fixture.example.invalid","record_type":"A","ttl":300,"value":"192.0.2.20","new_field":true}]`)
	result, err := BuildDNSCatalog(input)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Conflicts []struct {
			Kind  string `json:"kind"`
			Field string `json:"field"`
		} `json:"conflicts"`
		Admission struct {
			State string `json:"state"`
		} `json:"admission"`
	}
	if err := json.Unmarshal(result.Catalog, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Conflicts) != 1 || catalog.Conflicts[0].Kind != "unknown_observed_field" || catalog.Conflicts[0].Field != "new_field" {
		t.Fatalf("conflicts = %#v", catalog.Conflicts)
	}
	if catalog.Admission.State != "blocked" {
		t.Fatalf("admission state = %q, want blocked", catalog.Admission.State)
	}
}

func TestBuildDNSCatalogFailsClosed(t *testing.T) {
	tests := map[string]struct {
		mutate func(*Input)
		want   string
	}{
		"malformed observation": {
			mutate: func(input *Input) { input.ObservedResponse = []byte(`{"not":"an array"}`) },
			want:   "observation must be an array",
		},
		"unsafe field": {
			mutate: func(input *Input) {
				input.ObservedResponse = []byte(`[{"key":"fixture","x_password":"do-not-retain"}]`)
			},
			want: "unsafe observed field",
		},
		"mutable target": {
			mutate: func(input *Input) { input.Target.ImageManifestSHA256 = "" },
			want:   "immutable target digest",
		},
		"undeclared workflow": {
			mutate: func(input *Input) { input.Scenario.Mode = "production" },
			want:   "unsupported scenario mode",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			input := testInput(t, `[]`)
			test.mutate(&input)
			_, err := BuildDNSCatalog(input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("BuildDNSCatalog() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func testInput(t *testing.T, observation string) Input {
	t.Helper()
	specification := []byte(`{
  "version":"0.1",
  "provider":{"name":"unifi"},
  "resources":[{"name":"dns_record","schema":{"attributes":[
    {"name":"enabled","bool":{"computed_optional_required":"optional"}},
    {"name":"key","string":{"computed_optional_required":"computed_optional"}},
    {"name":"port","int64":{"computed_optional_required":"computed_optional"}},
    {"name":"priority","int64":{"computed_optional_required":"computed_optional"}},
    {"name":"record_type","string":{"computed_optional_required":"computed_optional"}},
    {"name":"ttl","int64":{"computed_optional_required":"computed_optional"}},
    {"name":"value","string":{"computed_optional_required":"computed_optional"}},
    {"name":"weight","int64":{"computed_optional_required":"computed_optional"}}
  ]}}]
}`)
	return Input{
		Target: TargetProfile{
			Name:                  "network-10.4.57-seeded",
			Product:               "unifi-network",
			Version:               "10.4.57",
			Architecture:          "amd64",
			ImageIndexSHA256:      "sha256:index",
			ImageManifestSHA256:   "sha256:manifest",
			ControllerFingerprint: "network-10.4.57",
		},
		Scenario: Scenario{
			ID:       "dns-record-list-v1",
			Mode:     "read_only",
			Resource: "dns_record",
			Method:   "GET",
			Path:     "/v2/api/site/{site}/static-dns",
		},
		CaptureLockSHA256: "136421431577ad8cce79aee0a25619577d78f41c1d7c70b914b0317bfbbf08ae",
		Specification:     specification,
		ObservedResponse:  []byte(observation),
	}
}
