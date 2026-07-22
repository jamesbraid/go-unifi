//go:build integration

// unifi/network_wan_unset_integration_test.go
package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationWANEncoderUnset verifies a live controller accepts the
// encoder's output for a plain DHCP WAN. marshalWAN emits a batch of WAN
// fields WITHOUT omitempty (wan_username, x_wan_password, wan_ipv6,
// wan_gateway_v6, wan_ip_aliases, ...), so a DHCP WAN that uses none of them
// is sent with those fields present-but-empty. Phase 2 probed each WAN field
// only when SET; this covers the unset case end-to-end, POSTing the real
// encoder output (via Network.MarshalJSON) rather than a hand-built payload.
// A rejection here would mean one of those always-emitted fields should be
// omitempty instead.
func TestIntegrationWANEncoderUnset(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.Start(ctx, t)
	s := c.NewSession(ctx, t)

	// A plain DHCP WAN with no static/PPPoE/DS-Lite settings: every
	// credential and address field marshalWAN emits is left at its zero
	// value and serialized as an empty string.
	n := &Network{
		Name:            strPtr("wan-encoder-unset-probe"),
		Purpose:         PurposeWAN,
		Enabled:         true,
		WANType:         strPtr("dhcp"),
		WANTypeV6:       strPtr("disabled"),
		WANNetworkGroup: strPtr("WAN2"),
	}

	// PostJSON marshals n through Network.MarshalJSON -> marshalWAN, so this
	// posts exactly what the SDK would send.
	body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", n)
	if err != nil {
		t.Fatalf("transport: %v", err)
	}
	if status != 200 {
		t.Fatalf("controller rejected the encoder's plain DHCP WAN output (HTTP %d): %v", status, body)
	}

	created := firstData(t, body)
	if id, _ := created["_id"].(string); id != "" {
		defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck
	}
	if created["wan_type"] != "dhcp" {
		t.Errorf("wan_type = %v, want dhcp", created["wan_type"])
	}
	if created["purpose"] != PurposeWAN {
		t.Errorf("purpose = %v, want %s", created["purpose"], PurposeWAN)
	}
}
