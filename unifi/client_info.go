package unifi

import (
	"context"
	"fmt"
	"net/http"
)

// ClientInfo is a v2 API object (see overrides/resources/ClientInfo.json).
// Read via three endpoints -- clients/active, clients/local/{mac} and
// clients/history -- each with its own fixed query parameters, so all
// three stay hand-written below; there is no create, update or delete.
//
// Its nested unifi_device_info generates as ClientInfoUnifiDeviceInfo, not
// ClientInfoDeviceInfo: the generated name is always StructName + field
// name (UnifiDeviceInfo here), and the override layer cannot rename a
// generated nested type (see the PowerSupervisor note in
// overrides/README.md). ClientInfoDeviceInfo -> ClientInfoUnifiDeviceInfo
// is a real, reported rename.

type ClientList []ClientInfo

func (c *ApiClient) ListClientInfo(ctx context.Context, site string) (ClientList, error) {
	var respBody []ClientInfo

	err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("v2/api/site/%s/clients/active", site),
		nil,
		&respBody,
		map[string]string{
			"includeUnifiDevices": "true",
		},
	)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

func (c *ApiClient) GetClientInfo(ctx context.Context, site string, mac string) (*ClientInfo, error) {
	var respBody ClientInfo

	err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("v2/api/site/%s/clients/local/%s", site, mac),
		nil,
		&respBody,
		map[string]string{
			"includeUnifiDevices": "true",
		},
	)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

// ListClientHistory returns all historical clients, including offline devices.
// The withinHours parameter controls how far back to look (0 = all time).
func (c *ApiClient) ListClientHistory(ctx context.Context, site string, withinHours int) ([]ClientInfo, error) {
	var respBody []ClientInfo

	err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("v2/api/site/%s/clients/history", site),
		nil,
		&respBody,
		map[string]string{
			"includeUnifiDevices": "true",
			"onlyNonBlocked":      "false",
			"withinHours":         fmt.Sprintf("%d", withinHours),
		},
	)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}
