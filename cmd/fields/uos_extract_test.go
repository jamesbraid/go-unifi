package main

import (
	"context"
	"encoding/binary"
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
	t.Cleanup(func() { require.NoError(t, result.Close()) })
	assert.Equal(t, "10.4.57", result.NetworkVersion)
}

func TestExtractUOSInstallerExtractsRequiredArtifactsAndNotices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unifi-os-server")
	require.NoError(t, os.WriteFile(path, syntheticInstaller(t, installerFixtureOptions{}), 0o600))
	tempRoot := t.TempDir()
	result, err := ExtractUOSInstaller(context.Background(), path, tempRoot, DefaultArchiveLimits())
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
	require.NoError(t, result.Close())
	require.NoError(t, result.Close(), "cleanup is idempotent")
	assert.FileExists(t, path, "cleanup must not remove the caller-owned installer")
	matches, err := filepath.Glob(filepath.Join(tempRoot, "uos-extract-*"))
	require.NoError(t, err)
	assert.Empty(t, matches)
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

func TestExtractUOSInstallerInventoriesNoticeBasenameSuffixes(t *testing.T) {
	internal := fixtureZip(t,
		fixtureEntry{name: "api/fields/Setting.json", body: []byte("{}")},
		fixtureEntry{name: "api/fields/Device.json", body: []byte("{}")},
		fixtureEntry{name: "sensitive_metadata.json", body: []byte("{}")},
		fixtureEntry{name: "NOTICE-third-party", body: []byte("inner root notice")},
		fixtureEntry{name: "META-INF/LICENSE_BSD", body: []byte("inner license")},
	)
	installer := syntheticInstaller(t, installerFixtureOptions{aceEntries: []fixtureEntry{
		{name: productPropsPath, body: []byte("version=10.4.57\n")},
		{name: internalJarPath, body: internal},
		{name: "LICENSE-APACHE", body: []byte("ace root license")},
		{name: "META-INF/notice_third-party", body: []byte("ace notice")},
	}})
	installerPath := filepath.Join(t.TempDir(), "installer")
	require.NoError(t, os.WriteFile(installerPath, installer, 0o600))

	result, err := ExtractUOSInstaller(context.Background(), installerPath, t.TempDir(), DefaultArchiveLimits())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, result.Close()) })
	assert.Contains(t, result.Notices, "ace.jar/LICENSE-APACHE")
	assert.Contains(t, result.Notices, "ace.jar/META-INF/notice_third-party")
	assert.Contains(t, result.Notices, "internal-dependencies.jar/NOTICE-third-party")
	assert.Contains(t, result.Notices, "internal-dependencies.jar/META-INF/LICENSE_BSD")
}

func TestExtractUOSInstallerInventoriesDirectDependencyNotices(t *testing.T) {
	internal := fixtureZip(t,
		fixtureEntry{name: "api/fields/Setting.json", body: []byte("{}")},
		fixtureEntry{name: "api/fields/Device.json", body: []byte("{}")},
		fixtureEntry{name: "sensitive_metadata.json", body: []byte("{}")},
		fixtureEntry{name: "META-INF/NOTICE.txt", body: []byte("internal notice")},
	)
	dependency := fixtureZip(t,
		fixtureEntry{name: "COPYING", body: []byte("copying terms")},
		fixtureEntry{name: "META-INF/THIRD-PARTY-NOTICES.txt", body: []byte("third party terms")},
		fixtureEntry{name: "README.txt", body: []byte("not a notice")},
	)
	installer := syntheticInstaller(t, installerFixtureOptions{aceEntries: []fixtureEntry{
		{name: productPropsPath, body: []byte("version=10.4.57\n")},
		{name: internalJarPath, body: internal},
		{name: "BOOT-INF/lib/example.jar", body: dependency},
	}})
	installerPath := filepath.Join(t.TempDir(), "installer")
	require.NoError(t, os.WriteFile(installerPath, installer, 0o600))

	result, err := ExtractUOSInstaller(context.Background(), installerPath, t.TempDir(), DefaultArchiveLimits())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, result.Close()) })
	assert.Contains(t, result.Notices, "internal-dependencies.jar/META-INF/NOTICE.txt")
	assert.Contains(t, result.Notices, "ace.jar/BOOT-INF/lib/internal-dependencies.jar/META-INF/NOTICE.txt")
	assert.Contains(t, result.Notices, "ace.jar/BOOT-INF/lib/example.jar/COPYING")
	assert.Contains(t, result.Notices, "ace.jar/BOOT-INF/lib/example.jar/META-INF/THIRD-PARTY-NOTICES.txt")
	assert.NotContains(t, result.Notices, "ace.jar/BOOT-INF/lib/example.jar/README.txt")
}

func TestExtractUOSInstallerRejectsInvalidDependencyJarsAndNoticeLimits(t *testing.T) {
	validInternal := fixtureZip(t,
		fixtureEntry{name: "api/fields/Setting.json", body: []byte("{}")},
		fixtureEntry{name: "api/fields/Device.json", body: []byte("{}")},
		fixtureEntry{name: "sensitive_metadata.json", body: []byte("{}")},
	)
	for _, tc := range []struct {
		name       string
		dependency []byte
		mutate     func(*ArchiveLimits)
	}{
		{name: "corrupt", dependency: []byte("not a ZIP")},
		{name: "CRC", dependency: corruptFixtureZipEntry(t, fixtureZip(t, fixtureEntry{name: "LICENSE", body: []byte("terms that must verify")}))},
		{name: "non-notice CRC", dependency: corruptFixtureZipEntry(t, fixtureZip(t, fixtureEntry{name: "README.txt", body: []byte("body that must verify")}))},
		{name: "symlink", dependency: fixtureZip(t, fixtureEntry{name: "LICENSE", body: []byte("target"), typeflag: '2'})},
		{name: "traversal", dependency: fixtureZip(t, fixtureEntry{name: "../LICENSE", body: []byte("terms")})},
		{name: "duplicate", dependency: fixtureZip(t, fixtureEntry{name: "LICENSE", body: []byte("one")}, fixtureEntry{name: "./LICENSE", body: []byte("two")})},
		{name: "case fold collision", dependency: fixtureZip(t, fixtureEntry{name: "LICENSE", body: []byte("one")}, fixtureEntry{name: "license", body: []byte("two")})},
		{name: "archive limit", dependency: fixtureZip(t, fixtureEntry{name: "LICENSE", body: []byte("terms")}), mutate: func(l *ArchiveLimits) { l.MaxNestedArchives = 1 }},
		{name: "notice entry limit", dependency: fixtureZip(t, fixtureEntry{name: "LICENSE", body: []byte("one")}, fixtureEntry{name: "NOTICE", body: []byte("two")}), mutate: func(l *ArchiveLimits) { l.MaxNoticeEntries = 1 }},
		{name: "notice byte limit", dependency: fixtureZip(t, fixtureEntry{name: "LICENSE", body: []byte("terms")}), mutate: func(l *ArchiveLimits) { l.MaxNoticeBytes = 4 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installer := syntheticInstaller(t, installerFixtureOptions{aceEntries: []fixtureEntry{
				{name: productPropsPath, body: []byte("version=10.4.57\n")},
				{name: internalJarPath, body: validInternal},
				{name: "BOOT-INF/lib/example.jar", body: tc.dependency},
			}})
			path := filepath.Join(t.TempDir(), "installer")
			require.NoError(t, os.WriteFile(path, installer, 0o600))
			limits := DefaultArchiveLimits()
			if tc.mutate != nil {
				tc.mutate(&limits)
			}
			_, err := ExtractUOSInstaller(context.Background(), path, t.TempDir(), limits)
			require.Error(t, err)
		})
	}
}

func corruptFixtureZipEntry(t *testing.T, archive []byte) []byte {
	t.Helper()
	corrupt := append([]byte(nil), archive...)
	require.GreaterOrEqual(t, len(corrupt), 30)
	nameLength := int(binary.LittleEndian.Uint16(corrupt[26:28]))
	extraLength := int(binary.LittleEndian.Uint16(corrupt[28:30]))
	dataOffset := 30 + nameLength + extraLength
	require.Less(t, dataOffset, len(corrupt))
	corrupt[dataOffset] ^= 0xff
	return corrupt
}

func TestExtractUOSInstallerDependencyNoticeInventoryIsOrderIndependent(t *testing.T) {
	internal := fixtureZip(t,
		fixtureEntry{name: "api/fields/Setting.json", body: []byte("{}")},
		fixtureEntry{name: "api/fields/Device.json", body: []byte("{}")},
		fixtureEntry{name: "sensitive_metadata.json", body: []byte("{}")},
	)
	first := fixtureEntry{name: "BOOT-INF/lib/a.jar", body: fixtureZip(t, fixtureEntry{name: "LICENSE.txt", body: []byte("a")})}
	second := fixtureEntry{name: "BOOT-INF/lib/b.jar", body: fixtureZip(t, fixtureEntry{name: "NOTICE.md", body: []byte("b")})}
	extract := func(entries []fixtureEntry) map[string]string {
		aceEntries := append([]fixtureEntry{{name: productPropsPath, body: []byte("version=10.4.57\n")}, {name: internalJarPath, body: internal}}, entries...)
		path := filepath.Join(t.TempDir(), "installer")
		require.NoError(t, os.WriteFile(path, syntheticInstaller(t, installerFixtureOptions{aceEntries: aceEntries}), 0o600))
		result, err := ExtractUOSInstaller(context.Background(), path, t.TempDir(), DefaultArchiveLimits())
		require.NoError(t, err)
		defer func() { require.NoError(t, result.Close()) }()
		hashes := make(map[string]string, len(result.Notices))
		for name, artifact := range result.Notices {
			hashes[name] = artifact.SHA256
		}
		return hashes
	}
	assert.Equal(t, extract([]fixtureEntry{first, second}), extract([]fixtureEntry{second, first}))
}

func TestDependencyNoticePathFamiliesExcludeBinaryLookalikes(t *testing.T) {
	for _, name := range []string{
		"LICENSE", "META-INF/LICENSE.txt", "META-INF/NOTICE.md",
		"META-INF/LICENSE-2.0.txt", "META-INF/COPYING", "META-INF/COPYRIGHT",
		"META-INF/THIRDPARTY", "META-INF/THIRD-PARTY-NOTICES.txt",
	} {
		assert.True(t, isDependencyNoticePath(name), name)
	}
	for _, name := range []string{
		"META-INF/LICENSE.class", "META-INF/NOTICE.properties", "LICENSE.bin",
		"com/example/Copyright.class", "README.txt", "META-INF/THIRDPARTY.class",
	} {
		assert.False(t, isDependencyNoticePath(name), name)
	}
}

func TestExtractUOSInstallerRejectsDependencyJarNamespaceCaseFoldCollision(t *testing.T) {
	internal := fixtureZip(t,
		fixtureEntry{name: "api/fields/Setting.json", body: []byte("{}")},
		fixtureEntry{name: "api/fields/Device.json", body: []byte("{}")},
		fixtureEntry{name: "sensitive_metadata.json", body: []byte("{}")},
	)
	dependency := fixtureZip(t, fixtureEntry{name: "LICENSE", body: []byte("terms")})
	installer := syntheticInstaller(t, installerFixtureOptions{aceEntries: []fixtureEntry{
		{name: productPropsPath, body: []byte("version=10.4.57\n")},
		{name: internalJarPath, body: internal},
		{name: "BOOT-INF/lib/Example.jar", body: dependency},
		{name: "BOOT-INF/lib/example.jar", body: dependency},
	}})
	path := filepath.Join(t.TempDir(), "installer")
	require.NoError(t, os.WriteFile(path, installer, 0o600))
	_, err := ExtractUOSInstaller(context.Background(), path, t.TempDir(), DefaultArchiveLimits())
	require.ErrorContains(t, err, "namespace case-fold collision")
}
