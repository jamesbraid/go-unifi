package unifi_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

func TestPowerSupervisorMarshalJSON(t *testing.T) {
	// Create body shape: power_sources may be sent empty.
	ps := unifi.PowerSupervisor{
		ClientMAC: "00:11:22:33:44:55",
		Enabled:   true,
		Settings: unifi.PowerSupervisorSettings{
			HeartbeatInterval: 60,
			SilenceThreshold:  900,
			PowerOffDuration:  120,
		},
		PowerSources: []unifi.PowerSupervisorSource{},
	}

	actual, err := json.Marshal(&ps)
	if err != nil {
		t.Fatal(err)
	}
	assert.JSONEq(t,
		`{"client_mac":"00:11:22:33:44:55","enabled":true,`+
			`"settings":{"heartbeat_interval":60,"silence_threshold":900,"power_off_duration":120},`+
			`"power_sources":[]}`,
		string(actual))
}
