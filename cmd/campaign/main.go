package main

import (
	"encoding/json"
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
	candidatePath := flags.String("candidate", "", "candidate catalog")
	receiptPath := flags.String("receipt", "", "candidate scenario receipt")
	elapsedMilliseconds := flags.Int64("elapsed-milliseconds", 0, "measured campaign duration")
	manualEdits := flags.Bool("manual-generated-file-edits", false, "record manual edits to generated files")
	outputPath := flags.String("output", "", "candidate attestation output")
	var decisions, reconfirmed stringList
	var attemptJSON stringList
	flags.Var(&decisions, "decision", "human decision recorded in the campaign (repeatable)")
	flags.Var(&reconfirmed, "reconfirmed-claim", "automatically reconfirmed catalog claim (repeatable)")
	flags.Var(&attemptJSON, "attempt-json", "campaign attempt provenance as JSON (repeatable, in execution order)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *campaignID == "" || *profileClass == "" || *builderImage == "" || *baselinePath == "" ||
		*candidatePath == "" || *receiptPath == "" || *outputPath == "" || len(attemptJSON) == 0 {
		fmt.Fprintln(stderr, "campaign-id, profile-class, builder-image, baseline, candidate, receipt, attempt-json, and output are required")
		return 2
	}
	baselineInfo, err := os.Stat(*baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "stat baseline catalog: %v\n", err)
		return 1
	}
	candidateInfo, err := os.Stat(*candidatePath)
	if err != nil {
		fmt.Fprintf(stderr, "stat candidate catalog: %v\n", err)
		return 1
	}
	if os.SameFile(baselineInfo, candidateInfo) {
		fmt.Fprintln(stderr, "baseline and candidate catalogs resolve to the same file")
		return 1
	}
	baseline, err := os.ReadFile(*baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "read baseline catalog: %v\n", err)
		return 1
	}
	candidate, err := os.ReadFile(*candidatePath)
	if err != nil {
		fmt.Fprintf(stderr, "read candidate catalog: %v\n", err)
		return 1
	}
	receipt, err := os.ReadFile(*receiptPath)
	if err != nil {
		fmt.Fprintf(stderr, "read scenario receipt: %v\n", err)
		return 1
	}
	attempts, err := parseAttempts(attemptJSON)
	if err != nil {
		fmt.Fprintf(stderr, "decode campaign attempts: %v\n", err)
		return 1
	}
	attestation, err := campaign.BuildAttestation(campaign.Input{
		CampaignID:                     *campaignID,
		ProfileClass:                   *profileClass,
		BuilderImageDigest:             *builderImage,
		BaselinePath:                   *baselinePath,
		CandidatePath:                  *candidatePath,
		BaselineCatalog:                baseline,
		CandidateCatalog:               candidate,
		ScenarioReceipt:                receipt,
		Attempts:                       attempts,
		Elapsed:                        time.Duration(*elapsedMilliseconds) * time.Millisecond,
		HumanDecisions:                 decisions,
		ManualGeneratedFileEdits:       *manualEdits,
		AutomaticallyReconfirmedClaims: reconfirmed,
	})
	if err != nil {
		fmt.Fprintf(stderr, "build attestation: %v\n", err)
		return 1
	}
	if err := writeAtomic(*outputPath, attestation); err != nil {
		fmt.Fprintf(stderr, "write attestation: %v\n", err)
		return 1
	}
	return 0
}

func parseAttempts(documents []string) ([]campaign.Attempt, error) {
	attempts := make([]campaign.Attempt, 0, len(documents))
	for index, document := range documents {
		decoder := json.NewDecoder(strings.NewReader(document))
		decoder.DisallowUnknownFields()
		var attempt campaign.Attempt
		if err := decoder.Decode(&attempt); err != nil {
			return nil, fmt.Errorf("attempt %d: %w", index+1, err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			if err == nil {
				return nil, fmt.Errorf("attempt %d: trailing JSON value", index+1)
			}
			return nil, fmt.Errorf("attempt %d: %w", index+1, err)
		}
		attempts = append(attempts, attempt)
	}
	return attempts, nil
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
