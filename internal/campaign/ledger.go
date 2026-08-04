package campaign

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AttemptEvent is one append-only transition in a campaign attempt ledger.
// The hash chain makes deletion, insertion, and rewriting detectable.
type AttemptEvent struct {
	FormatVersion          int    `json:"format_version"`
	Sequence               int    `json:"sequence"`
	AttemptID              string `json:"attempt_id"`
	State                  string `json:"state"`
	ExecutionReceiptSHA256 string `json:"execution_receipt_sha256,omitempty"`
	PreviousEventSHA256    string `json:"previous_event_sha256,omitempty"`
	ErrorClass             string `json:"error_class,omitempty"`
	ChecksumSHA256         string `json:"checksum_sha256"`
}

type attemptEventPayload struct {
	FormatVersion          int    `json:"format_version"`
	Sequence               int    `json:"sequence"`
	AttemptID              string `json:"attempt_id"`
	State                  string `json:"state"`
	ExecutionReceiptSHA256 string `json:"execution_receipt_sha256,omitempty"`
	PreviousEventSHA256    string `json:"previous_event_sha256,omitempty"`
	ErrorClass             string `json:"error_class,omitempty"`
}

// BeginAttempt durably appends a started event before campaign validation.
func BeginAttempt(path, attemptID string, executionReceipt []byte) error {
	if attemptID == "" || len(executionReceipt) == 0 {
		return fmt.Errorf("attempt identity and execution receipt are required")
	}
	events, err := ReadAttemptLedger(path)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.AttemptID == attemptID {
			return fmt.Errorf("attempt %q already exists", attemptID)
		}
	}
	if len(events) > 0 && events[len(events)-1].State == "started" {
		return fmt.Errorf("attempt %q has no terminal event", events[len(events)-1].AttemptID)
	}
	event := AttemptEvent{
		FormatVersion:          1,
		Sequence:               len(events) + 1,
		AttemptID:              attemptID,
		State:                  "started",
		ExecutionReceiptSHA256: digest(executionReceipt),
	}
	if len(events) > 0 {
		event.PreviousEventSHA256 = events[len(events)-1].ChecksumSHA256
	}
	return appendAttemptEvent(path, event)
}

// FinishAttempt durably appends the success or failure terminal transition.
func FinishAttempt(path, attemptID, state, errorClass string) error {
	if state != "succeeded" && state != "failed" {
		return fmt.Errorf("unsupported terminal attempt state %q", state)
	}
	if state == "failed" && errorClass == "" {
		return fmt.Errorf("failed attempt error class is required")
	}
	if state == "succeeded" && errorClass != "" {
		return fmt.Errorf("successful attempt cannot carry an error class")
	}
	events, err := ReadAttemptLedger(path)
	if err != nil {
		return err
	}
	if len(events) == 0 || events[len(events)-1].State != "started" || events[len(events)-1].AttemptID != attemptID {
		return fmt.Errorf("attempt %q has no current started event", attemptID)
	}
	return appendAttemptEvent(path, AttemptEvent{
		FormatVersion:       1,
		Sequence:            len(events) + 1,
		AttemptID:           attemptID,
		State:               state,
		PreviousEventSHA256: events[len(events)-1].ChecksumSHA256,
		ErrorClass:          errorClass,
	})
}

// ReadAttemptLedger validates and returns every event in an NDJSON ledger.
func ReadAttemptLedger(path string) ([]AttemptEvent, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return []AttemptEvent{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open attempt ledger: %w", err)
	}
	defer file.Close()

	events := []AttemptEvent{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			return nil, fmt.Errorf("attempt ledger contains an empty event")
		}
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		var event AttemptEvent
		if err := decoder.Decode(&event); err != nil {
			return nil, fmt.Errorf("attempt ledger event %d: %w", len(events)+1, err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return nil, fmt.Errorf("attempt ledger event %d has trailing JSON", len(events)+1)
		}
		if err := validateAttemptEvent(event, events); err != nil {
			return nil, fmt.Errorf("attempt ledger event %d: %w", len(events)+1, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read attempt ledger: %w", err)
	}
	return events, nil
}

func appendAttemptEvent(path string, event AttemptEvent) error {
	checksum, err := attemptEventChecksum(event)
	if err != nil {
		return err
	}
	event.ChecksumSHA256 = checksum
	document, err := json.Marshal(event)
	if err != nil {
		return err
	}
	document = append(document, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(document); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func validateAttemptEvent(event AttemptEvent, prior []AttemptEvent) error {
	if event.FormatVersion != 1 || event.Sequence != len(prior)+1 || event.AttemptID == "" {
		return fmt.Errorf("identity or sequence is invalid")
	}
	previous := ""
	if len(prior) > 0 {
		previous = prior[len(prior)-1].ChecksumSHA256
	}
	if event.PreviousEventSHA256 != previous {
		return fmt.Errorf("hash chain does not match previous event")
	}
	if event.State == "started" {
		if event.ExecutionReceiptSHA256 == "" || event.ErrorClass != "" {
			return fmt.Errorf("started event bindings are incomplete")
		}
		if len(prior) > 0 && prior[len(prior)-1].State == "started" {
			return fmt.Errorf("previous attempt has no terminal event")
		}
	} else if event.State == "failed" || event.State == "succeeded" {
		if len(prior) == 0 || prior[len(prior)-1].State != "started" || prior[len(prior)-1].AttemptID != event.AttemptID ||
			event.ExecutionReceiptSHA256 != "" || (event.State == "failed") != (event.ErrorClass != "") {
			return fmt.Errorf("terminal event does not match its started event")
		}
	} else {
		return fmt.Errorf("unsupported state %q", event.State)
	}
	checksum, err := attemptEventChecksum(event)
	if err != nil {
		return err
	}
	if event.ChecksumSHA256 != checksum {
		return fmt.Errorf("checksum does not match canonical event")
	}
	return nil
}

func attemptEventChecksum(event AttemptEvent) (string, error) {
	document, err := json.Marshal(attemptEventPayload{
		FormatVersion: event.FormatVersion, Sequence: event.Sequence, AttemptID: event.AttemptID, State: event.State,
		ExecutionReceiptSHA256: event.ExecutionReceiptSHA256, PreviousEventSHA256: event.PreviousEventSHA256,
		ErrorClass: event.ErrorClass,
	})
	if err != nil {
		return "", err
	}
	return digest(document), nil
}
