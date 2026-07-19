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

func firmwareResponse(product, platform, versionStr, href string) firmwareUpdateApiResponseEmbeddedFirmware {
	fw := firmwareUpdateApiResponseEmbeddedFirmware{
		Channel:  releaseChannel,
		Id:       "fw-" + platform,
		Platform: platform,
		Product:  product,
		Version:  version.Must(version.NewVersion(versionStr)),
	}
	if href != "" {
		fw.Links.Data.Href = mustParseURL(href)
	}
	return fw
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// fakeFirmwareServer serves firmware-latest responses keyed by the product
// filter in the request.
func fakeFirmwareServer(t *testing.T, byProduct map[string][]firmwareUpdateApiResponseEmbeddedFirmware) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		query := req.URL.Query()
		assert.Contains(t, query["filter"], firmwareUpdateApiFilter("eq", "channel", releaseChannel))

		var product string
		for _, filter := range query["filter"] {
			for p := range byProduct {
				if filter == firmwareUpdateApiFilter("eq", "product", p) {
					product = p
				}
			}
		}

		resp, err := json.Marshal(firmwareUpdateApiResponse{
			Embedded: firmwareUpdateApiResponseEmbedded{
				Firmware: byProduct[product],
			},
		})
		assert.NoError(t, err)

		_, err = rw.Write(resp)
		assert.NoError(t, err)
	}))

	t.Cleanup(server.Close)

	// Point the package at the fake and restore afterwards so later tests
	// don't inherit a closed server.
	saved := firmwareUpdateApi
	firmwareUpdateApi = server.URL
	t.Cleanup(func() { firmwareUpdateApi = saved })

	return server
}

func TestLatestReleasePrefersDebianController(t *testing.T) {
	require := require.New(t)

	debURL := "https://fw-download.ubnt.com/data/unifi-controller/fa30-debian-10.4.57.deb"
	fakeFirmwareServer(t, map[string][]firmwareUpdateApiResponseEmbeddedFirmware{
		unifiControllerProduct: {
			firmwareResponse(unifiControllerProduct, "document", "10.4.57+atag-10.4.57-34628", ""),
			firmwareResponse(unifiControllerProduct, debianPlatform, "10.4.57+atag-10.4.57-34628", debURL),
		},
		uosServerProduct: {
			firmwareResponse(uosServerProduct, uosServerPlatform, "5.1.21", "https://fw-download.ubnt.com/data/unifi-os-server/f5e2-linux-x64-5.1.21"),
		},
	})

	rel, err := latestRelease()
	require.NoError(err)

	require.Equal(unifiControllerProduct, rel.Product)
	require.Equal(version.Must(version.NewVersion("10.4.57")), rel.Version)
	require.Equal(mustParseURL(debURL), rel.URL)
	// The ID keeps build metadata and the firmware record id so a
	// republished artifact with the same core version is still detected.
	require.Equal("unifi-controller 10.4.57+atag-10.4.57-34628 fw-debian", rel.ID())
}

func TestLatestReleaseFallsBackToUOSServer(t *testing.T) {
	require := require.New(t)

	uosURL := "https://fw-download.ubnt.com/data/unifi-os-server/f5e2-linux-x64-5.1.21"
	fakeFirmwareServer(t, map[string][]firmwareUpdateApiResponseEmbeddedFirmware{
		// No debian build published any more.
		unifiControllerProduct: {
			firmwareResponse(unifiControllerProduct, "document", "10.4.57+atag-10.4.57-34628", ""),
		},
		uosServerProduct: {
			firmwareResponse(uosServerProduct, "linux-arm64", "5.1.21", ""),
			firmwareResponse(uosServerProduct, uosServerPlatform, "5.1.21", uosURL),
		},
	})

	rel, err := latestRelease()
	require.NoError(err)

	require.Equal(uosServerProduct, rel.Product)
	require.Equal(version.Must(version.NewVersion("5.1.21")), rel.Version)
	require.Equal(mustParseURL(uosURL), rel.URL)
	require.Equal("unifi-os-server 5.1.21 fw-linux-x64", rel.ID())
}
