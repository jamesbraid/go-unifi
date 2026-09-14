package unifi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

// DeviceTag is a v2 API object (see overrides/resources/DeviceTag.json).
// The only published verbs are List, below, and the assignment command;
// no single-tag get, create, update or delete has ever been measured
// against a live controller, so the generator emits none of them either
// (see handWrittenCRUD in cmd/fields/main.go) -- including the usual
// generated masked-update helper, which would otherwise guess at an
// unmeasured per-tag endpoint.

type deviceTagAssignment struct {
	Additions []string `json:"device_tag_additions"`
	Removals  []string `json:"device_tag_removals"`
}

func (c *ApiClient) ListDeviceTags(ctx context.Context, site string) ([]DeviceTag, error) {
	var respBody []DeviceTag

	err := c.do(ctx, "GET", fmt.Sprintf("v2/api/site/%s/device-tags", site), nil, &respBody)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

// AssignDeviceTag adds and removes device tag IDs for the given device MAC address.
// Returns the updated list of all device tags.
func (c *ApiClient) AssignDeviceTag(ctx context.Context, site string, mac string, additions []string, removals []string) ([]DeviceTag, error) {
	var respBody []DeviceTag

	body := deviceTagAssignment{
		Additions: additions,
		Removals:  removals,
	}

	err := c.do(ctx, http.MethodPost, fmt.Sprintf("v2/api/site/%s/device-tags/device-tag-assignment/%s", site, types.NormalizeMAC(mac)), body, &respBody)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}
