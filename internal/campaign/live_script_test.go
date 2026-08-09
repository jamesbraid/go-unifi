package campaign

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveScriptRecordsProvisioningFailureAndUsesStepNetwork(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required by the live campaign script")
	}
	directory := t.TempDir()
	bin := filepath.Join(directory, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	dockerArgs := filepath.Join(directory, "docker-args")
	fakeDocker := filepath.Join(bin, "docker")
	if err := os.WriteFile(fakeDocker, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" >\"$DOCKER_ARGS_LOG\"\nexit 42\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	command := exec.Command("bash", "campaigns/run-live.sh")
	command.Dir = "../.."
	command.Env = append(os.Environ(),
		"PATH="+bin+":"+os.Getenv("PATH"),
		"DOCKER_ARGS_LOG="+dockerArgs,
		"CAMPAIGN_LIVE_OUTPUT_DIR="+directory,
		"HOSTNAME=synthetic-step",
		// The full reference, not just the digest: run-live.sh compares against
		// image_repository plus image_index_sha256 from the target profile, so a
		// right-digest-wrong-registry value is now rejected before docker runs.
		"UNIFI_NETWORK_IMAGE=ghcr.io/jamesbraid/unifi-network@sha256:584be3a2e45c4913e1bc373eff9c7330609c82085d4fc6f5ea365abdcdb3e664",
		"UNIFI_USERNAME=admin",
		"UNIFI_PASSWORD=admin",
	)
	if err := command.Run(); err == nil {
		t.Fatal("live script succeeded with a failing Docker boundary")
	}

	arguments, err := os.ReadFile(dockerArgs)
	if err != nil {
		t.Fatal(err)
	}
	argumentText := string(arguments)
	if !strings.Contains(argumentText, "--network\ncontainer:synthetic-step\n") || strings.Contains(argumentText, "--publish") {
		t.Fatalf("docker arguments do not use the step network namespace:\n%s", argumentText)
	}

	ledger, err := os.Open(filepath.Join(directory, "outer-attempts.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	states := []string{}
	scanner := bufio.NewScanner(ledger)
	for scanner.Scan() {
		var event struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		states = append(states, event.State)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(states, ",") != "started,failed" {
		t.Fatalf("outer attempt states = %v, want started then failed", states)
	}
}
