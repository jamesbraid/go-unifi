package unifi

import (
	"context"
	"fmt"
	"net/http"
)

// The NetworkMembersGroup type is generated from
// overrides/resources/NetworkMembersGroup.json (a true-v2 object: wire "id",
// no envelope fields — see overrides/fields.toml). The client methods stay
// hand-written: the endpoint uses a plural path for List and a singular one
// for the per-id operations, which the generated client cannot express.

func (c *ApiClient) ListNetworkMembersGroups(ctx context.Context, site string) ([]NetworkMembersGroup, error) {
	var respBody []NetworkMembersGroup

	err := c.do(ctx, "GET", fmt.Sprintf("v2/api/site/%s/network-members-groups", site), nil, &respBody)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

func (c *ApiClient) GetNetworkMembersGroup(ctx context.Context, site string, id string) (*NetworkMembersGroup, error) {
	var respBody NetworkMembersGroup

	err := c.do(ctx, "GET", fmt.Sprintf("v2/api/site/%s/network-members-group/%s", site, id), nil, &respBody)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

func (c *ApiClient) CreateNetworkMembersGroup(ctx context.Context, site string, d *NetworkMembersGroup) (*NetworkMembersGroup, error) {
	var respBody NetworkMembersGroup
	d.ID = ""

	err := c.do(ctx, http.MethodPost, fmt.Sprintf("v2/api/site/%s/network-members-group", site), d, &respBody)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

func (c *ApiClient) UpdateNetworkMembersGroup(ctx context.Context, site string, d *NetworkMembersGroup) (*NetworkMembersGroup, error) {
	var respBody NetworkMembersGroup

	err := c.do(ctx, "PUT", fmt.Sprintf("v2/api/site/%s/network-members-group/%s", site, d.ID), d, &respBody)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

func (c *ApiClient) DeleteNetworkMembersGroup(ctx context.Context, site string, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("v2/api/site/%s/network-members-group/%s", site, id), nil, nil)
}
