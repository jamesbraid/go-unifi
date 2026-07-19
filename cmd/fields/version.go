package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-version"
)

// releaseInfo describes the newest downloadable build of a product. For
// unifi-controller the version is the UniFi Network application version; for
// unifi-os-server it is the UniFi OS Server version and the Network version
// is only known after extracting the artifact.
type releaseInfo struct {
	Product string
	Version *version.Version
	// RawVersion keeps the full upstream version string (including build
	// metadata) and RecordID the firmware record id, so a republished
	// artifact with the same core version still changes the release ID.
	RawVersion string
	RecordID   string
	URL        *url.URL
}

// ID identifies a release for change detection (schemas/SOURCE).
func (r *releaseInfo) ID() string {
	id := fmt.Sprintf("%s %s", r.Product, r.RawVersion)
	if r.RecordID != "" {
		id += " " + r.RecordID
	}
	return id
}

// latestRelease finds the newest controller build published on the firmware
// update API. The standalone debian package names the Network app version
// directly and is much smaller than the UniFi OS Server installer, so it is
// preferred while Ubiquiti keeps shipping it; the UniFi OS Server installer
// is the fallback.
func latestRelease() (*releaseInfo, error) {
	rel, err := queryLatestFirmware(unifiControllerProduct, debianPlatform)
	if err != nil {
		return nil, err
	}
	if rel == nil {
		rel, err = queryLatestFirmware(uosServerProduct, uosServerPlatform)
		if err != nil {
			return nil, err
		}
	}
	if rel == nil {
		return nil, errors.New("no controller release found on the firmware update API")
	}

	return rel, nil
}

func queryLatestFirmware(product, platform string) (*releaseInfo, error) {
	url, err := url.Parse(firmwareUpdateApi)
	if err != nil {
		return nil, err
	}

	query := url.Query()
	query.Add("filter", firmwareUpdateApiFilter("eq", "channel", releaseChannel))
	query.Add("filter", firmwareUpdateApiFilter("eq", "product", product))
	url.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// A non-200 error body would decode as an empty firmware list, which is
	// indistinguishable from "product not published" and would silently
	// trigger the fallback product.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("firmware update API returned HTTP %s for %s/%s", resp.Status, product, platform)
	}

	var respData firmwareUpdateApiResponse
	err = json.NewDecoder(resp.Body).Decode(&respData)
	if err != nil {
		return nil, err
	}

	for _, firmware := range respData.Embedded.Firmware {
		if firmware.Platform != platform {
			continue
		}

		return &releaseInfo{
			Product:    product,
			Version:    firmware.Version.Core(),
			RawVersion: firmware.Version.Original(),
			RecordID:   firmware.Id,
			URL:        firmware.Links.Data.Href,
		}, nil
	}

	return nil, nil
}
