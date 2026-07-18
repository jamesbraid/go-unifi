package testenv

import "regexp"

// versionFromPkgURL extracts a dotted x.y.z version from a UniFi Network
// controller .deb artifact URL. Artifact filenames look like
// ".../fa30-debian-10.4.57-86432683-....deb"; the version is the dash-
// delimited numeric triple. Returns "" when no version can be parsed.
var pkgURLVersionRE = regexp.MustCompile(`-(\d+\.\d+\.\d+)-`)

func versionFromPkgURL(url string) string {
	m := pkgURLVersionRE.FindStringSubmatch(url)
	if m == nil {
		return ""
	}
	return m[1]
}
