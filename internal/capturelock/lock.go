// Package capturelock defines and verifies the immutable controller artifact
// input used by schema generation.
package capturelock

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const FormatVersion = 1

const generatorEntrypointDirective = "//go:generate go run ../cmd/fields/ -output-dir=../unifi/"

type Lock struct {
	FormatVersion int        `json:"format_version"`
	Controller    Controller `json:"controller"`
	Source        Source     `json:"source"`
	Inputs        Inputs     `json:"inputs"`
	Snapshots     Snapshots  `json:"snapshots"`
	CapturedAt    string     `json:"captured_at"`
}

type Controller struct {
	Product        string `json:"product"`
	Build          string `json:"build"`
	NetworkVersion string `json:"network_version"`
	UOSVersion     string `json:"uos_version,omitempty"`
}

type Source struct {
	Location            string `json:"location"`
	MediaType           string `json:"media_type"`
	ByteSize            int64  `json:"byte_size"`
	SHA256              string `json:"sha256"`
	ContentStoreLocator string `json:"content_store_locator,omitempty"`
}

type Inputs struct {
	ExtractionRulesSHA256 string `json:"extraction_rules_sha256"`
	GeneratorInputsSHA256 string `json:"generator_inputs_sha256"`
}

type Snapshots struct {
	StructuralSHA256  string `json:"structural_sha256"`
	SensitivitySHA256 string `json:"sensitivity_sha256"`

	// FieldDocuments digests each extracted field definition on its own,
	// keyed by its path within the snapshot tree. StructuralSHA256 covers
	// the whole tree, so it moves whenever any definition moves, including
	// the eighty-one a given consumer does not care about. Anything that
	// needs to pin one surface pins its entry here instead, and stays valid
	// when an unrelated definition changes.
	//
	// Both come out of a single DigestSnapshot pass in cmd/fields, so they
	// cannot describe different files; the entries additionally say which
	// definition moved, which a tree digest on its own never could.
	FieldDocuments map[string]string `json:"field_documents,omitempty"`
}

type StoredArtifact struct {
	Path     string
	Locator  string
	ByteSize int64
	SHA256   string
}

type Inspection struct {
	NetworkVersion string    `json:"network_version"`
	Snapshots      Snapshots `json:"snapshots"`
}

func LoadFile(filename string) (Lock, error) {
	return loadFile(filename, true)
}

func LoadDraftFile(filename string) (Lock, error) {
	return loadFile(filename, false)
}

func loadFile(filename string, requireInspection bool) (Lock, error) {
	var lock Lock
	if err := decodeStrictJSONFile(filename, "capture lock", &lock); err != nil {
		return Lock{}, err
	}
	if err := lock.validate(requireInspection); err != nil {
		return Lock{}, err
	}
	return lock, nil
}

// decodeStrictJSONFile decodes filename into v, rejecting unknown fields and
// anything after the first JSON value.
func decodeStrictJSONFile(filename, what string, v any) error {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open %s: %w", what, err)
	}
	defer f.Close()

	decoder := json.NewDecoder(bufio.NewReader(f))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("decode %s: %w", what, err)
	}
	var extra any
	err = decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode %s trailer: %w", what, err)
	}
	return fmt.Errorf("%s contains more than one JSON value", what)
}

func (l Lock) Validate() error {
	return l.validate(true)
}

func (l Lock) validate(requireInspection bool) error {
	if l.FormatVersion != FormatVersion {
		return fmt.Errorf("format_version must be %d", FormatVersion)
	}
	for name, value := range map[string]string{
		"controller.product":             l.Controller.Product,
		"controller.build":               l.Controller.Build,
		"source.location":                l.Source.Location,
		"source.media_type":              l.Source.MediaType,
		"inputs.extraction_rules_sha256": l.Inputs.ExtractionRulesSHA256,
		"inputs.generator_inputs_sha256": l.Inputs.GeneratorInputsSHA256,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if l.Source.ByteSize <= 0 {
		return errors.New("source.byte_size must be positive")
	}
	for name, value := range map[string]string{
		"source.sha256":                  l.Source.SHA256,
		"inputs.extraction_rules_sha256": l.Inputs.ExtractionRulesSHA256,
		"inputs.generator_inputs_sha256": l.Inputs.GeneratorInputsSHA256,
	} {
		if !validSHA256(value) {
			return fmt.Errorf("%s must be 64 lowercase hexadecimal characters", name)
		}
	}
	if requireInspection {
		if strings.TrimSpace(l.Controller.NetworkVersion) == "" {
			return errors.New("controller.network_version is required")
		}
		if err := validateSnapshots(l.Snapshots); err != nil {
			return err
		}
	} else if err := validateFieldDocuments(l.Snapshots.FieldDocuments); err != nil {
		// A draft has no snapshots yet, but any field documents it does
		// carry still have to be well formed.
		return err
	}
	if _, err := time.Parse(time.RFC3339, l.CapturedAt); err != nil {
		return fmt.Errorf("captured_at must be RFC3339: %w", err)
	}
	if locator := l.Source.ContentStoreLocator; locator != "" {
		clean := path.Clean(locator)
		if strings.Contains(locator, `\`) || strings.HasPrefix(clean, "../") || clean == ".." || path.IsAbs(clean) || clean != locator {
			return errors.New("source.content_store_locator must be a clean relative slash path")
		}
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// validSnapshotMemberName accepts exactly the keys DigestSnapshot produces:
// clean, relative, slash-separated paths within the snapshot tree.
func validSnapshotMemberName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("must not be empty")
	}
	if strings.Contains(name, `\`) {
		return errors.New("must be slash separated")
	}
	clean := path.Clean(name)
	if clean != name || path.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return errors.New("must be a clean relative slash path")
	}
	return nil
}

func WriteFile(filename string, lock Lock) error {
	return writeFile(filename, lock, true)
}

func WriteDraftFile(filename string, lock Lock) error {
	return writeFile(filename, lock, false)
}

func LoadInspectionFile(filename string) (Inspection, error) {
	var inspection Inspection
	if err := decodeStrictJSONFile(filename, "capture inspection", &inspection); err != nil {
		return Inspection{}, err
	}
	if err := inspection.Validate(); err != nil {
		return Inspection{}, err
	}
	return inspection, nil
}

func WriteInspectionFile(filename string, inspection Inspection) error {
	if err := inspection.Validate(); err != nil {
		return err
	}
	return writeJSONFile(filename, inspection)
}

func (i Inspection) Validate() error {
	if strings.TrimSpace(i.NetworkVersion) == "" {
		return errors.New("network_version is required")
	}
	return validateSnapshots(i.Snapshots)
}

// validateSnapshots checks the complete digest set an inspected snapshot
// tree must carry.
func validateSnapshots(s Snapshots) error {
	if !validSHA256(s.StructuralSHA256) {
		return errors.New("snapshots.structural_sha256 must be 64 lowercase hexadecimal characters")
	}
	if !validSHA256(s.SensitivitySHA256) {
		return errors.New("snapshots.sensitivity_sha256 must be 64 lowercase hexadecimal characters")
	}
	if len(s.FieldDocuments) == 0 {
		return errors.New("snapshots.field_documents is required")
	}
	return validateFieldDocuments(s.FieldDocuments)
}

func validateFieldDocuments(docs map[string]string) error {
	for name, value := range docs {
		if err := validSnapshotMemberName(name); err != nil {
			return fmt.Errorf("snapshots.field_documents key %q %w", name, err)
		}
		if !validSHA256(value) {
			return fmt.Errorf("snapshots.field_documents.%s must be 64 lowercase hexadecimal characters", name)
		}
	}
	return nil
}

func writeFile(filename string, lock Lock, requireInspection bool) error {
	if err := lock.validate(requireInspection); err != nil {
		return err
	}
	return writeJSONFile(filename, lock)
}

// writeJSONFile writes v atomically in the canonical form both lock-adjacent
// files use: two-space indented, newline terminated.
func writeJSONFile(filename string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filename, append(data, '\n'), 0o644)
}

func ResolveArtifact(contentStore string, lock Lock) (string, error) {
	// Capture inspection resolves an otherwise complete draft before the
	// controller version and extracted snapshot digests are known. Full locks
	// have already passed LoadFile; validating the invariant source identity
	// here keeps both paths fail-closed on the retained bytes.
	if err := lock.validate(false); err != nil {
		return "", err
	}
	if strings.TrimSpace(contentStore) == "" {
		return "", errors.New("content store path is required")
	}
	locator := lock.Source.ContentStoreLocator
	if locator == "" {
		locator = path.Join("sha256", lock.Source.SHA256)
	}
	artifact := filepath.Join(contentStore, filepath.FromSlash(locator))
	stat, err := os.Stat(artifact)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("retained artifact %s is missing", locator)
		}
		return "", fmt.Errorf("stat retained artifact: %w", err)
	}
	if !stat.Mode().IsRegular() {
		return "", fmt.Errorf("retained artifact %s is not a regular file", locator)
	}
	if stat.Size() != lock.Source.ByteSize {
		return "", fmt.Errorf("retained artifact byte size is %d, lock requires %d", stat.Size(), lock.Source.ByteSize)
	}
	digest, err := DigestFile(artifact)
	if err != nil {
		return "", err
	}
	if digest != lock.Source.SHA256 {
		return "", fmt.Errorf("retained artifact SHA-256 is %s, lock requires %s", digest, lock.Source.SHA256)
	}
	return artifact, nil
}

func DigestFile(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DigestSnapshot measures root once and returns both digests the capture lock
// records for a snapshot: the whole-tree digest, and each document's own
// digest keyed by its tree-relative slash path.
//
// One traversal produces both, so the two values cannot come to describe
// different files. That is the point rather than an optimisation. The lock
// pins the tree while anything that pins a single document out of it reads
// from this same map; if the map could omit a file the tree digest covered,
// such a pin could describe a document the lock did not actually cover, and
// the two checks would look agreeing while measuring different things.
//
// Keys are tree-relative rather than bare file names because the tree digest
// has always descended into subdirectories. Every field definition sits at the
// top level today, so in practice the keys are file names, but a nested
// document has to stay addressable or it would be inside the tree digest and
// outside the map.
func DigestSnapshot(root string) (string, map[string]string, error) {
	var names []string
	err := filepath.WalkDir(root, func(filename string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("snapshot member %s is not a regular file", filename)
		}
		rel, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		names = append(names, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	if len(names) == 0 {
		return "", nil, errors.New("snapshot tree is empty")
	}
	sort.Strings(names)
	documents := make(map[string]string, len(names))
	h := sha256.New()
	for _, name := range names {
		if _, err := io.WriteString(h, name); err != nil {
			return "", nil, err
		}
		if _, err := h.Write([]byte{0}); err != nil {
			return "", nil, err
		}
		filename := filepath.Join(root, filepath.FromSlash(name))
		member := sha256.New()
		f, err := os.Open(filename)
		if err != nil {
			return "", nil, err
		}
		_, copyErr := io.Copy(io.MultiWriter(h, member), f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", nil, copyErr
		}
		if closeErr != nil {
			return "", nil, closeErr
		}
		documents[name] = hex.EncodeToString(member.Sum(nil))
		if _, err := h.Write([]byte{0}); err != nil {
			return "", nil, err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), documents, nil
}

func StoreArtifact(sourceFilename, contentStore string) (StoredArtifact, error) {
	stat, err := os.Stat(sourceFilename)
	if err != nil {
		return StoredArtifact{}, fmt.Errorf("stat source artifact: %w", err)
	}
	if !stat.Mode().IsRegular() {
		return StoredArtifact{}, errors.New("source artifact is not a regular file")
	}
	digest, err := DigestFile(sourceFilename)
	if err != nil {
		return StoredArtifact{}, fmt.Errorf("digest source artifact: %w", err)
	}
	locator := path.Join("sha256", digest)
	destination := filepath.Join(contentStore, filepath.FromSlash(locator))
	stored := StoredArtifact{
		Path:     destination,
		Locator:  locator,
		ByteSize: stat.Size(),
		SHA256:   digest,
	}

	if existing, err := os.Stat(destination); err == nil {
		if !existing.Mode().IsRegular() || existing.Size() != stored.ByteSize {
			return StoredArtifact{}, errors.New("existing content-store object does not match source artifact")
		}
		existingDigest, err := DigestFile(destination)
		if err != nil {
			return StoredArtifact{}, err
		}
		if existingDigest != digest {
			return StoredArtifact{}, errors.New("existing content-store object does not match source artifact")
		}
		return stored, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return StoredArtifact{}, fmt.Errorf("stat content-store object: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return StoredArtifact{}, err
	}
	source, err := os.Open(sourceFilename)
	if err != nil {
		return StoredArtifact{}, err
	}
	defer source.Close()
	if err := writeAtomicFrom(destination, source, 0o600); err != nil {
		return StoredArtifact{}, err
	}
	return stored, nil
}

func ComputeInputDigests(moduleRoot string) (Inputs, error) {
	if err := validateGeneratorEntrypoint(moduleRoot); err != nil {
		return Inputs{}, err
	}

	extractionFiles := []string{"cmd/fields/extract.go"}
	// go.mod and go.sum are deliberately absent: a dependency bump changes
	// nothing the generator reads, so it must not move this digest. The
	// rebuild workflow regenerates from scratch on every push touching
	// either file, which is what proves dependency-caused output drift.
	//
	// schemas/behavior.json is a measured generator input: the write
	// contracts and coercions it records change what the generator emits, so
	// a re-measure must move this digest exactly like an override edit does.
	generatorFiles := []string{"unifi/unifi.go"}
	if _, err := os.Stat(filepath.Join(moduleRoot, "schemas", "behavior.json")); err == nil {
		generatorFiles = append(generatorFiles, "schemas/behavior.json")
	}

	// wirecontract/ carries the second generate directive and the artifact's
	// published layout, so editing either moves this digest as an override edit
	// does. Its own .json output is not a .go file and is skipped below.
	for _, dir := range []string{
		"cmd/fields",
		"cmd/wirecontract",
		"wirecontract",
		"internal/behavior",
		"internal/capturelock",
		"internal/fields",
		"overrides",
	} {
		root := filepath.Join(moduleRoot, filepath.FromSlash(dir))
		if _, statErr := os.Stat(root); os.IsNotExist(statErr) {
			// A tree from before the directory existed still digests: the
			// inputs it lacked cannot have shaped its output.
			continue
		}
		err := filepath.WalkDir(root, func(filename string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("generator input %s is not a regular file", filename)
			}
			name := filepath.ToSlash(filename)
			if strings.HasSuffix(name, "_test.go") {
				return nil
			}
			if dir != "overrides" && filepath.Ext(name) != ".go" && filepath.Ext(name) != ".tmpl" {
				return nil
			}
			rel, err := filepath.Rel(moduleRoot, filename)
			if err != nil {
				return err
			}
			generatorFiles = append(generatorFiles, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return Inputs{}, err
		}
	}

	extractionDigest, err := digestNamedFiles(moduleRoot, extractionFiles)
	if err != nil {
		return Inputs{}, err
	}
	generatorDigest, err := digestNamedFiles(moduleRoot, generatorFiles)
	if err != nil {
		return Inputs{}, err
	}
	return Inputs{
		ExtractionRulesSHA256: extractionDigest,
		GeneratorInputsSHA256: generatorDigest,
	}, nil
}

func validateGeneratorEntrypoint(moduleRoot string) error {
	filename := filepath.Join(moduleRoot, "unifi", "unifi.go")
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open generator entrypoint: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "//go:generate") {
			continue
		}
		if line != generatorEntrypointDirective {
			return fmt.Errorf("unifi/unifi.go go:generate directive must be %q", generatorEntrypointDirective)
		}
		if found {
			return fmt.Errorf("unifi/unifi.go must contain only one go:generate directive")
		}
		found = true
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read generator entrypoint: %w", err)
	}
	if found {
		return nil
	}
	return fmt.Errorf("unifi/unifi.go must contain go:generate directive %q", generatorEntrypointDirective)
}

func digestNamedFiles(root string, names []string) (string, error) {
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		if _, err := io.WriteString(h, name); err != nil {
			return "", err
		}
		if _, err := h.Write([]byte{0}); err != nil {
			return "", err
		}
		filename := filepath.Join(root, filepath.FromSlash(name))
		f, err := os.Open(filename)
		if err != nil {
			return "", fmt.Errorf("open input %s: %w", name, err)
		}
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if _, err := h.Write([]byte{0}); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func WriteCompatibilityProjections(schemasDir string, lock Lock) error {
	if err := lock.Validate(); err != nil {
		return err
	}
	values := map[string]string{
		"VERSION":  lock.Controller.NetworkVersion,
		"SOURCE":   lock.Controller.Product + " " + lock.Controller.Build,
		"ARTIFACT": lock.Source.Location,
	}
	for _, name := range []string{"VERSION", "SOURCE", "ARTIFACT"} {
		if err := writeAtomic(filepath.Join(schemasDir, name), []byte(values[name]+"\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeAtomic(filename string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	return writeAtomicFrom(filename, bytes.NewReader(data), mode)
}

// writeAtomicFrom copies src to a temp file beside filename and renames it
// into place, so a reader never sees a partial write.
func writeAtomicFrom(filename string, src io.Reader, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(filename), ".capture-lock-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filename)
}
