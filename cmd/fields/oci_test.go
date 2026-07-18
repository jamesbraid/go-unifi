package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractImageTar(t *testing.T) {
	installer := writeTestInstaller(t, []byte("fake-tar-content"))
	dir := t.TempDir()

	require.NoError(t, extractImageTar(installer, dir))

	b, err := os.ReadFile(filepath.Join(dir, "image.tar"))
	require.NoError(t, err)
	assert.Equal(t, "fake-tar-content", string(b))
}

func TestExtractImageTarMissing(t *testing.T) {
	installer := writeTestInstaller(t, []byte("x"))
	// overwrite with a zip that has no image.tar
	require.NoError(t, os.WriteFile(installer,
		append([]byte("#!/bin/sh\n"), buildTestZip(t, "other.bin", []byte("x"))...), 0o755))

	err := extractImageTar(installer, t.TempDir())
	assert.ErrorContains(t, err, "image.tar")
}

func TestUntarLayoutAndFindAceJar(t *testing.T) {
	aceJar := buildTestAceJar(t, map[string]string{
		"api/fields/NetworkConf.json": `{"name":".{1,128}"}`,
	}, "10.4.57")

	layoutDir := writeTestLayout(t, map[string][]byte{
		"usr/lib/unifi/lib/ace.jar": aceJar,
		"usr/bin/bash":              []byte("not-a-jar"),
	})

	// round-trip through image.tar + untarLayout, like the real pipeline
	imageTar := tarDir(t, layoutDir)
	untarDir := t.TempDir()
	imageTarPath := filepath.Join(t.TempDir(), "image.tar")
	require.NoError(t, os.WriteFile(imageTarPath, imageTar, 0o644))
	require.NoError(t, untarLayout(imageTarPath, untarDir))

	got, err := findAceJar(untarDir, t.TempDir())
	require.NoError(t, err)

	b, err := os.ReadFile(got)
	require.NoError(t, err)
	assert.Equal(t, aceJar, b)
}

func TestUntarLayoutRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../evil.txt", "/tmp/evil.txt"} {
		t.Run(name, func(t *testing.T) {
			tarPath := filepath.Join(t.TempDir(), "evil.tar")
			writeTarFile(t, tarPath, name)

			err := untarLayout(tarPath, t.TempDir())
			assert.ErrorContains(t, err, "unsafe path")
		})
	}
}

func TestFindAceJarMissing(t *testing.T) {
	layoutDir := writeTestLayout(t, map[string][]byte{
		"usr/bin/bash": []byte("nope"),
	})
	_, err := findAceJar(layoutDir, t.TempDir())
	assert.ErrorContains(t, err, "ace.jar")
}

func TestFindAceJarTopLayerWins(t *testing.T) {
	layoutDir := writeTestLayoutMulti(t, []map[string][]byte{
		{"usr/lib/unifi/lib/ace.jar": []byte("stale")},
		{"./usr/lib/unifi/lib/ace.jar": []byte("fresh")}, // "./" prefix, as real layers often carry
	})

	got, err := findAceJar(layoutDir, t.TempDir())
	require.NoError(t, err)

	b, err := os.ReadFile(got)
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(b))
}
