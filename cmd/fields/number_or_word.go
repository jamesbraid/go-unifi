package main

import (
	"regexp"
	"strings"

	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

// Some controller validators admit two shapes for the same value: a bare
// integer, or a word. A firewall protocol is 47 or "gre". A radio's tx_power
// is 3 or "auto". Guest access expires after a count of units, or "custom".
//
// A field like that has to be modelled as a string, because the word form has
// to survive. But the controller serialises the numeric form as a JSON
// number, and a JSON number does not decode into a Go string -- so reading
// back an object that has the field set fails. Decoding through types.Number
// accepts either shape and yields the text.
//
// This was applied by hand three times before anyone called it a rule: to
// both radio tables, and to FirewallPolicy.Port. It was missed on six other
// fields carrying the same kind of validator. Deriving it from the validator
// is what stops the next one being missed.

// The classification comes from asking the validator what it accepts, not
// from reading it -- the same approach ranges.go takes, and for the same
// reason: a pattern is only reliably understood by what it matches.
var (
	numericProbes = []string{"0", "1", "7", "42", "100", "86400", "-1", "1.5", "-1.5"}
	wordProbes    = []string{"auto", "custom", "abc", "tcp", "all", "a1", "x", "1a"}
)

// splitAlternation returns the top-level branches of a pattern, leaving the
// | inside a group or a character class alone.
func splitAlternation(pattern string) []string {
	var out []string
	depth, inClass, start := 0, false, 0
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; {
		case c == '\\':
			i++
		case inClass:
			if c == ']' {
				inClass = false
			}
		case c == '[':
			inClass = true
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == '|' && depth == 0:
			out = append(out, pattern[start:i])
			start = i + 1
		}
	}
	return append(out, pattern[start:])
}

// branchMatcher compiles one branch on its own. Branches carry their own
// anchors as often as not, and a ^ or $ left in place would be judged as a
// zero-width assertion rather than stripped, so remove them first.
func branchMatcher(branch string) *regexp.Regexp {
	b := strings.TrimSuffix(strings.TrimPrefix(branch, "^"), "$")
	re, err := regexp.Compile(`\A(?:` + b + `)\z`)
	if err != nil {
		return nil // lookaround; those validators are not this shape
	}
	return re
}

func acceptsAny(re *regexp.Regexp, probes []string) bool {
	for _, p := range probes {
		if re.MatchString(p) {
			return true
		}
	}
	return false
}

// admitsNumberOrWord reports whether a validator accepts a bare number in one
// branch and a non-empty word in another.
//
// The empty-string branch that most numeric validators carry -- "^$", so the
// value can be cleared -- is not a word. Those fields are numbers with an
// escape hatch and are already modelled as int64; treating "" as a word would
// pull in a hundred of them.
func admitsNumberOrWord(pattern string) bool {
	if pattern == "" {
		return false
	}
	branches := splitAlternation(pattern)
	if len(branches) < 2 {
		return false
	}
	var number, word bool
	for _, b := range branches {
		re := branchMatcher(b)
		if re == nil {
			continue
		}
		switch {
		case acceptsAny(re, wordProbes):
			word = true
		case acceptsAny(re, numericProbes):
			number = true
		}
	}
	return number && word
}

// withNumberOrWordDecoding applies the rule after the resource's own
// processor has had its say, so a hand-written decision still wins.
//
// Only non-pointer strings are covered. No field of that shape is a pointer
// today, and the emitter has no tested form for a pointer string decoded
// through types.Number; TestEveryNumberOrWordFieldDecodesBothShapes fails
// loudly if one appears rather than letting it pass unnoticed.
func withNumberOrWordDecoding(next func(string, *FieldInfo) error) func(string, *FieldInfo) error {
	return func(name string, f *FieldInfo) error {
		if next != nil {
			if err := next(name, f); err != nil {
				return err
			}
		}
		if f.FieldType == fields.String && !f.IsPointer &&
			f.CustomUnmarshalType == "" && admitsNumberOrWord(f.FieldValidation) {
			f.CustomUnmarshalType = fields.Number
		}
		return nil
	}
}
