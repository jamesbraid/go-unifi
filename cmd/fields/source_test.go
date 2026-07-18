package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSourceSelector(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want SourceSelector
	}{
		{name: "legacy latest", args: []string{"-latest"}, want: SourceSelector{Kind: SourceLegacyLatest}},
		{name: "legacy version", args: []string{"9.5.21"}, want: SourceSelector{Kind: SourceLegacyVersion, Value: "9.5.21"}},
		{name: "UOS latest", args: []string{"-uos-latest"}, want: SourceSelector{Kind: SourceUOSLatest}},
		{name: "UOS version", args: []string{"-uos-version", "5.1.21"}, want: SourceSelector{Kind: SourceUOSVersion, Value: "5.1.21"}},
		{name: "installer file", args: []string{"-installer", "/tmp/unifi-os-server"}, want: SourceSelector{Kind: SourceInstallerFile, Value: "/tmp/unifi-os-server"}},
		{name: "installer URL", args: []string{"-installer-url", "https://fw-download.ubnt.com/data/unifi-os-server/test"}, want: SourceSelector{Kind: SourceInstallerURL, Value: "https://fw-download.ubnt.com/data/unifi-os-server/test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSourceSelector(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	for _, args := range [][]string{
		{},
		{"9.5.21", "extra"},
		{"9.5.21", "-latest"},
		{"-latest", "-uos-latest"},
		{"-uos-latest", "-uos-version", "5.1.21"},
		{"-uos-version", "5.1.21", "-installer", "/tmp/installer"},
		{"-installer", "/tmp/installer", "-installer-url", "https://ui.com/installer"},
	} {
		t.Run("rejects "+fmt.Sprint(args), func(t *testing.T) {
			_, err := ParseSourceSelector(args)
			require.Error(t, err, "ParseSourceSelector(%q)", args)
		})
	}
}

func TestValidateInstallerURL(t *testing.T) {
	for _, rawURL := range []string{
		"https://ui.com/installer",
		"https://fw-download.ui.com/installer",
		"https://ubnt.com/installer",
		"https://fw-download.ubnt.com/installer",
	} {
		t.Run("accepts "+rawURL, func(t *testing.T) {
			u, err := url.Parse(rawURL)
			require.NoError(t, err)
			require.NoError(t, ValidateInstallerURL(u))
		})
	}

	for _, rawURL := range []string{
		"http://ui.com/installer",
		"https://ui.com.example/installer",
		"https://notui.com/installer",
		"https://example.com/installer",
		"https://ui.com@evil.example/installer",
		"https:///installer",
	} {
		t.Run("rejects "+rawURL, func(t *testing.T) {
			u, err := url.Parse(rawURL)
			require.NoError(t, err)
			require.Error(t, ValidateInstallerURL(u))
		})
	}
}

func TestResolveInstaller(t *testing.T) {
	const (
		firmwareID = "0ab7907a-0e19-4ef1-8a48-997fda5cd7b5"
		sha256Sum  = "77e3feac1595779402dd87ff8d20d66faa39c87b572646f86ff0006711262445"
		md5Sum     = "72f3373dfaf441afe33536221837bafe"
	)
	created := time.Date(2026, 7, 8, 14, 37, 42, 0, time.UTC)
	downloadURL, err := url.Parse("https://fw-download.ubnt.com/data/unifi-os-server/test.21-x64")
	require.NoError(t, err)
	fwVersion, err := version.NewVersion("v5.1.21")
	require.NoError(t, err)

	record := firmwareUpdateApiResponseEmbeddedFirmware{
		Channel:        "release",
		Created:        created.Format(time.RFC3339),
		Updated:        created.Format(time.RFC3339),
		FileSize:       880119750,
		Id:             firmwareID,
		MD5:            md5Sum,
		SHA256Checksum: sha256Sum,
		Platform:       "linux-x64",
		Product:        "unifi-os-server",
		Version:        fwVersion,
		Links: firmwareUpdateApiResponseEmbeddedFirmwareLinks{
			Data: firmwareUpdateApiResponseEmbeddedFirmwareDataLink{Href: downloadURL},
		},
	}

	for _, tt := range []struct {
		name     string
		selector SourceSelector
		version  string
	}{
		{name: "latest", selector: SourceSelector{Kind: SourceUOSLatest}},
		{name: "version", selector: SourceSelector{Kind: SourceUOSVersion, Value: "5.1.21"}, version: "v5.1.21"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				assert.Equal(t, schemaFetcherUserAgent, req.Header.Get("User-Agent"))
				filters := req.URL.Query()["filter"]
				assert.Contains(t, filters, firmwareUpdateApiFilter("eq", "product", "unifi-os-server"))
				assert.Contains(t, filters, firmwareUpdateApiFilter("eq", "platform", "linux-x64"))
				assert.Contains(t, filters, firmwareUpdateApiFilter("eq", "channel", "release"))
				if tt.version != "" {
					assert.Contains(t, filters, firmwareUpdateApiFilter("eq", "version", tt.version))
				}
				require.NoError(t, json.NewEncoder(rw).Encode(firmwareUpdateApiResponse{
					Embedded: firmwareUpdateApiResponseEmbedded{Firmware: []firmwareUpdateApiResponseEmbeddedFirmware{record}},
				}))
			}))
			defer server.Close()

			got, err := ResolveInstaller(context.Background(), server.Client(), server.URL, tt.selector)
			require.NoError(t, err)
			assert.Equal(t, tt.selector.Kind, got.Kind)
			assert.Equal(t, "5.1.21", got.OSVersion)
			assert.Equal(t, firmwareID, got.FirmwareID)
			assert.Equal(t, "unifi-os-server", got.Product)
			assert.Equal(t, "linux-x64", got.Platform)
			assert.Equal(t, "release", got.Channel)
			assert.Equal(t, downloadURL, got.URL)
			assert.EqualValues(t, 880119750, got.ExpectedSize)
			assert.Equal(t, sha256Sum, got.ExpectedSHA256)
			assert.Equal(t, md5Sum, got.ExpectedMD5)
			assert.Equal(t, created, got.Created)
			assert.Equal(t, created, got.Updated)
		})
	}
}

func TestResolveInstallerRejectsIncompleteOrMismatchedRecords(t *testing.T) {
	valid := map[string]any{
		"channel": "release", "created": "2026-07-08T14:37:42Z", "updated": "2026-07-08T14:37:42Z",
		"file_size": 12, "id": "firmware-id", "md5": "72f3373dfaf441afe33536221837bafe",
		"sha256_checksum": "77e3feac1595779402dd87ff8d20d66faa39c87b572646f86ff0006711262445",
		"platform":        "linux-x64", "product": "unifi-os-server", "version": "v5.1.21",
		"_links": map[string]any{"data": map[string]any{"href": "https://fw-download.ubnt.com/installer"}},
	}

	for _, mutate := range []func([]map[string]any) []map[string]any{
		func(records []map[string]any) []map[string]any { return nil },
		func(records []map[string]any) []map[string]any { return append(records, valid) },
		func(records []map[string]any) []map[string]any {
			records[0]["product"] = "unifi-controller"
			return records
		},
		func(records []map[string]any) []map[string]any { records[0]["file_size"] = 0; return records },
		func(records []map[string]any) []map[string]any { records[0]["sha256_checksum"] = "bad"; return records },
		func(records []map[string]any) []map[string]any {
			records[0]["_links"] = map[string]any{}
			return records
		},
	} {
		record := make(map[string]any, len(valid))
		for key, value := range valid {
			record[key] = value
		}
		records := mutate([]map[string]any{record})
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			require.NoError(t, json.NewEncoder(rw).Encode(map[string]any{"_embedded": map[string]any{"firmware": records}}))
		}))
		_, err := ResolveInstaller(context.Background(), server.Client(), server.URL, SourceSelector{Kind: SourceUOSLatest})
		server.Close()
		require.Error(t, err)
	}
}

func TestResolveInstallerLegacyLatestCarriesAndEnforcesMetadata(t *testing.T) {
	contents := []byte("legacy installer")
	downloadURL := "https://fw-download.ubnt.com/data/unifi-controller/test.deb"
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(rw).Encode(map[string]any{
			"_embedded": map[string]any{"firmware": []map[string]any{{
				"channel": "release", "created": "2026-07-08T14:37:42Z", "updated": "2026-07-08T14:37:42Z",
				"file_size": len(contents), "id": "legacy-firmware-id", "md5": "eb92ab1b9f38c6c5333b32e940a961f5",
				"sha256_checksum": testSHA256(contents), "platform": "debian", "product": "unifi-controller",
				"version": "9.5.21", "_links": map[string]any{"data": map[string]any{"href": downloadURL}},
			}}},
		}))
	}))
	defer server.Close()

	src, err := ResolveInstaller(context.Background(), server.Client(), server.URL, SourceSelector{Kind: SourceLegacyLatest})
	require.NoError(t, err)
	assert.EqualValues(t, len(contents), src.ExpectedSize)
	assert.Equal(t, testSHA256(contents), src.ExpectedSHA256)
	assert.Equal(t, "eb92ab1b9f38c6c5333b32e940a961f5", src.ExpectedMD5)
	assert.Equal(t, "legacy-firmware-id", src.FirmwareID)
	assert.Equal(t, "unifi-controller", src.Product)
	assert.Equal(t, "debian", src.Platform)
	assert.Equal(t, "release", src.Channel)

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return bodyResponse(req, http.StatusOK, []byte("tampered content")), nil
	})}
	_, err = MaterializeInstaller(context.Background(), client, src, t.TempDir())
	require.ErrorContains(t, err, "SHA-256 mismatch", "legacy latest materialization must enforce API metadata")
}
