// Command evidence-digest prints the digests the capture lock and the
// structural projections pin each other by.
//
// Both values were previously computable only inside internal/scout. When a
// review legitimately fires, the operator has to put a new digest in
// schemas/capture.lock.json, and a gate whose expected value cannot be produced
// by any shipped tool is one that gets satisfied by trial and error against the
// failure message. This prints it.
//
//	evidence-digest -canonical schemas/structural/dns_record.json
//	    the value for scout.dns_structural_projection_sha256, and for
//	    scout.dns_semantic_predecessor_sha256 given that document
//
//	evidence-digest -raw schemas/fields/DnsRecord.json
//	    the value for a projection's source.field_document_sha256, and for the
//	    matching entry in the lock's snapshots.field_documents
//
//	evidence-digest -tree schemas/fields
//	    the value for snapshots.structural_sha256
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ubiquiti-community/go-unifi/internal/capturelock"
	"github.com/ubiquiti-community/go-unifi/internal/scout"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-unifi evidence-digest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	canonical := flags.String("canonical", "", "JSON evidence document to digest canonically, as the capture lock scout block pins it")
	raw := flags.String("raw", "", "file to digest byte for byte, as a structural projection pins its source field document")
	tree := flags.String("tree", "", "directory to digest as a snapshot, as the capture lock pins the structural snapshot")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	selected := 0
	for _, value := range []string{*canonical, *raw, *tree} {
		if value != "" {
			selected++
		}
	}
	if selected != 1 {
		fmt.Fprintln(stderr, "exactly one of -canonical, -raw, or -tree is required")
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", flags.Arg(0))
		return 2
	}

	if *raw != "" {
		digest, err := capturelock.DigestFile(*raw)
		if err != nil {
			fmt.Fprintf(stderr, "digest %s: %v\n", *raw, err)
			return 1
		}
		fmt.Fprintln(stdout, digest)
		return 0
	}
	if *tree != "" {
		digest, err := capturelock.DigestTree(*tree)
		if err != nil {
			fmt.Fprintf(stderr, "digest %s: %v\n", *tree, err)
			return 1
		}
		fmt.Fprintln(stdout, digest)
		return 0
	}

	document, err := os.ReadFile(*canonical)
	if err != nil {
		fmt.Fprintf(stderr, "read %s: %v\n", *canonical, err)
		return 1
	}
	digest, err := scout.CanonicalDocumentDigest(document)
	if err != nil {
		fmt.Fprintf(stderr, "canonicalize %s: %v\n", *canonical, err)
		return 1
	}
	fmt.Fprintln(stdout, digest)
	return 0
}
