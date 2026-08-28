package unifi

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type Site struct {
	ID string `json:"_id,omitempty"`

	// Hidden   bool   `json:"attr_hidden,omitempty"`
	// HiddenId string `json:"attr_hidden_id,omitempty"`
	// NoDelete bool   `json:"attr_no_delete,omitempty"`
	// NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Name        string `json:"name"`
	Description string `json:"desc"`

	// Role string `json:"role"`
}

func (c *ApiClient) ListSites(ctx context.Context) ([]Site, error) {
	var respBody struct {
		Meta meta   `json:"meta"`
		Data []Site `json:"data"`
	}

	err := c.do(ctx, "GET", "api/self/sites", nil, &respBody)
	if err != nil {
		return nil, err
	}

	return respBody.Data, nil
}

func (c *ApiClient) GetSite(ctx context.Context, id string) (*Site, error) {
	sites, err := c.ListSites(ctx)
	if err != nil {
		return nil, err
	}

	for _, s := range sites {
		if s.ID == id {
			return &s, nil
		}
	}

	return nil, &NotFoundError{
		Type:  "Site",
		Attr:  "ID",
		Value: id,
	}
}

func (c *ApiClient) GetSiteByName(ctx context.Context, name string) (*Site, error) {
	sites, err := c.ListSites(ctx)
	if err != nil {
		return nil, err
	}

	for _, s := range sites {
		if s.Name == name {
			return &s, nil
		}
	}

	return nil, &NotFoundError{
		Type:  "Site",
		Attr:  "Name",
		Value: name,
	}
}

func (c *ApiClient) CreateSite(ctx context.Context, description string) ([]Site, error) {
	reqBody := struct {
		Cmd  string `json:"cmd"`
		Desc string `json:"desc"`
	}{
		Cmd:  "add-site",
		Desc: description,
	}

	var respBody struct {
		Meta meta   `json:"meta"`
		Data []Site `json:"data"`
	}

	err := c.siteCommand(ctx, "default", "sitemgr", reqBody, &respBody)
	if err != nil {
		return nil, err
	}

	return respBody.Data, nil
}

func (c *ApiClient) DeleteSite(ctx context.Context, id string) ([]Site, error) {
	reqBody := struct {
		Cmd  string `json:"cmd"`
		Site string `json:"site"`
	}{
		Cmd:  "delete-site",
		Site: id,
	}

	var respBody struct {
		Meta meta   `json:"meta"`
		Data []Site `json:"data"`
	}

	err := c.siteCommand(ctx, "default", "sitemgr", reqBody, &respBody)
	if err != nil {
		return nil, err
	}

	return respBody.Data, nil
}

func (c *ApiClient) UpdateSite(ctx context.Context, name, description string) ([]Site, error) {
	reqBody := struct {
		Cmd  string `json:"cmd"`
		Desc string `json:"desc"`
	}{
		Cmd:  "update-site",
		Desc: description,
	}

	var respBody struct {
		Meta meta   `json:"meta"`
		Data []Site `json:"data"`
	}

	err := c.siteCommand(ctx, name, "sitemgr", reqBody, &respBody)
	if err != nil {
		return nil, err
	}

	return respBody.Data, nil
}

// UpdateSiteFields writes only the named fields of a site.
//
// A site is edited through the update-site command rather than a REST
// object, and that command takes exactly one field: desc, the display name.
// name is the site's URL slug, fixed at creation. So "desc" is the only mask
// this accepts; anything else is an error, in the terms maskedBody uses,
// rather than a silent no-op. It exists so a caller that drives every object
// through one masked-update shape can drive sites the same way.
func (c *ApiClient) UpdateSiteFields(ctx context.Context, d *Site, fields ...string) (*Site, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("a masked write needs at least one field; to write the whole object, use UpdateSite")
	}

	var unknown []string
	for _, wire := range fields {
		if wire != "desc" {
			unknown = append(unknown, wire)
		}
	}
	if len(unknown) > 0 {
		slices.Sort(unknown)
		return nil, fmt.Errorf(
			"masked write names %d field(s) a site does not write: %s.\n\n"+
				"update-site takes desc alone; name is the site's URL slug and is fixed at creation",
			len(unknown), strings.Join(unknown, ", "))
	}

	if d.Name == "" {
		return nil, fmt.Errorf("a masked site write needs the site name: it is the address of the update-site command")
	}

	sites, err := c.UpdateSite(ctx, d.Name, d.Description)
	if err != nil {
		return nil, err
	}

	for i := range sites {
		if sites[i].Name == d.Name {
			return &sites[i], nil
		}
	}

	return nil, &NotFoundError{Type: "Site", Attr: "Name", Value: d.Name}
}
