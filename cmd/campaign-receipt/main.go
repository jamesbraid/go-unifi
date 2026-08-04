package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ubiquiti-community/go-unifi/internal/campaign"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-unifi campaign-receipt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	workflow := flags.String("workflow", "", "checked runner workflow")
	controllerFingerprint := flags.String("controller-fingerprint", "", "fixture controller fingerprint")
	provisionerPath := flags.String("provisioner-receipt", "", "optional independently measured live provisioner receipt")
	output := flags.String("output", "", "execution receipt output")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *workflow == "" || *output == "" || (*controllerFingerprint == "" && *provisionerPath == "") {
		fmt.Fprintln(stderr, "workflow, output, and controller-fingerprint or provisioner-receipt are required")
		return 2
	}
	evidence, err := campaign.MeasureRunnerEvidence(*workflow)
	if err != nil {
		fmt.Fprintf(stderr, "measure runner evidence: %v\n", err)
		return 1
	}
	var provisioner []byte
	if *provisionerPath != "" {
		provisioner, err = os.ReadFile(*provisionerPath)
		if err != nil {
			fmt.Fprintf(stderr, "read provisioner receipt: %v\n", err)
			return 1
		}
	}
	receipt, err := campaign.BuildRunnerExecutionReceipt(evidence, *controllerFingerprint, provisioner)
	if err != nil {
		fmt.Fprintf(stderr, "build runner execution receipt: %v\n", err)
		return 1
	}
	if err := writeAtomic(*output, receipt); err != nil {
		fmt.Fprintf(stderr, "write execution receipt: %v\n", err)
		return 1
	}
	return 0
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".campaign-receipt-*")
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
