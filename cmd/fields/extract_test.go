package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
)

func buildZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	return buf.Bytes()
}

func buildTar(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, data := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(data)),
		}))
		_, err := tw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	return buf.Bytes()
}

type arMember struct {
	name string
	data []byte
}

// buildAr assembles a Unix ar(1) archive (the .deb container format): the
// "!<arch>\n" global header, then per member a 60-byte header — name (16),
// mtime (12), uid (6), gid (6), octal mode (8), size (10), "`\n" magic —
// followed by the data, padded to 2-byte alignment with a newline.
func buildAr(t *testing.T, members []arMember) []byte {
	t.Helper()

	var buf bytes.Buffer
	buf.WriteString("!<arch>\n")
	for _, m := range members {
		require.LessOrEqual(t, len(m.name), 16)
		fmt.Fprintf(&buf, "%-16s%-12d%-6d%-6d%-8o%-10d`\n", m.name, 0, 0, 0, 0o644, len(m.data))
		buf.Write(m.data)
		if len(m.data)%2 == 1 {
			buf.WriteByte('\n')
		}
	}

	return buf.Bytes()
}

func buildDeb(t *testing.T, dataFiles map[string][]byte) []byte {
	t.Helper()

	var dataXz bytes.Buffer
	xzw, err := xz.NewWriter(&dataXz)
	require.NoError(t, err)
	_, err = xzw.Write(buildTar(t, dataFiles))
	require.NoError(t, err)
	require.NoError(t, xzw.Close())

	return buildAr(t, []arMember{
		{"debian-binary", []byte("2.0\n")},
		{"data.tar.xz", dataXz.Bytes()},
	})
}

// buildInstaller assembles a fake UniFi OS Server self-extracting installer:
// an arbitrary stub followed by a zip holding an OCI-style image.tar whose
// gzipped layer contains the given files.
func buildInstaller(t *testing.T, layerFiles map[string][]byte) []byte {
	t.Helper()

	var layerGz bytes.Buffer
	gz := gzip.NewWriter(&layerGz)
	_, err := gz.Write(buildTar(t, layerFiles))
	require.NoError(t, err)
	require.NoError(t, gz.Close())

	imageTar := buildTar(t, map[string][]byte{
		"blobs/sha256/aaaa": []byte("tiny blob, skipped by size"),
		"blobs/sha256/bbbb": layerGz.Bytes(),
	})

	payload := buildZip(t, map[string][]byte{
		"image.tar":  imageTar,
		"install.sh": []byte("#!/bin/sh\n"),
	})

	// The stub deliberately contains decoy zip signatures: machine code can
	// contain any byte sequence, and archive/zip's prepended-data handling
	// must not be misled by them.
	stub := append([]byte("\x7fELF fake installer stub PK\x03\x04 decoy PK\x05\x06 decoy "), bytes.Repeat([]byte{0xab}, 4096)...)
	return append(stub, payload...)
}

func writeTempArtifact(t *testing.T, data []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "artifact")
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

const productProperties = "name=UniFi Network\nversion=10.4.57\nbuild=34628\n"

func fieldsUserJSON() []byte {
	return []byte(`{"name": ".{0,128}", "x_password": ".{1,128}"}`)
}

// runExtraction drives the full artifact -> schemas pipeline used by
// buildSchemas and returns the fields/metadata dirs.
func runExtraction(t *testing.T, artifact []byte) (fieldsDir, metadataDir string) {
	t.Helper()

	restoreMin := minFieldFiles
	minFieldFiles = 1
	t.Cleanup(func() { minFieldFiles = restoreMin })

	workDir := t.TempDir()
	arts, err := extractArtifacts(writeTempArtifact(t, artifact), workDir)
	require.NoError(t, err)

	networkVersion, err := readNetworkVersion(arts.aceJar)
	require.NoError(t, err)
	require.Equal(t, "10.4.57", networkVersion.String())

	defsJar, err := resolveDefsJar(arts, workDir)
	require.NoError(t, err)

	schemasDir := t.TempDir()
	fieldsDir = filepath.Join(schemasDir, "fields")
	metadataDir = filepath.Join(schemasDir, "metadata")
	customDir := filepath.Join(t.TempDir(), "custom")
	require.NoError(t, os.MkdirAll(customDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(customDir, "Custom.json"), []byte(`{"enabled": "true|false"}`), 0o644))
	require.NoError(t, extractSchemas(defsJar, fieldsDir, metadataDir, customDir))

	return fieldsDir, metadataDir
}

func requireExtractedSchemas(t *testing.T, fieldsDir, metadataDir string) {
	t.Helper()

	user, err := os.ReadFile(filepath.Join(fieldsDir, "User.json"))
	require.NoError(t, err)
	require.Equal(t, fieldsUserJSON(), user)

	// Setting.json is split into per-section files.
	setting, err := os.ReadFile(filepath.Join(fieldsDir, "SettingMgmt.json"))
	require.NoError(t, err)
	require.Contains(t, string(setting), "x_ssh_password")

	sensitive, err := os.ReadFile(filepath.Join(metadataDir, "sensitive_metadata.json"))
	require.NoError(t, err)
	require.Contains(t, string(sensitive), "sensitive_db_fields_by_collection")

	// The custom overlay is copied into the fields dir.
	custom, err := os.ReadFile(filepath.Join(fieldsDir, "Custom.json"))
	require.NoError(t, err)
	require.Contains(t, string(custom), "enabled")
}

// defsJarFiles is the content of internal-dependencies.jar in 10.x layouts,
// and of ace.jar itself in the pre-10.x layout.
func defsJarFiles() map[string][]byte {
	return map[string][]byte{
		"api/fields/User.json":    fieldsUserJSON(),
		"api/fields/Setting.json": []byte(`{"mgmt": {"x_ssh_password": ".{1,128}"}}`),
		"api/fields/ignored.txt":  []byte("not json"),
		"sensitive_metadata.json": []byte(`{"sensitive_db_fields_by_collection": {"user": ["hostname"]}}`),
		"oui_table.json":          []byte(`{"not": "extracted"}`),
	}
}

func TestExtractFromOldStyleDeb(t *testing.T) {
	// Pre-10.x: ace.jar carries the definitions at its root.
	aceJar := defsJarFiles()
	aceJar["product.properties"] = []byte(productProperties)

	deb := buildDeb(t, map[string][]byte{
		"./usr/lib/unifi/lib/ace.jar": buildZip(t, aceJar),
		"./usr/lib/unifi/readme.txt":  []byte("filler"),
	})

	fieldsDir, metadataDir := runExtraction(t, deb)
	requireExtractedSchemas(t, fieldsDir, metadataDir)
}

func TestExtractFromZstdDeb(t *testing.T) {
	// Ubuntu-ecosystem packages increasingly ship data.tar.zst.
	var dataZst bytes.Buffer
	zw, err := zstd.NewWriter(&dataZst)
	require.NoError(t, err)
	_, err = zw.Write(buildTar(t, map[string][]byte{
		"./usr/lib/unifi/lib/ace.jar": buildZip(t, func() map[string][]byte {
			m := defsJarFiles()
			m["product.properties"] = []byte(productProperties)
			return m
		}()),
	}))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	deb := buildAr(t, []arMember{
		{"debian-binary", []byte("2.0\n")},
		{"data.tar.zst", dataZst.Bytes()},
	})

	fieldsDir, metadataDir := runExtraction(t, deb)
	requireExtractedSchemas(t, fieldsDir, metadataDir)
}

func TestExtractFromThinLauncherDeb(t *testing.T) {
	// 10.x deb: thin ace.jar launcher plus a standalone
	// internal-dependencies.jar.
	deb := buildDeb(t, map[string][]byte{
		"./usr/lib/unifi/lib/ace.jar": buildZip(t, map[string][]byte{
			"product.properties": []byte(productProperties),
		}),
		"./usr/lib/unifi/lib/internal/internal-dependencies.jar": buildZip(t, defsJarFiles()),
	})

	fieldsDir, metadataDir := runExtraction(t, deb)
	requireExtractedSchemas(t, fieldsDir, metadataDir)
}

func TestExtractFromUOSInstaller(t *testing.T) {
	// UniFi OS Server: Spring Boot fat ace.jar inside an OCI image layer.
	restore := minLayerSize
	minLayerSize = 1
	t.Cleanup(func() { minLayerSize = restore })

	fatAceJar := buildZip(t, map[string][]byte{
		"BOOT-INF/classes/product.properties":    []byte(productProperties),
		"BOOT-INF/lib/internal-dependencies.jar": buildZip(t, defsJarFiles()),
		"BOOT-INF/lib/some-other-dependency.jar": {},
		"META-INF/MANIFEST.MF":                   []byte("Main-Class: x\n"),
	})

	installer := buildInstaller(t, map[string][]byte{
		"usr/lib/unifi/lib/ace.jar": fatAceJar,
		"usr/bin/unifi":             []byte("filler"),
	})

	fieldsDir, metadataDir := runExtraction(t, installer)
	requireExtractedSchemas(t, fieldsDir, metadataDir)
}

func TestExtractSchemasRejectsNearEmptyJar(t *testing.T) {
	// A wrong or truncated definitions jar must fail loudly instead of
	// blessing an almost-empty snapshot.
	defsJar := writeTempArtifact(t, buildZip(t, map[string][]byte{
		"api/fields/User.json": fieldsUserJSON(),
	}))

	schemasDir := t.TempDir()
	err := extractSchemas(defsJar,
		filepath.Join(schemasDir, "fields"),
		filepath.Join(schemasDir, "metadata"),
		t.TempDir())
	require.ErrorContains(t, err, "api/fields definitions")
}

func TestNewFieldInfoRejectsUnsafeSchemaText(t *testing.T) {
	// Wire names are rendered into struct tags and validation strings into
	// comments of generated Go code; hostile schema content must not be able
	// to inject source.
	for _, name := range []string{
		"evil\"`,},", "with space", "back`tick", "new\nline", "",
	} {
		require.Panics(t, func() {
			NewFieldInfo("Evil", name, "string", "", false, false, false, "")
		}, "name %q should be rejected", name)
	}

	f := NewFieldInfo("OK", "ok_name", "string", "a|b\n}\nfunc init() {`", false, false, false, "")
	require.NotContains(t, f.FieldValidation, "\n")
	require.NotContains(t, f.FieldValidation, "`")
}

func TestReadNetworkVersionMissing(t *testing.T) {
	aceJar := writeTempArtifact(t, buildZip(t, map[string][]byte{
		"META-INF/MANIFEST.MF": []byte("Main-Class: x\n"),
	}))

	_, err := readNetworkVersion(aceJar)
	require.Error(t, err)
}
