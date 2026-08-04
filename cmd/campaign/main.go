package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/campaign"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-unifi campaign", flag.ContinueOnError)
	flags.SetOutput(stderr)
	campaignID := flags.String("campaign-id", "", "stable campaign identifier")
	profileClass := flags.String("profile-class", "", "fresh_seeded, persisted_single_hop, or long_lived_multi_hop")
	builderImage := flags.String("builder-image", "", "digest-pinned builder image")
	baselinePath := flags.String("baseline", "", "admitted baseline catalog")
	admissionReceiptPath := flags.String("admission-receipt", "", "trusted baseline admission receipt")
	candidatePath := flags.String("candidate", "", "candidate catalog")
	receiptPath := flags.String("receipt", "", "candidate scenario receipt")
	executionReceiptPath := flags.String("execution-receipt", "", "fixture- or runner-scoped execution receipt")
	provisionerReceiptPath := flags.String("provisioner-receipt", "", "independent live provisioner receipt")
	runnerWorkflowPath := flags.String("runner-workflow", "", "checked workflow measured for runner execution")
	attemptLedgerPath := flags.String("attempt-ledger", "", "append-only NDJSON attempt ledger")
	attemptID := flags.String("attempt-id", "", "unique campaign attempt identifier")
	elapsedMilliseconds := flags.Int64("elapsed-milliseconds", 0, "measured campaign duration")
	manualEdits := flags.Bool("manual-generated-file-edits", false, "record manual edits to generated files")
	outputPath := flags.String("output", "", "candidate attestation output")
	var decisions, reconfirmed stringList
	flags.Var(&decisions, "decision", "human decision recorded in the campaign (repeatable)")
	flags.Var(&reconfirmed, "reconfirmed-claim", "automatically reconfirmed catalog claim (repeatable)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *campaignID == "" || *profileClass == "" || *builderImage == "" || *baselinePath == "" || *admissionReceiptPath == "" ||
		*candidatePath == "" || *receiptPath == "" || *executionReceiptPath == "" || *attemptLedgerPath == "" || *attemptID == "" || *outputPath == "" {
		fmt.Fprintln(stderr, "campaign-id, profile-class, builder-image, baseline, admission-receipt, candidate, receipt, execution-receipt, attempt-ledger, attempt-id, and output are required")
		return 2
	}
	executionReceipt, err := os.ReadFile(*executionReceiptPath)
	if err != nil {
		fmt.Fprintf(stderr, "read execution receipt: %v\n", err)
		return 1
	}
	if err := campaign.BeginAttempt(*attemptLedgerPath, *attemptID, executionReceipt); err != nil {
		fmt.Fprintf(stderr, "begin campaign attempt: %v\n", err)
		return 1
	}
	fail := func(errorClass, format string, args ...any) int {
		fmt.Fprintf(stderr, format+"\n", args...)
		if err := campaign.FinishAttempt(*attemptLedgerPath, *attemptID, "failed", errorClass); err != nil {
			fmt.Fprintf(stderr, "finish failed campaign attempt: %v\n", err)
		}
		return 1
	}
	baselineInfo, err := os.Stat(*baselinePath)
	if err != nil {
		return fail("input_read", "stat baseline catalog: %v", err)
	}
	candidateInfo, err := os.Stat(*candidatePath)
	if err != nil {
		return fail("input_read", "stat candidate catalog: %v", err)
	}
	if os.SameFile(baselineInfo, candidateInfo) {
		return fail("input_validation", "baseline and candidate catalogs resolve to the same file")
	}
	baseline, err := os.ReadFile(*baselinePath)
	if err != nil {
		return fail("input_read", "read baseline catalog: %v", err)
	}
	admissionReceipt, err := os.ReadFile(*admissionReceiptPath)
	if err != nil {
		return fail("input_read", "read admission receipt: %v", err)
	}
	candidate, err := os.ReadFile(*candidatePath)
	if err != nil {
		return fail("input_read", "read candidate catalog: %v", err)
	}
	receipt, err := os.ReadFile(*receiptPath)
	if err != nil {
		return fail("input_read", "read scenario receipt: %v", err)
	}
	var provisionerReceipt []byte
	if *provisionerReceiptPath != "" {
		provisionerReceipt, err = os.ReadFile(*provisionerReceiptPath)
		if err != nil {
			return fail("input_read", "read provisioner receipt: %v", err)
		}
	}
	var runnerEvidence *campaign.RunnerEvidence
	if *runnerWorkflowPath != "" {
		measured, measureErr := campaign.MeasureRunnerEvidence(*runnerWorkflowPath)
		if measureErr != nil {
			return fail("runner_measurement", "measure runner evidence: %v", measureErr)
		}
		runnerEvidence = &measured
	}
	attestation, err := campaign.BuildAttestation(campaign.Input{
		CampaignID:                     *campaignID,
		ProfileClass:                   *profileClass,
		BuilderImageDigest:             *builderImage,
		BaselinePath:                   *baselinePath,
		CandidatePath:                  *candidatePath,
		BaselineCatalog:                baseline,
		AdmissionReceipt:               admissionReceipt,
		CandidateCatalog:               candidate,
		ScenarioReceipt:                receipt,
		ExecutionReceipt:               executionReceipt,
		RunnerEvidence:                 runnerEvidence,
		ProvisionerReceipt:             provisionerReceipt,
		Elapsed:                        time.Duration(*elapsedMilliseconds) * time.Millisecond,
		HumanDecisions:                 decisions,
		ManualGeneratedFileEdits:       *manualEdits,
		AutomaticallyReconfirmedClaims: reconfirmed,
	})
	if err != nil {
		return fail("campaign_validation", "build attestation: %v", err)
	}
	if err := writeAtomic(*outputPath, attestation); err != nil {
		return fail("attestation_write", "write attestation: %v", err)
	}
	if err := campaign.FinishAttempt(*attemptLedgerPath, *attemptID, "succeeded", ""); err != nil {
		fmt.Fprintf(stderr, "finish successful campaign attempt: %v\n", err)
		return 1
	}
	return 0
}

type stringList []string

func (values *stringList) String() string { return strings.Join(*values, ",") }

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".campaign-*")
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
