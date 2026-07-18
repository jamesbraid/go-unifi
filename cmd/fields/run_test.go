package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAcceptsAllUOSSelectors(t *testing.T) {
	for _, args := range [][]string{
		{"-uos-latest", "-generate-spec"}, {"-uos-version", "5.1.21", "-generate-spec"},
		{"-installer", "/tmp/installer", "-generate-spec"},
		{"-installer-url", "https://fw-download.ubnt.com/data/installer", "-generate-spec"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			deps := defaultRunDeps()
			deps.moduleRoot = func() (string, error) { return t.TempDir(), nil }
			deps.resolve = func(context.Context, *http.Client, string, SourceSelector) (InstallerSource, error) {
				return InstallerSource{}, errors.New("sentinel resolve")
			}
			err := runWithDeps(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, deps)
			require.ErrorContains(t, err, "sentinel resolve")
		})
	}
}

func TestRunLocalInstallerEndToEndIsDeterministic(t *testing.T) {
	root := t.TempDir()
	fieldsRoot := filepath.Join(root, "cmd", "fields")
	require.NoError(t, os.MkdirAll(filepath.Join(fieldsRoot, "custom"), 0o755))
	metadata := []byte(`{"min_field_size":1,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{}}`)
	installerBody := syntheticInstaller(t, installerFixtureOptions{innerEntries: []fixtureEntry{
		{name: "api/fields/Setting.json", body: []byte(`{"system":{"enabled":"true|false"}}`)},
		{name: "api/fields/Device.json", body: []byte(`{"name":".*"}`)},
		{name: "sensitive_metadata.json", body: metadata},
	}})
	installer := filepath.Join(root, "installer")
	require.NoError(t, os.WriteFile(installer, installerBody, 0o644))
	digest, err := CanonicalJSONDigest(metadata)
	require.NoError(t, err)
	policyBody, err := json.Marshal(SensitivityPolicy{Version: "1", ApprovedMetadataSHA256: []string{digest}, SecretPaths: []string{}, NonGeneratedSecretPaths: []string{}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(fieldsRoot, "sensitive-policy.json"), policyBody, 0o644))
	deps := defaultRunDeps()
	deps.moduleRoot = func() (string, error) { return root, nil }
	deps.tempRoot = t.TempDir()
	args := []string{"-installer", installer, "-generate-spec"}
	require.NoError(t, runWithDeps(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, deps))
	first, firstDigest, err := HashGeneratedFiles(root, "unifi", "specification.json")
	require.NoError(t, err)
	require.NoError(t, runWithDeps(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, deps))
	second, secondDigest, err := HashGeneratedFiles(root, "unifi", "specification.json")
	require.NoError(t, err)
	assert.Equal(t, firstDigest, secondDigest)
	assert.Equal(t, first, second)
	require.NoError(t, verifyCommittedTree(root, filepath.Join("cmd", "fields", "schema-source.json")))
}

func TestRunPolicyFailureKeepsSnapshotAndPriorOutputs(t *testing.T) {
	root := t.TempDir()
	fieldsRoot := filepath.Join(root, "cmd", "fields")
	require.NoError(t, os.MkdirAll(filepath.Join(fieldsRoot, "custom"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "unifi"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "version.generated.go"), []byte("known-good"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "specification.json"), []byte(`{"known":"good"}`), 0o644))
	metadata := []byte(`{"min_field_size":1,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{}}`)
	body := syntheticInstaller(t, installerFixtureOptions{innerEntries: []fixtureEntry{
		{name: "api/fields/Setting.json", body: []byte(`{"system":{}}`)},
		{name: "api/fields/Device.json", body: []byte(`{"name":".*"}`)},
		{name: "sensitive_metadata.json", body: metadata},
	}})
	installer := filepath.Join(root, "installer")
	require.NoError(t, os.WriteFile(installer, body, 0o644))
	policy, err := json.Marshal(SensitivityPolicy{Version: "1", ApprovedMetadataSHA256: []string{}, SecretPaths: []string{}, NonGeneratedSecretPaths: []string{}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(fieldsRoot, "sensitive-policy.json"), policy, 0o644))
	deps := defaultRunDeps()
	deps.moduleRoot = func() (string, error) { return root, nil }
	deps.tempRoot = t.TempDir()
	err = runWithDeps(context.Background(), []string{"-installer", installer, "-generate-spec"}, &bytes.Buffer{}, &bytes.Buffer{}, deps)
	require.ErrorContains(t, err, "not approved")
	_, err = os.Stat(filepath.Join(fieldsRoot, "v10.4.57", "metadata", "source.json"))
	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(root, "unifi", "version.generated.go"))
	require.NoError(t, err)
	assert.Equal(t, "known-good", string(got))
	got, err = os.ReadFile(filepath.Join(root, "specification.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"known":"good"}`, string(got))
}

func TestRunTerminalModesRejectSelectors(t *testing.T) {
	for _, args := range [][]string{{"-verify-committed", "-latest"}, {"-verify-regeneration", "-installer", "x"}, {"-verify-committed", "-verify-regeneration"}} {
		err := runWithDeps(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, defaultRunDeps())
		require.Error(t, err)
	}
}

func TestRunVerifyCommittedOfflineAndFirstDifference(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "unifi"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "version.generated.go"), []byte("version"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "specification.json"), []byte(`{}`), 0o644))
	files, digest, err := HashGeneratedFiles(root, "unifi", "specification.json")
	require.NoError(t, err)
	source := SchemaSource{GeneratedTreeDigest: digest, GeneratedFiles: files}
	body, err := json.Marshal(source)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd-fields-schema-source.json"), body, 0o644))
	deps := defaultRunDeps()
	deps.moduleRoot = func() (string, error) { return root, nil }
	deps.schemaSourcePath = "cmd-fields-schema-source.json"
	require.NoError(t, runWithDeps(context.Background(), []string{"-verify-committed"}, &bytes.Buffer{}, &bytes.Buffer{}, deps))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "version.generated.go"), []byte("changed"), 0o644))
	err = runWithDeps(context.Background(), []string{"-verify-committed"}, &bytes.Buffer{}, &bytes.Buffer{}, deps)
	require.ErrorContains(t, err, "unifi/version.generated.go")
}

func TestRunRequiresSpecForUOSPublication(t *testing.T) {
	deps := defaultRunDeps()
	deps.moduleRoot = func() (string, error) { return t.TempDir(), nil }
	err := runWithDeps(context.Background(), []string{"-installer", filepath.Join(t.TempDir(), "installer")}, &bytes.Buffer{}, &bytes.Buffer{}, deps)
	require.ErrorContains(t, err, "-generate-spec")
}

func TestPublishGeneratedTreePreservesHandWrittenAndRemovesStale(t *testing.T) {
	root := t.TempDir()
	stage := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "unifi"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "keep.go"), []byte("keep"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "stale.generated.go"), []byte("stale"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(stage, "unifi"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stage, "unifi", "fresh.generated.go"), []byte("fresh"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(stage, "specification.json"), []byte(`{}`), 0o644))
	require.NoError(t, publishGeneratedTree(root, stage, "unifi", "specification.json", ""))
	_, err := os.Stat(filepath.Join(root, "unifi", "stale.generated.go"))
	assert.ErrorIs(t, err, os.ErrNotExist)
	body, err := os.ReadFile(filepath.Join(root, "unifi", "keep.go"))
	require.NoError(t, err)
	assert.Equal(t, "keep", string(body))
	body, err = os.ReadFile(filepath.Join(root, "unifi", "fresh.generated.go"))
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(body))
}

func TestSelectorFromValues(t *testing.T) {
	selector, err := selectorFromValues(sourceValues{uosVersion: "5.1.21"}, nil)
	require.NoError(t, err)
	assert.Equal(t, SourceSelector{Kind: SourceUOSVersion, Value: "5.1.21"}, selector)
	_, err = selectorFromValues(sourceValues{uosLatest: true, installer: "x"}, nil)
	require.Error(t, err)
	_, err = selectorFromValues(sourceValues{}, []string{"1", "2"})
	require.Error(t, err)
	_ = strings.Builder{}
}
