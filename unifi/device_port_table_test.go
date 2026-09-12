package unifi

import (
	"encoding/json"
	"testing"
)

// TestDevicePortTableDecodesToleratedNumbers pins the contract the shadowed
// numeric fields exist for: the controller mixes plain numbers, quoted
// numbers, and placeholder strings ("", "auto") into port_table stats, and
// the decode must keep whatever parses without failing the whole device.
//
// Worth pinning because the original hand-rolled version inverted every
// error check (err != nil guarded each assignment), so all of these fields
// silently decoded to zero and no test noticed.
func TestDevicePortTableDecodesToleratedNumbers(t *testing.T) {
	body := `{
		"port_idx": 3,
		"speed": "1000",
		"rx_bytes": 12345678901,
		"poe_current": "0.00",
		"stormctrl_bcast_rate": "auto",
		"name": "Port 3"
	}`

	var pt DevicePortTable
	if err := json.Unmarshal([]byte(body), &pt); err != nil {
		t.Fatalf("decode port table: %v", err)
	}

	if pt.PortIdx != 3 {
		t.Errorf("PortIdx = %d, want 3 (plain number)", pt.PortIdx)
	}
	if pt.Speed != 1000 {
		t.Errorf("Speed = %d, want 1000 (quoted number)", pt.Speed)
	}
	if pt.RxBytes != 12345678901 {
		t.Errorf("RxBytes = %d, want 12345678901", pt.RxBytes)
	}
	if pt.StormctrlBcastRate != 0 {
		t.Errorf("StormctrlBcastRate = %d, want 0 for the placeholder %q", pt.StormctrlBcastRate, "auto")
	}
	if pt.Name != "Port 3" {
		t.Errorf("Name = %q, want the non-shadowed fields decoded as usual", pt.Name)
	}
}
