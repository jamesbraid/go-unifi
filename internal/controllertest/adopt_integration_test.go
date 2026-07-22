//go:build integration

package controllertest

import (
	"context"
	"testing"
	"time"
)

// TestIntegrationAdoptDevice adopts one of the -sim image's seeded pending
// devices and proves AdoptDevice blocks until the controller reports the
// device connected. Two device kinds are skipped on purpose: gateways (a
// site gets one, and adopting the seeded UGW3 would reshape the site's
// topology for every other test sharing the controller) and models the
// controller flags unsupported (their adoption never completes — live
// verified: the doc lands on state=7 with adopted=true).
func TestIntegrationAdoptDevice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	c := Start(ctx, t)
	s := c.NewSession(ctx, t)

	// The seeded demo fleet informs in over the first seconds after the API
	// turns healthy, so the candidate poll gives it time to appear before
	// concluding the image seeds nothing.
	var candidate *Device
	deadline := time.Now().Add(2 * time.Minute)
	for candidate == nil {
		devices := listDevices(ctx, t, s, c.Site)
		for i := range devices {
			if !devices[i].Adopted && !devices[i].Unsupported && devices[i].Type != "ugw" {
				candidate = &devices[i]
				break
			}
		}
		if candidate == nil {
			if time.Now().After(deadline) {
				t.Skip("no pending non-gateway device seeded in this controller image")
			}
			time.Sleep(2 * time.Second)
		}
	}
	t.Logf("adopting pending %s (%s %s)", candidate.MAC, candidate.Type, candidate.Model)

	d := c.AdoptDevice(ctx, t, s, candidate.MAC)
	if d.State != 1 || !d.Adopted {
		t.Fatalf("device %s: state=%d adopted=%v, want state=1 adopted=true", candidate.MAC, d.State, d.Adopted)
	}
}
