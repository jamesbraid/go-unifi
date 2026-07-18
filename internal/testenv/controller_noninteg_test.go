//go:build !integration

package testenv

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestDockerHealthNoPanic pins the contract that dockerHealth converts
// testcontainers' discovery panic into a plain error: on a machine where no
// docker host is discoverable, testcontainers.NewDockerProvider panics via
// core.MustExtractDockerHost instead of returning an error, and before the
// recover in dockerHealth that panic crashed the integration suite instead
// of letting skipUnlessDockerAvailable skip.
//
// This file MUST keep the !integration build tag: testcontainers caches the
// discovered docker host in a package-level sync.Once, and the Once's done
// flag is set even when the wrapped discovery panics. If this test shared a
// test binary with the integration tests (plain `go test -tags integration
// ./internal/testenv/` runs files alphabetically, and this file sorts before
// testenv_integration_test.go), the scrubbed environment below would poison
// that cache and make TestIntegrationControllerBoots falsely skip with
// "docker unavailable" even on a machine with a working daemon.
func TestDockerHealthNoPanic(t *testing.T) {
	// Point discovery at paths that cannot exist so it fails regardless of
	// the machine's real docker setup. Depending on how testcontainers
	// resolves these, dockerHealth returns either a construction/health
	// error or the recovered panic — the assertion is only "an error, not
	// a panic".
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/claude-test.sock")
	t.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/nonexistent/claude-test-override.sock")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := dockerHealth(ctx)
	if err == nil {
		t.Fatal("dockerHealth succeeded against a nonexistent docker socket")
	}
	if strings.TrimSpace(err.Error()) == "" {
		t.Fatalf("dockerHealth returned an empty error: %#v", err)
	}
}
