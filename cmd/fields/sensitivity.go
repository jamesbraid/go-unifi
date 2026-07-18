package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
)

type SensitivityPolicy struct {
	Version                 string   `json:"version"`
	ApprovedMetadataSHA256  []string `json:"approved_metadata_sha256"`
	ApprovedNoticeSHA256    []string `json:"approved_notice_sha256"`
	SecretPaths             []string `json:"secret_paths"`
	NonGeneratedSecretPaths []string `json:"non_generated_secret_paths"`
}

type SensitiveMetadata struct {
	MinFieldSize     int                 `json:"min_field_size"`
	DefaultNames     []string            `json:"default_names"`
	SystemProperties []string            `json:"sensitive_system_properties"`
	DBFields         map[string][]string `json:"sensitive_db_fields_by_collection"`
	DistinctDBFields map[string]string   `json:"sensitive_distinct_db_fields_by_collection"`
}

type SensitivityCoverage struct {
	Generated           []string
	NonGenerated        []string
	SecretGenerated     []string
	SecretNonGenerated  []string
	PrivateGenerated    []string
	PrivateNonGenerated []string
}

type RawSchemaIndex map[string]json.RawMessage

func LoadSensitivityPolicy(path string) (SensitivityPolicy, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return SensitivityPolicy{}, fmt.Errorf("read sensitivity policy: %w", err)
	}
	var policy SensitivityPolicy
	if err := decodeStrictJSON(body, &policy); err != nil {
		return SensitivityPolicy{}, fmt.Errorf("parse sensitivity policy: %w", err)
	}
	if policy.Version != "1" {
		return SensitivityPolicy{}, fmt.Errorf("unsupported sensitivity policy version %q", policy.Version)
	}
	if policy.ApprovedMetadataSHA256 == nil || policy.ApprovedNoticeSHA256 == nil || policy.SecretPaths == nil || policy.NonGeneratedSecretPaths == nil {
		return SensitivityPolicy{}, errors.New("sensitivity policy arrays must be present")
	}
	for _, digest := range policy.ApprovedMetadataSHA256 {
		if !validSHA256(digest) {
			return SensitivityPolicy{}, fmt.Errorf("invalid approved metadata SHA-256 %q", digest)
		}
	}
	if !sort.StringsAreSorted(policy.ApprovedNoticeSHA256) {
		return SensitivityPolicy{}, errors.New("approved notice SHA-256 values must be sorted")
	}
	for i, digest := range policy.ApprovedNoticeSHA256 {
		if !validSHA256(digest) {
			return SensitivityPolicy{}, fmt.Errorf("invalid approved notice SHA-256 %q", digest)
		}
		if i > 0 && digest == policy.ApprovedNoticeSHA256[i-1] {
			return SensitivityPolicy{}, fmt.Errorf("duplicate approved notice SHA-256 %q", digest)
		}
	}
	for _, secretPath := range policy.SecretPaths {
		if _, _, err := splitSensitivityPath(secretPath); err != nil {
			return SensitivityPolicy{}, fmt.Errorf("invalid secret path %q: %w", secretPath, err)
		}
	}
	for _, secretPath := range policy.NonGeneratedSecretPaths {
		if _, _, err := splitSensitivityPath(secretPath); err != nil {
			return SensitivityPolicy{}, fmt.Errorf("invalid non-generated secret path %q: %w", secretPath, err)
		}
	}
	return policy, nil
}

func validSHA256(digest string) bool {
	decoded, err := hex.DecodeString(digest)
	return err == nil && len(decoded) == sha256.Size && digest == strings.ToLower(digest)
}

func RequireApprovedNoticeDigest(policy SensitivityPolicy, digest string) error {
	if !validSHA256(digest) {
		return fmt.Errorf("invalid notice digest %q", digest)
	}
	if !containsString(policy.ApprovedNoticeSHA256, digest) {
		return fmt.Errorf("notice digest %s is not approved", digest)
	}
	return nil
}

func ParseSensitiveMetadata(data []byte) (SensitiveMetadata, error) {
	var presence struct {
		MinFieldSize     *json.RawMessage `json:"min_field_size"`
		DefaultNames     *json.RawMessage `json:"default_names"`
		SystemProperties *json.RawMessage `json:"sensitive_system_properties"`
		DBFields         *json.RawMessage `json:"sensitive_db_fields_by_collection"`
		DistinctDBFields *json.RawMessage `json:"sensitive_distinct_db_fields_by_collection"`
	}
	if err := decodeStrictJSON(data, &presence); err != nil {
		return SensitiveMetadata{}, fmt.Errorf("parse sensitivity metadata: %w", err)
	}
	if presence.MinFieldSize == nil || presence.DefaultNames == nil || presence.SystemProperties == nil || presence.DBFields == nil || presence.DistinctDBFields == nil {
		return SensitiveMetadata{}, errors.New("sensitivity metadata is missing required fields")
	}
	var metadata SensitiveMetadata
	if err := decodeStrictJSON(data, &metadata); err != nil {
		return SensitiveMetadata{}, fmt.Errorf("parse sensitivity metadata: %w", err)
	}
	if metadata.MinFieldSize < 0 || metadata.DefaultNames == nil || metadata.SystemProperties == nil || metadata.DBFields == nil || metadata.DistinctDBFields == nil {
		return SensitiveMetadata{}, errors.New("sensitivity metadata fields must have non-null values")
	}
	return metadata, nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func SetResourceSourceIdentity(resource *ResourceInfo, filename string, rawSetting []byte) error {
	if resource == nil {
		return errors.New("resource is nil")
	}
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	if base == "" {
		return errors.New("source filename has no base")
	}
	if !strings.HasPrefix(base, "Setting") || base == "Setting" {
		resource.SourceFileBase = base
		resource.SourcePathPrefix = nil
		return nil
	}
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(rawSetting, &sections); err != nil {
		return fmt.Errorf("parse unsplit Setting.json for %s: %w", filename, err)
	}
	matches := make([]string, 0, 1)
	for rawKey := range sections {
		if "Setting"+strcase.ToCamel(rawKey) == base {
			matches = append(matches, rawKey)
		}
	}
	sort.Strings(matches)
	if len(matches) != 1 {
		return fmt.Errorf("source identity for %s matched %d Setting.json keys: %v", filename, len(matches), matches)
	}
	resource.SourceFileBase = "Setting"
	resource.SourcePathPrefix = []string{matches[0]}
	return nil
}

type resolvedRawPath struct {
	collection string
	segments   []string
	canonical  string
}

type sensitivityResolver struct {
	rawByFold map[string]struct {
		name string
		body json.RawMessage
	}
	resources []*ResourceInfo
}

func ApplySensitivity(resources []*ResourceInfo, raw RawSchemaIndex, metadataBody []byte, policy SensitivityPolicy) (SensitivityCoverage, error) {
	if policy.Version != "1" {
		return SensitivityCoverage{}, fmt.Errorf("unsupported sensitivity policy version %q", policy.Version)
	}
	metadata, err := ParseSensitiveMetadata(metadataBody)
	if err != nil {
		return SensitivityCoverage{}, err
	}
	digest, err := CanonicalJSONDigest(metadataBody)
	if err != nil {
		return SensitivityCoverage{}, fmt.Errorf("digest sensitivity metadata: %w", err)
	}
	if !containsString(policy.ApprovedMetadataSHA256, digest) {
		return SensitivityCoverage{}, fmt.Errorf("sensitivity metadata digest %s is not approved", digest)
	}
	resolver, err := newSensitivityResolver(resources, raw)
	if err != nil {
		return SensitivityCoverage{}, err
	}

	generated := make(map[string]struct{})
	nonGenerated := make(map[string]struct{})
	canonicalPaths := make(canonicalPathRegistry)
	classified := make([]string, 0)
	collections := make([]string, 0, len(metadata.DBFields))
	for collection := range metadata.DBFields {
		collections = append(collections, collection)
	}
	sort.Strings(collections)
	for _, collection := range collections {
		for _, fieldPath := range metadata.DBFields[collection] {
			classified = append(classified, collection+"."+fieldPath)
		}
	}
	collections = collections[:0]
	for collection := range metadata.DistinctDBFields {
		collections = append(collections, collection)
	}
	sort.Strings(collections)
	for _, collection := range collections {
		classified = append(classified, collection+"."+metadata.DistinctDBFields[collection])
	}
	for _, property := range metadata.SystemProperties {
		segments, err := splitPathSegments(property)
		if err != nil {
			return SensitivityCoverage{}, fmt.Errorf("invalid sensitive system property %q: %w", property, err)
		}
		canonical := canonicalSensitivityPath("systemproperty", segments)
		if err := canonicalPaths.add(canonical, "systemproperty."+property); err != nil {
			return SensitivityCoverage{}, err
		}
		nonGenerated[canonical] = struct{}{}
	}

	for _, path := range classified {
		resolved, missing, err := resolver.resolveRaw(path)
		if err != nil {
			return SensitivityCoverage{}, err
		}
		if len(resolved) == 0 {
			if err := canonicalPaths.add(missing, path); err != nil {
				return SensitivityCoverage{}, err
			}
			nonGenerated[missing] = struct{}{}
			continue
		}
		for _, rawPath := range resolved {
			origin, err := canonicalOrigin(path, rawPath.segments)
			if err != nil {
				return SensitivityCoverage{}, err
			}
			if err := canonicalPaths.add(rawPath.canonical, origin); err != nil {
				return SensitivityCoverage{}, err
			}
			field, found, err := resolver.resolveGenerated(rawPath)
			if err != nil {
				return SensitivityCoverage{}, err
			}
			if found && field != nil {
				generated[rawPath.canonical] = struct{}{}
			} else {
				nonGenerated[rawPath.canonical] = struct{}{}
			}
		}
	}

	secretFields := make(map[*FieldInfo]struct{})
	secretGenerated := make(map[string]struct{})
	secretNonGenerated := make(map[string]struct{})
	for _, secretPath := range policy.SecretPaths {
		resolved, _, err := resolver.resolveRaw(secretPath)
		if err != nil {
			return SensitivityCoverage{}, fmt.Errorf("resolve policy secret %q: %w", secretPath, err)
		}
		if len(resolved) != 1 {
			return SensitivityCoverage{}, fmt.Errorf("policy secret %q resolves to %d raw fields", secretPath, len(resolved))
		}
		origin, err := canonicalOrigin(secretPath, resolved[0].segments)
		if err != nil {
			return SensitivityCoverage{}, err
		}
		if err := canonicalPaths.add(resolved[0].canonical, origin); err != nil {
			return SensitivityCoverage{}, err
		}
		field, found, err := resolver.resolveGenerated(resolved[0])
		if err != nil {
			return SensitivityCoverage{}, fmt.Errorf("resolve policy secret %q: %w", secretPath, err)
		}
		if !found || field == nil {
			return SensitivityCoverage{}, fmt.Errorf("policy secret %q does not resolve to exactly one generated leaf", secretPath)
		}
		secretFields[field] = struct{}{}
		generated[resolved[0].canonical] = struct{}{}
		delete(nonGenerated, resolved[0].canonical)
		secretGenerated[resolved[0].canonical] = struct{}{}
	}
	for _, secretPath := range policy.NonGeneratedSecretPaths {
		collection, segments, err := splitSensitivityPath(secretPath)
		if err != nil {
			return SensitivityCoverage{}, fmt.Errorf("invalid non-generated policy secret %q: %w", secretPath, err)
		}
		resolved, missing, err := resolver.resolveRaw(secretPath)
		if err != nil {
			return SensitivityCoverage{}, fmt.Errorf("resolve non-generated policy secret %q: %w", secretPath, err)
		}
		if len(resolved) == 0 {
			if _, classified := nonGenerated[missing]; !classified {
				return SensitivityCoverage{}, fmt.Errorf("non-generated policy secret %q is absent and not present in approved sensitivity metadata", secretPath)
			}
			if err := canonicalPaths.add(missing, secretPath); err != nil {
				return SensitivityCoverage{}, err
			}
			field, found, err := resolver.resolveGenerated(resolvedRawPath{
				collection: collection,
				segments:   segments,
				canonical:  missing,
			})
			if err != nil {
				return SensitivityCoverage{}, fmt.Errorf("resolve non-generated policy secret %q without raw collection: %w", secretPath, err)
			}
			if found || field != nil {
				return SensitivityCoverage{}, fmt.Errorf("non-generated policy secret %q became generated and requires policy review", secretPath)
			}
			secretNonGenerated[missing] = struct{}{}
			nonGenerated[missing] = struct{}{}
			continue
		}
		if len(resolved) != 1 {
			return SensitivityCoverage{}, fmt.Errorf("non-generated policy secret %q resolves to %d raw fields", secretPath, len(resolved))
		}
		origin, err := canonicalOrigin(secretPath, resolved[0].segments)
		if err != nil {
			return SensitivityCoverage{}, err
		}
		if err := canonicalPaths.add(resolved[0].canonical, origin); err != nil {
			return SensitivityCoverage{}, err
		}
		field, found, err := resolver.resolveGenerated(resolved[0])
		if err != nil {
			return SensitivityCoverage{}, fmt.Errorf("resolve non-generated policy secret %q: %w", secretPath, err)
		}
		if found || field != nil {
			return SensitivityCoverage{}, fmt.Errorf("non-generated policy secret %q became generated and requires policy review", secretPath)
		}
		secretNonGenerated[resolved[0].canonical] = struct{}{}
		nonGenerated[resolved[0].canonical] = struct{}{}
		delete(generated, resolved[0].canonical)
	}

	// Mutation is intentionally last: every parse, digest, coverage, and identity
	// check above can fail without disturbing the previously applied policy.
	allFields := reachableFields(resources)
	for field := range allFields {
		field.Sensitive = false
	}
	for field := range secretFields {
		field.Sensitive = true
	}
	privateGenerated := subtractSet(generated, secretGenerated)
	privateNonGenerated := subtractSet(nonGenerated, secretNonGenerated)
	return SensitivityCoverage{
		Generated:           sortedSet(generated),
		NonGenerated:        sortedSet(nonGenerated),
		SecretGenerated:     sortedSet(secretGenerated),
		SecretNonGenerated:  sortedSet(secretNonGenerated),
		PrivateGenerated:    sortedSet(privateGenerated),
		PrivateNonGenerated: sortedSet(privateNonGenerated),
	}, nil
}

func newSensitivityResolver(resources []*ResourceInfo, raw RawSchemaIndex) (*sensitivityResolver, error) {
	resolver := &sensitivityResolver{rawByFold: make(map[string]struct {
		name string
		body json.RawMessage
	}, len(raw)), resources: resources}
	for name, body := range raw {
		if name == "" {
			return nil, errors.New("raw schema has an empty file base")
		}
		folded := strings.ToLower(name)
		if previous, ok := resolver.rawByFold[folded]; ok {
			return nil, fmt.Errorf("ambiguous raw schema collection identity %q and %q", previous.name, name)
		}
		resolver.rawByFold[folded] = struct {
			name string
			body json.RawMessage
		}{name: name, body: body}
	}
	return resolver, nil
}

func (r *sensitivityResolver) resolveRaw(path string) ([]resolvedRawPath, string, error) {
	collection, segments, err := splitSensitivityPath(path)
	if err != nil {
		return nil, "", fmt.Errorf("invalid sensitivity path %q: %w", path, err)
	}
	canonicalMissing := canonicalSensitivityPath(collection, segments)
	entry, ok := r.rawByFold[strings.ToLower(collection)]
	if !ok {
		return nil, canonicalMissing, nil
	}
	root, err := rawObject(entry.body)
	if err != nil {
		return nil, "", fmt.Errorf("raw collection %s: %w", entry.name, err)
	}
	if !strings.EqualFold(collection, "setting") {
		found, err := traverseRaw(root, segments)
		if err != nil {
			return nil, "", fmt.Errorf("raw path %s: %w", path, err)
		}
		if !found {
			return nil, canonicalMissing, nil
		}
		return []resolvedRawPath{{collection: entry.name, segments: segments, canonical: canonicalSensitivityPath(entry.name, segments)}}, "", nil
	}

	// Explicitly qualified settings address one raw top-level section. Otherwise
	// the metadata path is expanded in stable key order across every section.
	if _, qualified := root[segments[0]]; qualified {
		found, err := traverseRaw(root, segments)
		if err != nil {
			return nil, "", fmt.Errorf("raw path %s: %w", path, err)
		}
		if !found {
			return nil, canonicalMissing, nil
		}
		return []resolvedRawPath{{collection: entry.name, segments: segments, canonical: canonicalSensitivityPath(entry.name, segments)}}, "", nil
	}
	sectionNames := make([]string, 0, len(root))
	for section := range root {
		sectionNames = append(sectionNames, section)
	}
	sort.Strings(sectionNames)
	resolved := make([]resolvedRawPath, 0)
	for _, section := range sectionNames {
		sectionRoot, err := rawObject(root[section])
		if err != nil {
			return nil, "", fmt.Errorf("raw setting section %s: %w", section, err)
		}
		found, err := traverseRaw(sectionRoot, segments)
		if err != nil {
			return nil, "", fmt.Errorf("raw path setting.%s.%s: %w", section, strings.Join(segments, "."), err)
		}
		if found {
			expanded := append([]string{section}, segments...)
			resolved = append(resolved, resolvedRawPath{collection: entry.name, segments: expanded, canonical: canonicalSensitivityPath(entry.name, expanded)})
		}
	}
	return resolved, canonicalMissing, nil
}

func rawObject(body json.RawMessage) (map[string]json.RawMessage, error) {
	current := body
	for {
		var array []json.RawMessage
		if err := json.Unmarshal(current, &array); err == nil && array != nil {
			if len(array) != 1 {
				return nil, fmt.Errorf("schema array has %d elements, want 1", len(array))
			}
			current = array[0]
			continue
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(current, &object); err != nil || object == nil {
			return nil, errors.New("schema value is not an object")
		}
		return object, nil
	}
}

func traverseRaw(root map[string]json.RawMessage, segments []string) (bool, error) {
	current := root
	for index, segment := range segments {
		value, ok := current[segment]
		if !ok {
			return false, nil
		}
		if index == len(segments)-1 {
			return true, nil
		}
		next, err := rawObject(value)
		if err != nil {
			return false, fmt.Errorf("cannot traverse %q: %w", strings.Join(segments[:index+1], "."), err)
		}
		current = next
	}
	return false, nil
}

func (r *sensitivityResolver) resolveGenerated(rawPath resolvedRawPath) (*FieldInfo, bool, error) {
	candidates := make([]*ResourceInfo, 0, 1)
	for _, resource := range r.resources {
		if resource == nil || !strings.EqualFold(resource.SourceFileBase, rawPath.collection) {
			continue
		}
		if hasPathPrefix(rawPath.segments, resource.SourcePathPrefix) {
			candidates = append(candidates, resource)
		}
	}
	if len(candidates) == 0 {
		return nil, false, nil
	}
	if len(candidates) != 1 {
		return nil, false, fmt.Errorf("ambiguous generated collection identity for %s", rawPath.canonical)
	}
	resource := candidates[0]
	if resource.IsSetting() {
		return nil, false, nil
	}
	segments := rawPath.segments[len(resource.SourcePathPrefix):]
	if len(segments) == 0 {
		return nil, false, nil
	}
	base := resource.Types[resource.StructName]
	if base == nil || base.Fields == nil {
		return nil, false, nil
	}
	fields := base.Fields
	var current *FieldInfo
	for index, segment := range segments {
		byJSON := make(map[string]*FieldInfo, len(fields))
		for mapName, field := range fields {
			if field == nil {
				continue
			}
			if index == 0 && !isTerraformTopLevelField(mapName, field) {
				continue
			}
			if _, duplicate := byJSON[field.JSONName]; duplicate {
				return nil, false, fmt.Errorf("duplicate JSONName %q while resolving %s", field.JSONName, rawPath.canonical)
			}
			byJSON[field.JSONName] = field
		}
		var ok bool
		current, ok = byJSON[segment]
		if !ok {
			return nil, false, nil
		}
		children := generatedChildren(resource, current)
		if index == len(segments)-1 {
			if children != nil {
				return nil, false, nil
			}
			return current, true, nil
		}
		if children == nil {
			return nil, false, nil
		}
		fields = children
	}
	return nil, false, nil
}

func generatedChildren(resource *ResourceInfo, field *FieldInfo) map[string]*FieldInfo {
	if field.Fields != nil {
		return field.Fields
	}
	if custom := resource.Types[field.FieldType]; custom != nil {
		return custom.Fields
	}
	return nil
}

func splitSensitivityPath(path string) (string, []string, error) {
	parts, err := splitPathSegments(path)
	if err != nil {
		return "", nil, err
	}
	if len(parts) < 2 {
		return "", nil, errors.New("path must include collection and field")
	}
	return parts[0], parts[1:], nil
}

func splitPathSegments(path string) ([]string, error) {
	if path == "" || strings.TrimSpace(path) != path {
		return nil, errors.New("path is empty or has surrounding whitespace")
	}
	parts := strings.Split(path, ".")
	for _, part := range parts {
		if part == "" || strings.TrimSpace(part) != part {
			return nil, errors.New("path contains an empty or whitespace segment")
		}
	}
	return parts, nil
}

func canonicalSensitivityPath(collection string, segments []string) string {
	return strings.ToLower(collection) + "." + strings.Join(segments, ".")
}

type canonicalPathRegistry map[string]string

func (r canonicalPathRegistry) add(canonical, origin string) error {
	if previous, ok := r[canonical]; ok && previous != origin {
		return fmt.Errorf("canonical sensitivity path collision %q from %q and %q", canonical, previous, origin)
	}
	r[canonical] = origin
	return nil
}

func canonicalOrigin(input string, resolvedSegments []string) (string, error) {
	collection, _, err := splitSensitivityPath(input)
	if err != nil {
		return "", err
	}
	return collection + "." + strings.Join(resolvedSegments, "."), nil
}

func hasPathPrefix(path, prefix []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for index := range prefix {
		if path[index] != prefix[index] {
			return false
		}
	}
	return true
}

func reachableFields(resources []*ResourceInfo) map[*FieldInfo]struct{} {
	result := make(map[*FieldInfo]struct{})
	var visit func(*FieldInfo)
	visit = func(field *FieldInfo) {
		if field == nil {
			return
		}
		if _, seen := result[field]; seen {
			return
		}
		result[field] = struct{}{}
		for _, child := range field.Fields {
			visit(child)
		}
	}
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		for _, field := range resource.Types {
			visit(field)
		}
	}
	return result
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func subtractSet(values, remove map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for value := range values {
		if _, removed := remove[value]; !removed {
			result[value] = struct{}{}
		}
	}
	return result
}
