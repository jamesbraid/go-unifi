package unifi

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFirewallPolicyScheduleRoundTripsControllerFields(t *testing.T) {
	raw := []byte(`{
		"date":"2026-07-10",
		"date_start":"2026-07-01",
		"date_end":"2026-07-31",
		"mode":"CUSTOM",
		"repeat_on_days":["mon","wed","fri"],
		"time_all_day":false,
		"time_range_start":"09:00",
		"time_range_end":"17:30"
	}`)

	var got FirewallPolicySchedule
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal schedule: %v", err)
	}

	if got.Date != "2026-07-10" {
		t.Fatalf("Date = %q, want %q", got.Date, "2026-07-10")
	}
	if got.DateStart != "2026-07-01" {
		t.Fatalf("DateStart = %q, want %q", got.DateStart, "2026-07-01")
	}
	if got.DateEnd != "2026-07-31" {
		t.Fatalf("DateEnd = %q, want %q", got.DateEnd, "2026-07-31")
	}
	if got.Mode != "CUSTOM" {
		t.Fatalf("Mode = %q, want %q", got.Mode, "CUSTOM")
	}
	if !reflect.DeepEqual(got.RepeatOnDays, []string{"mon", "wed", "fri"}) {
		t.Fatalf("RepeatOnDays = %#v", got.RepeatOnDays)
	}
	if got.TimeAllDay == nil || *got.TimeAllDay {
		t.Fatalf("TimeAllDay = %v, want pointer to false", got.TimeAllDay)
	}
	if got.TimeRangeStart != "09:00" || got.TimeRangeEnd != "17:30" {
		t.Fatalf("time range = %q-%q", got.TimeRangeStart, got.TimeRangeEnd)
	}

	roundTripped, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal schedule: %v", err)
	}

	var wantObject, gotObject map[string]any
	if err := json.Unmarshal(raw, &wantObject); err != nil {
		t.Fatalf("decode expected JSON: %v", err)
	}
	if err := json.Unmarshal(roundTripped, &gotObject); err != nil {
		t.Fatalf("decode round-tripped JSON: %v", err)
	}
	if !reflect.DeepEqual(gotObject, wantObject) {
		t.Fatalf("round-trip mismatch:\n got: %s\nwant: %s", roundTripped, raw)
	}
}
