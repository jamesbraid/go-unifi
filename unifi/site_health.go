package unifi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

// GetHealth returns the per-subsystem health metrics for a site.
//
// SiteHealth is a v1 object (see overrides/resources/SiteHealth.json), read
// via api/s/%s/stat/health. That path is neither rest/<collection> nor
// Device's stat/device special case, and the object has no create, update
// or delete at all, so GetHealth stays entirely hand-written.
func (c *ApiClient) GetHealth(ctx context.Context, site string) ([]SiteHealth, error) {
	var respBody struct {
		Meta meta         `json:"meta"`
		Data []SiteHealth `json:"data"`
	}

	err := c.do(ctx, http.MethodGet, fmt.Sprintf("api/s/%s/stat/health", site), nil, &respBody)
	if err != nil {
		return nil, err
	}

	if err := respBody.Meta.error(); err != nil {
		return nil, err
	}

	return respBody.Data, nil
}

// identityTypesNumber passes a decoded types.Number straight through. It
// exists so overrides/fields.toml can pin a numeric field's Go type back to
// types.Number: the generator's own numeric inference always lands on
// *int64 with a types.Number-typed shadow field for the initial tolerant
// decode (real controllers mix bare numbers and quoted strings for the same
// field), then narrows it to an int64 and throws the tolerance away. An
// unmarshal_func always overrides that narrowing step, so naming this one
// keeps the field at types.Number itself instead.
func identityTypesNumber(n types.Number) types.Number { return n }
