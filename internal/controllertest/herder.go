package controllertest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The herder is consumed as a process, not as a package: its Go API is
// deliberately internal to unifi-emu, and its supported integration surface
// is the foreground NDJSON protocol on stdout. Both coordinates therefore
// come from the environment rather than from go.mod.
//
// A release herder carries a version-matched synthetic image, so the image
// variable is optional; a development build has no default and needs one.
const (
	herderBinEnv   = "UNIFI_TEST_HERDER_BIN"
	herderImageEnv = "UNIFI_TEST_HERDER_SYNTHETIC_IMAGE"
)

// Variables rather than constants so the process-level tests can drive a
// stand-in herder on a short clock instead of a real one.
var (
	// herderStartupTimeout and herderStopTimeout are passed to the child, so
	// the fixture and the herder are working to the same clock.
	herderStartupTimeout = 5 * time.Minute
	herderStopTimeout    = 30 * time.Second
	// herderSlack keeps every fixture-side wait strictly longer than the
	// child's own deadline for the same phase. A stuck run is then reported
	// by the herder as the failure it is — with a code and a phase — rather
	// than by the fixture as an unexplained timeout.
	herderSlack = 30 * time.Second
	// herderKillGrace bounds the wait after SIGKILL. Killing a process
	// normally closes its pipe ends at once, so this only ever elapses when
	// something else is holding them.
	herderKillGrace = 5 * time.Second
)

// DeviceRequest is one device to start. Only Model is required; anything left
// unset is allocated by the herder, and the wire form omits it so the herder
// can tell "allocate this" from "use this". There is deliberately no address
// field: Docker decides it once the container joins the network.
type DeviceRequest struct {
	Model  string `json:"model"`
	MAC    string `json:"mac,omitempty"`
	Serial string `json:"serial,omitempty"`
	Name   string `json:"name,omitempty"`
}

// EmulatedDevice is one device the herder resolved and started, as its ready
// event published it.
//
// IP is the address Docker assigned, read back by inspection. It is NOT a
// device identity: devices batched into one synthetic container share a
// single IP by design, and the controller keys on MAC. Adopt by MAC.
type EmulatedDevice struct {
	Index  int    `json:"index"`
	Model  string `json:"model"`
	MAC    string `json:"mac"`
	Serial string `json:"serial"`
	Name   string `json:"name"`
	IP     string `json:"ip"`
}

// deviceRequestDocument is the single strict JSON document the herder reads
// from stdin. Its decoder rejects unknown fields and a second document, so
// the encoding here is exact rather than convenient.
type deviceRequestDocument struct {
	Version int             `json:"version"`
	Devices []DeviceRequest `json:"devices"`
}

// encodeDeviceRequest writes one device request to w.
func encodeDeviceRequest(w io.Writer, devices []DeviceRequest) error {
	return json.NewEncoder(w).Encode(deviceRequestDocument{Version: 1, Devices: devices})
}

// herderEvent is the subset of a protocol-1 event this fixture reads. Every
// event decodes into the one shape: the fields an event does not carry stay
// zero, which is cheaper than a discriminated union for four event kinds.
type herderEvent struct {
	Protocol        int              `json:"protocol"`
	Event           string           `json:"event"`
	RunID           string           `json:"run_id"`
	Devices         []EmulatedDevice `json:"devices"`
	Phase           string           `json:"phase"`
	Code            string           `json:"code"`
	Message         string           `json:"message"`
	Reason          string           `json:"reason"`
	CleanupComplete bool             `json:"cleanup_complete"`
}

// herderStream is the fixture's view of one herder run's stdout: the
// milestones it reached and their payloads.
//
// It records rather than delivers. The decode loop must never block — a full
// stdout pipe deadlocks the child mid-run — so every event is stored under
// the mutex and waiters are released by closing a channel, which no reader
// can stall.
type herderStream struct {
	mu       sync.Mutex
	started  *herderEvent
	ready    *herderEvent
	terminal *herderEvent
	// protoErr latches the first violation of the stdout contract — an
	// undecodable event, or one that breaks the documented ordering. It is
	// reported at cleanup rather than raised where it happens: the run still
	// has to be torn down, and a violation must never be silently dropped.
	protoErr error

	startedCh  chan struct{}
	readyCh    chan struct{}
	terminalCh chan struct{}
	// done closes when stdout reaches EOF, releasing waiters whose milestone
	// is never coming: a herder killed mid-run emits no terminal event, and
	// its reader must not sit until its own deadline.
	done chan struct{}
}

func newHerderStream() *herderStream {
	return &herderStream{
		startedCh:  make(chan struct{}),
		readyCh:    make(chan struct{}),
		terminalCh: make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// consume decodes events from r until EOF and records what it sees. It runs
// for the whole lifetime of the run — decoding on demand would let the
// child's stdout pipe fill and deadlock it — so callers run it in a goroutine
// and read the milestones through the await helpers.
func (h *herderStream) consume(r io.Reader) {
	defer close(h.done)

	dec := json.NewDecoder(r)
	for {
		var ev herderEvent
		if err := dec.Decode(&ev); err != nil {
			// EOF is how every run ends — the child closed stdout. Anything
			// else means the stream stopped being the protocol, and the
			// decoder cannot resynchronise mid-document.
			if !errors.Is(err, io.EOF) {
				h.fail(fmt.Errorf("decode event: %w", err))
				// Interpreting is over, but draining is not: an abandoned
				// pipe fills, and a child blocked writing to it can no
				// longer handle SIGTERM or remove its containers.
				_, _ = io.Copy(io.Discard, r)
			}
			return
		}
		// Fields may be added within protocol 1, so an unknown one is
		// ignored by decoding into a subset. A different protocol version
		// is not this fixture's to interpret at all.
		if ev.Protocol != 1 {
			continue
		}
		h.record(ev)
	}
}

// record files one event against the protocol's ordering rules: started
// exactly once and first, ready at most once and only before the terminal
// event, then exactly one terminal event. The herder guarantees all of that,
// so a breach is a bug somewhere — which is precisely why it is latched and
// reported rather than quietly ignored. In particular a second terminal event
// must not be dropped on the floor: a `failed` arriving after a `stopped`
// would otherwise turn a failed run into a passing test.
func (h *herderStream) record(ev herderEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch ev.Event {
	case "started":
		if h.started != nil {
			h.violate("a second started event")
			return
		}
		h.started = &ev
		close(h.startedCh)
	case "ready":
		switch {
		case h.started == nil:
			h.violate("a ready event before started")
		case h.terminal != nil:
			h.violate("a ready event after the terminal event")
		case h.ready != nil:
			h.violate("a second ready event")
		default:
			h.ready = &ev
			close(h.readyCh)
		}
	case "failed", "stopped":
		if h.terminal != nil {
			h.violate("a second terminal event (" + ev.Event + ")")
			return
		}
		h.terminal = &ev
		close(h.terminalCh)
	}
}

// violate latches an ordering breach. The caller holds the mutex.
func (h *herderStream) violate(what string) {
	if h.protoErr == nil {
		h.protoErr = fmt.Errorf("herder emitted %s", what)
	}
}

func (h *herderStream) fail(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.protoErr == nil {
		h.protoErr = err
	}
}

// Each await blocks until its milestone lands, stdout ends, the context is
// done or the timeout expires — then reads the recorded state, whichever of
// those woke it. Reading the record rather than the channel is what makes a
// milestone that landed just before EOF report from the record instead of
// from whichever case happened to win the race.

// awaitStarted waits for the run's first event, which precedes even request
// decoding: its absence means the child is not a herder, or died before it
// could say anything.
func (h *herderStream) awaitStarted(ctx context.Context, timeout time.Duration) *herderEvent {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-h.startedCh:
	case <-h.done:
	case <-ctx.Done():
	case <-timer.C:
	}
	return h.startedEvent()
}

// awaitReady waits for the fleet to come up. It gives up the moment the run
// ends instead: a terminal event means ready is never coming, and waiting out
// the full timeout would only delay the report.
func (h *herderStream) awaitReady(ctx context.Context, timeout time.Duration) *herderEvent {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-h.readyCh:
	case <-h.terminalCh:
	case <-h.done:
	case <-ctx.Done():
	case <-timer.C:
	}
	return h.readyEvent()
}

// awaitTerminal waits for the run to end.
func (h *herderStream) awaitTerminal(ctx context.Context, timeout time.Duration) *herderEvent {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-h.terminalCh:
	case <-h.done:
	case <-ctx.Done():
	case <-timer.C:
	}
	return h.terminalEvent()
}

func (h *herderStream) startedEvent() *herderEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.started
}

func (h *herderStream) readyEvent() *herderEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ready
}

func (h *herderStream) terminalEvent() *herderEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.terminal
}

// protocolError reports the first breach of the stdout contract, if any.
func (h *herderStream) protocolError() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.protoErr
}

// syncBuffer collects the child's stderr while the test may be reading it.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// StartDevices runs a herder child process that starts devices as containers
// on the controller's own network, and returns the identities it published.
// It returns nil when the controller has no network to herd on: a
// UNIFI_TEST_URL target is an external controller whose inform plane is never
// the suite's to point devices at, and the UOS harness is unsupported for now
// (its -sim image seeds its own demo devices, and a seeded UOS image — the
// real target there — is not yet available).
//
// The caller keeps owning adoption and the assertions; this only produces
// resolved identities. Adopt by MAC: devices batched into one synthetic
// container share an IP by design.
//
// No runtime mapping is configured here, so the fleet is all-synthetic unless
// the runner itself has one installed — which is the runner's decision to
// make, not the fixture's.
func StartDevices(ctx context.Context, t *testing.T, c *Controller, devices ...DeviceRequest) []EmulatedDevice {
	t.Helper()

	if c.Network == "" || c.InformURL == "" {
		t.Log("controller owns no device network; not starting devices")
		return nil
	}
	bin := os.Getenv(herderBinEnv)
	if bin == "" {
		// A required gate must never be satisfiable by a skip, so CI's
		// UNIFI_TEST_REQUIRE turns the missing herder into a failure.
		if os.Getenv("UNIFI_TEST_REQUIRE") != "" {
			t.Fatalf("%s is required but not set", herderBinEnv)
		}
		t.Logf("%s is not set; not starting devices", herderBinEnv)
		return nil
	}

	args := []string{
		"--network", c.Network,
		"--inform-url", c.InformURL,
		"--devices", "-",
		"--startup-timeout", herderStartupTimeout.String(),
		"--stop-timeout", herderStopTimeout.String(),
	}
	// A release herder resolves its own version-matched image; only a
	// development build needs one supplied.
	if image := os.Getenv(herderImageEnv); image != "" {
		args = append(args, "--synthetic-image", image)
	}

	// Deliberately exec.Command, not exec.CommandContext: a cancelled test
	// context would kill the child outright and skip the signal cleanup that
	// removes its containers, leaking a fleet. The context below bounds how
	// long the fixture waits for startup; teardown answers to t.Cleanup and
	// its own timeouts alone.
	cmd := exec.Command(bin, args...)

	h, stdin, err := spawnHerder(cmd)
	if err != nil {
		t.Fatalf("herder: %v", err)
	}
	t.Cleanup(func() { h.stop(t) })
	stream := h.stream

	if err := encodeDeviceRequest(stdin, devices); err != nil {
		t.Fatalf("write herder request: %v", err)
	}
	// The request ends at EOF; nothing else is ever written to stdin.
	if err := stdin.Close(); err != nil {
		t.Fatalf("close herder stdin: %v", err)
	}

	if started := stream.awaitStarted(ctx, herderSlack); started == nil {
		h.fatalf(t, "herder never emitted a started event")
	}
	ready := stream.awaitReady(ctx, herderStartupTimeout+herderSlack)
	if ready == nil {
		if term := stream.terminalEvent(); term != nil {
			h.fatalf(t, "herder %s during %s: %s", term.Code, term.Phase, term.Message)
		}
		h.fatalf(t, "herder never became ready")
	}
	for _, d := range ready.Devices {
		t.Logf("herded device %d: model=%s mac=%s serial=%s name=%s ip=%s", d.Index, d.Model, d.MAC, d.Serial, d.Name, d.IP)
	}
	return ready.Devices
}

// herder is a running herder child process and the pipes feeding it.
type herder struct {
	cmd      *exec.Cmd
	stream   *herderStream
	evidence *syncBuffer
	readers  *sync.WaitGroup
}

// reporter is the slice of *testing.T the teardown reports through. It exists
// so the teardown's own failure paths can be exercised by a test without that
// test failing: a subtest's failure propagates to its parent, so a fake
// recorder is the only way to assert on what a bad teardown reports.
type reporter interface {
	Errorf(format string, args ...any)
	Logf(format string, args ...any)
	Failed() bool
	Helper()
}

// spawnHerder starts cmd with the pipes and the drains the protocol needs,
// and returns the live run alongside its stdin. Both readers run for the
// whole lifetime of the child; the caller writes the request to stdin and
// closes it.
func spawnHerder(cmd *exec.Cmd) (*herder, io.WriteCloser, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	// Separate pipes: stdout carries the control protocol and nothing else,
	// while stderr carries diagnostics and the aggregated device logs, which
	// are the only evidence of why a device never came up. Never parsed —
	// only reported.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start %s: %w", cmd.Path, err)
	}

	h := &herder{
		cmd:      cmd,
		stream:   newHerderStream(),
		evidence: &syncBuffer{},
		readers:  &sync.WaitGroup{},
	}
	// cmd.Wait closes both pipes, so it may only run once both readers have
	// finished; the WaitGroup is what stop waits on before reaping.
	h.readers.Add(2)
	go func() {
		defer h.readers.Done()
		h.stream.consume(stdout)
	}()
	go func() {
		defer h.readers.Done()
		_, _ = io.Copy(h.evidence, stderr)
	}()
	return h, stdin, nil
}

// fatalf fails the test. The child's stderr is not repeated here: stop
// attaches it once, at cleanup, for whatever failed the test.
func (h *herder) fatalf(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Fatalf(format, args...)
}

// stop ends the run the way the protocol expects: SIGTERM, drain stdout until
// the terminal event, then reap the process. It never uses the test's
// context — a cancelled context must not be able to skip the cleanup that
// removes the device containers.
//
// It is also where a failed test gets the device-side story attached. The
// control protocol carries stable codes but no detail, and the assertions
// upstream only ever see the controller's view; the herder's stderr — its
// diagnostics and the aggregated device logs — is the only record of what
// the devices themselves were doing. Cleanup is the one place that sees both
// the stream and the verdict, so every failure gets it, not just the ones
// this file raises.
func (h *herder) stop(t reporter) {
	t.Helper()

	// One budget covers the whole teardown — signal, drain and reap — so a
	// child that stalls at every step cannot hold the test for a multiple of
	// the stop timeout.
	deadline := time.Now().Add(herderStopTimeout + herderSlack)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	if err := h.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// An already-exited child is not a cleanup failure; the terminal
		// event and exit status below are still the verdict.
		t.Logf("signal herder: %v", err)
	}

	term := h.stream.awaitTerminal(ctx, herderStopTimeout+herderSlack)
	if term == nil {
		// Nothing else will end this run, and a herder still holding
		// containers open would block the network's removal.
		t.Errorf("herder emitted no terminal event within %s; killing it", herderStopTimeout+herderSlack)
		_ = h.cmd.Process.Kill()
	} else if term.Event == "failed" {
		// A failure after ready is a real finding: the fleet died under the
		// test that was measuring it.
		t.Errorf("herder %s during %s: %s (cleanup_complete=%v)",
			term.Code, term.Phase, term.Message, term.CleanupComplete)
	}

	// Both pipes must reach EOF before Wait, which closes them. Whatever is
	// left of the budget goes to the reap, but never less than a short grace:
	// a child that answered just before the deadline deserves to be reaped
	// rather than killed on the buzzer.
	exit := make(chan error, 1)
	go func() {
		h.readers.Wait()
		exit <- h.cmd.Wait()
	}()
	reap := max(time.Until(deadline), herderKillGrace)
	select {
	case err := <-exit:
		if err != nil {
			t.Errorf("herder exited with %v", err)
		}
	case <-time.After(reap):
		t.Errorf("herder did not exit within %s; killing it", reap)
		_ = h.cmd.Process.Kill()
		// The kill closes the child's pipe ends, which releases the reaper
		// above — unless something that inherited them outlived it. Bounded
		// so that case is reported instead of hanging the test forever.
		select {
		case <-exit:
		case <-time.After(herderKillGrace):
			t.Errorf("herder stdout/stderr still open %s after SIGKILL; a surviving child may hold it", herderKillGrace)
		}
	}

	if err := h.stream.protocolError(); err != nil {
		t.Errorf("herder violated the stdout protocol: %v", err)
	}

	// Last, so the evidence sits below whatever failed rather than above it.
	if t.Failed() {
		t.Logf("herder stderr:\n%s", h.evidence.String())
	}
}
