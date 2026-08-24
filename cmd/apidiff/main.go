// Command apidiff compares the working tree's public Go API against the
// latest release tag and reports the incompatibilities that matter for
// release gating. The baseline is extracted from the local tag via git
// archive rather than the module proxy: the proxy may not yet serve (or may
// have cached a 404 for) a tag that auto-release pushed recently.
//
// cmd/ holds main packages, which are not importable API, so
// incompatibilities there are ignored. UnifiVersion names the controller
// train and changes value on every refresh by design, so a change to its
// value is not drift.
//
// Output: the filtered incompatibilities on stdout, one per line. When
// GITHUB_OUTPUT is set, `base`, `breaking`, and a markdown `summary`
// suitable for a PR body are appended for the calling workflow.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	apidiffModule = "golang.org/x/exp/cmd/apidiff@v0.0.0-20260709172345-9ea1abe57597"
	modulePath    = "github.com/ubiquiti-community/go-unifi"
)

func main() {
	os.Exit(run(os.Stdout, os.Stderr))
}

func run(stdout, stderr io.Writer) int {
	base, err := latestTag()
	if err != nil {
		fmt.Fprintf(stderr, "resolve baseline tag: %v\n", err)
		return 1
	}
	var breaking []string
	if base != "" {
		breaking, err = incompatibilities(base)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	for _, line := range breaking {
		fmt.Fprintln(stdout, line)
	}
	if len(breaking) == 0 {
		fmt.Fprintf(stderr, "no breaking API changes against %s\n", orNone(base))
	}
	if err := writeGitHubOutput(base, breaking); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	// Breaking changes are a reported fact, not a tool failure: both
	// callers proceed (capture still opens the PR, auto-release still
	// summarizes) and branch on the `breaking` output.
	return 0
}

func latestTag() (string, error) {
	out, err := exec.Command("git", "tag", "--list", "v*", "--sort=-v:refname").Output()
	if err != nil {
		return "", err
	}
	tags := strings.Fields(string(out))
	if len(tags) == 0 {
		return "", nil
	}
	return tags[0], nil
}

// incompatibilities extracts the baseline tag into a scratch tree, builds
// its export data, and returns the incompatible changes that gate a
// release.
func incompatibilities(base string) ([]string, error) {
	scratch, err := os.MkdirTemp("", "go-unifi-apidiff")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)

	baseTree := filepath.Join(scratch, "base")
	if err := os.Mkdir(baseTree, 0o755); err != nil {
		return nil, err
	}
	archive := exec.Command("git", "archive", base)
	untar := exec.Command("tar", "-x", "-C", baseTree)
	untar.Stdin, err = archive.StdoutPipe()
	if err != nil {
		return nil, err
	}
	untar.Stderr = os.Stderr
	if err := untar.Start(); err != nil {
		return nil, err
	}
	if err := archive.Run(); err != nil {
		return nil, fmt.Errorf("git archive %s: %w", base, err)
	}
	if err := untar.Wait(); err != nil {
		return nil, fmt.Errorf("extract %s: %w", base, err)
	}

	export := filepath.Join(scratch, "base.export")
	build := exec.Command("go", "run", apidiffModule, "-m", "-w", export, modulePath)
	build.Dir = baseTree
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return nil, fmt.Errorf("build baseline export: %w", err)
	}
	compare := exec.Command("go", "run", apidiffModule, "-m", "-incompatible", export, modulePath)
	compare.Stderr = os.Stderr
	out, err := compare.Output()
	if err != nil {
		return nil, fmt.Errorf("apidiff compare: %w", err)
	}
	return filterIncompatibilities(strings.Split(string(out), "\n")), nil
}

func filterIncompatibilities(lines []string) []string {
	var kept []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" ||
			strings.HasPrefix(line, "- ./cmd/") ||
			strings.Contains(line, "UnifiVersion") {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

func writeGitHubOutput(base string, breaking []string) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return nil
	}
	output, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer output.Close()

	isBreaking := "false"
	if len(breaking) > 0 {
		isBreaking = "true"
	}
	var summary strings.Builder
	if len(breaking) > 0 {
		fmt.Fprintf(&summary, "**Breaking API changes** against `%s` (apidiff):\n\n```\n", base)
		shown := breaking
		if len(shown) > 100 {
			shown = shown[:100]
		}
		summary.WriteString(strings.Join(shown, "\n"))
		if len(breaking) > 100 {
			fmt.Fprintf(&summary, "\n... (%d incompatible changes total)", len(breaking))
		}
		summary.WriteString("\n```\n\nA maintainer must review. After a\nmanual merge the auto-release workflow will refuse to tag,\nso tag by hand (accepted minor, or a /v2 major).")
	} else {
		fmt.Fprintf(&summary, "No breaking API changes against `%s`.\n", orNone(base))
		summary.WriteString("A maintainer must still review the lock and generated diff.")
	}
	_, err = fmt.Fprintf(output, "base=%s\nbreaking=%s\nsummary<<APIDIFF_SUMMARY\n%s\nAPIDIFF_SUMMARY\n", base, isBreaking, summary.String())
	return err
}

func orNone(base string) string {
	if base == "" {
		return "<no release tags>"
	}
	return base
}
