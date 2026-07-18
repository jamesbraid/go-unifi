package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	pemPrivateKey = regexp.MustCompile(`-----BEGIN (?:[A-Z0-9 ]+ )?PRIVATE KEY-----`)
	jwtToken      = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{8,}\b`)
	awsAccessKey  = regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`)
	gcpAPIKey     = regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)
)

var knownMetadataShapes = map[string]byte{
	"country_codes_list.json": '[', "event_defs.json": '{', "geo_ip_country_codes_list.json": '[',
	"legacy_endpoint_segments.json": '{', "radio_specification.json": '{', "sensitive_metadata.json": '{',
	"ssl-inspection-file-extension.json": '[', "timezones.json": '[',
}

// ScanExtractedInputs validates the deliberately small set of generator inputs.
// It rejects credential-shaped concrete values, but treats schema regex strings
// as syntax rather than attempting unreliable entropy classification.
func ScanExtractedInputs(root string) error {
	return filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("input is not a regular file: %s", name)
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "metadata/notices/") || rel == "metadata/source.json" {
			return nil
		}
		if filepath.Ext(rel) != ".json" {
			return fmt.Errorf("unexpected extracted input %s", rel)
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if err := rejectCredentialMaterial(body); err != nil {
			return fmt.Errorf("scan %s: %w", rel, err)
		}
		var value any
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		if err := dec.Decode(&value); err != nil {
			return fmt.Errorf("scan %s: invalid JSON: %w", rel, err)
		}
		var trailing any
		if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
			return fmt.Errorf("scan %s: trailing JSON", rel)
		}
		if strings.HasPrefix(rel, "metadata/") && !strings.HasPrefix(rel, "metadata/raw-fields/") {
			base := filepath.Base(rel)
			shape, ok := knownMetadataShapes[base]
			if !ok {
				return fmt.Errorf("unexpected metadata file %s", rel)
			}
			if base == "sensitive_metadata.json" {
				if _, err := ParseSensitiveMetadata(body); err != nil {
					return fmt.Errorf("scan %s: %w", rel, err)
				}
				return nil
			}
			if !matchesJSONShape(value, shape) {
				return fmt.Errorf("metadata %s has unexpected top-level structure", rel)
			}
			return validateMetadataValues(value)
		}
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("field schema %s must be an object", rel)
		}
		return validateSchemaNode(object)
	})
}

func rejectCredentialMaterial(body []byte) error {
	text := string(body)
	for label, pattern := range map[string]*regexp.Regexp{"PEM private key": pemPrivateKey, "JWT-shaped token": jwtToken, "AWS access key": awsAccessKey, "Google API key": gcpAPIKey} {
		if pattern.MatchString(text) {
			return fmt.Errorf("contains %s", label)
		}
	}
	return nil
}

func matchesJSONShape(v any, shape byte) bool {
	if shape == '{' {
		_, ok := v.(map[string]any)
		return ok
	}
	_, ok := v.([]any)
	return ok
}

func validateMetadataValues(v any) error {
	switch x := v.(type) {
	case map[string]any:
		for _, child := range x {
			if err := validateMetadataValues(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range x {
			if err := validateMetadataValues(child); err != nil {
				return err
			}
		}
	case string, json.Number, bool, nil:
	default:
		return fmt.Errorf("unsupported metadata value type %T", v)
	}
	return nil
}

func validateSchemaNode(v any) error {
	switch x := v.(type) {
	case map[string]any:
		for _, child := range x {
			if err := validateSchemaNode(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range x {
			if err := validateSchemaNode(child); err != nil {
				return err
			}
		}
	case string:
		if !schemaString(x) {
			return fmt.Errorf("unexpected concrete scalar %q", x)
		}
	case json.Number, bool, nil:
	default:
		return errors.New("unsupported schema structure")
	}
	return nil
}

func schemaString(value string) bool {
	if value == "" || value == "true|false" {
		return true
	}
	switch value {
	case "string", "number", "integer", "boolean", "object", "array", "null":
		return true
	}
	if strings.ContainsAny(value, `.*+?{}[]()|\\^$`) {
		return true
	}
	parts := strings.Split(value, "|")
	if len(parts) > 1 {
		for _, part := range parts {
			if part == "" || strings.ToUpper(part) != part {
				return false
			}
		}
		return true
	}
	return false
}
