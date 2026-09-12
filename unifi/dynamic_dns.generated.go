// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type DynamicDNS struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	CustomService string   `json:"custom_service,omitempty"` // ^[^"' ]+$
	HostName      string   `json:"host_name,omitempty"`      // ^[^"' ]+$
	Interface     string   `json:"interface,omitempty"`      // wan[2-9]?
	Login         string   `json:"login,omitempty"`          // ^[^"' ]+$
	Options       []string `json:"options,omitempty"`        // ^[^"' ]+$
	Password      string   `json:"x_password,omitempty"`     // ^[^"' ]+$
	Server        string   `json:"server"`                   // ^[^"' ]+$|^$
	Service       string   `json:"service,omitempty"`        // afraid|changeip|cloudflare|cloudxns|ddnss|dhis|dnsexit|dnsomatic|dnspark|dnspod|dslreports|dtdns|duckdns|duiadns|dyn|dyndns|dynv6|easydns|freemyip|googledomains|loopia|namecheap|noip|nsupdate|ovh|sitelutions|spdyn|strato|tunnelbroker|zoneedit|cloudflare|custom
}

func (dst *DynamicDNS) UnmarshalJSON(b []byte) error {
	type Alias DynamicDNS
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}

	return nil
}

func (c *ApiClient) ListDynamicDNS(ctx context.Context, site string) ([]DynamicDNS, error) {
	return envelopeList[DynamicDNS](ctx, c, fmt.Sprintf("api/s/%s/rest/dynamicdns", site))
}

func (c *ApiClient) GetDynamicDNS(ctx context.Context, site string, id string) (*DynamicDNS, error) {
	return envelopeOne[DynamicDNS](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/dynamicdns/%s", site, id), nil)
}

func (c *ApiClient) DeleteDynamicDNS(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/dynamicdns/%s", site, id))
}

func (c *ApiClient) CreateDynamicDNS(ctx context.Context, site string, d *DynamicDNS) (*DynamicDNS, error) {
	return envelopeOne[DynamicDNS](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/dynamicdns", site), d)
}

// UpdateDynamicDNSFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateDynamicDNSFields(ctx context.Context, site string, d *DynamicDNS, fields ...string) (*DynamicDNS, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/dynamicdns/%s", site, d.ID), d, fields, func() (*DynamicDNS, error) { return c.GetDynamicDNS(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateDynamicDNS(ctx context.Context, site string, d *DynamicDNS) (*DynamicDNS, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/dynamicdns/%s", site, d.ID), d, func() (*DynamicDNS, error) { return c.GetDynamicDNS(ctx, site, d.ID) })
}
