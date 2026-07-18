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
		return err
	}

	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return fmt.Errorf("unable to open installer zip payload: %w", err)
	}

	for _, zf := range zr.File {
		if zf.Name != "image.tar" {
			continue
		}
		src, err := zf.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.Create(filepath.Join(dir, "image.tar"))
		if err != nil {
			return err
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("unable to extract image.tar: %w", err)
		}
		return nil
	}

	return errors.New("image.tar not found in installer zip payload")
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
			return "", err
		}

		layers, err := img.Layers()
		if err != nil {
			return "", err
		}

		for _, layer := range layers {
			path, err := extractAceJarFromLayer(layer, workDir)
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
		return "", err
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
		defer aceJar.Close()

		if _, err := io.Copy(aceJar, tr); err != nil {
			return "", err
		}
		return aceJar.Name(), nil
	}
}
