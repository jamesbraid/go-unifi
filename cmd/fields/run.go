package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/go-version"
)

type sourceValues struct {
	latest, uosLatest                   bool
	uosVersion, installer, installerURL string
}

type runDeps struct {
	client           *http.Client
	firmwareEndpoint string
	tempRoot         string
	schemaSourcePath string
	moduleRoot       func() (string, error)
	resolve          func(context.Context, *http.Client, string, SourceSelector) (InstallerSource, error)
	materialize      func(context.Context, *http.Client, InstallerSource, string) (*MaterializedInstaller, error)
	extractUOS       func(context.Context, string, string, ArchiveLimits) (*ExtractedDefinitions, error)
	buildSnapshot    func(context.Context, SnapshotOptions) (*LocalManifest, error)
	scan             func(string) error
	render           func(string, string, *version.Version, func([]*ResourceInfo) error) error
	publish          func(string, string, string, string, string) error
}

func defaultRunDeps() runDeps {
	return runDeps{client: http.DefaultClient, firmwareEndpoint: firmwareUpdateApi, schemaSourcePath: filepath.Join("cmd", "fields", "schema-source.json"),
		moduleRoot: currentModuleRoot, resolve: ResolveInstaller, materialize: MaterializeInstaller,
		extractUOS: ExtractUOSInstaller, buildSnapshot: BuildSnapshot, scan: ScanExtractedInputs,
		render: renderGeneratedStage, publish: publishGeneratedTree}
}

func currentModuleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root := findModuleRoot(wd)
	if root == "" {
		return "", errors.New("Go module root not found")
	}
	return root, nil
}

func selectorFromValues(v sourceValues, positional []string) (SourceSelector, error) {
	selectors := make([]SourceSelector, 0, 2)
	if v.latest {
		selectors = append(selectors, SourceSelector{Kind: SourceLegacyLatest})
	}
	if v.uosLatest {
		selectors = append(selectors, SourceSelector{Kind: SourceUOSLatest})
	}
	if v.uosVersion != "" {
		selectors = append(selectors, SourceSelector{Kind: SourceUOSVersion, Value: v.uosVersion})
	}
	if v.installer != "" {
		selectors = append(selectors, SourceSelector{Kind: SourceInstallerFile, Value: v.installer})
	}
	if v.installerURL != "" {
		selectors = append(selectors, SourceSelector{Kind: SourceInstallerURL, Value: v.installerURL})
	}
	if len(positional) > 1 {
		return SourceSelector{}, errors.New("source selector accepts at most one positional version")
	}
	if len(positional) == 1 {
		selectors = append(selectors, SourceSelector{Kind: SourceLegacyVersion, Value: positional[0]})
	}
	if len(selectors) != 1 {
		return SourceSelector{}, errors.New("exactly one source selector is required")
	}
	return selectors[0], nil
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runWithDeps(ctx, args, stdout, stderr, defaultRunDeps())
}

func runWithDeps(ctx context.Context, args []string, stdout, stderr io.Writer, deps runDeps) error {
	fs := flag.NewFlagSet("fields", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var values sourceValues
	fs.BoolVar(&values.latest, "latest", false, "use latest legacy Network release")
	fs.BoolVar(&values.uosLatest, "uos-latest", false, "use latest UniFi OS Server release")
	fs.StringVar(&values.uosVersion, "uos-version", "", "use exact UniFi OS Server version")
	fs.StringVar(&values.installer, "installer", "", "use local UniFi OS Server installer")
	fs.StringVar(&values.installerURL, "installer-url", "", "use official UniFi OS Server installer URL")
	outputDir := fs.String("output-dir", "unifi", "generated Go output directory")
	downloadOnly := fs.Bool("download-only", false, "extract inputs without generating")
	generateSpec := fs.Bool("generate-spec", false, "generate Terraform provider specification")
	specOutput := fs.String("spec-output", "specification.json", "Terraform specification output")
	verifyCommitted := fs.Bool("verify-committed", false, "verify committed generated files offline")
	verifyRegeneration := fs.Bool("verify-regeneration", false, "verify regeneration from local snapshot")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse command: %w", err)
	}
	terminal := *verifyCommitted || *verifyRegeneration
	if *verifyCommitted && *verifyRegeneration {
		return errors.New("-verify-committed and -verify-regeneration are mutually exclusive")
	}
	if terminal {
		if values != (sourceValues{}) || len(fs.Args()) != 0 || *downloadOnly || *generateSpec || *outputDir != "unifi" || *specOutput != "specification.json" {
			return errors.New("verification modes are selector-free and mutually exclusive with generation options")
		}
		root, err := deps.moduleRoot()
		if err != nil {
			return fmt.Errorf("locate module root: %w", err)
		}
		if *verifyCommitted {
			return verifyCommittedTree(root, deps.schemaSourcePath)
		}
		return verifyRegeneratedTree(ctx, root, deps, stdout)
	}
	selector, err := selectorFromValues(values, fs.Args())
	if err != nil {
		return fmt.Errorf("select source: %w", err)
	}
	uos := selector.Kind == SourceUOSLatest || selector.Kind == SourceUOSVersion || selector.Kind == SourceInstallerFile || selector.Kind == SourceInstallerURL
	if uos && !*downloadOnly && !*generateSpec {
		return errors.New("UniFi OS generation requires -generate-spec")
	}
	if uos && (*outputDir != "unifi" || *specOutput != "specification.json") {
		return errors.New("UniFi OS provenance publication requires canonical unifi and specification.json outputs")
	}
	root, err := deps.moduleRoot()
	if err != nil {
		return fmt.Errorf("locate module root: %w", err)
	}
	source, err := deps.resolve(ctx, deps.client, deps.firmwareEndpoint, selector)
	if err != nil {
		return fmt.Errorf("resolve source: %w", err)
	}
	installer, err := deps.materialize(ctx, deps.client, source, deps.tempRoot)
	if err != nil {
		return fmt.Errorf("materialize source: %w", err)
	}
	defer installer.Close()
	if uos {
		return runUOS(ctx, root, source, installer, *downloadOnly, deps, stdout)
	}
	return runLegacy(ctx, root, source, installer, *downloadOnly, *generateSpec, *outputDir, *specOutput, stdout)
}

func runUOS(ctx context.Context, root string, source InstallerSource, installer *MaterializedInstaller, downloadOnly bool, deps runDeps, stdout io.Writer) error {
	definitions, err := deps.extractUOS(ctx, installer.Path, deps.tempRoot, DefaultArchiveLimits())
	if err != nil {
		return fmt.Errorf("extract UniFi OS installer: %w", err)
	}
	fieldsRoot := filepath.Join(root, "cmd", "fields")
	policyPath := filepath.Join(fieldsRoot, "sensitive-policy.json")
	policyVersion := bestEffortPolicyVersion(policyPath)
	manifest, err := deps.buildSnapshot(ctx, SnapshotOptions{Root: fieldsRoot, CustomDir: filepath.Join(fieldsRoot, "custom"), Source: source, Installer: installer, Definitions: definitions, PolicyVersion: policyVersion})
	if err != nil {
		return fmt.Errorf("publish local snapshot: %w", err)
	}
	snapshot := filepath.Join(fieldsRoot, "v"+manifest.NetworkVersion)
	if err := deps.scan(snapshot); err != nil {
		return fmt.Errorf("scan extracted inputs: %w", err)
	}
	if downloadOnly {
		fmt.Fprintf(stdout, "Fields JSON ready: %s\n", snapshot)
		return nil
	}
	policy, err := LoadSensitivityPolicy(policyPath)
	if err != nil {
		return fmt.Errorf("load sensitivity policy after snapshot publication: %w", err)
	}
	return regenerateAndPublish(ctx, root, snapshot, source, *manifest, policy, stdout, deps)
}

func bestEffortPolicyVersion(name string) string {
	body, err := os.ReadFile(name)
	if err != nil {
		return ""
	}
	var value struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(body, &value) != nil {
		return ""
	}
	return value.Version
}

func runLegacy(ctx context.Context, root string, source InstallerSource, installer *MaterializedInstaller, downloadOnly, generateSpec bool, outputDir, specOutput string, stdout io.Writer) error {
	v, err := version.NewVersion(source.OSVersion)
	if err != nil {
		return fmt.Errorf("parse legacy Network version: %w", err)
	}
	fieldsRoot := filepath.Join(root, "cmd", "fields")
	final := filepath.Join(fieldsRoot, "v"+v.String())
	if _, err := os.Stat(final); errors.Is(err, os.ErrNotExist) {
		stage, err := os.MkdirTemp(fieldsRoot, ".legacy-staging-*")
		if err != nil {
			return fmt.Errorf("stage legacy extraction: %w", err)
		}
		defer os.RemoveAll(stage)
		jar, err := extractLegacyInstaller(installer.Path, stage)
		if err != nil {
			return fmt.Errorf("extract verified legacy package: %w", err)
		}
		if err := extractJSON(jar, stage); err != nil {
			return fmt.Errorf("extract legacy fields: %w", err)
		}
		if err := copyCustomTo(filepath.Join(fieldsRoot, "custom"), stage); err != nil {
			return fmt.Errorf("overlay legacy custom fields: %w", err)
		}
		if err := os.Rename(stage, final); err != nil {
			return fmt.Errorf("publish legacy fields: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("stat legacy fields: %w", err)
	}
	if downloadOnly {
		fmt.Fprintf(stdout, "Fields JSON ready: %s\n", final)
		return nil
	}
	out := outputDir
	if !filepath.IsAbs(out) {
		out = filepath.Join(root, out)
	}
	spec := specOutput
	if !filepath.IsAbs(spec) {
		spec = filepath.Join(root, spec)
	}
	return generateFromFields(final, out, v, generateSpec, spec, stdout, nil)
}

func copyCustomTo(source, target string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, entry.Name()), body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func regenerateAndPublish(ctx context.Context, root, snapshot string, source InstallerSource, manifest LocalManifest, policy SensitivityPolicy, stdout io.Writer, deps runDeps) error {
	stage, err := os.MkdirTemp(filepath.Dir(root), ".go-unifi-generate-*")
	if err != nil {
		return fmt.Errorf("create generation staging tree: %w", err)
	}
	defer os.RemoveAll(stage)
	raw, metadata, err := loadSensitivityInputs(snapshot)
	if err != nil {
		return err
	}
	prepare := func(resources []*ResourceInfo) error {
		_, err := ApplySensitivity(resources, raw, metadata, policy)
		return err
	}
	v, err := version.NewVersion(manifest.NetworkVersion)
	if err != nil {
		return fmt.Errorf("parse Network version: %w", err)
	}
	if err := deps.render(snapshot, stage, v, prepare); err != nil {
		return fmt.Errorf("render generated outputs: %w", err)
	}
	files, digest, err := HashGeneratedFiles(stage, "unifi", "specification.json")
	if err != nil {
		return fmt.Errorf("validate generated outputs: %w", err)
	}
	schemaSource := SchemaSource{OSVersion: source.OSVersion, NetworkVersion: manifest.NetworkVersion, FirmwareID: source.FirmwareID, InstallerSHA256: installerSHA(source, manifest), SchemaDigest: manifest.SchemaDigest, SensitivityDigest: manifest.SensitivityDigest, NoticeDigest: manifest.NoticeDigest, GeneratedTreeDigest: digest, GeneratedFiles: files, PolicyVersion: policy.Version, InstallerSize: manifest.InstallerSize, Created: source.Created, Updated: source.Updated}
	if source.URL != nil {
		schemaSource.InstallerURL = source.URL.String()
	}
	schemaRel := filepath.Join("cmd", "fields", "schema-source.json")
	if err := WriteSchemaSource(filepath.Join(stage, schemaRel), schemaSource); err != nil {
		return fmt.Errorf("stage schema provenance: %w", err)
	}
	if err := deps.publish(root, stage, "unifi", "specification.json", schemaRel); err != nil {
		return fmt.Errorf("publish generated outputs: %w", err)
	}
	fmt.Fprintf(stdout, "%s\n", filepath.Join(root, "unifi"))
	return nil
}

func renderGeneratedStage(snapshot, stage string, v *version.Version, prepare func([]*ResourceInfo) error) error {
	return generateFromFields(snapshot, filepath.Join(stage, "unifi"), v, true, filepath.Join(stage, "specification.json"), io.Discard, prepare)
}

func installerSHA(source InstallerSource, manifest LocalManifest) string {
	if source.ExpectedSHA256 != "" {
		return source.ExpectedSHA256
	}
	return manifest.InstallerSHA256
}

func loadSensitivityInputs(snapshot string) (RawSchemaIndex, []byte, error) {
	raw := RawSchemaIndex{}
	rawRoot := filepath.Join(snapshot, "metadata", "raw-fields")
	err := filepath.WalkDir(rawRoot, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		raw[base] = body
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("load raw schemas: %w", err)
	}
	metadata, err := os.ReadFile(filepath.Join(snapshot, "metadata", "sensitive_metadata.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("load sensitivity metadata: %w", err)
	}
	return raw, metadata, nil
}

func HashGeneratedFiles(root, outputDir, specOutput string) (map[string]string, string, error) {
	files := map[string][]byte{}
	goRoot := filepath.Join(root, outputDir)
	err := filepath.WalkDir(goRoot, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".generated.go") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("generated output is not regular: %s", name)
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = body
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	body, err := os.ReadFile(filepath.Join(root, specOutput))
	if err != nil {
		return nil, "", err
	}
	files[filepath.ToSlash(specOutput)] = body
	digest, err := CanonicalTreeDigest(files)
	if err != nil {
		return nil, "", err
	}
	hashes := make(map[string]string, len(files))
	for name, body := range files {
		sum := sha256.Sum256(body)
		hashes[name] = hex.EncodeToString(sum[:])
	}
	return hashes, digest, nil
}

func verifyCommittedTree(root, sourceRel string) error {
	body, err := os.ReadFile(filepath.Join(root, sourceRel))
	if err != nil {
		return fmt.Errorf("read schema source: %w", err)
	}
	var source SchemaSource
	if err := json.Unmarshal(body, &source); err != nil {
		return fmt.Errorf("parse schema source: %w", err)
	}
	files, digest, err := HashGeneratedFiles(root, "unifi", "specification.json")
	if err != nil {
		return fmt.Errorf("hash committed outputs: %w", err)
	}
	if different := firstHashDifference(source.GeneratedFiles, files); different != "" {
		return fmt.Errorf("committed generated file differs: %s", different)
	}
	if digest != source.GeneratedTreeDigest {
		return fmt.Errorf("generated tree digest differs: expected %s, got %s", source.GeneratedTreeDigest, digest)
	}
	return nil
}

func firstHashDifference(want, got map[string]string) string {
	keys := make([]string, 0, len(want)+len(got))
	seen := map[string]bool{}
	for key := range want {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range got {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if want[key] != got[key] {
			return key
		}
	}
	return ""
}

func verifyRegeneratedTree(ctx context.Context, root string, deps runDeps, stdout io.Writer) error {
	body, err := os.ReadFile(filepath.Join(root, deps.schemaSourcePath))
	if err != nil {
		return fmt.Errorf("read schema source: %w", err)
	}
	var source SchemaSource
	if err := json.Unmarshal(body, &source); err != nil {
		return err
	}
	snapshot := filepath.Join(root, "cmd", "fields", "v"+source.NetworkVersion)
	if err := deps.scan(snapshot); err != nil {
		return fmt.Errorf("scan local snapshot: %w", err)
	}
	policy, err := LoadSensitivityPolicy(filepath.Join(root, "cmd", "fields", "sensitive-policy.json"))
	if err != nil {
		return err
	}
	raw, metadata, err := loadSensitivityInputs(snapshot)
	if err != nil {
		return err
	}
	stage := filepath.Join(os.TempDir(), "unused")
	stage, err = os.MkdirTemp(deps.tempRoot, "verify-regeneration-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	v, err := version.NewVersion(source.NetworkVersion)
	if err != nil {
		return err
	}
	if err := deps.render(snapshot, stage, v, func(resources []*ResourceInfo) error {
		_, err := ApplySensitivity(resources, raw, metadata, policy)
		return err
	}); err != nil {
		return err
	}
	files, digest, err := HashGeneratedFiles(stage, "unifi", "specification.json")
	if err != nil {
		return err
	}
	if different := firstHashDifference(source.GeneratedFiles, files); different != "" {
		return fmt.Errorf("regenerated file differs: %s", different)
	}
	if digest != source.GeneratedTreeDigest {
		return errors.New("regenerated tree digest differs")
	}
	fmt.Fprintln(stdout, "regeneration verified")
	return ctx.Err()
}

func publishGeneratedTree(root, stage, outputDir, specOutput, schemaSource string) error {
	return publishGeneratedTreeWithOps(root, stage, outputDir, specOutput, schemaSource, defaultPublishFileOps())
}

type publishFileOps struct {
	rename    func(string, string) error
	remove    func(string) error
	removeAll func(string) error
}

func defaultPublishFileOps() publishFileOps {
	return publishFileOps{rename: os.Rename, remove: os.Remove, removeAll: os.RemoveAll}
}

func publishGeneratedTreeWithOps(root, stage, outputDir, specOutput, schemaSource string, ops publishFileOps) error {
	current := filepath.Join(root, outputDir)
	replacement, err := os.MkdirTemp(root, ".generated-tree-*")
	if err != nil {
		return err
	}
	defer func() { _ = ops.removeAll(replacement) }()
	if err := copyTreeExcludingOwned(current, replacement); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := copyOwnedGenerated(filepath.Join(stage, outputDir), replacement); err != nil {
		return err
	}
	type fileSwap struct {
		target, prepared, backup string
		hadOld, installed        bool
	}
	swaps := make([]fileSwap, 0, 2)
	// Prepare and validate every file replacement before the first mutation.
	for _, rel := range []string{specOutput, schemaSource} {
		if rel == "" {
			continue
		}
		target := filepath.Join(root, rel)
		staged := filepath.Join(stage, rel)
		if info, err := os.Stat(staged); err != nil || !info.Mode().IsRegular() {
			if err == nil {
				err = fmt.Errorf("staged file is not regular")
			}
			return fmt.Errorf("validate staged %s: %w", rel, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		prepared, err := copyPreparedFile(staged, target)
		if err != nil {
			return fmt.Errorf("prepare staged %s: %w", rel, err)
		}
		defer func() { _ = ops.remove(prepared) }()
		holder, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".backup-*")
		if err != nil {
			return err
		}
		backupName := holder.Name()
		if err := holder.Close(); err != nil {
			return err
		}
		if err := ops.remove(backupName); err != nil {
			return err
		}
		swaps = append(swaps, fileSwap{target: target, prepared: prepared, backup: backupName})
	}
	dirBackup, err := os.MkdirTemp(root, "."+filepath.Base(outputDir)+".generated-backup-*")
	if err != nil {
		return err
	}
	if err := ops.removeAll(dirBackup); err != nil {
		return err
	}
	dirHadOld := false
	dirInstalled := false
	rollback := func(cause error) error {
		var recovery []string
		for i := len(swaps) - 1; i >= 0; i-- {
			swap := &swaps[i]
			if swap.installed {
				if err := ops.remove(swap.target); err != nil && !errors.Is(err, os.ErrNotExist) {
					recovery = append(recovery, err.Error())
				}
			}
			if swap.hadOld {
				if err := ops.rename(swap.backup, swap.target); err != nil {
					recovery = append(recovery, err.Error())
				}
			}
		}
		if dirInstalled {
			if err := ops.removeAll(current); err != nil && !errors.Is(err, os.ErrNotExist) {
				recovery = append(recovery, err.Error())
			}
		}
		if dirHadOld {
			if err := ops.rename(dirBackup, current); err != nil {
				recovery = append(recovery, err.Error())
			}
		}
		if len(recovery) > 0 {
			return fmt.Errorf("%w; rollback incomplete (backups retained): %s", cause, strings.Join(recovery, "; "))
		}
		return cause
	}
	if _, err := os.Stat(current); err == nil {
		if err := ops.rename(current, dirBackup); err != nil {
			return err
		}
		dirHadOld = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := ops.rename(replacement, current); err != nil {
		return rollback(err)
	}
	dirInstalled = true
	for i := range swaps {
		swap := &swaps[i]
		if _, err := os.Stat(swap.target); err == nil {
			if err := ops.rename(swap.target, swap.backup); err != nil {
				return rollback(err)
			}
			swap.hadOld = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return rollback(err)
		}
		if err := ops.rename(swap.prepared, swap.target); err != nil {
			return rollback(err)
		}
		swap.installed = true
	}
	for i := range swaps {
		if swaps[i].hadOld {
			_ = ops.remove(swaps[i].backup)
		}
	}
	if dirHadOld {
		_ = ops.removeAll(dirBackup)
	}
	return nil
}

func copyPreparedFile(source, target string) (string, error) {
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".prepared-*")
	if err != nil {
		return "", err
	}
	name := out.Name()
	keep := false
	defer func() {
		_ = out.Close()
		if !keep {
			_ = os.Remove(name)
		}
	}()
	if err := out.Chmod(0o644); err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	if err := out.Sync(); err != nil {
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	keep = true
	return name, nil
}

func copyTreeExcludingOwned(source, target string) error {
	return filepath.WalkDir(source, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(target, 0o755)
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("output tree contains non-regular file %s", name)
		}
		if strings.HasSuffix(entry.Name(), ".generated.go") {
			return nil
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, body, info.Mode().Perm())
	})
}
func copyOwnedGenerated(source, target string) error {
	return filepath.WalkDir(source, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(target, 0o755)
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(target, rel), 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("staged output contains non-regular file %s", name)
		}
		if !strings.HasSuffix(entry.Name(), ".generated.go") {
			return nil
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(target, rel), body, 0o644)
	})
}

func main() {
	if err := Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "fields:", err)
		os.Exit(1)
	}
}
