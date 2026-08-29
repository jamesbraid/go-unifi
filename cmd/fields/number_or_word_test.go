package main

import "testing"

// The generated output of this rule is committed, so every other test would
// still pass if admitsNumberOrWord quietly stopped matching anything -- the
// shims are already baked into the files on disk. Nothing would notice until
// a controller release added a field that needed one, which is the case the
// rule exists for.
//
// So it needs its own positive control: real patterns it must accept, and
// real patterns it must refuse.
func TestAdmitsNumberOrWord(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		want    bool
	}{
		// Accept: a bare integer in one branch, a word in another. These are
		// the live validators for the six fields the rule covers.
		{"guest access expire", `[\d]+|custom`, true},
		{"radio tx_power", `[\d]+|auto`, true},
		{"firewall policy protocol", `all|([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])|tcp|udp|ax\.25`, true},
		{"firewall rule protocol", `^$|all|([0-9]|[1-9][0-9]|1[0-9]{2})|tcp|udp`, true},
		{"radio channel", `[0-9]|[1][0-4]|36|40|auto`, true},

		// Refuse: the empty-string branch is not a word. Counting it selects
		// every int64 that carries ^$ to allow clearing -- 130 fields rather
		// than 10.
		{"vlan with a clear escape", `[2-9]|[1-9][0-9]{1,2}|[1-3][0-9]{3}|400[0-9]|^$`, false},
		{"port with a clear escape", `[1-9][0-9]{0,3}|[1-5][0-9]{4}|^$`, false},
		{"wan cos", `[0-7]|^$`, false},

		// Refuse: an id accepts digits, but no branch is exclusively numeric
		// and none is a word -- it is one branch that takes both.
		{"object id", `[\d\w-]+`, false},
		{"object id with a clear escape", `[\d\w-]+|^$`, false},

		// Refuse: every branch numeric. Position is a float, not a word.
		{"spatial position", `(^([-]?[\d]+)$)|(^([-]?[\d]+[.]?[\d]+)$)`, false},
		{"stp priority", `0|4096|8192|61440`, false},

		// Refuse: no alternation, and nothing at all.
		{"single branch", `[0-9]+`, false},
		{"empty", ``, false},

		// Refuse: words only, no numeric branch.
		{"enum of names", `auto|medium|high|low|custom`, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := admitsNumberOrWord(c.pattern); got != c.want {
				t.Errorf("admitsNumberOrWord(%q) = %v, want %v", c.pattern, got, c.want)
			}
		})
	}
}

// TestSplitAlternationLeavesGroupsAlone guards the one piece of parsing the
// rule does. A | inside a group or a character class is not a branch, and
// treating it as one silently changes which fields the rule selects.
func TestSplitAlternationLeavesGroupsAlone(t *testing.T) {
	cases := []struct {
		pattern string
		want    int
	}{
		{`a|b|c`, 3},
		{`(a|b)|c`, 2},
		{`[a|b]|c`, 2},
		{`(a|(b|c))`, 1},
		{`a`, 1},
		{`\||a`, 2}, // an escaped pipe is a literal, not a separator
	}
	for _, c := range cases {
		if got := len(splitAlternation(c.pattern)); got != c.want {
			t.Errorf("splitAlternation(%q) gave %d branches, want %d", c.pattern, got, c.want)
		}
	}
}
