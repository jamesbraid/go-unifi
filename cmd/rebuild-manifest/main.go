// Command rebuild-manifest prints the digest of the complete generated output.
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
	flag.Parse()

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		panic(err)
	}
	digest, err := rebuild.OutputDigest(absRoot)
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(os.Stdout, "%s\n", digest)
}
