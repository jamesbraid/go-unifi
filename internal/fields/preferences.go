package fields

import (
	"fmt"
	"os"
	"path/filepath"

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
}

// preferenceFile is the subset of overrides/fields.toml this package decodes.
// Unmatched keys are ignored, so the field and path entries the generator
// cares about pass straight through.
type preferenceFile struct {
	Preference map[string]Preference `toml:"preference"`
}

// LoadPreferences reads the preference tables out of overrides/fields.toml,
// keyed by resource struct name and then by mode-field wire name.
func LoadPreferences() (map[string]map[string]Preference, error) {
	root := ModuleRoot()
	if root == "" {
		return nil, fmt.Errorf("unable to locate the module root (go.mod)")
	}

	path := filepath.Join(root, "overrides", "fields.toml")
	var file map[string]preferenceFile
	if _, err := toml.DecodeFile(path, &file); err != nil {
		return nil, fmt.Errorf("unable to load %s: %w", path, err)
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
