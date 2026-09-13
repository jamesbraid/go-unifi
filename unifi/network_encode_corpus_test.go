package unifi

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var updateEncodeCorpus = flag.Bool("update-encode-corpus", false,
	"rewrite testdata/network_encode_corpus.txt from the current encoder")

const networkEncodeCorpusPath = "testdata/network_encode_corpus.txt"

// TestNetworkEncodeCorpus pins the exact bytes Network.MarshalJSON produces
// for a matrix of networks: every purpose, sparse and dense field sets, and
// the inputs that trigger each derivation the encoder performs (the
// create-only DHCP range, the vlan_enabled inference, the WAN ipv6_enabled
// synthesis, the empty-pointer handling).
//
// cmd/apidiff compares which keys a zero object sends per purpose, and the
// generated types' always-serialized set, against the last release. Neither
// pins the full
// output -- key order, derived values, what a populated pointer does -- so a
// restructuring of the encoder could shift bytes without either noticing.
// This golden holds the whole surface still: any change to what any purpose
// puts on the wire, for sparse or dense input, has to show up as a diff of
// this file and say why.
func TestNetworkEncodeCorpus(t *testing.T) {
	corpus := networkEncodeCorpus(t)

	names := make([]string, 0, len(corpus))
	for name := range corpus {
		names = append(names, name)
	}
	sort.Strings(names)

	if *updateEncodeCorpus {
		var sb strings.Builder
		for _, name := range names {
			fmt.Fprintf(&sb, "%s\t%s\n", name, corpus[name])
		}
		if err := os.WriteFile(networkEncodeCorpusPath, []byte(sb.String()), 0o644); err != nil {
			t.Fatalf("write %s: %v", networkEncodeCorpusPath, err)
		}
		t.Logf("recorded %d corpus entries", len(corpus))
		return
	}

	raw, err := os.ReadFile(networkEncodeCorpusPath)
	if err != nil {
		t.Fatalf("read %s: %v\n\nRecord it with: go test ./unifi/ -run %s -update-encode-corpus",
			networkEncodeCorpusPath, err, t.Name())
	}

	golden := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSuffix(string(raw), "\n"), "\n") {
		name, output, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s: malformed line %q", networkEncodeCorpusPath, line)
		}
		golden[name] = output
	}

	for _, name := range names {
		if want, ok := golden[name]; !ok {
			t.Errorf("case %q is not in the golden file; if the case is new, record it with -update-encode-corpus", name)
		} else if corpus[name] != want {
			t.Errorf("case %q: encoder output changed.\n got: %s\nwant: %s\n\n"+
				"This is a wire-format change. If it is intentional, update the golden with "+
				"-update-encode-corpus and explain the change in the commit.", name, corpus[name], want)
		}
	}
	for name := range golden {
		if _, ok := corpus[name]; !ok {
			t.Errorf("golden case %q is no longer produced; remove it with -update-encode-corpus", name)
		}
	}
}

// networkEncodeCorpus marshals every corpus case and returns name -> output.
func networkEncodeCorpus(t *testing.T) map[string]string {
	t.Helper()

	out := map[string]string{}
	add := func(name string, n *Network) {
		t.Helper()
		if _, dup := out[name]; dup {
			t.Fatalf("duplicate corpus case %q", name)
		}
		data, err := json.Marshal(n)
		if err != nil {
			t.Fatalf("marshal corpus case %q: %v", name, err)
		}
		out[name] = string(data)
	}

	for _, purpose := range NetworkPurposes {
		// The caller set nothing: the sparse floor of every purpose.
		add(purpose+"/zero", &Network{Purpose: purpose})

		// Every field populated: the dense ceiling. An update (the _id is
		// set) and a create, which is the only shape where the corporate and
		// guest encoders derive a DHCP range for an unset dhcpd_start/stop.
		dense := denseCorpusNetwork(purpose)
		add(purpose+"/dense-update", dense)

		denseCreate := denseCorpusNetwork(purpose)
		denseCreate.ID = ""
		denseCreate.DHCPDStart = nil
		denseCreate.DHCPDStop = nil
		add(purpose+"/dense-create", denseCreate)

		// Every *string pointing at "": the read-modify-write shape that
		// separates fields dropped when empty from the measured clearable
		// slots that must reach the wire as "".
		add(purpose+"/empty-pointers", newPointerStringNetwork(purpose, ""))
		add(purpose+"/sentinel-pointers", newPointerStringNetwork(purpose, "sentinel"))

		// The create-only DHCP range derivation, from both sides: a create
		// with only a subnet derives, an update with the same input must not.
		add(purpose+"/subnet-create", &Network{Purpose: purpose, IPSubnet: strPtr("192.168.7.0/24")})
		add(purpose+"/subnet-update", &Network{ID: "aabbccddeeff001122334455", Purpose: purpose, IPSubnet: strPtr("192.168.7.0/24")})
		add(purpose+"/subnet-create-explicit-range", &Network{
			Purpose:    purpose,
			IPSubnet:   strPtr("192.168.7.0/24"),
			DHCPDStart: strPtr("192.168.7.10"),
			DHCPDStop:  strPtr("192.168.7.20"),
		})
		// The /29 branch and the too-small refusal of the range calculation.
		add(purpose+"/subnet-create-slash29", &Network{Purpose: purpose, IPSubnet: strPtr("10.9.9.0/29")})
		add(purpose+"/subnet-create-slash30", &Network{Purpose: purpose, IPSubnet: strPtr("10.9.9.0/30")})
	}

	// The vlan_enabled inference: a VLAN id with the flag left off is turned
	// on for vlan-only, and only there.
	vlan := int64(7)
	zero := int64(0)
	add("vlan-only/vlan-id-without-flag", &Network{Purpose: PurposeVLANOnly, VLAN: &vlan})
	add("vlan-only/vlan-id-with-flag", &Network{Purpose: PurposeVLANOnly, VLAN: &vlan, VLANEnabled: true})
	add("vlan-only/vlan-zero", &Network{Purpose: PurposeVLANOnly, VLAN: &zero})
	add("corporate/vlan-id-without-flag", &Network{Purpose: PurposeCorporate, VLAN: &vlan})

	// The WAN ipv6_enabled synthesis reads wan_type_v6 three ways: unset,
	// explicitly disabled, and a real value.
	add("wan/wan-type-v6-empty", &Network{Purpose: PurposeWAN, WANTypeV6: strPtr("")})
	add("wan/wan-type-v6-disabled", &Network{Purpose: PurposeWAN, WANTypeV6: strPtr("disabled")})
	add("wan/wan-type-v6-dhcpv6", &Network{Purpose: PurposeWAN, WANTypeV6: strPtr("dhcpv6")})

	// The DHCP guard slots ride along unconditionally.
	add("corporate/dhcpguard", &Network{Purpose: PurposeCorporate, DHCPguardEnabled: true, DHCPDIP1: "10.0.0.5"})

	// Site-to-site subnet lists reach the wire as [] when unset and as-is
	// when set, including an explicit empty.
	add("site-vpn/subnets", &Network{
		Purpose:           PurposeSiteVPN,
		RemoteVPNSubnets:  []string{"10.1.0.0/24", "10.2.0.0/24"},
		RemoteSiteSubnets: []string{},
	})

	return out
}

// denseCorpusNetwork returns a Network with every field populated non-zero,
// the same shape the coverage tests use, with a parseable subnet so the
// range calculation has real input.
func denseCorpusNetwork(purpose string) *Network {
	var n Network
	populateNonZero(reflect.ValueOf(&n).Elem(), 10)
	n.Purpose = purpose
	subnet := "10.0.0.0/24"
	n.IPSubnet = &subnet
	return &n
}
