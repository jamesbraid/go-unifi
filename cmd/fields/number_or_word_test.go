package main

import (
	"errors"
	"testing"
)

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

// The tests above cover the predicate. Nothing covered the wrapper that
// applies it, and the corpus hides that: no field is a pointer string, and no
// field carries both a hand-written decoder and a qualifying validator. So
// every branch that decides *when not to act* was unexercised, and dropping
// any of them left the whole suite green -- the shims are already committed,
// so a wrapper that over-applies changes nothing until the next regeneration.
//
// Each case here kills a mutant that survived: removing the pointer
// exclusion, removing the hand-written-wins guard, and running the rule
// before the resource's own processor instead of after.
func TestWithNumberOrWordDecoding(t *testing.T) {
	const qualifying = `[\d]+|custom`

	cases := []struct {
		name  string
		field *FieldInfo
		want  string
	}{
		{
			name:  "plain string with a qualifying validator is shimmed",
			field: NewFieldInfo("Expire", "expire", "string", qualifying, true, false, false, ""),
			want:  "types.Number",
		},
		{
			name: "pointer string is left alone",
			// The emitter has no tested form for a pointer string decoded
			// through types.Number, so the rule declines rather than
			// generating something unverified.
			field: NewFieldInfo("Expire", "expire", "string", qualifying, true, false, true, ""),
			want:  "",
		},
		{
			name: "a hand-written decoder wins",
			// FirewallPolicy.Port and DeviceRadioTable.Ht set their own; the
			// rule must not overwrite a deliberate decision.
			field: NewFieldInfo("Port", "port", "string", qualifying, true, false, false, "types.Something"),
			want:  "types.Something",
		},
		{
			name:  "a non-string is untouched",
			field: NewFieldInfo("Expire", "expire", "int64", qualifying, true, false, false, ""),
			want:  "",
		},
		{
			name:  "a string whose validator does not qualify is untouched",
			field: NewFieldInfo("ID", "id", "string", `[\d\w-]+|^$`, true, false, false, ""),
			want:  "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			processor := withNumberOrWordDecoding(nil)
			if err := processor(c.field.FieldName, c.field); err != nil {
				t.Fatalf("processor: %v", err)
			}
			if c.field.CustomUnmarshalType != c.want {
				t.Errorf("CustomUnmarshalType = %q, want %q", c.field.CustomUnmarshalType, c.want)
			}
		})
	}
}

// The rule runs after the resource's own processor, so it sees the type that
// processor decided and cannot overwrite a decoder it set. Running it first
// would break both, and the corpus does not exercise either ordering.
func TestWithNumberOrWordDecodingRunsAfterTheResourceProcessor(t *testing.T) {
	const qualifying = `[\d]+|auto`

	t.Run("sees a type the processor changed", func(t *testing.T) {
		field := NewFieldInfo("TxPower", "tx_power", "int64", qualifying, true, false, false, "")
		processor := withNumberOrWordDecoding(func(_ string, f *FieldInfo) error {
			f.FieldType = "string"
			return nil
		})
		if err := processor(field.FieldName, field); err != nil {
			t.Fatal(err)
		}
		if field.CustomUnmarshalType != "types.Number" {
			t.Errorf("CustomUnmarshalType = %q; the rule ran before the processor set the type",
				field.CustomUnmarshalType)
		}
	})

	t.Run("respects a decoder the processor set", func(t *testing.T) {
		field := NewFieldInfo("Ht", "ht", "string", qualifying, true, false, false, "")
		processor := withNumberOrWordDecoding(func(_ string, f *FieldInfo) error {
			f.CustomUnmarshalType = "types.Chosen"
			return nil
		})
		if err := processor(field.FieldName, field); err != nil {
			t.Fatal(err)
		}
		if field.CustomUnmarshalType != "types.Chosen" {
			t.Errorf("CustomUnmarshalType = %q; the rule overwrote the processor's choice",
				field.CustomUnmarshalType)
		}
	})

	t.Run("an error from the processor stops the rule", func(t *testing.T) {
		field := NewFieldInfo("Expire", "expire", "string", qualifying, true, false, false, "")
		processor := withNumberOrWordDecoding(func(_ string, _ *FieldInfo) error {
			return errRefused
		})
		if err := processor(field.FieldName, field); !errors.Is(err, errRefused) {
			t.Fatalf("err = %v, want errRefused", err)
		}
		if field.CustomUnmarshalType != "" {
			t.Errorf("the rule ran despite the processor failing")
		}
	})
}

var errRefused = errors.New("refused")
