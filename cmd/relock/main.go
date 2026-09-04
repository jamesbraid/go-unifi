// Command relock recomputes the capture lock's input digests for the current
// tree and rewrites the two digest values in schemas/capture.lock.json in
// place, touching nothing else.
//
// The digests tie a generated tree to the exact inputs that produced it. Any
// deliberate change to those inputs -- generator code, an override, the
// behaviour artifact, or a dependency bump moving go.mod/go.sum -- must be
// followed by a re-lock, or verification refuses the tree. Renovate's
// post-upgrade task runs this so a dependency bump carries its own re-lock
// rather than arriving pre-broken.
//
// It edits the two values textually rather than round-tripping the JSON,
// because re-encoding a decoded map reorders the file's keys and would bury a
// one-value change under a whole-file diff.
package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/ubiquiti-community/go-unifi/internal/capturelock"
)

const path = "schemas/capture.lock.json"

func main() {
	inputs, err := capturelock.ComputeInputDigests(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "relock: %v\n", err)
		os.Exit(1)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "relock: %v\n", err)
		os.Exit(1)
	}
	out := raw
	for field, want := range map[string]string{
		"extraction_rules_sha256": inputs.ExtractionRulesSHA256,
		"generator_inputs_sha256": inputs.GeneratorInputsSHA256,
	} {
		re := regexp.MustCompile(`("` + field + `"\s*:\s*")[0-9a-f]{64}(")`)
		if !re.Match(out) {
			fmt.Fprintf(os.Stderr, "relock: %s not found in %s\n", field, path)
			os.Exit(1)
		}
		out = re.ReplaceAll(out, []byte(`${1}`+want+`${2}`))
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "relock: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("relocked: generator_inputs=%s extraction_rules=%s\n",
		inputs.GeneratorInputsSHA256[:12], inputs.ExtractionRulesSHA256[:12])
}
