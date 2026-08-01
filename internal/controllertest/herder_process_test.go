package controllertest

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// These tests drive the adapter against a stand-in herder — this very test
// binary, re-executed — because the mechanics that matter most are the ones a
// pure in-memory test cannot see. exec.Command versus exec.CommandContext,
// closing stdin, and the SIGTERM teardown are all invisible to a test that
// only feeds bytes to herderStream: break any of them and such a suite stays
// green while every real run leaks containers.
const (
	// fakeHerderEnv switches this binary into stand-in-herder mode.
	fakeHerderEnv = "CONTROLLERTEST_FAKE_HERDER"
	// fakeHerderReportEnv names the file the stand-in writes its account of
	// the run to, which is how the parent inspects what actually reached the
	// child and how it was asked to stop.
	fakeHerderReportEnv = "CONTROLLERTEST_FAKE_HERDER_REPORT"
)

// fakeHerderReport is what the stand-in herder records for the parent test.
type fakeHerderReport struct {
	Network        string          `json:"network"`
	InformURL      string          `json:"inform_url"`
	Devices        string          `json:"devices"`
	SyntheticImage string          `json:"synthetic_image"`
	StartupTimeout string          `json:"startup_timeout"`
	StopTimeout    string          `json:"stop_timeout"`
	Request        json.RawMessage `json:"request"`
	StdinClosed    bool            `json:"stdin_closed"`
	Signal         string          `json:"signal"`
}

// TestFakeHerderProcess is not a test. It is the child process the tests
// below exec: when fakeHerderEnv is set it speaks the herder's protocol on
// stdout and exits, and otherwise it does nothing at all.
func TestFakeHerderProcess(t *testing.T) {
	mode := os.Getenv(fakeHerderEnv)
	if mode == "" {
		t.Skip("not a stand-in herder invocation")
	}
	// os.Exit, never return: letting the testing package regain control would
	// print its own summary onto stdout, which is the control stream.
	os.Exit(runFakeHerder(mode))
}

// runFakeHerder implements just enough of the herder to exercise the adapter:
// the flag surface, the started/ready/stopped ordering, one identity per
// requested device, and a SIGTERM that produces a clean terminal event.
func runFakeHerder(mode string) int {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)

	var report fakeHerderReport
	fs := flag.NewFlagSet("fake-herder", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&report.Network, "network", "", "")
	fs.StringVar(&report.InformURL, "inform-url", "", "")
	fs.StringVar(&report.Devices, "devices", "", "")
	fs.StringVar(&report.SyntheticImage, "synthetic-image", "", "")
	fs.StringVar(&report.StartupTimeout, "startup-timeout", "", "")
	fs.StringVar(&report.StopTimeout, "stop-timeout", "", "")
	// Everything after the separator is the adapter's command line; the
	// leading arguments belong to the test binary that is standing in.
	args := os.Args[1:]
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	const runID = "7f8b1ab4"
	emit := func(v any) {
		body, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		if _, err := os.Stdout.Write(append(body, '\n')); err != nil {
			panic(err)
		}
	}
	emit(map[string]any{"protocol": 1, "event": "started", "run_id": runID})

	// Reading to EOF is the point: if the adapter ever stops closing stdin
	// this blocks forever and the parent test times out rather than passing.
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake herder: read stdin: %v\n", err)
		return 1
	}
	report.StdinClosed = true
	report.Request = json.RawMessage(strings.TrimSpace(string(raw)))

	var request struct {
		Version int `json:"version"`
		Devices []struct {
			Model string `json:"model"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		fmt.Fprintf(os.Stderr, "fake herder: decode request: %v\n", err)
		return 1
	}

	devices := make([]map[string]any, 0, len(request.Devices))
	for i, d := range request.Devices {
		devices = append(devices, map[string]any{
			"index": i, "model": d.Model,
			"mac":    fmt.Sprintf("02:00:00:00:00:%02x", i+1),
			"serial": fmt.Sprintf("EMU%04d", i+1),
			"name":   fmt.Sprintf("emu-%s-%d", strings.ToLower(d.Model), i+1),
			"ip":     "172.28.0.4",
		})
	}
	// Device logs belong on stderr, and enough of them to prove the adapter
	// drains that pipe too: a fixture that only read stdout would wedge here.
	for i := range 200 {
		fmt.Fprintf(os.Stderr, "fake herder: device log line %d\n", i)
	}
	emit(map[string]any{"protocol": 1, "event": "ready", "run_id": runID, "devices": devices})

	if mode == "deaf" {
		// Never answer the signal: the adapter must give up and kill it
		// rather than wait forever.
		select {}
	}

	sig := <-signals
	report.Signal = sig.String()
	if path := os.Getenv(fakeHerderReportEnv); path != "" {
		body, err := json.Marshal(report)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(path, body, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "fake herder: write report: %v\n", err)
			return 1
		}
	}

	if mode == "failterminal" {
		// A fleet that died under the test measuring it: the run ends in a
		// failure, not a clean stop.
		emit(map[string]any{
			"protocol": 1, "event": "failed", "run_id": runID,
			"phase": "runtime", "code": "device_exited",
			"message":          "one or more device containers exited",
			"cleanup_complete": true, "devices": []any{},
		})
		return 1
	}

	emit(map[string]any{"protocol": 1, "event": "stopped", "run_id": runID, "reason": "signal"})
	return 0
}

// fakeHerder points the adapter at this test binary in stand-in mode and
// returns the path of the report the child will write. The shim is a script
// because the adapter builds the herder's whole command line itself, and the
// test binary needs its own -test.run before any of it.
func fakeHerder(t *testing.T, mode string) string {
	t.Helper()

	dir := t.TempDir()
	report := filepath.Join(dir, "report.json")
	shim := filepath.Join(dir, "herder")
	script := fmt.Sprintf("#!/bin/sh\nexec %q -test.run='^TestFakeHerderProcess$' -- \"$@\"\n", os.Args[0])
	if err := os.WriteFile(shim, []byte(script), 0o700); err != nil {
		t.Fatalf("write herder shim: %v", err)
	}

	t.Setenv(fakeHerderEnv, mode)
	t.Setenv(fakeHerderReportEnv, report)
	t.Setenv(herderBinEnv, shim)
	t.Setenv(herderImageEnv, "example.invalid/synthetic:test")
	return report
}

func readReport(t *testing.T, path string) fakeHerderReport {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stand-in herder report: %v", err)
	}
	var got fakeHerderReport
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode stand-in herder report: %v", err)
	}
	return got
}

func testController() *Controller {
	return &Controller{
		BaseURL: "https://127.0.0.1:8443", Username: demoUsername, Password: demoPassword,
		Site: demoSite, Network: "controllertest-net", InformURL: "http://172.28.0.2:8080/inform",
	}
}

// TestStartDevicesDrivesTheHerderProcess walks the whole child lifecycle: the
// command line the adapter builds, the request reaching stdin and stdin being
// closed behind it, the identities coming back from ready, and cleanup ending
// the run with SIGTERM rather than a kill.
func TestStartDevicesDrivesTheHerderProcess(t *testing.T) {
	report := fakeHerder(t, "ok")
	c := testController()

	var devices []EmulatedDevice
	ok := t.Run("run", func(t *testing.T) {
		devices = StartDevices(context.Background(), t, c,
			DeviceRequest{Model: "USM8P"}, DeviceRequest{Model: "UXGENT"})
	})
	if !ok {
		t.Fatal("herder run failed")
	}

	if len(devices) != 2 {
		t.Fatalf("devices = %d, want 2", len(devices))
	}
	if devices[0].Model != "USM8P" || devices[0].MAC != "02:00:00:00:00:01" || devices[0].IP != "172.28.0.4" {
		t.Errorf("device 0 = %+v", devices[0])
	}
	if devices[1].Model != "UXGENT" || devices[1].Index != 1 {
		t.Errorf("device 1 = %+v", devices[1])
	}

	// The subtest has returned, so its cleanup has run and the child has
	// written its account of the run.
	got := readReport(t, report)
	if got.Network != c.Network || got.InformURL != c.InformURL {
		t.Errorf("child saw network=%q inform-url=%q, want %q/%q", got.Network, got.InformURL, c.Network, c.InformURL)
	}
	if got.Devices != "-" {
		t.Errorf("child saw --devices %q, want %q (the request must come over stdin)", got.Devices, "-")
	}
	if got.SyntheticImage != "example.invalid/synthetic:test" {
		t.Errorf("child saw --synthetic-image %q", got.SyntheticImage)
	}
	if !got.StdinClosed {
		t.Error("child never reached EOF on stdin; the adapter must close it after the request")
	}
	want := `{"version":1,"devices":[{"model":"USM8P"},{"model":"UXGENT"}]}`
	if string(got.Request) != want {
		t.Errorf("child read request %s, want %s", got.Request, want)
	}
	if got.Signal != syscall.SIGTERM.String() {
		t.Errorf("child was stopped by %q, want SIGTERM", got.Signal)
	}
}

// TestStartDevicesOmitsTheImageFlagWhenUnset proves a release herder — which
// resolves its own version-matched image — is not handed an empty override.
func TestStartDevicesOmitsTheImageFlagWhenUnset(t *testing.T) {
	report := fakeHerder(t, "ok")
	t.Setenv(herderImageEnv, "")

	ok := t.Run("run", func(t *testing.T) {
		StartDevices(context.Background(), t, testController(), DeviceRequest{Model: "USM8P"})
	})
	if !ok {
		t.Fatal("herder run failed")
	}
	if got := readReport(t, report).SyntheticImage; got != "" {
		t.Errorf("child saw --synthetic-image %q, want the flag absent", got)
	}
}

// TestStartDevicesSurvivesContextCancellation is the regression test for the
// single most load-bearing line in the adapter: exec.Command, never
// exec.CommandContext. Cancelling the test's context after the fleet is up
// must not touch the child — it must still be alive to receive the SIGTERM
// that makes it remove its containers. Under CommandContext the cancel kills
// it outright, no terminal event ever arrives, and every run leaks a fleet.
func TestStartDevicesSurvivesContextCancellation(t *testing.T) {
	report := fakeHerder(t, "ok")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ok := t.Run("run", func(t *testing.T) {
		if devices := StartDevices(ctx, t, testController(), DeviceRequest{Model: "USM8P"}); len(devices) != 1 {
			t.Fatalf("devices = %d, want 1", len(devices))
		}
		// Cancel while the fleet is up and cleanup still pending.
		cancel()
		time.Sleep(50 * time.Millisecond)
	})
	if !ok {
		t.Fatal("herder run failed after its context was cancelled")
	}

	if got := readReport(t, report).Signal; got != syscall.SIGTERM.String() {
		t.Errorf("child was stopped by %q, want SIGTERM: a cancelled context must not skip signal cleanup", got)
	}
}

// recordingReporter stands in for *testing.T so a deliberately bad teardown
// can be asserted on without failing the test doing the asserting.
type recordingReporter struct {
	mu     sync.Mutex
	errors []string
}

func (r *recordingReporter) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *recordingReporter) Logf(string, ...any) {}
func (r *recordingReporter) Helper()             {}

func (r *recordingReporter) Failed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.errors) > 0
}

func (r *recordingReporter) joined() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.errors, "\n")
}

// TestHerderStopReportsATerminalFailure proves a run that ends in `failed`
// rather than `stopped` fails the test and names the code and phase, instead
// of being written off as an ordinary teardown. A fleet that died under the
// test measuring it must never leave that test green.
func TestHerderStopReportsATerminalFailure(t *testing.T) {
	fakeHerder(t, "failterminal")

	cmd := exec.Command(os.Getenv(herderBinEnv), "--network", "n",
		"--inform-url", "http://172.28.0.2:8080/inform", "--devices", "-")
	h, stdin, err := spawnHerder(cmd)
	if err != nil {
		t.Fatalf("spawn stand-in herder: %v", err)
	}
	if err := encodeDeviceRequest(stdin, []DeviceRequest{{Model: "USM8P"}}); err != nil {
		t.Fatalf("write request: %v", err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatalf("close stdin: %v", err)
	}
	if ready := h.stream.awaitReady(context.Background(), 30*time.Second); ready == nil {
		t.Fatal("stand-in herder never became ready")
	}

	rec := &recordingReporter{}
	h.stop(rec)

	got := rec.joined()
	if !rec.Failed() {
		t.Fatal("a terminal failed event was not reported as a failure")
	}
	for _, want := range []string{"device_exited", "runtime"} {
		if !strings.Contains(got, want) {
			t.Errorf("teardown reported %q, want it to name %q", got, want)
		}
	}
}

// TestHerderStopKillsAChildThatIgnoresSIGTERM proves teardown is bounded. A
// child that never answers the signal must be killed and reaped and the whole
// thing reported, so one wedged run cannot hold the suite until the test
// binary's own timeout.
func TestHerderStopKillsAChildThatIgnoresSIGTERM(t *testing.T) {
	fakeHerder(t, "deaf")
	shrinkHerderClock(t)

	cmd := exec.Command(os.Getenv(herderBinEnv), "--network", "n",
		"--inform-url", "http://172.28.0.2:8080/inform", "--devices", "-")
	h, stdin, err := spawnHerder(cmd)
	if err != nil {
		t.Fatalf("spawn stand-in herder: %v", err)
	}
	if err := encodeDeviceRequest(stdin, []DeviceRequest{{Model: "USM8P"}}); err != nil {
		t.Fatalf("write request: %v", err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatalf("close stdin: %v", err)
	}
	if ready := h.stream.awaitReady(context.Background(), 30*time.Second); ready == nil {
		t.Fatal("stand-in herder never became ready")
	}

	rec := &recordingReporter{}
	start := time.Now()
	h.stop(rec)
	elapsed := time.Since(start)

	if !rec.Failed() {
		t.Error("teardown of an unresponsive herder reported success, want a failure")
	}
	if got := rec.joined(); !strings.Contains(got, "did not exit") && !strings.Contains(got, "no terminal event") {
		t.Errorf("teardown reported %q, want it to name the unresponsive child", got)
	}
	// The clock is shrunk to sub-second timeouts, so anything near the real
	// 60s budget means the bound is not being applied at all.
	if elapsed > 20*time.Second {
		t.Errorf("teardown took %s on a shortened clock; it is not bounded", elapsed)
	}
	// However badly it went, the process must not be left running.
	if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
		t.Error("stand-in herder is still alive after teardown")
	}
}

// shrinkHerderClock puts the adapter on a short clock for tests that
// deliberately wait out a timeout.
func shrinkHerderClock(t *testing.T) {
	t.Helper()

	startup, stop, slack, grace := herderStartupTimeout, herderStopTimeout, herderSlack, herderKillGrace
	t.Cleanup(func() {
		herderStartupTimeout, herderStopTimeout, herderSlack, herderKillGrace = startup, stop, slack, grace
	})
	herderStartupTimeout = 5 * time.Second
	herderStopTimeout = 500 * time.Millisecond
	herderSlack = 500 * time.Millisecond
	herderKillGrace = 500 * time.Millisecond
}
