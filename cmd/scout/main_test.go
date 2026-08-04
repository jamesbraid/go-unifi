package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
	specification := write("specification.json", `{
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
	captureLock := write("capture.lock.json", `{"format_version":1}`)
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
		"-specification", specification,
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
}

func TestRunRequiresDeclaredInputs(t *testing.T) {
	var stderr bytes.Buffer
	if exitCode := run(nil, &stderr); exitCode == 0 {
		t.Fatal("run() succeeded without inputs")
	}
}
