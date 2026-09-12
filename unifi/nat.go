package unifi

import (
	"context"
)

func (c *ApiClient) ListNat(
	ctx context.Context,
	site string,
	query ...map[string]string,
) ([]Nat, error) {
	return c.listNat(ctx, site, query...)
}
