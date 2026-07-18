package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func osServerFixture(t *testing.T, entries []firmwareUpdateApiResponseEmbeddedFirmware) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		resp, err := json.Marshal(firmwareUpdateApiResponse{
			Embedded: firmwareUpdateApiResponseEmbedded{Firmware: entries},
		})
		require.NoError(t, err)
		_, err = rw.Write(resp)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	return server
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func mustVersion(t *testing.T, v string) *version.Version {
	t.Helper()
	ver, err := version.NewVersion(v)
	require.NoError(t, err)
	return ver
}

func TestLatestOsServerRelease(t *testing.T) {
	server := osServerFixture(t, []firmwareUpdateApiResponseEmbeddedFirmware{
		{
			Channel: releaseChannel, Platform: "linux-arm64", Product: osServerProduct,
			Version:        mustVersion(t, "5.1.21"),
			Sha256Checksum: "arm64sha",
			Links: firmwareUpdateApiResponseEmbeddedFirmwareLinks{
				Data: firmwareUpdateApiResponseEmbeddedFirmwareDataLink{
					Href: mustURL(t, "https://fw-download.ubnt.com/arm64"),
				},
			},
		},
		{
			Channel: releaseChannel, Platform: osServerPlatform, Product: osServerProduct,
			Version:        mustVersion(t, "5.1.21"),
			Sha256Checksum: "x64sha",
			Links: firmwareUpdateApiResponseEmbeddedFirmwareLinks{
				Data: firmwareUpdateApiResponseEmbeddedFirmwareDataLink{
					Href: mustURL(t, "https://fw-download.ubnt.com/x64"),
				},
			},
		},
	})

	old := firmwareLatestApi
	firmwareLatestApi = server.URL
	t.Cleanup(func() { firmwareLatestApi = old })

	rel, err := latestOsServerRelease()
	require.NoError(t, err)
	assert.Equal(t, "5.1.21", rel.Version.String())
	assert.Equal(t, "https://fw-download.ubnt.com/x64", rel.URL.String())
	assert.Equal(t, "x64sha", rel.SHA256)
}

func TestFindOsServerRelease(t *testing.T) {
	server := osServerFixture(t, []firmwareUpdateApiResponseEmbeddedFirmware{
		{
			Channel: releaseChannel, Platform: osServerPlatform, Product: osServerProduct,
			Version: mustVersion(t, "5.1.21"),
			Links: firmwareUpdateApiResponseEmbeddedFirmwareLinks{
				Data: firmwareUpdateApiResponseEmbeddedFirmwareDataLink{
					Href: mustURL(t, "https://fw-download.ubnt.com/new"),
				},
			},
		},
		{
			Channel: releaseChannel, Platform: osServerPlatform, Product: osServerProduct,
			Version: mustVersion(t, "5.0.8"),
			Links: firmwareUpdateApiResponseEmbeddedFirmwareLinks{
				Data: firmwareUpdateApiResponseEmbeddedFirmwareDataLink{
					Href: mustURL(t, "https://fw-download.ubnt.com/old"),
				},
			},
		},
	})

	old := firmwareApi
	firmwareApi = server.URL
	t.Cleanup(func() { firmwareApi = old })

	rel, err := findOsServerRelease("5.0.8")
	require.NoError(t, err)
	assert.Equal(t, "https://fw-download.ubnt.com/old", rel.URL.String())

	_, err = findOsServerRelease("9.9.9")
	assert.ErrorContains(t, err, "9.9.9")
}
