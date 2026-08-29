package unifi

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

var updatePurposeShape = flag.Bool("update-purpose-shape", false,
	"rewrite testdata/purpose_wire_shape.txt from the current encoder")

const purposeWireShapePath = "testdata/purpose_wire_shape.txt"

// TestNetworkPurposeWireShape pins what each Network purpose encoder puts on
// the wire for an object the caller left alone.
//
// TestGeneratedWriteShape covers the same ground for the generated types by
// reading their struct tags. It cannot cover Network: the purpose encoders
// are hand-written, they re-declare their own tags, and a field's tag there
// can differ from the one on Network itself. So the only honest way to ask
// what a purpose sends unconditionally is to encode a zero Network and look.
//
// This is the gap that let a real change pass unnoticed. remote_vpn_subnets
// lost its omitempty in the site-to-site encoder -- a wire contract change,
// since the key now appears on every write -- and apidiff reported no
// wire-surface change at all, because its baseline only sees the generated
// marshalers. With this file it sees both.
func TestNetworkPurposeWireShape(t *testing.T) {
	current := purposeWireShape(t)

	if *updatePurposeShape {
		if err := os.WriteFile(purposeWireShapePath, []byte(strings.Join(current, "\n")+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", purposeWireShapePath, err)
		}
		t.Logf("recorded %d purpose/field pairs", len(current))
		return
	}

	data, err := os.ReadFile(purposeWireShapePath)
	if err != nil {
		t.Fatalf("read %s: %v\n\nRecord it with: go test ./unifi/ -run %s -update-purpose-shape",
			purposeWireShapePath, err, t.Name())
	}
	var accepted []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			accepted = append(accepted, line)
		}
	}

	added, removed := wireShapeDelta(accepted, current)
	if len(added) > 0 {
		t.Errorf("%d purpose field(s) joined the unconditional write shape:\n  %s\n\n"+
			"Each is now sent on every write of that purpose where the caller left it unset, "+
			"and the controller will store it. If that is what the field needs -- a list the "+
			"controller requires, or one that can only be cleared with an explicit empty -- "+
			"re-record with -update-purpose-shape. If not, give it omitempty.",
			len(added), strings.Join(added, "\n  "))
	}
	if len(removed) > 0 {
		t.Errorf("%d purpose field(s) left the unconditional write shape:\n  %s\n\n"+
			"The key no longer reaches the controller for a caller that did not set it, so "+
			"whatever it holds now survives writes that used to overwrite it. Re-record with "+
			"-update-purpose-shape if that is intended.",
			len(removed), strings.Join(removed, "\n  "))
	}
}

// purposeWireShape encodes a zero Network for every purpose and returns the
// keys that reached the wire, as "purpose field" pairs.
func purposeWireShape(t *testing.T) []string {
	t.Helper()

	var out []string
	for _, purpose := range NetworkPurposes {
		n := &Network{Purpose: purpose}
		raw, err := json.Marshal(n)
		if err != nil {
			t.Fatalf("marshal a zero Network for purpose %q: %v", purpose, err)
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("unmarshal the encoding for purpose %q: %v", purpose, err)
		}
		for wire := range payload {
			out = append(out, fmt.Sprintf("%s %s", purpose, wire))
		}
	}
	slices.Sort(out)
	return out
}

func wireShapeDelta(accepted, current []string) (added, removed []string) {
	inAccepted := make(map[string]bool, len(accepted))
	for _, line := range accepted {
		inAccepted[line] = true
	}
	inCurrent := make(map[string]bool, len(current))
	for _, line := range current {
		inCurrent[line] = true
		if !inAccepted[line] {
			added = append(added, line)
		}
	}
	for _, line := range accepted {
		if !inCurrent[line] {
			removed = append(removed, line)
		}
	}
	return added, removed
}
