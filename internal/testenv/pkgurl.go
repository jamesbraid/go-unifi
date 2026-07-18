package testenv

import (
	"regexp"
	"strings"
)

// versionFromPkgURL extracts a dotted x.y.z version from a UniFi Network
// controller .deb artifact URL. It extracts only the final path segment
// (filename) and finds the last occurrence of a version-like segment
// (dash-delimited numeric triple). Returns "" when no version can be parsed.
var pkgURLVersionRE = regexp.MustCompile(`-(\d+\.\d+\.\d+)`)

func versionFromPkgURL(url string) string {
	// Extract only the filename (final path segment after the last /)
	filename := url
	if lastSlash := strings.LastIndex(url, "/"); lastSlash >= 0 {
		filename = url[lastSlash+1:]
	}

	// Find all matches of the version pattern
	matches := pkgURLVersionRE.FindAllStringSubmatch(filename, -1)
	if len(matches) == 0 {
		return ""
	}

	// Return the capture group from the last match
	return matches[len(matches)-1][1]
}
