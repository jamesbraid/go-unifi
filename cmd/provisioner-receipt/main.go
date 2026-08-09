package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/scout"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-unifi provisioner-receipt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	containerID := flags.String("container-id", "", "started disposable Network container")
	imageReference := flags.String("image", "", "digest-pinned Network image index reference")
	profileName := flags.String("profile-name", "network-10.4.57-seeded", "locked target profile name")
	expectedVersion := flags.String("expected-version", "", "locked Network version")
	expectedIndex := flags.String("expected-image-index", "", "locked image index digest")
	expectedManifest := flags.String("expected-image-manifest", "", "locked platform manifest digest")
	architecture := flags.String("architecture", "amd64", "locked target architecture")
	output := flags.String("output", "", "provisioner receipt output")
	// Onboarding a controller needs the profile's controller_fingerprint, and
	// that value must come from this package's own function over a real
	// receipt. Recomputing the canonical digest anywhere else would be a second
	// implementation of an identity, and scout hard-errors on mismatch, so a
	// divergence would only surface later as a refusal nobody can explain.
	fingerprintOutput := flags.String("fingerprint-output", "", "write the profile controller_fingerprint for this receipt")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *containerID == "" || *imageReference == "" || *expectedVersion == "" || *expectedIndex == "" || *expectedManifest == "" || *output == "" {
		fmt.Fprintln(stderr, "container-id, image, expected-version, expected-image-index, expected-image-manifest, and output are required")
		return 2
	}
	if !strings.HasSuffix(*imageReference, "@"+*expectedIndex) {
		fmt.Fprintln(stderr, "Network image reference does not name the locked index digest")
		return 1
	}
	observedArchitecture, err := inspectStartedContainer(*containerID, *imageReference)
	if err != nil {
		fmt.Fprintf(stderr, "inspect started container: %v\n", err)
		return 1
	}
	if observedArchitecture != *architecture {
		fmt.Fprintf(stderr, "container architecture %q does not match locked architecture %q\n", observedArchitecture, *architecture)
		return 1
	}
	manifestOutput, err := exec.Command("docker", "manifest", "inspect", *imageReference).Output()
	if err != nil {
		fmt.Fprintf(stderr, "inspect image index: %v\n", dockerError(err))
		return 1
	}
	manifestDigest, err := selectManifest(manifestOutput, *architecture)
	if err != nil {
		fmt.Fprintf(stderr, "inspect image index: %v\n", err)
		return 1
	}
	if manifestDigest != *expectedManifest {
		fmt.Fprintf(stderr, "image manifest %q does not match locked manifest %q\n", manifestDigest, *expectedManifest)
		return 1
	}
	version, uuid, err := observeControllerIdentity(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "observe controller identity: %v\n", err)
		return 1
	}
	if version != *expectedVersion {
		fmt.Fprintf(stderr, "controller version %q does not match locked version %q\n", version, *expectedVersion)
		return 1
	}
	instanceIdentity, err := scout.InstanceIdentitySHA256(uuid)
	if err != nil {
		fmt.Fprintf(stderr, "hash controller identity: %v\n", err)
		return 1
	}
	receipt := scout.ProvisionerTargetReceipt{
		FormatVersion: 1, ProfileName: *profileName, Product: "unifi-network", Version: version,
		Architecture: *architecture, ImageIndexSHA256: *expectedIndex, ImageManifestSHA256: manifestDigest,
		InstanceIdentitySHA256: instanceIdentity,
	}
	document, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "encode provisioner receipt: %v\n", err)
		return 1
	}
	document = append(document, '\n')
	if err := writeAtomic(*output, document); err != nil {
		fmt.Fprintf(stderr, "write provisioner receipt: %v\n", err)
		return 1
	}
	if *fingerprintOutput != "" {
		fingerprint, err := scout.ProvisionerReceiptFingerprint(receipt)
		if err != nil {
			fmt.Fprintf(stderr, "derive controller fingerprint: %v\n", err)
			return 1
		}
		if err := writeAtomic(*fingerprintOutput, []byte(fingerprint+"\n")); err != nil {
			fmt.Fprintf(stderr, "write controller fingerprint: %v\n", err)
			return 1
		}
	}
	return 0
}

// dockerError surfaces what the docker CLI actually wrote. exec.Cmd.Output()
// already captures stderr into ExitError.Stderr, so the explanation was being
// collected and then discarded at print time -- a failure here reported only
// "exit status 1", which is true and useless.
func dockerError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return err
}

func inspectStartedContainer(containerID, imageReference string) (string, error) {
	containerOutput, err := exec.Command("docker", "container", "inspect", containerID).Output()
	if err != nil {
		return "", dockerError(err)
	}
	var containers []struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(containerOutput, &containers); err != nil || len(containers) != 1 {
		return "", fmt.Errorf("decode container inspection")
	}
	if !containers[0].State.Running || containers[0].Config.Image != imageReference {
		return "", fmt.Errorf("container is not running the exact locked image reference")
	}
	imageOutput, err := exec.Command("docker", "image", "inspect", imageReference).Output()
	if err != nil {
		return "", dockerError(err)
	}
	var images []struct {
		Architecture string `json:"Architecture"`
	}
	if err := json.Unmarshal(imageOutput, &images); err != nil || len(images) != 1 || images[0].Architecture == "" {
		return "", fmt.Errorf("decode image inspection")
	}
	return images[0].Architecture, nil
}

func selectManifest(document []byte, architecture string) (string, error) {
	var index struct {
		Manifests []struct {
			Digest   string `json:"digest"`
			Platform struct {
				Architecture string `json:"architecture"`
				OS           string `json:"os"`
			} `json:"platform"`
		} `json:"manifests"`
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&index); err != nil {
		// Docker includes optional descriptor fields; decode those without
		// weakening the receipt's own strict JSON validation.
		if err := json.Unmarshal(document, &index); err != nil {
			return "", err
		}
	}
	for _, manifest := range index.Manifests {
		if manifest.Platform.OS == "linux" && manifest.Platform.Architecture == architecture {
			return manifest.Digest, nil
		}
	}
	return "", fmt.Errorf("no linux/%s manifest in image index", architecture)
}

func observeControllerIdentity(ctx context.Context) (string, string, error) {
	api := os.Getenv("UNIFI_API")
	username := os.Getenv("UNIFI_USERNAME")
	password := os.Getenv("UNIFI_PASSWORD")
	if api == "" || username == "" || password == "" {
		return "", "", fmt.Errorf("UNIFI_API, UNIFI_USERNAME, and UNIFI_PASSWORD are required")
	}
	deadline := time.Now().Add(5 * time.Minute)
	var lastErr error
	for time.Now().Before(deadline) {
		client, err := unifi.New(ctx, &unifi.Config{BaseURL: api, Username: username, Password: password, AllowInsecure: true})
		if err == nil && client.Version() != "" && client.ControllerUUID() != "" {
			return client.Version(), client.ControllerUUID(), nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("controller did not report version and UUID")
		}
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return "", "", fmt.Errorf("controller did not become ready: %w", lastErr)
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".provisioner-receipt-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
