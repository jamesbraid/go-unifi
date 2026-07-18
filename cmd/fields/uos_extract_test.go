package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyntheticInstallerFixtureIsELFPrefixedZIP(t *testing.T) {
	installer := syntheticInstaller(t, installerFixtureOptions{})
	path := filepath.Join(t.TempDir(), "unifi-os-server")
	require.NoError(t, os.WriteFile(path, installer, 0o600))
	result, err := ExtractUOSInstaller(context.Background(), path, t.TempDir(), DefaultArchiveLimits())
	require.NoError(t, err)
	assert.Equal(t, "10.4.57", result.NetworkVersion)
}

func TestExtractUOSInstallerExtractsRequiredArtifactsAndNotices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unifi-os-server")
	require.NoError(t, os.WriteFile(path, syntheticInstaller(t, installerFixtureOptions{}), 0o600))
	result, err := ExtractUOSInstaller(context.Background(), path, t.TempDir(), DefaultArchiveLimits())
	require.NoError(t, err)
	assert.Equal(t, "10.4.57", result.NetworkVersion)
	assert.Contains(t, result.Fields, "api/fields/Setting.json")
	assert.Contains(t, result.Fields, "api/fields/Device.json")
	assert.Contains(t, result.Metadata, "sensitive_metadata.json")
	assert.Contains(t, result.Metadata, "event_defs.json")
	assert.Contains(t, result.Notices, "ace.jar/META-INF/LICENSE")
	assert.Contains(t, result.Notices, "internal-dependencies.jar/META-INF/NOTICE.txt")
	assert.Contains(t, result.MissingOptional, "radio_specification.json")
	for _, artifacts := range []map[string]ExtractedArtifact{result.Fields, result.Metadata, result.Notices} {
		for _, artifact := range artifacts {
			assert.NotEmpty(t, artifact.SHA256)
			assert.Positive(t, artifact.Size)
			assert.FileExists(t, artifact.Path)
		}
	}
}

func TestExtractUOSInstallerRejectsMissingRequiredArtifacts(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts installerFixtureOptions
	}{
		{"version", installerFixtureOptions{aceEntries: []fixtureEntry{{name: "BOOT-INF/classes/product.properties", body: []byte("name=x\n")}, {name: "BOOT-INF/lib/internal-dependencies.jar", body: fixtureZip(t, fixtureEntry{name: "api/fields/Setting.json", body: []byte("{}")}, fixtureEntry{name: "api/fields/Device.json", body: []byte("{}")}, fixtureEntry{name: "sensitive_metadata.json", body: []byte("{}")})}}}},
		{"nested jar", installerFixtureOptions{aceEntries: []fixtureEntry{{name: "BOOT-INF/classes/product.properties", body: []byte("version=10.4.57\n")}}}},
		{"setting", installerFixtureOptions{innerEntries: []fixtureEntry{{name: "api/fields/Device.json", body: []byte("{}")}, {name: "sensitive_metadata.json", body: []byte("{}")}}}},
		{"second field", installerFixtureOptions{innerEntries: []fixtureEntry{{name: "api/fields/Setting.json", body: []byte("{}")}, {name: "sensitive_metadata.json", body: []byte("{}")}}}},
		{"metadata", installerFixtureOptions{innerEntries: []fixtureEntry{{name: "api/fields/Setting.json", body: []byte("{}")}, {name: "api/fields/Device.json", body: []byte("{}")}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "installer")
			require.NoError(t, os.WriteFile(path, syntheticInstaller(t, tc.opts), 0o600))
			_, err := ExtractUOSInstaller(context.Background(), path, t.TempDir(), DefaultArchiveLimits())
			require.Error(t, err)
		})
	}
}

func TestExtractUOSInstallerRejectsArchiveAttacksAndLimits(t *testing.T) {
	duplicateInner := []fixtureEntry{{name: "api/fields/Setting.json", body: []byte("{}")}, {name: "./api/fields/Setting.json", body: []byte("{}")}, {name: "api/fields/Device.json", body: []byte("{}")}, {name: "sensitive_metadata.json", body: []byte("{}")}}
	traversalInner := []fixtureEntry{{name: "../api/fields/Setting.json", body: []byte("{}")}, {name: "api/fields/Device.json", body: []byte("{}")}, {name: "sensitive_metadata.json", body: []byte("{}")}}
	for _, tc := range []struct {
		name   string
		opts   installerFixtureOptions
		mutate func(ArchiveLimits) ArchiveLimits
	}{
		{"duplicate", installerFixtureOptions{innerEntries: duplicateInner}, nil},
		{"traversal", installerFixtureOptions{innerEntries: traversalInner}, nil},
		{"entry limit", installerFixtureOptions{}, func(l ArchiveLimits) ArchiveLimits { l.MaxEntries = 1; return l }},
		{"json limit", installerFixtureOptions{}, func(l ArchiveLimits) ArchiveLimits { l.MaxJSONBytes = 2; return l }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			limits := DefaultArchiveLimits()
			if tc.mutate != nil {
				limits = tc.mutate(limits)
			}
			path := filepath.Join(t.TempDir(), "installer")
			require.NoError(t, os.WriteFile(path, syntheticInstaller(t, tc.opts), 0o600))
			_, err := ExtractUOSInstaller(context.Background(), path, t.TempDir(), limits)
			require.Error(t, err)
		})
	}
}

func TestExtractUOSInstallerHonorsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installer")
	require.NoError(t, os.WriteFile(path, syntheticInstaller(t, installerFixtureOptions{}), 0o600))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ExtractUOSInstaller(ctx, path, t.TempDir(), DefaultArchiveLimits())
	require.ErrorIs(t, err, context.Canceled)
}
