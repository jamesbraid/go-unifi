package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

// TestPreferenceDescriptionsNameRealAttributes checks the committed
// specification: every attribute a preference description tells a
// practitioner to set must be an attribute the specification emits.
//
// The descriptions exist to make an invisible relationship followable, and a
// name that does not resolve is worse than no description -- it sends the
// reader looking for an attribute that was never generated. This used to
// happen for every field whose Go name splits differently from its wire
// name: the spec said "Set ipv_6_setting_preference to manual" while
// emitting ipv6_setting_preference, and listed dhc_pguard_enabled,
// u_pn_p_lan_enabled and wandns_1 as owned fields, none of which exist.
//
// Reads the artifact rather than rebuilding it, so it also catches a
// specification.json committed out of step with the generator.
func TestPreferenceDescriptionsNameRealAttributes(t *testing.T) {
	root := fields.ModuleRoot()
	if root == "" {
		t.Skip("module root not found")
	}
	raw, err := os.ReadFile(filepath.Join(root, "specification.json"))
	if err != nil {
		t.Skipf("no specification.json to check: %v", err)
	}
	spec := string(raw)

	emitted := map[string]bool{}
	for _, m := range regexp.MustCompile(`"name":\s*"([a-z0-9_]+)"`).FindAllStringSubmatch(spec, -1) {
		emitted[m[1]] = true
	}
	if len(emitted) == 0 {
		t.Fatal("no attribute names found in specification.json; the check would pass vacuously")
	}

	cited := map[string]bool{}
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`Set ([a-z0-9_]+) to`),
		regexp.MustCompile(`Ignored while ([a-z0-9_]+) is`),
	} {
		for _, m := range re.FindAllStringSubmatch(spec, -1) {
			cited[m[1]] = true
		}
	}
	// The owned-field list inside a mode's own description.
	owned := regexp.MustCompile(`without reporting it: ([^"]*?)\. Measured`)
	splitter := regexp.MustCompile(`,\s*`)
	for _, m := range owned.FindAllStringSubmatch(spec, -1) {
		for _, wire := range splitter.Split(m[1], -1) {
			if wire != "" {
				cited[wire] = true
			}
		}
	}
	if len(cited) == 0 {
		t.Fatal("no preference descriptions found; either they stopped being emitted or the " +
			"wording changed and this check is now looking for nothing")
	}

	var missing []string
	for name := range cited {
		if !emitted[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d name(s) cited in preference descriptions are not attributes the spec emits: %v\n\n"+
			"Attribute names come from the wire name (JSONName). A description naming anything else "+
			"points a practitioner at something that does not exist.", len(missing), missing)
	}
	t.Logf("%d cited name(s), all emitted", len(cited))
}
