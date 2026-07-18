package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCIAllowsOnlyExplicitlyReviewedBreakingAPIChanges(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	body, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yaml"))
	require.NoError(t, err)
	workflow := string(body)

	assert.Contains(t, workflow, "types: [opened, synchronize, reopened, labeled, unlabeled]")
	assert.Contains(t, workflow, "permissions:\n  contents: read\n  pull-requests: read")
	assert.NotContains(t, workflow, "go run ./cmd/apicompat")

	pullRequest := between(t, workflow, "      - name: Compare pull request API", "      - name: Compare push API")
	assert.Contains(t, pullRequest, "breaking-api-approved")
	assert.Contains(t, pullRequest, "contains(github.event.pull_request.labels.*.name, 'breaking-api-approved')")
	assert.Contains(t, pullRequest, "0) exit 0")
	assert.Contains(t, pullRequest, "2)")
	assert.Contains(t, pullRequest, `if [[ "$BREAKING_API_APPROVED" == "true" ]]`)
	assert.Contains(t, pullRequest, "Breaking API change approved by maintainer label")
	assert.Contains(t, pullRequest, "3) exit 3")
	assert.NotContains(t, pullRequest, "github.token")
	assert.NotContains(t, pullRequest, "secrets.")

	pushComparison := between(t, workflow, "      - name: Compare push API", "      - name: Verify approved breaking push")
	assert.Contains(t, pushComparison, `echo "status=breaking" >> "$GITHUB_OUTPUT"`)
	assert.Contains(t, pushComparison, "3) exit 3")
	assert.NotContains(t, pushComparison, "github.token")
	assert.NotContains(t, pushComparison, "secrets.")

	pushApproval := between(t, workflow, "      - name: Verify approved breaking push", "      - name: Require clean API comparison trees")
	assert.Contains(t, pushApproval, "steps.push-api.outputs.status == 'breaking'")
	assert.Contains(t, pushApproval, "GH_TOKEN: ${{ github.token }}")
	assert.Contains(t, pushApproval, `"/repos/$REPOSITORY/commits/$CANDIDATE_SHA/pulls"`)
	assert.Contains(t, pushApproval, `.base.ref == "main"`)
	assert.Contains(t, pushApproval, `.merged_at != null`)
	assert.Contains(t, pushApproval, `index("breaking-api-approved")`)
	assert.Contains(t, pushApproval, "length == 1")
}

func TestWorkflowsInvokeBuiltComparatorAndReleaseStaysCompatibleOnly(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	for _, name := range []string{"ci.yaml", "schema-update.yaml"} {
		body, err := os.ReadFile(filepath.Join(root, ".github", "workflows", name))
		require.NoError(t, err)
		workflow := string(body)
		assert.NotContains(t, workflow, "go run ./cmd/apicompat", name)
		assert.Contains(t, workflow, `go build -o "$RUNNER_TEMP/apicompat"`, name)
		assert.Contains(t, workflow, `"$RUNNER_TEMP/apicompat"`, name)
	}

	release, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "schema-release.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(release), "api-compatibility: compatible")
	assert.NotContains(t, string(release), "breaking-api-approved")

	docs, err := os.ReadFile(filepath.Join(root, "docs", "schema-generation.md"))
	require.NoError(t, err)
	assert.Contains(t, string(docs), "breaking-api-approved")
	assert.Contains(t, strings.ToLower(string(docs)), "major release")
	assert.Contains(t, string(docs), "Task 8")
}
