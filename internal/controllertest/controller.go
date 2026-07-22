package controllertest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// defaultImage is the published multi-arch simulation image of the
	// exact build the schemas came from (built by
	// github.com/jamesbraid/unifi-containers). Bump the tag when
	// schemas/VERSION bumps. CI derives its tag from the marker so only
	// bare local runs depend on this constant.
	defaultImage = "ghcr.io/jamesbraid/unifi-network:10.4.57-sim"
	// Simulation mode seeds this account.
	demoUsername = "admin"
	demoPassword = "admin"
	demoSite     = "default"
)

type Controller struct {
	BaseURL  string
	Username string
	Password string
	Site     string
	// InformURL is the controller's device inform endpoint, reachable
	// from the test host — an in-process device simulator informs here.
	// Only the classic Start sets it: the container's 8080 is pinned to
	// host 127.0.0.1:8080 and the URL is built from the container host,
	// like BaseURL. SYSTEM_IP makes the controller advertise 127.0.0.1
	// to adopted devices — the same place under the local-daemon
	// assumption (see Start), so the post-adoption inform target keeps
	// working. It is empty for UNIFI_TEST_URL targets (an external
	// controller's inform plane is not the suite's to point devices at)
	// and for UOS harness controllers.
	InformURL string
}

func imageFromEnv() string {
	if img := os.Getenv("UNIFI_TEST_IMAGE"); img != "" {
		return img
	}
	return defaultImage
}

// Start boots a disposable simulation-mode controller and returns its
// coordinates. It skips the test when docker is unavailable, honours
// UNIFI_TEST_URL to target an existing controller instead, and cleans the
// container up via t.Cleanup. The image must be a simulation-mode
// controller image (admin/admin seeded, no setup wizard, healthcheck that
// signals API readiness) — the published `-sim` tags, or anything
// honoring the same contract.
//
// The inform port is pinned to host 127.0.0.1:8080 (SYSTEM_IP advertises
// 127.0.0.1, the same host loopback InformURL is built from) so an
// in-process device simulator has a stable, host-reachable inform target
// — container IPs are unroutable from the macOS host. The tradeoff: host
// port 8080 is always bound while a classic controller runs, so two of
// them cannot coexist; never parallelize classic-controller tests within
// a package, and pass -p 1 to multi-package runs locally (go test -tags
// integration ./... boots packages concurrently, and the second 8080
// bind dies "port is already allocated"; CI already passes -p 1). A
// local docker daemon is assumed — 127.0.0.1 means nothing on a remote
// one.
func Start(ctx context.Context, t *testing.T) *Controller {
	t.Helper()

	if target := os.Getenv("UNIFI_TEST_URL"); target != "" {
		c := &Controller{BaseURL: strings.TrimRight(target, "/"), Username: demoUsername, Password: demoPassword, Site: demoSite}
		// A URL target may still be booting (CI service containers start
		// alongside the test step with nothing gating on their health), and
		// there is no image healthcheck to lean on here, so readiness is a
		// bounded login poll.
		waitReady(ctx, t, c)
		return c
	}

	// Skip is reserved for a genuinely unreachable docker daemon; any
	// failure past this probe (bad image, crashing boot) fails the test
	// rather than skipping green. UNIFI_TEST_REQUIRE (set by CI) disables
	// even that skip: a required gate run without docker must go red, never
	// green.
	if os.Getenv("UNIFI_TEST_REQUIRE") == "" {
		testcontainers.SkipIfProviderIsNotHealthy(t)
	}

	req := testcontainers.ContainerRequest{
		Image:        imageFromEnv(),
		ExposedPorts: []string{"8443/tcp", "8080/tcp"},
		Env: map[string]string{
			"UNIFI_STDOUT": "true",
			"TZ":           "Etc/UTC",
			// The address the controller advertises as its inform/adopt
			// target (the -sim entrypoint writes it to system.properties).
			// Adopted devices re-inform there, so it must match the
			// host-reachable address the pin below creates.
			"SYSTEM_IP": "127.0.0.1",
		},
		// 8443 (the API) stays ephemeral; 8080 (inform) is pinned to the
		// host loopback so an in-process device simulator informs a stable
		// address. testcontainers has no fixed-binding ExposedPorts syntax,
		// but its merge keeps HostConfig bindings for exposed ports.
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{
				network.MustParsePort("8080/tcp"): {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "8080"}},
			}
		},
		// The -sim images' healthcheck reports healthy only once the API
		// answers a real JSON login (the controller serves an HTML
		// placeholder with HTTP 200 on every path early in boot, so port
		// or status probes lie); waiting on it IS the readiness gate.
		WaitingFor: wait.ForHealthCheck().WithStartupTimeout(5 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		// The wait error alone ("container is not healthy") says nothing
		// about why; the controller's own output does (UNIFI_STDOUT routes
		// it to the container log).
		dumpLogs(ctx, t, container)
		t.Fatalf("start controller container: %v", err)
	}
	t.Cleanup(func() {
		if os.Getenv("UNIFI_TEST_KEEP") != "" {
			t.Logf("UNIFI_TEST_KEEP set; leaving controller running at %s", container.GetContainerID())
			return
		}
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "8443/tcp")
	if err != nil {
		t.Fatalf("mapped port: %v", err)
	}

	return &Controller{
		BaseURL:   fmt.Sprintf("https://%s:%s", host, port.Port()),
		Username:  demoUsername,
		Password:  demoPassword,
		Site:      demoSite,
		InformURL: fmt.Sprintf("http://%s:8080/inform", host),
	}
}

// readyPollInterval is a variable so unit tests can poll fast.
var readyPollInterval = 3 * time.Second

// waitReady login-polls an existing controller until it answers a real
// JSON login. Connection errors and non-JSON bodies (the boot placeholder
// answers HTTP 200 HTML on every path) mean "still booting" and retry;
// anything else — an HTTP error status or a JSON rc != "ok" — is a real
// rejection and fails immediately.
func waitReady(ctx context.Context, t *testing.T, c *Controller) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Minute)
	s := NewSession(c.BaseURL)
	for {
		err := s.Login(ctx, c.Username, c.Password)
		if err == nil {
			return
		}
		var uerr *url.Error
		if !errors.Is(err, ErrNotJSON) && !errors.As(err, &uerr) {
			t.Fatalf("login to %s: %v", c.BaseURL, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("controller at %s never became ready: %v", c.BaseURL, err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("controller at %s never became ready: %v", c.BaseURL, ctx.Err())
		case <-time.After(readyPollInterval):
		}
	}
}

// dumpLogs surfaces the tail of the controller's own output on startup
// failure.
func dumpLogs(ctx context.Context, t *testing.T, container testcontainers.Container) {
	t.Helper()

	if container == nil {
		return
	}
	rc, err := container.Logs(ctx)
	if err != nil {
		return
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil || len(raw) == 0 {
		return
	}
	const tail = 16 << 10
	if len(raw) > tail {
		raw = raw[len(raw)-tail:]
	}
	t.Logf("controller log tail:\n%s", raw)
}

// NewSession returns a logged-in raw session against the controller.
func (c *Controller) NewSession(ctx context.Context, t *testing.T) *Session {
	t.Helper()
	s := NewSession(c.BaseURL)
	if err := s.Login(ctx, c.Username, c.Password); err != nil {
		t.Fatalf("login to %s: %v", c.BaseURL, err)
	}
	return s
}
