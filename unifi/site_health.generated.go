// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"encoding/json"
	"fmt"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

type SiteHealth struct {
	Drops                 types.Number            `json:"drops,omitempty"`
	Gateways              []string                `json:"gateways,omitempty"`
	GwMac                 string                  `json:"gw_mac,omitempty"`
	GwName                string                  `json:"gw_name,omitempty"`
	GwSystemStats         SiteHealthGwSystemStats `json:"gw_system-stats"`
	GwVersion             string                  `json:"gw_version,omitempty"`
	LanIP                 string                  `json:"lan_ip,omitempty"`
	Latency               types.Number            `json:"latency,omitempty"`
	Nameservers           []string                `json:"nameservers,omitempty"`
	Netmask               string                  `json:"netmask,omitempty"`
	NumAdopted            types.Number            `json:"num_adopted,omitempty"`
	NumAp                 types.Number            `json:"num_ap,omitempty"`
	NumDisabled           types.Number            `json:"num_disabled,omitempty"`
	NumDisconnected       types.Number            `json:"num_disconnected,omitempty"`
	NumGuest              types.Number            `json:"num_guest,omitempty"`
	NumGw                 types.Number            `json:"num_gw,omitempty"`
	NumIot                types.Number            `json:"num_iot,omitempty"`
	NumPending            types.Number            `json:"num_pending,omitempty"`
	NumSta                types.Number            `json:"num_sta,omitempty"`
	NumSw                 types.Number            `json:"num_sw,omitempty"`
	NumUser               types.Number            `json:"num_user,omitempty"`
	RemoteUserEnabled     bool                    `json:"remote_user_enabled,omitempty"`
	RemoteUserNumActive   types.Number            `json:"remote_user_num_active,omitempty"`
	RemoteUserNumInactive types.Number            `json:"remote_user_num_inactive,omitempty"`
	RemoteUserRxBytes     types.Number            `json:"remote_user_rx_bytes,omitempty"`
	RemoteUserRxPackets   types.Number            `json:"remote_user_rx_packets,omitempty"`
	RemoteUserTxBytes     types.Number            `json:"remote_user_tx_bytes,omitempty"`
	RemoteUserTxPackets   types.Number            `json:"remote_user_tx_packets,omitempty"`
	RxBytesR              types.Number            `json:"rx_bytes-r,omitempty"`
	SiteToSiteEnabled     bool                    `json:"site_to_site_enabled,omitempty"`
	SiteToSiteNumActive   types.Number            `json:"site_to_site_num_active,omitempty"`
	SiteToSiteNumInactive types.Number            `json:"site_to_site_num_inactive,omitempty"`
	SiteToSiteRxBytes     types.Number            `json:"site_to_site_rx_bytes,omitempty"`
	SiteToSiteRxPackets   types.Number            `json:"site_to_site_rx_packets,omitempty"`
	SiteToSiteTxBytes     types.Number            `json:"site_to_site_tx_bytes,omitempty"`
	SiteToSiteTxPackets   types.Number            `json:"site_to_site_tx_packets,omitempty"`
	SpeedtestLastrun      types.Number            `json:"speedtest_lastrun,omitempty"`
	SpeedtestPing         types.Number            `json:"speedtest_ping,omitempty"`
	SpeedtestStatus       string                  `json:"speedtest_status,omitempty"`
	Status                string                  `json:"status"`
	Subsystem             string                  `json:"subsystem"`
	TxBytesR              types.Number            `json:"tx_bytes-r,omitempty"`
	Uptime                types.Number            `json:"uptime,omitempty"`
	WanIP                 string                  `json:"wan_ip,omitempty"`
	XputDown              types.Number            `json:"xput_down,omitempty"`
	XputUp                types.Number            `json:"xput_up,omitempty"`
}

func (dst *SiteHealth) UnmarshalJSON(b []byte) error {
	type Alias SiteHealth
	aux := &struct {
		Drops                 types.Number `json:"drops"`
		Latency               types.Number `json:"latency"`
		NumAdopted            types.Number `json:"num_adopted"`
		NumAp                 types.Number `json:"num_ap"`
		NumDisabled           types.Number `json:"num_disabled"`
		NumDisconnected       types.Number `json:"num_disconnected"`
		NumGuest              types.Number `json:"num_guest"`
		NumGw                 types.Number `json:"num_gw"`
		NumIot                types.Number `json:"num_iot"`
		NumPending            types.Number `json:"num_pending"`
		NumSta                types.Number `json:"num_sta"`
		NumSw                 types.Number `json:"num_sw"`
		NumUser               types.Number `json:"num_user"`
		RemoteUserNumActive   types.Number `json:"remote_user_num_active"`
		RemoteUserNumInactive types.Number `json:"remote_user_num_inactive"`
		RemoteUserRxBytes     types.Number `json:"remote_user_rx_bytes"`
		RemoteUserRxPackets   types.Number `json:"remote_user_rx_packets"`
		RemoteUserTxBytes     types.Number `json:"remote_user_tx_bytes"`
		RemoteUserTxPackets   types.Number `json:"remote_user_tx_packets"`
		RxBytesR              types.Number `json:"rx_bytes-r"`
		SiteToSiteNumActive   types.Number `json:"site_to_site_num_active"`
		SiteToSiteNumInactive types.Number `json:"site_to_site_num_inactive"`
		SiteToSiteRxBytes     types.Number `json:"site_to_site_rx_bytes"`
		SiteToSiteRxPackets   types.Number `json:"site_to_site_rx_packets"`
		SiteToSiteTxBytes     types.Number `json:"site_to_site_tx_bytes"`
		SiteToSiteTxPackets   types.Number `json:"site_to_site_tx_packets"`
		SpeedtestLastrun      types.Number `json:"speedtest_lastrun"`
		SpeedtestPing         types.Number `json:"speedtest_ping"`
		TxBytesR              types.Number `json:"tx_bytes-r"`
		Uptime                types.Number `json:"uptime"`
		XputDown              types.Number `json:"xput_down"`
		XputUp                types.Number `json:"xput_up"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	dst.Drops = identityTypesNumber(aux.Drops)
	dst.Latency = identityTypesNumber(aux.Latency)
	dst.NumAdopted = identityTypesNumber(aux.NumAdopted)
	dst.NumAp = identityTypesNumber(aux.NumAp)
	dst.NumDisabled = identityTypesNumber(aux.NumDisabled)
	dst.NumDisconnected = identityTypesNumber(aux.NumDisconnected)
	dst.NumGuest = identityTypesNumber(aux.NumGuest)
	dst.NumGw = identityTypesNumber(aux.NumGw)
	dst.NumIot = identityTypesNumber(aux.NumIot)
	dst.NumPending = identityTypesNumber(aux.NumPending)
	dst.NumSta = identityTypesNumber(aux.NumSta)
	dst.NumSw = identityTypesNumber(aux.NumSw)
	dst.NumUser = identityTypesNumber(aux.NumUser)
	dst.RemoteUserNumActive = identityTypesNumber(aux.RemoteUserNumActive)
	dst.RemoteUserNumInactive = identityTypesNumber(aux.RemoteUserNumInactive)
	dst.RemoteUserRxBytes = identityTypesNumber(aux.RemoteUserRxBytes)
	dst.RemoteUserRxPackets = identityTypesNumber(aux.RemoteUserRxPackets)
	dst.RemoteUserTxBytes = identityTypesNumber(aux.RemoteUserTxBytes)
	dst.RemoteUserTxPackets = identityTypesNumber(aux.RemoteUserTxPackets)
	dst.RxBytesR = identityTypesNumber(aux.RxBytesR)
	dst.SiteToSiteNumActive = identityTypesNumber(aux.SiteToSiteNumActive)
	dst.SiteToSiteNumInactive = identityTypesNumber(aux.SiteToSiteNumInactive)
	dst.SiteToSiteRxBytes = identityTypesNumber(aux.SiteToSiteRxBytes)
	dst.SiteToSiteRxPackets = identityTypesNumber(aux.SiteToSiteRxPackets)
	dst.SiteToSiteTxBytes = identityTypesNumber(aux.SiteToSiteTxBytes)
	dst.SiteToSiteTxPackets = identityTypesNumber(aux.SiteToSiteTxPackets)
	dst.SpeedtestLastrun = identityTypesNumber(aux.SpeedtestLastrun)
	dst.SpeedtestPing = identityTypesNumber(aux.SpeedtestPing)
	dst.TxBytesR = identityTypesNumber(aux.TxBytesR)
	dst.Uptime = identityTypesNumber(aux.Uptime)
	dst.XputDown = identityTypesNumber(aux.XputDown)
	dst.XputUp = identityTypesNumber(aux.XputUp)

	return nil
}

type SiteHealthGwSystemStats struct {
	CPU    types.Number `json:"cpu"`
	Mem    types.Number `json:"mem"`
	Uptime types.Number `json:"uptime"`
}

func (dst *SiteHealthGwSystemStats) UnmarshalJSON(b []byte) error {
	type Alias SiteHealthGwSystemStats
	aux := &struct {
		CPU    types.Number `json:"cpu"`
		Mem    types.Number `json:"mem"`
		Uptime types.Number `json:"uptime"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}
	dst.CPU = identityTypesNumber(aux.CPU)
	dst.Mem = identityTypesNumber(aux.Mem)
	dst.Uptime = identityTypesNumber(aux.Uptime)

	return nil
}
