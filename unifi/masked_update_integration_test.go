//go:build integration

package unifi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
	"github.com/ubiquiti-community/go-unifi/unifi/settings"
)

// TestIntegrationSettingPartialWriteMerges pins set/setting/<key> as merging
// a partial body: a field the body leaves out keeps its stored value. That
// is what lets UpdateSettingFields be a direct masked PUT rather than a
// read-modify-write.
//
// Measured on ntp, mgmt and radius when the method was written; ntp is the
// one pinned here because every field is a scalar with an obvious non-default.
func TestIntegrationSettingPartialWriteMerges(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	seed := map[string]any{
		"key": "ntp", "setting_preference": "manual",
		"ntp_server_1": "1.1.1.1", "ntp_server_2": "2.2.2.2",
		"ntp_server_3": "3.3.3.3", "ntp_server_4": "4.4.4.4",
	}
	if body, status, err := s.PutJSON(ctx, "/api/s/"+c.Site+"/set/setting/ntp", seed); err != nil || status != 200 {
		t.Fatalf("seed rejected (HTTP %d): %v %v", status, body, err)
	}

	client := harnessClient(ctx, t, c)
	// Only the first server is meant. Every other field on this struct is
	// zero, and would have been written as such by a full UpdateSetting.
	ntp := &settings.Ntp{NtpServer1: "5.5.5.5"}
	if err := client.UpdateSettingFields(ctx, c.Site, ntp, "ntp_server_1"); err != nil {
		t.Fatalf("UpdateSettingFields: %v", err)
	}

	body, status, err := s.GetJSON(ctx, "/api/s/"+c.Site+"/get/setting/ntp")
	if err != nil || status != 200 {
		t.Fatalf("re-read failed (HTTP %d): %v", status, err)
	}
	after := firstData(t, body)
	if after["ntp_server_1"] != "5.5.5.5" {
		t.Errorf("the named field did not land: ntp_server_1 = %v", after["ntp_server_1"])
	}
	for _, wire := range []string{"ntp_server_2", "ntp_server_3", "ntp_server_4", "setting_preference"} {
		if !jsonEqual(after[wire], seed[wire]) {
			t.Errorf("%s did not survive a masked write that omitted it: was %v, now %v.\n\n"+
				"set/setting no longer merges. UpdateSettingFields has to take the "+
				"read-overlay path (see overlayMasked) instead of a direct masked PUT.",
				wire, seed[wire], after[wire])
		}
	}
	// The response refreshed the struct, so the caller sees the merged section.
	if ntp.NtpServer2 != "2.2.2.2" || ntp.SettingPreference != "manual" {
		t.Errorf("section not refreshed from the controller's answer: %+v", ntp)
	}
}

// TestIntegrationWireGuardPeerPartialWriteRejected pins why
// UpdateWireGuardPeerFields reads before it writes: the peer batch PUT does
// not merge. A body carrying only _id and name is refused outright and
// nothing is stored, so the mask can only be honoured by writing the whole
// peer back. The second half proves the method does exactly that.
func TestIntegrationWireGuardPeerPartialWriteRejected(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", map[string]any{
		"name": "masked-wg", "purpose": PurposeUserVPN, "enabled": true,
		"vpn_type": "wireguard-server", "ip_subnet": "10.198.0.1/24", "local_port": 51821,
		"x_wireguard_private_key": "6KpcbNfK7kFzOlKjnDbSaYbmDbAZBOKwFqjOWMkSCFU=",
	})
	if err != nil || status != 200 {
		t.Fatalf("wireguard-server network rejected (HTTP %d): %v %v", status, body, err)
	}
	networkID, _ := firstData(t, body)["_id"].(string)
	defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+networkID) //nolint:errcheck

	client := harnessClient(ctx, t, c)
	peer, err := client.CreateWireGuardPeer(ctx, c.Site, networkID, &WireGuardPeer{
		Name: "peer1", InterfaceIP: "10.198.0.2",
		PublicKey:  "yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=",
		AllowedIPs: []string{"10.198.0.2/32"},
	})
	if err != nil {
		t.Fatalf("CreateWireGuardPeer: %v", err)
	}

	// The fact: a partial body is refused, not merged.
	path := "/v2/api/site/" + c.Site + "/wireguard/" + networkID + "/users"
	_, status, _ = s.PutJSON(ctx, path+"/batch", []any{map[string]any{"_id": peer.ID, "name": "partial"}})
	if status == 200 {
		t.Log("note: the controller now accepts a partial peer PUT; UpdateWireGuardPeerFields " +
			"could become a direct masked write once a merge is measured")
	}
	stored, err := client.GetWireGuardPeer(ctx, c.Site, networkID, peer.ID)
	if err != nil {
		t.Fatalf("GetWireGuardPeer after the partial PUT: %v", err)
	}
	if stored.Name == "partial" {
		t.Fatalf("the partial PUT landed (HTTP %d); it was measured as refused with nothing stored", status)
	}
	if stored.InterfaceIP != "10.198.0.2" {
		t.Fatalf("the partial PUT damaged the peer (HTTP %d): %+v", status, stored)
	}

	// The method: only name changes, everything else is what was stored.
	got, err := client.UpdateWireGuardPeerFields(ctx, c.Site, networkID, &WireGuardPeer{ID: peer.ID, Name: "renamed"}, "name")
	if err != nil {
		t.Fatalf("UpdateWireGuardPeerFields: %v", err)
	}
	if got.Name != "renamed" {
		t.Errorf("returned peer name = %q, want renamed", got.Name)
	}
	stored, err = client.GetWireGuardPeer(ctx, c.Site, networkID, peer.ID)
	if err != nil {
		t.Fatalf("GetWireGuardPeer after the masked write: %v", err)
	}
	if stored.Name != "renamed" || stored.InterfaceIP != "10.198.0.2" ||
		stored.PublicKey != peer.PublicKey || len(stored.AllowedIPs) != 1 || stored.AllowedIPs[0] != "10.198.0.2/32" {
		t.Errorf("masked write did not leave the unnamed fields alone: %+v", stored)
	}
}
