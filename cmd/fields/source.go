package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
)

type SourceKind string

const (
	SourceLegacyLatest  SourceKind = "legacy-latest"
	SourceLegacyVersion SourceKind = "legacy-version"
	SourceUOSLatest     SourceKind = "uos-latest"
	SourceUOSVersion    SourceKind = "uos-version"
	SourceInstallerURL  SourceKind = "installer-url"
	SourceInstallerFile SourceKind = "installer-file"

	schemaFetcherUserAgent = "go-unifi-schema-fetcher/1 (+https://github.com/ubiquiti-community/go-unifi; contact=https://github.com/ubiquiti-community/go-unifi/issues)"
	uosServerProduct       = "unifi-os-server"
	uosServerPlatform      = "linux-x64"
)

type SourceSelector struct {
	Kind  SourceKind
	Value string
}

type InstallerSource struct {
	Kind                                     SourceKind
	OSVersion, FirmwareID, Product, Platform string
	Channel                                  string
	URL                                      *url.URL
	LocalPath                                string
	ExpectedSize                             int64
	ExpectedSHA256, ExpectedMD5              string
	Created, Updated                         time.Time
}

func ParseSourceSelector(args []string) (SourceSelector, error) {
	fs := flag.NewFlagSet("source", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	latest := fs.Bool("latest", false, "")
	uosLatest := fs.Bool("uos-latest", false, "")
	uosVersion := fs.String("uos-version", "", "")
	installer := fs.String("installer", "", "")
	installerURL := fs.String("installer-url", "", "")
	if err := fs.Parse(args); err != nil {
		return SourceSelector{}, fmt.Errorf("parse source selector: %w", err)
	}

	selectors := make([]SourceSelector, 0, 2)
	if *latest {
		selectors = append(selectors, SourceSelector{Kind: SourceLegacyLatest})
	}
	if *uosLatest {
		selectors = append(selectors, SourceSelector{Kind: SourceUOSLatest})
	}
	if *uosVersion != "" {
		selectors = append(selectors, SourceSelector{Kind: SourceUOSVersion, Value: *uosVersion})
	}
	if *installer != "" {
		selectors = append(selectors, SourceSelector{Kind: SourceInstallerFile, Value: *installer})
	}
	if *installerURL != "" {
		selectors = append(selectors, SourceSelector{Kind: SourceInstallerURL, Value: *installerURL})
	}
	if fs.NArg() == 1 {
		selectors = append(selectors, SourceSelector{Kind: SourceLegacyVersion, Value: fs.Arg(0)})
	} else if fs.NArg() > 1 {
		return SourceSelector{}, errors.New("source selector accepts at most one positional version")
	}

	if len(selectors) != 1 {
		return SourceSelector{}, errors.New("exactly one source selector is required")
	}
	return selectors[0], nil
}

func ValidateInstallerURL(u *url.URL) error {
	if u == nil {
		return errors.New("installer URL is missing")
	}
	if u.Scheme != "https" {
		return errors.New("installer URL must use HTTPS")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || !allowedUbiquitiHost(host) {
		return fmt.Errorf("installer URL host %q is not allowed", host)
	}
	return nil
}

func allowedUbiquitiHost(host string) bool {
	for _, domain := range []string{"ui.com", "ubnt.com"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func ResolveInstaller(ctx context.Context, client *http.Client, endpoint string, selector SourceSelector) (InstallerSource, error) {
	if client == nil {
		client = http.DefaultClient
	}

	switch selector.Kind {
	case SourceLegacyLatest:
		v, downloadURL, err := latestUnifiVersion(ctx, client, endpoint)
		if err != nil {
			return InstallerSource{}, err
		}
		if v == nil || downloadURL == nil {
			return InstallerSource{}, errors.New("legacy installer was not found")
		}
		return InstallerSource{Kind: selector.Kind, OSVersion: v.String(), URL: downloadURL}, nil
	case SourceLegacyVersion:
		v, err := version.NewVersion(selector.Value)
		if err != nil {
			return InstallerSource{}, fmt.Errorf("invalid legacy version: %w", err)
		}
		downloadURL, err := url.Parse(fmt.Sprintf("https://dl.ui.com/unifi/%s/unifi_sysvinit_all.deb", v))
		if err != nil {
			return InstallerSource{}, fmt.Errorf("build legacy installer URL: %w", err)
		}
		return InstallerSource{Kind: selector.Kind, OSVersion: v.String(), URL: downloadURL}, nil
	case SourceInstallerFile:
		if selector.Value == "" {
			return InstallerSource{}, errors.New("installer path is empty")
		}
		return InstallerSource{Kind: selector.Kind, LocalPath: selector.Value}, nil
	case SourceInstallerURL:
		downloadURL, err := url.Parse(selector.Value)
		if err != nil {
			return InstallerSource{}, fmt.Errorf("parse installer URL: %w", err)
		}
		if err := ValidateInstallerURL(downloadURL); err != nil {
			return InstallerSource{}, err
		}
		return InstallerSource{Kind: selector.Kind, URL: downloadURL}, nil
	case SourceUOSLatest, SourceUOSVersion:
		return resolveUOSInstaller(ctx, client, endpoint, selector)
	default:
		return InstallerSource{}, fmt.Errorf("unsupported source kind %q", selector.Kind)
	}
}

func resolveUOSInstaller(ctx context.Context, client *http.Client, endpoint string, selector SourceSelector) (InstallerSource, error) {
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return InstallerSource{}, fmt.Errorf("parse firmware endpoint: %w", err)
	}
	query := endpointURL.Query()
	query.Add("filter", firmwareUpdateApiFilter("eq", "product", uosServerProduct))
	query.Add("filter", firmwareUpdateApiFilter("eq", "platform", uosServerPlatform))
	query.Add("filter", firmwareUpdateApiFilter("eq", "channel", releaseChannel))
	wantedVersion := strings.TrimPrefix(selector.Value, "v")
	if selector.Kind == SourceUOSVersion {
		if wantedVersion == "" {
			return InstallerSource{}, errors.New("UniFi OS version is empty")
		}
		query.Add("filter", firmwareUpdateApiFilter("eq", "version", "v"+wantedVersion))
	}
	endpointURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL.String(), nil)
	if err != nil {
		return InstallerSource{}, fmt.Errorf("create firmware request: %w", err)
	}
	req.Header.Set("User-Agent", schemaFetcherUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return InstallerSource{}, fmt.Errorf("resolve installer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return InstallerSource{}, fmt.Errorf("resolve installer: unexpected HTTP status %s", resp.Status)
	}

	var response firmwareUpdateApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return InstallerSource{}, fmt.Errorf("decode firmware response: %w", err)
	}
	if len(response.Embedded.Firmware) != 1 {
		return InstallerSource{}, fmt.Errorf("expected exactly one firmware result, got %d", len(response.Embedded.Firmware))
	}
	record := response.Embedded.Firmware[0]
	if record.Product != uosServerProduct || record.Platform != uosServerPlatform || record.Channel != releaseChannel {
		return InstallerSource{}, errors.New("firmware result does not match requested product, platform, and channel")
	}
	if record.Version == nil {
		return InstallerSource{}, errors.New("firmware result has no version")
	}
	resolvedVersion := strings.TrimPrefix(record.Version.String(), "v")
	if selector.Kind == SourceUOSVersion && resolvedVersion != wantedVersion {
		return InstallerSource{}, fmt.Errorf("firmware result version %q does not match %q", resolvedVersion, wantedVersion)
	}
	if record.Id == "" || record.FileSize <= 0 || record.Links.Data.Href == nil {
		return InstallerSource{}, errors.New("firmware result is incomplete")
	}
	if err := validateHexDigest(record.SHA256Checksum, 32); err != nil {
		return InstallerSource{}, fmt.Errorf("invalid firmware SHA-256: %w", err)
	}
	if record.MD5 != "" {
		if err := validateHexDigest(record.MD5, 16); err != nil {
			return InstallerSource{}, fmt.Errorf("invalid firmware MD5: %w", err)
		}
	}
	if err := ValidateInstallerURL(record.Links.Data.Href); err != nil {
		return InstallerSource{}, fmt.Errorf("invalid firmware data URL: %w", err)
	}
	created, err := time.Parse(time.RFC3339, record.Created)
	if err != nil {
		return InstallerSource{}, fmt.Errorf("invalid firmware creation time: %w", err)
	}
	updated, err := time.Parse(time.RFC3339, record.Updated)
	if err != nil {
		return InstallerSource{}, fmt.Errorf("invalid firmware update time: %w", err)
	}

	return InstallerSource{
		Kind:           selector.Kind,
		OSVersion:      resolvedVersion,
		FirmwareID:     record.Id,
		Product:        record.Product,
		Platform:       record.Platform,
		Channel:        record.Channel,
		URL:            record.Links.Data.Href,
		ExpectedSize:   record.FileSize,
		ExpectedSHA256: strings.ToLower(record.SHA256Checksum),
		ExpectedMD5:    strings.ToLower(record.MD5),
		Created:        created,
		Updated:        updated,
	}, nil
}

func validateHexDigest(value string, byteLength int) error {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return err
	}
	if len(decoded) != byteLength {
		return fmt.Errorf("expected %d bytes, got %d", byteLength, len(decoded))
	}
	return nil
}
