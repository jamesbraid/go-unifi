package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowActionsAreImmutableAndBlocking(t *testing.T) {
	root := repositoryRoot(t)
	workflows := workflowFiles(t, root)
	require.NotEmpty(t, workflows)
	immutableUse := regexp.MustCompile(`^[^@[:space:]]+@[0-9a-f]{40}$`)
	versionComment := regexp.MustCompile(`# v[0-9]+(?:\.[0-9]+){1,2}(?:[[:space:]]|$)`)

	for _, name := range workflows {
		body := readWorkflowFile(t, name)
		lower := strings.ToLower(body)
		assert.NotContains(t, lower, "continue-on-error", name)
		assert.NotContains(t, lower, "upload-artifact", name)
		assert.NotContains(t, lower, "go generate", name)
		assert.NotContains(t, lower, "go mod tidy", name)
		assert.NotRegexp(t, regexp.MustCompile(`(?m)^\s*version:\s*['\"]?latest`), body, name)

		for _, line := range strings.Split(body, "\n") {
			trimmed := strings.TrimSpace(line)
			trimmed = strings.TrimPrefix(trimmed, "- ")
			if !strings.HasPrefix(trimmed, "uses:") {
				continue
			}
			spec := strings.Fields(strings.TrimSpace(strings.TrimPrefix(trimmed, "uses:")))
			require.NotEmpty(t, spec, "%s: empty uses line %q", name, line)
			assert.Regexp(t, immutableUse, spec[0], "%s: uses must be pinned to a full commit SHA: %q", name, line)
			assert.Regexp(t, versionComment, line, "%s: pinned action needs a version comment: %q", name, line)
		}
	}
}

func TestOrdinaryCIIsOfflineAndBlocking(t *testing.T) {
	root := repositoryRoot(t)
	ci := readWorkflowFile(t, filepath.Join(root, ".github", "workflows", "ci.yaml"))

	assert.Contains(t, ci, "permissions:\n  contents: read")
	for _, jobName := range []string{
		"Verify Generated Tree",
		"Go Test",
		"Go Vet",
		"Go Lint",
		"YAML Lint",
		"API Compatibility",
		"Tracked Diff",
	} {
		assert.Contains(t, ci, "name: "+jobName, "missing stable required check name")
	}
	for _, command := range []string{
		"go run ./cmd/fields -verify-committed",
		"go test ./...",
		"go vet ./...",
		"git diff --exit-code",
		"version: v2.11.4",
	} {
		assert.Contains(t, ci, command)
	}
	for _, selector := range []string{"-uos-release", "-installer-url", "fw-download.ubnt.com", "go generate"} {
		assert.NotContains(t, ci, selector)
	}

	assert.Contains(t, ci, "github.event.pull_request.base.sha")
	assert.Contains(t, ci, "github.event.pull_request.head.sha")
	assert.Contains(t, ci, "github.event.before")
	assert.Contains(t, ci, "github.sha")
	assert.Contains(t, ci, "path: base")
	assert.Contains(t, ci, "path: candidate")
	assert.Contains(t, ci, "persist-credentials: false")
	assert.Contains(t, ci, "go run ./cmd/apicompat")
	assert.Contains(t, ci, `-markdown "$RUNNER_TEMP/api-compat.md"`)
	assert.Contains(t, ci, "set +e")
	assert.GreaterOrEqual(t, strings.Count(ci, "git diff --exit-code"), 5, "mutation-sensitive jobs need their own final diff check")

	aggregate := ci[strings.Index(ci, "  tracked-diff:"):]
	assert.NotContains(t, aggregate, "uses: actions/checkout", "aggregate gate must not pretend a fresh checkout detects earlier job mutations")
	assert.Contains(t, aggregate, "if: always()")
	assert.Contains(t, aggregate, "needs.verify-generated-tree.result")
}

func TestObsoleteGeneratorWorkflowIsRemoved(t *testing.T) {
	root := repositoryRoot(t)
	_, err := os.Stat(filepath.Join(root, ".github", "workflows", "generate.yaml"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestReleaseVerifiesWithoutMutatingGeneratedFiles(t *testing.T) {
	root := repositoryRoot(t)
	release := readWorkflowFile(t, filepath.Join(root, ".github", "workflows", "release.yaml"))
	verification := strings.Index(release, "go run ./cmd/fields -verify-committed")
	goreleaser := strings.Index(release, "goreleaser/goreleaser-action@")
	require.NotEqual(t, -1, verification)
	require.NotEqual(t, -1, goreleaser)
	assert.Less(t, verification, goreleaser)
	assert.Contains(t, release, "version: v2.15.4")

	config := readWorkflowFile(t, filepath.Join(root, ".goreleaser.yaml"))
	assert.Contains(t, config, "go mod verify")
	assert.Contains(t, config, "go run ./cmd/fields -verify-committed")
	assert.NotContains(t, config, "go generate")
	assert.NotContains(t, config, "go mod tidy")
}

func TestDependabotDoesNotSuppressAutoMergeFailureOrCheckoutPRCode(t *testing.T) {
	root := repositoryRoot(t)
	workflow := readWorkflowFile(t, filepath.Join(root, ".github", "workflows", "dependabot.yml"))
	assert.NotContains(t, workflow, "||")
	assert.NotContains(t, workflow, "actions/checkout")
	assert.Contains(t, workflow, "github.actor == 'dependabot[bot]'")
	assert.Contains(t, workflow, "gh pr merge --auto --rebase")
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}

func workflowFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	for _, pattern := range []string{"*.yaml", "*.yml"} {
		matches, err := filepath.Glob(filepath.Join(root, ".github", "workflows", pattern))
		require.NoError(t, err)
		files = append(files, matches...)
	}
	return files
}

func readWorkflowFile(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(name)
	require.NoError(t, err)
	return string(body)
}
