package testenv

import (
	"net/url"
	"regexp"
	"strings"
)

// versionFromPkgURL extracts a dotted x.y.z version from a UniFi Network
// controller package URL. Two shapes are supported:
//
//  1. fw-download-style filenames that embed the version as a
//     dash-delimited numeric triple somewhere in the final path segment,
//     e.g. ".../fa30-debian-10.4.57-<uuid>.deb". When several such matches
//     exist (e.g. a leftmost build-ID triple), the last one wins.
//  2. The canonical dl.ui.com download shape, which instead carries the
//     version as its own path segment (a directory), e.g.
//     "https://dl.ui.com/unifi/10.4.57/unifi_sysvinit_all.deb". This is
//     only consulted when the filename heuristic above finds nothing.
//
// Any query string or fragment is ignored. Returns "" when no version can
// be parsed by either heuristic.
var pkgURLVersionRE = regexp.MustCompile(`-(\d+\.\d+\.\d+)`)
var pkgURLFullSegmentVersionRE = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func versionFromPkgURL(rawURL string) string {
	path := rawURL
	if parsed, err := url.Parse(rawURL); err == nil {
		path = parsed.Path
	}

	segments := strings.Split(path, "/")

	// Heuristic 1: the filename (final path segment).
	filename := segments[len(segments)-1]
	if matches := pkgURLVersionRE.FindAllStringSubmatch(filename, -1); len(matches) > 0 {
		return matches[len(matches)-1][1]
	}

	// Heuristic 2: a directory segment that is exactly a x.y.z version. If
	// more than one segment qualifies, the last one wins.
	var version string
	for _, segment := range segments[:len(segments)-1] {
		if pkgURLFullSegmentVersionRE.MatchString(segment) {
			version = segment
		}
	}
	return version
}
