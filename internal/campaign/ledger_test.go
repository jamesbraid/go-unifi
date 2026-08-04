package campaign

import (
	"path/filepath"
	"testing"
)

func TestAttemptLedgerDurablyRetainsFailedTerminalAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempts.ndjson")
	receipt := mustReadTestFile(t, "../../campaigns/fixtures/dns_record.execution-receipt.json")
	if err := BeginAttempt(path, "fixture-attempt-1", receipt); err != nil {
		t.Fatal(err)
	}
	if err := FinishAttempt(path, "fixture-attempt-1", "failed", "candidate_validation"); err != nil {
		t.Fatal(err)
	}
	events, err := ReadAttemptLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].State != "started" || events[1].State != "failed" || events[1].ErrorClass != "candidate_validation" {
		t.Fatalf("attempt events = %#v", events)
	}
	if events[1].PreviousEventSHA256 != events[0].ChecksumSHA256 {
		t.Fatalf("terminal event is not hash-chained: %#v", events)
	}
}

func TestAttemptLedgerRequiresTerminalBeforeNextAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempts.ndjson")
	receipt := mustReadTestFile(t, "../../campaigns/fixtures/dns_record.execution-receipt.json")
	if err := BeginAttempt(path, "attempt-1", receipt); err != nil {
		t.Fatal(err)
	}
	if err := BeginAttempt(path, "attempt-2", receipt); err == nil {
		t.Fatal("BeginAttempt() accepted a second start before the first terminal event")
	}
}
