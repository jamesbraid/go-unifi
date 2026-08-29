package unifi

import (
	"regexp"
	"strings"
	"testing"
)

// perlOnlyPatterns are the controller validators Go cannot compile, because
// they use lookahead or lookbehind and Go's regexp is RE2, which has neither.
//
// They are published verbatim rather than rewritten. An equivalent without
// lookaround would be a rule this project invented, and a rule that differs
// from the controller's by a little rejects input the controller accepts --
// the failure this table exists to prevent. A consumer that compiles every
// pattern needs to know which four to skip, and that is what this list is
// for.
//
// All four use lookaround only to assert an overall length around an
// alternation, which is why they resist a mechanical rewrite: the length
// applies to the whole match, not to any one branch.
var perlOnlyPatterns = map[string][]string{
	"DHCPOption": {"code"},
	"Network":    {"dhcpd_boot_server", "domain_name", "pptpc_server_ip"},
}

// TestFieldValidationPatternsCompile checks every published pattern against
// Go's regexp, so a controller release that adds another unusable one says so
// rather than surfacing as a compile failure in somebody else's generator.
func TestFieldValidationPatternsCompile(t *testing.T) {
	expected := map[string]bool{}
	for typeName, wires := range perlOnlyPatterns {
		for _, wire := range wires {
			expected[typeName+"."+wire] = true
		}
	}

	seen := map[string]bool{}
	checked := 0
	for typeName, byWire := range FieldValidationPatterns {
		for wire, pattern := range byWire {
			checked++
			key := typeName + "." + wire
			_, err := regexp.Compile(pattern)
			switch {
			case err == nil && expected[key]:
				t.Errorf("%s compiles now; take it off perlOnlyPatterns", key)
			case err != nil && !expected[key]:
				t.Errorf("%s does not compile under Go's regexp: %v\n\n"+
					"pattern: %s\n\n"+
					"Do not rewrite the controller's validator to suit Go -- an equivalent "+
					"this project invents can reject input the controller accepts. Add it to "+
					"perlOnlyPatterns so consumers know to skip it.", key, err, pattern)
			}
			if err != nil {
				seen[key] = true
			}
		}
	}

	for key := range expected {
		if !seen[key] {
			t.Errorf("perlOnlyPatterns lists %s, but no such pattern failed to compile; "+
				"the entry is stale", key)
		}
	}
	if checked == 0 {
		t.Fatal("no patterns were checked")
	}
	t.Logf("checked %d patterns; %d need lookaround Go does not have", checked, len(seen))
}

// TestProtocolNamesAreMatchedLiterally guards the one protocol name carrying
// a character the regex would otherwise read as syntax.
//
// The firewall policy protocol pattern is assembled from names measured
// against the controller, and one of them is "ax.25". Joined unescaped, its
// dot matches any character, so the pattern accepted axX25 -- a strict
// superset of what the controller takes, and a value it rejects.
func TestProtocolNamesAreMatchedLiterally(t *testing.T) {
	pattern, ok := FieldValidationPatterns["FirewallPolicy"]["protocol"]
	if !ok {
		t.Fatal("FirewallPolicy.protocol has no published pattern")
	}
	re, err := regexp.Compile("^(?:" + pattern + ")$")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	for _, accepted := range []string{"ax.25", "all", "tcp", "icmp", "icmpv6", "6", "58"} {
		if !re.MatchString(accepted) {
			t.Errorf("%q is rejected; it was measured as accepted by the controller", accepted)
		}
	}
	for _, refused := range []string{"axX25", "ax25", "not-a-protocol", "icmpvX"} {
		if re.MatchString(refused) {
			t.Errorf("%q matches; a name's punctuation must be matched literally, not as regex syntax", refused)
		}
	}

	// Every literal name in the alternation should be matched as itself.
	for _, token := range strings.Split(pattern, "|") {
		if strings.ContainsAny(token, "()[]{}*+?^$") {
			continue // the numeric range, not a name
		}
		name := strings.ReplaceAll(token, `\.`, ".")
		if !re.MatchString(name) {
			t.Errorf("the pattern does not match its own name token %q", name)
		}
	}
}
