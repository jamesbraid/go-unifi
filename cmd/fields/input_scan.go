package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	pemPrivateKey          = regexp.MustCompile(`-----BEGIN (?:[A-Z0-9 ]+ )?PRIVATE KEY-----`)
	jwtToken               = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{8,}\b`)
	awsAccessKey           = regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`)
	gcpAPIKey              = regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)
	base64Like             = regexp.MustCompile(`[A-Za-z0-9+/_=-]{32,}`)
	endpointSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,34}$`)
	eventNamePattern       = regexp.MustCompile(`^EVT_[A-Za-z0-9_]+$`)
	numericKeyPattern      = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
)

var knownMetadataShapes = map[string]byte{
	"country_codes_list.json": '[', "event_defs.json": '{', "geo_ip_country_codes_list.json": '{',
	"legacy_endpoint_segments.json": '[', "radio_specification.json": '{', "sensitive_metadata.json": '{',
	"ssl-inspection-file-extension.json": '{', "timezones.json": '{',
}

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
		if strings.HasPrefix(rel, "metadata/notices/") {
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
		value, err := decodeScanJSON(body)
		if err != nil {
			return fmt.Errorf("scan %s: %w", rel, err)
		}
		if strings.HasPrefix(rel, "metadata/") && !strings.HasPrefix(rel, "metadata/raw-fields/") {
			base := filepath.Base(rel)
			if base == "source.json" {
				return validateSourceMetadata(body)
			}
			shape, ok := knownMetadataShapes[base]
			if !ok {
				return fmt.Errorf("unexpected metadata file %s", rel)
			}
			if base == "sensitive_metadata.json" {
				_, err := ParseSensitiveMetadata(body)
				return err
			}
			if !matchesJSONShape(value, shape) {
				return fmt.Errorf("metadata %s has unexpected top-level structure", rel)
			}
			if err := validateKnownMetadata(base, value); err != nil {
				return fmt.Errorf("scan %s: %w", rel, err)
			}
			return nil
		}
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("field schema %s must be an object", rel)
		}
		return validateSchemaNode(object)
	})
}

func decodeScanJSON(body []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing JSON")
	}
	return value, nil
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

func validateSourceMetadata(body []byte) error {
	var source LocalManifest
	if err := decodeStrictJSON(body, &source); err != nil {
		return fmt.Errorf("source metadata structure: %w", err)
	}
	if source.InstallerURL != "" {
		parsed, err := url.Parse(source.InstallerURL)
		if err != nil {
			return fmt.Errorf("installer URL: %w", err)
		}
		if err := ValidateInstallerURL(parsed); err != nil {
			return fmt.Errorf("installer URL: %w", err)
		}
	}
	for label, digest := range map[string]string{"installer_sha256": source.InstallerSHA256, "schema_digest": source.SchemaDigest, "sensitivity_digest": source.SensitivityDigest, "notice_digest": source.NoticeDigest} {
		if digest != "" && !isLowerHexDigest(digest, 64) {
			return fmt.Errorf("source metadata %s is invalid", label)
		}
	}
	for name, digest := range source.Artifacts {
		if name == "" || !isLowerHexDigest(digest, 64) {
			return fmt.Errorf("source metadata artifact %q is invalid", name)
		}
	}
	return nil
}
func isLowerHexDigest(value string, n int) bool {
	if len(value) != n {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func validateKnownMetadata(name string, v any) error {
	switch name {
	case "timezones.json":
		return validateSingleStringRecordMap(v, "TZ")
	case "ssl-inspection-file-extension.json":
		return validateSingleStringRecordMap(v, "mime")
	case "country_codes_list.json":
		return validateCountryCodes(v)
	case "geo_ip_country_codes_list.json":
		object := v.(map[string]any)
		if len(object) != 1 {
			return errors.New("geo country metadata has unexpected fields")
		}
		countries, ok := object["countries"].([]any)
		if !ok {
			return errors.New("geo countries must be an array")
		}
		for _, country := range countries {
			code, ok := country.(string)
			if !ok || len(code) != 2 || strings.ToUpper(code) != code {
				return fmt.Errorf("invalid geo country code %v", country)
			}
		}
		return nil
	case "legacy_endpoint_segments.json":
		values := v.([]any)
		for _, value := range values {
			segment, ok := value.(string)
			if !ok || !endpointSegmentPattern.MatchString(segment) {
				return fmt.Errorf("invalid legacy endpoint segment %v", value)
			}
		}
		return nil
	case "event_defs.json":
		return validateEventDefinitions(v)
	case "radio_specification.json":
		return validateRadioSpecification(v)
	default:
		return fmt.Errorf("no validator for metadata %s", name)
	}
}

func validateSingleStringRecordMap(v any, field string) error {
	object := v.(map[string]any)
	for key, value := range object {
		if key == "" {
			return errors.New("metadata record key is empty")
		}
		record, ok := value.(map[string]any)
		if !ok || len(record) != 1 {
			return fmt.Errorf("metadata record %s has unexpected structure", key)
		}
		text, ok := record[field].(string)
		if !ok || text == "" {
			return fmt.Errorf("metadata record %s has invalid %s", key, field)
		}
		if err := validateConcreteString(text); err != nil {
			return err
		}
	}
	return nil
}
func validateCountryCodes(v any) error {
	records := v.([]any)
	for _, value := range records {
		record, ok := value.(map[string]any)
		if !ok {
			return errors.New("country entry must be an object")
		}
		for _, required := range []string{"name", "key", "code"} {
			text, ok := record[required].(string)
			if !ok || text == "" {
				return fmt.Errorf("country %s is invalid", required)
			}
			if err := validateConcreteString(text); err != nil {
				return err
			}
		}
		for key, field := range record {
			switch {
			case key == "name" || key == "key" || key == "code":
			case key == "hints":
				if err := validateTypedArray(field, "string"); err != nil {
					return fmt.Errorf("country hints: %w", err)
				}
			case strings.HasPrefix(key, "channels_"):
				if err := validateTypedArray(field, "number"); err != nil {
					return fmt.Errorf("country %s: %w", key, err)
				}
			case key == "afc":
				afc, ok := field.(map[string]any)
				if !ok {
					return errors.New("country afc must be object")
				}
				for afcKey, channels := range afc {
					if !strings.HasPrefix(afcKey, "channels_6e") {
						return fmt.Errorf("unexpected afc field %s", afcKey)
					}
					if err := validateTypedArray(channels, "number"); err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("unexpected country field %s", key)
			}
		}
	}
	return nil
}
func validateTypedArray(v any, kind string) error {
	values, ok := v.([]any)
	if !ok {
		return errors.New("expected array")
	}
	for _, value := range values {
		if kind == "string" {
			if text, ok := value.(string); !ok {
				return fmt.Errorf("expected string, got %T", value)
			} else if err := validateConcreteString(text); err != nil {
				return err
			}
		} else if _, ok := value.(json.Number); !ok {
			return fmt.Errorf("expected number, got %T", value)
		}
	}
	return nil
}
func validateEventDefinitions(v any) error {
	events := v.(map[string]any)
	allowed := map[string]string{"subsystem": "string", "alert_repeat": "bool", "alert_sendmail": "bool", "alert_subject": "string", "key": "string", "event_enabled": "bool", "msg": "string", "is_alert": "bool", "is_negative": "bool"}
	for name, value := range events {
		if !eventNamePattern.MatchString(name) {
			return fmt.Errorf("invalid event name %s", name)
		}
		record, ok := value.(map[string]any)
		if !ok || len(record) != len(allowed) {
			return fmt.Errorf("event %s has unexpected structure", name)
		}
		for field, kind := range allowed {
			leaf, ok := record[field]
			if !ok {
				return fmt.Errorf("event %s missing %s", name, field)
			}
			if kind == "string" {
				text, ok := leaf.(string)
				if !ok {
					return fmt.Errorf("event %s field %s must be string", name, field)
				}
				if err := validateConcreteString(text); err != nil {
					return err
				}
			} else if _, ok := leaf.(bool); !ok {
				return fmt.Errorf("event %s field %s must be boolean", name, field)
			}
		}
	}
	return nil
}
func validateRadioSpecification(v any) error {
	bands := v.(map[string]any)
	for band, widthValue := range bands {
		if band != "ad" && band != "ng" && band != "na" && band != "6e" {
			return fmt.Errorf("unexpected radio band %s", band)
		}
		widths, ok := widthValue.(map[string]any)
		if !ok {
			return errors.New("radio widths must be objects")
		}
		for width, channelValue := range widths {
			if !numericKeyPattern.MatchString(width) {
				return fmt.Errorf("invalid radio width %s", width)
			}
			channels, ok := channelValue.(map[string]any)
			if !ok {
				return errors.New("radio channels must be objects")
			}
			for channel, recordValue := range channels {
				if !numericKeyPattern.MatchString(channel) {
					return fmt.Errorf("invalid radio channel %s", channel)
				}
				record, ok := recordValue.(map[string]any)
				if !ok {
					return errors.New("radio channel record must be object")
				}
				for _, field := range []string{"lowerFrequency", "centerFrequency", "upperFrequency"} {
					if _, ok := record[field].(json.Number); !ok {
						return fmt.Errorf("radio channel %s missing numeric %s", channel, field)
					}
				}
				if err := validateTypedArray(record["subChannels"], "number"); err != nil {
					return err
				}
				if len(record) > 5 {
					return errors.New("radio channel has unexpected fields")
				}
				if label, exists := record["band"]; exists {
					if _, ok := label.(string); !ok {
						return errors.New("radio band label must be string")
					}
				}
			}
		}
	}
	return nil
}

func validateConcreteString(value string) error { return rejectHighEntropy(value) }
func rejectHighEntropy(value string) error {
	for _, token := range base64Like.FindAllString(value, -1) {
		upper, lower, encodedMarker := false, false, false
		for _, r := range token {
			upper = upper || r >= 'A' && r <= 'Z'
			lower = lower || r >= 'a' && r <= 'z'
			encodedMarker = encodedMarker || r >= '0' && r <= '9' || strings.ContainsRune("=+/", r)
		}
		if upper && lower && encodedMarker && shannonEntropy(token) >= 4 {
			return errors.New("contains high-entropy base64-like value")
		}
	}
	return nil
}
func shannonEntropy(value string) float64 {
	counts := map[rune]int{}
	runes := []rune(value)
	for _, r := range runes {
		counts[r]++
	}
	var result float64
	for _, count := range counts {
		p := float64(count) / float64(len(runes))
		result -= p * math.Log2(p)
	}
	return result
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
		if err := rejectHighEntropy(x); err != nil {
			return err
		}
		if !schemaString(x) {
			return fmt.Errorf("unexpected concrete scalar %q", x)
		}
	default:
		return fmt.Errorf("schema leaves must be strings, got %T", v)
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
