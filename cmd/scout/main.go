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

	"github.com/ubiquiti-community/go-unifi/internal/capturelock"
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
	structuralPath := flags.String("structural", "", "policy-free locked structural projection JSON")
	semanticPredecessorPath := flags.String("semantic-predecessor", "", "immutable predecessor semantic ID registry JSON")
	semanticIDsPath := flags.String("semantic-ids", "", "reviewed stable semantic ID registry JSON")
	captureLockPath := flags.String("capture-lock", "", "capture lock JSON")
	targetReceiptPath := flags.String("target-receipt", "", "provisioner-measured live target receipt JSON")
	responsePath := flags.String("response", "", "optional captured response for offline replay")
	catalogOutput := flags.String("catalog-output", "", "observed catalog output")
	receiptOutput := flags.String("receipt-output", "", "scenario receipt output")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *targetPath == "" || *scenarioPath == "" || *structuralPath == "" || *semanticPredecessorPath == "" || *semanticIDsPath == "" ||
		*captureLockPath == "" || *catalogOutput == "" || *receiptOutput == "" {
		fmt.Fprintln(stderr, "target-profile, scenario, structural, semantic-predecessor, semantic-ids, capture-lock, catalog-output, and receipt-output are required")
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
	structural, err := os.ReadFile(*structuralPath)
	if err != nil {
		fmt.Fprintf(stderr, "read structural projection: %v\n", err)
		return 1
	}
	semanticPredecessor, err := os.ReadFile(*semanticPredecessorPath)
	if err != nil {
		fmt.Fprintf(stderr, "read semantic predecessor: %v\n", err)
		return 1
	}
	semanticIDs, err := os.ReadFile(*semanticIDsPath)
	if err != nil {
		fmt.Fprintf(stderr, "read semantic IDs: %v\n", err)
		return 1
	}
	captureLockBytes, err := os.ReadFile(*captureLockPath)
	if err != nil {
		fmt.Fprintf(stderr, "read capture lock: %v\n", err)
		return 1
	}
	lock, err := capturelock.LoadFile(*captureLockPath)
	if err != nil {
		fmt.Fprintf(stderr, "read capture lock: %v\n", err)
		return 1
	}
	if target.Version != lock.Controller.NetworkVersion {
		fmt.Fprintf(stderr, "target version %s does not match capture lock Network version %s\n", target.Version, lock.Controller.NetworkVersion)
		return 1
	}
	if lock.Scout == nil {
		fmt.Fprintln(stderr, "capture lock does not pin scout evidence digests")
		return 1
	}
	// The lock is scout's only account of the field definitions, and has to
	// be: schemas/fields/ is extracted from the controller artifact and is
	// gitignored, so in a fresh checkout there is nothing on disk to measure
	// and no artifact to re-extract it from. What keeps these entries honest
	// is cmd/fields, which re-measures the tree it just extracted and refuses
	// to generate against a lock whose per-document digests disagree with it.
	fieldDigests := scout.FieldDocumentDigests(lock.Snapshots.FieldDocuments)

	var observed []byte
	var targetReceipt *scout.ProvisionerTargetReceipt
	controllerVersion := ""
	observedInstanceIdentitySHA256 := ""
	executionMode := "fixture"
	if *responsePath != "" {
		if *targetReceiptPath != "" {
			fmt.Fprintln(stderr, "fixture execution forbids target-receipt")
			return 1
		}
		observed, err = os.ReadFile(*responsePath)
		if err != nil {
			fmt.Fprintf(stderr, "read response: %v\n", err)
			return 1
		}
	} else {
		if *targetReceiptPath == "" {
			fmt.Fprintln(stderr, "live execution requires target-receipt")
			return 1
		}
		var receipt scout.ProvisionerTargetReceipt
		if err := readStrictJSON(*targetReceiptPath, &receipt); err != nil {
			fmt.Fprintf(stderr, "read target receipt: %v\n", err)
			return 1
		}
		targetReceipt = &receipt
		executionMode = "live"
		observation, observeErr := observeLive(context.Background(), scenario)
		err = observeErr
		if err != nil {
			fmt.Fprintf(stderr, "observe controller: %v\n", err)
			return 1
		}
		observed = observation.Response
		controllerVersion = observation.ControllerVersion
		observedInstanceIdentitySHA256 = observation.InstanceIdentitySHA256
	}

	result, err := scout.BuildDNSCatalog(scout.Input{
		Target:                         target,
		TargetReceipt:                  targetReceipt,
		ControllerVersion:              controllerVersion,
		ObservedInstanceIdentitySHA256: observedInstanceIdentitySHA256,
		Scenario:                       scenario,
		ExecutionMode:                  executionMode,
		LockedSources: scout.LockedSources{
			CaptureLockSHA256:          sha256Hex(captureLockBytes),
			ControllerNetworkVersion:   lock.Controller.NetworkVersion,
			ExtractionRulesSHA256:      lock.Inputs.ExtractionRulesSHA256,
			StructuralSHA256:           lock.Snapshots.StructuralSHA256,
			SensitivitySHA256:          lock.Snapshots.SensitivitySHA256,
			StructuralProjectionSHA256: lock.Scout.DNSStructuralProjectionSHA256,
			SemanticPredecessorSHA256:  lock.Scout.DNSSemanticPredecessorSHA256,
		},
		FieldDocumentDigests: fieldDigests,
		StructuralProjection: structural,
		SemanticPredecessor:  semanticPredecessor,
		SemanticIDs:          semanticIDs,
		ObservedResponse:     observed,
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

type liveObservation struct {
	Response               []byte
	ControllerVersion      string
	InstanceIdentitySHA256 string
}

func observeLive(ctx context.Context, scenario scout.Scenario) (liveObservation, error) {
	if scenario.Resource != "dns_record" || scenario.Method != "GET" || scenario.Path != "/v2/api/site/{site}/static-dns" {
		return liveObservation{}, fmt.Errorf("live scout permits only the declared DNS GET")
	}
	baseURL := os.Getenv("UNIFI_API")
	username := os.Getenv("UNIFI_USERNAME")
	password := os.Getenv("UNIFI_PASSWORD")
	if baseURL == "" || username == "" || password == "" {
		return liveObservation{}, fmt.Errorf("UNIFI_API, UNIFI_USERNAME, and UNIFI_PASSWORD are required")
	}
	allowInsecure, err := strconv.ParseBool(envDefault("UNIFI_INSECURE", "false"))
	if err != nil {
		return liveObservation{}, fmt.Errorf("UNIFI_INSECURE: %w", err)
	}
	client, err := unifi.New(ctx, &unifi.Config{
		BaseURL:       baseURL,
		Username:      username,
		Password:      password,
		AllowInsecure: allowInsecure,
	})
	if err != nil {
		return liveObservation{}, err
	}
	response, err := client.ListDNSRecordRaw(ctx, envDefault("UNIFI_SITE", "default"))
	if err != nil {
		return liveObservation{}, err
	}
	if client.Version() == "" {
		return liveObservation{}, fmt.Errorf("controller did not report a Network version")
	}
	if client.ControllerUUID() == "" {
		return liveObservation{}, fmt.Errorf("controller UUID is required for live evidence")
	}
	instanceIdentitySHA256, err := scout.InstanceIdentitySHA256(client.ControllerUUID())
	if err != nil {
		return liveObservation{}, err
	}
	return liveObservation{
		Response:               response,
		ControllerVersion:      client.Version(),
		InstanceIdentitySHA256: instanceIdentitySHA256,
	}, nil
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
