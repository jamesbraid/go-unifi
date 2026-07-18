package unifi

import "testing"

// Compile-time coverage for models and clients retained after their schemas
// disappeared from newer UniFi Network releases.
func TestLegacyResourceAPIRemainsAvailable(t *testing.T) {
	_ = HeatMap{}
	_ = HeatMapPoint{}
	_ = Map{}
	_ = Tag{}
	_ = VirtualDevice{}

	var client *ApiClient
	_ = client.GetHeatMap
	_ = client.GetHeatMapPoint
	_ = client.GetMap
	_ = client.GetTag
	_ = client.GetVirtualDevice
}
