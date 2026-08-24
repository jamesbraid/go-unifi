package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/capturelock"
)

type captureConfig struct {
	ArtifactPath           string
	ContentStore           string
	SourceLocation         string
	MediaType              string
	Product                string
	Build                  string
	ExpectedNetworkVersion string
	UOSVersion             string
}

type inspection = capturelock.Inspection

type inspectorFunc func(capturelock.Lock, string) (inspection, error)

func captureArtifact(
	config captureConfig,
	inputs capturelock.Inputs,
	capturedAt time.Time,
	inspect inspectorFunc,
) (capturelock.Lock, error) {
	// Named by flag, all at once, in a fixed order. This used to range over a
	// map and return on the first empty value, so with two inputs missing it
	// reported an arbitrary one of them -- Go randomises map iteration, so
	// which one was not even stable between runs. An operator fixed that one,
	// re-ran, and was told about the next. It also said "media type", which is
	// not something you can pass; -media-type is.
	var missing []string
	for _, required := range []struct {
		flag  string
		value string
	}{
		{"-file or -url", config.ArtifactPath},
		{"-content-store (or GO_UNIFI_CONTENT_STORE)", config.ContentStore},
		{"-source-location", config.SourceLocation},
	} {
		if strings.TrimSpace(required.value) == "" {
			missing = append(missing, required.flag)
		}
	}
	missing = append(missing, requiredFlagInputs(config.MediaType, config.Product, config.Build)...)
	if len(missing) > 0 {
		return capturelock.Lock{}, fmt.Errorf("missing required input: %s", strings.Join(missing, ", "))
	}
	if inspect == nil {
		return capturelock.Lock{}, errors.New("artifact inspector is required")
	}

	stored, err := capturelock.StoreArtifact(config.ArtifactPath, config.ContentStore)
	if err != nil {
		return capturelock.Lock{}, err
	}
	lock := capturelock.Lock{
		FormatVersion: capturelock.FormatVersion,
		Controller: capturelock.Controller{
			Product:    config.Product,
			Build:      config.Build,
			UOSVersion: config.UOSVersion,
		},
		Source: capturelock.Source{
			Location:            config.SourceLocation,
			MediaType:           config.MediaType,
			ByteSize:            stored.ByteSize,
			SHA256:              stored.SHA256,
			ContentStoreLocator: stored.Locator,
		},
		Inputs:     inputs,
		CapturedAt: capturedAt.UTC().Format(time.RFC3339),
	}

	result, err := inspect(lock, config.ContentStore)
	if err != nil {
		return capturelock.Lock{}, fmt.Errorf("inspect retained artifact: %w", err)
	}
	if config.ExpectedNetworkVersion != "" && result.NetworkVersion != config.ExpectedNetworkVersion {
		return capturelock.Lock{}, fmt.Errorf(
			"artifact reports UniFi Network %s, requested %s",
			result.NetworkVersion,
			config.ExpectedNetworkVersion,
		)
	}
	lock.Controller.NetworkVersion = result.NetworkVersion
	lock.Snapshots = result.Snapshots
	if err := lock.Validate(); err != nil {
		return capturelock.Lock{}, fmt.Errorf("complete capture lock: %w", err)
	}
	return lock, nil
}

func downloadArtifact(ctx context.Context, client *http.Client, sourceURL, outputDir string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("download controller artifact: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download controller artifact: HTTP %s", response.Status)
	}
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return "", err
	}
	output, err := os.CreateTemp(outputDir, ".download-*")
	if err != nil {
		return "", err
	}
	filename := output.Name()
	keep := false
	defer func() {
		if !keep {
			os.Remove(filename)
		}
	}()
	if err := output.Chmod(0o600); err != nil {
		output.Close()
		return "", err
	}
	if _, err := io.Copy(output, response.Body); err != nil {
		output.Close()
		return "", err
	}
	if err := output.Close(); err != nil {
		return "", err
	}
	keep = true
	return filename, nil
}

func inspectWithFields(moduleRoot string) inspectorFunc {
	return func(draft capturelock.Lock, contentStore string) (inspection, error) {
		tmpDir, err := os.MkdirTemp("", "go-unifi-capture-inspection-*")
		if err != nil {
			return inspection{}, err
		}
		defer os.RemoveAll(tmpDir)
		draftPath := filepath.Join(tmpDir, "capture.draft.json")
		outputPath := filepath.Join(tmpDir, "inspection.json")
		if err := capturelock.WriteDraftFile(draftPath, draft); err != nil {
			return inspection{}, err
		}
		command := exec.Command(
			"go", "run", "./cmd/fields",
			"-lock="+draftPath,
			"-content-store="+contentStore,
			"-snapshot-output="+outputPath,
		)
		command.Dir = moduleRoot
		command.Env = os.Environ()
		output, err := command.CombinedOutput()
		if err != nil {
			return inspection{}, fmt.Errorf("run schema inspector: %w\n%s", err, output)
		}
		return capturelock.LoadInspectionFile(outputPath)
	}
}

func findModuleRoot(dir string) string {
	for {
		if stat, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && stat.Mode().IsRegular() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// fail reports an operator-facing error and exits. Every one of these was
// panic(err), which printed a Go stack trace rooted in main.go for conditions
// like "you did not pass -media-type". The trace names the tool's internals
// when the thing that went wrong belongs to the caller, and it buries the one
// line that says what to do under twenty that do not.
// requiredFlagInputs names the inputs that come only from flags, in a fixed
// order. Kept in one place because it is checked twice: once before the
// artifact is fetched, so a missing flag costs nothing, and once inside
// captureArtifact, which has callers other than main.
func requiredFlagInputs(mediaType, product, build string) []string {
	var missing []string
	for _, required := range []struct {
		flag  string
		value string
	}{
		{"-media-type", mediaType},
		{"-product", product},
		{"-build", build},
	} {
		if strings.TrimSpace(required.value) == "" {
			missing = append(missing, required.flag)
		}
	}
	return missing
}

// sameLock compares two locks by their serialized form, so a field added later
// is included automatically rather than silently escaping the comparison.
func sameLock(a, b capturelock.Lock) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "schema-capture: %v\n", err)
	os.Exit(1)
}

func main() {
	artifactFile := flag.String("file", "", "Local controller artifact to capture")
	artifactURL := flag.String("url", "", "Controller artifact URL to download and capture")
	sourceLocation := flag.String("source-location", "", "Original artifact location (required with -file)")
	mediaType := flag.String("media-type", "", "Artifact media type")
	product := flag.String("product", "", "Controller product identity")
	build := flag.String("build", "", "Controller build identity")
	expectedNetwork := flag.String("network-version", "", "Expected UniFi Network version")
	uosVersion := flag.String("uos-version", "", "UniFi OS version, when applicable")
	contentStore := flag.String("content-store", os.Getenv("GO_UNIFI_CONTENT_STORE"), "Restricted content store")
	output := flag.String("output", "", "Proposed lock output (default: schemas/capture.lock.json)")
	capturedAtFlag := flag.String("captured-at", "", "Capture time in RFC3339 (default: current UTC time)")
	flag.Parse()

	// Before the artifact is fetched. This used to be checked only inside
	// captureArtifact, which runs after the download, so a forgotten flag cost
	// a 130 MB transfer and then reported itself as a panic.
	if missing := requiredFlagInputs(*mediaType, *product, *build); len(missing) > 0 {
		fail(fmt.Errorf("missing required input: %s", strings.Join(missing, ", ")))
	}
	if (*artifactFile == "") == (*artifactURL == "") {
		fail(errors.New("exactly one of -file or -url is required"))
	}
	wd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	moduleRoot := findModuleRoot(wd)
	if moduleRoot == "" {
		fail(errors.New("unable to locate module root: run this from inside the repository"))
	}
	if *output == "" {
		*output = filepath.Join(moduleRoot, "schemas", "capture.lock.json")
	}
	if *sourceLocation == "" {
		*sourceLocation = *artifactURL
	}
	if *contentStore == "" {
		fail(errors.New("-content-store or GO_UNIFI_CONTENT_STORE is required"))
	}

	capturedAt := time.Now().UTC()
	if *capturedAtFlag != "" {
		capturedAt, err = time.Parse(time.RFC3339, *capturedAtFlag)
		if err != nil {
			fail(fmt.Errorf("parse -captured-at: %w", err))
		}
	}
	artifactPath := *artifactFile
	var downloadDir string
	if *artifactURL != "" {
		if err := os.MkdirAll(*contentStore, 0o700); err != nil {
			fail(err)
		}
		downloadDir, err = os.MkdirTemp(*contentStore, ".capture-download-*")
		if err != nil {
			fail(err)
		}
		defer os.RemoveAll(downloadDir)
		artifactPath, err = downloadArtifact(
			context.Background(),
			&http.Client{Timeout: 2 * time.Hour},
			*artifactURL,
			downloadDir,
		)
		if err != nil {
			fail(err)
		}
	}
	inputs, err := capturelock.ComputeInputDigests(moduleRoot)
	if err != nil {
		fail(err)
	}

	lock, err := captureArtifact(captureConfig{
		ArtifactPath:           artifactPath,
		ContentStore:           *contentStore,
		SourceLocation:         *sourceLocation,
		MediaType:              *mediaType,
		Product:                *product,
		Build:                  *build,
		ExpectedNetworkVersion: *expectedNetwork,
		UOSVersion:             *uosVersion,
	}, inputs, capturedAt, inspectWithFields(moduleRoot))
	if err != nil {
		fail(err)
	}
	// Keep the previous timestamp when nothing else moved. captured_at is the
	// only field that changes on a re-capture of identical bytes, and letting
	// it change is not cosmetic churn: a byte-identical re-capture must
	// produce no diff. With the timestamp always moving, a capture of the
	// locked version produced a one-line diff, which is a commit, a branch
	// push, a CI run and a pull request proposing nothing.
	if previousLock, prevErr := capturelock.LoadFile(*output); prevErr == nil {
		normalized := lock
		normalized.CapturedAt = previousLock.CapturedAt
		if sameLock(normalized, previousLock) {
			lock.CapturedAt = previousLock.CapturedAt
			fmt.Println("capture is identical to the committed lock; keeping its captured_at")
		}
	}

	if err := capturelock.WriteFile(*output, lock); err != nil {
		fail(err)
	}
	fmt.Printf("proposed capture lock %s for sha256:%s\n", *output, lock.Source.SHA256)
}
