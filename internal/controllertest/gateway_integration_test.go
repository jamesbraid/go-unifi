//go:build integration

package controllertest

import (
	"context"
	"testing"
	"time"
)

// TestIntegrationAdoptGateway brings up an emulated modern gateway
// (UXGENT / "Gateway Enterprise", type uxg) and proves AdoptGateway drives it
// all the way to a connected, adopted device controller-side. Its MAC is
// allocated by the herder from a locally administered range, so stat/device
// lookups can only match the emulated gateway — never the seeded,
// still-pending UGW3.
//
// This is the first live proof that a uxg-type device adopts to CONNECTED: the
// emulator only unit-tests that payload path. A failure here (adoption stalls,
// or the controller flags the model unsupported) is a finding about the
// emulator or the controller build's uxg support, not necessarily a go-unifi
// bug.
func TestIntegrationAdoptGateway(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	c := Start(ctx, t)
	s := c.NewSession(ctx, t)

	d := AdoptGateway(ctx, t, c, s)
	if d.Type != GatewayType || d.State != 1 || !d.Adopted {
		t.Fatalf("gateway %s: type=%q state=%d adopted=%v, want %s/1/true", d.MAC, d.Type, d.State, d.Adopted, GatewayType)
	}
	t.Logf("adopted gateway %s: model=%q type=%q state=%d", d.MAC, d.Model, d.Type, d.State)
}
