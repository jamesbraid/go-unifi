package unifi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// THE PER-PURPOSE ENCODERS MUST NOT ADD omitempty THE STRUCT DOES NOT HAVE.
//
// Network.MarshalJSON dispatches to one of seven alias structs, each
// re-declaring the subset of fields its purpose sends. A tag copied with an
// extra ,omitempty makes the encoder drop that field's zero value -- and for a
// bool the zero value is `false`, which is a setting the caller asked for.
//
// MEASURED, not theorised: a site-vpn Network with IPSecPfs=false emits no
// ipsec_pfs at all, so PFS can be turned on and never off, by this library and
// therefore by every caller of it.
//
// This test compares each alias tag against the Network field of the same json
// name and fails on any bool that has drifted.
func TestPurposeEncodersDoNotAddOmitemptyToBools(t *testing.T) {
	structTags, structTypes := networkStructTags(t)
	drifted := map[string][]string{}

	for encoder, tags := range purposeEncoderTags(t) {
		for _, tag := range tags {
			name, aliasOmit := splitJSONTag(tag)
			structOmit, ok := structTags[name]
			if !ok || structTypes[name] != "bool" {
				continue
			}
			if aliasOmit && !structOmit {
				drifted[name] = append(drifted[name], encoder)
			}
		}
	}

	for name, encoders := range drifted {
		sort.Strings(encoders)
		t.Errorf("%s is tagged omitempty in %v but not on Network, so a false is dropped "+
			"and the setting can be turned on and never off", name, encoders)
	}
}

// THE STRING DRIFT IS RECORDED RATHER THAN FIXED, and the difference is
// whether the zero value is legal.
//
// A false is always a meaningful bool. An empty string may not be: several
// controller fields reject "" and are omitted deliberately, which is why the
// provider has an optStr helper doing the same thing on its side. Clearing a
// DHCP DNS server is a real thing a practitioner does, so these are probably
// defects too -- but "probably" is not a reason to change a request shape, and
// deciding needs a controller.
//
// Listing them keeps the check useful in the meantime: a NEW string drift
// fails here, and the day one of these is measured it comes off the list.
func TestKnownStringOmitemptyDriftIsUnchanged(t *testing.T) {
	awaitingControllerMeasurement := map[string]bool{
		"dhcpd_boot_server": true,
		"dhcpd_dns_1":       true,
		"dhcpd_dns_2":       true,
		"dhcpd_dns_3":       true,
		"dhcpd_dns_4":       true,
		"mac_override":      true,
	}

	structTags, structTypes := networkStructTags(t)
	found := map[string]bool{}
	for encoder, tags := range purposeEncoderTags(t) {
		for _, tag := range tags {
			name, aliasOmit := splitJSONTag(tag)
			structOmit, ok := structTags[name]
			if !ok || structTypes[name] != "string" || !aliasOmit || structOmit {
				continue
			}
			found[name] = true
			if !awaitingControllerMeasurement[name] {
				t.Errorf("%s in %s adds omitempty the struct does not have; if clearing it is "+
					"a thing a caller does, the empty string never reaches the controller",
					name, encoder)
			}
		}
	}
	for name := range awaitingControllerMeasurement {
		if !found[name] {
			t.Errorf("%s is listed as awaiting measurement but no encoder drifts on it any "+
				"more; take it off the list", name)
		}
	}
}

// networkStructTags reads Network's own json names, whether each carries
// omitempty, and its Go type.
func networkStructTags(t *testing.T) (map[string]bool, map[string]string) {
	t.Helper()
	omit := map[string]bool{}
	types := map[string]string{}
	value := reflect.TypeOf(Network{})
	for i := range value.NumField() {
		field := value.Field(i)
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name, hasOmit := splitJSONTag(tag)
		omit[name] = hasOmit
		types[name] = field.Type.String()
	}
	if len(omit) == 0 {
		t.Fatal("Network has no json tags, so both tests above would pass vacuously")
	}
	return omit, types
}

// purposeEncoderTags returns every json tag declared inside each marshalX
// method, read from the source because the alias structs are local to their
// functions and reflection cannot reach them.
func purposeEncoderTags(t *testing.T) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "network_encode.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing network_encode.go: %v", err)
	}
	out := map[string][]string{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || !strings.HasPrefix(fn.Name.Name, "marshal") {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			field, ok := n.(*ast.Field)
			if !ok || field.Tag == nil {
				return true
			}
			raw, err := strconv.Unquote(field.Tag.Value)
			if err != nil {
				return true
			}
			if tag := reflect.StructTag(raw).Get("json"); tag != "" {
				out[fn.Name.Name] = append(out[fn.Name.Name], tag)
			}
			return true
		})
	}
	if len(out) < 7 {
		t.Fatalf("found %d purpose encoders, want the seven purposes; the parse missed some "+
			"and both tests would check less than they claim", len(out))
	}
	return out
}

func splitJSONTag(tag string) (string, bool) {
	name, rest, _ := strings.Cut(tag, ",")
	return name, strings.Contains(rest, "omitempty")
}
