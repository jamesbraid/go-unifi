package unifi

import (
	"context"
	"fmt"
	"net/http"
)

// The APGroup type is generated from overrides/resources/ApGroups.json; the
// client methods stay hand-written because the v2 apgroups endpoint has no
// per-id GET (see GetAPGroup).

func (c *ApiClient) ListAPGroup(ctx context.Context, site string) ([]APGroup, error) {
	var respBody []APGroup

	err := c.do(ctx, http.MethodGet, fmt.Sprintf("v2/api/site/%s/apgroups", site), nil, &respBody)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

// GetAPGroup returns a single AP group by ID. The v2 apgroups endpoint exposes
// no per-id GET (it returns HTTP 405), so read the collection and filter by ID —
// the same approach the unifi_ap_group data source uses. PUT and DELETE on the
// per-id path are supported, so UpdateAPGroup/DeleteAPGroup below use it.
func (c *ApiClient) GetAPGroup(ctx context.Context, site, id string) (*APGroup, error) {
	groups, err := c.ListAPGroup(ctx, site)
	if err != nil {
		return nil, err
	}

	for i := range groups {
		if groups[i].ID == id {
			return &groups[i], nil
		}
	}

	return nil, &NotFoundError{}
}

func (c *ApiClient) CreateAPGroup(ctx context.Context, site string, d *APGroup) (*APGroup, error) {
	var respBody APGroup

	err := c.do(ctx, http.MethodPost, fmt.Sprintf("v2/api/site/%s/apgroups", site), d, &respBody)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

func (c *ApiClient) UpdateAPGroup(ctx context.Context, site string, d *APGroup) (*APGroup, error) {
	var respBody APGroup

	err := c.do(ctx, http.MethodPut, fmt.Sprintf("v2/api/site/%s/apgroups/%s", site, d.ID), d, &respBody)
	if err != nil {
		return nil, err
	}

	return &respBody, nil
}

func (c *ApiClient) DeleteAPGroup(ctx context.Context, site, id string) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("v2/api/site/%s/apgroups/%s", site, id), struct{}{}, nil)
}
