package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

type fixtureEntry struct {
	name     string
	body     []byte
	typeflag byte
	linkname string
}

func fixtureTar(t *testing.T, entries ...fixtureEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		size := int64(len(entry.body))
		if typeflag != tar.TypeReg && typeflag != tar.TypeRegA {
			size = 0
		}
		require.NoError(t, w.WriteHeader(&tar.Header{Name: entry.name, Typeflag: typeflag, Linkname: entry.linkname, Mode: 0o644, Size: size}))
		if size > 0 {
			_, err := w.Write(entry.body)
			require.NoError(t, err)
		}
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func fixtureGzip(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write(body)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func fixtureZip(t *testing.T, entries ...fixtureEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, entry := range entries {
		h := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.typeflag == tar.TypeSymlink {
			h.SetMode(0o777 | 0o120000)
		}
		zw, err := w.CreateHeader(h)
		require.NoError(t, err)
		_, err = zw.Write(entry.body)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

type ociFixtureOptions struct {
	layers            [][]byte
	mediaTypes        []string
	platform          v1.Platform
	nested            bool
	manifestSizeDelta int64
	manifestDigest    digest.Digest
	layerSizeDelta    int64
	layerDigest       digest.Digest
}

func fixtureOCI(t *testing.T, opts ociFixtureOptions) []byte {
	t.Helper()
	if opts.platform.OS == "" {
		opts.platform = v1.Platform{OS: "linux", Architecture: "amd64"}
	}
	var blobs []fixtureEntry
	layerDescs := make([]v1.Descriptor, 0, len(opts.layers))
	for i, layer := range opts.layers {
		mediaType := v1.MediaTypeImageLayer
		if i < len(opts.mediaTypes) && opts.mediaTypes[i] != "" {
			mediaType = opts.mediaTypes[i]
		}
		d := digest.FromBytes(layer)
		if i == len(opts.layers)-1 && opts.layerDigest != "" {
			d = opts.layerDigest
		}
		sz := int64(len(layer))
		if i == len(opts.layers)-1 {
			sz += opts.layerSizeDelta
		}
		layerDescs = append(layerDescs, v1.Descriptor{MediaType: mediaType, Digest: d, Size: sz})
		blobs = append(blobs, fixtureEntry{name: fmt.Sprintf("blobs/sha256/%s", digest.FromBytes(layer).Encoded()), body: layer})
	}
	manifest := v1.Manifest{Versioned: specs.Versioned{SchemaVersion: 2}, MediaType: v1.MediaTypeImageManifest, Config: v1.Descriptor{MediaType: v1.MediaTypeImageConfig, Digest: digest.FromBytes([]byte("{}")), Size: 2}, Layers: layerDescs}
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)
	manifestDigest := digest.FromBytes(manifestBytes)
	if opts.manifestDigest != "" {
		manifestDigest = opts.manifestDigest
	}
	manifestDesc := v1.Descriptor{MediaType: v1.MediaTypeImageManifest, Digest: manifestDigest, Size: int64(len(manifestBytes)) + opts.manifestSizeDelta, Platform: &opts.platform}
	blobs = append(blobs, fixtureEntry{name: "blobs/sha256/" + digest.FromBytes(manifestBytes).Encoded(), body: manifestBytes})

	rootDesc := manifestDesc
	if opts.nested {
		nested := v1.Index{Versioned: specs.Versioned{SchemaVersion: 2}, MediaType: v1.MediaTypeImageIndex, Manifests: []v1.Descriptor{manifestDesc}}
		nestedBytes, err := json.Marshal(nested)
		require.NoError(t, err)
		nestedDigest := digest.FromBytes(nestedBytes)
		rootDesc = v1.Descriptor{MediaType: v1.MediaTypeImageIndex, Digest: nestedDigest, Size: int64(len(nestedBytes))}
		blobs = append(blobs, fixtureEntry{name: "blobs/sha256/" + nestedDigest.Encoded(), body: nestedBytes})
	}
	index := v1.Index{Versioned: specs.Versioned{SchemaVersion: 2}, MediaType: v1.MediaTypeImageIndex, Manifests: []v1.Descriptor{rootDesc}}
	indexBytes, err := json.Marshal(index)
	require.NoError(t, err)
	entries := []fixtureEntry{{name: "oci-layout", body: []byte(`{"imageLayoutVersion":"1.0.0"}`)}, {name: "index.json", body: indexBytes}}
	entries = append(entries, blobs...)
	return fixtureTar(t, entries...)
}

func fixtureReadAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return b
}

func fixtureDigestBytes(body []byte) [32]byte { return sha256.Sum256(body) }

type installerFixtureOptions struct {
	version          string
	innerEntries     []fixtureEntry
	aceEntries       []fixtureEntry
	installerEntries []fixtureEntry
}

func syntheticInstaller(t *testing.T, opts installerFixtureOptions) []byte {
	t.Helper()
	if opts.version == "" {
		opts.version = "10.4.57"
	}
	innerEntries := opts.innerEntries
	if innerEntries == nil {
		innerEntries = []fixtureEntry{
			{name: "api/fields/Setting.json", body: []byte(`{"system":{}}`)},
			{name: "api/fields/Device.json", body: []byte(`{"name":""}`)},
			{name: "sensitive_metadata.json", body: []byte(`{"sensitive":[]}`)},
			{name: "event_defs.json", body: []byte(`{"events":[]}`)},
			{name: "META-INF/NOTICE.txt", body: []byte("inner notice")},
		}
	}
	internal := fixtureZip(t, innerEntries...)
	aceEntries := opts.aceEntries
	if aceEntries == nil {
		aceEntries = []fixtureEntry{
			{name: "BOOT-INF/classes/product.properties", body: []byte("name=UniFi Network\nversion=" + opts.version + "\n")},
			{name: "BOOT-INF/lib/internal-dependencies.jar", body: internal},
			{name: "META-INF/LICENSE", body: []byte("ace license")},
		}
	}
	ace := fixtureZip(t, aceEntries...)
	layer := fixtureTar(t, fixtureEntry{name: "usr/lib/unifi/lib/ace.jar", body: ace})
	imageTar := fixtureOCI(t, ociFixtureOptions{layers: [][]byte{fixtureGzip(t, layer)}, mediaTypes: []string{v1.MediaTypeImageLayerGzip}})
	installerEntries := opts.installerEntries
	if installerEntries == nil {
		installerEntries = []fixtureEntry{{name: "image.tar", body: imageTar}}
	}
	return append([]byte("\x7fELFsynthetic-stub"), fixtureZip(t, installerEntries...)...)
}
