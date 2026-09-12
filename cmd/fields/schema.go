package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-codegen-spec/code"
	"github.com/hashicorp/terraform-plugin-codegen-spec/datasource"
	"github.com/hashicorp/terraform-plugin-codegen-spec/provider"
	"github.com/hashicorp/terraform-plugin-codegen-spec/resource"
	"github.com/hashicorp/terraform-plugin-codegen-spec/schema"
	"github.com/hashicorp/terraform-plugin-codegen-spec/spec"
	"github.com/iancoleman/strcase"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

const SpecVersion = "0.1"

// SpecificationGenerator generates a Terraform provider specification from resources.
type SpecificationGenerator struct {
	ProviderName string
	Resources    []*ResourceInfo
	Sensitive    sensitiveIndex
}

// NewSpecificationGenerator creates a new specification generator.
func NewSpecificationGenerator(providerName string, sensitive sensitiveIndex) *SpecificationGenerator {
	return &SpecificationGenerator{
		ProviderName: providerName,
		Resources:    make([]*ResourceInfo, 0),
		Sensitive:    sensitive,
	}
}

// sensitiveIndex maps a controller collection name (lowercased schema file
// base name, e.g. "wlanconf") to the set of wire field leaf names UniFi
// lists in sensitive_metadata.json.
type sensitiveIndex map[string]map[string]bool

// loadSensitiveMetadata builds a sensitiveIndex from the controller's
// sensitive_metadata.json. A missing file yields a nil index (only the x_
// prefix rule applies then).
func loadSensitiveMetadata(path string) (sensitiveIndex, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Values are usually lists of field names, but single-field entries ship
	// as a bare string (e.g. "rogue": "essid" in the distinct section).
	var meta struct {
		ByCollection         map[string]any `json:"sensitive_db_fields_by_collection"`
		DistinctByCollection map[string]any `json:"sensitive_distinct_db_fields_by_collection"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, fmt.Errorf("unable to parse sensitive metadata: %w", err)
	}

	index := make(sensitiveIndex)
	addLeaf := func(collection string, field string) {
		leaves := index[collection]
		if leaves == nil {
			leaves = make(map[string]bool)
			index[collection] = leaves
		}
		// Nested entries are dotted paths (auth_servers.x_secret);
		// FieldInfo carries leaf wire names, so index the leaf.
		parts := strings.Split(field, ".")
		leaves[parts[len(parts)-1]] = true
	}

	for _, byCollection := range []map[string]any{meta.ByCollection, meta.DistinctByCollection} {
		for collection, value := range byCollection {
			switch entry := value.(type) {
			case string:
				addLeaf(collection, entry)
			case []any:
				for _, field := range entry {
					name, ok := field.(string)
					if !ok {
						return nil, fmt.Errorf("unexpected sensitive metadata entry %v for %s", field, collection)
					}
					addLeaf(collection, name)
				}
			default:
				return nil, fmt.Errorf("unexpected sensitive metadata shape %T for %s", value, collection)
			}
		}
	}

	return index, nil
}

// secretNameRe separates secret material from the anonymization-only entries
// in sensitive_metadata.json (name, hostname, serial, usernames,
// certificates, ...). ipsec_key_exchange is a protocol setting, which is why
// the key match is suffix-anchored.
var secretNameRe = regexp.MustCompile(`(?i)passw|passphrase|secret|token|psk|sim_pin|private_key|auth_?key|_key$`)

// AddResource adds a resource to the specification generator.
func (g *SpecificationGenerator) AddResource(r *ResourceInfo) {
	g.Resources = append(g.Resources, r)
}

// Generate creates the Terraform provider specification.
func (g *SpecificationGenerator) Generate() *spec.Specification {
	spec := &spec.Specification{
		Version: SpecVersion,
		Provider: &provider.Provider{
			Name: g.ProviderName,
			Schema: &provider.Schema{
				Attributes: g.generateProviderAttributes(),
			},
		},
		DataSources: make([]datasource.DataSource, 0),
		Resources:   make([]resource.Resource, 0),
	}

	// Sort resources by name for consistent output
	sortedResources := slices.SortedFunc(slices.Values(g.Resources), func(a, b *ResourceInfo) int {
		return strings.Compare(a.StructName, b.StructName)
	})

	for _, r := range sortedResources {
		// Skip settings for now - they have a different pattern
		if r.IsSetting() {
			continue
		}

		name := toTerraformName(r.StructName)
		attrs := g.generateResourceAttributes(r)
		spec.DataSources = append(spec.DataSources, datasource.DataSource{
			Name:   name,
			Schema: &datasource.Schema{Attributes: dataSourceAttributes(attrs)},
		})
		spec.Resources = append(spec.Resources, resource.Resource{
			Name:   name,
			Schema: &resource.Schema{Attributes: attrs},
		})
	}

	return spec
}

// generateProviderAttributes creates the provider configuration attributes.
func (g *SpecificationGenerator) generateProviderAttributes() []provider.Attribute {
	return []provider.Attribute{
		{
			Name: "username",
			String: &provider.StringAttribute{
				OptionalRequired: "optional",
				Description:      ptr("Username for UniFi controller authentication"),
			},
		},
		{
			Name: "password",
			String: &provider.StringAttribute{
				OptionalRequired: "optional",
				Sensitive:        ptr(true),
				Description:      ptr("Password for UniFi controller authentication"),
			},
		},
		{
			Name: "api_url",
			String: &provider.StringAttribute{
				OptionalRequired: "optional",
				Description:      ptr("URL of the UniFi controller API"),
			},
		},
		{
			Name: "api_key",
			String: &provider.StringAttribute{
				OptionalRequired: "optional",
				Description:      ptr("API key for the Unifi controller. Can be specified with the `UNIFI_API_KEY` environment variable"),
				Sensitive:        ptr(true),
			},
		},
		{
			Name: "site",
			String: &provider.StringAttribute{
				OptionalRequired: "optional",
				Description:      ptr("Site name for the UniFi controller"),
			},
		},
		{
			Name: "allow_insecure",
			Bool: &provider.BoolAttribute{
				OptionalRequired: "optional",
				Description:      ptr("Allow insecure HTTPS connections to the UniFi controller"),
			},
		},
	}
}

// sensitivePtr marks secret-bearing attributes. Two rules, union:
//
//  1. UniFi's own convention: an x_ prefix on secret wire fields
//     (x_passphrase, x_auth_key, ...).
//  2. Fields the controller's sensitive_metadata.json lists for this
//     resource's collection, filtered to secret-looking names — the metadata
//     is a support-file anonymization list, so it also names fields (name,
//     hostname, wan_username, certificates) that must stay visible in
//     Terraform plans. The intersection currently adds lte_password,
//     lte_sim_pin, and secret_verifier_encoded, and catches future secrets
//     that ship without the x_ prefix.
func (g *SpecificationGenerator) sensitivePtr(r *ResourceInfo, field *FieldInfo) *bool {
	if strings.HasPrefix(field.JSONName, "x_") {
		return ptr(true)
	}
	if r != nil && g.Sensitive[r.Collection][field.JSONName] && secretNameRe.MatchString(field.JSONName) {
		return ptr(true)
	}
	return nil
}

// dataSourceAttributes derives a resource's data-source schema from its
// resource schema. The two were maintained as parallel walkers that differed
// in exactly two ways, both stated here once: every data-source attribute is
// computed (a data source cannot write), and none carries a description (the
// descriptions warn about write-time ownership traps, which cannot bite a
// reader -- see describePreference).
func dataSourceAttributes(attrs []resource.Attribute) []datasource.Attribute {
	if attrs == nil {
		return nil
	}
	out := make([]datasource.Attribute, 0, len(attrs))
	for _, a := range attrs {
		d := datasource.Attribute{Name: a.Name}
		switch {
		case a.Bool != nil:
			d.Bool = &datasource.BoolAttribute{
				ComputedOptionalRequired: schema.Computed,
				Sensitive:                a.Bool.Sensitive,
			}
		case a.Int64 != nil:
			d.Int64 = &datasource.Int64Attribute{
				ComputedOptionalRequired: schema.Computed,
				Sensitive:                a.Int64.Sensitive,
				Validators:               a.Int64.Validators,
			}
		case a.Float64 != nil:
			d.Float64 = &datasource.Float64Attribute{
				ComputedOptionalRequired: schema.Computed,
				Sensitive:                a.Float64.Sensitive,
			}
		case a.String != nil:
			d.String = &datasource.StringAttribute{
				ComputedOptionalRequired: schema.Computed,
				Sensitive:                a.String.Sensitive,
				Validators:               a.String.Validators,
			}
		case a.List != nil:
			d.List = &datasource.ListAttribute{
				ComputedOptionalRequired: schema.Computed,
				ElementType:              a.List.ElementType,
				Sensitive:                a.List.Sensitive,
			}
		case a.ListNested != nil:
			d.ListNested = &datasource.ListNestedAttribute{
				ComputedOptionalRequired: schema.Computed,
				NestedObject: datasource.NestedAttributeObject{
					Attributes: dataSourceAttributes(a.ListNested.NestedObject.Attributes),
				},
			}
		case a.SingleNested != nil:
			d.SingleNested = &datasource.SingleNestedAttribute{
				ComputedOptionalRequired: schema.Computed,
				Attributes:               dataSourceAttributes(a.SingleNested.Attributes),
			}
		}
		out = append(out, d)
	}
	return out
}

// generateResourceAttributes generates resource attributes from a Resource.
func (g *SpecificationGenerator) generateResourceAttributes(r *ResourceInfo) []resource.Attribute {
	baseType := r.Types[r.StructName]
	if baseType == nil || baseType.Fields == nil {
		return nil
	}

	attrs := make([]resource.Attribute, 0)

	// Sort fields by name for consistent output
	fieldNames := slices.Sorted(maps.Keys(baseType.Fields))

	for _, fieldName := range fieldNames {
		field := baseType.Fields[fieldName]
		if field == nil || strings.HasPrefix(fieldName, " ") || strings.HasSuffix(fieldName, "_Spacer") {
			continue
		}

		attr := g.fieldToResourceAttribute(r, "", field)
		if attr != nil {
			attrs = append(attrs, *attr)
		}
	}

	return attrs
}

// fieldToResourceAttribute converts a FieldInfo to a ResourceAttribute.
func (g *SpecificationGenerator) fieldToResourceAttribute(r *ResourceInfo, container string, field *FieldInfo) *resource.Attribute {
	attr := g.buildResourceAttribute(r, container, field)
	describePreference(r, container, field, attr)
	return attr
}

func (g *SpecificationGenerator) buildResourceAttribute(r *ResourceInfo, container string, field *FieldInfo) *resource.Attribute {
	if field == nil {
		return nil
	}

	// The wire name is already snake_case and is what the API actually
	// calls the field. Deriving the attribute name from the Go field name
	// instead produced names no API user would recognise --
	// open_vpn_encryption_cipher for openvpn_encryption_cipher, and
	// l_2_tp_allow_weak_ciphers for l2tp_allow_weak_ciphers.
	name := field.JSONName
	computedOptionalRequired := g.determineComputedOptionalRequired(field)

	attr := &resource.Attribute{
		Name: name,
	}

	// Handle array types
	if field.IsArray {
		if field.Fields != nil {
			// Nested object array - use list_nested
			nestedAttrs := g.generateNestedResourceAttributes(r, joinContainer(container, field.JSONName), field)
			attr.ListNested = &resource.ListNestedAttribute{
				ComputedOptionalRequired: computedOptionalRequired,
				NestedObject: resource.NestedAttributeObject{
					Attributes: nestedAttrs,
				},
			}
		} else {
			// Simple array - use list
			attr.List = &resource.ListAttribute{
				ComputedOptionalRequired: computedOptionalRequired,
				ElementType:              g.fieldTypeToElementType(field.FieldType),
				Sensitive:                g.sensitivePtr(r, field),
			}
		}
		return attr
	}

	// Handle nested object types
	if field.Fields != nil {
		nestedAttrs := g.generateNestedResourceAttributes(r, joinContainer(container, field.JSONName), field)
		attr.SingleNested = &resource.SingleNestedAttribute{
			ComputedOptionalRequired: computedOptionalRequired,
			Attributes:               nestedAttrs,
		}
		return attr
	}

	// Handle primitive types
	switch field.FieldType {
	case "bool":
		attr.Bool = &resource.BoolAttribute{
			ComputedOptionalRequired: computedOptionalRequired,
			Sensitive:                g.sensitivePtr(r, field),
		}
	case fields.Int:
		intAttr := &resource.Int64Attribute{
			ComputedOptionalRequired: computedOptionalRequired,
			Sensitive:                g.sensitivePtr(r, field),
		}
		if validators := g.buildInt64Validators(field.FieldValidation); len(validators) > 0 {
			intAttr.Validators = validators
		}
		attr.Int64 = intAttr
	case "float64":
		attr.Float64 = &resource.Float64Attribute{
			ComputedOptionalRequired: computedOptionalRequired,
			Sensitive:                g.sensitivePtr(r, field),
		}
	case "string":
		strAttr := &resource.StringAttribute{
			ComputedOptionalRequired: computedOptionalRequired,
			Sensitive:                g.sensitivePtr(r, field),
		}
		if validators := g.buildStringValidators(field.FieldValidation); len(validators) > 0 {
			strAttr.Validators = validators
		}
		attr.String = strAttr
	default:
		// Check if it's a custom type defined in Types
		if typeInfo, ok := r.Types[field.FieldType]; ok {
			attr.SingleNested = &resource.SingleNestedAttribute{
				ComputedOptionalRequired: computedOptionalRequired,
				Attributes:               g.generateNestedResourceAttributes(r, joinContainer(container, field.JSONName), typeInfo),
			}
		} else {
			// Default to string for unknown types
			attr.String = &resource.StringAttribute{
				ComputedOptionalRequired: computedOptionalRequired,
				Sensitive:                g.sensitivePtr(r, field),
			}
		}
	}

	return attr
}

// generateNestedResourceAttributes generates nested attributes for resources.
func (g *SpecificationGenerator) generateNestedResourceAttributes(r *ResourceInfo, container string, field *FieldInfo) []resource.Attribute {
	if field.Fields == nil {
		return nil
	}

	attrs := make([]resource.Attribute, 0)
	fieldNames := slices.Sorted(maps.Keys(field.Fields))

	for _, fieldName := range fieldNames {
		childField := field.Fields[fieldName]
		if childField == nil {
			continue
		}

		attr := g.fieldToResourceAttribute(r, container, childField)
		if attr != nil {
			attrs = append(attrs, *attr)
		}
	}

	return attrs
}

// determineComputedOptionalRequired determines the computed_optional_required value for a field.
func (g *SpecificationGenerator) determineComputedOptionalRequired(field *FieldInfo) schema.ComputedOptionalRequired {
	// ID and SiteID are computed
	if field.FieldName == "ID" || field.FieldName == "SiteID" {
		return schema.Computed
	}

	// Hidden attributes are computed
	if field.FieldName == "Hidden" || field.FieldName == "HiddenID" ||
		field.FieldName == "NoDelete" || field.FieldName == "NoEdit" {
		return schema.Computed
	}

	// If OmitEmpty is true, the field is optional
	if field.OmitEmpty {
		return schema.ComputedOptional
	}

	return schema.Optional
}

// Validators for the Terraform code specification, derived from the same
// controller patterns the SDK exports. Nothing here transcribes a rule by
// hand: enums.go and ranges.go decide what a pattern means, and refuse
// anything they cannot read confidently, so a field either gets a validator
// that matches its schema or gets none.

// buildStringValidators turns a string field's validator into the code-spec
// form: an enumeration becomes OneOf, a bare length rule becomes
// LengthBetween, and anything else is handed through as the regex it is.
func (g *SpecificationGenerator) buildStringValidators(validation string) []schema.StringValidator {
	if strings.TrimSpace(validation) == "" {
		return nil
	}

	if values := enumValues(validation); values != nil {
		quoted := make([]string, len(values))
		for i, v := range values {
			quoted[i] = strconv.Quote(v)
		}
		return []schema.StringValidator{customStringValidator(
			fmt.Sprintf("stringvalidator.OneOf(%s)", strings.Join(quoted, ", ")),
			stringValidatorImport,
		)}
	}

	if low, high, ok := lengthBounds(validation); ok {
		return []schema.StringValidator{customStringValidator(
			fmt.Sprintf("stringvalidator.LengthBetween(%d, %d)", low, high),
			stringValidatorImport,
		)}
	}

	// Not something with a shorter name: keep the controller's own rule.
	// It has to compile under RE2, which a few lookahead patterns do not.
	if _, err := compileAnchored(validation); err != nil {
		return nil
	}
	return []schema.StringValidator{customStringValidator(
		fmt.Sprintf("stringvalidator.RegexMatches(regexp.MustCompile(%s), %s)",
			strconv.Quote(anchoredPattern(validation)),
			strconv.Quote("must match the controller's validator: "+validation)),
		stringValidatorImport, regexpImport,
	)}
}

// buildInt64Validators turns a numeric field's validator into OneOf for a set
// of values or Between for a contiguous range. A pattern that is neither
// yields nothing rather than a guess.
func (g *SpecificationGenerator) buildInt64Validators(validation string) []schema.Int64Validator {
	if strings.TrimSpace(validation) == "" {
		return nil
	}

	if values := enumInt64Values(validation); values != nil {
		parts := make([]string, len(values))
		for i, v := range values {
			parts[i] = strconv.FormatInt(v, 10)
		}
		return []schema.Int64Validator{{
			Custom: &schema.CustomValidator{
				Imports:          []code.Import{int64ValidatorImport},
				SchemaDefinition: fmt.Sprintf("int64validator.OneOf(%s)", strings.Join(parts, ", ")),
			},
		}}
	}

	if low, high, ok := numericRange(validation); ok {
		return []schema.Int64Validator{{
			Custom: &schema.CustomValidator{
				Imports:          []code.Import{int64ValidatorImport},
				SchemaDefinition: fmt.Sprintf("int64validator.Between(%d, %d)", low, high),
			},
		}}
	}

	return nil
}

// There is deliberately no buildFloat64Validators. Every float64 field the
// schema constrains is a map coordinate (x, y, z) whose pattern --
// (^([-]?[\d]+)$)|(^([-]?[\d]+[.]?[\d]+)$) -- says "an optionally signed
// integer or decimal", which is what a float64 already is. A validator built
// from it would be a tautology. TestNoConstrainableFloat64Fields fails if a
// schema refresh ever introduces one worth expressing.

var (
	stringValidatorImport = code.Import{Path: "github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"}
	int64ValidatorImport  = code.Import{Path: "github.com/hashicorp/terraform-plugin-framework-validators/int64validator"}
	regexpImport          = code.Import{Path: "regexp"}
)

func customStringValidator(definition string, imports ...code.Import) schema.StringValidator {
	return schema.StringValidator{
		Custom: &schema.CustomValidator{
			Imports:          imports,
			SchemaDefinition: definition,
		},
	}
}

// anchoredPattern makes a validator match the whole value. Several are
// written unanchored, and RegexMatches on an unanchored pattern accepts
// anything that merely contains a match.
func anchoredPattern(validation string) string {
	return `\A(?:` + validation + `)\z`
}

func (g *SpecificationGenerator) fieldTypeToElementType(fieldType string) schema.ElementType {
	switch fieldType {
	case "bool":
		return schema.ElementType{Bool: &schema.BoolType{}}
	case fields.Int:
		return schema.ElementType{Int64: &schema.Int64Type{}}
	case "float64":
		return schema.ElementType{Float64: &schema.Float64Type{}}
	default:
		// Strings, and anything unknown, are string elements.
		return schema.ElementType{String: &schema.StringType{}}
	}
}

// toTerraformName converts a Go struct name to a Terraform resource/data source name.
func toTerraformName(name string) string {
	// Convert CamelCase to snake_case and lowercase
	return strings.ToLower(strcase.ToSnake(name))
}

// WriteSpecification writes the specification to a JSON file.
func (g *SpecificationGenerator) WriteSpecification(outputPath string) error {
	spec := g.Generate()

	if err := spec.Validate(context.Background()); err != nil {
		panic(err)
	}

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal specification: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write specification file: %w", err)
	}

	return nil
}

func ptr[T any](in T) *T {
	return &in
}

func findAttr(name string) func(a resource.Attribute) bool {
	return func(a resource.Attribute) bool {
		return a.Name == name
	}
}
