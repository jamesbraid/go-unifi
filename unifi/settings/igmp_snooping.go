package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

// IgmpSnooping is the site-level IGMP snooping setting (key "igmp_snooping").
//
// Hand-maintained for source compatibility. UniFi Network 10.4.x now defines
// this setting with object-shaped querier_addresses, while released go-unifi
// versions exposed []string. UnmarshalJSON accepts both wire shapes and keeps
// the public slice as the contained querier_address values.
type IgmpSnooping struct {
	BaseSetting

	Enabled                            bool                         `json:"enabled"`
	NetworkIDs                         []string                     `json:"network_ids,omitempty"`
	FloodKnownProtocols                bool                         `json:"flood_known_protocols"`
	ForwardUnknownMcastRouterPorts     bool                         `json:"forward_unknown_mcast_router_ports"`
	FastleaveForNetworkIDs             []string                     `json:"fastleave_for_network_ids,omitempty"`
	FloodUnknownMulticastForNetworkIDs []string                     `json:"flood_unknown_multicast_for_network_ids,omitempty"`
	SubscriptionMode                   string                       `json:"subscription_mode,omitempty"`
	QuerierMode                        string                       `json:"querier_mode,omitempty"`
	QuerierSubscriptionMode            string                       `json:"querier_subscription_mode,omitempty"`
	QuerierSwitches                    []string                     `json:"querier_switches,omitempty"`
	QuerierAddresses                   []string                     `json:"querier_addresses,omitempty"`
	QuerierAddressDetails              []IgmpSnoopingQuerierAddress `json:"-"`
	querierAddressesBaseline           []string
	Switches                           []string `json:"switches,omitempty"`
	PrimaryQuerier                     string   `json:"primary_querier,omitempty"`
	FailoverQuerier                    string   `json:"failover_querier,omitempty"`
}

// IgmpSnoopingQuerierAddress retains the structured querier metadata returned
// by UniFi Network 10.4.x without changing the released QuerierAddresses API.
type IgmpSnoopingQuerierAddress struct {
	MAC            string `json:"mac,omitempty"`
	NetworkID      string `json:"network_id,omitempty"`
	QuerierAddress string `json:"querier_address,omitempty"`
	QueryInterval  *int64 `json:"query_interval,omitempty"`
}

// UnmarshalJSON accepts both the released string-array representation and the
// object-array representation returned by newer controllers. Structured
// metadata is retained in QuerierAddressDetails while QuerierAddresses remains
// a source-compatible string projection.
func (s *IgmpSnooping) UnmarshalJSON(body []byte) error {
	type alias IgmpSnooping
	aux := struct {
		QuerierAddresses json.RawMessage `json:"querier_addresses"`
		*alias
	}{alias: (*alias)(s)}
	if err := json.Unmarshal(body, &aux); err != nil {
		return err
	}
	if len(aux.QuerierAddresses) == 0 {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(aux.QuerierAddresses), []byte("null")) {
		s.QuerierAddresses = nil
		s.QuerierAddressDetails = nil
		s.querierAddressesBaseline = nil
		return nil
	}
	var rawAddresses []json.RawMessage
	if err := json.Unmarshal(aux.QuerierAddresses, &rawAddresses); err != nil {
		return fmt.Errorf("unmarshal querier_addresses: %w", err)
	}
	addresses := make([]string, 0, len(rawAddresses))
	details := make([]IgmpSnoopingQuerierAddress, 0, len(rawAddresses))
	sawObject := false
	for i, raw := range rawAddresses {
		var address string
		if err := json.Unmarshal(raw, &address); err == nil {
			addresses = append(addresses, address)
			details = append(details, IgmpSnoopingQuerierAddress{QuerierAddress: address})
			continue
		}
		sawObject = true
		var object struct {
			MAC            string        `json:"mac,omitempty"`
			NetworkID      string        `json:"network_id,omitempty"`
			QuerierAddress string        `json:"querier_address,omitempty"`
			QueryInterval  *types.Number `json:"query_interval,omitempty"`
		}
		if err := json.Unmarshal(raw, &object); err != nil {
			return fmt.Errorf("unmarshal querier_addresses[%d]: %w", i, err)
		}
		detail := IgmpSnoopingQuerierAddress{
			MAC:            object.MAC,
			NetworkID:      object.NetworkID,
			QuerierAddress: object.QuerierAddress,
		}
		if object.QueryInterval != nil {
			interval, err := object.QueryInterval.Int64()
			if err != nil {
				return fmt.Errorf("unmarshal querier_addresses[%d].query_interval: %w", i, err)
			}
			detail.QueryInterval = &interval
		}
		addresses = append(addresses, object.QuerierAddress)
		details = append(details, detail)
	}
	s.QuerierAddresses = addresses
	s.QuerierAddressDetails = nil
	s.querierAddressesBaseline = nil
	if sawObject {
		s.QuerierAddressDetails = details
		s.querierAddressesBaseline = append([]string(nil), addresses...)
	}
	return nil
}

// MarshalJSON writes the structured 10.4.x shape when detailed metadata is
// available, and otherwise preserves the released string-array shape.
func (s IgmpSnooping) MarshalJSON() ([]byte, error) {
	type alias IgmpSnooping
	addresses := s.marshalQuerierAddresses()
	return json.Marshal(struct {
		QuerierAddresses any `json:"querier_addresses,omitempty"`
		*alias
	}{
		QuerierAddresses: addresses,
		alias:            (*alias)(&s),
	})
}

func (s IgmpSnooping) marshalQuerierAddresses() any {
	if len(s.QuerierAddressDetails) == 0 {
		if len(s.QuerierAddresses) == 0 {
			return nil
		}
		return s.QuerierAddresses
	}
	// Details constructed directly through the new API have no decoded legacy
	// baseline and are authoritative.
	if len(s.querierAddressesBaseline) == 0 {
		return s.QuerierAddressDetails
	}
	if slices.Equal(s.QuerierAddresses, s.querierAddressesBaseline) {
		return s.QuerierAddressDetails
	}
	if len(s.QuerierAddresses) == 0 {
		return nil
	}

	synchronized := make([]IgmpSnoopingQuerierAddress, len(s.QuerierAddresses))
	used := make([]bool, len(s.QuerierAddressDetails))
	matched := make([]bool, len(s.QuerierAddresses))
	// Preserve metadata by identity first, including when callers reorder or
	// clone the released string slice.
	for i, address := range s.QuerierAddresses {
		for detailIndex, detail := range s.QuerierAddressDetails {
			if used[detailIndex] || detail.QuerierAddress != address {
				continue
			}
			synchronized[i] = detail
			used[detailIndex] = true
			matched[i] = true
			break
		}
	}
	// A same-length positional replacement is a field edit: clone the original
	// detail record and update only its address.
	if len(s.QuerierAddresses) == len(s.querierAddressesBaseline) {
		for i, address := range s.QuerierAddresses {
			if matched[i] || i >= len(s.QuerierAddressDetails) || used[i] {
				continue
			}
			synchronized[i] = s.QuerierAddressDetails[i]
			synchronized[i].QuerierAddress = address
			used[i] = true
			matched[i] = true
		}
	}
	for i, address := range s.QuerierAddresses {
		if !matched[i] {
			synchronized[i].QuerierAddress = address
		}
	}
	return synchronized
}
