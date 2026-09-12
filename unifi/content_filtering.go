package unifi

import (
	"context"
)

func (c *ApiClient) ListContentFiltering(
	ctx context.Context,
	site string,
	query ...map[string]string,
) ([]ContentFiltering, error) {
	return c.listContentFiltering(ctx, site, query...)
}
