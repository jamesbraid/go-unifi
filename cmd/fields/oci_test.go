package main

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDockerManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	testDockerManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	testDockerLayerGzip    = "application/vnd.docker.image.rootfs.diff.tar.gzip"
)

func TestImportOCIStagesValidatedLayout(t *testing.T) {
	layer := fixtureTar(t, fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", body: []byte("jar")})
	layout, err := ImportOCI(bytes.NewReader(fixtureOCI(t, ociFixtureOptions{layers: [][]byte{layer}})), t.TempDir(), DefaultArchiveLimits())
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(layout.Root, "index.json"))
	assert.FileExists(t, filepath.Join(layout.Root, "oci-layout"))
}

func TestImportOCIRejectsTraversalLinksDuplicatesAndLimits(t *testing.T) {
	valid := DefaultArchiveLimits()
	for _, tc := range []struct {
		name    string
		archive []byte
		limits  ArchiveLimits
	}{
		{"traversal", fixtureTar(t, fixtureEntry{name: "../index.json", body: []byte("x")}), valid},
		{"link", fixtureTar(t, fixtureEntry{name: "index.json", typeflag: tar.TypeSymlink, linkname: "elsewhere"}), valid},
		{"duplicate", fixtureTar(t, fixtureEntry{name: "index.json", body: []byte("x")}, fixtureEntry{name: "./index.json", body: []byte("x")}), valid},
		{"entries", fixtureTar(t, fixtureEntry{name: "oci-layout", body: []byte("x")}, fixtureEntry{name: "index.json", body: []byte("x")}), func() ArchiveLimits { l := valid; l.MaxEntries = 1; return l }()},
		{"bytes", fixtureTar(t, fixtureEntry{name: "oci-layout", body: []byte("x")}), func() ArchiveLimits { l := valid; l.MaxImageTarBytes = 8; return l }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ImportOCI(bytes.NewReader(tc.archive), t.TempDir(), tc.limits)
			require.Error(t, err)
		})
	}
}

func TestResolveImageSelectsNestedLinuxAMD64(t *testing.T) {
	layer := fixtureTar(t, fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", body: []byte("jar")})
	layout, err := ImportOCI(bytes.NewReader(fixtureOCI(t, ociFixtureOptions{layers: [][]byte{layer}, nested: true})), t.TempDir(), DefaultArchiveLimits())
	require.NoError(t, err)
	image, err := ResolveImage(layout, v1.Platform{OS: "linux", Architecture: "amd64"})
	require.NoError(t, err)
	require.Len(t, image.Manifest.Layers, 1)
}

func TestResolveImageAcceptsDockerSchema2ManifestAndNestedList(t *testing.T) {
	layer := fixtureGzip(t, fixtureTar(t, fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", body: []byte("jar")}))
	opts := ociFixtureOptions{
		layers:            [][]byte{layer},
		mediaTypes:        []string{testDockerLayerGzip},
		nested:            true,
		manifestMediaType: testDockerManifest,
		nestedMediaType:   testDockerManifestList,
		indexMediaType:    testDockerManifestList,
	}
	layout, err := ImportOCI(bytes.NewReader(fixtureOCI(t, opts)), t.TempDir(), DefaultArchiveLimits())
	require.NoError(t, err)
	image, err := ResolveImage(layout, v1.Platform{OS: "linux", Architecture: "amd64"})
	require.NoError(t, err)
	f, err := FindFileInLayers(image, "usr/lib/unifi/lib/ace.jar", DefaultArchiveLimits())
	require.NoError(t, err)
	defer f.Close()
	got, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "jar", string(got))
}

func TestResolveImageRejectsDockerMediaTypesInWrongRolesAndUnknownTypes(t *testing.T) {
	layer := fixtureTar(t, fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", body: []byte("jar")})
	for _, tc := range []struct {
		name string
		opts ociFixtureOptions
	}{
		{"list as manifest", ociFixtureOptions{layers: [][]byte{layer}, manifestMediaType: testDockerManifestList, manifestDescType: testDockerManifest}},
		{"manifest as nested list", ociFixtureOptions{layers: [][]byte{layer}, nested: true, nestedMediaType: testDockerManifest, nestedDescType: testDockerManifestList}},
		{"layer as manifest descriptor", ociFixtureOptions{layers: [][]byte{layer}, manifestDescType: testDockerLayerGzip}},
		{"unknown manifest body", ociFixtureOptions{layers: [][]byte{layer}, manifestMediaType: "application/vnd.example.manifest.v2+json", manifestDescType: testDockerManifest}},
		{"unknown index body", ociFixtureOptions{layers: [][]byte{layer}, indexMediaType: "application/vnd.example.index.v1+json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layout, err := ImportOCI(bytes.NewReader(fixtureOCI(t, tc.opts)), t.TempDir(), DefaultArchiveLimits())
			require.NoError(t, err)
			_, err = ResolveImage(layout, v1.Platform{OS: "linux", Architecture: "amd64"})
			require.Error(t, err)
		})
	}
}

func TestResolveImageRejectsDescriptorMismatch(t *testing.T) {
	layer := fixtureTar(t, fixtureEntry{name: "file", body: []byte("x")})
	for _, opts := range []ociFixtureOptions{
		{layers: [][]byte{layer}, manifestSizeDelta: 1},
		{layers: [][]byte{layer}, manifestDigest: digest.FromString("wrong")},
		{layers: [][]byte{layer}, manifestMediaType: testDockerManifest, manifestSizeDelta: 1},
		{layers: [][]byte{layer}, manifestMediaType: testDockerManifest, manifestDigest: digest.FromString("wrong")},
	} {
		layout, err := ImportOCI(bytes.NewReader(fixtureOCI(t, opts)), t.TempDir(), DefaultArchiveLimits())
		require.NoError(t, err)
		_, err = ResolveImage(layout, v1.Platform{OS: "linux", Architecture: "amd64"})
		require.Error(t, err)
	}
}

func TestFindFileInLayersCompressionOverlayAndWhiteouts(t *testing.T) {
	base := fixtureTar(t, fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", body: []byte("old")})
	newer := fixtureTar(t, fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", body: []byte("new")})
	gz := fixtureGzip(t, newer)
	for _, tc := range []struct {
		name        string
		layers      [][]byte
		media       []string
		want        string
		wantMissing bool
	}{
		{"newest gzip wins", [][]byte{base, gz}, []string{v1.MediaTypeImageLayer, v1.MediaTypeImageLayerGzip}, "new", false},
		{"regular whiteout", [][]byte{base, fixtureTar(t, fixtureEntry{name: "usr/lib/unifi/lib/.wh.ace.jar"})}, nil, "", true},
		{"ancestor whiteout", [][]byte{base, fixtureTar(t, fixtureEntry{name: "usr/.wh.lib"})}, nil, "", true},
		{"opaque whiteout", [][]byte{base, fixtureTar(t, fixtureEntry{name: "usr/lib/.wh..wh..opq"})}, nil, "", true},
		{"root opaque whiteout", [][]byte{base, fixtureTar(t, fixtureEntry{name: ".wh..wh..opq"})}, nil, "", true},
		{"same layer addition survives opaque", [][]byte{base, fixtureTar(t, fixtureEntry{name: ".wh..wh..opq"}, fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", body: []byte("replacement")})}, nil, "replacement", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layout, err := ImportOCI(bytes.NewReader(fixtureOCI(t, ociFixtureOptions{layers: tc.layers, mediaTypes: tc.media})), t.TempDir(), DefaultArchiveLimits())
			require.NoError(t, err)
			image, err := ResolveImage(layout, v1.Platform{OS: "linux", Architecture: "amd64"})
			require.NoError(t, err)
			f, err := FindFileInLayers(image, "usr/lib/unifi/lib/ace.jar", DefaultArchiveLimits())
			if tc.wantMissing {
				require.Error(t, err)
				assert.True(t, errors.Is(err, os.ErrNotExist))
				return
			}
			require.NoError(t, err)
			defer f.Close()
			got, err := io.ReadAll(f)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}
}

func TestFindFileInLayersAllowsLiteralBackslashInUnrelatedPOSIXName(t *testing.T) {
	layer := fixtureTar(t,
		fixtureEntry{name: "lib/", typeflag: tar.TypeDir},
		fixtureEntry{name: `lib/systemd/system/system-systemd\x2dcryptsetup.slice`, body: []byte("unit")},
		fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", body: []byte("jar")},
	)
	layout, err := ImportOCI(bytes.NewReader(fixtureOCI(t, ociFixtureOptions{layers: [][]byte{layer}})), t.TempDir(), DefaultArchiveLimits())
	require.NoError(t, err)
	image, err := ResolveImage(layout, v1.Platform{OS: "linux", Architecture: "amd64"})
	require.NoError(t, err)
	f, err := FindFileInLayers(image, aceJarPath, DefaultArchiveLimits())
	require.NoError(t, err)
	defer f.Close()
	got, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "jar", string(got))
}

func TestFindFileInLayersRejectsMalformedOrDuplicateLayerNames(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []fixtureEntry
	}{
		{"slash traversal", []fixtureEntry{{name: "../etc/passwd", body: []byte("x")}}},
		{"absolute", []fixtureEntry{{name: "/etc/passwd", body: []byte("x")}}},
		{"empty slash component", []fixtureEntry{{name: "usr//lib/unifi", body: []byte("x")}}},
		{"dot slash component", []fixtureEntry{{name: "usr/./lib/unifi", body: []byte("x")}}},
		{"duplicate", []fixtureEntry{{name: "usr/share/data", body: []byte("x")}, {name: "usr/share/data", body: []byte("y")}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := append(tc.entries, fixtureEntry{name: aceJarPath, body: []byte("jar")})
			layer := fixtureTar(t, entries...)
			layout, err := ImportOCI(bytes.NewReader(fixtureOCI(t, ociFixtureOptions{layers: [][]byte{layer}})), t.TempDir(), DefaultArchiveLimits())
			require.NoError(t, err)
			image, err := ResolveImage(layout, v1.Platform{OS: "linux", Architecture: "amd64"})
			require.NoError(t, err)
			_, err = FindFileInLayers(image, aceJarPath, DefaultArchiveLimits())
			require.Error(t, err)
		})
	}
}

func TestFindFileInLayersRejectsLayerMismatchUnsupportedTargetLinkAndLimits(t *testing.T) {
	plain := fixtureTar(t, fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", body: []byte("jar")})
	link := fixtureTar(t, fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", typeflag: tar.TypeSymlink, linkname: "elsewhere"})
	for _, tc := range []struct {
		name   string
		opts   ociFixtureOptions
		mutate func(ArchiveLimits) ArchiveLimits
	}{
		{"size", ociFixtureOptions{layers: [][]byte{plain}, layerSizeDelta: 1}, nil},
		{"digest", ociFixtureOptions{layers: [][]byte{plain}, layerDigest: digest.FromString("wrong")}, nil},
		{"zstd", ociFixtureOptions{layers: [][]byte{plain}, mediaTypes: []string{v1.MediaTypeImageLayerZstd}}, nil},
		{"unknown media type", ociFixtureOptions{layers: [][]byte{plain}, mediaTypes: []string{"application/vnd.example.layer.v1+tar"}}, nil},
		{"Docker manifest as layer", ociFixtureOptions{layers: [][]byte{plain}, mediaTypes: []string{testDockerManifest}}, nil},
		{"target link", ociFixtureOptions{layers: [][]byte{link}}, nil},
		{"layer bytes", ociFixtureOptions{layers: [][]byte{plain}}, func(l ArchiveLimits) ArchiveLimits { l.MaxLayerBytes = 16; return l }},
		{"jar bytes", ociFixtureOptions{layers: [][]byte{plain}}, func(l ArchiveLimits) ArchiveLimits { l.MaxJarBytes = 2; return l }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			limits := DefaultArchiveLimits()
			if tc.mutate != nil {
				limits = tc.mutate(limits)
			}
			layout, err := ImportOCI(bytes.NewReader(fixtureOCI(t, tc.opts)), t.TempDir(), limits)
			require.NoError(t, err)
			image, err := ResolveImage(layout, v1.Platform{OS: "linux", Architecture: "amd64"})
			require.NoError(t, err)
			_, err = FindFileInLayers(image, "usr/lib/unifi/lib/ace.jar", limits)
			require.Error(t, err)
		})
	}
}

func TestFindFileInLayersRejectsLinkOrSpecialTargetAncestors(t *testing.T) {
	target := fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", body: []byte("jar")}
	base := fixtureTar(t, target)
	for _, tc := range []struct {
		name   string
		layers [][]byte
	}{
		{
			name: "same layer ancestor symlink",
			layers: [][]byte{fixtureTar(t,
				fixtureEntry{name: "usr/lib/unifi", typeflag: tar.TypeSymlink, linkname: "elsewhere"},
				target,
			)},
		},
		{
			name: "newer layer ancestor hardlink",
			layers: [][]byte{base, fixtureTar(t,
				fixtureEntry{name: "usr/lib", typeflag: tar.TypeLink, linkname: "elsewhere"},
			)},
		},
		{
			name: "newer layer ancestor special",
			layers: [][]byte{base, fixtureTar(t,
				fixtureEntry{name: "usr", typeflag: tar.TypeChar},
			)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			limits := DefaultArchiveLimits()
			layout, err := ImportOCI(bytes.NewReader(fixtureOCI(t, ociFixtureOptions{layers: tc.layers})), t.TempDir(), limits)
			require.NoError(t, err)
			image, err := ResolveImage(layout, v1.Platform{OS: "linux", Architecture: "amd64"})
			require.NoError(t, err)
			_, err = FindFileInLayers(image, aceJarPath, limits)
			require.Error(t, err)
		})
	}
}
