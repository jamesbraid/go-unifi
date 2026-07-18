package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-version"
)

// osServerRelease describes a UniFi OS Server download published on the
// firmware update API.
type osServerRelease struct {
	Version *version.Version
	URL     *url.URL
	SHA256  string
}

func fetchFirmware(u *url.URL) (*firmwareUpdateApiResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var respData firmwareUpdateApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, err
	}
	return &respData, nil
}

// latestOsServerRelease returns the newest UniFi OS Server release for
// linux-x64 on the release channel.
func latestOsServerRelease() (*osServerRelease, error) {
	u, err := url.Parse(firmwareLatestApi)
	if err != nil {
		return nil, err
	}

	query := u.Query()
	query.Add("filter", firmwareUpdateApiFilter("eq", "channel", releaseChannel))
	query.Add("filter", firmwareUpdateApiFilter("eq", "product", osServerProduct))
	u.RawQuery = query.Encode()

	respData, err := fetchFirmware(u)
	if err != nil {
		return nil, err
	}

	return pickOsServerRelease(respData, nil)
}

// findOsServerRelease returns a specific UniFi OS Server release for
// linux-x64 (e.g. "5.1.21").
func findOsServerRelease(want string) (*osServerRelease, error) {
	wantV, err := version.NewVersion(want)
	if err != nil {
		return nil, fmt.Errorf("invalid unifi-os-server version %q: %w", want, err)
	}

	u, err := url.Parse(firmwareApi)
	if err != nil {
		return nil, err
	}

	query := u.Query()
	query.Add("filter", firmwareUpdateApiFilter("eq", "channel", releaseChannel))
	query.Add("filter", firmwareUpdateApiFilter("eq", "product", osServerProduct))
	query.Add("filter", firmwareUpdateApiFilter("eq", "platform", osServerPlatform))
	u.RawQuery = query.Encode()

	respData, err := fetchFirmware(u)
	if err != nil {
		return nil, err
	}

	return pickOsServerRelease(respData, wantV)
}

func pickOsServerRelease(respData *firmwareUpdateApiResponse, want *version.Version) (*osServerRelease, error) {
	for _, firmware := range respData.Embedded.Firmware {
		if firmware.Platform != osServerPlatform || firmware.Version == nil {
			continue
		}
		if want != nil && !firmware.Version.Equal(want) {
			continue
		}
		return &osServerRelease{
			Version: firmware.Version,
			URL:     firmware.Links.Data.Href,
			SHA256:  firmware.Sha256Checksum,
		}, nil
	}
	if want != nil {
		return nil, fmt.Errorf("unifi-os-server %s not found on firmware API", want)
	}
	return nil, fmt.Errorf("no unifi-os-server linux-x64 release found on firmware API")
}

func latestUnifiVersion() (*version.Version, *url.URL, error) {
	url, err := url.Parse(firmwareUpdateApi)
	if err != nil {
		return nil, nil, err
	}

	query := url.Query()
	query.Add("filter", firmwareUpdateApiFilter("eq", "channel", releaseChannel))
	query.Add("filter", firmwareUpdateApiFilter("eq", "product", unifiControllerProduct))
	query.Add("filter", firmwareUpdateApiFilter("lt", "version", maxVersion))
	url.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var respData firmwareUpdateApiResponse
	err = json.NewDecoder(resp.Body).Decode(&respData)
	if err != nil {
		return nil, nil, err
	}

	for _, firmware := range respData.Embedded.Firmware {
		if firmware.Platform != debianPlatform {
			continue
		}

		return firmware.Version.Core(), firmware.Links.Data.Href, nil
	}

	return nil, nil, nil
}
