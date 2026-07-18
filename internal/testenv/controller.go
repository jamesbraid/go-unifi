package testenv

import (
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
			Reader:            strings.NewReader(string(demoModeScript)),
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
		t.Skipf("unable to start controller container (docker unavailable?): %v", err)
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
	deadline := time.Now().Add(4 * time.Minute)
	for {
		s := NewSession(c.BaseURL)
		if err := s.Login(ctx, c.Username, c.Password); err == nil {
			return c
		} else if time.Now().After(deadline) {
			t.Fatalf("controller never became ready: %v", err)
		}
		time.Sleep(3 * time.Second)
	}
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
