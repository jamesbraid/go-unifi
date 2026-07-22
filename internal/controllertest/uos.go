package controllertest

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// uosDefaultImage is the UniFi OS Server simulation image. Unlike the
	// standalone -sim network image, it runs the full UOS stack (systemd,
	// ucore), which is the harness we reach for when a probe needs
	// gateway-dependent features (BGP, firewall zones, NAT, route-based
	// IPsec) the standalone controller reports as unsupported. Its version
	// tracks UniFi OS Server, not the Network app; bump alongside the UOS
	// component the schemas came from.
	uosDefaultImage = "ghcr.io/jamesbraid/unifi-os-server:5.1.21-sim"

	// uosNetworkPort is where UOS_NETWORK_DIRECT (default in the -sim tags)
	// proxies the bundled Network Application API, bypassing UOS SSO. Plain
	// HTTP, unlike the standalone controller's self-signed HTTPS on 8443.
	uosNetworkPort = "7443/tcp"
)

func uosImageFromEnv() string {
	if img := os.Getenv("UNIFI_UOS_IMAGE"); img != "" {
		return img
	}
	return uosDefaultImage
}

// StartUOS boots a UniFi OS Server simulation container and returns a
// controller pointed at its bundled Network Application API. UOS runs
// systemd as PID 1, so it needs the exact runtime contract from
// jamesbraid/unifi-containers' unifi-os/examples/docker-compose.yml: a
// capability list (no privileged mode), the host cgroup namespace with
// /sys/fs/cgroup mounted rw, and a tmpfs set. The image's healthcheck
// reports healthy once the direct Network API answers a JSON login, so
// wait.ForHealthCheck is the readiness gate — but budget minutes: UOS is a
// heavy boot next to the standalone sim's ~30s (~42s on native arm64, but
// jamesbraid/unifi-containers' own GitHub Actions health gate budgets 900s
// on a hosted ubuntu-24.04 amd64 runner, so match that ceiling).
//
// Skip is reserved for a genuinely unreachable docker daemon;
// UNIFI_TEST_REQUIRE (set by CI) disables even that so a required gate goes
// red rather than green when docker is missing.
func StartUOS(ctx context.Context, t *testing.T) *Controller {
	t.Helper()

	if os.Getenv("UNIFI_TEST_REQUIRE") == "" {
		testcontainers.SkipIfProviderIsNotHealthy(t)
	}

	// The exact contract systemd-as-PID-1 needs; see the unifi-os compose in
	// jamesbraid/unifi-containers. Dropping to this cap list (no privileged
	// mode) plus a host cgroup namespace is what lets ucore/systemd come up.
	caps := []string{
		"SYS_ADMIN", "NET_ADMIN", "NET_RAW", "NET_BIND_SERVICE",
		"DAC_OVERRIDE", "DAC_READ_SEARCH", "FOWNER", "CHOWN",
		"SETUID", "SETGID", "KILL", "SYS_CHROOT", "SYS_PTRACE",
		"SYS_RESOURCE", "AUDIT_WRITE", "MKNOD",
	}
	tmpfs := map[string]string{
		"/run":               "exec",
		"/run/lock":          "",
		"/tmp":               "exec",
		"/var/lib/journal":   "",
		"/var/opt/unifi/tmp": "size=64m",
	}

	req := testcontainers.ContainerRequest{
		Image:        uosImageFromEnv(),
		ExposedPorts: []string{uosNetworkPort},
		WaitingFor:   wait.ForHealthCheck().WithStartupTimeout(15 * time.Minute),
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.CapDrop = []string{"ALL"}
			hc.CapAdd = caps
			hc.CgroupnsMode = "host"
			hc.Tmpfs = tmpfs
			hc.Binds = append(hc.Binds, "/sys/fs/cgroup:/sys/fs/cgroup:rw")
		},
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		dumpLogs(ctx, t, c)
		t.Fatalf("start UOS container: %v", err)
	}
	t.Cleanup(func() {
		if os.Getenv("UNIFI_TEST_KEEP") != "" {
			t.Logf("UNIFI_TEST_KEEP set; leaving UOS running at %s", c.GetContainerID())
			return
		}
		_ = c.Terminate(context.Background())
	})

	host, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := c.MappedPort(ctx, "7443/tcp")
	if err != nil {
		t.Fatalf("mapped port: %v", err)
	}

	return &Controller{
		BaseURL:  fmt.Sprintf("http://%s:%s", host, port.Port()),
		Username: demoUsername,
		Password: demoPassword,
		Site:     demoSite,
	}
}
