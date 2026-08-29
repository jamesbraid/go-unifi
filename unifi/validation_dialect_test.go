package unifi

import (
	"regexp"
	"sort"
	"testing"

	"github.com/ubiquiti-community/go-unifi/unifi/settings"
)

// The controller evaluates these patterns with java.util.regex, through
// Matcher.matches() with a single-argument Pattern.compile -- so a full-input
// match, with no flags, which leaves \d, \w and \s ASCII.
//
// Consumers outside Java have to reproduce that on a different engine. The
// capture happens here, so this is where a controller release that introduces
// a construct they cannot reproduce should say so -- not in a downstream
// build, months later, against a pattern nobody can explain.
//
// Both published tables are checked. The lookaround guard in
// TestFieldValidationPatternsCompile reads only the unifi one, which leaves
// the settings patterns unexamined by anything.

// untranslatableConstructs have no equivalent in the engines consumers
// actually use (RE2, .NET). A pattern using one cannot be mechanically
// translated at all, so it needs a decision rather than a rewrite.
//
// The probes are deliberately coarse -- a tripwire, not a parser. \}+ is a
// literal brace followed by a quantifier, not a possessive one, and would
// trip the first probe. A false positive is a loud failure somebody reads; a
// false negative ships a silently wrong validator.
var untranslatableConstructs = []struct {
	name   string
	re     *regexp.Regexp
	sample string
}{
	{"possessive quantifier", regexp.MustCompile(`(?:[*+?]|\})\+`), `[0-9]++`},
	{"character class intersection (&&)", regexp.MustCompile(`&&`), `[a-z&&[^bc]]`},
	{`\Q...\E quoted literal`, regexp.MustCompile(`\\[QE]`), `\Qax.25\E`},
	{`\p{java...} property`, regexp.MustCompile(`\\p\{java`), `\p{javaLowerCase}+`},
	{`\R any-linebreak`, regexp.MustCompile(`\\R`), `a\Rb`},
	{`\h horizontal whitespace`, regexp.MustCompile(`\\h`), `a\h+b`},
	{`\v vertical whitespace (a single vertical tab elsewhere)`, regexp.MustCompile(`\\v`), `a\v+b`},
}

// asciiClasses are the shorthands java.util.regex leaves ASCII without
// UNICODE_CHARACTER_CLASS, and that .NET makes Unicode-aware. A consumer must
// expand them or it accepts input the controller rejects.
var asciiClasses = regexp.MustCompile(`\\[dws]`)

// asciiClassFields pins which validators carry one. It is not a rule, it is a
// record: a consumer's translation table is built from exactly these, so a
// controller release that adds one has to reach them, and this is the only
// thing that would notice.
//
// Every entry here holds its \d or \w INSIDE a character class, which decides
// how the expansion is written: .NET character classes do not nest and Java's
// do, so [\d]+ -> [[0-9]]+ parses as intended in Java and, elsewhere, rejects
// "5" while accepting "5]".
var asciiClassFields = []string{
	"ChannelPlanRadioTable.tx_power",
	"Device.dot1x_fallback_networkconf_id",
	"Device.mgmt_network_id",
	"DevicePortOverrides.portconf_id",
	"DeviceRadioTable.tx_power",
	"DpiGroup.dpiapp_ids",
	"FirewallRule.dst_firewallgroup_ids",
	"FirewallRule.dst_networkconf_id",
	"FirewallRule.src_firewallgroup_ids",
	"FirewallRule.src_networkconf_id",
	"Network.dpigroup_id",
	"Network.local_vpn_networkconf_ids",
	"Routing.static-route_interface",
	"SpatialRecordPosition.x",
	"SpatialRecordPosition.y",
	"SpatialRecordPosition.z",
	"WLAN.dpigroup_id",
	"settings.SettingGlobalSwitch.dot1x_fallback_networkconf_id",
	"settings.SettingGuestAccess.expire",
}

// eachPublishedPattern walks both generated tables. Settings keys carry a
// "settings." prefix so the two namespaces cannot collide on a shared type
// name.
func eachPublishedPattern(fn func(key, pattern string)) {
	walk := func(prefix string, tbl map[string]map[string]string) {
		for typeName, byWire := range tbl {
			for wire, pattern := range byWire {
				fn(prefix+typeName+"."+wire, pattern)
			}
		}
	}
	walk("", FieldValidationPatterns)
	walk("settings.", settings.FieldValidationPatterns)
}

func TestNoUntranslatableValidatorConstructs(t *testing.T) {
	checked := 0
	eachPublishedPattern(func(key, pattern string) {
		checked++
		for _, c := range untranslatableConstructs {
			if c.re.MatchString(pattern) {
				t.Errorf("%s uses %s, which consumers outside Java cannot reproduce\n\n"+
					"pattern: %s\n\n"+
					"Do not rewrite it to suit another engine -- an equivalent this project "+
					"invents can reject input the controller accepts. Tell the consumers, and "+
					"record the decision here.", key, c.name, pattern)
			}
		}
	})
	if checked == 0 {
		t.Fatal("no patterns were checked")
	}
	t.Logf("checked %d patterns for constructs with no equivalent elsewhere", checked)
}

func TestASCIIClassFieldsArePinned(t *testing.T) {
	expected := map[string]bool{}
	for _, key := range asciiClassFields {
		expected[key] = true
	}

	found := map[string]bool{}
	eachPublishedPattern(func(key, pattern string) {
		if !asciiClasses.MatchString(pattern) {
			return
		}
		found[key] = true
		if !expected[key] {
			t.Errorf("%s now uses \\d, \\w or \\s and is not pinned\n\n"+
				"pattern: %s\n\n"+
				"These are ASCII on the controller and Unicode-aware on other engines, so a "+
				"consumer translates them by hand. Add it here and tell them, or they accept "+
				"input the controller rejects.", key, pattern)
		}
	})

	var stale []string
	for key := range expected {
		if !found[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		t.Errorf("asciiClassFields pins %s, but its pattern no longer uses \\d, \\w or \\s; "+
			"the entry is stale", key)
	}

	if len(found) == 0 {
		t.Fatal("no patterns were checked")
	}
	t.Logf("%d validators need ASCII shorthand expansion", len(found))
}

// TestUntranslatableProbesFire keeps the tripwire honest. Every probe above
// currently matches nothing, so a typo in one would never be noticed -- it
// would report a clean sweep forever. Each probe has to recognise an example
// of the construct it is named for.
func TestUntranslatableProbesFire(t *testing.T) {
	for _, c := range untranslatableConstructs {
		if !c.re.MatchString(c.sample) {
			t.Errorf("the %s probe does not match its own example %q; it would never fire",
				c.name, c.sample)
		}
	}
}
