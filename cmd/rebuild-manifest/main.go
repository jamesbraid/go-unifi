// Command rebuild-manifest hashes and compares the complete generated output.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ubiquiti-community/go-unifi/internal/rebuild"
)

func main() {
	root := flag.String("root", ".", "Repository root")
	output := flag.String("output", "", "Manifest output path")
	compare := flag.String("compare", "", "Earlier manifest to compare")
	flag.Parse()

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		panic(err)
	}
	manifest, err := rebuild.BuildManifest(absRoot)
	if err != nil {
		panic(err)
	}
	if *compare != "" {
		want, err := rebuild.LoadManifest(*compare)
		if err != nil {
			panic(err)
		}
		if err := rebuild.Compare(want, manifest); err != nil {
			panic(err)
		}
	}
	if *output != "" {
		if err := rebuild.WriteManifest(*output, manifest); err != nil {
			panic(err)
		}
	}
	fmt.Fprintf(os.Stdout, "%s\n", manifest.OutputSHA256)
}
