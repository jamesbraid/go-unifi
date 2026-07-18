package main

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/go-version"
)

var firmwareLatestApi = "https://fw-update.ubnt.com/api/firmware-latest"
var firmwareApi = "https://fw-update.ubnt.com/api/firmware"

const (
	releaseChannel   = "release"
	osServerProduct  = "unifi-os-server"
	osServerPlatform = "linux-x64"
)

type firmwareUpdateApiResponse struct {
	Embedded firmwareUpdateApiResponseEmbedded `json:"_embedded"`
}

type firmwareUpdateApiResponseEmbedded struct {
	Firmware []firmwareUpdateApiResponseEmbeddedFirmware `json:"firmware"`
}

type firmwareUpdateApiResponseEmbeddedFirmware struct {
	Channel        string                                         `json:"channel"`
	Created        string                                         `json:"created"`
	Id             string                                         `json:"id"`
	Platform       string                                         `json:"platform"`
	Product        string                                         `json:"product"`
	Version        *version.Version                               `json:"version"`
	Sha256Checksum string                                         `json:"sha256_checksum"`
	FileSize       int64                                          `json:"file_size"`
	Links          firmwareUpdateApiResponseEmbeddedFirmwareLinks `json:"_links"`
}

type firmwareUpdateApiResponseEmbeddedFirmwareDataLink struct {
	Href *url.URL `json:"href"`
}

func (l firmwareUpdateApiResponseEmbeddedFirmwareDataLink) MarshalJSON() ([]byte, error) {
	var href string
	if l.Href != nil {
		href = l.Href.String()
	}

	aux := struct {
		Href string `json:"href"`
	}{
		Href: href,
	}

	return json.Marshal(aux)
}

func (l *firmwareUpdateApiResponseEmbeddedFirmwareDataLink) UnmarshalJSON(j []byte) error {
	var m map[string]any

	err := json.Unmarshal(j, &m)
	if err != nil {
		return err
	}

	if href := m["href"]; href != nil {
		url, err := url.Parse(href.(string))
		if err != nil {
			return err
		}

		l.Href = url
	}

	return nil
}

type firmwareUpdateApiResponseEmbeddedFirmwareLinks struct {
	Data firmwareUpdateApiResponseEmbeddedFirmwareDataLink `json:"data"`
}

func firmwareUpdateApiFilter(operator, key, value string) string {
	return fmt.Sprintf("%s~~%s~~%s", operator, key, value)
}
