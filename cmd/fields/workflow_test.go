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

func TestSchemaUpdateWorkflowValidatesBeforePrivilege(t *testing.T) {
	root := repositoryRoot(t)
	workflow := readWorkflowFile(t, filepath.Join(root, ".github", "workflows", "schema-update.yaml"))

	assert.Contains(t, workflow, "name: UniFi Schema Update")
	assert.Regexp(t, regexp.MustCompile(`cron: ['\"][^'\"]+['\"]`), workflow)
	assert.Contains(t, workflow, "uos_version:")
	assert.Contains(t, workflow, "installer_url:")
	assert.Contains(t, workflow, "permissions:")
	assert.Contains(t, workflow, "  contents: read")
	assert.Contains(t, workflow, "cancel-in-progress: false")
	assert.Contains(t, workflow, "ref: main")
	assert.Contains(t, workflow, "fetch-depth: 0")
	assert.Contains(t, workflow, "persist-credentials: false")
	assert.Contains(t, workflow, "cache: false")
	assert.Contains(t, workflow, "skip-cache: true")
	assert.Contains(t, workflow, "refs/heads/automation/unifi-schema-update")
	assert.Contains(t, workflow, "automation_sha")
	assert.Contains(t, workflow, "go run ./cmd/fields -print-source")
	assert.Contains(t, workflow, ".installer_sha256")
	assert.Contains(t, workflow, "go run ./cmd/fields -verify-committed")
	assert.GreaterOrEqual(t, strings.Count(workflow, "go run ./cmd/fields -verify-regeneration"), 2)
	assert.Contains(t, workflow, "go test ./...")
	assert.Contains(t, workflow, "go vet ./...")
	assert.Contains(t, workflow, "go run ./cmd/apicompat")
	assert.Contains(t, workflow, "api_status=breaking")
	assert.Contains(t, workflow, "exit 3")
	assert.Contains(t, workflow, "cmd/fields/sensitive-policy.json")
	assert.Contains(t, workflow, "unexpected generated path")
	assert.Contains(t, workflow, "automation/unifi-schema-update")
	assert.Contains(t, workflow, "--force-with-lease=refs/heads/automation/unifi-schema-update:")
	assert.Contains(t, workflow, "Schema digest:")
	assert.Contains(t, workflow, "Notice digest:")
	assert.Contains(t, workflow, "api-compatibility:")
	assert.Contains(t, workflow, "ALLOW_AUTOMATED_SCHEMA_RELEASES")
	assert.NotContains(t, workflow, "upload-artifact")
	assert.NotContains(t, workflow, "download-artifact")
	assert.NotContains(t, workflow, "actions/cache")
	assert.NotContains(t, workflow, "vars.SCHEMA_AUTOMATION_APP_ID")
	assert.Contains(t, workflow, "secrets.SCHEMA_AUTOMATION_APP_ID")
	assert.Contains(t, workflow, "secrets.SCHEMA_AUTOMATION_PRIVATE_KEY")

	validated := strings.Index(workflow, "name: Validation complete")
	token := strings.Index(workflow, "name: Mint GitHub App token")
	prBody := workflow[strings.Index(workflow, "name: Prepare reviewed commit and safe PR body"):validated]
	repoCodeAfterToken := strings.Index(workflow[token+1:], "go run ./")
	require.NotEqual(t, -1, validated)
	require.NotEqual(t, -1, token)
	assert.Less(t, validated, token)
	assert.NotContains(t, prBody, "installer_url", "PR body must not copy raw resolver metadata")
	assert.Equal(t, -1, repoCodeAfterToken, "privileged token must not be exposed to repository code")
}

func TestSchemaReleaseWorkflowTagsOnlyTrustedMainSHA(t *testing.T) {
	root := repositoryRoot(t)
	workflow := readWorkflowFile(t, filepath.Join(root, ".github", "workflows", "schema-release.yaml"))

	assert.Contains(t, workflow, "name: UniFi Schema Release")
	assert.Contains(t, workflow, "workflow_run:")
	assert.Contains(t, workflow, "workflows: [CI]")
	assert.Contains(t, workflow, "types: [completed]")
	for _, gate := range []string{
		"github.event.workflow_run.conclusion == 'success'",
		"github.event.workflow_run.event == 'push'",
		"github.event.workflow_run.head_branch == 'main'",
		"vars.ALLOW_AUTOMATED_SCHEMA_RELEASES == 'true'",
	} {
		assert.Contains(t, workflow, gate)
	}
	assert.Contains(t, workflow, "permissions:")
	assert.Contains(t, workflow, "  contents: read")
	assert.Contains(t, workflow, "cancel-in-progress: false")
	assert.Contains(t, workflow, "ref: ${{ github.event.workflow_run.head_sha }}")
	assert.Contains(t, workflow, "persist-credentials: false")
	assert.Contains(t, workflow, "cache: false")
	assert.Contains(t, workflow, "github.event.workflow_run.head_repository.full_name")
	assert.Contains(t, workflow, "merge-base --is-ancestor")
	assert.Contains(t, workflow, "/pulls\"")
	assert.Contains(t, workflow, "automation/unifi-schema-update")
	assert.Contains(t, workflow, "API Compatibility")
	assert.Contains(t, workflow, "gh pr checks")
	assert.Contains(t, workflow, "api-compatibility: compatible")
	assert.Contains(t, workflow, "generated_tree_digest")
	assert.Contains(t, workflow, "expected-generated-tree-digest")
	assert.Contains(t, workflow, "go run ./cmd/fields -verify-committed")
	assert.Contains(t, workflow, `semver_re='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'`)
	assert.Contains(t, workflow, "refs/tags/$TAG")
	assert.NotContains(t, workflow, "pull_request.head.sha")
	assert.NotContains(t, workflow, "upload-artifact")
	assert.NotContains(t, workflow, "download-artifact")
	assert.NotContains(t, workflow, "actions/cache")
	assert.NotContains(t, workflow, "GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}")
	assert.NotContains(t, workflow, "vars.SCHEMA_AUTOMATION_APP_ID")
	assert.Contains(t, workflow, "secrets.SCHEMA_AUTOMATION_APP_ID")
	assert.Contains(t, workflow, "secrets.SCHEMA_AUTOMATION_PRIVATE_KEY")

	validated := strings.Index(workflow, "name: Validation complete")
	token := strings.Index(workflow, "name: Mint GitHub App token")
	require.NotEqual(t, -1, validated)
	require.NotEqual(t, -1, token)
	assert.Less(t, validated, token)
	assert.Equal(t, -1, strings.Index(workflow[token+1:], "go run ./"), "privileged token must not be exposed to repository code")
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
