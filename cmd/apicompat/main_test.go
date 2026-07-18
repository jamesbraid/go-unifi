package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const baselineAPI = `package unifi

type Config struct {
	Name string ` + "`json:\"name\"`" + `
}

type Runner interface {
	Run(string) error
}

func Parse(string) (*Config, error) { return nil, nil }

func (Config) Value() string { return "" }
`

func TestClassifyCompatibleAdditions(t *testing.T) {
	base := writeModule(t, baselineAPI)
	candidate := writeModule(t, baselineAPI+`

type Added struct{}

func AddedFunction() {}

func (Config) AddedMethod() {}
`)

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, compatible, report.Status)
	assert.Empty(t, report.Changes)
}

func TestClassifyBreakingChanges(t *testing.T) {
	tests := map[string]string{
		"removed declaration": `package unifi

type Config struct {
	Name string ` + "`json:\"name\"`" + `
}

type Runner interface {
	Run(string) error
}

func (Config) Value() string { return "" }
`,
		"changed function signature": strings.Replace(baselineAPI, "func Parse(string) (*Config, error)", "func Parse([]byte) (*Config, error)", 1),
		"added interface method":     strings.Replace(baselineAPI, "Run(string) error", "Run(string) error\n\tStop()", 1),
		"changed struct field":       strings.Replace(baselineAPI, "Name string", "Name []string", 1),
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			base := writeModule(t, baselineAPI)
			candidate := writeModule(t, source)

			report := classify(context.Background(), base, candidate)

			assert.Equal(t, breaking, report.Status)
			assert.NotEmpty(t, report.Changes)
		})
	}
}

func TestClassifyConcreteMethodAdditionCompatible(t *testing.T) {
	base := writeModule(t, baselineAPI)
	candidate := writeModule(t, baselineAPI+"\nfunc (*Config) PointerMethod(int) error { return nil }\n")

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, compatible, report.Status)
}

func TestClassifyGenericParameterRenameCompatible(t *testing.T) {
	base := writeModule(t, `package unifi
type Box[T ~string] struct { Value T }
func Convert[T ~int](value T) T { return value }
`)
	candidate := writeModule(t, `package unifi
type Box[Value ~string] struct { Value Value }
func Convert[Number ~int](value Number) Number { return value }
`)

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, compatible, report.Status)
}

func TestClassifyGenericConstraintChangeBreaking(t *testing.T) {
	base := writeModule(t, "package unifi\ntype Box[T ~string] struct { Value T }\n")
	candidate := writeModule(t, "package unifi\ntype Box[T ~int] struct { Value T }\n")

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, breaking, report.Status)
}

func TestClassifyParameterNamesIgnoredButVariadicPreserved(t *testing.T) {
	base := writeModule(t, "package unifi\nfunc Join(values ...string) string { return \"\" }\n")
	renamed := writeModule(t, "package unifi\nfunc Join(parts ...string) string { return \"\" }\n")
	nonVariadic := writeModule(t, "package unifi\nfunc Join(parts []string) string { return \"\" }\n")

	assert.Equal(t, compatible, classify(context.Background(), base, renamed).Status)
	assert.Equal(t, breaking, classify(context.Background(), base, nonVariadic).Status)
}

func TestClassifyAliasAndStructTagChangesBreaking(t *testing.T) {
	tests := map[string][2]string{
		"alias target": {
			"package unifi\ntype Identifier = string\n",
			"package unifi\ntype Identifier = int\n",
		},
		"struct tag": {
			"package unifi\ntype Config struct { Name string `json:\"name\"` }\n",
			"package unifi\ntype Config struct { Name string `json:\"display_name\"` }\n",
		},
	}

	for name, sources := range tests {
		t.Run(name, func(t *testing.T) {
			report := classify(context.Background(), writeModule(t, sources[0]), writeModule(t, sources[1]))
			assert.Equal(t, breaking, report.Status)
		})
	}
}

func TestClassifyUnexportedEmbeddingRemovalBreaking(t *testing.T) {
	base := writeModule(t, "package unifi\ntype hidden struct { Visible string }\ntype Config struct { hidden }\n")
	candidate := writeModule(t, "package unifi\ntype hidden struct { Visible string }\ntype Config struct{}\n")

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, breaking, report.Status)
}

func TestClassifyReachableUnexportedMethodRemovalBreaking(t *testing.T) {
	base := writeModule(t, "package unifi\ntype hidden struct{}\nfunc New() hidden { return hidden{} }\nfunc (hidden) Do() {}\n")
	candidate := writeModule(t, "package unifi\ntype hidden struct{}\nfunc New() hidden { return hidden{} }\n")

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, breaking, report.Status)
}

func TestClassifyPrivateFieldComparabilityChangeBreaking(t *testing.T) {
	base := writeModule(t, "package unifi\ntype Config struct { state int }\n")
	candidate := writeModule(t, "package unifi\ntype Config struct { state []int }\n")

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, breaking, report.Status)
}

func TestClassifyEquivalentInterfaceEmbeddingCompatible(t *testing.T) {
	base := writeModule(t, "package unifi\ntype Base interface { Run() }\ntype Runner interface { Base }\n")
	candidate := writeModule(t, "package unifi\ntype Base interface { Run() }\ntype Runner interface { Run() }\n")

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, compatible, report.Status)
}

func TestClassifyAllPackagesRemovedBreaking(t *testing.T) {
	base := writeModule(t, baselineAPI)
	candidate := writeEmptyModule(t)

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, breaking, report.Status)
	assert.NotEmpty(t, report.Changes)
}

func TestClassifyPackageClauseRenameBreaking(t *testing.T) {
	base := writeModule(t, "package unifi\n")
	candidate := writeModule(t, "package renamed\n")

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, breaking, report.Status)
	assert.NotEmpty(t, report.Changes)
}

func TestClassifyPackageWithoutExportsRemovalBreaking(t *testing.T) {
	base := writeModule(t, "package unifi\nvar private int\n")
	candidate := writeEmptyModule(t)

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, breaking, report.Status)
	assert.NotEmpty(t, report.Changes)
}

func TestFixedGoEnvClearsBuildFlags(t *testing.T) {
	t.Setenv("GOFLAGS", "-tags=unexpected")

	env := fixedGoEnv()

	assert.Contains(t, env, "GOFLAGS=")
	assert.NotContains(t, env, "GOFLAGS=-tags=unexpected")
}

func TestClassifyValueReceiverChangedToPointerBreaking(t *testing.T) {
	base := writeModule(t, "package unifi\ntype Config struct{}\nfunc (Config) Apply() {}\n")
	candidate := writeModule(t, "package unifi\ntype Config struct{}\nfunc (*Config) Apply() {}\n")

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, breaking, report.Status)
}

func TestClassifyLoadErrorAmbiguous(t *testing.T) {
	base := writeModule(t, baselineAPI)
	candidate := writeModule(t, "package unifi\nfunc Broken(\n")

	report := classify(context.Background(), base, candidate)

	assert.Equal(t, ambiguous, report.Status)
	require.NotEmpty(t, report.Errors)
	assert.Contains(t, report.Errors[0], "candidate:")
}

func TestMarkdownIsDeterministic(t *testing.T) {
	base := writeModule(t, baselineAPI)
	candidate := writeModule(t, strings.Replace(baselineAPI, "Name string", "Name []string", 1))

	first := classify(context.Background(), base, candidate).Markdown()
	second := classify(context.Background(), base, candidate).Markdown()

	assert.Equal(t, first, second)
	assert.Equal(t, `# Go API compatibility

Status: **breaking**

## Breaking changes

- Changed `+"`example.com/api/unifi.Config`"+`
`, first)
}

func TestRunExitCodesAndMarkdownFile(t *testing.T) {
	base := writeModule(t, baselineAPI)
	candidate := writeModule(t, strings.Replace(baselineAPI, "Name string", "Name []string", 1))
	markdown := filepath.Join(t.TempDir(), "api.md")
	var stdout, stderr bytes.Buffer

	code := run([]string{"-base", base, "-candidate", candidate, "-markdown", markdown}, &stdout, &stderr)

	assert.Equal(t, 2, code)
	assert.Empty(t, stderr.String())
	written, err := os.ReadFile(markdown)
	require.NoError(t, err)
	assert.Equal(t, stdout.String(), string(written))
}

func TestRunCompatibleAndAmbiguousExitCodes(t *testing.T) {
	base := writeModule(t, baselineAPI)
	tests := map[string]struct {
		candidate string
		want      int
	}{
		"compatible": {candidate: baselineAPI + "\nfunc Added() {}\n", want: 0},
		"ambiguous":  {candidate: "package unifi\nfunc Broken(\n", want: 3},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"-base", base, "-candidate", writeModule(t, test.candidate)}, &stdout, &stderr)
			assert.Equal(t, test.want, code)
			assert.Contains(t, stdout.String(), "# Go API compatibility")
		})
	}
}

func writeModule(t *testing.T, source string) string {
	t.Helper()
	root := writeEmptyModule(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "unifi"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unifi", "api.go"), []byte(source), 0o600))
	return root
}

func writeEmptyModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/api\n\ngo 1.25.0\n"), 0o600))
	return root
}
