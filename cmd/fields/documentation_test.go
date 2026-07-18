package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemaGenerationDocumentationSafetyBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name     string
		required []string
	}{
		{"../../README.md", []string{"independent and unofficial", "not affiliated", "not redistributed", "still\nexist in state"}},
		{"../../LICENSES/README.md", []string{"not covered by this", "repository's MPL-2.0 license", "only to the extent", "not redistributed"}},
		{"../../docs/schema-generation.md", []string{"8-10 GiB", "-verify-committed", "-verify-regeneration", "does not encrypt, redact, or remove", "not proof\nof a complete license inventory", "remain\ndisabled", "complete new snapshot remains", "replaces the\nprevious snapshot", "138 captured entries", "approved_notice_sha256"}},
	} {
		body, err := os.ReadFile(tc.name)
		require.NoError(t, err)
		for _, text := range tc.required {
			require.Contains(t, string(body), text, "%s must document %q", tc.name, text)
		}
	}
}
