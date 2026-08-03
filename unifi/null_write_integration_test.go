//go:build integration

// unifi/null_write_integration_test.go
package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// nullWriteCase is one field the client serializes unconditionally whose zero
// value marshals as null, and what the controller does with that null.
type nullWriteCase struct {
	field string // "Struct.wire_name", as recorded in testdata/always_serialized_fields.txt
	wire  string

	// prepare returns the endpoint of an object to write and the object as
	// the controller stores it. An empty path means this resource cannot be
	// exercised on a bare harness, which is reported rather than skipped
	// silently.
	prepare func(ctx context.Context, t *testing.T, s *controllertest.Session, site string) (string, map[string]any)

	// want is the measured outcome: "accepted" or "rejected". Empty means
	// unmeasured, and the test reports it rather than passing quietly.
	want string
}

// TestIntegrationNullWrites measures what the controller does with a null in
// each field the client sends unconditionally.
//
// WLAN.schedule_with_duration was one of these. The client sent null whenever
// the caller had not set a schedule, the controller answered
// api.err.InvalidPayload, and every read-modify-write of a WLAN failed. The
// remaining fields of that shape had never been measured, so this establishes
// whether any of them does the same thing.
//
// Device.port_overrides is not here: it needs an adopted device, so it lives
// with the rest of the device work in
// TestIntegrationDevicePortOverridePreference.
func TestIntegrationNullWrites(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	for _, tc := range nullWriteCases() {
		t.Run(tc.field, func(t *testing.T) {
			path, stored := tc.prepare(ctx, t, s, c.Site)
			if path == "" {
				t.Skipf("%s cannot be exercised on this harness; the null behaviour of %s is unmeasured",
					tc.field, tc.wire)
			}

			// Control: the controller must accept its own object, or the
			// comparison below measures something else.
			if body, status, err := s.PutJSON(ctx, path, clone(stored)); err != nil || (status != 200 && status != 201) {
				t.Fatalf("control write rejected (HTTP %d): %v %v", status, errCode(body), err)
			}

			withNull := clone(stored)
			withNull[tc.wire] = nil

			// Pinned as accepted or rejected rather than by message: these
			// come back as Spring validation prose with obfuscated class
			// names, not an api.err.* code, and that text is not a stable
			// thing to assert on.
			got := "accepted"
			body, status, err := s.PutJSON(ctx, path, withNull)
			if err != nil || (status != 200 && status != 201) {
				got = "rejected"
				t.Logf("rejected (HTTP %d): %v %v", status, body, err)
			}

			if tc.want == "" {
				t.Errorf("UNPINNED %s: null -> %s. Record it in nullWriteCases.", tc.field, got)
				return
			}
			if got != tc.want {
				t.Errorf("%s: null -> %q, want %q", tc.field, got, tc.want)
			}
		})
	}
}

func nullWriteCases() []nullWriteCase {
	return []nullWriteCase{
		{
			// A group of our own, not the site default: writing the default
			// back verbatim is rejected outright, because the controller
			// requires attr_hidden_id to be absent and attr_no_delete to be
			// false, and the default carries both.
			field: "APGroup.device_macs", wire: "device_macs", want: "rejected",
			prepare: func(ctx context.Context, t *testing.T, s *controllertest.Session, site string) (string, map[string]any) {
				base := "/v2/api/site/" + site + "/apgroups"
				body, status, err := s.PostJSON(ctx, base, map[string]any{
					"name": "null-write", "device_macs": []string{},
				})
				if err != nil || (status != 200 && status != 201) {
					t.Logf("cannot create an AP group (HTTP %d): %v", status, errCode(body))
					return "", nil
				}
				stored := firstData(t, body)
				id := objectID(stored)
				if id == "" {
					return "", nil
				}
				t.Cleanup(func() {
					s.DeleteJSON(context.WithoutCancel(ctx), base+"/"+id) //nolint:errcheck
				})
				return base + "/" + id, stored
			},
		},
		{
			field: "FirewallZone.network_ids", wire: "network_ids",
			prepare: existingV2Object("firewall/zone"),
		},
		{
			field: "FirewallPolicy.connection_states", wire: "connection_states",
			prepare: existingV2Object("firewall-policies"),
		},
		{
			// The singular path, which is the one that serves every verb but
			// the list. type is an enum of USERS|CLIENTS, not an address
			// family -- the controller names the accepted values when a
			// wrong one is sent, which is how this was found.
			field: "NetworkMembersGroup.members", wire: "members", want: "rejected",
			prepare: func(ctx context.Context, t *testing.T, s *controllertest.Session, site string) (string, map[string]any) {
				base := "/v2/api/site/" + site + "/network-members-group"
				body, status, err := s.PostJSON(ctx, base, map[string]any{
					"name": "null-write", "type": "CLIENTS", "members": []string{},
				})
				if err != nil || (status != 200 && status != 201) {
					t.Logf("cannot create a network members group (HTTP %d): %v", status, errCode(body))
					return "", nil
				}
				stored := firstData(t, body)
				id := objectID(stored)
				if id == "" {
					return "", nil
				}
				t.Cleanup(func() {
					s.DeleteJSON(context.WithoutCancel(ctx), base+"/"+id) //nolint:errcheck
				})
				return base + "/" + id, stored
			},
		},
	}
}

// existingV2Object writes against whatever the controller already has in a v2
// collection, since these resources are gateway-gated and cannot be created
// on a bare harness.
func existingV2Object(collection string) func(context.Context, *testing.T, *controllertest.Session, string) (string, map[string]any) {
	return func(ctx context.Context, t *testing.T, s *controllertest.Session, site string) (string, map[string]any) {
		base := "/v2/api/site/" + site + "/" + collection
		body, status, err := s.GetJSON(ctx, base)
		if err != nil || status != 200 {
			t.Logf("cannot list %s (HTTP %d): %v", collection, status, err)
			return "", nil
		}
		items, ok := body.([]any)
		if !ok || len(items) == 0 {
			t.Logf("%s is empty on this harness", collection)
			return "", nil
		}
		stored, _ := items[0].(map[string]any)
		id := objectID(stored)
		if id == "" {
			return "", nil
		}
		return base + "/" + id, stored
	}
}

func objectID(m map[string]any) string {
	if id, ok := m["_id"].(string); ok && id != "" {
		return id
	}
	id, _ := m["id"].(string)
	return id
}

func clone(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
