package unifi

import (
	"context"
)

func (c *ApiClient) GetWLANByName(ctx context.Context, site, name string) (*WLAN, error) {
	wlans, err := c.ListWLAN(ctx, site)
	if err != nil {
		return nil, err
	}

	for _, w := range wlans {
		if w.Name == name {
			return &w, nil
		}
	}

	return nil, &NotFoundError{
		Type:  "WLAN",
		Attr:  "Name",
		Value: name,
	}
}
