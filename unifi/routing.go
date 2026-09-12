package unifi

import (
	"context"
)

func (c *ApiClient) ListRouting(ctx context.Context, site string) ([]Routing, error) {
	return c.listRouting(ctx, site)
}

func (c *ApiClient) GetRouting(ctx context.Context, site, id string) (*Routing, error) {
	return c.getRouting(ctx, site, id)
}

func (c *ApiClient) DeleteRouting(ctx context.Context, site, id string) error {
	return c.deleteRouting(ctx, site, id)
}

func (c *ApiClient) CreateRouting(ctx context.Context, site string, d *Routing) (*Routing, error) {
	return c.createRouting(ctx, site, d)
}

func (c *ApiClient) UpdateRouting(ctx context.Context, site string, d *Routing) (*Routing, error) {
	return c.updateRouting(ctx, site, d)
}
