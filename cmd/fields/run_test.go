package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"
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
	require.NoError(t, os.MkdirAll(filepath.Join(root, "unifi"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "device.go"), []byte("package unifi\n"), 0o644))
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
	policyBody, err := json.Marshal(SensitivityPolicy{Version: "1", ApprovedMetadataSHA256: []string{digest}, ApprovedNoticeSHA256: []string{syntheticRunNoticeDigest(t)}, SecretPaths: []string{}, NonGeneratedSecretPaths: []string{}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(fieldsRoot, "sensitive-policy.json"), policyBody, 0o644))
	deps := defaultRunDeps()
	deps.moduleRoot = func() (string, error) { return root, nil }
	deps.tempRoot = t.TempDir()
	args := []string{"-installer", installer, "-generate-spec"}
	require.NoError(t, runWithDeps(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, deps))
	assertNoExtractArtifacts(t, deps.tempRoot)
	first, firstDigest, err := HashGeneratedFiles(root, "unifi", "specification.json")
	require.NoError(t, err)
	require.NoError(t, runWithDeps(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, deps))
	assertNoExtractArtifacts(t, deps.tempRoot)
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
	require.NoError(t, os.WriteFile(filepath.Join(fieldsRoot, "schema-source.json"), []byte("known-source"), 0o644))
	metadata := []byte(`{"min_field_size":1,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{}}`)
	body := syntheticInstaller(t, installerFixtureOptions{innerEntries: []fixtureEntry{
		{name: "api/fields/Setting.json", body: []byte(`{"system":{}}`)},
		{name: "api/fields/Device.json", body: []byte(`{"name":".*"}`)},
		{name: "sensitive_metadata.json", body: metadata},
	}})
	installer := filepath.Join(root, "installer")
	require.NoError(t, os.WriteFile(installer, body, 0o644))
	policy, err := json.Marshal(SensitivityPolicy{Version: "1", ApprovedMetadataSHA256: []string{}, ApprovedNoticeSHA256: []string{syntheticRunNoticeDigest(t)}, SecretPaths: []string{}, NonGeneratedSecretPaths: []string{}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(fieldsRoot, "sensitive-policy.json"), policy, 0o644))
	deps := defaultRunDeps()
	deps.moduleRoot = func() (string, error) { return root, nil }
	deps.tempRoot = t.TempDir()
	err = runWithDeps(context.Background(), []string{"-installer", installer, "-generate-spec"}, &bytes.Buffer{}, &bytes.Buffer{}, deps)
	require.ErrorContains(t, err, "not approved")
	assertNoExtractArtifacts(t, deps.tempRoot)
	_, err = os.Stat(filepath.Join(fieldsRoot, "v10.4.57", "metadata", "source.json"))
	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(root, "unifi", "version.generated.go"))
	require.NoError(t, err)
	assert.Equal(t, "known-good", string(got))
	got, err = os.ReadFile(filepath.Join(root, "specification.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"known":"good"}`, string(got))
	got, err = os.ReadFile(filepath.Join(fieldsRoot, "schema-source.json"))
	require.NoError(t, err)
	assert.Equal(t, "known-source", string(got))
}

func TestRunBoundaryFailuresPreserveAllCommittedOutputs(t *testing.T) {
	for _, boundary := range []string{"materialize", "extract", "snapshot", "scan", "render"} {
		t.Run(boundary, func(t *testing.T) {
			root, installer, deps := newRunFailureFixture(t)
			injected := errors.New("injected " + boundary)
			switch boundary {
			case "materialize":
				deps.materialize = func(context.Context, *http.Client, InstallerSource, string) (*MaterializedInstaller, error) {
					return nil, injected
				}
			case "extract":
				deps.extractUOS = func(context.Context, string, string, ArchiveLimits) (*ExtractedDefinitions, error) {
					return nil, injected
				}
			case "snapshot":
				deps.buildSnapshot = func(context.Context, SnapshotOptions) (*LocalManifest, error) { return nil, injected }
			case "scan":
				deps.scan = func(string) error { return injected }
			case "render":
				deps.render = func(string, string, *version.Version, func([]*ResourceInfo) error) error { return injected }
			}
			err := runWithDeps(context.Background(), []string{"-installer", installer, "-generate-spec"}, &bytes.Buffer{}, &bytes.Buffer{}, deps)
			require.ErrorContains(t, err, injected.Error())
			assertRunOutputsOld(t, root)
			if boundary == "snapshot" || boundary == "scan" || boundary == "render" {
				assertNoExtractArtifacts(t, deps.tempRoot)
			}
			if boundary == "scan" || boundary == "render" {
				_, statErr := os.Stat(filepath.Join(root, "cmd", "fields", "v10.4.57", "metadata", "source.json"))
				require.NoError(t, statErr, "published snapshot remains available after %s failure", boundary)
			}
		})
	}
}

func TestRunUnapprovedNoticeDigestKeepsSnapshotAndPriorOutputs(t *testing.T) {
	root, installer, deps := newRunFailureFixture(t)
	policyPath := filepath.Join(root, "cmd", "fields", "sensitive-policy.json")
	policy, err := LoadSensitivityPolicy(policyPath)
	require.NoError(t, err)
	policy.ApprovedNoticeSHA256 = []string{}
	body, err := json.Marshal(policy)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(policyPath, body, 0o644))

	err = runWithDeps(context.Background(), []string{"-installer", installer, "-generate-spec"}, &bytes.Buffer{}, &bytes.Buffer{}, deps)
	require.ErrorContains(t, err, "notice digest")
	require.ErrorContains(t, err, "is not approved")
	assertRunOutputsOld(t, root)
	assertNoExtractArtifacts(t, deps.tempRoot)
	_, statErr := os.Stat(filepath.Join(root, "cmd", "fields", "v10.4.57", "metadata", "source.json"))
	require.NoError(t, statErr, "complete snapshot remains available for notice review")
}

func TestRunCleansExtractedDefinitionsBeforeSnapshotScan(t *testing.T) {
	root, installer, deps := newRunFailureFixture(t)
	deps.scan = func(string) error {
		assertNoExtractArtifacts(t, deps.tempRoot)
		return errors.New("stop after cleanup assertion")
	}
	err := runWithDeps(context.Background(), []string{"-installer", installer, "-generate-spec"}, &bytes.Buffer{}, &bytes.Buffer{}, deps)
	require.ErrorContains(t, err, "stop after cleanup assertion")
	assertRunOutputsOld(t, root)
}

func TestVerificationModesRejectUnapprovedSchemaSourceNoticeDigest(t *testing.T) {
	root, installer, deps := newRunFailureFixture(t)
	require.NoError(t, runWithDeps(context.Background(), []string{"-installer", installer, "-generate-spec"}, &bytes.Buffer{}, &bytes.Buffer{}, deps))
	before := captureRunOutputs(t, root)
	policyPath := filepath.Join(root, "cmd", "fields", "sensitive-policy.json")
	policy, err := LoadSensitivityPolicy(policyPath)
	require.NoError(t, err)
	policy.ApprovedNoticeSHA256 = []string{}
	body, err := json.Marshal(policy)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(policyPath, body, 0o644))

	err = verifyCommittedTree(root, filepath.Join("cmd", "fields", "schema-source.json"))
	require.ErrorContains(t, err, "notice digest")
	err = verifyRegeneratedTree(context.Background(), root, deps, &bytes.Buffer{})
	require.ErrorContains(t, err, "notice digest")
	assert.Equal(t, before, captureRunOutputs(t, root))
}

func TestVerifyRegenerationRejectsDifferentActualSnapshotNoticeTree(t *testing.T) {
	root, installer, deps := newRunFailureFixture(t)
	require.NoError(t, runWithDeps(context.Background(), []string{"-installer", installer, "-generate-spec"}, &bytes.Buffer{}, &bytes.Buffer{}, deps))
	before := captureRunOutputs(t, root)
	snapshot := filepath.Join(root, "cmd", "fields", "v10.4.57")
	noticePath := filepath.Join(snapshot, "metadata", "notices", "ace.jar", "META-INF", "LICENSE")
	require.NoError(t, os.WriteFile(noticePath, []byte("changed reviewed terms"), 0o644))
	newDigest, err := CanonicalTreeDigest(map[string][]byte{"ace.jar/META-INF/LICENSE": []byte("changed reviewed terms")})
	require.NoError(t, err)
	manifestPath := filepath.Join(snapshot, "metadata", "source.json")
	body, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	var manifest LocalManifest
	require.NoError(t, json.Unmarshal(body, &manifest))
	manifest.NoticeDigest = newDigest
	body, err = json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, body, 0o644))

	err = verifyRegeneratedTree(context.Background(), root, deps, &bytes.Buffer{})
	require.ErrorContains(t, err, newDigest)
	require.ErrorContains(t, err, "not approved")
	assert.Equal(t, before, captureRunOutputs(t, root))
}

func TestVerifyRegeneratedTreeNeverOverwritesAndReportsFirstPath(t *testing.T) {
	root, installer, deps := newRunFailureFixture(t)
	require.NoError(t, runWithDeps(context.Background(), []string{"-installer", installer, "-generate-spec"}, &bytes.Buffer{}, &bytes.Buffer{}, deps))
	before := captureRunOutputs(t, root)
	deps.render = func(_ string, stage string, _ *version.Version, _ func([]*ResourceInfo) error) error {
		require.NoError(t, os.MkdirAll(filepath.Join(stage, "unifi"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(stage, "unifi", "aaa.generated.go"), []byte("different"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(stage, "specification.json"), []byte(`{}`), 0o644))
		return nil
	}
	err := verifyRegeneratedTree(context.Background(), root, deps, &bytes.Buffer{})
	require.ErrorContains(t, err, "regenerated file differs: specification.json")
	assert.Equal(t, before, captureRunOutputs(t, root))
}

func newRunFailureFixture(t *testing.T) (string, string, runDeps) {
	t.Helper()
	root := t.TempDir()
	fieldsRoot := filepath.Join(root, "cmd", "fields")
	require.NoError(t, os.MkdirAll(filepath.Join(fieldsRoot, "custom"), 0o755))
	metadata := []byte(`{"min_field_size":1,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{}}`)
	body := syntheticInstaller(t, installerFixtureOptions{innerEntries: []fixtureEntry{{name: "api/fields/Setting.json", body: []byte(`{"system":{}}`)}, {name: "api/fields/Device.json", body: []byte(`{"name":".*"}`)}, {name: "sensitive_metadata.json", body: metadata}}})
	installer := filepath.Join(root, "installer")
	require.NoError(t, os.WriteFile(installer, body, 0o644))
	digest, err := CanonicalJSONDigest(metadata)
	require.NoError(t, err)
	policy, err := json.Marshal(SensitivityPolicy{Version: "1", ApprovedMetadataSHA256: []string{digest}, ApprovedNoticeSHA256: []string{syntheticRunNoticeDigest(t)}, SecretPaths: []string{}, NonGeneratedSecretPaths: []string{}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(fieldsRoot, "sensitive-policy.json"), policy, 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "unifi"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "device.go"), []byte("package unifi\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "old.generated.go"), []byte("old-go"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "specification.json"), []byte("old-spec"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(fieldsRoot, "schema-source.json"), []byte("old-source"), 0o644))
	deps := defaultRunDeps()
	deps.moduleRoot = func() (string, error) { return root, nil }
	deps.tempRoot = t.TempDir()
	return root, installer, deps
}

func syntheticRunNoticeDigest(t *testing.T) string {
	t.Helper()
	digest, err := CanonicalTreeDigest(map[string][]byte{"ace.jar/META-INF/LICENSE": []byte("ace license")})
	require.NoError(t, err)
	return digest
}
func captureRunOutputs(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, path := range []string{"specification.json", "cmd/fields/schema-source.json"} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, err)
		result[path] = string(body)
	}
	require.NoError(t, filepath.WalkDir(filepath.Join(root, "unifi"), func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".generated.go") {
			return nil
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(strings.TrimPrefix(name, root+string(filepath.Separator)))] = string(body)
		return nil
	}))
	return result
}
func assertRunOutputsOld(t *testing.T, root string) {
	t.Helper()
	assert.Equal(t, map[string]string{"unifi/old.generated.go": "old-go", "specification.json": "old-spec", "cmd/fields/schema-source.json": "old-source"}, captureRunOutputs(t, root))
}

func assertNoExtractArtifacts(t *testing.T, tempRoot string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(tempRoot, "uos-extract-*"))
	require.NoError(t, err)
	assert.Empty(t, matches, "temporary extracted definitions must be cleaned")
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
	noticeDigest := syntheticRunNoticeDigest(t)
	source := SchemaSource{NoticeDigest: noticeDigest, GeneratedTreeDigest: digest, GeneratedFiles: files}
	body, err := json.Marshal(source)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd-fields-schema-source.json"), body, 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd", "fields"), 0o755))
	policyBody, err := json.Marshal(SensitivityPolicy{Version: "1", ApprovedMetadataSHA256: []string{}, ApprovedNoticeSHA256: []string{noticeDigest}, SecretPaths: []string{}, NonGeneratedSecretPaths: []string{}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", "fields", "sensitive-policy.json"), policyBody, 0o644))
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
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "keep.go"), []byte("keep"), 0o600))
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
	info, err := os.Stat(filepath.Join(root, "unifi", "keep.go"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	body, err = os.ReadFile(filepath.Join(root, "unifi", "fresh.generated.go"))
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(body))
}

func TestRegenerateSeedsCanonicalHandWrittenImplementations(t *testing.T) {
	root, snapshot := generationStageFixture(t)
	implementation := []byte("package unifi\n// canonical BGP implementation\n")
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "bgp_config.go"), implementation, 0o644))
	published := false
	deps := defaultRunDeps()
	deps.render = func(_ string, stage string, _ *version.Version, _ func([]*ResourceInfo) error) error {
		body, err := os.ReadFile(filepath.Join(stage, "unifi", "bgp_config.go"))
		require.NoError(t, err, "canonical hand-written implementation must be visible while rendering")
		assert.Equal(t, implementation, body)
		writeGenerationStageOutputs(t, stage)
		return nil
	}
	deps.publish = func(string, string, string, string, string) error { published = true; return nil }
	err := regenerateAndPublish(context.Background(), root, snapshot, InstallerSource{}, LocalManifest{NetworkVersion: "10.4.57"}, SensitivityPolicy{}, &bytes.Buffer{}, deps)
	require.NoError(t, err)
	assert.True(t, published)
}

func TestRegenerateRejectsNewHandWrittenImplementationScaffold(t *testing.T) {
	root, snapshot := generationStageFixture(t)
	knownGood := filepath.Join(root, "unifi", "known.generated.go")
	require.NoError(t, os.WriteFile(knownGood, []byte("known-good"), 0o644))
	published := false
	deps := defaultRunDeps()
	deps.render = func(_ string, stage string, _ *version.Version, _ func([]*ResourceInfo) error) error {
		writeGenerationStageOutputs(t, stage)
		require.NoError(t, os.WriteFile(filepath.Join(stage, "unifi", "new_resource.go"), []byte("package unifi\n"), 0o644))
		return nil
	}
	deps.publish = func(string, string, string, string, string) error { published = true; return nil }
	err := regenerateAndPublish(context.Background(), root, snapshot, InstallerSource{}, LocalManifest{NetworkVersion: "10.4.57"}, SensitivityPolicy{}, &bytes.Buffer{}, deps)
	require.ErrorContains(t, err, "new_resource.go")
	require.ErrorContains(t, err, "canonical hand-written implementation")
	assert.False(t, published)
	body, readErr := os.ReadFile(knownGood)
	require.NoError(t, readErr)
	assert.Equal(t, "known-good", string(body))
}

func TestValidateStagedHandWrittenFilesRejectsChangedAndRemoved(t *testing.T) {
	for _, mode := range []string{"changed", "removed"} {
		t.Run(mode, func(t *testing.T) {
			canonical := filepath.Join(t.TempDir(), "unifi")
			staged := filepath.Join(t.TempDir(), "unifi")
			require.NoError(t, os.MkdirAll(filepath.Join(canonical, "nested"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(canonical, "nested", "keep.go"), []byte("canonical"), 0o644))
			require.NoError(t, copyTreeExcludingOwned(canonical, staged))
			if mode == "changed" {
				require.NoError(t, os.WriteFile(filepath.Join(staged, "nested", "keep.go"), []byte("changed"), 0o644))
			} else {
				require.NoError(t, os.Remove(filepath.Join(staged, "nested", "keep.go")))
			}
			err := validateStagedHandWrittenFiles(canonical, staged)
			require.ErrorContains(t, err, "unifi/nested/keep.go")
		})
	}
}

func TestVerifyRegeneratedTreeSeedsAndRejectsUnexpectedImplementation(t *testing.T) {
	root, installer, deps := newRunFailureFixture(t)
	require.NoError(t, runWithDeps(context.Background(), []string{"-installer", installer, "-generate-spec"}, &bytes.Buffer{}, &bytes.Buffer{}, deps))
	before := captureRunOutputs(t, root)
	deps.render = func(_ string, stage string, _ *version.Version, _ func([]*ResourceInfo) error) error {
		require.FileExists(t, filepath.Join(stage, "unifi", "device.go"), "verification render must see canonical implementation")
		writeGenerationStageOutputs(t, stage)
		require.NoError(t, os.WriteFile(filepath.Join(stage, "unifi", "unexpected.go"), []byte("package unifi\n"), 0o644))
		return nil
	}
	err := verifyRegeneratedTree(context.Background(), root, deps, &bytes.Buffer{})
	require.ErrorContains(t, err, "unifi/unexpected.go")
	require.ErrorContains(t, err, "canonical hand-written implementation")
	assert.Equal(t, before, captureRunOutputs(t, root))
}

func generationStageFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "unifi"), 0o755))
	snapshot := filepath.Join(root, "snapshot")
	require.NoError(t, os.MkdirAll(filepath.Join(snapshot, "metadata", "raw-fields"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(snapshot, "metadata", "sensitive_metadata.json"), []byte(`{"min_field_size":1,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{}}`), 0o644))
	return root, snapshot
}

func writeGenerationStageOutputs(t *testing.T, stage string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(stage, "unifi"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stage, "unifi", "resource.generated.go"), []byte("package unifi\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(stage, "specification.json"), []byte(`{}`), 0o644))
}

func TestPublishGeneratedTreePreflightFailureAndFixedPathAreSafe(t *testing.T) {
	root, stage := publicationFixture(t)
	fixed := filepath.Join(root, ".unifi.generated-backup")
	require.NoError(t, os.WriteFile(fixed, []byte("do-not-touch"), 0o600))
	require.NoError(t, os.Remove(filepath.Join(stage, "cmd", "fields", "schema-source.json")))
	require.Error(t, publishGeneratedTree(root, stage, "unifi", "specification.json", filepath.Join("cmd", "fields", "schema-source.json")))
	assertPublicationFixtureOld(t, root)
	body, err := os.ReadFile(fixed)
	require.NoError(t, err)
	assert.Equal(t, "do-not-touch", string(body))
}

func TestPublishGeneratedTreeRollsBackEverySwapFailure(t *testing.T) {
	for failRename := 1; failRename <= 6; failRename++ {
		t.Run(fmt.Sprintf("rename_%d", failRename), func(t *testing.T) {
			root, stage := publicationFixture(t)
			ops := defaultPublishFileOps()
			calls := 0
			realRename := ops.rename
			ops.rename = func(old, new string) error {
				calls++
				if calls == failRename {
					return errors.New("injected rename")
				}
				return realRename(old, new)
			}
			err := publishGeneratedTreeWithOps(root, stage, "unifi", "specification.json", filepath.Join("cmd", "fields", "schema-source.json"), ops)
			require.ErrorContains(t, err, "injected rename")
			assertPublicationFixtureOld(t, root)
		})
	}
}

func TestPublishGeneratedTreeCleanupFailureDoesNotTurnCommitIntoFailure(t *testing.T) {
	root, stage := publicationFixture(t)
	ops := defaultPublishFileOps()
	realRemoveAll := ops.removeAll
	backupCalls := 0
	ops.removeAll = func(name string) error {
		if strings.Contains(filepath.Base(name), "backup") {
			backupCalls++
			if backupCalls > 1 {
				return errors.New("injected cleanup")
			}
		}
		return realRemoveAll(name)
	}
	require.NoError(t, publishGeneratedTreeWithOps(root, stage, "unifi", "specification.json", filepath.Join("cmd", "fields", "schema-source.json"), ops))
	body, err := os.ReadFile(filepath.Join(root, "unifi", "fresh.generated.go"))
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(body))
}

func publicationFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	stage := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "unifi"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd", "fields"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "old.generated.go"), []byte("old-go"), 0o640))
	require.NoError(t, os.WriteFile(filepath.Join(root, "specification.json"), []byte("old-spec"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", "fields", "schema-source.json"), []byte("old-source"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(stage, "unifi"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(stage, "cmd", "fields"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stage, "unifi", "fresh.generated.go"), []byte("fresh"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(stage, "specification.json"), []byte("new-spec"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(stage, "cmd", "fields", "schema-source.json"), []byte("new-source"), 0o644))
	return root, stage
}
func assertPublicationFixtureOld(t *testing.T, root string) {
	t.Helper()
	for path, want := range map[string]string{"unifi/old.generated.go": "old-go", "specification.json": "old-spec", "cmd/fields/schema-source.json": "old-source"} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, err, path)
		assert.Equal(t, want, string(body), path)
	}
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
