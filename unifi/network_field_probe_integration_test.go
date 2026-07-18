//go:build integration

// unifi/network_field_probe_integration_test.go
package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationNetworkFieldProbe classifies every candidate field by
// creating a throwaway network with the field set (raw request — the SDK
// encoder would strip it), reading it back, and comparing.
//
//	PERSISTED: controller stored and returned the value -> wire it (Task 4)
//	STRIPPED:  create succeeded, field absent/zero on read-back
//	REJECTED:  create failed with the field present
func TestIntegrationNetworkFieldProbe(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.Start(ctx, t)
	s := c.NewSession(ctx, t)

	// Resolve "@zone" placeholders to a real zone id once.
	zoneID := firstZoneID(ctx, t, s, c.Site)

	base := func(purpose string, n int) map[string]any {
		m := map[string]any{
			"name":    fmt.Sprintf("probe-%d", n),
			"purpose": purpose,
			"enabled": true,
		}
		if purpose == PurposeCorporate || purpose == PurposeGuest {
			m["ip_subnet"] = fmt.Sprintf("10.99.%d.1/24", n%250)
			m["vlan_enabled"] = true
			m["vlan"] = 100 + n
		}
		return m
	}

	for i, cand := range networkFieldCandidates {
		t.Run(cand.Wire, func(t *testing.T) {
			payload := base(cand.Purpose, i)
			for k, v := range cand.Prereq {
				payload[k] = v
			}
			value := cand.Value
			if value == "@zone" {
				value = zoneID
			}
			payload[cand.Wire] = value

			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", payload)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}
			if status != 200 {
				t.Logf("REJECTED %s (HTTP %d): %v", cand.Wire, status, body)
				return
			}

			created := firstData(t, body)
			id, _ := created["_id"].(string)
			if id == "" {
				t.Logf("no _id in create response; skipping cleanup: %v", created)
			} else {
				defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck
			}

			got, ok := created[cand.Wire]
			// Some fields only appear on read-back; check GET too. Without an
			// id there's nothing to re-GET, so classify from the create
			// response alone.
			if !ok && id != "" {
				fresh, status, _ := s.GetJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id)
				if status == 200 {
					if m := firstData(t, fresh); m != nil {
						got, ok = m[cand.Wire]
					}
				}
			}

			if ok && jsonEqual(got, value) {
				t.Logf("PERSISTED %s = %v", cand.Wire, got)
			} else if ok {
				t.Logf("MUTATED %s: sent %v, got %v", cand.Wire, value, got)
			} else {
				t.Logf("STRIPPED %s", cand.Wire)
			}
		})
	}
}

// firstData unwraps {"meta":..., "data":[obj,...]} envelopes; v2-style bare
// objects/arrays pass through.
func firstData(t *testing.T, body any) map[string]any {
	t.Helper()
	switch v := body.(type) {
	case map[string]any:
		if data, ok := v["data"].([]any); ok && len(data) > 0 {
			if m, ok := data[0].(map[string]any); ok {
				return m
			}
			return nil
		}
		return v
	case []any:
		if len(v) > 0 {
			if m, ok := v[0].(map[string]any); ok {
				return m
			}
		}
	}
	return nil
}

// jsonEqual compares a and b by round-tripping both through JSON and
// comparing the decoded result with reflect.DeepEqual. Raw comparison of a
// Go struct/slice candidate value against the map[string]any the controller
// hands back on read-back never matches -- and fmt.Sprintf("%v", ...) on
// either side is no better, since it string-formats field order and types
// the wire format doesn't preserve. Normalizing both sides through JSON
// gives them the same shape (map[string]any, []any, float64, ...) before
// comparing. The float64 normalization also means int/float differences
// between what was sent and what came back are treated as equal, which is
// the desired semantic here: did the controller keep the value.
func jsonEqual(a, b any) bool {
	na, err := normalizeJSON(a)
	if err != nil {
		return false
	}
	nb, err := normalizeJSON(b)
	if err != nil {
		return false
	}
	return reflect.DeepEqual(na, nb)
}

func normalizeJSON(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func firstZoneID(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()
	body, status, err := s.GetJSON(ctx, "/v2/api/site/"+site+"/firewall/zone")
	if err != nil || status != 200 {
		t.Logf("no firewall zones available (status %d, %v); @zone candidates will be REJECTED", status, err)
		return "000000000000000000000000"
	}
	if items, ok := body.([]any); ok && len(items) > 0 {
		if m, ok := items[0].(map[string]any); ok {
			if id, ok := m["_id"].(string); ok {
				return id
			}
		}
	}
	return "000000000000000000000000"
}
