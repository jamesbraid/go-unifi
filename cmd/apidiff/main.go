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
	"flag"
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
	flags := flag.NewFlagSet("go-unifi apidiff", flag.ContinueOnError)
	flags.SetOutput(stderr)
	baseFlag := flags.String("base", "", "baseline tag (default: latest v* tag)")
	markdown := flags.Bool("markdown", false, "print the markdown summary to stdout")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return 2
	}
	base := *baseFlag
	if base == "" {
		var err error
		base, err = latestTag()
		if err != nil {
			fmt.Fprintf(stderr, "resolve baseline tag: %v\n", err)
			return 1
		}
	}
	var breaking, wireAdded, wireRemoved []string
	if base != "" {
		baseTree, cleanup, err := extractBase(base)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer cleanup()
		breaking, err = incompatibilities(baseTree)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		wireAdded, wireRemoved, err = wireSurfaceDelta(baseTree)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if *markdown {
		fmt.Fprintln(stdout, summaryMarkdown(base, breaking, wireAdded, wireRemoved))
	} else {
		for _, line := range breaking {
			fmt.Fprintln(stdout, line)
		}
	}
	if len(breaking) == 0 {
		fmt.Fprintf(stderr, "no breaking API changes against %s\n", orNone(base))
	}
	if err := writeGitHubOutput(base, breaking, wireAdded, wireRemoved); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	// Breaking changes are a reported fact, not a tool failure: both
	// callers proceed (capture still opens the PR, auto-release still
	// summarizes) and branch on the `breaking` output.
	return 0
}

// extractBase materializes the baseline tag into a scratch tree via git
// archive, so both the API and wire-surface comparisons read the same
// bytes.
func extractBase(base string) (string, func(), error) {
	scratch, err := os.MkdirTemp("", "go-unifi-apidiff")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(scratch) }
	baseTree := filepath.Join(scratch, "base")
	if err := os.Mkdir(baseTree, 0o755); err != nil {
		cleanup()
		return "", nil, err
	}
	archive := exec.Command("git", "archive", base)
	untar := exec.Command("tar", "-x", "-C", baseTree)
	untar.Stdin, err = archive.StdoutPipe()
	if err != nil {
		cleanup()
		return "", nil, err
	}
	untar.Stderr = os.Stderr
	if err := untar.Start(); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := archive.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("git archive %s: %w", base, err)
	}
	if err := untar.Wait(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("extract %s: %w", base, err)
	}
	return baseTree, cleanup, nil
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

// incompatibilities builds the baseline tree's export data and returns the
// incompatible changes that gate a release.
func incompatibilities(baseTree string) ([]string, error) {
	export := filepath.Join(filepath.Dir(baseTree), "base.export")
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

// wireBaselinePath records every field the generated marshalers send
// unconditionally; a diff of it between releases is the wire-surface
// change apidiff's compile-time view cannot see (struct tags are not API
// to the type checker, but omitempty is a wire contract).
const wireBaselinePath = "unifi/testdata/always_serialized_fields.txt"

// purposeBaselinePath records what each Network purpose encoder sends for an
// object the caller left alone.
//
// The generated marshalers are covered by wireBaselinePath, which is read
// from struct tags. Network's encoders are hand-written and re-declare their
// own tags, so a change there is invisible to that baseline: dropping
// omitempty from remote_vpn_subnets put the key on every site-to-site write
// and this tool reported no wire-surface change at all. Both files are
// compared so that neither half of the encoder can move quietly.
const purposeBaselinePath = "unifi/testdata/purpose_wire_shape.txt"

func wireSurfaceDelta(baseTree string) (added, removed []string, err error) {
	for _, path := range []string{wireBaselinePath, purposeBaselinePath} {
		baseLines, recorded, err := baselineLines(filepath.Join(baseTree, path))
		if err != nil {
			return nil, nil, err
		}
		// A baseline the release being compared against never had says
		// nothing about what moved since. Reporting its whole contents as
		// additions would bury the real ones the first time a baseline is
		// introduced.
		if !recorded {
			continue
		}
		currentLines, _, err := baselineLines(path)
		if err != nil {
			return nil, nil, err
		}
		fileAdded, fileRemoved := wireDelta(baseLines, currentLines)
		added = append(added, fileAdded...)
		removed = append(removed, fileRemoved...)
	}
	return added, removed, nil
}

// baselineLines reads a wire baseline, reporting whether the file was there
// at all: a release that predates a baseline has no record to compare
// against, which is different from a record that is empty.
func baselineLines(path string) ([]string, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines, true, nil
}

func wireDelta(base, current []string) (added, removed []string) {
	inBase := make(map[string]bool, len(base))
	for _, line := range base {
		inBase[line] = true
	}
	inCurrent := make(map[string]bool, len(current))
	for _, line := range current {
		inCurrent[line] = true
		if !inBase[line] {
			added = append(added, line)
		}
	}
	for _, line := range base {
		if !inCurrent[line] {
			removed = append(removed, line)
		}
	}
	return added, removed
}

func wireSection(base string, added, removed []string) string {
	if len(added) == 0 && len(removed) == 0 {
		return fmt.Sprintf("No wire-surface changes against `%s`: the always-serialized field set is unchanged.", orNone(base))
	}
	var section strings.Builder
	fmt.Fprintf(&section, "**Wire surface** vs `%s` (fields the marshalers always send; invisible to apidiff):\n\n```\n", base)
	for _, line := range added {
		fmt.Fprintf(&section, "+ %s\n", line)
	}
	for _, line := range removed {
		fmt.Fprintf(&section, "- %s\n", line)
	}
	section.WriteString("```\n")
	if len(added) > 0 {
		section.WriteString("+ now always sent (a zero value reaches the controller and is stored)\n")
	}
	if len(removed) > 0 {
		section.WriteString("- no longer always sent (an unset value stays off the wire)\n")
	}
	return strings.TrimRight(section.String(), "\n")
}

func summaryMarkdown(base string, breaking, wireAdded, wireRemoved []string) string {
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
	summary.WriteString("\n\n")
	summary.WriteString(wireSection(base, wireAdded, wireRemoved))
	return summary.String()
}

func writeGitHubOutput(base string, breaking, wireAdded, wireRemoved []string) error {
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
	wireChanged := "false"
	if len(wireAdded)+len(wireRemoved) > 0 {
		wireChanged = "true"
	}
	_, err = fmt.Fprintf(output, "base=%s\nbreaking=%s\nwire_changed=%s\nsummary<<APIDIFF_SUMMARY\n%s\nAPIDIFF_SUMMARY\n",
		base, isBreaking, wireChanged, summaryMarkdown(base, breaking, wireAdded, wireRemoved))
	return err
}

func orNone(base string) string {
	if base == "" {
		return "<no release tags>"
	}
	return base
}
