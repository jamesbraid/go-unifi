package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const installerDownloadTimeout = 30 * time.Minute

type MaterializedInstaller struct {
	Path      string
	Size      int64
	SHA256    string
	temporary bool
	closed    bool
}

func MaterializeInstaller(ctx context.Context, client *http.Client, src InstallerSource, tempRoot string) (*MaterializedInstaller, error) {
	if src.Kind == SourceInstallerFile {
		return materializeLocalInstaller(src)
	}
	if src.URL == nil {
		return nil, errors.New("installer source has no URL")
	}
	if client == nil {
		client = http.DefaultClient
	}
	client = installerHTTPClient(client)
	downloadCtx, cancel := context.WithTimeout(ctx, installerDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, src.URL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create installer request: %w", err)
	}
	req.Header.Set("User-Agent", schemaFetcherUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download installer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download installer: unexpected HTTP status %s", resp.Status)
	}

	file, err := os.CreateTemp(tempRoot, "unifi-os-server-*")
	if err != nil {
		return nil, fmt.Errorf("create installer temporary file: %w", err)
	}
	path := file.Name()
	keep := false
	defer func() {
		if !keep {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()

	hasher := sha256.New()
	var reader io.Reader = resp.Body
	if src.ExpectedSize > 0 {
		reader = io.LimitReader(resp.Body, src.ExpectedSize+1)
	}
	size, err := io.Copy(io.MultiWriter(file, hasher), reader)
	if err != nil {
		return nil, fmt.Errorf("write installer temporary file: %w", err)
	}
	if src.ExpectedSize > 0 && size != src.ExpectedSize {
		return nil, fmt.Errorf("installer size mismatch: expected %d, got %d", src.ExpectedSize, size)
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	if src.ExpectedSHA256 != "" && !strings.EqualFold(digest, src.ExpectedSHA256) {
		return nil, fmt.Errorf("installer SHA-256 mismatch: expected %s, got %s", src.ExpectedSHA256, digest)
	}
	if err := file.Sync(); err != nil {
		return nil, fmt.Errorf("sync installer temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close installer temporary file: %w", err)
	}

	keep = true
	return &MaterializedInstaller{
		Path:      path,
		Size:      size,
		SHA256:    digest,
		temporary: true,
	}, nil
}

func installerHTTPClient(client *http.Client) *http.Client {
	redirectClient := *client
	callerCheckRedirect := client.CheckRedirect
	redirectClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := ValidateInstallerURL(req.URL); err != nil {
			return fmt.Errorf("invalid installer redirect: %w", err)
		}
		if callerCheckRedirect != nil {
			return callerCheckRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &redirectClient
}

func materializeLocalInstaller(src InstallerSource) (*MaterializedInstaller, error) {
	if src.LocalPath == "" {
		return nil, errors.New("installer source has no local path")
	}
	file, err := os.Open(src.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("open local installer: %w", err)
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(hasher, file)
	closeErr := file.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("hash local installer: %w", copyErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close local installer: %w", closeErr)
	}
	return &MaterializedInstaller{
		Path:   src.LocalPath,
		Size:   size,
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func (m *MaterializedInstaller) Close() error {
	if m == nil || m.closed {
		return nil
	}
	if !m.temporary {
		m.closed = true
		return nil
	}
	if err := os.Remove(m.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove materialized installer: %w", err)
	}
	m.closed = true
	return nil
}
