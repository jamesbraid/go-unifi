package main

import (
	"regexp"
	"strconv"
	"strings"
)

// Numeric bounds and length bounds, the other two things the controller's
// validators say that a consumer would otherwise retype by hand.
//
// Enumerations (see enums.go) are the easy half: the pattern lists its
// values. Ranges are not written as ranges -- the controller expresses
// "68 to 65536" as
//
//	^(6[89]|[7-9][0-9]|[1-9][0-9]{2,3}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-6])$
//
// so the bounds have to be recovered from a regex that encodes them
// digit-position by digit-position. Rather than parse that, ask the
// validator itself: run every integer in a bounded domain past it and see
// which it accepts.

const (
	// numericScanMin and numericScanMax bound the search. They comfortably
	// cover the real fields (ports to 65535, MTUs to 65536, time offsets to
	// +/-86400) while keeping generation to a couple of seconds.
	numericScanMin = -100_000
	numericScanMax = 100_000
)

// numericRange returns the inclusive bounds a numeric field's validator
// accepts, and whether that set is a contiguous range worth expressing as
// one.
//
// It refuses more than it accepts, on purpose:
//
//   - A set with holes is not a range. Device stp_priority accepts
//     0|4096|8192|... and clamping a consumer to 0..61440 would let through
//     61439, which the controller rejects. Those are enumerations, and
//     enumInt64Values already covers them.
//   - A set that reaches either end of the scan domain is unbounded as far
//     as this can tell, so the domain would be reported as the bound rather
//     than the schema's. That is not hypothetical: priority is an int64
//     field whose pattern is .{1,128} -- a *length* rule -- which matches
//     every integer offered to it and would otherwise yield bounds invented
//     from the scan.
//   - A single accepted value is a constant, not a range.
func numericRange(pattern string) (low, high int64, ok bool) {
	re, err := compileAnchored(pattern)
	if err != nil {
		return 0, 0, false
	}

	low, high = 0, 0
	count, seen := 0, false
	for n := int64(numericScanMin); n <= numericScanMax; n++ {
		if !re.MatchString(strconv.FormatInt(n, 10)) {
			continue
		}
		if !seen {
			low, seen = n, true
		}
		high = n
		count++
	}
	if !seen {
		return 0, 0, false
	}
	// Holes: not a range.
	if int64(count) != high-low+1 {
		return 0, 0, false
	}
	// The domain is doing the bounding, not the pattern.
	if low == numericScanMin || high == numericScanMax {
		return 0, 0, false
	}
	if low == high {
		return 0, 0, false
	}
	return low, high, true
}

// lengthPatternRe matches a bare .{n,m} length rule, optionally anchored.
var lengthPatternRe = regexp.MustCompile(`^\^?\.\{(\d+),(\d+)\}\$?$`)

// lengthBounds returns the character-count bounds of a .{n,m} validator.
// Only that exact shape is recognised -- a pattern that constrains content
// as well as length says more than a length bound can carry.
func lengthBounds(pattern string) (low, high int64, ok bool) {
	m := lengthPatternRe.FindStringSubmatch(strings.TrimSpace(pattern))
	if m == nil {
		return 0, 0, false
	}
	low, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	high, err = strconv.ParseInt(m[2], 10, 64)
	if err != nil || high < low {
		return 0, 0, false
	}
	return low, high, true
}

// compileAnchored compiles a validator so it must match the whole value.
// Several patterns are written unanchored, and left that way "1" would be
// accepted on the strength of a match inside "12345".
func compileAnchored(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(`\A(?:` + pattern + `)\z`)
}
