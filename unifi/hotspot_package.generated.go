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

type HotspotPackage struct {
	ID     string `json:"_id,omitempty"`
	SiteID string `json:"site_id,omitempty"`

	Hidden   bool   `json:"attr_hidden,omitempty"`
	HiddenID string `json:"attr_hidden_id,omitempty"`
	NoDelete bool   `json:"attr_no_delete,omitempty"`
	NoEdit   bool   `json:"attr_no_edit,omitempty"`

	Amount                         float64 `json:"amount,omitempty"`
	ChargedAs                      string  `json:"charged_as,omitempty"`
	Currency                       string  `json:"currency,omitempty"` // [A-Z]{3}
	CustomPaymentFieldsEnabled     bool    `json:"custom_payment_fields_enabled"`
	Hours                          *int64  `json:"hours,omitempty"`
	Index                          *int64  `json:"index,omitempty"`
	LimitDown                      *int64  `json:"limit_down,omitempty"`
	LimitOverwrite                 bool    `json:"limit_overwrite"`
	LimitQuota                     *int64  `json:"limit_quota,omitempty"`
	LimitUp                        *int64  `json:"limit_up,omitempty"`
	Name                           string  `json:"name,omitempty"`
	PaymentFieldsAddressEnabled    bool    `json:"payment_fields_address_enabled"`
	PaymentFieldsAddressRequired   bool    `json:"payment_fields_address_required"`
	PaymentFieldsCityEnabled       bool    `json:"payment_fields_city_enabled"`
	PaymentFieldsCityRequired      bool    `json:"payment_fields_city_required"`
	PaymentFieldsCountryEnabled    bool    `json:"payment_fields_country_enabled"`
	PaymentFieldsCountryRequired   bool    `json:"payment_fields_country_required"`
	PaymentFieldsEmailEnabled      bool    `json:"payment_fields_email_enabled"`
	PaymentFieldsEmailRequired     bool    `json:"payment_fields_email_required"`
	PaymentFieldsFirstNameEnabled  bool    `json:"payment_fields_first_name_enabled"`
	PaymentFieldsFirstNameRequired bool    `json:"payment_fields_first_name_required"`
	PaymentFieldsLastNameEnabled   bool    `json:"payment_fields_last_name_enabled"`
	PaymentFieldsLastNameRequired  bool    `json:"payment_fields_last_name_required"`
	PaymentFieldsStateEnabled      bool    `json:"payment_fields_state_enabled"`
	PaymentFieldsStateRequired     bool    `json:"payment_fields_state_required"`
	PaymentFieldsZipEnabled        bool    `json:"payment_fields_zip_enabled"`
	PaymentFieldsZipRequired       bool    `json:"payment_fields_zip_required"`
	TrialDurationMinutes           *int64  `json:"trial_duration_minutes,omitempty"`
	TrialReset                     float64 `json:"trial_reset,omitempty"`
}

func (dst *HotspotPackage) UnmarshalJSON(b []byte) error {
	type Alias HotspotPackage
	aux := &struct {
		Hours                *types.Number `json:"hours"`
		Index                *types.Number `json:"index"`
		LimitDown            *types.Number `json:"limit_down"`
		LimitQuota           *types.Number `json:"limit_quota"`
		LimitUp              *types.Number `json:"limit_up"`
		TrialDurationMinutes *types.Number `json:"trial_duration_minutes"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.Hours != nil {
		if val, err := aux.Hours.Int64(); err == nil {
			dst.Hours = &val
		} else if string(*aux.Hours) == "" {
			var zero int64
			dst.Hours = &zero
		}
	}
	if aux.Index != nil {
		if val, err := aux.Index.Int64(); err == nil {
			dst.Index = &val
		} else if string(*aux.Index) == "" {
			var zero int64
			dst.Index = &zero
		}
	}
	if aux.LimitDown != nil {
		if val, err := aux.LimitDown.Int64(); err == nil {
			dst.LimitDown = &val
		} else if string(*aux.LimitDown) == "" {
			var zero int64
			dst.LimitDown = &zero
		}
	}
	if aux.LimitQuota != nil {
		if val, err := aux.LimitQuota.Int64(); err == nil {
			dst.LimitQuota = &val
		} else if string(*aux.LimitQuota) == "" {
			var zero int64
			dst.LimitQuota = &zero
		}
	}
	if aux.LimitUp != nil {
		if val, err := aux.LimitUp.Int64(); err == nil {
			dst.LimitUp = &val
		} else if string(*aux.LimitUp) == "" {
			var zero int64
			dst.LimitUp = &zero
		}
	}
	if aux.TrialDurationMinutes != nil {
		if val, err := aux.TrialDurationMinutes.Int64(); err == nil {
			dst.TrialDurationMinutes = &val
		} else if string(*aux.TrialDurationMinutes) == "" {
			var zero int64
			dst.TrialDurationMinutes = &zero
		}
	}

	return nil
}

func (c *ApiClient) ListHotspotPackage(ctx context.Context, site string) ([]HotspotPackage, error) {
	return envelopeList[HotspotPackage](ctx, c, fmt.Sprintf("api/s/%s/rest/hotspotpackage", site))
}

func (c *ApiClient) GetHotspotPackage(ctx context.Context, site string, id string) (*HotspotPackage, error) {
	return envelopeOne[HotspotPackage](ctx, c, http.MethodGet, fmt.Sprintf("api/s/%s/rest/hotspotpackage/%s", site, id), nil)
}

func (c *ApiClient) DeleteHotspotPackage(ctx context.Context, site string, id string) error {
	return deleteResource(ctx, c, fmt.Sprintf("api/s/%s/rest/hotspotpackage/%s", site, id))
}

func (c *ApiClient) CreateHotspotPackage(ctx context.Context, site string, d *HotspotPackage) (*HotspotPackage, error) {
	return envelopeOne[HotspotPackage](ctx, c, http.MethodPost, fmt.Sprintf("api/s/%s/rest/hotspotpackage", site), d)
}

// UpdateHotspotPackageFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateHotspotPackageFields(ctx context.Context, site string, d *HotspotPackage, fields ...string) (*HotspotPackage, error) {
	return envelopeMasked(ctx, c, fmt.Sprintf("api/s/%s/rest/hotspotpackage/%s", site, d.ID), d, fields, func() (*HotspotPackage, error) { return c.GetHotspotPackage(ctx, site, d.ID) })
}

func (c *ApiClient) UpdateHotspotPackage(ctx context.Context, site string, d *HotspotPackage) (*HotspotPackage, error) {
	return envelopeUpdate(ctx, c, fmt.Sprintf("api/s/%s/rest/hotspotpackage/%s", site, d.ID), d, func() (*HotspotPackage, error) { return c.GetHotspotPackage(ctx, site, d.ID) })
}
