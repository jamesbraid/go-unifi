package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
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
	StructName       string
	ResourcePath     string
	SourceFileBase   string
	SourcePathPrefix []string
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
	Sensitive           bool
}

func NewResource(structName string, resourcePath string) *ResourceInfo {
	baseType := NewFieldInfo(structName, resourcePath, "struct", "", false, false, false, "")
	resource := &ResourceInfo{
		StructName:   structName,
		ResourcePath: resourcePath,
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
		"   ID":      NewFieldInfo("ID", "_id", fields.String, "", true, false, false, ""),
		"   SiteID":  NewFieldInfo("SiteID", "site_id", fields.String, "", true, false, false, ""),
		"   _Spacer": nil,

		"  Hidden":   NewFieldInfo("Hidden", "attr_hidden", fields.Bool, "", true, false, false, ""),
		"  HiddenID": NewFieldInfo("HiddenID", "attr_hidden_id", fields.String, "", true, false, false, ""),
		"  NoDelete": NewFieldInfo("NoDelete", "attr_no_delete", fields.Bool, "", true, false, false, ""),
		"  NoEdit":   NewFieldInfo("NoEdit", "attr_no_edit", fields.Bool, "", true, false, false, ""),
		"  _Spacer":  nil,

		" _Spacer": nil,
	}

	switch {
	case resource.IsSetting():
		resource.ResourcePath = strcase.ToSnake(strings.TrimPrefix(structName, "Setting"))
		baseType.Fields[" Key"] = NewFieldInfo("Key", "key", fields.String, "", false, false, false, "")
		if resource.StructName == "SettingUsg" {
			// Removed in v7, retaining for backwards compatibility
			baseType.Fields["MdnsEnabled"] = NewFieldInfo("MdnsEnabled", "mdns_enabled", fields.Bool, "", false, false, false, "")
		}
	case resource.StructName == "DNSRecord":
		resource.ResourcePath = "static-dns"
	case resource.StructName == "FirewallZone":
		resource.ResourcePath = "firewall/zone"
		resource.FieldProcessor = func(name string, f *FieldInfo) error {
			// default_zone is server-computed/read-only. Sending it in the
			// create body makes UniFi Network 10.4.x reject the POST with
			// "Unrecognized field default_zone" (terraform-provider-unifi#310).
			// Make it a *bool with omitempty so an unset value is omitted.
			if name == "DefaultZone" {
				f.OmitEmpty = true
				f.IsPointer = true
			}
			// network_ids must always be present in the create body. The v2
			// firewall/zone POST returns HTTP 500 when the field is omitted
			// entirely, and the default codegen marks slices omitempty — which
			// drops an empty []string and breaks creating a zone that has no
			// networks assigned yet. Keep it always serialized so an empty list
			// is sent as "network_ids":[] (which the controller accepts).
			if name == "NetworkIDs" {
				f.OmitEmpty = false
			}
			return nil
		}
	case resource.StructName == "OSPFRouter":
		resource.ResourcePath = "ospf/router"
	case resource.StructName == "FirewallPolicy":
		resource.ResourcePath = "firewall-policies"
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
	case resource.StructName == "TrafficRoute":
		resource.ResourcePath = "trafficroutes"
	case resource.StructName == "Network":
		baseType.Fields["WANEgressQOSEnabled"] = NewFieldInfo("WANEgressQOSEnabled", "wan_egress_qos_enabled", fields.Bool, "", true, false, true, "")
		baseType.Fields["UPnPEnabled"] = NewFieldInfo("UPnPEnabled", "upnp_enabled", fields.Bool, "", true, false, true, "")
		baseType.Fields["UPnPWANInterface"] = NewFieldInfo("UPnPWANInterface", "upnp_wan_interface", fields.String, "", true, false, true, "")
		baseType.Fields["UPnPNatPMPEnabled"] = NewFieldInfo("UPnPNatPMPEnabled", "upnp_nat_pmp_enabled", fields.Bool, "", true, false, true, "")
		baseType.Fields["UPnPSecureMode"] = NewFieldInfo("UPnPSecureMode", "upnp_secure_mode", fields.Bool, "", true, false, true, "")
		baseType.Fields["IPAliases"] = NewFieldInfo("IPAliases", "ip_aliases", fields.String, "", true, true, false, "")
		baseType.Fields["DHCPRelayServers"] = NewFieldInfo("DHCPRelayServers", "dhcp_relay_servers", fields.String, "", true, true, false, "")
		baseType.Fields["WireguardInterfaceBindingModeIPVersion"] = NewFieldInfo(
			"WireguardInterfaceBindingModeIPVersion",
			"wireguard_interface_binding_mode_ip_version",
			fields.String,
			"v4|v6",
			true,
			false,
			true,
			"",
		)
	case resource.StructName == "Device":
		baseType.Fields["PortTable"] = NewFieldInfo("PortTable", "port_table", "[]DevicePortTable", "", true, false, false, "")
		baseType.Fields[" MAC"] = NewFieldInfo("MAC", "mac", fields.String, "", true, false, false, "")
		baseType.Fields["Adopted"] = NewFieldInfo("Adopted", "adopted", fields.Bool, "", false, false, false, "")
		baseType.Fields["Model"] = NewFieldInfo("Model", "model", fields.String, "", true, false, false, "")
		baseType.Fields["State"] = NewFieldInfo("State", "state", "DeviceState", "", false, false, false, "")
		baseType.Fields["Type"] = NewFieldInfo("Type", "type", fields.String, "", true, false, false, "")
		baseType.Fields["InformIP"] = NewFieldInfo("InformIP", "inform_ip", fields.String, "", true, false, false, "")
		baseType.Fields["IP"] = NewFieldInfo("IP", "ip", fields.String, "", true, false, false, "")
	case resource.StructName == "Client":
		baseType.Fields[" DisplayName"] = NewFieldInfo("DisplayName", "display_name", fields.String, "non-generated field", true, false, false, "")
		// The controller reports the client's most recent IP on /rest/user but
		// ace.jar has no schema for it; surface it as a read-only field.
		baseType.Fields["LastIP"] = NewFieldInfo("LastIP", "last_ip", fields.String, "non-generated field", true, false, false, "")
	case resource.StructName == "WLAN":
		// this field removed in v6, retaining for backwards compatibility
		baseType.Fields["WLANGroupID"] = NewFieldInfo("WLANGroupID", "wlangroup_id", fields.String, "", true, false, false, "")
	case resource.StructName == "BGPConfig":
		resource.ResourcePath = "bgp/config"
	}

	return resource
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
	return &FieldInfo{
		FieldName:           fieldName,
		JSONName:            jsonName,
		FieldType:           fieldType,
		FieldValidation:     fieldValidation,
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
	fmt.Printf("Usage: %s [OPTIONS] version\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func generateFromFields(fieldsDir, outDir string, unifiVersion *version.Version, generateSpec bool, specOutputFile string, stdout io.Writer, prepare func([]*ResourceInfo) error) error {
	fieldsFiles, err := os.ReadDir(fieldsDir)
	if err != nil {
		return fmt.Errorf("read fields directory: %w", err)
	}

	// Initialize specification generator
	specGen := NewSpecificationGenerator("unifi")
	type generationJob struct {
		resource           *ResourceInfo
		goFile, structName string
	}
	jobs := make([]generationJob, 0, len(fieldsFiles))
	settingSchema, settingSchemaErr := os.ReadFile(filepath.Join(fieldsDir, "Setting.json"))
	if settingSchemaErr != nil && !errors.Is(settingSchemaErr, os.ErrNotExist) {
		return fmt.Errorf("read Setting.json: %w", settingSchemaErr)
	}

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
			return fmt.Errorf("read field schema %s: %w", fieldsFile.Name(), err)
		}

		resource := NewResource(structName, urlPath)
		if err := SetResourceSourceIdentity(resource, fieldsFile.Name(), settingSchema); err != nil {
			return fmt.Errorf("set source identity for %s: %w", fieldsFile.Name(), err)
		}

		switch resource.StructName {
		case "Account":
			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				switch name {
				case "IP", "NetworkID":
					f.OmitEmpty = true
				}
				return nil
			}
		case "ChannelPlan":
			resource.FieldProcessor = func(name string, f *FieldInfo) error {
				switch name {
				case "Channel", "BackupChannel", "TxPower":
					if f.FieldType == fields.String {
						f.CustomUnmarshalType = fields.Number
					}
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
				case "Channel", "TxPower":
					// String fields in DeviceRadioTable that newer controllers
					// (UniFi 10.x) return as numbers; accept either form.
					if f.FieldType == fields.String {
						f.CustomUnmarshalType = fields.Number
					}
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
				case "WireguardInterfaceBindingModeVersion":
					// Older schemas lacked this field, so NewResource supplies a
					// compatibility definition. Newer schemas include it, but the
					// global IPVersion cleanup shortens the generated Go name.
					// Reuse the compatibility identity so the upstream definition
					// replaces it instead of emitting a duplicate JSON tag.
					f.FieldName = "WireguardInterfaceBindingModeIPVersion"
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
					"DHCPDTFTPServer", "DHCPDWins1", "DHCPDWins2", "DHCPDWPAdUrl", "DomainName", "DHCPDGateway", "DHCPDNtp1", "DHCPDNtp2":
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

		err = resource.processJSON(b)
		if err != nil {
			return fmt.Errorf("process field schema %s: %w", fieldsFile.Name(), err)
		}

		// Add fields not present in the JAR schema to nested types.
		if resource.StructName == "Device" {
			if portOverrides, ok := resource.Types["DevicePortOverrides"]; ok {
				portOverrides.Fields["TaggedNetworkIDs"] = NewFieldInfo("TaggedNetworkIDs", "tagged_networkconf_ids", fields.String, "", true, true, false, "")
			}
		}

		jobs = append(jobs, generationJob{resource: resource, goFile: goFile, structName: structName})
	}
	resources := make([]*ResourceInfo, len(jobs))
	for i := range jobs {
		resources[i] = jobs[i].resource
	}
	if prepare != nil {
		if err := prepare(resources); err != nil {
			return fmt.Errorf("prepare generated resources: %w", err)
		}
	}
	for _, job := range jobs {
		resource := job.resource
		specGen.AddResource(resource)
		code, err := resource.generateCode(false)
		if err != nil {
			return fmt.Errorf("generate %s: %w", resource.StructName, err)
		}
		targetDir := outDir
		if resource.IsSetting() {
			targetDir = filepath.Join(outDir, "settings")
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return fmt.Errorf("create generated output directory: %w", err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, job.goFile), []byte(code), 0o644); err != nil {
			return fmt.Errorf("write generated %s: %w", job.goFile, err)
		}
		if !resource.IsSetting() {
			implFilePath := filepath.Join(targetDir, strcase.ToSnake(job.structName)+".go")
			if _, statErr := os.Stat(implFilePath); errors.Is(statErr, os.ErrNotExist) {
				implCode, genErr := resource.generateCode(true)
				if genErr != nil {
					return fmt.Errorf("generate implementation scaffold %s: %w", resource.StructName, genErr)
				}
				if err := os.WriteFile(implFilePath, []byte(implCode), 0o644); err != nil {
					return fmt.Errorf("write implementation scaffold: %w", err)
				}
			} else if statErr != nil {
				return fmt.Errorf("stat implementation scaffold: %w", statErr)
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
		return fmt.Errorf("format generated version: %w", err)
	}

	if err := os.WriteFile(filepath.Join(outDir, "version.generated.go"), versionGo, 0o644); err != nil {
		return fmt.Errorf("write generated version: %w", err)
	}

	// Generate Terraform provider specification if requested
	if generateSpec {
		if err := specGen.WriteSpecification(specOutputFile); err != nil {
			return fmt.Errorf("write Terraform specification: %w", err)
		}
		fmt.Fprintf(stdout, "Generated specification: %s\n", specOutputFile)
	}

	fmt.Fprintf(stdout, "%s\n", outDir)
	return nil
}

func (r *ResourceInfo) IsSetting() bool {
	return strings.HasPrefix(r.StructName, "Setting")
}

func (r *ResourceInfo) IsDevice() bool {
	return r.StructName == "Device"
}

func (r *ResourceInfo) IsV2() bool {
	return slices.Contains([]string{
		"ApGroup",
		"BGPConfig",
		"DNSRecord",
		"FirewallPolicy",
		"FirewallZone",
		"Nat",
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
