//go:build integration

package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationSiteVPNRemoteSubnets pins the two facts that decide whether
// the site-to-site encoder may drop an empty subnet list.
//
// Measured on 10.6.101: a site-to-site network created without
// remote_vpn_subnets is refused with api.err.Invalid, whether or not dynamic
// routing is on, and an explicit [] is accepted and stored. So the field is
// required and empty is a legal value -- which omitempty made impossible to
// send, since encoding/json drops an empty slice exactly as it drops a nil
// one. A caller with no remote subnets, which is the whole point of dynamic
// routing, could not create the network at all.
//
// The neighbour behaves differently and is fixed for a different reason:
// remote_site_subnets may be absent at create, but once it holds something,
// an explicit [] is the only way to empty it.
func TestIntegrationSiteVPNRemoteSubnets(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	client := harnessClient(ctx, t, c)

	base := func(name, peer string) *Network {
		vpnType, iface := "ipsec-vpn", "wan"
		exchange, profile := "ikev2", "customized"
		psk := "s3cret-psk"
		enc, hash := "aes256", "sha256"
		dh := int64(14)
		return &Network{
			Name: &name, Purpose: PurposeSiteVPN, Enabled: true,
			VPNType: &vpnType, IPSecInterface: &iface, IPSecPeerIP: &peer,
			IPSecKeyExchange: &exchange, IPSecPreSharedKey: &psk, IPSecProfile: &profile,
			IPSecEncryption: &enc, IPSecHash: &hash, IPSecDhGroup: &dh,
			IPSecEspEncryption: &enc, IPSecEspHash: &hash, IPSecEspDhGroup: &dh,
		}
	}

	// The case that could not be expressed: no remote subnets at all.
	t.Run("created with no remote subnets", func(t *testing.T) {
		n := base("subnets-empty", "203.0.113.51")
		n.IPSecDynamicRouting = true

		created, err := client.CreateNetwork(ctx, c.Site, n)
		if err != nil {
			t.Fatalf("a site-to-site network with no remote subnets was refused: %v.\n\n"+
				"The controller accepts remote_vpn_subnets as an explicit empty list; if the "+
				"encoder has gone back to dropping it, this is what that costs.", err)
		}
		defer client.DeleteNetwork(ctx, c.Site, created.ID, *created.Name) //nolint:errcheck

		if len(created.RemoteVPNSubnets) != 0 {
			t.Errorf("remote_vpn_subnets came back as %v, want empty", created.RemoteVPNSubnets)
		}
	})

	// And the neighbour: set, then emptied.
	t.Run("remote site subnets cleared with an empty list", func(t *testing.T) {
		n := base("subnets-clear", "203.0.113.52")
		n.RemoteVPNSubnets = []string{"192.0.2.0/24"}
		n.RemoteSiteSubnets = []string{"198.51.100.0/24"}

		created, err := client.CreateNetwork(ctx, c.Site, n)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		defer client.DeleteNetwork(ctx, c.Site, created.ID, *created.Name) //nolint:errcheck
		if len(created.RemoteSiteSubnets) != 1 {
			t.Fatalf("the seed did not store remote_site_subnets: %v", created.RemoteSiteSubnets)
		}

		created.RemoteSiteSubnets = []string{}
		updated, err := client.UpdateNetwork(ctx, c.Site, created)
		if err != nil {
			t.Fatalf("clearing remote_site_subnets: %v", err)
		}
		if len(updated.RemoteSiteSubnets) != 0 {
			t.Errorf("remote_site_subnets is %v after an explicit empty list; it should be cleared.\n\n"+
				"Omitting the key leaves the stored value alone, so an explicit [] is the only "+
				"way a caller can empty it.", updated.RemoteSiteSubnets)
		}
		if len(updated.RemoteVPNSubnets) != 1 {
			t.Errorf("clearing one list emptied the other: remote_vpn_subnets = %v", updated.RemoteVPNSubnets)
		}
	})
}
