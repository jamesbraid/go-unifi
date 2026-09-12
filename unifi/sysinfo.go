package unifi

import (
	"context"
	"fmt"
)

// sysInfo carries the one stat/sysinfo field the client reads; the endpoint
// returns much more, all ignored here.
type sysInfo struct {
	Version string `json:"version"`
}

func (c *ApiClient) sysinfo(ctx context.Context, site string) (*sysInfo, error) {
	var respBody struct {
		Meta meta      `json:"meta"`
		Data []sysInfo `json:"data"`
	}

	err := c.do(ctx, "GET", fmt.Sprintf("api/s/%s/stat/sysinfo", site), nil, &respBody)
	if err != nil {
		return nil, err
	}

	if len(respBody.Data) != 1 {
		return nil, &NotFoundError{}
	}

	return &respBody.Data[0], nil
}
