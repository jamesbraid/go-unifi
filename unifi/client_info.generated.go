// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"encoding/json"
	"fmt"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

type ClientInfo struct {
	Anomalies                           *int64                     `json:"anomalies,omitempty"`
	ApMac                               string                     `json:"ap_mac,omitempty"`
	AssocTime                           *int64                     `json:"assoc_time,omitempty"`
	Authorized                          bool                       `json:"authorized,omitempty"`
	Blocked                             bool                       `json:"blocked,omitempty"`
	Bssid                               string                     `json:"bssid,omitempty"`
	Ccq                                 *int64                     `json:"ccq,omitempty"`
	Channel                             *int64                     `json:"channel,omitempty"`
	ChannelWidth                        string                     `json:"channel_width,omitempty"`
	DetailedStates                      ClientInfoDetailedStates   `json:"detailed_states"`
	DhcpendTime                         *int64                     `json:"dhcpend_time,omitempty"`
	DisplayName                         string                     `json:"display_name,omitempty"`
	Essid                               string                     `json:"essid,omitempty"`
	Fingerprint                         ClientInfoFingerprint      `json:"fingerprint"`
	FirstSeen                           *int64                     `json:"first_seen,omitempty"`
	FixedApEnabled                      bool                       `json:"fixed_ap_enabled,omitempty"`
	FixedIP                             string                     `json:"fixed_ip,omitempty"`
	GwMac                               string                     `json:"gw_mac,omitempty"`
	Hostname                            string                     `json:"hostname,omitempty"`
	IP                                  string                     `json:"ip,omitempty"`
	Id                                  string                     `json:"id,omitempty"`
	Idletime                            *int64                     `json:"idletime,omitempty"`
	Ipv4LeaseExpirationTimestampSeconds *int64                     `json:"ipv4_lease_expiration_timestamp_seconds,omitempty"`
	Ipv6Address                         []string                   `json:"ipv6_address,omitempty"`
	IsAllowedInVisualProgramming        bool                       `json:"is_allowed_in_visual_programming,omitempty"`
	IsGuest                             bool                       `json:"is_guest,omitempty"`
	IsMlo                               bool                       `json:"is_mlo,omitempty"`
	IsWired                             bool                       `json:"is_wired,omitempty"`
	LastConnectionNetworkId             string                     `json:"last_connection_network_id,omitempty"`
	LastConnectionNetworkName           string                     `json:"last_connection_network_name,omitempty"`
	LastIP                              string                     `json:"last_ip,omitempty"`
	LastIpv6                            []string                   `json:"last_ipv6,omitempty"`
	LastRadio                           string                     `json:"last_radio,omitempty"`
	LastSeen                            *int64                     `json:"last_seen,omitempty"`
	LastUplinkMac                       string                     `json:"last_uplink_mac,omitempty"`
	LastUplinkName                      string                     `json:"last_uplink_name,omitempty"`
	LastUplinkRemotePort                *int64                     `json:"last_uplink_remote_port,omitempty"`
	LatestAssocTime                     *int64                     `json:"latest_assoc_time,omitempty"`
	LocalDNSRecord                      string                     `json:"local_dns_record,omitempty"`
	LocalDNSRecordEnabled               bool                       `json:"local_dns_record_enabled,omitempty"`
	Mac                                 string                     `json:"mac,omitempty"`
	Mimo                                string                     `json:"mimo,omitempty"`
	ModelName                           string                     `json:"model_name,omitempty"`
	Name                                string                     `json:"name,omitempty"`
	NetworkId                           string                     `json:"network_id,omitempty"`
	NetworkMembersGroupIDs              []string                   `json:"network_members_group_ids,omitempty"`
	NetworkName                         string                     `json:"network_name,omitempty"`
	Noise                               *int64                     `json:"noise,omitempty"`
	Noted                               bool                       `json:"noted,omitempty"`
	Oui                                 string                     `json:"oui,omitempty"`
	PowersaveEnabled                    bool                       `json:"powersave_enabled,omitempty"`
	Radio                               string                     `json:"radio,omitempty"`
	RadioName                           string                     `json:"radio_name,omitempty"`
	RadioProto                          string                     `json:"radio_proto,omitempty"`
	RateImbalance                       *int64                     `json:"rate_imbalance,omitempty"`
	Rssi                                *int64                     `json:"rssi,omitempty"`
	RxBytes                             *int64                     `json:"rx_bytes,omitempty"`
	RxBytesR                            *int64                     `json:"rx_bytes-r,omitempty"`
	RxPackets                           *int64                     `json:"rx_packets,omitempty"`
	RxRate                              *int64                     `json:"rx_rate,omitempty"`
	Signal                              *int64                     `json:"signal,omitempty"`
	SiteId                              string                     `json:"site_id,omitempty"`
	Status                              string                     `json:"status,omitempty"`
	SwPort                              *int64                     `json:"sw_port,omitempty"`
	Tags                                []string                   `json:"tags,omitempty"`
	TxBytes                             *int64                     `json:"tx_bytes,omitempty"`
	TxBytesR                            *int64                     `json:"tx_bytes-r,omitempty"`
	TxMcsIndex                          *int64                     `json:"tx_mcs_index,omitempty"`
	TxPackets                           *int64                     `json:"tx_packets,omitempty"`
	TxRate                              *int64                     `json:"tx_rate,omitempty"`
	Type                                string                     `json:"type,omitempty"`
	UnifiDevice                         bool                       `json:"unifi_device,omitempty"`
	UnifiDeviceInfo                     *ClientInfoUnifiDeviceInfo `json:"unifi_device_info,omitempty"`
	UplinkMac                           string                     `json:"uplink_mac,omitempty"`
	Uptime                              *int64                     `json:"uptime,omitempty"`
	UseFixedip                          bool                       `json:"use_fixedip,omitempty"`
	UserId                              string                     `json:"user_id,omitempty"`
	UsergroupId                         string                     `json:"usergroup_id,omitempty"`
	VirtualNetworkOverrideEnabled       bool                       `json:"virtual_network_override_enabled,omitempty"`
	VirtualNetworkOverrideId            string                     `json:"virtual_network_override_id,omitempty"`
	WifiExperienceAverage               *int64                     `json:"wifi_experience_average,omitempty"`
	WifiExperienceScore                 *int64                     `json:"wifi_experience_score,omitempty"`
	WifiTxAttempts                      *int64                     `json:"wifi_tx_attempts,omitempty"`
	WifiTxRetriesPercentage             float64                    `json:"wifi_tx_retries_percentage,omitempty"`
	WiredRateMbps                       *int64                     `json:"wired_rate_mbps,omitempty"`
	WlanconfId                          string                     `json:"wlanconf_id,omitempty"`
}

func (dst *ClientInfo) UnmarshalJSON(b []byte) error {
	type Alias ClientInfo
	aux := &struct {
		Anomalies                           *types.Number `json:"anomalies"`
		AssocTime                           *types.Number `json:"assoc_time"`
		Ccq                                 *types.Number `json:"ccq"`
		Channel                             *types.Number `json:"channel"`
		DhcpendTime                         *types.Number `json:"dhcpend_time"`
		FirstSeen                           *types.Number `json:"first_seen"`
		Idletime                            *types.Number `json:"idletime"`
		Ipv4LeaseExpirationTimestampSeconds *types.Number `json:"ipv4_lease_expiration_timestamp_seconds"`
		LastSeen                            *types.Number `json:"last_seen"`
		LastUplinkRemotePort                *types.Number `json:"last_uplink_remote_port"`
		LatestAssocTime                     *types.Number `json:"latest_assoc_time"`
		Noise                               *types.Number `json:"noise"`
		RateImbalance                       *types.Number `json:"rate_imbalance"`
		Rssi                                *types.Number `json:"rssi"`
		RxBytes                             *types.Number `json:"rx_bytes"`
		RxBytesR                            *types.Number `json:"rx_bytes-r"`
		RxPackets                           *types.Number `json:"rx_packets"`
		RxRate                              *types.Number `json:"rx_rate"`
		Signal                              *types.Number `json:"signal"`
		SwPort                              *types.Number `json:"sw_port"`
		TxBytes                             *types.Number `json:"tx_bytes"`
		TxBytesR                            *types.Number `json:"tx_bytes-r"`
		TxMcsIndex                          *types.Number `json:"tx_mcs_index"`
		TxPackets                           *types.Number `json:"tx_packets"`
		TxRate                              *types.Number `json:"tx_rate"`
		Uptime                              *types.Number `json:"uptime"`
		WifiExperienceAverage               *types.Number `json:"wifi_experience_average"`
		WifiExperienceScore                 *types.Number `json:"wifi_experience_score"`
		WifiTxAttempts                      *types.Number `json:"wifi_tx_attempts"`
		WiredRateMbps                       *types.Number `json:"wired_rate_mbps"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.Anomalies != nil {
		if val, err := aux.Anomalies.Int64(); err == nil {
			dst.Anomalies = &val
		} else if string(*aux.Anomalies) == "" {
			var zero int64
			dst.Anomalies = &zero
		}
	}
	if aux.AssocTime != nil {
		if val, err := aux.AssocTime.Int64(); err == nil {
			dst.AssocTime = &val
		} else if string(*aux.AssocTime) == "" {
			var zero int64
			dst.AssocTime = &zero
		}
	}
	if aux.Ccq != nil {
		if val, err := aux.Ccq.Int64(); err == nil {
			dst.Ccq = &val
		} else if string(*aux.Ccq) == "" {
			var zero int64
			dst.Ccq = &zero
		}
	}
	if aux.Channel != nil {
		if val, err := aux.Channel.Int64(); err == nil {
			dst.Channel = &val
		} else if string(*aux.Channel) == "" {
			var zero int64
			dst.Channel = &zero
		}
	}
	if aux.DhcpendTime != nil {
		if val, err := aux.DhcpendTime.Int64(); err == nil {
			dst.DhcpendTime = &val
		} else if string(*aux.DhcpendTime) == "" {
			var zero int64
			dst.DhcpendTime = &zero
		}
	}
	if aux.FirstSeen != nil {
		if val, err := aux.FirstSeen.Int64(); err == nil {
			dst.FirstSeen = &val
		} else if string(*aux.FirstSeen) == "" {
			var zero int64
			dst.FirstSeen = &zero
		}
	}
	if aux.Idletime != nil {
		if val, err := aux.Idletime.Int64(); err == nil {
			dst.Idletime = &val
		} else if string(*aux.Idletime) == "" {
			var zero int64
			dst.Idletime = &zero
		}
	}
	if aux.Ipv4LeaseExpirationTimestampSeconds != nil {
		if val, err := aux.Ipv4LeaseExpirationTimestampSeconds.Int64(); err == nil {
			dst.Ipv4LeaseExpirationTimestampSeconds = &val
		} else if string(*aux.Ipv4LeaseExpirationTimestampSeconds) == "" {
			var zero int64
			dst.Ipv4LeaseExpirationTimestampSeconds = &zero
		}
	}
	if aux.LastSeen != nil {
		if val, err := aux.LastSeen.Int64(); err == nil {
			dst.LastSeen = &val
		} else if string(*aux.LastSeen) == "" {
			var zero int64
			dst.LastSeen = &zero
		}
	}
	if aux.LastUplinkRemotePort != nil {
		if val, err := aux.LastUplinkRemotePort.Int64(); err == nil {
			dst.LastUplinkRemotePort = &val
		} else if string(*aux.LastUplinkRemotePort) == "" {
			var zero int64
			dst.LastUplinkRemotePort = &zero
		}
	}
	if aux.LatestAssocTime != nil {
		if val, err := aux.LatestAssocTime.Int64(); err == nil {
			dst.LatestAssocTime = &val
		} else if string(*aux.LatestAssocTime) == "" {
			var zero int64
			dst.LatestAssocTime = &zero
		}
	}
	if aux.Noise != nil {
		if val, err := aux.Noise.Int64(); err == nil {
			dst.Noise = &val
		} else if string(*aux.Noise) == "" {
			var zero int64
			dst.Noise = &zero
		}
	}
	if aux.RateImbalance != nil {
		if val, err := aux.RateImbalance.Int64(); err == nil {
			dst.RateImbalance = &val
		} else if string(*aux.RateImbalance) == "" {
			var zero int64
			dst.RateImbalance = &zero
		}
	}
	if aux.Rssi != nil {
		if val, err := aux.Rssi.Int64(); err == nil {
			dst.Rssi = &val
		} else if string(*aux.Rssi) == "" {
			var zero int64
			dst.Rssi = &zero
		}
	}
	if aux.RxBytes != nil {
		if val, err := aux.RxBytes.Int64(); err == nil {
			dst.RxBytes = &val
		} else if string(*aux.RxBytes) == "" {
			var zero int64
			dst.RxBytes = &zero
		}
	}
	if aux.RxBytesR != nil {
		if val, err := aux.RxBytesR.Int64(); err == nil {
			dst.RxBytesR = &val
		} else if string(*aux.RxBytesR) == "" {
			var zero int64
			dst.RxBytesR = &zero
		}
	}
	if aux.RxPackets != nil {
		if val, err := aux.RxPackets.Int64(); err == nil {
			dst.RxPackets = &val
		} else if string(*aux.RxPackets) == "" {
			var zero int64
			dst.RxPackets = &zero
		}
	}
	if aux.RxRate != nil {
		if val, err := aux.RxRate.Int64(); err == nil {
			dst.RxRate = &val
		} else if string(*aux.RxRate) == "" {
			var zero int64
			dst.RxRate = &zero
		}
	}
	if aux.Signal != nil {
		if val, err := aux.Signal.Int64(); err == nil {
			dst.Signal = &val
		} else if string(*aux.Signal) == "" {
			var zero int64
			dst.Signal = &zero
		}
	}
	if aux.SwPort != nil {
		if val, err := aux.SwPort.Int64(); err == nil {
			dst.SwPort = &val
		} else if string(*aux.SwPort) == "" {
			var zero int64
			dst.SwPort = &zero
		}
	}
	if aux.TxBytes != nil {
		if val, err := aux.TxBytes.Int64(); err == nil {
			dst.TxBytes = &val
		} else if string(*aux.TxBytes) == "" {
			var zero int64
			dst.TxBytes = &zero
		}
	}
	if aux.TxBytesR != nil {
		if val, err := aux.TxBytesR.Int64(); err == nil {
			dst.TxBytesR = &val
		} else if string(*aux.TxBytesR) == "" {
			var zero int64
			dst.TxBytesR = &zero
		}
	}
	if aux.TxMcsIndex != nil {
		if val, err := aux.TxMcsIndex.Int64(); err == nil {
			dst.TxMcsIndex = &val
		} else if string(*aux.TxMcsIndex) == "" {
			var zero int64
			dst.TxMcsIndex = &zero
		}
	}
	if aux.TxPackets != nil {
		if val, err := aux.TxPackets.Int64(); err == nil {
			dst.TxPackets = &val
		} else if string(*aux.TxPackets) == "" {
			var zero int64
			dst.TxPackets = &zero
		}
	}
	if aux.TxRate != nil {
		if val, err := aux.TxRate.Int64(); err == nil {
			dst.TxRate = &val
		} else if string(*aux.TxRate) == "" {
			var zero int64
			dst.TxRate = &zero
		}
	}
	if aux.Uptime != nil {
		if val, err := aux.Uptime.Int64(); err == nil {
			dst.Uptime = &val
		} else if string(*aux.Uptime) == "" {
			var zero int64
			dst.Uptime = &zero
		}
	}
	if aux.WifiExperienceAverage != nil {
		if val, err := aux.WifiExperienceAverage.Int64(); err == nil {
			dst.WifiExperienceAverage = &val
		} else if string(*aux.WifiExperienceAverage) == "" {
			var zero int64
			dst.WifiExperienceAverage = &zero
		}
	}
	if aux.WifiExperienceScore != nil {
		if val, err := aux.WifiExperienceScore.Int64(); err == nil {
			dst.WifiExperienceScore = &val
		} else if string(*aux.WifiExperienceScore) == "" {
			var zero int64
			dst.WifiExperienceScore = &zero
		}
	}
	if aux.WifiTxAttempts != nil {
		if val, err := aux.WifiTxAttempts.Int64(); err == nil {
			dst.WifiTxAttempts = &val
		} else if string(*aux.WifiTxAttempts) == "" {
			var zero int64
			dst.WifiTxAttempts = &zero
		}
	}
	if aux.WiredRateMbps != nil {
		if val, err := aux.WiredRateMbps.Int64(); err == nil {
			dst.WiredRateMbps = &val
		} else if string(*aux.WiredRateMbps) == "" {
			var zero int64
			dst.WiredRateMbps = &zero
		}
	}

	return nil
}

type ClientInfoDetailedStates struct {
	UplinkNearPowerLimit bool `json:"uplink_near_power_limit,omitempty"`
}

func (dst *ClientInfoDetailedStates) UnmarshalJSON(b []byte) error {
	type Alias ClientInfoDetailedStates
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

type ClientInfoFingerprint struct {
	ComputedDevId  *int64 `json:"computed_dev_id,omitempty"`
	ComputedEngine *int64 `json:"computed_engine,omitempty"`
	Confidence     *int64 `json:"confidence,omitempty"`
	DevCat         *int64 `json:"dev_cat,omitempty"`
	DevFamily      *int64 `json:"dev_family,omitempty"`
	DevId          *int64 `json:"dev_id,omitempty"`
	DevIdOverride  *int64 `json:"dev_id_override,omitempty"`
	DevVendor      *int64 `json:"dev_vendor,omitempty"`
	HasOverride    bool   `json:"has_override,omitempty"`
	OsName         *int64 `json:"os_name,omitempty"`
}

func (dst *ClientInfoFingerprint) UnmarshalJSON(b []byte) error {
	type Alias ClientInfoFingerprint
	aux := &struct {
		ComputedDevId  *types.Number `json:"computed_dev_id"`
		ComputedEngine *types.Number `json:"computed_engine"`
		Confidence     *types.Number `json:"confidence"`
		DevCat         *types.Number `json:"dev_cat"`
		DevFamily      *types.Number `json:"dev_family"`
		DevId          *types.Number `json:"dev_id"`
		DevIdOverride  *types.Number `json:"dev_id_override"`
		DevVendor      *types.Number `json:"dev_vendor"`
		OsName         *types.Number `json:"os_name"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	if aux.ComputedDevId != nil {
		if val, err := aux.ComputedDevId.Int64(); err == nil {
			dst.ComputedDevId = &val
		} else if string(*aux.ComputedDevId) == "" {
			var zero int64
			dst.ComputedDevId = &zero
		}
	}
	if aux.ComputedEngine != nil {
		if val, err := aux.ComputedEngine.Int64(); err == nil {
			dst.ComputedEngine = &val
		} else if string(*aux.ComputedEngine) == "" {
			var zero int64
			dst.ComputedEngine = &zero
		}
	}
	if aux.Confidence != nil {
		if val, err := aux.Confidence.Int64(); err == nil {
			dst.Confidence = &val
		} else if string(*aux.Confidence) == "" {
			var zero int64
			dst.Confidence = &zero
		}
	}
	if aux.DevCat != nil {
		if val, err := aux.DevCat.Int64(); err == nil {
			dst.DevCat = &val
		} else if string(*aux.DevCat) == "" {
			var zero int64
			dst.DevCat = &zero
		}
	}
	if aux.DevFamily != nil {
		if val, err := aux.DevFamily.Int64(); err == nil {
			dst.DevFamily = &val
		} else if string(*aux.DevFamily) == "" {
			var zero int64
			dst.DevFamily = &zero
		}
	}
	if aux.DevId != nil {
		if val, err := aux.DevId.Int64(); err == nil {
			dst.DevId = &val
		} else if string(*aux.DevId) == "" {
			var zero int64
			dst.DevId = &zero
		}
	}
	if aux.DevIdOverride != nil {
		if val, err := aux.DevIdOverride.Int64(); err == nil {
			dst.DevIdOverride = &val
		} else if string(*aux.DevIdOverride) == "" {
			var zero int64
			dst.DevIdOverride = &zero
		}
	}
	if aux.DevVendor != nil {
		if val, err := aux.DevVendor.Int64(); err == nil {
			dst.DevVendor = &val
		} else if string(*aux.DevVendor) == "" {
			var zero int64
			dst.DevVendor = &zero
		}
	}
	if aux.OsName != nil {
		if val, err := aux.OsName.Int64(); err == nil {
			dst.OsName = &val
		} else if string(*aux.OsName) == "" {
			var zero int64
			dst.OsName = &zero
		}
	}

	return nil
}

type ClientInfoUnifiDeviceInfo struct {
	IconFilename      string     `json:"icon_filename,omitempty"`
	IconResolutions   [][]*int64 `json:"icon_resolutions,omitempty"`
	ViewInApplication bool       `json:"view_in_application,omitempty"`
}

func (dst *ClientInfoUnifiDeviceInfo) UnmarshalJSON(b []byte) error {
	type Alias ClientInfoUnifiDeviceInfo
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
