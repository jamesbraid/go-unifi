package unifi

import (
	"context"
)

type Cmd struct {
	Command  string `json:"cmd"`
	Mac      string `json:"mac,omitempty"`
	PortIdx  *int64 `json:"port_idx,omitempty"`
	FileName string `json:"filename,omitempty"`
	SiteId   string `json:"site_id,omitempty"`
}

func (c *ApiClient) ExecuteCmd(ctx context.Context, site string, mgr string, cmd Cmd) (any, error) {
	var respBody struct{}

	err := c.siteCommand(ctx, site, mgr, &cmd, &respBody)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}
