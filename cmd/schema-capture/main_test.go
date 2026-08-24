package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/capturelock"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestCaptureArtifactBuildsCompleteLock(t *testing.T) {
	artifactBytes := []byte("controller artifact")
	artifact := filepath.Join(t.TempDir(), "controller.deb")
	if err := os.WriteFile(artifact, artifactBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	store := t.TempDir()
	inputs := capturelock.Inputs{
		ExtractionRulesSHA256: strings.Repeat("a", 64),
		GeneratorInputsSHA256: strings.Repeat("b", 64),
	}
	wantSnapshots := capturelock.Snapshots{
		StructuralSHA256:  strings.Repeat("c", 64),
		SensitivitySHA256: strings.Repeat("d", 64),
		FieldDocuments:    map[string]string{"DnsRecord.json": strings.Repeat("e", 64)},
	}
	capturedAt := time.Date(2026, time.August, 3, 12, 34, 56, 0, time.UTC)

	lock, err := captureArtifact(captureConfig{
		ArtifactPath:   artifact,
		ContentStore:   store,
		SourceLocation: "https://downloads.example.invalid/unifi.deb",
		MediaType:      "application/vnd.debian.binary-package",
		Product:        "unifi-controller",
		Build:          "v10.4.57+build record-id",
		UOSVersion:     "5.1.21",
	}, inputs, capturedAt, func(draft capturelock.Lock, gotStore string) (inspection, error) {
		if gotStore != store {
			t.Fatalf("inspector store = %q, want %q", gotStore, store)
		}
		if draft.Controller.NetworkVersion != "" || !reflect.DeepEqual(draft.Snapshots, capturelock.Snapshots{}) {
			t.Fatalf("inspector received completed draft: %#v", draft)
		}
		if _, err := capturelock.ResolveArtifact(store, func() capturelock.Lock {
			check := draft
			check.Controller.NetworkVersion = "10.4.57"
			check.Snapshots = wantSnapshots
			return check
		}()); err != nil {
			t.Fatalf("stored artifact did not verify: %v", err)
		}
		return inspection{NetworkVersion: "10.4.57", Snapshots: wantSnapshots}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Validate(); err != nil {
		t.Fatalf("complete lock did not validate: %v", err)
	}
	if lock.Controller.NetworkVersion != "10.4.57" || lock.Controller.UOSVersion != "5.1.21" {
		t.Fatalf("controller identity = %#v", lock.Controller)
	}
	if lock.Inputs != inputs || !reflect.DeepEqual(lock.Snapshots, wantSnapshots) {
		t.Fatalf("lock inputs/snapshots = %#v / %#v", lock.Inputs, lock.Snapshots)
	}
	if lock.CapturedAt != "2026-08-03T12:34:56Z" {
		t.Fatalf("captured_at = %q", lock.CapturedAt)
	}
}

func TestCaptureArtifactRejectsRequestedNetworkVersionMismatch(t *testing.T) {
	artifact := filepath.Join(t.TempDir(), "controller.deb")
	if err := os.WriteFile(artifact, []byte("controller artifact"), 0o644); err != nil {
		t.Fatal(err)
	}
	inputs := capturelock.Inputs{
		ExtractionRulesSHA256: strings.Repeat("a", 64),
		GeneratorInputsSHA256: strings.Repeat("b", 64),
	}

	_, err := captureArtifact(captureConfig{
		ArtifactPath:           artifact,
		ContentStore:           t.TempDir(),
		SourceLocation:         "https://downloads.example.invalid/unifi.deb",
		MediaType:              "application/vnd.debian.binary-package",
		Product:                "unifi-controller",
		Build:                  "v10.4.57+build record-id",
		ExpectedNetworkVersion: "10.4.57",
	}, inputs, time.Now(), func(capturelock.Lock, string) (inspection, error) {
		return inspection{
			NetworkVersion: "10.5.0",
			Snapshots: capturelock.Snapshots{
				StructuralSHA256:  strings.Repeat("c", 64),
				SensitivitySHA256: strings.Repeat("d", 64),
			},
		}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "reports UniFi Network 10.5.0") {
		t.Fatalf("captureArtifact() error = %v, want version mismatch", err)
	}
}

func TestDownloadArtifactStreamsSuccessfulResponseAndRejectsHTTPFailure(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://downloads.example.invalid/controller.deb" {
			t.Fatalf("request URL = %q", request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("artifact bytes")),
			Header:     make(http.Header),
		}, nil
	})}

	filename, err := downloadArtifact(
		context.Background(),
		client,
		"https://downloads.example.invalid/controller.deb",
		t.TempDir(),
	)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "artifact bytes" {
		t.Fatalf("downloaded bytes = %q", got)
	}

	client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Status:     "404 Not Found",
			Body:       io.NopCloser(strings.NewReader("missing")),
			Header:     make(http.Header),
		}, nil
	})
	_, err = downloadArtifact(
		context.Background(),
		client,
		"https://downloads.example.invalid/controller.deb",
		t.TempDir(),
	)
	if err == nil || !strings.Contains(err.Error(), "404 Not Found") {
		t.Fatalf("downloadArtifact() error = %v, want HTTP failure", err)
	}
}
