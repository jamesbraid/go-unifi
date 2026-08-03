package fields

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// Preference is one [Resource.preference.<wire>] entry from
// overrides/fields.toml: an auto|manual mode field, and the wire names the
// controller takes ownership of while that mode is "auto".
//
// Ownership is not in the schema. The extracted validators describe each
// field on its own, and an auto|manual field looks like any other two-value
// enum, so the only way to learn what a mode owns is to write the same object
// twice -- once under each mode -- and diff what came back. That measurement
// is TestIntegrationPreferenceOwnership; these entries are its answer.
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
	Owns []string `toml:"owns"`

	// Measured names the controller build the set was measured against, so a
	// table that has fallen behind reads as stale rather than merely wrong.
	Measured string `toml:"measured"`

	// UOSExcludes lists entries of Owns that do NOT hold when the same
	// Network build runs inside UniFi OS, because the console owns the field
	// outright and neither mode reaches it.
	//
	// Measured is the Network version and does not separate these: UOS
	// 5.1.21 bundles Network 10.4.57 and reports that exact build, yet
	// data_retention_time_in_hours_for_5minutes_scale is pinned to 24 there
	// under both modes -- asking for 1 stores 24 -- while standalone 10.4.57
	// stores whatever manual asks. Same code, different product, different
	// answer, so one list cannot describe both.
	//
	// Only ever a subset of Owns. A field UOS pins is still owned by the
	// mode on standalone, and that measurement stays recorded rather than
	// being dropped to make the two agree.
	UOSExcludes []string `toml:"uos_excludes"`
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

// preferenceFile is the subset of overrides/fields.toml this package decodes.
// Unmatched keys are ignored, so the field and path entries the generator
// cares about pass straight through.
type preferenceFile struct {
	Preference map[string]Preference `toml:"preference"`
}

// LoadPreferences reads the preference tables out of overrides/fields.toml,
// keyed by resource struct name and then by the mode's key.
//
// A key is the mode's wire name, or a dotted path when the mode sits inside a
// sub-object ("port_overrides.setting_preference"). Such a key must be quoted
// in the file: an unquoted dotted key decodes as nested tables, without error,
// into an entry that owns nothing.
func LoadPreferences() (map[string]map[string]Preference, error) {
	root := ModuleRoot()
	if root == "" {
		return nil, fmt.Errorf("unable to locate the module root (go.mod)")
	}

	path := filepath.Join(root, "overrides", "fields.toml")
	var file map[string]preferenceFile
	md, err := toml.DecodeFile(path, &file)
	if err != nil {
		return nil, fmt.Errorf("unable to load %s: %w", path, err)
	}

	// A key no struct field claims is ignored rather than rejected, so a
	// misspelled property silently leaves the real one unset -- "onws"
	// yields an empty Owns, which reads here as a measured "owns nothing".
	//
	// Only preference properties can be judged from this side: this struct
	// deliberately models a subset of the file, so everything else is
	// undecoded by design and says nothing about correctness. A preference
	// property is Resource.preference.<mode>.<property>, four segments deep.
	var unknown []string
	for _, key := range md.Undecoded() {
		if len(key) == 4 && key[1] == "preference" {
			unknown = append(unknown, key.String())
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return nil, fmt.Errorf("%s has %d preference key(s) nothing reads: %s.\n\nEach is a "+
			"misspelling: the property it meant to set is still at its zero value, and an empty "+
			"owns set reads as a measured result rather than a mistake",
			path, len(unknown), strings.Join(unknown, ", "))
	}

	out := map[string]map[string]Preference{}
	for resource, entry := range file {
		if len(entry.Preference) > 0 {
			out[resource] = entry.Preference
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
