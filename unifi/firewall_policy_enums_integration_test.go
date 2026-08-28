//go:build integration

package unifi

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// enumClassRe pulls the accepted values out of the controller's own
// deserialization failure, which names the Java enum and lists its
// constants: `... not one of the values accepted for Enum class: [A, B]`.
var enumClassRe = regexp.MustCompile(`not one of the values accepted for Enum class: \[([^\]]*)\]`)

// TestIntegrationFirewallPolicyEnumsMatchTheController checks the generated
// value lists for the firewall policy against the controller itself.
//
// These are v2 fields, so their definitions are hand-maintained in
// overrides/resources rather than extracted from the controller's own schema
// files -- nothing keeps them honest except a measurement. The controller
// turns out to be a good oracle for the enum-typed ones: handed a value it
// cannot deserialize, it answers with the enum class and every constant in
// it, which is the list this compares against.
func TestIntegrationFirewallPolicyEnumsMatchTheController(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)
	path := "/v2/api/site/" + c.Site + "/firewall-policies"

	for _, name := range []string{"enum-probe-src", "enum-probe-dst"} {
		s.PostJSON(ctx, "/v2/api/site/"+c.Site+"/firewall/zone", //nolint:errcheck
			map[string]any{"name": name, "network_ids": []string{}})
	}
	zones, status, err := s.GetJSON(ctx, "/v2/api/site/"+c.Site+"/firewall/zone")
	if err != nil || status != 200 {
		t.Fatalf("list zones (HTTP %d): %v", status, err)
	}
	var zoneIDs []string
	for _, z := range asSlice(zones) {
		m, _ := z.(map[string]any)
		if id, _ := m["_id"].(string); id != "" {
			zoneIDs = append(zoneIDs, id)
		}
	}
	if len(zoneIDs) == 0 {
		t.Skip("no firewall zones to address a policy with")
	}
	src, dst := zoneIDs[0], zoneIDs[len(zoneIDs)-1]

	n := 0
	post := func(overrides map[string]any) (int, map[string]any) {
		n++
		payload := map[string]any{
			"name": fmt.Sprintf("enum-probe-%d", n), "enabled": true,
			"action": "ALLOW", "predefined": false, "index": 22000 + n,
			"protocol": "all", "ip_version": "BOTH",
			"connection_state_type": "ALL", "connection_states": []string{},
			"source":      map[string]any{"zone_id": src, "matching_target": "ANY"},
			"destination": map[string]any{"zone_id": dst, "matching_target": "ANY"},
			"logging":     false, "create_allow_respond": true,
			"schedule": map[string]any{"mode": "ALWAYS", "time_all_day": true, "repeat_on_days": []string{}},
		}
		for k, v := range overrides {
			payload[k] = v
		}
		body, status, err := s.PostJSON(ctx, path, payload)
		if err != nil {
			t.Fatalf("transport: %v", err)
		}
		m, _ := body.(map[string]any)
		if status == 200 || status == 201 {
			if id, _ := m["_id"].(string); id != "" {
				s.DeleteJSON(ctx, path+"/"+id) //nolint:errcheck
			}
		}
		return status, m
	}

	// Ask the controller to name an enum's constants by handing it one it
	// cannot parse.
	controllerEnum := func(field string) []string {
		t.Helper()
		_, body := post(map[string]any{field: "NOT_A_VALID_VALUE_XYZ"})
		msg, _ := body["message"].(string)
		match := enumClassRe.FindStringSubmatch(msg)
		if match == nil {
			t.Fatalf("%s: the controller did not answer with an enum class list, so this "+
				"test cannot read its accepted values. It said: %s", field, msg)
		}
		values := strings.Split(match[1], ", ")
		slices.Sort(values)
		return values
	}

	for _, tc := range []struct {
		field     string
		generated []string
	}{
		{"connection_state_type", FirewallPolicyConnectionStateTypeValues},
		// FirewallPolicyVersionValues, not ...IPVersionValues: the generated
		// identifier is composed from the Go field name (Version), not the
		// wire name. FieldConstraints["FirewallPolicy"]["ip_version"] is the
		// form that does not require knowing that.
		{"ip_version", FirewallPolicyVersionValues},
	} {
		t.Run(tc.field, func(t *testing.T) {
			want := controllerEnum(tc.field)
			got := slices.Clone(tc.generated)
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("the generated values for %s are %v; the controller accepts %v.\n\n"+
					"This is a v2 field, so its definition is hand-maintained in "+
					"overrides/resources/FirewallPolicy.json -- update it there and "+
					"regenerate.", tc.field, got, want)
			}
		})
	}

	// protocol is not an enum on the wire: it takes protocol names and
	// numbers, so the generated form is a pattern rather than a value list.
	// What matters is that the names the definition claims are really
	// accepted, and that the two ICMP flavours are, since they were missing
	// until measured.
	t.Run("protocol", func(t *testing.T) {
		for _, tc := range []struct{ proto, version string }{
			{"all", "BOTH"}, {"tcp_udp", "IPV4"}, {"icmp", "IPV4"}, {"icmpv6", "IPV6"}, {"6", "IPV4"},
		} {
			status, body := post(map[string]any{"protocol": tc.proto, "ip_version": tc.version})
			if status != 200 && status != 201 {
				msg, _ := body["message"].(string)
				t.Errorf("protocol %q with ip_version %q was refused (HTTP %d): %s",
					tc.proto, tc.version, status, msg)
			}
		}

		// And the pairing that makes protocol impossible to validate on its
		// own: icmp is a real protocol name, refused only because the IP
		// version does not suit it. A consumer cannot check this from the
		// field's own pattern, which is why the pattern does not try.
		status, body := post(map[string]any{"protocol": "icmp", "ip_version": "IPV6"})
		msg, _ := body["message"].(string)
		if status == 200 || status == 201 {
			t.Log("note: this controller now accepts icmp on IPV6; the pairing recorded here has gone")
		} else if !strings.Contains(msg, "unsupported on IP version") {
			t.Errorf("icmp on IPV6 was refused with %q, not the IP-version pairing message; "+
				"the reason a protocol is refused has changed", msg)
		}
	})
}

func asSlice(v any) []any {
	out, _ := v.([]any)
	return out
}
