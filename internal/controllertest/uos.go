package controllertest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartForHarness boots the controller selected by UNIFI_TEST_HARNESS: "uos"
// boots UniFi OS Server (StartUOS), anything else — including unset — boots
// the standalone Network app -sim (Start). It lets one integration suite run
// against both harnesses by varying the env per CI job, so the encoder and
// drift checks run against the full UOS stack as well as the standalone
// controller. Tests that are inherently one-harness (e.g. the UOS gateway
// probe) call Start / StartUOS directly instead.
func StartForHarness(ctx context.Context, t *testing.T) *Controller {
	t.Helper()
	if strings.EqualFold(os.Getenv("UNIFI_TEST_HARNESS"), "uos") {
		return StartUOS(ctx, t)
	}
	return Start(ctx, t)
}

// MutatingHarness is the opening of a probe that writes to the controller: a
// disposable controller chosen by UNIFI_TEST_HARNESS, a logged-in raw session
// on it, and a context that expires after timeout.
//
// The UNIFI_TEST_URL skip is the part worth having in one place. These probes
// create, overwrite and delete site objects to find out what the controller
// does with them, so they are only safe on a controller the run owns and
// throws away; aimed at a real site they would rewrite it. One container per
// test function, too — several probes write site singletons and clean nothing
// up because the container is the cleanup, so nothing here may be shared
// between test functions.
//
// Probes that boot a specific harness (Start, StartUOS, StartUOSSeeded) or
// carry a second gate of their own keep their own preamble.
func MutatingHarness(t *testing.T, timeout time.Duration) (context.Context, *Controller, *Session) {
	t.Helper()
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	// t.Cleanup, not the caller's defer: cleanups run LIFO and this one is
	// registered before the boot's, so cancel happens after the container
	// teardown and after any delete a test registered with t.Cleanup. A
	// cancel that lands first turns those deletes into silent no-ops, and a
	// leaked object collides with the next candidate, which then reads as a
	// field verdict.
	t.Cleanup(cancel)
	c := StartForHarness(ctx, t)
	return ctx, c, c.NewSession(ctx, t)
}

const (
	// uosDefaultImage is the UniFi OS Server simulation image. Unlike the
	// standalone -sim network image it runs the full UOS stack (systemd,
	// ucore), so it is a higher-fidelity test target. It does NOT, on its
	// own, make the gateway-dependent features (BGP, firewall zones, NAT,
	// route-based IPsec) available: those need an adopted gateway device, and
	// UOS reports them unsupported exactly like the standalone controller
	// (verified 2026-07-22). Its version tracks UniFi OS Server, not the
	// Network app; bump alongside the UOS component the schemas came from.
	uosDefaultImage = "ghcr.io/jamesbraid/unifi-os-server:5.1.37-sim"

	// uosNetworkVersion is the Network app bundled in uosDefaultImage, which is
	// NOT schemas/VERSION and never can be: UniFi OS Server trails the
	// standalone .deb, so the newest UOS release embeds an older Network app
	// than the one the schemas were captured from. Bump both together.
	//
	// It exists so the UOS arm asserts something. Tying the check to
	// schemas/VERSION would fail permanently; leaving it unasserted is how the
	// pin sat two Network releases behind for weeks while the job stayed green.
	uosNetworkVersion = "10.5.67"

	// uosNetworkPort is where UOS_NETWORK_DIRECT (default in the -sim tags)
	// proxies the bundled Network Application API, bypassing UOS SSO. Plain
	// HTTP, unlike the standalone controller's self-signed HTTPS on 8443.
	uosNetworkPort = "7443/tcp"

	// uosSeededImage is UniFi OS Server with the setup wizard already
	// completed and an owner account seeded (admin/admin) — real mode, not
	// simulation. It has NO UOS_NETWORK_DIRECT, so its Network API is reached
	// only through the console proxy on 443: SSO login at /api/auth/login, a
	// CSRF header on writes, and every Network path under /proxy/network.
	uosSeededImage = "ghcr.io/jamesbraid/unifi-os-server:seeded"

	// uosConsolePort is the console's HTTPS port, which fronts both SSO and
	// the proxied Network API on the seeded image.
	uosConsolePort = "443/tcp"

	// uosNetworkPrefix is the console's path prefix for the bundled Network
	// application.
	uosNetworkPrefix = "/proxy/network"

	// The owner account the seeded image provisions (UOS_SEED_OWNER).
	uosSeededUsername = "admin"
	uosSeededPassword = "admin"
)

func uosSeededImageFromEnv() string {
	if img := os.Getenv("UNIFI_UOS_SEEDED_IMAGE"); img != "" {
		return img
	}
	return uosSeededImage
}

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

	maybeSkipDocker(t)

	// Resolved once and announced before the start, as the standalone path
	// does. The bundled Network version is named too: it is not
	// schemas/VERSION, so a transcript that shows only the image leaves a
	// reader to assume the locked controller produced the result.
	image := uosImageFromEnv()
	if image == uosDefaultImage {
		t.Logf("controller: %s (bundles Network %s)", image, uosNetworkVersion)
	} else {
		t.Logf("controller: %s (overridden; bundled Network version not known from here)", image)
	}

	req := testcontainers.ContainerRequest{
		Image:              image,
		ExposedPorts:       []string{uosNetworkPort},
		WaitingFor:         wait.ForHealthCheck().WithStartupTimeout(15 * time.Minute),
		HostConfigModifier: uosHostConfig,
	}

	c := startContainer(ctx, t, req, "UOS")

	host, port := mappedHostPort(ctx, t, c, "7443/tcp")

	return &Controller{
		BaseURL:  fmt.Sprintf("http://%s:%s", host, port),
		Username: demoUsername,
		Password: demoPassword,
		Site:     demoSite,
	}
}

// uosHostConfig applies the runtime contract systemd-as-PID-1 needs, from
// jamesbraid/unifi-containers' unifi-os compose: a capability list (no
// privileged mode), the host cgroup namespace with /sys/fs/cgroup mounted
// rw, and a tmpfs set. Dropping to this is what lets ucore/systemd come up.
func uosHostConfig(hc *container.HostConfig) {
	hc.CapDrop = []string{"ALL"}
	hc.CapAdd = []string{
		"SYS_ADMIN", "NET_ADMIN", "NET_RAW", "NET_BIND_SERVICE",
		"DAC_OVERRIDE", "DAC_READ_SEARCH", "FOWNER", "CHOWN",
		"SETUID", "SETGID", "KILL", "SYS_CHROOT", "SYS_PTRACE",
		"SYS_RESOURCE", "AUDIT_WRITE", "MKNOD",
	}
	hc.CgroupnsMode = "host"
	hc.Tmpfs = map[string]string{
		"/run":               "exec",
		"/run/lock":          "",
		"/tmp":               "exec",
		"/var/lib/journal":   "",
		"/var/opt/unifi/tmp": "size=64m",
	}
	hc.Binds = append(hc.Binds, "/sys/fs/cgroup:/sys/fs/cgroup:rw")
}

// StartUOSSeeded boots the seeded UniFi OS Server image — real mode with the
// setup wizard completed, as opposed to the simulation-mode images — and
// returns a controller pointed at its proxied Network API.
//
// Unlike StartUOS it owns a Docker network and an InformURL on it, which is
// what lets an emulated device adopt into a console: device containers join
// Network and inform the address InformURL carries, exactly as they do for the
// classic Start. Nothing is published to the host for the inform plane.
//
// The seeded image has no UOS_NETWORK_DIRECT, so everything goes through the
// console: SSO login, a CSRF header on writes, and Network paths under
// /proxy/network. Controller.RootURL carries the console root for the login,
// and NewSession picks the UniFi OS dialect off it.
func StartUOSSeeded(ctx context.Context, t *testing.T) *Controller {
	t.Helper()

	maybeSkipDocker(t)

	net := newDeviceNetwork(ctx, t, "UOS")

	req := testcontainers.ContainerRequest{
		Image: uosSeededImageFromEnv(),
		// Only the console is published, on an ephemeral port. The inform
		// port stays a container port reached across the network, and exactly
		// one network keeps ContainerIP unambiguous.
		ExposedPorts:       []string{uosConsolePort},
		Networks:           []string{net.Name},
		WaitingFor:         wait.ForHealthCheck().WithStartupTimeout(15 * time.Minute),
		HostConfigModifier: uosHostConfig,
	}

	c := startContainer(ctx, t, req, "seeded UOS")

	host, port := mappedHostPort(ctx, t, c, uosConsolePort)
	ip, err := c.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("UOS container IP: %v", err)
	}
	informURL, err := informURLFor(ip)
	if err != nil {
		t.Fatalf("seeded UOS on network %s: %v", net.Name, err)
	}

	root := fmt.Sprintf("https://%s:%s", host, port)
	return &Controller{
		BaseURL:   root + uosNetworkPrefix,
		RootURL:   root,
		Username:  uosSeededUsername,
		Password:  uosSeededPassword,
		Site:      demoSite,
		Network:   net.Name,
		InformURL: informURL,
	}
}
