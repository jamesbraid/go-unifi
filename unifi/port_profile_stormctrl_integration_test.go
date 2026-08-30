//go:build integration

// unifi/port_profile_stormctrl_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// stormctrlCase is one storm-control payload: whether a level, a rate, or
// both are set alongside stormctrl_type, and what the controller did.
type stormctrlCase struct {
	name      string
	kind      string // bcast, mcast or ucast
	typeField string // stormctrl_type: "level", "rate", or unset
	level     any    // nil to omit
	rate      any    // nil to omit
	want      string // measured api.err.* code, or "accepted"
	// wantBoth asserts that a payload carrying both a level and a rate has
	// both stored. That is the substantive finding: the controller does not
	// treat them as alternatives, so neither is discarded.
	wantBoth bool
}

// networkStormctrlRules records what the controller does when a port profile
// sets a storm-control level, a rate, or both.
//
// The reason to measure it: terraform-provider-unifi declares
// stormctrl_bcast_level and stormctrl_bcast_rate mutually exclusive with
// ConflictsWith, and the same for mcast and ucast -- six declarations,
// verified present 2026-08-29. Nothing measured says the controller agrees.
//
// That sentence is a claim about another project's current source, and
// nothing here is gated on it: if those declarations go, this comment is
// quietly wrong and no build breaks. Dated so a reader knows what it was
// checked against rather than reading it as a permanent fact. The schema has a separate stormctrl_type field taking
// level|rate, which suggests the type selects which of the two applies rather
// than the two being illegal together -- in which case ConflictsWith rejects
// configurations the controller would accept, and that failure looks like a
// controller bug to whoever hits it.
//
// This is the same class as networkCrossFieldRules and the openvpn RADIUS
// table: a rule about how two fields interact, which the extracted schema
// cannot express and only the controller can answer.
var networkStormctrlRules = []stormctrlCase{
	{name: "level only, type level", kind: "bcast", typeField: "level", level: 50, want: "accepted"},
	{name: "rate only, type rate", kind: "bcast", typeField: "rate", rate: 1000, want: "accepted"},
	{name: "both set, type level", kind: "bcast", typeField: "level", level: 50, rate: 1000, want: "accepted", wantBoth: true},
	{name: "both set, type rate", kind: "bcast", typeField: "rate", level: 50, rate: 1000, want: "accepted", wantBoth: true},
	{name: "both set, no type", kind: "bcast", level: 50, rate: 1000, want: "accepted", wantBoth: true},
	{name: "level only, type rate", kind: "bcast", typeField: "rate", level: 50, want: "accepted"},
	{name: "mcast both set", kind: "mcast", typeField: "level", level: 50, rate: 1000, want: "accepted", wantBoth: true},
	{name: "ucast both set", kind: "ucast", typeField: "level", level: 50, rate: 1000, want: "accepted", wantBoth: true},
}

// TestIntegrationPortProfileStormctrlRules measures each combination. Run it
// with -v: the log lines are the point, since what the table should say is
// exactly what this measures.
func TestIntegrationPortProfileStormctrlRules(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	for i, tc := range networkStormctrlRules {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{
				"name":    fmt.Sprintf("stormctrl-%d", i),
				"forward": "all",
				fmt.Sprintf("stormctrl_%s_enabled", tc.kind): true,
			}
			if tc.typeField != "" {
				payload["stormctrl_type"] = tc.typeField
			}
			if tc.level != nil {
				payload[fmt.Sprintf("stormctrl_%s_level", tc.kind)] = tc.level
			}
			if tc.rate != nil {
				payload[fmt.Sprintf("stormctrl_%s_rate", tc.kind)] = tc.rate
			}

			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/portconf", payload)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}

			got := "accepted"
			stored := ""
			if status != 200 {
				got = errCode(body)
			} else if created := firstData(t, body); created != nil {
				if id, _ := created["_id"].(string); id != "" {
					defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/portconf/"+id) //nolint:errcheck
				}
				stored = fmt.Sprintf("type=%v level=%v rate=%v",
					created["stormctrl_type"],
					created[fmt.Sprintf("stormctrl_%s_level", tc.kind)],
					created[fmt.Sprintf("stormctrl_%s_rate", tc.kind)])
			}

			t.Logf("MEASURED %-24s -> %s %s", tc.name, got, stored)

			if got != tc.want {
				t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
			}
			if tc.wantBoth {
				created := firstData(t, body)
				level := created[fmt.Sprintf("stormctrl_%s_level", tc.kind)]
				rate := created[fmt.Sprintf("stormctrl_%s_rate", tc.kind)]
				if level == nil || rate == nil {
					t.Errorf("sent both a level and a rate, controller stored level=%v rate=%v; one was discarded", level, rate)
				}
			}
		})
	}
}
