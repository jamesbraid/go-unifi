package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-version"
)

func latestUnifiVersion(ctx context.Context, client *http.Client, endpoint string) (*version.Version, *url.URL, error) {
	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, nil, err
	}

	query := url.Query()
	query.Add("filter", firmwareUpdateApiFilter("eq", "channel", releaseChannel))
	query.Add("filter", firmwareUpdateApiFilter("eq", "product", unifiControllerProduct))
	query.Add("filter", firmwareUpdateApiFilter("lt", "version", maxVersion))
	url.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("User-Agent", schemaFetcherUserAgent)
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("resolve legacy installer: unexpected HTTP status %s", resp.Status)
	}

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
