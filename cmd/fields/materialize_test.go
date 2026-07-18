package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaterializeInstallerLocalInput(t *testing.T) {
	contents := []byte("local installer")
	path := filepath.Join(t.TempDir(), "unifi-os-server")
	require.NoError(t, os.WriteFile(path, contents, 0o600))

	got, err := MaterializeInstaller(context.Background(), nil, InstallerSource{
		Kind:      SourceInstallerFile,
		LocalPath: path,
	}, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, path, got.Path)
	assert.EqualValues(t, len(contents), got.Size)
	assert.Equal(t, testSHA256(contents), got.SHA256)

	f, err := os.Open(got.Path)
	require.NoError(t, err)
	defer f.Close()
	_, err = f.Seek(2, io.SeekStart)
	require.NoError(t, err)

	require.NoError(t, got.Close())
	_, err = os.Stat(path)
	require.NoError(t, err, "Close must not delete a local installer")
}

func TestMaterializeInstallerVerifiedDownloadAndClose(t *testing.T) {
	contents := []byte("verified remote installer")
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, schemaFetcherUserAgent, req.Header.Get("User-Agent"))
		_, err := rw.Write(contents)
		require.NoError(t, err)
	}))
	defer server.Close()

	tempRoot := t.TempDir()
	got, err := MaterializeInstaller(context.Background(), server.Client(), remoteInstallerSource(t, server.URL, contents), tempRoot)
	require.NoError(t, err)
	assert.EqualValues(t, len(contents), got.Size)
	assert.Equal(t, testSHA256(contents), got.SHA256)
	assert.Equal(t, tempRoot, filepath.Dir(got.Path))

	f, err := os.Open(got.Path)
	require.NoError(t, err)
	readBack, err := io.ReadAll(f)
	require.NoError(t, err)
	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	assert.Equal(t, contents, readBack)

	require.NoError(t, got.Close())
	_, err = os.Stat(got.Path)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, got.Close(), "Close should be idempotent")
}

func TestMaterializeInstallerRejectsNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		http.Error(rw, "nope", http.StatusBadGateway)
	}))
	defer server.Close()

	_, err := MaterializeInstaller(context.Background(), server.Client(), remoteInstallerSource(t, server.URL, []byte("unused")), t.TempDir())
	require.Error(t, err)
}

func TestMaterializeInstallerHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		<-req.Context().Done()
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 20 * time.Millisecond

	_, err := MaterializeInstaller(context.Background(), client, remoteInstallerSource(t, server.URL, []byte("unused")), t.TempDir())
	require.Error(t, err)
}

func TestMaterializeInstallerHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	u, err := url.Parse("https://fw-download.ubnt.com/installer")
	require.NoError(t, err)

	_, err = MaterializeInstaller(ctx, http.DefaultClient, InstallerSource{Kind: SourceInstallerURL, URL: u}, t.TempDir())
	require.ErrorIs(t, err, context.Canceled)
}

func TestMaterializeInstallerRejectsDeclaredSizeMismatch(t *testing.T) {
	contents := []byte("wrong size")
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		_, err := rw.Write(contents)
		require.NoError(t, err)
	}))
	defer server.Close()
	src := remoteInstallerSource(t, server.URL, contents)
	src.ExpectedSize++
	tempRoot := t.TempDir()

	_, err := MaterializeInstaller(context.Background(), server.Client(), src, tempRoot)
	require.Error(t, err)
	assertDirectoryEmpty(t, tempRoot)
}

func TestMaterializeInstallerRejectsSHA256Mismatch(t *testing.T) {
	contents := []byte("wrong digest")
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		_, err := rw.Write(contents)
		require.NoError(t, err)
	}))
	defer server.Close()
	src := remoteInstallerSource(t, server.URL, contents)
	src.ExpectedSHA256 = testSHA256([]byte("different"))
	tempRoot := t.TempDir()

	_, err := MaterializeInstaller(context.Background(), server.Client(), src, tempRoot)
	require.Error(t, err)
	assertDirectoryEmpty(t, tempRoot)
}

func TestMaterializeInstallerCleansPartialFileOnCopyFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Length", "100")
		_, err := rw.Write([]byte("short"))
		require.NoError(t, err)
	}))
	defer server.Close()
	tempRoot := t.TempDir()

	_, err := MaterializeInstaller(context.Background(), server.Client(), remoteInstallerSource(t, server.URL, make([]byte, 100)), tempRoot)
	require.Error(t, err)
	assertDirectoryEmpty(t, tempRoot)
}

func TestMaterializeInstallerRejectsMissingLocalInput(t *testing.T) {
	_, err := MaterializeInstaller(context.Background(), nil, InstallerSource{
		Kind:      SourceInstallerFile,
		LocalPath: filepath.Join(t.TempDir(), "missing"),
	}, t.TempDir())
	require.Error(t, err)
	assert.False(t, errors.Is(err, context.Canceled))
}

func remoteInstallerSource(t *testing.T, rawURL string, contents []byte) InstallerSource {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return InstallerSource{
		Kind:           SourceUOSLatest,
		URL:            u,
		ExpectedSize:   int64(len(contents)),
		ExpectedSHA256: testSHA256(contents),
	}
}

func testSHA256(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}

func assertDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
