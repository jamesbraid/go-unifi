package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

const (
	SpecVersion       = "0.1"
	GoUnifiImportPath = "github.com/ubiquiti-community/go-unifi/unifi"
)

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
	sortedResources := make([]*ResourceInfo, len(g.Resources))
	copy(sortedResources, g.Resources)
	sort.Slice(sortedResources, func(i, j int) bool {
		return sortedResources[i].StructName < sortedResources[j].StructName
	})

	for _, r := range sortedResources {
		// Skip settings for now - they have a different pattern
		if r.IsSetting() {
			continue
		}

		// Generate data source
		ds := g.generateDataSource(r)
		spec.DataSources = append(spec.DataSources, ds)

		// Generate resource
		res := g.generateResource(r)
		spec.Resources = append(spec.Resources, res)
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

// generateDataSource generates a data source specification from a resource.
func (g *SpecificationGenerator) generateDataSource(r *ResourceInfo) datasource.DataSource {
	name := toTerraformName(r.StructName)

	ds := datasource.DataSource{
		Name: name,
		Schema: &datasource.Schema{
			Attributes: g.generateDataSourceAttributes(r),
		},
	}

	return ds
}

// generateDataSourceAttributes generates data source attributes from a resource.
func (g *SpecificationGenerator) generateDataSourceAttributes(r *ResourceInfo) []datasource.Attribute {
	baseType := r.Types[r.StructName]
	if baseType == nil || baseType.Fields == nil {
		return nil
	}

	attrs := make([]datasource.Attribute, 0)

	// Sort fields by name for consistent output
	fieldNames := make([]string, 0, len(baseType.Fields))
	for name := range baseType.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	for _, fieldName := range fieldNames {
		field := baseType.Fields[fieldName]
		if field == nil || strings.HasPrefix(fieldName, " ") || strings.HasSuffix(fieldName, "_Spacer") {
			continue
		}

		attr := g.fieldToDataSourceAttribute(r, field)
		if attr != nil {
			attrs = append(attrs, *attr)
		}
	}

	return attrs
}

// fieldToDataSourceAttribute converts a FieldInfo to a datasource.Attribute.
func (g *SpecificationGenerator) fieldToDataSourceAttribute(r *ResourceInfo, field *FieldInfo) *datasource.Attribute {
	if field == nil {
		return nil
	}

	// The wire name is already snake_case and is what the API actually
	// calls the field. Deriving the attribute name from the Go field name
	// instead produced names no API user would recognise --
	// open_vpn_encryption_cipher for openvpn_encryption_cipher, and
	// l_2_tp_allow_weak_ciphers for l2tp_allow_weak_ciphers.
	name := field.JSONName
	_ = g.buildAssociatedExternalType(r, field)
	var externalType *schema.AssociatedExternalType = nil

	attr := &datasource.Attribute{
		Name: name,
	}

	// Handle array types
	if field.IsArray {
		if field.Fields != nil {
			// Nested object array - use list_nested
			nestedAttrs := g.generateNestedDataSourceAttributes(r, field)
			attr.ListNested = &datasource.ListNestedAttribute{
				ComputedOptionalRequired: "computed",
				NestedObject: datasource.NestedAttributeObject{
					AssociatedExternalType: externalType,
					Attributes:             nestedAttrs,
				},
			}
		} else {
			// Simple array - use list
			attr.List = &datasource.ListAttribute{
				ComputedOptionalRequired: "computed",
				ElementType:              g.fieldTypeToElementType(field.FieldType),
				AssociatedExternalType:   externalType,
				Sensitive:                g.sensitivePtr(r, field),
			}
		}
		return attr
	}

	// Handle nested object types
	if field.Fields != nil {
		nestedAttrs := g.generateNestedDataSourceAttributes(r, field)
		attr.SingleNested = &datasource.SingleNestedAttribute{
			ComputedOptionalRequired: "computed",
			Attributes:               nestedAttrs,
			AssociatedExternalType:   externalType,
		}
		return attr
	}

	// Handle primitive types
	switch field.FieldType {
	case "bool":
		attr.Bool = &datasource.BoolAttribute{
			ComputedOptionalRequired: "computed",
			AssociatedExternalType:   externalType,
			Sensitive:                g.sensitivePtr(r, field),
		}
	case "int64":
		intAttr := &datasource.Int64Attribute{
			ComputedOptionalRequired: "computed",
			AssociatedExternalType:   externalType,
			Sensitive:                g.sensitivePtr(r, field),
		}
		if validators := g.buildInt64Validators(field.FieldValidation); len(validators) > 0 {
			intAttr.Validators = validators
		}
		attr.Int64 = intAttr
	case "float64":
		attr.Float64 = &datasource.Float64Attribute{
			ComputedOptionalRequired: "computed",
			AssociatedExternalType:   externalType,
			Sensitive:                g.sensitivePtr(r, field),
		}
	case "string":
		strAttr := &datasource.StringAttribute{
			ComputedOptionalRequired: "computed",
			AssociatedExternalType:   externalType,
			Sensitive:                g.sensitivePtr(r, field),
		}
		if validators := g.buildStringValidators(field.FieldValidation); len(validators) > 0 {
			strAttr.Validators = validators
		}
		attr.String = strAttr
	default:
		// Check if it's a custom type defined in Types
		if _, ok := r.Types[field.FieldType]; ok {
			nestedAttrs := g.generateNestedDataSourceAttributesFromType(r, field.FieldType)
			attr.SingleNested = &datasource.SingleNestedAttribute{
				ComputedOptionalRequired: "computed",
				Attributes:               nestedAttrs,
				AssociatedExternalType:   externalType,
			}
		} else {
			// Default to string for unknown types
			attr.String = &datasource.StringAttribute{
				ComputedOptionalRequired: "computed",
				AssociatedExternalType:   externalType,
				Sensitive:                g.sensitivePtr(r, field),
			}
		}
	}

	return attr
}

// generateNestedDataSourceAttributes generates nested attributes for data sources.
func (g *SpecificationGenerator) generateNestedDataSourceAttributes(r *ResourceInfo, field *FieldInfo) []datasource.Attribute {
	if field.Fields == nil {
		return nil
	}

	attrs := make([]datasource.Attribute, 0)
	fieldNames := make([]string, 0, len(field.Fields))
	for name := range field.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	for _, fieldName := range fieldNames {
		childField := field.Fields[fieldName]
		if childField == nil {
			continue
		}

		attr := g.fieldToDataSourceAttribute(r, childField)
		if attr != nil {
			attrs = append(attrs, *attr)
		}
	}

	return attrs
}

// generateNestedDataSourceAttributesFromType generates nested attributes from a type name.
func (g *SpecificationGenerator) generateNestedDataSourceAttributesFromType(r *ResourceInfo, typeName string) []datasource.Attribute {
	typeInfo, ok := r.Types[typeName]
	if !ok || typeInfo.Fields == nil {
		return nil
	}

	return g.generateNestedDataSourceAttributes(r, typeInfo)
}

// generateResource generates a resource specification from a Resource.
func (g *SpecificationGenerator) generateResource(r *ResourceInfo) resource.Resource {
	name := toTerraformName(r.StructName)

	res := resource.Resource{
		Name: name,
		Schema: &resource.Schema{
			Attributes: g.generateResourceAttributes(r),
		},
	}

	return res
}

// generateResourceAttributes generates resource attributes from a Resource.
func (g *SpecificationGenerator) generateResourceAttributes(r *ResourceInfo) []resource.Attribute {
	baseType := r.Types[r.StructName]
	if baseType == nil || baseType.Fields == nil {
		return nil
	}

	attrs := make([]resource.Attribute, 0)

	// Sort fields by name for consistent output
	fieldNames := make([]string, 0, len(baseType.Fields))
	for name := range baseType.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

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
	_ = g.buildAssociatedExternalType(r, field)
	var externalType *schema.AssociatedExternalType = nil
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
					AssociatedExternalType: externalType,
					Attributes:             nestedAttrs,
				},
			}
		} else {
			// Simple array - use list
			attr.List = &resource.ListAttribute{
				ComputedOptionalRequired: computedOptionalRequired,
				ElementType:              g.fieldTypeToElementType(field.FieldType),
				AssociatedExternalType:   externalType,
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
			AssociatedExternalType:   externalType,
		}
		return attr
	}

	// Handle primitive types
	switch field.FieldType {
	case "bool":
		attr.Bool = &resource.BoolAttribute{
			ComputedOptionalRequired: computedOptionalRequired,
			AssociatedExternalType:   externalType,
			Sensitive:                g.sensitivePtr(r, field),
		}
	case fields.Int:
		intAttr := &resource.Int64Attribute{
			ComputedOptionalRequired: computedOptionalRequired,
			AssociatedExternalType:   externalType,
			Sensitive:                g.sensitivePtr(r, field),
		}
		if validators := g.buildInt64Validators(field.FieldValidation); len(validators) > 0 {
			intAttr.Validators = validators
		}
		attr.Int64 = intAttr
	case "float64":
		attr.Float64 = &resource.Float64Attribute{
			ComputedOptionalRequired: computedOptionalRequired,
			AssociatedExternalType:   externalType,
			Sensitive:                g.sensitivePtr(r, field),
		}
	case "string":
		strAttr := &resource.StringAttribute{
			ComputedOptionalRequired: computedOptionalRequired,
			AssociatedExternalType:   externalType,
			Sensitive:                g.sensitivePtr(r, field),
		}
		if validators := g.buildStringValidators(field.FieldValidation); len(validators) > 0 {
			strAttr.Validators = validators
		}
		attr.String = strAttr
	default:
		// Check if it's a custom type defined in Types
		if _, ok := r.Types[field.FieldType]; ok {
			nestedAttrs := g.generateNestedResourceAttributesFromType(r, joinContainer(container, field.JSONName), field.FieldType)
			attr.SingleNested = &resource.SingleNestedAttribute{
				ComputedOptionalRequired: computedOptionalRequired,
				Attributes:               nestedAttrs,
				AssociatedExternalType:   externalType,
			}
		} else {
			// Default to string for unknown types
			attr.String = &resource.StringAttribute{
				ComputedOptionalRequired: computedOptionalRequired,
				AssociatedExternalType:   externalType,
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
	fieldNames := make([]string, 0, len(field.Fields))
	for name := range field.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

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

// generateNestedResourceAttributesFromType generates nested attributes from a type name.
func (g *SpecificationGenerator) generateNestedResourceAttributesFromType(r *ResourceInfo, container string, typeName string) []resource.Attribute {
	typeInfo, ok := r.Types[typeName]
	if !ok || typeInfo.Fields == nil {
		return nil
	}

	return g.generateNestedResourceAttributes(r, container, typeInfo)
}

// buildAssociatedExternalType creates an AssociatedExternalType for a field.
func (g *SpecificationGenerator) buildAssociatedExternalType(_ *ResourceInfo, field *FieldInfo) *schema.AssociatedExternalType {
	if field == nil {
		return nil
	}

	var typeName string

	// Build the full type name including pointer and array notation
	if field.IsArray {
		if field.Fields != nil {
			// Nested object array type
			typeName = field.FieldType
		} else {
			typeName = field.FieldType
		}
	} else if field.Fields != nil {
		// Nested object type
		if field.OmitEmpty && field.IsPointer {
			typeName = fmt.Sprintf("*%s", field.FieldType)
		} else {
			typeName = field.FieldType
		}
	} else {
		// Primitive type
		if field.OmitEmpty && field.IsPointer {
			typeName = fmt.Sprintf("*%s", field.FieldType)
		} else {
			typeName = field.FieldType
		}
	}

	if regexp.MustCompile(`string|bool|int64|float64`).MatchString(typeName) {
		return nil
	}

	return &schema.AssociatedExternalType{
		Import: &code.Import{
			Path: GoUnifiImportPath,
		},
		Type: typeName,
	}
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

// buildValidators creates validators from a FieldValidation string.
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
	case "string":
		return schema.ElementType{String: &schema.StringType{}}
	default:
		// Default to string
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

func findMembers(a resource.Attribute) bool {
	return a.Name == "members"
}

func findConfigNetwork(a resource.Attribute) bool {
	return a.Name == "config_network"
}

func findAttr(name string) func(a resource.Attribute) bool {
	return func(a resource.Attribute) bool {
		return a.Name == name
	}
}
