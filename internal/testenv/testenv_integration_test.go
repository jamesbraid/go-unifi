//go:build integration

package testenv

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegrationControllerBoots proves the harness end to end: container
// up, simulation admin seeded, classic API answering. When
// UNIFI_TEST_PKGURL is set it also proves the requested controller build
// was actually installed — current jacobalberty/unifi images silently
// ignore PKGURL (docker-build.sh does not exist in the image), so without
// this check CI can log "pinning to 10.4.57" while quietly booting
// whatever build the image bundles.
func TestIntegrationControllerBoots(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	c := Start(ctx, t)
	s := c.NewSession(ctx, t)

	body, status, err := s.GetJSON(ctx, "/api/s/"+c.Site+"/stat/sysinfo")
	if err != nil || status != 200 {
		t.Fatalf("sysinfo: status=%d err=%v", status, err)
	}
	t.Logf("sysinfo: %#v", body)

	wrapped, ok := body.(map[string]any)
	if !ok || wrapped["data"] == nil {
		t.Fatalf("unexpected sysinfo shape: %#v", body)
	}

	runningVersion := sysinfoVersion(wrapped)
	t.Logf("controller version: %q", runningVersion)

	if pkgURL := os.Getenv("UNIFI_TEST_PKGURL"); pkgURL != "" {
		wantVersion := versionFromPkgURL(pkgURL)
		if wantVersion == "" {
			t.Logf("could not parse a version out of UNIFI_TEST_PKGURL=%q; skipping pin check", pkgURL)
		} else if runningVersion == "" {
			t.Errorf("controller pin check could not be performed: UNIFI_TEST_PKGURL specifies version %q but sysinfo response has no version field", wantVersion)
		} else if runningVersion != wantVersion {
			t.Errorf("controller pin not honoured: requested version %q (from UNIFI_TEST_PKGURL=%q) but controller reports %q; "+
				"current jacobalberty/unifi images cannot install a runtime PKGURL, so the container runs its bundled build",
				wantVersion, pkgURL, runningVersion)
		}
	}
}

// sysinfoVersion walks a decoded {"meta":...,"data":[{...,"version":"x"}]}
// sysinfo response and returns the version field of the first data entry,
// or "" if the shape doesn't match.
func sysinfoVersion(body map[string]any) string {
	data, ok := body["data"].([]any)
	if !ok || len(data) == 0 {
		return ""
	}
	entry, ok := data[0].(map[string]any)
	if !ok {
		return ""
	}
	version, _ := entry["version"].(string)
	return version
}
