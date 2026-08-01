package controllertest

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestEncodeDeviceRequestOmitsUnsetIdentity pins the request wire form: the
// herder's decoder rejects unknown fields, and anything the caller leaves
// unset must be absent rather than empty so the herder allocates it.
func TestEncodeDeviceRequestOmitsUnsetIdentity(t *testing.T) {
	var buf strings.Builder
	if err := encodeDeviceRequest(&buf, []DeviceRequest{{Model: "UXGENT"}}); err != nil {
		t.Fatalf("encodeDeviceRequest: %v", err)
	}

	want := `{"version":1,"devices":[{"model":"UXGENT"}]}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("request = %q, want %q", got, want)
	}
}

// TestEncodeDeviceRequestCarriesSuppliedIdentity proves a caller that pins an
// identity gets it on the wire verbatim; the herder canonicalizes a supplied
// MAC but never replaces one.
func TestEncodeDeviceRequestCarriesSuppliedIdentity(t *testing.T) {
	var buf strings.Builder
	err := encodeDeviceRequest(&buf, []DeviceRequest{
		{Model: "USM8P", MAC: "02:00:00:00:00:01", Serial: "EMU0001", Name: "emu-usm8p"},
	})
	if err != nil {
		t.Fatalf("encodeDeviceRequest: %v", err)
	}

	want := `{"version":1,"devices":[{"model":"USM8P","mac":"02:00:00:00:00:01",` +
		`"serial":"EMU0001","name":"emu-usm8p"}]}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("request = %q, want %q", got, want)
	}
}

// TestHerderStreamReportsReadyThenStopped walks a whole successful run's
// stdout and proves both milestones land with their payloads: ready carries
// the resolved identities and their addresses, stopped is terminal.
func TestHerderStreamReportsReadyThenStopped(t *testing.T) {
	const stdout = `{"protocol":1,"event":"started","run_id":"7f8b1ab4"}
{"protocol":1,"event":"ready","run_id":"7f8b1ab4","devices":[` +
		`{"index":0,"model":"USM8P","mac":"02:00:00:00:00:01","serial":"EMU0001","name":"emu-usm8p","ip":"172.28.0.4"}]}
{"protocol":1,"event":"stopped","run_id":"7f8b1ab4","reason":"signal"}
`

	h := newHerderStream()
	h.consume(strings.NewReader(stdout))

	ready := h.awaitReady(context.Background(), time.Second)
	if ready == nil {
		t.Fatal("no ready event")
	}
	if len(ready.Devices) != 1 {
		t.Fatalf("ready devices = %d, want 1", len(ready.Devices))
	}
	got := ready.Devices[0]
	want := EmulatedDevice{
		Index: 0, Model: "USM8P", MAC: "02:00:00:00:00:01",
		Serial: "EMU0001", Name: "emu-usm8p", IP: "172.28.0.4",
	}
	if got != want {
		t.Errorf("ready device = %+v, want %+v", got, want)
	}

	term := h.awaitTerminal(context.Background(), time.Second)
	if term == nil {
		t.Fatal("no terminal event")
	}
	if term.Event != "stopped" || term.Reason != "signal" {
		t.Errorf("terminal = %+v, want stopped/signal", term)
	}
}

// TestHerderStreamReportsFailedBeforeReady proves a run that dies during
// startup is surfaced as its terminal failure rather than as a ready that
// never comes: awaitReady must return once the terminal event lands, so a
// caller waiting on startup is never left blocking until its own deadline.
func TestHerderStreamReportsFailedBeforeReady(t *testing.T) {
	const stdout = `{"protocol":1,"event":"started","run_id":"7f8b1ab4"}
{"protocol":1,"event":"failed","run_id":"7f8b1ab4","phase":"health","code":"device_unhealthy",` +
		`"message":"one or more device runtimes did not become healthy","cleanup_complete":true,"devices":[]}
`

	h := newHerderStream()
	h.consume(strings.NewReader(stdout))

	if ready := h.awaitReady(context.Background(), time.Second); ready != nil {
		t.Fatalf("ready = %+v, want none", ready)
	}
	term := h.awaitTerminal(context.Background(), time.Second)
	if term == nil {
		t.Fatal("no terminal event")
	}
	if term.Event != "failed" || term.Code != "device_unhealthy" || term.Phase != "health" {
		t.Errorf("terminal = %+v, want failed/device_unhealthy/health", term)
	}
	if !term.CleanupComplete {
		t.Error("cleanup_complete = false, want true")
	}
}

// TestHerderStreamIgnoresFieldsItDoesNotKnow pins the consumer half of the
// protocol's compatibility rule. Fields may be added within protocol 1, so an
// unknown one must not break decoding; a future protocol version, by
// contrast, is not this fixture's to interpret and is skipped entirely.
func TestHerderStreamIgnoresFieldsItDoesNotKnow(t *testing.T) {
	const stdout = `{"protocol":1,"event":"started","run_id":"7f8b1ab4","future_field":{"a":1}}
{"protocol":2,"event":"ready","run_id":"7f8b1ab4","devices":[{"index":0,"mac":"02:00:00:00:00:09"}]}
{"protocol":1,"event":"ready","run_id":"7f8b1ab4","devices":[` +
		`{"index":0,"model":"USM8P","mac":"02:00:00:00:00:01","ip":"172.28.0.4","future_field":true}],"extra":"ignored"}
{"protocol":1,"event":"stopped","run_id":"7f8b1ab4","reason":"signal"}
`

	h := newHerderStream()
	h.consume(strings.NewReader(stdout))

	ready := h.awaitReady(context.Background(), time.Second)
	if ready == nil {
		t.Fatal("no ready event")
	}
	if len(ready.Devices) != 1 || ready.Devices[0].MAC != "02:00:00:00:00:01" {
		t.Errorf("ready devices = %+v, want only the protocol-1 device", ready.Devices)
	}
}

// TestHerderStreamRejectsASecondTerminalEvent pins the rule that matters most
// for a green build: a `failed` arriving after a `stopped` must not be
// swallowed. Dropping it would turn a run whose fleet died into a passing
// test, since the first terminal event alone looks like a clean stop.
func TestHerderStreamRejectsASecondTerminalEvent(t *testing.T) {
	const stdout = `{"protocol":1,"event":"started","run_id":"7f8b1ab4"}
{"protocol":1,"event":"stopped","run_id":"7f8b1ab4","reason":"signal"}
{"protocol":1,"event":"failed","run_id":"7f8b1ab4","phase":"runtime","code":"device_exited","message":"x","cleanup_complete":true,"devices":[]}
`

	h := newHerderStream()
	h.consume(strings.NewReader(stdout))

	if err := h.protocolError(); err == nil {
		t.Error("a second terminal event was accepted silently, want a protocol error")
	}
}

// TestHerderStreamRejectsReadyBeforeStarted pins the rest of the ordering
// rule: started comes first, always.
func TestHerderStreamRejectsReadyBeforeStarted(t *testing.T) {
	const stdout = `{"protocol":1,"event":"ready","run_id":"7f8b1ab4","devices":[]}
{"protocol":1,"event":"started","run_id":"7f8b1ab4"}
`

	h := newHerderStream()
	h.consume(strings.NewReader(stdout))

	if err := h.protocolError(); err == nil {
		t.Error("ready before started was accepted silently, want a protocol error")
	}
	if ready := h.awaitReady(context.Background(), time.Second); ready != nil {
		t.Errorf("ready = %+v, want none: the out-of-order event must not be recorded", ready)
	}
}

// TestHerderStreamDrainsPastAMalformedEvent proves the decoder giving up does
// not abandon the pipe. Interpreting stops at the first unreadable byte, but
// reading must continue to EOF: a child blocked writing to a full stdout pipe
// can no longer handle SIGTERM, so it would never remove its containers.
func TestHerderStreamDrainsPastAMalformedEvent(t *testing.T) {
	stdout := `{"protocol":1,"event":"started","run_id":"7f8b1ab4"}` + "\n" +
		"this is not JSON\n" + strings.Repeat("x", 512<<10) + "\n"
	r := strings.NewReader(stdout)

	h := newHerderStream()
	h.consume(r)

	if err := h.protocolError(); err == nil {
		t.Error("a malformed event was accepted silently, want a protocol error")
	}
	if r.Len() != 0 {
		t.Errorf("%d bytes left unread; the stream must be drained to EOF", r.Len())
	}
}

// TestHerderStreamEndsWaitsWhenStdoutCloses proves a herder that dies without
// emitting a terminal event does not strand its waiters: stdout reaching EOF
// releases them, so the caller reports a missing terminal instead of blocking
// for the whole stop timeout.
func TestHerderStreamEndsWaitsWhenStdoutCloses(t *testing.T) {
	h := newHerderStream()
	h.consume(strings.NewReader(`{"protocol":1,"event":"started","run_id":"7f8b1ab4"}` + "\n"))

	// The timeout is the thing under test, so it is set far longer than the
	// test may take: waiting it out would be the bug.
	start := time.Now()
	term := h.awaitTerminal(context.Background(), time.Minute)
	elapsed := time.Since(start)

	if term != nil {
		t.Fatalf("terminal = %+v, want none", term)
	}
	if elapsed > 5*time.Second {
		t.Errorf("awaitTerminal blocked for %s after stdout closed, want a prompt return", elapsed)
	}
}
