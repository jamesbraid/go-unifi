// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

// just to fix compile issues with the import.
var (
	_ context.Context
	_ fmt.Formatter
	_ json.Marshaler
	_ types.Number
	_ strconv.NumError
	_ http.Client
)

type ScheduleTask struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Action          string                       `json:"action,omitempty"` // upgrade
	CronExpr        string                       `json:"cron_expr,omitempty"`
	ExecuteOnlyOnce bool                         `json:"execute_only_once"`
	Name            string                       `json:"name,omitempty"`
	UpgradeTargets  []ScheduleTaskUpgradeTargets `json:"upgrade_targets,omitempty"`
}

func (dst *ScheduleTask) UnmarshalJSON(b []byte) error {
	type Alias ScheduleTask
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

type ScheduleTaskUpgradeTargets struct {
	MAC string `json:"mac,omitempty"` // ^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$
}

func (dst *ScheduleTaskUpgradeTargets) UnmarshalJSON(b []byte) error {
	type Alias ScheduleTaskUpgradeTargets
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

func (c *ApiClient) ListScheduleTask(ctx context.Context, site string) ([]ScheduleTask, error) {
	return envelopeList[ScheduleTask](ctx, c, fmt.Sprintf("api/s/%s/rest/scheduletask", site))
}

func (c *ApiClient) GetScheduleTask(ctx context.Context, site string, id string) (*ScheduleTask, error) {
	return envelopeOne[ScheduleTask](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/scheduletask/%s", site, id), nil)
}

func (c *ApiClient) DeleteScheduleTask(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/scheduletask/%s", site, id))
}

func (c *ApiClient) CreateScheduleTask(ctx context.Context, site string, d *ScheduleTask) (*ScheduleTask, error) {
	return envelopeOne[ScheduleTask](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/scheduletask", site), d)
}

// UpdateScheduleTaskFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateScheduleTaskFields(ctx context.Context, site string, d *ScheduleTask, fields ...string) (*ScheduleTask, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/scheduletask/%s", site, d.ID), d, fields, func() (*ScheduleTask, error) { return c.GetScheduleTask(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateScheduleTask(ctx context.Context, site string, d *ScheduleTask) (*ScheduleTask, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/scheduletask/%s", site, d.ID), d, func() (*ScheduleTask, error) { return c.GetScheduleTask(ctx, site, d.ID) })
}
