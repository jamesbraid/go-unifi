package probe

import "testing"

func TestClassify(t *testing.T) {
	asked := map[string]any{
		"kept":    "same",
		"changed": 7200,
		"dropped": true,
	}
	stored := map[string]any{
		"kept":    "same",
		"changed": 86400, // controller floored it
		// dropped absent
		"extra": "controller-added", // not asked -> ignored
	}
	got := map[string]Verdict{}
	for _, r := range Classify(asked, stored) {
		got[r.Wire] = r.Verdict
	}
	if _, ok := got["kept"]; ok {
		t.Error("a kept field should not appear in the result")
	}
	if got["changed"] != Changed {
		t.Errorf("changed = %v, want Changed", got["changed"])
	}
	if got["dropped"] != Dropped {
		t.Errorf("dropped = %v, want Dropped", got["dropped"])
	}
	if _, ok := got["extra"]; ok {
		t.Error("a controller-added field was not asked and must not be classified")
	}
}

// The float/int and string/number representation differences the controller
// introduces must read as Kept, or every numeric field looks changed.
func TestJSONEqualIgnoresRepresentation(t *testing.T) {
	if !JSONEqual(7200, 7200.0) {
		t.Error("int and float of the same value should be equal")
	}
	if !JSONEqual([]int{1, 6, 11}, []any{1.0, 6.0, 11.0}) {
		t.Error("a typed slice and its JSON-decoded form should be equal")
	}
	if JSONEqual("30", 30) {
		t.Error("a string and a number are genuinely different on the wire")
	}
}

func TestFirstData(t *testing.T) {
	cases := []struct {
		name string
		body any
		want string
	}{
		{"v1 envelope", map[string]any{"data": []any{map[string]any{"_id": "a"}}}, "a"},
		{"bare object", map[string]any{"_id": "b"}, "b"},
		{"bare array", []any{map[string]any{"_id": "c"}}, "c"},
		{"empty data", map[string]any{"data": []any{}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FirstData(c.body)
			id, _ := got["_id"].(string)
			if id != c.want {
				t.Errorf("id = %q, want %q", id, c.want)
			}
		})
	}
}
