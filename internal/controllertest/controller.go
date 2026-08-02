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

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
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

	// informPort is the controller's device inform port. It is a container
	// port only: nothing publishes it to the host, because devices reach it
	// across the controller's own Docker network.
	informPort = 8080
)

type Controller struct {
	BaseURL  string
	Username string
	Password string
	Site     string
	// Network is the user-defined Docker network this controller sits on,
	// and the one device containers must join to reach InformURL. Only the
	// classic Start sets it; it is empty for UNIFI_TEST_URL targets and for
	// UOS harness controllers, which own neither a network nor an inform
	// plane the suite may point devices at.
	Network string
	// InformURL is the controller's device inform endpoint as seen from
	// Network — http://<container IPv4>:8080/inform. It is deliberately not
	// host-reachable: a device is a container beside the controller, not a
	// process on the test host, so the address that matters is the one on
	// the shared network. Empty whenever Network is (see above).
	InformURL string
}

// informURLFor builds the device inform endpoint for a controller reachable
// at ip. The form is exactly http://<canonical-IPv4-literal>:8080/inform,
// and the narrowness is the contract rather than fussiness: device
// containers resolve no names, and the controller rejects an inform_ip that
// is not an IP literal post-adopt ("invalid inform_ip localhost", HTTP 400).
// A hostname, a loopback address or an IPv6 literal all produce a fleet that
// starts cleanly and never adopts, so they fail here instead — while the
// problem is still one line of fixture configuration.
func informURLFor(ip string) (string, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return "", fmt.Errorf("inform host %q is not an IP literal: %w", ip, err)
	}
	if !addr.Is4() {
		return "", fmt.Errorf("inform host %q is not IPv4", ip)
	}
	// A canonical literal round-trips. Anything else — a leading-zero octet,
	// an IPv4-mapped IPv6 spelling — would reach the controller as a
	// different string than the one it compares against.
	if addr.String() != ip {
		return "", fmt.Errorf("inform host %q is not canonical IPv4 text", ip)
	}
	if addr.IsLoopback() || addr.IsUnspecified() {
		return "", fmt.Errorf("inform host %q is not reachable from a device container", ip)
	}
	return fmt.Sprintf("http://%s:%d/inform", ip, informPort), nil
}

func imageFromEnv() string {
	if img := os.Getenv("UNIFI_TEST_IMAGE"); img != "" {
		return img
	}
	return defaultImage
}

// Start boots a disposable simulation-mode controller on a user-defined
// Docker network of its own and returns its coordinates. It skips the test
// when docker is unavailable, honours UNIFI_TEST_URL to target an existing
// controller instead, and cleans the container and the network up via
// t.Cleanup. The image must be a simulation-mode controller image
// (admin/admin seeded, no setup wizard, healthcheck that signals API
// readiness) — the published `-sim` tags, or anything honoring the same
// contract.
//
// The network is the fixture's half of the device contract: device
// containers join it and inform the controller at its address on it, so
// nothing about the inform plane touches the host. That is why 8080 is no
// longer published and SYSTEM_IP is no longer set — the controller
// auto-detects its own address on its single interface, which is exactly
// the address InformURL carries, so the inform target it advertises
// post-adopt keeps working. Both host-global constraints the old fixed
// 127.0.0.1:8080 binding imposed are gone with it: controllers no longer
// collide on a host port, and no local-docker-daemon assumption remains.
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

	// The network is created before the controller so t.Cleanup tears them
	// down in the right order: cleanups run LIFO, so the container is
	// removed first and the network is free to go afterwards.
	net, err := network.New(ctx)
	if err != nil {
		t.Fatalf("create controller network: %v", err)
	}
	t.Cleanup(func() {
		if os.Getenv("UNIFI_TEST_KEEP") != "" {
			t.Logf("UNIFI_TEST_KEEP set; leaving network %s in place", net.Name)
			return
		}
		// A network that will not go away means something is still attached
		// to it, which the next run inherits as an ever-growing pile of
		// networks. Discarding the error would hide exactly that.
		if err := net.Remove(context.Background()); err != nil {
			t.Errorf("remove controller network %s: %v", net.Name, err)
		}
	})

	req := testcontainers.ContainerRequest{
		Image: imageFromEnv(),
		// Only the API is published, and only on an ephemeral port: the
		// test host drives the controller over BaseURL, while 8080 (inform)
		// is reached across the network below and needs no host binding at
		// all. Exactly one network keeps ContainerIP unambiguous.
		ExposedPorts: []string{"8443/tcp"},
		Networks:     []string{net.Name},
		Env: map[string]string{
			"UNIFI_STDOUT": "true",
			"TZ":           "Etc/UTC",
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
	// Registered before the error is checked, and on purpose: a container
	// that failed its health wait is returned started, not absent (which is
	// why dumpLogs below can read its log). Registering after the check
	// would mean t.Fatalf skipped it, stranding the container — and with it
	// the network, whose removal then fails on the still-attached endpoint.
	if container != nil {
		t.Cleanup(func() {
			if os.Getenv("UNIFI_TEST_KEEP") != "" {
				t.Logf("UNIFI_TEST_KEEP set; leaving controller running at %s", container.GetContainerID())
				return
			}
			if err := container.Terminate(context.Background()); err != nil {
				t.Errorf("terminate controller container: %v", err)
			}
		})
	}
	if err != nil {
		// The wait error alone ("container is not healthy") says nothing
		// about why; the controller's own output does (UNIFI_STDOUT routes
		// it to the container log).
		dumpLogs(ctx, t, container)
		t.Fatalf("start controller container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "8443/tcp")
	if err != nil {
		t.Fatalf("mapped port: %v", err)
	}

	// ContainerIP reports an address only while the container sits on
	// exactly one network, which is the shape the request asks for — so an
	// empty answer here means the topology drifted, not that the address is
	// unknown, and informURLFor turns it into a named failure.
	ip, err := container.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("controller container IP: %v", err)
	}
	informURL, err := informURLFor(ip)
	if err != nil {
		t.Fatalf("controller on network %s: %v", net.Name, err)
	}

	return &Controller{
		BaseURL:   fmt.Sprintf("https://%s:%s", host, port.Port()),
		Username:  demoUsername,
		Password:  demoPassword,
		Site:      demoSite,
		Network:   net.Name,
		InformURL: informURL,
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
