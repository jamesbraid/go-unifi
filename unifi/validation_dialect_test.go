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

// shorthandsOutsideAClass returns the ASCII shorthands in pattern that sit
// outside any [...] class. The scan tracks class depth rather than matching,
// because whether an occurrence is inside one is the whole question.
func shorthandsOutsideAClass(pattern string) []string {
	var out []string
	inClass := false
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; {
		case c == '\\' && i+1 < len(pattern):
			if next := pattern[i+1]; !inClass && (next == 'd' || next == 'w' || next == 's') {
				out = append(out, `\`+string(next))
			}
			i++
		case c == '[' && !inClass:
			inClass = true
		case c == ']' && inClass:
			inClass = false
		}
	}
	return out
}

// TestPinnedShorthandsAreAllInsideACharacterClass checks the claim
// asciiClassFields makes about itself.
//
// The comment there says every pinned pattern holds its \d or \w inside a
// character class, and that fact decides how a consumer writes the expansion:
// inside a class it must be 0-9, because .NET classes do not nest and [[0-9]]
// then rejects "5" and accepts "5]". Nothing checked the claim -- the pin
// records which fields carry a shorthand, not where.
//
// It matters beyond keeping a comment honest. A consumer translating these
// patterns for another engine writes the expansion differently inside a class
// than outside one, and no pattern here has ever been outside one -- so a
// consumer's outside-a-class handling cannot be exercised by this corpus at
// all. The first controller release to publish a bare \d makes that case real
// for every consumer at once.
//
// This test is the notification, NOT the protection. A consumer's own tests
// are what prove its translation correct in both positions, and they have to
// stand alone: this guard sees the corpus, not their code, and it cannot fail
// on their behalf. Nobody downstream should relax a local test because this
// exists. What it buys is that the case is announced when it arrives, rather
// than discovered afterwards by a translation that quietly mis-validated.
func TestPinnedShorthandsAreAllInsideACharacterClass(t *testing.T) {
	pinned := map[string]bool{}
	for _, key := range asciiClassFields {
		pinned[key] = true
	}

	checked := 0
	eachPublishedPattern(func(key, pattern string) {
		if !pinned[key] {
			return
		}
		checked++
		if loose := shorthandsOutsideAClass(pattern); len(loose) > 0 {
			t.Errorf("%s has %v outside a character class\n\n"+
				"pattern: %s\n\n"+
				"Every pinned pattern until now kept its shorthand inside a class, so a "+
				"consumer expanding \\d to 0-9 (no brackets, because .NET classes do not "+
				"nest) was always right. A bare one needs the bracketed form instead, and "+
				"that branch of a consumer's translation has never run. Tell the consumers.",
				key, loose, pattern)
		}
	})

	if checked != len(asciiClassFields) {
		t.Errorf("checked %d of %d pinned fields; the pin and the tables disagree",
			checked, len(asciiClassFields))
	}
}

// The scan has to be right in both directions or the test above is decoration.
func TestShorthandsOutsideAClass(t *testing.T) {
	cases := []struct {
		pattern string
		want    int
	}{
		{`[\d]+|auto`, 0},
		{`[\d\w-]+|^$`, 0},
		{`\d+`, 1},        // bare, the case that has never appeared
		{`\d+|[\w]`, 1},   // one of each
		{`[a-z]\d`, 1},    // after a class has closed
		{`\\d`, 0},        // an escaped backslash, then a literal d
		{`[\d]\w[\s]`, 1}, // only the middle one is loose
		{`no shorthands here`, 0},
	}
	for _, c := range cases {
		if got := len(shorthandsOutsideAClass(c.pattern)); got != c.want {
			t.Errorf("shorthandsOutsideAClass(%q) found %d, want %d", c.pattern, got, c.want)
		}
	}
}
