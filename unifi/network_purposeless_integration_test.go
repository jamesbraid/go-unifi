//go:build integration

// unifi/network_purposeless_integration_test.go
package unifi

import (
	"context"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationNetworkWithNoPurposeRoundTrips answers the question
// Network.MarshalJSON used to get wrong: can the client write back a network
// it can read?
//
// Measured on 10.6.101: a create whose body names no "purpose" key at all is
// accepted (HTTP 200), and neither the create response nor a later GET
// carries a "purpose" key either -- the controller stores the object with
// nothing to dispatch the encoder on. Before this test, decoding that object
// into a Network left Purpose as "", and MarshalJSON treated any Purpose
// absent from networkPurposeFields as an error: "unknown network purpose: ".
// GetNetwork could read the object; UpdateNetwork could never write it back,
// including a write that changes nothing at all.
//
// A network like this is not something this SDK can create -- every purpose
// constant it knows sends the key -- so the only way there is a document
// this client did not write in the first place. That is exactly why the
// fallback matters: this client has to survive documents it did not create.
func TestIntegrationNetworkWithNoPurposeRoundTrips(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 15*time.Minute)
	client := harnessClient(ctx, t, c)

	path := "/api/s/" + c.Site + "/rest/networkconf"
	seed := map[string]any{
		"name": "no-purpose-rt", "enabled": true,
		"ip_subnet": "10.79.5.1/24", "vlan_enabled": true, "vlan": 795,
		"networkgroup": "LAN",
	}
	body, status, err := s.PostJSON(ctx, path, seed)
	mustTransport(t, err)
	if status != 200 {
		t.Fatalf("a purposeless create is the whole premise of this test; it was rejected (HTTP %d): %v", status, body)
	}
	created := firstData(t, body)
	id := objectID(created)
	if id == "" {
		t.Fatalf("no id in create response: %v", created)
	}
	defer s.DeleteJSON(context.WithoutCancel(ctx), path+"/"+id) //nolint:errcheck

	if _, has := created["purpose"]; has {
		t.Fatalf("the create response already carries a purpose key (%v); "+
			"this harness no longer reproduces the case this test measures", created["purpose"])
	}

	read := storedReader(ctx, t, s, c.Site, "networkconf", id)
	baseline := read()

	net, err := client.GetNetwork(ctx, c.Site, id)
	if err != nil {
		t.Fatalf("GetNetwork: %v", err)
	}
	if net.Purpose != "" {
		t.Fatalf("expected Purpose to decode as the empty string, got %q", net.Purpose)
	}

	// The regression this guards: a write that changes nothing must still
	// succeed, and must leave every field the controller already held
	// exactly as it was. Before the fix this failed before a single byte
	// reached the wire, with "unknown network purpose: ".
	if _, err := client.UpdateNetwork(ctx, c.Site, net); err != nil {
		t.Fatalf("UpdateNetwork on an unmodified purposeless network: %v", err)
	}

	afterNoop := read()
	for wire, want := range baseline {
		if wire == "purpose" {
			continue
		}
		if got, ok := afterNoop[wire]; !ok || !jsonEqual(got, want) {
			t.Errorf("a no-op UpdateNetwork changed %s: was %v, now %v (present=%v)", wire, want, got, ok)
		}
	}
	if _, has := afterNoop["purpose"]; has {
		t.Errorf("a no-op UpdateNetwork invented a purpose key (%v) where none was stored", afterNoop["purpose"])
	}

	// A real read-modify-write, not just a no-op: change one field the
	// caller actually cares about and confirm it lands without disturbing
	// anything else or the still-absent purpose.
	net, err = client.GetNetwork(ctx, c.Site, id)
	if err != nil {
		t.Fatalf("GetNetwork (second read): %v", err)
	}
	net.Name = strPtr("no-purpose-rt-renamed")
	if _, err := client.UpdateNetwork(ctx, c.Site, net); err != nil {
		t.Fatalf("UpdateNetwork renaming a purposeless network: %v", err)
	}

	afterRename := read()
	if got, _ := afterRename["name"].(string); got != "no-purpose-rt-renamed" {
		t.Errorf("rename did not land: name is %q", got)
	}
	for wire, want := range baseline {
		if wire == "name" || wire == "purpose" {
			continue
		}
		if got, ok := afterRename[wire]; !ok || !jsonEqual(got, want) {
			t.Errorf("renaming changed %s: was %v, now %v (present=%v)", wire, want, got, ok)
		}
	}
	if _, has := afterRename["purpose"]; has {
		t.Errorf("renaming invented a purpose key (%v) where none was stored", afterRename["purpose"])
	}
}
