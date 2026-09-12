// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

type DNSRecord struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Enabled    bool   `json:"enabled"`
	Key        string `json:"key,omitempty"`         // .{1,128}
	Port       *int64 `json:"port,omitempty"`        // [1-9][0-9]{0,4}
	Priority   int64  `json:"priority,omitempty"`    // .{1,128}
	RecordType string `json:"record_type,omitempty"` // A|AAAA|CNAME|MX|NS|SRV|TXT
	Ttl        int64  `json:"ttl,omitempty"`
	Value      string `json:"value,omitempty"` // .{1,256}
	Weight     int64  `json:"weight,omitempty"`
}

func (dst *DNSRecord) UnmarshalJSON(b []byte) error {
	type Alias DNSRecord
	aux := &struct {
		Port     *types.Number `json:"port"`
		Priority types.Number  `json:"priority"`
		Ttl      types.Number  `json:"ttl"`
		Weight   types.Number  `json:"weight"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.Port != nil {
		if val, err := aux.Port.Int64(); err == nil {
			dst.Port = &val
		} else if string(*aux.Port) == "" {
			var zero int64
			dst.Port = &zero
		}
	}
	if val, err := aux.Priority.Int64(); err == nil {
		dst.Priority = val
	}
	if val, err := aux.Ttl.Int64(); err == nil {
		dst.Ttl = val
	}
	if val, err := aux.Weight.Int64(); err == nil {
		dst.Weight = val
	}

	return nil
}

func (c *ApiClient) ListDNSRecord(ctx context.Context, site string) ([]DNSRecord, error) {
	return bareList[DNSRecord](ctx, c, fmt.Sprintf("v2/api/site/%s/static-dns", site))
}

func (c *ApiClient) GetDNSRecord(ctx context.Context, site string, id string) (*DNSRecord, error) {
	stored, err := c.ListDNSRecord(ctx, site)
	if err != nil {
		return nil, err
	}
	return findByID(stored, id, func(v *DNSRecord) string { return v.ID })
}

func (c *ApiClient) DeleteDNSRecord(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("v2/api/site/%s/static-dns/%s", site, id))
}

func (c *ApiClient) CreateDNSRecord(ctx context.Context, site string, d *DNSRecord) (*DNSRecord, error) {
	return bareOne[DNSRecord](ctx, c, http.MethodPost, fmt.Sprintf("v2/api/site/%s/static-dns", site), d)
}

// UpdateDNSRecordFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateDNSRecordFields(ctx context.Context, site string, d *DNSRecord, fields ...string) (*DNSRecord, error) {
	return bareMasked(ctx, c, fmt.Sprintf("v2/api/site/%s/static-dns/%s", site, d.ID), d, fields)
}

func (c *ApiClient) UpdateDNSRecord(ctx context.Context, site string, d *DNSRecord) (*DNSRecord, error) {
	return bareOne[DNSRecord](ctx, c, http.MethodPut, fmt.Sprintf("v2/api/site/%s/static-dns/%s", site, d.ID), d)
}
