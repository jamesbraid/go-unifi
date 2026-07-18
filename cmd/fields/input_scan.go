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
	"sort"
	"strings"
)

var (
	pemPrivateKey           = regexp.MustCompile(`-----BEGIN (?:[A-Z0-9 ]+ )?PRIVATE KEY-----`)
	jwtToken                = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{8,}\b`)
	awsAccessKey            = regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`)
	gcpAPIKey               = regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)
	base64Like              = regexp.MustCompile(`[A-Za-z0-9+/_=-]{32,}`)
	endpointSegmentPattern  = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,34}$`)
	eventNamePattern        = regexp.MustCompile(`^EVT_[A-Za-z0-9_]+$`)
	numericKeyPattern       = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
	sensitiveNamePattern    = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,256}$`)
	countryKeyPattern       = regexp.MustCompile(`^[A-Z]{2}$`)
	decimalStringPattern    = regexp.MustCompile(`^[0-9]+$`)
	subsystemPattern        = regexp.MustCompile(`^[a-z0-9_-]*$`)
	radioBandPattern        = regexp.MustCompile(`^unii[1-8](?:ext)?$`)
	mimePattern             = regexp.MustCompile(`^[A-Za-z0-9!#$&^_.+-]+/[A-Za-z0-9!#$&^_.+*-]+$`)
	posixTZPattern          = regexp.MustCompile(`^[A-Za-z0-9<>,.+:/_-]{1,256}$`)
	lowerHexOpaquePattern   = regexp.MustCompile(`[0-9a-f]{32,}`)
	alphabeticOpaquePattern = regexp.MustCompile(`[A-Za-z]{32,}`)
	versionPattern          = regexp.MustCompile(`^v?[0-9]+(?:\.[0-9]+){1,3}(?:[-+][A-Za-z0-9.-]+)?$`)
	uuidPattern             = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	policyVersionPattern    = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*$`)
	artifactNamePattern     = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,512}$`)
	timezoneKeyPattern      = regexp.MustCompile(`^(?:UTC|[A-Za-z0-9._+-]+(?:/[A-Za-z0-9._+-]+)+)$`)
	extensionKeyPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,31}$`)
	countryChannelPattern   = regexp.MustCompile(`^channels_(?:ng|na|ad|6e)(?:_(?:outdoor|indoor|dfs|40|80|160|240|320|4320|1080|2160|psc|ext_(?:outdoor|1080|2160)))?$`)
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
				metadata, err := ParseSensitiveMetadata(body)
				if err != nil {
					return err
				}
				return validateSensitiveMetadataStrings(metadata)
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
		if err := validateSchemaNode(object); err != nil {
			return fmt.Errorf("scan %s: %w", rel, err)
		}
		return nil
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
	if source.OSVersion != "" && !versionPattern.MatchString(source.OSVersion) {
		return errors.New("source metadata OS version is invalid")
	}
	if !versionPattern.MatchString(source.NetworkVersion) {
		return errors.New("source metadata Network version is invalid")
	}
	if source.FirmwareID != "" && (!uuidPattern.MatchString(source.FirmwareID) || rejectHighEntropy(source.FirmwareID) != nil) {
		return errors.New("source metadata firmware ID is invalid")
	}
	if source.Product != "" && source.Product != "unifi-os-server" && source.Product != "unifi-controller" {
		return errors.New("source metadata product is invalid")
	}
	if source.Platform != "" && source.Platform != "linux-x64" && source.Platform != "debian" {
		return errors.New("source metadata platform is invalid")
	}
	if source.Channel != "" && source.Channel != "release" {
		return errors.New("source metadata channel is invalid")
	}
	if !policyVersionPattern.MatchString(source.PolicyVersion) {
		return errors.New("source metadata policy version is invalid")
	}
	if source.InstallerMD5 != "" && !isLowerHexDigest(source.InstallerMD5, 32) {
		return errors.New("source metadata installer MD5 is invalid")
	}
	for label, digest := range map[string]string{"installer_sha256": source.InstallerSHA256, "schema_digest": source.SchemaDigest, "sensitivity_digest": source.SensitivityDigest, "notice_digest": source.NoticeDigest} {
		if digest != "" && !isLowerHexDigest(digest, 64) {
			return fmt.Errorf("source metadata %s is invalid", label)
		}
	}
	artifactNames := make([]string, 0, len(source.Artifacts))
	for name := range source.Artifacts {
		artifactNames = append(artifactNames, name)
	}
	sort.Strings(artifactNames)
	artifactNamesByFold := make(map[string]string, len(artifactNames))
	for _, name := range artifactNames {
		digest := source.Artifacts[name]
		folded := strings.ToLower(name)
		if previous, exists := artifactNamesByFold[folded]; exists {
			return fmt.Errorf("source metadata artifact case ambiguity between %q and %q", previous, name)
		}
		artifactNamesByFold[folded] = name
		if validateSourceArtifactName(name) != nil || !isLowerHexDigest(digest, 64) {
			return fmt.Errorf("source metadata artifact %q is invalid", name)
		}
	}
	allowedMissing := map[string]bool{}
	for _, name := range metadataAllowlist {
		if name != "sensitive_metadata.json" {
			allowedMissing[name] = true
		}
	}
	for _, name := range source.MissingOptional {
		if !allowedMissing[name] || rejectHighEntropy(name) != nil {
			return fmt.Errorf("source metadata missing optional %q is invalid", name)
		}
	}
	return nil
}

func validateSourceArtifactName(name string) error {
	if !artifactNamePattern.MatchString(name) {
		return errors.New("artifact path contains unsupported characters or length")
	}
	if err := validateDigestPath(name); err != nil {
		return err
	}
	parts := strings.Split(name, "/")
	for _, component := range parts {
		if len(component) == 0 || len(component) > 255 {
			return errors.New("artifact path component length is invalid")
		}
	}
	if len(parts) == 3 && parts[0] == "api" && parts[1] == "fields" && strings.HasSuffix(parts[2], ".json") && len(strings.TrimSuffix(parts[2], ".json")) > 0 {
		return nil
	}
	if len(parts) == 1 {
		for _, allowed := range metadataAllowlist {
			if name == allowed {
				return nil
			}
		}
		return errors.New("artifact is not approved root metadata")
	}
	var noticePath string
	switch {
	case parts[0] == "internal-dependencies.jar" && len(parts) >= 2:
		noticePath = strings.Join(parts[1:], "/")
	case parts[0] == "ace.jar" && len(parts) >= 2:
		if len(parts) >= 5 && parts[1] == "BOOT-INF" && parts[2] == "lib" && strings.EqualFold(filepath.Ext(parts[3]), ".jar") {
			noticePath = strings.Join(parts[4:], "/")
		} else {
			noticePath = strings.Join(parts[1:], "/")
		}
	default:
		return errors.New("artifact namespace is not recognized")
	}
	if !isReviewedNoticePath(noticePath) {
		return errors.New("artifact notice path is not reviewed")
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
			if !ok || !endpointSegmentPattern.MatchString(segment) || rejectHighEntropy(segment) != nil {
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
		if rejectHighEntropy(key) != nil {
			return fmt.Errorf("metadata record key %s is opaque", key)
		}
		record, ok := value.(map[string]any)
		if !ok || len(record) != 1 {
			return fmt.Errorf("metadata record %s has unexpected structure", key)
		}
		text, ok := record[field].(string)
		if !ok || text == "" {
			return fmt.Errorf("metadata record %s has invalid %s", key, field)
		}
		if field == "TZ" {
			if !timezoneKeyPattern.MatchString(key) || !posixTZPattern.MatchString(text) || rejectHighEntropy(text) != nil {
				return fmt.Errorf("metadata record %s has invalid POSIX timezone", key)
			}
		} else if !extensionKeyPattern.MatchString(key) || len(text) > 256 || !mimePattern.MatchString(text) || rejectHighEntropy(text) != nil {
			return fmt.Errorf("metadata record %s has invalid MIME", key)
		}
	}
	return nil
}

func validateSensitiveMetadataStrings(metadata SensitiveMetadata) error {
	for _, name := range metadata.DefaultNames {
		if name == "" || len(name) > 128 || !isBoundedHumanText(name) || rejectHighEntropy(name) != nil {
			return errors.New("invalid sensitive default name")
		}
	}
	for _, name := range metadata.SystemProperties {
		if !sensitiveNamePattern.MatchString(name) || rejectHighEntropy(name) != nil {
			return errors.New("invalid sensitive system property")
		}
	}
	for collection, paths := range metadata.DBFields {
		if !sensitiveNamePattern.MatchString(collection) || rejectHighEntropy(collection) != nil {
			return errors.New("invalid sensitive collection")
		}
		for _, fieldPath := range paths {
			if !sensitiveNamePattern.MatchString(fieldPath) || rejectHighEntropy(fieldPath) != nil {
				return errors.New("invalid sensitive field path")
			}
		}
	}
	for collection, fieldPath := range metadata.DistinctDBFields {
		if !sensitiveNamePattern.MatchString(collection) || rejectHighEntropy(collection) != nil {
			return errors.New("invalid distinct sensitive collection")
		}
		if !sensitiveNamePattern.MatchString(fieldPath) || rejectHighEntropy(fieldPath) != nil {
			return errors.New("invalid distinct sensitive field path")
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
		if !countryKeyPattern.MatchString(record["key"].(string)) {
			return errors.New("country key must be two uppercase letters")
		}
		if !decimalStringPattern.MatchString(record["code"].(string)) {
			return errors.New("country code must be decimal")
		}
		for key, field := range record {
			switch {
			case key == "name" || key == "key" || key == "code":
			case key == "hints":
				if err := validateTypedArray(field, "string"); err != nil {
					return fmt.Errorf("country hints: %w", err)
				}
			case countryChannelPattern.MatchString(key):
				if err := validateTypedArray(field, "number"); err != nil {
					return fmt.Errorf("country %s: %w", key, err)
				}
			case key == "afc":
				afc, ok := field.(map[string]any)
				if !ok {
					return errors.New("country afc must be object")
				}
				for afcKey, channels := range afc {
					if !map[string]bool{"channels_6e": true, "channels_6e_40": true, "channels_6e_80": true, "channels_6e_160": true, "channels_6e_320": true}[afcKey] {
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
		if !eventNamePattern.MatchString(name) || rejectNaturalTextOpaque(name) != nil {
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
				switch field {
				case "key":
					if text != name {
						return fmt.Errorf("event %s key does not match", name)
					}
				case "subsystem":
					if len(text) > 32 || !subsystemPattern.MatchString(text) {
						return fmt.Errorf("event %s subsystem invalid", name)
					}
				default:
					if len(text) > 2048 || !isBoundedHumanText(text) || rejectNaturalTextOpaque(text) != nil {
						return fmt.Errorf("event %s field %s invalid", name, field)
					}
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
				allowed := map[string]bool{"lowerFrequency": true, "centerFrequency": true, "upperFrequency": true, "subChannels": true, "band": true}
				for key := range record {
					if !allowed[key] {
						return fmt.Errorf("radio channel has unexpected field %s", key)
					}
				}
				if label, exists := record["band"]; exists {
					text, ok := label.(string)
					if !ok || !radioBandPattern.MatchString(text) || rejectHighEntropy(text) != nil {
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
	if lowerHexOpaquePattern.MatchString(value) {
		return errors.New("contains high-entropy lowercase hexadecimal value")
	}
	for _, token := range alphabeticOpaquePattern.FindAllString(value, -1) {
		if shannonEntropy(token) >= 3.75 {
			return errors.New("contains high-entropy alphabetic value")
		}
	}
	for _, token := range base64Like.FindAllString(value, -1) {
		upper, lower, encodedMarker := false, false, false
		for _, r := range token {
			upper = upper || r >= 'A' && r <= 'Z'
			lower = lower || r >= 'a' && r <= 'z'
			encodedMarker = encodedMarker || r >= '0' && r <= '9' || strings.ContainsRune("=+/", r)
		}
		letters := strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
				return r
			}
			return -1
		}, token)
		if upper && lower && shannonEntropy(token) >= 4 && (encodedMarker || opaqueAlphabeticRun(letters)) {
			return errors.New("contains high-entropy base64-like value")
		}
	}
	return nil
}

func rejectNaturalTextOpaque(value string) error {
	if lowerHexOpaquePattern.MatchString(value) {
		return errors.New("contains high-entropy lowercase hexadecimal value")
	}
	for _, token := range base64Like.FindAllString(value, -1) {
		encoded := strings.ContainsAny(token, "=+/")
		letters := strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
				return r
			}
			return -1
		}, token)
		if shannonEntropy(token) >= 4 && (encoded || opaqueAlphabeticRun(letters)) {
			return errors.New("contains opaque token")
		}
	}
	for _, token := range alphabeticOpaquePattern.FindAllString(value, -1) {
		if shannonEntropy(token) >= 3.75 && opaqueAlphabeticRun(token) {
			return errors.New("contains opaque alphabetic token")
		}
	}
	return nil
}

func opaqueAlphabeticRun(token string) bool {
	runes := []rune(token)
	upper, lower, transitions := false, false, 0
	wasUpper := false
	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'
		upper = upper || isUpper
		lower = lower || r >= 'a' && r <= 'z'
		if i > 0 && isUpper != wasUpper {
			transitions++
		}
		wasUpper = isUpper
	}
	if !upper || !lower {
		return true
	}
	return float64(transitions)/float64(len(runes)-1) > .45
}

func isBoundedHumanText(value string) bool {
	for _, r := range value {
		if r < 0x20 && r != '\t' && r != '\n' {
			return false
		}
	}
	return true
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
	return validateSchemaNodeAt(v, "")
}

func validateSchemaNodeAt(v any, pointer string) error {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for key := range x {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := validateSchemaNodeAt(x[key], appendJSONPointer(pointer, key)); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range x {
			if err := validateSchemaNodeAt(child, fmt.Sprintf("%s/%d", pointer, i)); err != nil {
				return err
			}
		}
	case string:
		if err := rejectHighEntropy(x); err != nil {
			return fmt.Errorf("schema path %s: %w", displayJSONPointer(pointer), err)
		}
		if !schemaString(x) {
			return fmt.Errorf("schema path %s: unexpected concrete scalar %q", displayJSONPointer(pointer), x)
		}
	default:
		return fmt.Errorf("schema path %s: leaves must be strings, got %T", displayJSONPointer(pointer), v)
	}
	return nil
}

func appendJSONPointer(pointer, key string) string {
	key = strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
	return pointer + "/" + key
}

func displayJSONPointer(pointer string) string {
	if pointer == "" {
		return "/"
	}
	return pointer
}

func schemaString(value string) bool {
	if value == "" || value == "true|false" {
		return true
	}
	switch value {
	case "string", "number", "integer", "boolean", "object", "array", "null", "false":
		return true
	}
	if reviewedSingletonEnum(value) {
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

func reviewedSingletonEnum(value string) bool {
	switch value {
	case "static-route", "switch", "upgrade":
		return true
	default:
		return false
	}
}
