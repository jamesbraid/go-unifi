//go:build integration

// unifi/clearing_client_integration_test.go
package unifi

import (
	"context"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationClearingThroughTheClient asks whether a caller holding this
// client can actually clear a field the controller clears on "".
//
// TestIntegrationClearingSemantics already answers the controller half of
// that question, by PUTting raw documents: on 10.6.101 domain_name,
// mac_override, native_networkconf_id and charged_as all accept "", all
// clear on it, and all keep their stored value when the key is left out.
// None of that says the client can express it, and it takes a different
// method for each. A full write drops an empty string for most fields, so
// through UpdateNetwork, UpdatePortProfile or UpdateHotspotPackage the
// caller's "" never reaches the wire and the write comes back 200 having
// changed nothing. Only domain_name is clearable that way, and only since
// it joined networkClearableSlots. The other three need the masked write.
//
// So this probe drives the public methods instead of the wire, and measures
// three things per field, in order:
//
//	SEEDED     a non-default value written through the client is stored
//	SURVIVES   a write that does not name the field leaves it alone
//	CLEARED    the clearing call empties it in the stored document
//
// SURVIVES is the one that would catch this going wrong. Making an empty
// string reach the wire is easy; making it reach the wire ONLY when the
// caller asked for it is the whole problem, and a fix that sent "" on every
// write would trade a missing feature for silent data loss. Each SURVIVES
// step therefore also asserts that its write landed -- a rejected write, or
// one the encoder rendered into nothing, preserves every field there is, and
// would pass an assertion that only looked at the field under test.
//
// Every verdict comes from a plain GET of the stored document. Never from
// the write's own response: a v1 PUT that changed nothing answers 200 with
// an empty data array, so a field read out of that response looks cleared
// whatever the controller did. That is the exact mistake that put
// OMIT-CLEARS in schemas/behavior.json for 39 fields the controller was
// preserving.
//
// TestIntegrationDHCPSlotsClearWithAnEmptyString is the same question asked
// of the eight DHCP slots, which were measured first and are all *string.
// This one adds the fields that are not: three of the four here are plain
// strings, and their clear goes through the masked write instead.
func TestIntegrationClearingThroughTheClient(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 20*time.Minute)
	client := harnessClient(ctx, t, c)

	// domain_name is a *string, so the encoder can tell "the caller wants it
	// empty" from "the caller never set it" and the ordinary full write
	// carries the clear. It is the only one of the four in that position.
	t.Run("networkconf/domain_name", func(t *testing.T) {
		created, err := client.CreateNetwork(ctx, c.Site, &Network{
			Name: strPtr("clear-client-domain"), Purpose: PurposeCorporate, Enabled: true,
			IPSubnet: strPtr("10.98.1.1/24"), VLANEnabled: true, VLAN: ptrInt64(981),
			NetworkGroup: strPtr("LAN"), DomainName: strPtr("clear.example"),
		})
		if err != nil {
			t.Fatalf("CreateNetwork: %v", err)
		}
		defer client.DeleteNetwork(context.WithoutCancel(ctx), c.Site, created.ID, "") //nolint:errcheck
		read := networkReader(ctx, t, s, c.Site, created.ID)

		wantStored(t, read(), "domain_name", "clear.example", "seeded through CreateNetwork")

		// A caller who never set the field. UpdateNetwork writes the whole
		// object, so this is the write that would clear domain_name if the
		// encoder had started sending "" unconditionally.
		net := mustGetNetwork(ctx, t, client, c.Site, created.ID)
		net.DomainName = nil
		net.Name = strPtr("clear-client-domain-renamed")
		if _, err := client.UpdateNetwork(ctx, c.Site, net); err != nil {
			t.Fatalf("UpdateNetwork leaving domain_name unset: %v", err)
		}
		after := read()
		wantWriteLanded(t, after, "name", "clear-client-domain-renamed")
		wantStored(t, after, "domain_name", "clear.example",
			"a full write that left DomainName nil")

		// And the clear the caller came for.
		net = mustGetNetwork(ctx, t, client, c.Site, created.ID)
		net.DomainName = strPtr("")
		if _, err := client.UpdateNetwork(ctx, c.Site, net); err != nil {
			t.Fatalf("UpdateNetwork clearing domain_name: %v", err)
		}
		wantCleared(t, read(), "domain_name", "UpdateNetwork with DomainName pointing at \"\"")
	})

	// The other three are plain strings on the generated struct. A plain
	// string holds no "the caller asked for empty", so the full write cannot
	// carry the clear and must not try; the masked write can, because it
	// force-sends the zero value of a field the mask names.
	t.Run("networkconf/mac_override", func(t *testing.T) {
		created, err := client.CreateNetwork(ctx, c.Site, &Network{
			Name: strPtr("clear-client-mac"), Purpose: PurposeCorporate, Enabled: true,
			IPSubnet: strPtr("10.98.2.1/24"), VLANEnabled: true, VLAN: ptrInt64(982),
			NetworkGroup:       strPtr("LAN"),
			MACOverrideEnabled: true, MACOverride: "00:11:22:33:44:66",
		})
		if err != nil {
			t.Fatalf("CreateNetwork: %v", err)
		}
		defer client.DeleteNetwork(context.WithoutCancel(ctx), c.Site, created.ID, "") //nolint:errcheck
		read := networkReader(ctx, t, s, c.Site, created.ID)

		wantStored(t, read(), "mac_override", "00:11:22:33:44:66", "seeded through CreateNetwork")

		net := mustGetNetwork(ctx, t, client, c.Site, created.ID)
		net.MACOverride = ""
		net.Name = strPtr("clear-client-mac-renamed")
		if _, err := client.UpdateNetwork(ctx, c.Site, net); err != nil {
			t.Fatalf("UpdateNetwork with an empty MACOverride: %v", err)
		}
		after := read()
		wantWriteLanded(t, after, "name", "clear-client-mac-renamed")
		wantStored(t, after, "mac_override", "00:11:22:33:44:66",
			"a full write whose MACOverride was the zero string")

		net = mustGetNetwork(ctx, t, client, c.Site, created.ID)
		net.MACOverride = ""
		if _, err := client.UpdateNetworkFields(ctx, c.Site, net, "mac_override"); err != nil {
			t.Fatalf("UpdateNetworkFields clearing mac_override: %v", err)
		}
		wantCleared(t, read(), "mac_override", `UpdateNetworkFields naming "mac_override"`)
	})

	t.Run("portconf/native_networkconf_id", func(t *testing.T) {
		native := clearingClientNetwork(ctx, t, client, c.Site,
			"clear-client-native", "10.98.3.1/24", 983)

		created, err := client.CreatePortProfile(ctx, c.Site, &PortProfile{
			Name: "clear-client-port", Forward: "all", PoeMode: "auto", OpMode: "switch",
			NATiveNetworkID: native,
		})
		if err != nil {
			t.Fatalf("CreatePortProfile: %v", err)
		}
		defer client.DeletePortProfile(context.WithoutCancel(ctx), c.Site, created.ID) //nolint:errcheck
		read := storedReader(ctx, t, s, c.Site, "portconf", created.ID)

		wantStored(t, read(), "native_networkconf_id", native, "seeded through CreatePortProfile")

		profile, err := client.GetPortProfile(ctx, c.Site, created.ID)
		if err != nil {
			t.Fatalf("GetPortProfile: %v", err)
		}
		profile.NATiveNetworkID = ""
		profile.Name = "clear-client-port-renamed"
		if _, err := client.UpdatePortProfile(ctx, c.Site, profile); err != nil {
			t.Fatalf("UpdatePortProfile with an empty NATiveNetworkID: %v", err)
		}
		after := read()
		wantWriteLanded(t, after, "name", "clear-client-port-renamed")
		wantStored(t, after, "native_networkconf_id", native,
			"a full write whose NATiveNetworkID was the zero string")

		profile.NATiveNetworkID = ""
		if _, err := client.UpdatePortProfileFields(ctx, c.Site, profile, "native_networkconf_id"); err != nil {
			t.Fatalf("UpdatePortProfileFields clearing native_networkconf_id: %v", err)
		}
		wantCleared(t, read(), "native_networkconf_id",
			`UpdatePortProfileFields naming "native_networkconf_id"`)
	})

	t.Run("hotspotpackage/charged_as", func(t *testing.T) {
		// The controller's sanitizer refuses a package carrying both
		// duration fields and refuses one carrying neither, so
		// TrialDurationMinutes is here and Hours must stay nil.
		created, err := client.CreateHotspotPackage(ctx, c.Site, &HotspotPackage{
			Name: "clear-client-package", ChargedAs: "hour", Currency: "USD",
			TrialDurationMinutes: ptrInt64(60), TrialReset: 24,
		})
		if err != nil {
			t.Fatalf("CreateHotspotPackage: %v", err)
		}
		defer client.DeleteHotspotPackage(context.WithoutCancel(ctx), c.Site, created.ID) //nolint:errcheck
		read := storedReader(ctx, t, s, c.Site, "hotspotpackage", created.ID)

		wantStored(t, read(), "charged_as", "hour", "seeded through CreateHotspotPackage")

		pkg, err := client.GetHotspotPackage(ctx, c.Site, created.ID)
		if err != nil {
			t.Fatalf("GetHotspotPackage: %v", err)
		}
		pkg.ChargedAs = ""
		pkg.Name = "clear-client-package-renamed"
		if _, err := client.UpdateHotspotPackage(ctx, c.Site, pkg); err != nil {
			t.Fatalf("UpdateHotspotPackage with an empty ChargedAs: %v", err)
		}
		after := read()
		wantWriteLanded(t, after, "name", "clear-client-package-renamed")
		wantStored(t, after, "charged_as", "hour",
			"a full write whose ChargedAs was the zero string")

		// The mask names the duration as well, and has to. The sanitizer
		// runs on the request body rather than on the merged result, so a
		// masked write that carries neither duration field is refused with
		// api.err.InvalidHotspotPackageDuration even though the stored
		// package has one -- measured on 10.6.101. Masking this collection
		// means carrying whichever duration the package uses.
		pkg.ChargedAs = ""
		if _, err := client.UpdateHotspotPackageFields(ctx, c.Site, pkg,
			"charged_as", "trial_duration_minutes"); err != nil {
			t.Fatalf("UpdateHotspotPackageFields clearing charged_as: %v", err)
		}
		wantCleared(t, read(), "charged_as", `UpdateHotspotPackageFields naming "charged_as"`)
	})

	// A caller must still say "empty" with a pointer to "", even through a
	// mask. A mask that names a *string the caller left nil sends JSON null,
	// because null is the zero value of a pointer -- and 10.6.101 refuses
	// that with api.err.InvalidValue where it would have accepted "".
	//
	// This corner is reachable on domain_name only because the encoder now
	// emits its explicit "": before, the mask rejected the name client-side,
	// since the probe that asks "would the encoder send this field" points
	// the pointer at "" and watches the encoder drop it again. The same
	// applies to the eight DHCP slots, which have always been in this
	// position.
	//
	// Pinned rather than fixed. Making zeroJSONFor render "" for a *string
	// would make the two write paths agree here, but it would change what
	// every masked write on every type sends for a nil pointer, and nothing
	// has measured what the other collections do with "" where they take
	// null today -- trafficroutes stores null for a cleared string, so at
	// least one of them means something by it. What matters for now is that
	// the refusal is loud and the stored value survives it.
	t.Run("networkconf/domain_name via a mask naming an unset pointer", func(t *testing.T) {
		created, err := client.CreateNetwork(ctx, c.Site, &Network{
			Name: strPtr("clear-client-domain-null"), Purpose: PurposeCorporate, Enabled: true,
			IPSubnet: strPtr("10.98.4.1/24"), VLANEnabled: true, VLAN: ptrInt64(984),
			NetworkGroup: strPtr("LAN"), DomainName: strPtr("null.example"),
		})
		if err != nil {
			t.Fatalf("CreateNetwork: %v", err)
		}
		defer client.DeleteNetwork(context.WithoutCancel(ctx), c.Site, created.ID, "") //nolint:errcheck
		read := networkReader(ctx, t, s, c.Site, created.ID)

		net := mustGetNetwork(ctx, t, client, c.Site, created.ID)
		net.DomainName = nil
		if _, err := client.UpdateNetworkFields(ctx, c.Site, net, "domain_name"); err == nil {
			t.Errorf("a mask naming an unset domain_name was accepted, and the stored value is "+
				"now %v. Measured on 10.6.101 it is refused with api.err.InvalidValue, because "+
				"the mask sends null for a nil pointer. If this now works, the masked path can "+
				"express the clear too and this test should say so.", read()["domain_name"])
		}
		wantStored(t, read(), "domain_name", "null.example",
			"a mask the controller refused")
	})
}

// storedReader returns a function that reads one v1 object back as the
// controller holds it.
//
// The indirection is the point: nothing in this probe may take a verdict
// from a write's own response, and a helper that only knows how to GET
// cannot be used that way by accident.
func storedReader(
	ctx context.Context,
	t *testing.T,
	s *controllertest.Session,
	site, collection, id string,
) func() map[string]any {
	t.Helper()
	path := "/api/s/" + site + "/rest/" + collection + "/" + id
	return func() map[string]any {
		body, status, err := s.GetJSON(ctx, path)
		mustTransport(t, err)
		doc := firstData(t, body)
		if status != 200 || doc == nil {
			t.Fatalf("re-reading %s answered HTTP %d with %v; a verdict cannot be taken from a "+
				"document that did not come back", path, status, body)
		}
		return doc
	}
}

func networkReader(
	ctx context.Context,
	t *testing.T,
	s *controllertest.Session,
	site, id string,
) func() map[string]any {
	t.Helper()
	return storedReader(ctx, t, s, site, "networkconf", id)
}

// mustGetNetwork reads a network back through the client, which is how a
// caller gets the object it is about to edit.
func mustGetNetwork(ctx context.Context, t *testing.T, client *ApiClient, site, id string) *Network {
	t.Helper()
	net, err := client.GetNetwork(ctx, site, id)
	if err != nil {
		t.Fatalf("GetNetwork: %v", err)
	}
	return net
}

// clearingClientNetwork creates a corporate network for another object to
// point at, and returns its id.
func clearingClientNetwork(
	ctx context.Context,
	t *testing.T,
	client *ApiClient,
	site, name, subnet string,
	vlan int64,
) string {
	t.Helper()
	net, err := client.CreateNetwork(ctx, site, &Network{
		Name: strPtr(name), Purpose: PurposeCorporate, Enabled: true,
		IPSubnet: strPtr(subnet), VLANEnabled: true, VLAN: ptrInt64(vlan),
		NetworkGroup: strPtr("LAN"),
	})
	if err != nil {
		t.Fatalf("create the network %s to reference: %v", name, err)
	}
	t.Cleanup(func() {
		client.DeleteNetwork(context.WithoutCancel(ctx), site, net.ID, "") //nolint:errcheck
	})
	return net.ID
}

// wantStored fails when the stored document does not hold want for wire.
func wantStored(t *testing.T, doc map[string]any, wire string, want any, after string) {
	t.Helper()
	if !jsonEqual(doc[wire], want) {
		t.Errorf("%s is %v after %s, want %v", wire, doc[wire], after, want)
	}
}

// wantCleared fails when the field is still set. blankValue is what decides:
// the controller spells a cleared field as "" on some collections and as a
// dropped key on others, and both mean the same thing to a caller.
func wantCleared(t *testing.T, doc map[string]any, wire, how string) {
	t.Helper()
	if !blankValue(doc[wire]) {
		t.Errorf("%s is still %v after %s.\n\n"+
			"The controller clears this field on an explicit \"\" and preserves it when the key "+
			"is absent, so a caller with no way to send \"\" cannot clear it at all -- they get a "+
			"200 and no change.", wire, doc[wire], how)
	}
}

// wantWriteLanded fails when the write meant to leave a field alone did not
// itself do anything. Preservation measured across a write the controller
// refused is not a measurement.
func wantWriteLanded(t *testing.T, doc map[string]any, wire string, want any) {
	t.Helper()
	if !jsonEqual(doc[wire], want) {
		t.Fatalf("the write did not land: %s is %v, want %v. Nothing this write preserved counts "+
			"for anything until it is known to have changed something.", wire, doc[wire], want)
	}
}
