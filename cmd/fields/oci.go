package main

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
)

const aceJarLayerPath = "usr/lib/unifi/lib/ace.jar"

// extractImageTar extracts image.tar from the zip archive appended to the
// UniFi OS Server installer ELF. Go's archive/zip locates the central
// directory via the end record (baseOffset), so the prepended ELF stub needs
// no special handling.
func extractImageTar(installerPath, dir string) error {
	f, err := os.Open(installerPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("unable to stat installer: %w", err)
	}

	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return fmt.Errorf("unable to open installer zip payload: %w", err)
	}

	for _, zf := range zr.File {
		if zf.Name != "image.tar" {
			continue
		}
		return writeImageTarEntry(zf, dir)
	}

	return errors.New("image.tar not found in installer zip payload")
}

// writeImageTarEntry extracts a single zip entry to dir/image.tar. Close
// errors on the destination are checked so late writeback failures (e.g.
// ENOSPC) surface here instead of as a later parse error.
func writeImageTarEntry(zf *zip.File, dir string) error {
	src, err := zf.Open()
	if err != nil {
		return fmt.Errorf("unable to open image.tar zip entry: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(filepath.Join(dir, "image.tar"))
	if err != nil {
		return fmt.Errorf("unable to create image.tar: %w", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return fmt.Errorf("unable to extract image.tar: %w", err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("unable to finish writing image.tar: %w", err)
	}
	return nil
}

// untarLayout extracts an OCI image layout tar into dir.
func untarLayout(tarPath, dir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		clean := filepath.Clean(header.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("unsafe path in image.tar: %q", header.Name)
		}
		dest := filepath.Join(dir, clean)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			out, err := os.Create(dest)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
}

// findAceJar walks the OCI image layout at layoutDir and extracts ace.jar
// into workDir, returning the temp file path.
func findAceJar(layoutDir, workDir string) (string, error) {
	ii, err := layout.ImageIndexFromPath(layoutDir)
	if err != nil {
		return "", fmt.Errorf("unable to read OCI image layout: %w", err)
	}

	manifest, err := ii.IndexManifest()
	if err != nil {
		return "", err
	}

	for _, m := range manifest.Manifests {
		img, err := ii.Image(m.Digest)
		if err != nil {
			return "", fmt.Errorf("unable to open image: %w", err)
		}

		layers, err := img.Layers()
		if err != nil {
			return "", fmt.Errorf("unable to list image layers: %w", err)
		}

		// Top layer first: OCI overlay semantics give the topmost layer the
		// final say for a path, so its ace.jar wins over stale copies below.
		for i := len(layers) - 1; i >= 0; i-- {
			path, err := extractAceJarFromLayer(layers[i], workDir)
			if err != nil {
				return "", err
			}
			if path != "" {
				return path, nil
			}
		}
	}

	return "", errors.New("ace.jar not found in any image layer")
}

func extractAceJarFromLayer(layer v1.Layer, workDir string) (string, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return "", fmt.Errorf("unable to uncompress layer: %w", err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if strings.TrimPrefix(header.Name, "./") != aceJarLayerPath {
			continue
		}

		aceJar, err := os.CreateTemp(workDir, "ace-*.jar")
		if err != nil {
			return "", err
		}

		if _, err := io.Copy(aceJar, tr); err != nil {
			aceJar.Close()
			return "", err
		}
		if err := aceJar.Close(); err != nil {
			return "", fmt.Errorf("unable to finish writing ace.jar: %w", err)
		}
		return aceJar.Name(), nil
	}
}
