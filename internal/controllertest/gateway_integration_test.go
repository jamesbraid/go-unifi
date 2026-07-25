//go:build integration

package controllertest

import (
	"context"
	"testing"
	"time"

	emu "github.com/jamesbraid/unifi-emu"
)

// TestIntegrationAdoptGateway brings up an emulated modern gateway
// (UXGENT / "Gateway Enterprise", type uxg) and proves AdoptGateway drives it
// all the way to a connected, adopted device controller-side. The MAC sits in
// a UBNT OUI suffix the -sim demo fleet never uses, so stat/device lookups can
// only match the emulated gateway — never the seeded, still-pending UGW3.
//
// This is the first live proof that a uxg-type device adopts to CONNECTED: the
// emu only unit-tests that payload path. A failure here (adoption stalls, the
// controller flags the model unsupported, or the sim never reaches CONNECTED)
// is a finding about the emu or the controller build's uxg support, not
// necessarily a go-unifi bug.
func TestIntegrationAdoptGateway(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	c := Start(ctx, t)
	s := c.NewSession(ctx, t)

	const mac = "00:27:22:e0:00:51"
	d := AdoptGateway(ctx, t, c, s, emu.DeviceSpec{MAC: mac, Model: GatewayModel, IP: "192.168.1.242"})
	if d.Type != GatewayType || d.State != 1 || !d.Adopted {
		t.Fatalf("gateway %s: type=%q state=%d adopted=%v, want %s/1/true", mac, d.Type, d.State, d.Adopted, GatewayType)
	}
	t.Logf("adopted gateway %s: model=%q type=%q state=%d", d.MAC, d.Model, d.Type, d.State)
}
