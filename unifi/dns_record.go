// Code generated from ace.jar fields *.json files
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ListDNSRecordRaw returns the controller's DNS collection JSON without
// decoding it through DNSRecord. Observation tooling uses this narrow seam so
// controller-added fields remain available for structural admission checks.
func (c *ApiClient) ListDNSRecordRaw(ctx context.Context, site string) (json.RawMessage, error) {
	var response json.RawMessage
	if err := c.do(
		ctx,
		http.MethodGet,
		fmt.Sprintf("v2/api/site/%s/static-dns", site),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *ApiClient) ListDNSRecord(ctx context.Context, site string) ([]DNSRecord, error) {
	return c.listDNSRecord(ctx, site)
}

func (c *ApiClient) GetDNSRecord(ctx context.Context, site, id string) (*DNSRecord, error) {
	return c.getDNSRecord(ctx, site, id)
}

func (c *ApiClient) DeleteDNSRecord(ctx context.Context, site, id string) error {
	return c.deleteDNSRecord(ctx, site, id)
}

func (c *ApiClient) CreateDNSRecord(
	ctx context.Context,
	site string,
	d *DNSRecord,
) (*DNSRecord, error) {
	return c.createDNSRecord(ctx, site, d)
}

func (c *ApiClient) UpdateDNSRecord(
	ctx context.Context,
	site string,
	d *DNSRecord,
) (*DNSRecord, error) {
	return c.updateDNSRecord(ctx, site, d)
}
