package testenv

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

//go:embed demo-mode
var demoModeScript []byte

const (
	defaultImage = "jacobalberty/unifi:v10.0.162"
	// Simulation mode seeds this account (see demo-mode).
	demoUsername = "admin"
	demoPassword = "admin"
	demoSite     = "default"
)

type Controller struct {
	BaseURL  string
	Username string
	Password string
	Site     string
}

type config struct {
	Image  string
	PkgURL string
}

func configFromEnv() config {
	cfg := config{Image: defaultImage, PkgURL: os.Getenv("UNIFI_TEST_PKGURL")}
	if img := os.Getenv("UNIFI_TEST_IMAGE"); img != "" {
		cfg.Image = img
	}
	return cfg
}

// Start boots a disposable simulation-mode controller and returns its
// coordinates. It skips the test when docker is unavailable, honours
// UNIFI_TEST_URL to target an existing controller instead, and cleans the
// container up via t.Cleanup.
func Start(ctx context.Context, t *testing.T) *Controller {
	t.Helper()

	if url := os.Getenv("UNIFI_TEST_URL"); url != "" {
		return &Controller{BaseURL: strings.TrimRight(url, "/"), Username: demoUsername, Password: demoPassword, Site: demoSite}
	}

	skipUnlessDockerAvailable(ctx, t)

	cfg := configFromEnv()
	req := testcontainers.ContainerRequest{
		Image:        cfg.Image,
		ExposedPorts: []string{"8443/tcp"},
		Env: map[string]string{
			"UNIFI_STDOUT": "true",
			"TZ":           "Etc/UTC",
			"PKGURL":       cfg.PkgURL,
		},
		Files: []testcontainers.ContainerFile{{
			Reader:            bytes.NewReader(demoModeScript),
			ContainerFilePath: "/unifi/init.d/demo-mode",
			FileMode:          0o755,
		}},
		// TLS comes up before the app finishes booting; the real readiness
		// gate is the login poll below.
		WaitingFor: wait.ForListeningPort("8443/tcp").WithStartupTimeout(5 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start controller container: %v", err)
	}
	t.Cleanup(func() {
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

	c := &Controller{
		BaseURL:  fmt.Sprintf("https://%s:%s", host, port.Port()),
		Username: demoUsername,
		Password: demoPassword,
		Site:     demoSite,
	}

	// The seeded admin appears some time after the port opens; poll login.
	// One Session is reused across attempts: a stale cookie from a failed
	// login is harmless (each attempt is a fresh POST), and constructing
	// a fresh Transport+TLS+cookiejar every 3s would otherwise abandon up
	// to ~80 of them over the full deadline.
	deadline := time.Now().Add(4 * time.Minute)
	s := NewSession(c.BaseURL)
	for {
		err := s.Login(ctx, c.Username, c.Password)
		if err == nil {
			return c
		}
		if ctx.Err() != nil {
			t.Fatalf("controller never became ready: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("controller never became ready: %v", err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("controller never became ready: %v", ctx.Err())
		case <-time.After(3 * time.Second):
		}
	}
}

// skipUnlessDockerAvailable probes the docker daemon before doing any real
// work, so a missing/unreachable daemon is reported as a skip while every
// other failure (bad image, crashing container, ...) surfaces as a hard
// test failure instead of being mistaken for "docker isn't installed". On a
// machine with no docker host discoverable at all (no DOCKER_HOST, no
// /var/run/docker.sock, no testcontainers.properties),
// testcontainers.NewDockerProvider panics rather than returning an error, so
// the probe runs through dockerHealth, which recovers that panic and turns
// it into an error like any other "docker unavailable" case.
func skipUnlessDockerAvailable(ctx context.Context, t *testing.T) {
	t.Helper()

	if err := dockerHealth(ctx); err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
}

// dockerHealth constructs a docker provider and checks its health, recovering
// any panic from provider construction (e.g. testcontainers-go's
// NewDockerProvider, which calls core.MustExtractDockerHost and panics when
// no docker host can be found) and reporting it as an error instead.
func dockerHealth(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("docker provider panicked: %v", r)
		}
	}()

	provider, providerErr := testcontainers.NewDockerProvider()
	if providerErr != nil {
		return providerErr
	}
	return provider.Health(ctx)
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
