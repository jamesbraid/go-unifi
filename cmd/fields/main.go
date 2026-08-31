package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/hashicorp/go-version"
	"github.com/iancoleman/strcase"
	"github.com/ubiquiti-community/go-unifi/internal/capturelock"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

type replacement struct {
	Old string
	New string
}

var fieldReps = []replacement{
	{"Dhcpdv6", "DHCPDV6"},

	{"Dhcpd", "DHCPD"},
	{"Idx", "IDX"},
	{"Ipsec", "IPSec"},
	{"Ipv6", "IPV6"},
	{"Openvpn", "OpenVPN"},
	{"Tftp", "TFTP"},
	{"Wlangroup", "WLANGroup"},

	{"FrrBgpdConfig", "Config"},
	{"BgpConfig", "BGPConfig"},

	{"Bc", "Broadcast"},
	{"Dhcp", "DHCP"},
	{"Dns", "DNS"},
	{"Dpi", "DPI"},
	{"Dtim", "DTIM"},
	{"Firewallgroup", "FirewallGroup"},
	{"Fixedip", "FixedIP"},
	{"Icmp", "ICMP"},
	{"Id", "ID"},
	{"Igmp", "IGMP"},
	{"Ip", "IP"},
	{"Leasetime", "LeaseTime"},
	{"Mac", "MAC"},
	{"Mcastenhance", "MulticastEnhance"},
	{"Minrssi", "MinRSSI"},
	{"Monthdays", "MonthDays"},
	{"Nat", "NAT"},
	{"Networkconf", "Network"},
	{"Networkgroup", "NetworkGroup"},
	{"Pd", "PD"},
	{"Pmf", "PMF"},
	{"pnp", "PnP"},
	{"Portconf", "PortProfile"},
	{"Qos", "QOS"},
	{"Radiusprofile", "RADIUSProfile"},
	{"Radius", "RADIUS"},
	{"Ssid", "SSID"},
	{"Smartq", "SmartQ"},
	{"Startdate", "StartDate"},
	{"Starttime", "StartTime"},
	{"Stopdate", "StopDate"},
	{"Stoptime", "StopTime"},
	{"Supression", "Suppression"}, //nolint:misspell
	{"Tcp", "TCP"},
	{"Udp", "UDP"},
	{"Usergroup", "UserGroup"},
	{"Utc", "UTC"},
	{"Vlan", "VLAN"},
	{"Vpn", "VPN"},
	{"Wan", "WAN"},
	{"Wep", "WEP"},
	{"Wlan", "WLAN"},
	{"Wpa", "WPA"},
	{"XWireguardPrivateKey", "WireguardPrivateKey"},
	{"XSsh", "SSH"},
	{"XMgmt", "Mgmt"},
	{"UnifiIDp", "UniFiIdentityProvider"},
	{"PortStop", "Stop"},
	{"PortStart", "Start"},
	{"IPStart", "Start"},
	{"IPStop", "Stop"},
	{"IPVersion", "Version"},
	{"IPOrSubnet", "Address"},
}

var fileReps = []replacement{
	{"WlanConf", "WLAN"},
	{"Dhcp", "DHCP"},
	{"Wlan", "WLAN"},
	{"NetworkConf", "Network"},
	{"PortConf", "PortProfile"},
	{"RadiusProfile", "RADIUSProfile"},
	{"ApGroups", "APGroup"},
	{"DnsRecord", "DNSRecord"},
	{"BgpConfig", "BGPConfig"},
	{"User", "Client"},
	{"UserGroup", "ClientGroup"},
}

type ResourceInfo struct {
	StructName string
	// ResourcePath is the REST path segment, which several resources
	// override; Collection keeps the controller's collection name (the
	// lowercased schema file base name) for sensitive_metadata lookups.
	ResourcePath string
	// ListResourcePath is the collection path used by the list call, for the
	// rare resource whose list endpoint differs from the one every other
	// verb uses. Defaults to ResourcePath.
	ListResourcePath string
	Collection       string
	Types            map[string]*FieldInfo
	FieldProcessor   func(name string, f *FieldInfo) error
}

type FieldInfo struct {
	FieldName           string
	JSONName            string
	FieldType           string
	IsPointer           bool
	FieldValidation     string
	OmitEmpty           bool
	IsArray             bool
	Fields              map[string]*FieldInfo
	CustomUnmarshalType string
	CustomUnmarshalFunc string
	// ReadOnly marks a field the controller sends but refuses on a write.
	// It still unmarshals; the generated MarshalJSON drops it, so a value
	// read from the controller can be edited and written straight back.
	ReadOnly bool
	// Doc renders as a doc comment above the generated field (e.g. a
	// "Deprecated:" marker on compat pins); set from overrides/fields.toml.
	Doc string
	// NilAsEmpty makes the generated MarshalJSON send [] rather than null
	// for a nil slice. A slice serialized unconditionally otherwise puts
	// null on the wire whenever the caller left it alone, and the
	// controller rejects null for several of these (measured by
	// TestIntegrationNullWrites).
	NilAsEmpty bool
}

// HasReadOnly reports whether any of the type's fields is read-only, i.e.
// whether the generated type needs a MarshalJSON that drops them.
func (f *FieldInfo) HasReadOnly() bool {
	for _, child := range f.Fields {
		if child != nil && child.ReadOnly {
			return true
		}
	}
	return false
}

// HasNilAsEmpty reports whether any of the type's fields needs a nil slice
// rendered as [], i.e. whether the generated type needs a MarshalJSON for it.
func (f *FieldInfo) HasNilAsEmpty() bool {
	for _, child := range f.Fields {
		if child != nil && child.NilAsEmpty {
			return true
		}
	}
	return false
}

// Controller envelope JSON keys the generator adds to every resource
// itself (see baseType.Fields below). This is the single source of truth
// for them: drift.go's driftIgnoredKeys is derived from envelopeJSONKeys
// so the schema-drift probe can never fall out of sync with what the
// generator actually emits.
const (
	envelopeIDKey       = "_id"
	envelopeSiteIDKey   = "site_id"
	envelopeHiddenKey   = "attr_hidden"
	envelopeHiddenIDKey = "attr_hidden_id"
	envelopeNoDeleteKey = "attr_no_delete"
	envelopeNoEditKey   = "attr_no_edit"
)

var envelopeJSONKeys = []string{
	envelopeIDKey,
	envelopeSiteIDKey,
	envelopeHiddenKey,
	envelopeHiddenIDKey,
	envelopeNoDeleteKey,
	envelopeNoEditKey,
}

func NewResource(structName string, resourcePath string) *ResourceInfo {
	baseType := NewFieldInfo(structName, resourcePath, "struct", "", false, false, false, "")
	resource := &ResourceInfo{
		StructName:   structName,
		ResourcePath: resourcePath,
		Collection:   resourcePath,
		Types: map[string]*FieldInfo{
			structName: baseType,
		},
		FieldProcessor: func(name string, f *FieldInfo) error { return nil },
	}

	// Since template files iterate through map keys in sorted order, these initial fields
	// are named such that they stay at the top for consistency. The spacer items create a
	// blank line in the resulting generated file.
	//
	// This hack is here for stability of the generatd code, but can be removed if desired.
	baseType.Fields = map[string]*FieldInfo{
		"   ID":      NewFieldInfo("ID", envelopeIDKey, fields.String, "", true, false, false, ""),
		"   SiteID":  NewFieldInfo("SiteID", envelopeSiteIDKey, fields.String, "", true, false, false, ""),
		"   _Spacer": nil,

		"  Hidden":   NewFieldInfo("Hidden", envelopeHiddenKey, fields.Bool, "", true, false, false, ""),
		"  HiddenID": NewFieldInfo("HiddenID", envelopeHiddenIDKey, fields.String, "", true, false, false, ""),
		"  NoDelete": NewFieldInfo("NoDelete", envelopeNoDeleteKey, fields.Bool, "", true, false, false, ""),
		"  NoEdit":   NewFieldInfo("NoEdit", envelopeNoEditKey, fields.Bool, "", true, false, false, ""),
		"  _Spacer":  nil,

		" _Spacer": nil,
	}

	switch {
	case resource.IsSetting():
		resource.ResourcePath = strcase.ToSnake(strings.TrimPrefix(structName, "Setting"))
		resource.ListResourcePath = resource.ResourcePath
		baseType.Fields[" Key"] = NewFieldInfo("Key", "key", fields.String, "", false, false, false, "")
	case resource.StructName == "FirewallPolicy":
		resource.FieldProcessor = func(name string, f *FieldInfo) error {
			// The source/destination `port` is not a single number: the
			// firmware stores and expects it as a string, and it may carry a
			// comma-separated list (e.g. "80,443"). Model it as a string and
			// decode either wire form (bare number or quoted string) through
			// types.Number. An empty string is dropped by omitempty, which is
			// what the controller expects for port_matching_type ANY
			// (terraform-provider-unifi #288, #286).
			if name == "Port" {
				f.FieldType = fields.String
				f.IsPointer = false
				f.OmitEmpty = true
				f.FieldValidation = ""
				f.CustomUnmarshalType = fields.Number
				f.CustomUnmarshalFunc = ""
			}
			return nil
		}
	case resource.StructName == "Device":
		// Keyed with a leading space so it sorts to the top of the struct;
		// stays here rather than overrides/fields.toml for that reason.
		baseType.Fields[" MAC"] = NewFieldInfo("MAC", "mac", fields.String, "", true, false, false, "")
	case resource.StructName == "Client":
		// No validation: the controller has no schema for this field, and
		// the validation slot is for what the controller's own validator
		// says. A note there would be published as a rule -- as a pattern in
		// FieldValidationPatterns, and as a RegexMatches in the generated
		// specification that accepts only the note's own text.
		baseType.Fields[" DisplayName"] = NewFieldInfo("DisplayName", "display_name", fields.String, "", true, false, false, "")
	}

	// REST paths that differ from the schema name come from
	// overrides/fields.toml.
	if override, ok := resourceOverrides()[structName]; ok && override.Path != "" {
		resource.ResourcePath = override.Path
	}
	resource.ListResourcePath = resource.ResourcePath
	if override, ok := resourceOverrides()[structName]; ok && override.ListPath != "" {
		resource.ListResourcePath = override.ListPath
	}

	return resource
}

// jsonNameRe bounds what a schema wire name may contain. Names are rendered
// into struct tags (inside backtick-quoted literals) and identifiers, so
// anything outside this set could inject Go source into the generated code,
// which CI compiles and runs.
var jsonNameRe = regexp.MustCompile(`^[A-Za-z0-9_.:+-]+$`)

// newlineRe collapses line breaks in validation strings, which are rendered
// into // comments in the generated code — a newline there would escape the
// comment.
var newlineRe = regexp.MustCompile(`[\r\n\x60]+`)

// nonValidatorNotes are strings that have been used to explain a field
// rather than to validate it. They are refused where a validator is
// expected, because everything downstream treats that slot as a regex.
var nonValidatorNotes = map[string]struct{}{
	"non-generated field": {},
}

func NewFieldInfo(
	fieldName string,
	jsonName string,
	fieldType string,
	fieldValidation string,
	omitempty bool,
	isArray bool,
	isPointer bool,
	customUnmarshalType string,
) *FieldInfo {
	if !jsonNameRe.MatchString(jsonName) {
		panic(fmt.Sprintf("refusing to generate code for unsafe schema field name %q", jsonName))
	}
	// A field the controller does not describe has no validation, and saying
	// so in prose puts the prose where a validator belongs: it reaches
	// FieldValidationPatterns as a pattern and the generated specification as
	// a validator matching only that sentence. Leave it empty instead.
	if _, prose := nonValidatorNotes[fieldValidation]; prose {
		panic(fmt.Sprintf(
			"field %q carries the note %q where its validator belongs; leave the validation "+
				"empty and put the note in the field's doc", jsonName, fieldValidation))
	}

	return &FieldInfo{
		FieldName:           fieldName,
		JSONName:            jsonName,
		FieldType:           fieldType,
		FieldValidation:     newlineRe.ReplaceAllString(fieldValidation, " "),
		OmitEmpty:           omitempty,
		IsArray:             isArray,
		IsPointer:           isPointer,
		CustomUnmarshalType: customUnmarshalType,
	}
}

func cleanName(name string, reps []replacement) string {
	for _, rep := range reps {
		name = strings.ReplaceAll(name, rep.Old, rep.New)
	}

	if strings.HasPrefix(name, "X") && len(name) > 1 && unicode.IsUpper(rune(name[1])) {
		name = name[1:]
	}

	return name
}

func usage() {
	fmt.Printf("Usage: %s [OPTIONS]\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func buildLockedSchemas(
	schemasDir, fieldsDir, metadataDir, customDir string,
	artifactPath string,
	expectedNetworkVersion string,
	expectedSnapshots *capturelock.Snapshots,
) (*version.Version, capturelock.Snapshots, error) {
	tmpRoot := filepath.Join(filepath.Dir(schemasDir), ".tmp")
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		return nil, capturelock.Snapshots{}, err
	}
	workDir, err := os.MkdirTemp(tmpRoot, "schema-run-")
	if err != nil {
		return nil, capturelock.Snapshots{}, err
	}
	defer os.RemoveAll(workDir)

	arts, err := extractArtifacts(artifactPath, workDir)
	if err != nil {
		return nil, capturelock.Snapshots{}, err
	}
	networkVersion, err := readNetworkVersion(arts.aceJar)
	if err != nil {
		return nil, capturelock.Snapshots{}, fmt.Errorf("unable to determine UniFi Network version: %w", err)
	}
	if expectedNetworkVersion != "" && networkVersion.String() != expectedNetworkVersion {
		return nil, capturelock.Snapshots{}, fmt.Errorf(
			"artifact reports UniFi Network %s, lock requires %s",
			networkVersion,
			expectedNetworkVersion,
		)
	}

	defsJar, err := resolveDefsJar(arts, workDir)
	if err != nil {
		return nil, capturelock.Snapshots{}, err
	}
	stagingFields := filepath.Join(workDir, "fields")
	stagingMetadata := filepath.Join(workDir, "metadata")
	if err := extractSchemas(defsJar, stagingFields, stagingMetadata, customDir); err != nil {
		return nil, capturelock.Snapshots{}, err
	}
	structuralDigest, fieldDocuments, err := capturelock.DigestSnapshot(stagingFields)
	if err != nil {
		return nil, capturelock.Snapshots{}, fmt.Errorf("digest structural snapshot: %w", err)
	}
	sensitivityDigest, err := capturelock.DigestFile(filepath.Join(stagingMetadata, "sensitive_metadata.json"))
	if err != nil {
		return nil, capturelock.Snapshots{}, fmt.Errorf("digest sensitivity snapshot: %w", err)
	}
	snapshots := capturelock.Snapshots{
		StructuralSHA256:  structuralDigest,
		SensitivitySHA256: sensitivityDigest,
		FieldDocuments:    fieldDocuments,
	}
	if expectedSnapshots != nil {
		if structuralDigest != expectedSnapshots.StructuralSHA256 {
			return nil, capturelock.Snapshots{}, fmt.Errorf(
				"structural snapshot SHA-256 is %s, lock requires %s",
				structuralDigest,
				expectedSnapshots.StructuralSHA256,
			)
		}
		if sensitivityDigest != expectedSnapshots.SensitivitySHA256 {
			return nil, capturelock.Snapshots{}, fmt.Errorf(
				"sensitivity snapshot SHA-256 is %s, lock requires %s",
				sensitivityDigest,
				expectedSnapshots.SensitivitySHA256,
			)
		}
		// The tree digest just agreed, and it covers exactly these bytes,
		// so a mismatch here means the lock's two descriptions of one
		// snapshot disagree: a hand-edited entry, or a lock assembled from
		// two different captures. Anything that pins a single document reads
		// these per-document entries without ever seeing the tree, so this is
		// the only place that disagreement is visible at all.
		if err := verifyFieldDocuments(expectedSnapshots.FieldDocuments, fieldDocuments); err != nil {
			return nil, capturelock.Snapshots{}, err
		}
	}

	for _, swap := range []struct{ from, to string }{
		{stagingFields, fieldsDir},
		{stagingMetadata, metadataDir},
	} {
		if err := os.RemoveAll(swap.to); err != nil {
			return nil, capturelock.Snapshots{}, err
		}
		if err := os.Rename(swap.from, swap.to); err != nil {
			return nil, capturelock.Snapshots{}, err
		}
	}
	return networkVersion, snapshots, nil
}

// verifyFieldDocuments reports the first way the lock's per-document digests
// differ from the snapshot just measured, naming the document rather than only
// saying that something moved.
//
// Naming it is most of the value. A whole-tree digest that has changed says
// only that one of eighty-two definitions is different, which is what made an
// unrelated firewall-policy override look like a dns_record problem.
func verifyFieldDocuments(want, got map[string]string) error {
	names := make([]string, 0, len(got))
	for name := range got {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		locked, ok := want[name]
		if !ok {
			return fmt.Errorf("field document %s is absent from the lock's snapshots.field_documents", name)
		}
		if locked != got[name] {
			return fmt.Errorf("field document %s SHA-256 is %s, lock requires %s", name, got[name], locked)
		}
	}
	missing := make([]string, 0)
	for name := range want {
		if _, ok := got[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return fmt.Errorf(
			"lock pins snapshots.field_documents entries the capture does not produce: %s",
			strings.Join(missing, ", "),
		)
	}
	return nil
}

func verifyInputDigests(want, got capturelock.Inputs) error {
	if want.ExtractionRulesSHA256 != got.ExtractionRulesSHA256 {
		return fmt.Errorf(
			"extraction-rules SHA-256 is %s, lock requires %s",
			got.ExtractionRulesSHA256,
			want.ExtractionRulesSHA256,
		)
	}
	if want.GeneratorInputsSHA256 != got.GeneratorInputsSHA256 {
		return fmt.Errorf(
			"generator-input SHA-256 is %s, lock requires %s",
			got.GeneratorInputsSHA256,
			want.GeneratorInputsSHA256,
		)
	}
	return nil
}

var handWrittenTypesCache = map[string]map[string]string{}

// handWrittenTypes returns every type declared by non-generated .go files in
// dir (any declaration form, via go/parser), mapped to the declaring file
// name. Results are cached per directory.
func handWrittenTypes(dir string) map[string]string {
	if cached, ok := handWrittenTypesCache[dir]; ok {
		return cached
	}

	decls := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		handWrittenTypesCache[dir] = decls
		return decls
	}

	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, ".generated.go") {
			continue
		}

		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			panic(fmt.Sprintf("unable to parse %s for the type-collision check: %v", filepath.Join(dir, name), err))
		}
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					decls[typeSpec.Name.Name] = name
				}
			}
		}
	}

	handWrittenTypesCache[dir] = decls
	return decls
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func main() {
	flag.Usage = usage
	outputDirFlag := flag.String(
		"output-dir",
		"unifi",
		"The output directory of the generated Go code",
	)
	extractOnly := flag.Bool(
		"extract-only",
		false,
		"Only verify the lock and extract the fields JSON directory",
	)
	verifyLockOnly := flag.Bool(
		"verify-lock-only",
		false,
		"Verify lock structure and generator-input digests without resolving the retained artifact",
	)
	lockPathFlag := flag.String(
		"lock",
		"",
		"Capture lock path (default: schemas/capture.lock.json under the module root)",
	)
	contentStoreFlag := flag.String(
		"content-store",
		os.Getenv("GO_UNIFI_CONTENT_STORE"),
		"Restricted content-addressed artifact store (or GO_UNIFI_CONTENT_STORE)",
	)
	snapshotOutput := flag.String(
		"snapshot-output",
		"",
		"Capture helper: inspect a draft lock and write snapshot digests without updating the lock",
	)
	generateSpec := flag.Bool(
		"generate-spec",
		false,
		"Generate Terraform provider specification JSON file",
	)
	specOutputPath := flag.String(
		"spec-output",
		"specification.json",
		"Output path for the Terraform provider specification JSON file",
	)

	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Print("error: generation accepts no version argument; capture a new lock separately\n\n")
		usage()
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	moduleRoot := findModuleRoot(wd)
	if moduleRoot == "" {
		panic("unable to locate the module root (go.mod)")
	}

	schemasDir := filepath.Join(moduleRoot, "schemas")
	fieldsDir := filepath.Join(schemasDir, "fields")
	metadataDir := filepath.Join(schemasDir, "metadata")
	customDir := filepath.Join(moduleRoot, "overrides", "resources")
	outDir := filepath.Join(wd, *outputDirFlag)
	lockPath := *lockPathFlag
	if lockPath == "" {
		lockPath = filepath.Join(schemasDir, "capture.lock.json")
	}
	var lock capturelock.Lock
	if *snapshotOutput != "" {
		lock, err = capturelock.LoadDraftFile(lockPath)
	} else {
		lock, err = capturelock.LoadFile(lockPath)
	}
	if err != nil {
		panic(err)
	}
	inputs, err := capturelock.ComputeInputDigests(moduleRoot)
	if err != nil {
		panic(err)
	}
	if err := verifyInputDigests(lock.Inputs, inputs); err != nil {
		panic(err)
	}
	if *verifyLockOnly {
		fmt.Println("Capture lock inputs verified!")
		return
	}
	artifactPath, err := capturelock.ResolveArtifact(*contentStoreFlag, lock)
	if err != nil {
		panic(err)
	}
	var expectedSnapshots *capturelock.Snapshots
	if *snapshotOutput == "" {
		expectedSnapshots = &lock.Snapshots
	}
	unifiVersion, snapshots, err := buildLockedSchemas(
		schemasDir,
		fieldsDir,
		metadataDir,
		customDir,
		artifactPath,
		lock.Controller.NetworkVersion,
		expectedSnapshots,
	)
	if err != nil {
		panic(err)
	}
	if *snapshotOutput != "" {
		if err := capturelock.WriteInspectionFile(*snapshotOutput, capturelock.Inspection{
			NetworkVersion: unifiVersion.String(),
			Snapshots:      snapshots,
		}); err != nil {
			panic(err)
		}
		return
	}
	if err := capturelock.WriteCompatibilityProjections(schemasDir, lock); err != nil {
		panic(err)
	}

	if *extractOnly {
		fmt.Println("Fields JSON ready!")
		return
	}

	fieldsFiles, err := os.ReadDir(fieldsDir)
	if err != nil {
		panic(err)
	}

	// Tracks every .generated.go written this run so files whose schema
	// disappeared upstream can be removed afterwards.
	writtenGenerated := map[string]bool{}

	// Resources this run actually produced, so a preference table naming a
	// resource the schema no longer defines is reported rather than emitted
	// against nothing.
	generatedResources := map[string]bool{}

	// Schema validation patterns, accumulated across every resource and
	// written out per package once the loop finishes.
	var unifiValidation, settingsValidation []validationEntry

	// Initialize specification generator
	sensitive, err := loadSensitiveMetadata(filepath.Join(metadataDir, "sensitive_metadata.json"))
	if err != nil {
		panic(err)
	}
	specGen := NewSpecificationGenerator("unifi", sensitive)

	for _, fieldsFile := range fieldsFiles {
		name := fieldsFile.Name()
		ext := filepath.Ext(name)

		switch name {
		case "AuthenticationRequest.json", "Setting.json", "Wall.json":
			continue
		}

		if filepath.Ext(name) != ".json" {
			continue
		}

		name = name[:len(name)-len(ext)]

		urlPath := strings.ToLower(name)
		structName := cleanName(name, fileReps)

		// For settings, create a cleaner filename without "setting_" prefix
		goFile := strcase.ToSnake(structName) + ".generated.go"
		if after, ok0 := strings.CutPrefix(structName, "Setting"); ok0 {
			// Remove "Setting" prefix for the file name
			cleanStructName := after
			goFile = strcase.ToSnake(cleanStructName) + ".generated.go"
		}
		fieldsFilePath := filepath.Join(fieldsDir, fieldsFile.Name())
		b, err := os.ReadFile(fieldsFilePath)
		if err != nil {
			fmt.Printf("skipping file %s: %s", fieldsFile.Name(), err)
			continue
		}

		resource := NewResource(structName, urlPath)

		switch resource.StructName {
		case "Account":
			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				switch name {
				case "IP", "NetworkID":
					f.OmitEmpty = true
				}
				return nil
			}
		case "Device":
			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				switch name {
				case "X", "Y":
					f.FieldType = "float64"
				case "StpPriority":
					f.FieldType = fields.Int
					f.CustomUnmarshalType = fields.Number
				case "ConfigNetwork", "EtherLighting", "MbbOverrides", "NutServer", "RpsOverride", "QOSProfile":
					f.IsPointer = true
				case "Ht":
					// Field within DeviceRadioTable nested type
					f.CustomUnmarshalType = "types.Number"
					f.CustomUnmarshalFunc = "types.ToInt64Pointer"
				}

				f.OmitEmpty = true
				switch name {
				case "PortOverrides":
					f.OmitEmpty = false
				}

				return nil
			}
		case "Network":
			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				switch name {
				case "InternetAccessEnabled", "IntraNetworkAccessEnabled":
					if f.FieldType == fields.Bool {
						f.CustomUnmarshalType = "*bool"
						f.CustomUnmarshalFunc = "emptyBoolToTrue"
					}
				case "DHCPDEnabled":
					// Some controllers (UniFi Network 10.x) return "true"/"false"
					// as JSON strings for this flag, which breaks a plain bool.
					// Decode through the tolerant types.Bool. See
					// terraform-provider-unifi #65.
					if f.FieldType == fields.Bool {
						f.CustomUnmarshalType = "*types.Bool"
						f.CustomUnmarshalFunc = "boolValue"
					}
				case "IPSecEspLifetime", "IPSecIkeLifetime":
					f.FieldType = fields.Int
					f.IsPointer = true
				case "WANDNS1", "WANDNS2", "WANIPV6DNS1", "WANIPV6DNS2", "DHCPDStart", "DHCPDStop", "DHCPDUnifiController",
					"DHCPDTFTPServer", "DHCPDWins1", "DHCPDWins2", "DHCPDWPAdUrl", "DomainName", "DHCPDGateway", "DHCPDNtp1", "DHCPDNtp2",
					"DHCPDDNS1", "DHCPDDNS2", "DHCPDDNS3", "DHCPDDNS4":
					// A pointer so the field carries three states, not two.
					//
					// The DHCP slots used to be plain strings, pinned that way
					// for type stability. Measured on 10.6.101, that made them
					// impossible to clear: sending "" is the only thing that
					// clears a slot -- omitting the key leaves the stored value
					// alone -- and a plain string with omitempty cannot put ""
					// on the wire. A caller emptying a DNS list had no way to
					// say so.
					//
					// nil is "leave it alone" and a pointer to "" is "clear it".
					f.OmitEmpty = true
					f.IsPointer = true
				case "Purpose":
					f.OmitEmpty = false
					f.IsPointer = false
				}
				if f.OmitEmpty && !f.IsArray {
					switch f.FieldType {
					case fields.Bool, fields.String:
						f.IsPointer = true
					}
				}
				return nil
			}
		case "SettingGlobalAp":
			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				if strings.HasPrefix(name, "6E") {
					f.FieldName = strings.Replace(f.FieldName, "6E", "SixE", 1)
				}

				return nil
			}
		case "SettingMgmt":
			sshKeyField := NewFieldInfo(resource.StructName+"SSHKeys", "x_ssh_keys", "struct", "", false, false, false, "")
			sshKeyField.Fields = map[string]*FieldInfo{
				"name":        NewFieldInfo("Name", "name", fields.String, "", false, false, false, ""),
				"keyType":     NewFieldInfo("KeyType", "type", fields.String, "", false, false, false, ""),
				"key":         NewFieldInfo("Key", "key", fields.String, "", false, false, false, ""),
				"comment":     NewFieldInfo("Comment", "comment", fields.String, "", false, false, false, ""),
				"date":        NewFieldInfo("Date", "date", fields.String, "", false, false, false, ""),
				"fingerprint": NewFieldInfo("Fingerprint", "fingerprint", fields.String, "", false, false, false, ""),
			}
			resource.Types[sshKeyField.FieldName] = sshKeyField

			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				if name == "SSHKeys" {
					f.FieldType = sshKeyField.FieldName
				}
				return nil
			}
		case "SettingUsg":
			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				if strings.HasSuffix(name, "Timeout") && name != "ArpCacheTimeout" {
					f.FieldType = fields.Int
					f.CustomUnmarshalType = fields.Number
				}
				return nil
			}
		case "Nat":
			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				switch name {
				case "SourceFilter":
					f.IsPointer = true
				case "DestinationFilter":
					f.IsPointer = true
				}
				return nil
			}
		case "Client":
			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				switch name {
				case "Blocked":
					f.FieldType = fields.Bool
					f.IsPointer = true
					// Some controllers return "true"/"false" as JSON strings
					// for this flag, which breaks a plain *bool. Decode through
					// the tolerant types.Bool. See terraform-provider-unifi #132.
					f.CustomUnmarshalType = "*types.Bool"
					f.CustomUnmarshalFunc = "boolPtrValue"
				case "VirtualNetworkOverrideEnabled":
					f.FieldType = fields.Bool
					f.IsPointer = true
					f.OmitEmpty = true
				case "LastSeen":
					f.FieldType = fields.Int
					f.IsPointer = true
				}
				return nil
			}
		case "WLAN":
			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				switch name {
				case "ScheduleWithDuration":
					// always send schedule, so we can empty it if we want to
					f.OmitEmpty = false
				}
				return nil
			}
		case "DNSRecord":
			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				switch name {
				case "Hidden", "NoDelete", "NoEdit", "Enabled":
					f.FieldType = fields.Bool
				case "Priority", "Ttl", "Weight":
					f.FieldType = fields.Int
					f.CustomUnmarshalType = fields.Number
				}
				return nil
			}
		}

		// After the resource's own processor, so one that installs its own
		// does not lose the rule -- and so a hand-written decision, which
		// runs first, still wins.
		resource.FieldProcessor = withNumberOrWordDecoding(resource.FieldProcessor)

		err = resource.processJSON(b)
		if err != nil {
			fmt.Printf("skipping file %s: %s", fieldsFile.Name(), err)
			continue
		}

		// Add fields not present in the JAR schema to nested types.
		if resource.StructName == "Device" {
			if portOverrides, ok := resource.Types["DevicePortOverrides"]; ok {
				portOverrides.Fields["TaggedNetworkIDs"] = NewFieldInfo("TaggedNetworkIDs", "tagged_networkconf_ids", fields.String, "", true, true, false, "")
			}
		}

		if err := resource.applyOverrides(); err != nil {
			panic(err)
		}
		generatedResources[resource.StructName] = true

		// Add resource to specification generator
		specGen.AddResource(resource)

		// Capture the schema's validation patterns before the template
		// renders them into comments, so they can be emitted as consumable
		// Go alongside the structs.
		if resource.IsSetting() {
			settingsValidation = append(settingsValidation, collectValidation(resource)...)
		} else {
			unifiValidation = append(unifiValidation, collectValidation(resource)...)
		}

		var code string
		if code, err = resource.generateCode(false); err != nil {
			panic(err)
		}

		// Determine output directory based on whether it's a setting
		var targetDir string
		if resource.IsSetting() {
			targetDir = filepath.Join(outDir, "settings")
			// Ensure settings directory exists
			if err := os.MkdirAll(targetDir, 0o755); err != nil {
				panic(err)
			}
		} else {
			targetDir = outDir
		}

		// A schema update can start defining a type that was previously
		// hand-written (e.g. IgmpSnooping when 10.x added its schema). Fail
		// with the resolution instead of leaving a duplicate declaration for
		// the compiler to trip over.
		for typeName := range resource.Types {
			if declFile, ok := handWrittenTypes(targetDir)[typeName]; ok {
				panic(fmt.Sprintf(
					"generated type %s (from %s) collides with the hand-written declaration in %s; the schema now defines it - remove or rename the hand-written type",
					typeName, fieldsFile.Name(), declFile,
				))
			}
		}

		_ = os.Remove(filepath.Join(targetDir, goFile))
		if err := os.WriteFile(filepath.Join(targetDir, goFile), ([]byte)(code), 0o644); err != nil {
			panic(err)
		}
		writtenGenerated[filepath.Join(targetDir, goFile)] = true

		if !resource.IsSetting() {
			implFile := strcase.ToSnake(structName) + ".go"
			implFilePath := filepath.Join(targetDir, implFile)

			if _, err := os.Stat(implFilePath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					var implCode string
					if implCode, err = resource.generateCode(true); err != nil {
						panic(err)
					}

					if err := os.WriteFile(filepath.Join(implFilePath), ([]byte)(implCode), 0o644); err != nil {
						panic(err)
					}
				}
			}
		}
	}

	// Write version file.
	versionGo := fmt.Appendf(nil, `
// Generated code. DO NOT EDIT.

package unifi

const UnifiVersion = %q
`, unifiVersion)

	versionGo, err = format.Source(versionGo)
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(filepath.Join(outDir, "version.generated.go"), versionGo, 0o644); err != nil {
		panic(err)
	}
	writtenGenerated[filepath.Join(outDir, "version.generated.go")] = true

	preferenceGo, err := generatePreferenceFile(generatedResources)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "preference.generated.go"), preferenceGo, 0o644); err != nil {
		panic(err)
	}
	writtenGenerated[filepath.Join(outDir, "preference.generated.go")] = true

	// Write the validation patterns and enumeration values per package.
	for _, target := range []struct {
		pkg     string
		dir     string
		entries []validationEntry
	}{
		{pkg: "unifi", dir: outDir, entries: unifiValidation},
		{pkg: "settings", dir: filepath.Join(outDir, "settings"), entries: settingsValidation},
	} {
		if len(target.entries) == 0 {
			continue
		}
		validationGo, err := renderValidationFile(target.pkg, target.entries)
		if err != nil {
			panic(fmt.Errorf("render %s validation file: %w", target.pkg, err))
		}
		path := filepath.Join(target.dir, "validation.generated.go")
		if err := os.WriteFile(path, validationGo, 0o644); err != nil {
			panic(err)
		}
		writtenGenerated[path] = true
	}

	// The controller's own sensitive-field list, so the client redacts by
	// name instead of guessing from substrings.
	if sensitiveGo, err := renderSensitiveFile("unifi", sensitive); err != nil {
		panic(fmt.Errorf("render sensitive file: %w", err))
	} else {
		path := filepath.Join(outDir, "sensitive.generated.go")
		if err := os.WriteFile(path, sensitiveGo, 0o644); err != nil {
			panic(err)
		}
		writtenGenerated[path] = true
	}

	// A resource that left the schema must also leave the SDK, or the public
	// API silently diverges from the controller (and apidiff never sees the
	// removal). Delete generated files no schema produced this run, and fail
	// when a hand-written companion would be orphaned so a maintainer
	// removes it deliberately.
	var orphans []string
	for _, dir := range []string{outDir, filepath.Join(outDir, "settings")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".generated.go") || writtenGenerated[filepath.Join(dir, name)] {
				continue
			}

			fmt.Printf("removing %s: its schema no longer exists upstream\n", name)
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				panic(err)
			}

			companion := strings.TrimSuffix(name, ".generated.go") + ".go"
			if fileExists(filepath.Join(dir, companion)) {
				orphans = append(orphans, filepath.Join(dir, companion))
			}
		}
	}
	if len(orphans) > 0 {
		panic(fmt.Sprintf(
			"hand-written files reference resources whose schema was removed upstream; delete them (and their tests) to match: %s",
			strings.Join(orphans, ", "),
		))
	}

	// Generate Terraform provider specification if requested
	if *generateSpec {
		specOutputFile := *specOutputPath
		if !filepath.IsAbs(specOutputFile) {
			specOutputFile = filepath.Join(wd, specOutputFile)
		}
		if err := specGen.WriteSpecification(specOutputFile); err != nil {
			panic(err)
		}
		fmt.Printf("Generated specification: %s\n", specOutputFile)
	}

	fmt.Printf("%s\n", outDir)
}

func (r *ResourceInfo) IsSetting() bool {
	return strings.HasPrefix(r.StructName, "Setting")
}

func (r *ResourceInfo) IsDevice() bool {
	return r.StructName == "Device"
}

func (r *ResourceInfo) IsV2() bool {
	return slices.Contains([]string{
		"APGroup",
		"BGPConfig",
		"ContentFiltering",
		"DNSRecord",
		"FirewallPolicy",
		"FirewallZone",
		"Nat",
		"NetworkMembersGroup",
		"OSPFRouter",
		"TrafficRoute",
	}, r.StructName)
}

func (r *ResourceInfo) CleanStructName() string {
	if r.IsSetting() {
		return strings.TrimPrefix(r.StructName, "Setting")
	}
	return r.StructName
}

func (r *ResourceInfo) processFields(fields map[string]any) {
	t := r.Types[r.StructName]
	for name, validation := range fields {
		fieldInfo, err := r.fieldInfoFromValidation(name, validation)
		if err != nil {
			continue
		}

		t.Fields[fieldInfo.FieldName] = fieldInfo
	}
}

func (r *ResourceInfo) fieldInfoFromValidation(name string, validation any) (*FieldInfo, error) {
	fieldName := strcase.ToCamel(name)
	fieldName = cleanName(fieldName, fieldReps)

	empty := &FieldInfo{}
	var fieldInfo *FieldInfo

	switch validation := validation.(type) {
	case []any:
		if len(validation) == 0 {
			fieldInfo = NewFieldInfo(fieldName, name, fields.String, "", false, true, false, "")
			err := r.FieldProcessor(fieldName, fieldInfo)
			return fieldInfo, err
		}
		if len(validation) > 1 {
			return empty, fmt.Errorf("unknown validation %#v", validation)
		}

		fieldInfo, err := r.fieldInfoFromValidation(name, validation[0])
		if err != nil {
			return empty, err
		}

		fieldInfo.OmitEmpty = true
		fieldInfo.IsArray = true
		fieldInfo.IsPointer = false

		err = r.FieldProcessor(fieldName, fieldInfo)
		return fieldInfo, err

	case map[string]any:
		typeName := r.StructName + fieldName

		result := NewFieldInfo(fieldName, name, typeName, "", true, false, true, "")
		result.Fields = make(map[string]*FieldInfo)

		for name, fv := range validation {
			child, err := r.fieldInfoFromValidation(name, fv)
			if err != nil {
				return empty, err
			}

			result.Fields[child.FieldName] = child
		}

		err := r.FieldProcessor(fieldName, result)
		r.Types[typeName] = result
		return result, err

	case string:
		fieldValidation := validation
		normalized := normalizeValidation(validation)

		omitEmpty := false

		switch normalized {
		case "falsetrue", "truefalse":
			fieldInfo = NewFieldInfo(fieldName, name, fields.Bool, "", omitEmpty, false, false, "")
			return fieldInfo, r.FieldProcessor(fieldName, fieldInfo)
		default:
			if _, err := strconv.ParseFloat(normalized, 64); err == nil {
				if normalized == "09" || normalized == "09.09" {
					fieldValidation = ""
				}

				if strings.Contains(normalized, ".") {
					if strings.Contains(validation, "\\.){3}") {
						break
					}

					omitEmpty = true
					fieldInfo = NewFieldInfo(fieldName, name, "float64", fieldValidation, omitEmpty, false, false, "")
					return fieldInfo, r.FieldProcessor(fieldName, fieldInfo)
				}

				omitEmpty = true
				fieldInfo = NewFieldInfo(fieldName, name, fields.Int, fieldValidation, omitEmpty, false, true, fields.Number)
				return fieldInfo, r.FieldProcessor(fieldName, fieldInfo)
			}
		}
		if validation != "" && normalized != "" {
			fmt.Printf("normalize %q to %q\n", validation, normalized)
		}

		omitEmpty = omitEmpty || (!strings.Contains(validation, "^$") && !strings.HasSuffix(fieldName, "Id"))
		fieldInfo = NewFieldInfo(fieldName, name, fields.String, fieldValidation, omitEmpty, false, false, "")
		return fieldInfo, r.FieldProcessor(fieldName, fieldInfo)
	}

	return empty, fmt.Errorf("unable to determine type from validation %q", validation)
}

func (r *ResourceInfo) processJSON(b []byte) error {
	var fields map[string]any
	err := json.Unmarshal(b, &fields)
	if err != nil {
		return err
	}

	r.processFields(fields)

	return nil
}

//go:embed api.go.tmpl
var apiGoTemplate string

//go:embed client.go.tmpl
var clientGoTemplate string

func (r *ResourceInfo) generateCode(isImpl bool) (string, error) {
	var err error
	var buf bytes.Buffer
	writer := io.Writer(&buf)

	var tpl *template.Template
	funcMap := template.FuncMap{
		"trimPrefix": strings.TrimPrefix,
	}

	if isImpl {
		tpl = template.Must(template.New("client.go.tmpl").Funcs(funcMap).Parse(clientGoTemplate))
	} else {
		tpl = template.Must(template.New("api.go.tmpl").Funcs(funcMap).Parse(apiGoTemplate))
	}

	err = tpl.Execute(writer, r)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	src, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("failed to format source: %w", err)
	}

	return string(src), err
}

func normalizeValidation(re string) string {
	re = strings.ReplaceAll(re, "\\d", "[0-9]")
	re = strings.ReplaceAll(re, "[-+]?", "")
	re = strings.ReplaceAll(re, "[+-]?", "")
	re = strings.ReplaceAll(re, "[-]?", "")
	re = strings.ReplaceAll(re, "\\.", ".")
	re = strings.ReplaceAll(re, "[.]?", ".")

	quants := regexp.MustCompile(`\{\d*,?\d*\}|\*|\+|\?`)
	re = quants.ReplaceAllString(re, "")

	control := regexp.MustCompile(`[\(\[\]\)\|\-\$\^]`)
	re = control.ReplaceAllString(re, "")

	re = strings.TrimPrefix(re, "^")
	re = strings.TrimSuffix(re, "$")

	return re
}
