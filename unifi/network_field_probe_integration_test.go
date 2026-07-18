//go:build integration

// unifi/network_field_probe_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/testenv"
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
	c := testenv.Start(ctx, t)
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
			defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck

			got, ok := created[cand.Wire]
			// Some fields only appear on read-back; check GET too.
			if !ok {
				fresh, status, _ := s.GetJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id)
				if status == 200 {
					if m := firstData(t, fresh); m != nil {
						got, ok = m[cand.Wire]
					}
				}
			}

			if ok && fmt.Sprintf("%v", got) == fmt.Sprintf("%v", value) {
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

func firstZoneID(ctx context.Context, t *testing.T, s *testenv.Session, site string) string {
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
