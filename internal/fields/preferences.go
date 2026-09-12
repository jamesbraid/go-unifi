package fields

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/ubiquiti-community/go-unifi/internal/behavior"
)

// Preference is one auto|manual mode field and the wire names the controller
// takes ownership of while that mode is "auto".
//
// Ownership is not in the schema. The extracted validators describe each
// field on its own, and an auto|manual field looks like any other two-value
// enum, so the only way to learn what a mode owns is to write the same object
// twice -- once under each mode -- and diff what came back. That measurement
// is TestIntegrationPreferenceOwnership (plus the device port-override probe
// for the one mode that needs an adopted device); the ownership and uos_pins
// sections of schemas/behavior.json are its answer.
//
// The type lives here because both sides need the same shape: the generator
// turns these entries into client code and provider schema metadata, and the
// integration test reads them back to check the controller still agrees.
type Preference struct {
	// Owns lists wire names on the same object that the controller
	// overwrites with its own values while the mode field is "auto". It
	// answers rc: ok and reports nothing, so a caller finds out from the
	// next read, or from a downstream diff, or not at all.
	//
	// An empty list is a result, not a gap: it records a mode field that was
	// measured and owns nothing.
	Owns []string

	// Measured names the controller build the set was measured against, so a
	// table that has fallen behind reads as stale rather than merely wrong.
	Measured string

	// UOSExcludes lists entries of Owns that do NOT hold when the Network
	// app runs inside UniFi OS, because the console owns the field outright
	// and neither mode reaches it. The artifact's uos_pins section, stamped
	// with the build the UOS harness bundled (its uos_network_version).
	//
	// Only ever a subset of Owns. A field UOS pins is still owned by the
	// mode on standalone, and that measurement stays recorded rather than
	// being dropped to make the two agree.
	UOSExcludes []string
}

// OwnsOn returns the wire names the mode owns on one harness.
//
// uos selects the UniFi OS answer, which is Owns minus the fields the console
// pins. Everything else gets Owns unchanged.
func (p Preference) OwnsOn(uos bool) []string {
	if !uos || len(p.UOSExcludes) == 0 {
		return p.Owns
	}
	out := make([]string, 0, len(p.Owns))
	for _, wire := range p.Owns {
		if !slices.Contains(p.UOSExcludes, wire) {
			out = append(out, wire)
		}
	}
	return out
}

// LoadPreferences returns the measured ownership tables from
// schemas/behavior.json, keyed by resource struct name and then by the
// mode's key -- the mode's wire name, or a dotted path when the mode sits
// inside a sub-object ("port_overrides.setting_preference").
//
// The probes write both sections under BEHAVIOR_WRITE=1 and stamp the builds
// they ran against. A uos_pins entry for a mode the ownership section does
// not cover is a keying mistake and errors, so the two sections cannot drift
// apart in silence; pins naming unowned fields are caught downstream by the
// generator's UOSExcludes-subset-of-Owns check.
func LoadPreferences() (map[string]map[string]Preference, error) {
	root := ModuleRoot()
	if root == "" {
		return nil, fmt.Errorf("unable to locate the module root (go.mod)")
	}
	a, _, err := behavior.Load(root)
	if err != nil {
		return nil, err
	}

	out := map[string]map[string]Preference{}
	for resource, byKey := range a.Ownership {
		out[resource] = make(map[string]Preference, len(byKey))
		for key, owns := range byKey {
			out[resource][key] = Preference{Owns: owns, Measured: a.ControllerVersion}
		}
	}
	for resource, byKey := range a.UOSPins {
		for key, pins := range byKey {
			entry, covered := out[resource][key]
			if !covered {
				return nil, fmt.Errorf("%s records uos_pins for %s.%s, which its ownership section "+
					"does not measure; re-run the ownership sweep on both harnesses with "+
					"BEHAVIOR_WRITE=1 instead of editing the file", behavior.Path, resource, key)
			}
			entry.UOSExcludes = pins
			out[resource][key] = entry
		}
	}
	return out, nil
}

// ModuleRoot walks up from the working directory to the enclosing go.mod,
// returning "" when there is none.
func ModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return ModuleRootFrom(dir)
}

// ModuleRootFrom walks up from dir to the enclosing go.mod.
func ModuleRootFrom(dir string) string {
	if dir == "" {
		return ""
	}
	dir = filepath.Clean(dir)
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
