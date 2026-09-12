package unifi_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

func TestWireGuardPeerMarshalJSON(t *testing.T) {
	peer := unifi.WireGuardPeer{
		Name:        "test-peer",
		InterfaceIP: "192.0.2.10",
		PublicKey:   "ZmFrZS10ZXN0LXdpcmVndWFyZC1wdWJrZXkAAAAAAAA=",
		AllowedIPs:  []string{},
	}

	actual, err := json.Marshal(&peer)
	if err != nil {
		t.Fatal(err)
	}
	assert.JSONEq(t,
		`{"name":"test-peer","interface_ip":"192.0.2.10","public_key":"ZmFrZS10ZXN0LXdpcmVndWFyZC1wdWJrZXkAAAAAAAA=","allowed_ips":[]}`,
		string(actual))
}
