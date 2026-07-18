package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"
)

// buildTestZip returns zip bytes containing a single file.
func buildTestZip(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// buildTestAceJar returns ace.jar bytes: an internal-dependencies.jar (with
// the given files) plus product.properties carrying the given version.
func buildTestAceJar(t *testing.T, files map[string]string, networkVersion string) []byte {
	t.Helper()

	var ibuf bytes.Buffer
	izw := zip.NewWriter(&ibuf)
	for name, content := range files {
		w, err := izw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, izw.Close())

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("BOOT-INF/lib/internal-dependencies.jar")
	require.NoError(t, err)
	_, err = w.Write(ibuf.Bytes())
	require.NoError(t, err)
	pw, err := zw.Create("BOOT-INF/classes/product.properties")
	require.NoError(t, err)
	_, err = pw.Write([]byte("product=UniFi\nversion=" + networkVersion + "\n"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// writeTestLayout writes an OCI image layout whose single layer contains the
// given files, returning the layout dir.
func writeTestLayout(t *testing.T, files map[string][]byte) string {
	t.Helper()

	var layerBuf bytes.Buffer
	tw := tar.NewWriter(&layerBuf)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(layerBuf.Bytes())), nil
	})
	require.NoError(t, err)

	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	dir := t.TempDir()
	p, err := layout.Write(dir, empty.Index)
	require.NoError(t, err)
	require.NoError(t, p.AppendImage(img))
	return dir
}

// tarDir returns dir's contents as tar bytes (paths relative to dir).
func tarDir(t *testing.T, dir string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		require.NoError(t, err)
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     rel,
			Mode:     0o644,
			Size:     int64(len(b)),
			Typeflag: tar.TypeReg,
		}))
		_, err = tw.Write(b)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

// writeTestInstaller writes a fake self-extracting installer (shell stub +
// appended zip holding image.tar) and returns its path.
func writeTestInstaller(t *testing.T, imageTar []byte) string {
	t.Helper()
	installer := filepath.Join(t.TempDir(), "installer")
	content := append([]byte("#!/bin/sh\n# fake self-extract stub\nexit 0\n"), buildTestZip(t, "image.tar", imageTar)...)
	require.NoError(t, os.WriteFile(installer, content, 0o755))
	return installer
}
