//go:build integration

// unifi/network_openvpn_radius_integration_test.go
package unifi

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// openVPNRadiusCase is one openvpn-server precondition: which RADIUS profile
// the VPN server points at, whether the site's own RADIUS server is running,
// and what the controller was measured to do with that combination.
//
// These are cross-object rules, not the missing-partner pairings in
// networkCrossFieldRules: what decides the outcome is a field on the
// referenced radiusprofile, not a sibling in the network payload. That is
// why they live in their own table.
type openVPNRadiusCase struct {
	name string
	// profile selects the radiusprofile the network references:
	// "none", "builtin" (use_usg_auth_server, i.e. local credentials), or
	// "external" (a profile pointing at a real RADIUS server).
	profile string
	// siteRadius is whether the site's built-in RADIUS server is enabled.
	siteRadius bool
	// want is a measured api.err.* code, or "accepted".
	want string
}

// networkOpenVPNRadiusRules pins what an openvpn-server remote-user-vpn
// network requires before the controller will create it.
//
// The rule is easy to state wrongly, and was, twice. "openvpn-server needs
// the site RADIUS server enabled" is false: it needs a RADIUS *profile*, and
// only if that profile is the built-in one does the site server have to be
// running. Local credentials are the controller's own RADIUS server, which
// is why that path needs it and an external profile does not. The UniFi UI
// says as much when you create one -- "RADIUS Server must be enabled in
// order to set up a VPN Server with local credentials" -- and enables it for
// you as part of the flow.
//
// The last row is the one that keeps this honest. Without it, a reader sees
// two rejections and "simplifies" the rule into something that would make
// every external-RADIUS deployment look broken.
var networkOpenVPNRadiusRules = []openVPNRadiusCase{
	{name: "no radius profile", profile: "none", siteRadius: false, want: "api.err.RadiusProfileRequired"},
	{name: "builtin profile, site radius off", profile: "builtin", siteRadius: false, want: "api.err.RadiusServerNotEnabled"},
	{name: "builtin profile, site radius on", profile: "builtin", siteRadius: true, want: "accepted"},
	{name: "external profile, site radius off", profile: "external", siteRadius: false, want: "accepted"},
}

// TestIntegrationNetworkOpenVPNRadiusRules measures each combination against
// a live controller. The cases mutate a site-wide setting, so they run in
// sequence rather than in parallel.
func TestIntegrationNetworkOpenVPNRadiusRules(t *testing.T) {
	if os.Getenv("UNIFI_TEST_URL") != "" {
		t.Skip("mutating probe only runs against the disposable container")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := controllertest.StartForHarness(ctx, t)
	s := c.NewSession(ctx, t)

	builtin := builtinRadiusProfileID(ctx, t, s, c.Site)
	external := createExternalRadiusProfile(ctx, t, s, c.Site)

	for i, tc := range networkOpenVPNRadiusRules {
		t.Run(tc.name, func(t *testing.T) {
			setSiteRadiusEnabled(ctx, t, s, c.Site, tc.siteRadius)

			n := 120 + i
			payload := map[string]any{
				"name":                      fmt.Sprintf("ovpn-radius-%d", n),
				"purpose":                   PurposeUserVPN,
				"enabled":                   true,
				"vpn_type":                  "openvpn-server",
				"openvpn_mode":              "server",
				"openvpn_encryption_cipher": "AES_256_CBC",
				"ip_subnet":                 fmt.Sprintf("10.%d.0.1/24", n),
				// The controller validates the listener before it looks at
				// authentication: without local_port every case below comes
				// back api.err.MissingLocalPort and measures nothing.
				"local_port": 1194,
			}
			switch tc.profile {
			case "builtin":
				payload["radiusprofile_id"] = builtin
			case "external":
				payload["radiusprofile_id"] = external
			case "none":
				// deliberately absent
			default:
				t.Fatalf("unknown profile kind %q", tc.profile)
			}

			body, status, err := s.PostJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf", payload)
			if err != nil {
				t.Fatalf("transport: %v", err)
			}

			got := "accepted"
			if status != 200 {
				got = errCode(body)
			} else if created := firstData(t, body); created != nil {
				if id, _ := created["_id"].(string); id != "" {
					defer s.DeleteJSON(ctx, "/api/s/"+c.Site+"/rest/networkconf/"+id) //nolint:errcheck
				}
			}

			if got != tc.want {
				t.Errorf("profile=%s site_radius=%v: got %q, want %q", tc.profile, tc.siteRadius, got, tc.want)
			}
		})
	}
}

// builtinRadiusProfileID returns the id of the stock profile that
// authenticates against the controller's own RADIUS server -- the "local
// credentials" path. Every site ships one.
func builtinRadiusProfileID(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()

	body, status, err := s.GetJSON(ctx, "/api/s/"+site+"/rest/radiusprofile")
	if err != nil || status != 200 {
		t.Fatalf("list radiusprofile: status %d, %v", status, err)
	}
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("radiusprofile list is not an object: %v", body)
	}
	items, _ := m["data"].([]any)
	for _, item := range items {
		profile, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if profile["use_usg_auth_server"] == true {
			if id, _ := profile["_id"].(string); id != "" {
				return id
			}
		}
	}
	t.Fatalf("no built-in (use_usg_auth_server) radius profile on this site; the local-credentials rows cannot be measured")
	return ""
}

// createExternalRadiusProfile makes a profile pointing at a RADIUS server
// that is not the controller's own. Nothing ever authenticates against it --
// the point is only that the network references a non-built-in profile.
func createExternalRadiusProfile(ctx context.Context, t *testing.T, s *controllertest.Session, site string) string {
	t.Helper()

	body, status, err := s.PostJSON(ctx, "/api/s/"+site+"/rest/radiusprofile", map[string]any{
		"name":                "external-probe",
		"use_usg_auth_server": false,
		"use_usg_acct_server": false,
		"auth_servers": []map[string]any{
			{"ip": "192.0.2.10", "port": 1812, "x_secret": "probe-secret"},
		},
	})
	if err != nil || status != 200 {
		t.Fatalf("create external radiusprofile: status %d, %v, body %v", status, err, body)
	}
	created := firstData(t, body)
	id, _ := created["_id"].(string)
	if id == "" {
		t.Fatalf("external radiusprofile has no id: %v", created)
	}
	t.Cleanup(func() {
		s.DeleteJSON(context.WithoutCancel(ctx), "/api/s/"+site+"/rest/radiusprofile/"+id) //nolint:errcheck
	})
	return id
}

// setSiteRadiusEnabled turns the site's built-in RADIUS server on or off.
func setSiteRadiusEnabled(ctx context.Context, t *testing.T, s *controllertest.Session, site string, enabled bool) {
	t.Helper()

	body, status, err := s.PutJSON(ctx, "/api/s/"+site+"/set/setting/radius", map[string]any{
		"key":     "radius",
		"enabled": enabled,
	})
	if err != nil || status != 200 {
		t.Fatalf("set site radius enabled=%v: status %d, %v, body %v", enabled, status, err, body)
	}
}
