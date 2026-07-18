package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltCLIPropagatesCompatibilityExitCodes(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "apicompat")
	build := exec.Command("go", "build", "-o", binary, ".")
	buildOutput, err := build.CombinedOutput()
	require.NoError(t, err, string(buildOutput))

	base := writeModule(t, baselineAPI)
	tests := map[string]struct {
		candidate string
		want      int
	}{
		"compatible": {candidate: baselineAPI + "\nfunc Added() {}\n", want: 0},
		"breaking":   {candidate: strings.Replace(baselineAPI, "func Parse(string) (*Config, error)", "func Parse([]byte) (*Config, error)", 1), want: 2},
		"ambiguous":  {candidate: "package unifi\nfunc Broken(\n", want: 3},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			command := exec.Command(binary, "-base", base, "-candidate", writeModule(t, test.candidate))
			output, err := command.CombinedOutput()
			assert.Equal(t, test.want, processExitCode(err), string(output))
		})
	}
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return -1
}
