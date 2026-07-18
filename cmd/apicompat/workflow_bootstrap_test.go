package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestWorkflowBootstrapsOnlyReviewedComparator(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yaml"))
	require.NoError(t, err)
	comparator, err := os.ReadFile(filepath.Join(root, "cmd", "apicompat", "main.go"))
	require.NoError(t, err)

	expectedDigest := fmt.Sprintf("%x", sha256.Sum256(comparator))
	pullRequestStep := between(t, string(workflow), "      - name: Compare pull request API", "      - name: Compare push API")

	assert.Contains(t, pullRequestStep, "if [[ -f cmd/apicompat/main.go ]]; then")
	assert.Contains(t, pullRequestStep, `go build -o "$RUNNER_TEMP/apicompat" ./cmd/apicompat`)
	assert.Contains(t, pullRequestStep, "sha256sum ../candidate/cmd/apicompat/main.go")
	assert.Contains(t, pullRequestStep, "APICOMPAT_BOOTSTRAP_SHA256: "+expectedDigest)
	assert.Contains(t, pullRequestStep, "GOWORK=off GOFLAGS=-mod=readonly")
	assert.Contains(t, pullRequestStep, `go build -o "$RUNNER_TEMP/apicompat" ../candidate/cmd/apicompat/main.go`)
	assert.Contains(t, pullRequestStep, "refusing unreviewed API comparator bootstrap")
	assert.NotContains(t, pullRequestStep, "cd ../candidate")
	assert.NotContains(t, pullRequestStep, "working-directory: candidate")
	assert.Less(t,
		strings.Index(pullRequestStep, `go build -o "$RUNNER_TEMP/apicompat" ./cmd/apicompat`),
		strings.Index(pullRequestStep, `go build -o "$RUNNER_TEMP/apicompat" ../candidate/cmd/apicompat/main.go`),
		"trusted base comparator must remain the normal path",
	)
}

func between(t *testing.T, body, start, end string) string {
	t.Helper()
	startIndex := strings.Index(body, start)
	require.NotEqual(t, -1, startIndex)
	endIndex := strings.Index(body[startIndex+len(start):], end)
	require.NotEqual(t, -1, endIndex)
	return body[startIndex : startIndex+len(start)+endIndex]
}
