package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ubiquiti-community/go-unifi/internal/scout"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-unifi scout", flag.ContinueOnError)
	flags.SetOutput(stderr)
	targetPath := flags.String("target-profile", "", "immutable target profile JSON")
	scenarioPath := flags.String("scenario", "", "declared scenario JSON")
	specificationPath := flags.String("specification", "", "generated structural specification JSON")
	captureLockPath := flags.String("capture-lock", "", "capture lock JSON")
	responsePath := flags.String("response", "", "optional captured response for offline replay")
	catalogOutput := flags.String("catalog-output", "", "observed catalog output")
	receiptOutput := flags.String("receipt-output", "", "scenario receipt output")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *targetPath == "" || *scenarioPath == "" || *specificationPath == "" ||
		*captureLockPath == "" || *catalogOutput == "" || *receiptOutput == "" {
		fmt.Fprintln(stderr, "target-profile, scenario, specification, capture-lock, catalog-output, and receipt-output are required")
		return 2
	}

	var target scout.TargetProfile
	if err := readStrictJSON(*targetPath, &target); err != nil {
		fmt.Fprintf(stderr, "read target profile: %v\n", err)
		return 1
	}
	var scenario scout.Scenario
	if err := readStrictJSON(*scenarioPath, &scenario); err != nil {
		fmt.Fprintf(stderr, "read scenario: %v\n", err)
		return 1
	}
	specification, err := os.ReadFile(*specificationPath)
	if err != nil {
		fmt.Fprintf(stderr, "read specification: %v\n", err)
		return 1
	}
	captureLock, err := os.ReadFile(*captureLockPath)
	if err != nil {
		fmt.Fprintf(stderr, "read capture lock: %v\n", err)
		return 1
	}
	if !json.Valid(captureLock) {
		fmt.Fprintln(stderr, "read capture lock: invalid JSON")
		return 1
	}

	var observed []byte
	if *responsePath != "" {
		observed, err = os.ReadFile(*responsePath)
		if err != nil {
			fmt.Fprintf(stderr, "read response: %v\n", err)
			return 1
		}
	} else {
		observed, err = observeLive(context.Background(), scenario)
		if err != nil {
			fmt.Fprintf(stderr, "observe controller: %v\n", err)
			return 1
		}
	}

	result, err := scout.BuildDNSCatalog(scout.Input{
		Target:            target,
		Scenario:          scenario,
		CaptureLockSHA256: sha256Hex(captureLock),
		Specification:     specification,
		ObservedResponse:  observed,
	})
	if err != nil {
		fmt.Fprintf(stderr, "build catalog: %v\n", err)
		return 1
	}
	if err := writeAtomic(*catalogOutput, result.Catalog); err != nil {
		fmt.Fprintf(stderr, "write catalog: %v\n", err)
		return 1
	}
	if err := writeAtomic(*receiptOutput, result.Receipt); err != nil {
		fmt.Fprintf(stderr, "write receipt: %v\n", err)
		return 1
	}
	return 0
}

func observeLive(ctx context.Context, scenario scout.Scenario) ([]byte, error) {
	if scenario.Resource != "dns_record" || scenario.Method != "GET" {
		return nil, fmt.Errorf("live scout permits only the declared DNS GET")
	}
	baseURL := os.Getenv("UNIFI_API")
	username := os.Getenv("UNIFI_USERNAME")
	password := os.Getenv("UNIFI_PASSWORD")
	if baseURL == "" || username == "" || password == "" {
		return nil, fmt.Errorf("UNIFI_API, UNIFI_USERNAME, and UNIFI_PASSWORD are required")
	}
	allowInsecure, err := strconv.ParseBool(envDefault("UNIFI_INSECURE", "false"))
	if err != nil {
		return nil, fmt.Errorf("UNIFI_INSECURE: %w", err)
	}
	client, err := unifi.New(ctx, &unifi.Config{
		BaseURL:       baseURL,
		Username:      username,
		Password:      password,
		AllowInsecure: allowInsecure,
	})
	if err != nil {
		return nil, err
	}
	records, err := client.ListDNSRecord(ctx, envDefault("UNIFI_SITE", "default"))
	if err != nil {
		return nil, err
	}
	return json.Marshal(records)
}

func readStrictJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".scout-*")
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
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
