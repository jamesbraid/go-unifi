//go:build integration

package controllertest

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegrationControllerBoots proves the harness end to end: container
// up, simulation admin seeded, classic API answering. Set
// UNIFI_TEST_EXPECT_VERSION to also prove a specific controller build was
// actually installed — without an explicit expectation, a stale or
// mistagged UNIFI_TEST_IMAGE can boot silently without anyone noticing the
// version drifted.
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

	if wantVersion := os.Getenv("UNIFI_TEST_EXPECT_VERSION"); wantVersion != "" {
		if runningVersion == "" {
			t.Errorf("cannot verify controller pin: UNIFI_TEST_EXPECT_VERSION=%q but sysinfo shape yielded no version", wantVersion)
		} else if runningVersion != wantVersion {
			t.Errorf("expected controller version %q (from UNIFI_TEST_EXPECT_VERSION) was not booted; controller reports %q", wantVersion, runningVersion)
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
