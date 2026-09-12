package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/ubiquiti-community/go-unifi/unifi/types"
)

//go:generate go tool golang.org/x/tools/cmd/stringer -trimprefix DeviceState -type DeviceState
type DeviceState int64

const (
	DeviceStateUnknown          DeviceState = 0
	DeviceStateConnected        DeviceState = 1
	DeviceStatePending          DeviceState = 2
	DeviceStateFirmwareMismatch DeviceState = 3
	DeviceStateUpgrading        DeviceState = 4
	DeviceStateProvisioning     DeviceState = 5
	DeviceStateHeartbeatMissed  DeviceState = 6
	DeviceStateAdopting         DeviceState = 7
	DeviceStateDeleting         DeviceState = 8
	DeviceStateInformError      DeviceState = 9
	DeviceStateAdoptFailed      DeviceState = 10
	DeviceStateIsolated         DeviceState = 11
)

type DeviceLastConnection struct {
	MAC      string `json:"mac,omitempty"`
	LastSeen int64  `json:"last_seen,omitempty"`
}
type DevicePortTable struct {
	PortIdx             int64                `json:"port_idx,omitempty"`
	Media               string               `json:"media,omitempty"`
	PortPoe             bool                 `json:"port_poe,omitempty"`
	PoeCaps             int64                `json:"poe_caps,omitempty"`
	SpeedCaps           int64                `json:"speed_caps,omitempty"`
	LastConnection      DeviceLastConnection `json:"last_connection,omitempty"`
	OpMode              string               `json:"op_mode,omitempty"`
	Forward             string               `json:"forward,omitempty"`
	PoeMode             string               `json:"poe_mode,omitempty"`
	Anomalies           int64                `json:"anomalies,omitempty"`
	Autoneg             bool                 `json:"autoneg,omitempty"`
	Dot1XMode           string               `json:"dot1x_mode,omitempty"`
	Dot1XStatus         string               `json:"dot1x_status,omitempty"`
	Enable              bool                 `json:"enable,omitempty"`
	FlowctrlRx          bool                 `json:"flowctrl_rx,omitempty"`
	FlowctrlTx          bool                 `json:"flowctrl_tx,omitempty"`
	FullDuplex          bool                 `json:"full_duplex,omitempty"`
	IsUplink            bool                 `json:"is_uplink,omitempty"`
	Jumbo               bool                 `json:"jumbo,omitempty"`
	MacTableCount       int64                `json:"mac_table_count,omitempty"`
	PoeClass            string               `json:"poe_class,omitempty"`
	PoeCurrent          string               `json:"poe_current,omitempty"`
	PoeEnable           bool                 `json:"poe_enable,omitempty"`
	PoeGood             bool                 `json:"poe_good,omitempty"`
	PoePower            string               `json:"poe_power,omitempty"`
	PoeVoltage          string               `json:"poe_voltage,omitempty"`
	RxBroadcast         int64                `json:"rx_broadcast,omitempty"`
	RxBytes             int64                `json:"rx_bytes,omitempty"`
	RxDropped           int64                `json:"rx_dropped,omitempty"`
	RxErrors            int64                `json:"rx_errors,omitempty"`
	RxMulticast         int64                `json:"rx_multicast,omitempty"`
	RxPackets           int64                `json:"rx_packets,omitempty"`
	Satisfaction        int64                `json:"satisfaction,omitempty"`
	SatisfactionReason  int64                `json:"satisfaction_reason,omitempty"`
	Speed               int64                `json:"speed,omitempty"`
	StpPathcost         int64                `json:"stp_pathcost,omitempty"`
	StpState            string               `json:"stp_state,omitempty"`
	TxBroadcast         int64                `json:"tx_broadcast,omitempty"`
	TxBytes             int64                `json:"tx_bytes,omitempty"`
	TxDropped           int64                `json:"tx_dropped,omitempty"`
	TxErrors            int64                `json:"tx_errors,omitempty"`
	TxMulticast         int64                `json:"tx_multicast,omitempty"`
	TxPackets           int64                `json:"tx_packets,omitempty"`
	Up                  bool                 `json:"up,omitempty"`
	TxBytesR            float64              `json:"tx_bytes-r,omitempty"`
	RxBytesR            float64              `json:"rx_bytes-r,omitempty"`
	BytesR              float64              `json:"bytes-r,omitempty"`
	FlowControlEnabled  bool                 `json:"flow_control_enabled,omitempty"`
	NativeNetworkconfID string               `json:"native_networkconf_id,omitempty"`
	Name                string               `json:"name,omitempty"`
	SettingPreference   string               `json:"setting_preference,omitempty"`
	StormctrlBcastRate  int64                `json:"stormctrl_bcast_rate,omitempty"`
	StormctrlMcastRate  int64                `json:"stormctrl_mcast_rate,omitempty"`
	StormctrlUcastRate  int64                `json:"stormctrl_ucast_rate,omitempty"`
	TaggedVlanMgmt      string               `json:"tagged_vlan_mgmt,omitempty"`
	Masked              bool                 `json:"masked,omitempty"`
	AggregatedBy        types.NumberOrFalse  `json:"aggregated_by,omitempty"`
}

func (dst *DevicePortTable) UnmarshalJSON(b []byte) error {
	type Alias DevicePortTable
	aux := &struct {
		PortIdx            types.Number `json:"port_idx,omitempty"`
		PoeCaps            types.Number `json:"poe_caps,omitempty"`
		SpeedCaps          types.Number `json:"speed_caps,omitempty"`
		Anomalies          types.Number `json:"anomalies,omitempty"`
		MacTableCount      types.Number `json:"mac_table_count,omitempty"`
		RxBroadcast        types.Number `json:"rx_broadcast,omitempty"`
		RxBytes            types.Number `json:"rx_bytes,omitempty"`
		RxDropped          types.Number `json:"rx_dropped,omitempty"`
		RxErrors           types.Number `json:"rx_errors,omitempty"`
		RxMulticast        types.Number `json:"rx_multicast,omitempty"`
		RxPackets          types.Number `json:"rx_packets,omitempty"`
		Satisfaction       types.Number `json:"satisfaction,omitempty"`
		SatisfactionReason types.Number `json:"satisfaction_reason,omitempty"`
		Speed              types.Number `json:"speed,omitempty"`
		StpPathcost        types.Number `json:"stp_pathcost,omitempty"`
		TxBroadcast        types.Number `json:"tx_broadcast,omitempty"`
		TxBytes            types.Number `json:"tx_bytes,omitempty"`
		TxDropped          types.Number `json:"tx_dropped,omitempty"`
		TxErrors           types.Number `json:"tx_errors,omitempty"`
		TxMulticast        types.Number `json:"tx_multicast,omitempty"`
		TxPackets          types.Number `json:"tx_packets,omitempty"`
		StormctrlBcastRate types.Number `json:"stormctrl_bcast_rate,omitempty"`
		StormctrlMcastRate types.Number `json:"stormctrl_mcast_rate,omitempty"`
		StormctrlUcastRate types.Number `json:"stormctrl_ucast_rate,omitempty"`

		*Alias
	}{
		Alias: (*Alias)(dst),
	}

	err := json.Unmarshal(b, &aux)
	if err != nil {
		return fmt.Errorf("unable to unmarshal alias: %w", err)
	}

	// The shadow fields exist to tolerate the placeholder strings the
	// controller mixes into these numeric stats ("", "auto"): whatever
	// parses lands in the struct, and a placeholder leaves the zero value
	// instead of failing the whole device decode.
	for field, n := range map[*int64]types.Number{
		&dst.PortIdx:            aux.PortIdx,
		&dst.PoeCaps:            aux.PoeCaps,
		&dst.SpeedCaps:          aux.SpeedCaps,
		&dst.Anomalies:          aux.Anomalies,
		&dst.MacTableCount:      aux.MacTableCount,
		&dst.RxBroadcast:        aux.RxBroadcast,
		&dst.RxBytes:            aux.RxBytes,
		&dst.RxDropped:          aux.RxDropped,
		&dst.RxErrors:           aux.RxErrors,
		&dst.RxMulticast:        aux.RxMulticast,
		&dst.RxPackets:          aux.RxPackets,
		&dst.Satisfaction:       aux.Satisfaction,
		&dst.SatisfactionReason: aux.SatisfactionReason,
		&dst.Speed:              aux.Speed,
		&dst.StpPathcost:        aux.StpPathcost,
		&dst.TxBroadcast:        aux.TxBroadcast,
		&dst.TxBytes:            aux.TxBytes,
		&dst.TxDropped:          aux.TxDropped,
		&dst.TxErrors:           aux.TxErrors,
		&dst.TxMulticast:        aux.TxMulticast,
		&dst.TxPackets:          aux.TxPackets,
		&dst.StormctrlBcastRate: aux.StormctrlBcastRate,
		&dst.StormctrlMcastRate: aux.StormctrlMcastRate,
		&dst.StormctrlUcastRate: aux.StormctrlUcastRate,
	} {
		if v, err := n.Int64(); err == nil {
			*field = v
		}
	}

	return nil
}

func (c *ApiClient) GetDeviceByMAC(ctx context.Context, site, mac string) (*Device, error) {
	return c.getDevice(ctx, site, types.NormalizeMAC(mac))
}

// rereadDevice fetches a device by whichever identifier it carries, for the
// update paths that have to re-read after a controller answers a successful
// PUT with an empty data array.
//
// Device is the one resource whose read endpoint is keyed by MAC --
// stat/device/{mac}, not {id} -- so the generated "re-read by id" does not
// work here. But a masked write only needs the id (it addresses the object
// through the URL), so a caller doing the documented partial update,
// &Device{ID: id, Name: name}, legitimately holds no MAC. Passing that empty
// string through builds stat/device/ with a trailing slash, which is the
// list endpoint: it answers every device on the site, the caller's
// len(Data) != 1 check trips, and a write that succeeded reports NotFound.
// On a single-device site it answers one device and looks like it worked,
// which is worse.
//
// So take the MAC when there is one, and otherwise go the way GetDevice
// goes: list and filter on id.
func (c *ApiClient) rereadDevice(ctx context.Context, site string, d *Device) (*Device, error) {
	if d.MAC != "" {
		return c.getDevice(ctx, site, d.MAC)
	}
	return c.GetDevice(ctx, site, d.ID)
}

func (c *ApiClient) UpdateDevice(ctx context.Context, site string, d *Device) (*Device, error) {
	var respBody struct {
		Meta meta     `json:"meta"`
		Data []Device `json:"data"`
	}

	// Get the existing device to compare
	existing, err := c.getDevice(ctx, site, d.MAC)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing device: %w", err)
	}

	// Create a patch with only changed fields
	patch, err := getDeviceDiff(existing, d)
	if err != nil {
		return nil, fmt.Errorf("failed to create device diff: %w", err)
	}

	err = c.do(
		ctx,
		"PUT",
		fmt.Sprintf("api/s/%s/rest/device/%s", site, d.ID),
		patch,
		&respBody,
	)
	if err != nil {
		return nil, err
	}

	// Through the cloud connector proxy the PUT response may have an empty data
	// array even on success. Re-read by MAC to get the updated device.
	if len(respBody.Data) != 1 {
		if d.MAC != "" {
			return c.getDevice(ctx, site, d.MAC)
		}
		return nil, &NotFoundError{}
	}

	res := respBody.Data[0]

	return &res, nil
}

// encodedFields renders v through its own encoder and hands back the wire
// fields as a generic map, so two objects can be compared as the controller
// would see them.
func encodedFields(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}
	return m, nil
}

// getDeviceDiff compares two Device objects and returns a map containing only changed fields.
//
// port_overrides needs handling the field-by-field diff cannot do. The diff
// compares the two objects as the encoder renders them, and Device's encoder sends nil
// port_overrides as [] -- deliberately, because the controller rejects null
// there and an empty list is the only way to clear the field. That is right
// for a full write and wrong here: a caller updating one unrelated field
// holds a Device it never populated port_overrides on, so the diff sees []
// against the stored overrides, calls it a change, and the patch wipes every
// port override on the device.
//
// nil means "not supplied" and comes out of the patch. An explicitly empty
// slice still means "clear these", and still goes through -- which is why
// this tests the field rather than the rendered [].
func getDeviceDiff(original, target *Device) (map[string]any, error) {
	origMap, err := encodedFields(original)
	if err != nil {
		return nil, err
	}
	targetMap, err := encodedFields(target)
	if err != nil {
		return nil, err
	}

	// Read-only fields never belong in a patch.
	skip := map[string]bool{"_id": true, "site_id": true, "adopted": true, "state": true}

	patch := make(map[string]any)
	for key, targetValue := range targetMap {
		if skip[key] {
			continue
		}
		origValue, exists := origMap[key]
		if !exists || !reflect.DeepEqual(origValue, targetValue) {
			patch[key] = targetValue
		}
	}

	if target.PortOverrides == nil {
		delete(patch, "port_overrides")
	}

	return patch, nil
}

func (c *ApiClient) GetDevice(ctx context.Context, site, id string) (*Device, error) {
	devices, err := c.ListDevice(ctx, site)
	if err != nil {
		return nil, err
	}

	for _, d := range devices {
		if d.ID == id {
			return &d, nil
		}
	}

	return nil, &NotFoundError{}
}

func (c *ApiClient) AdoptDevice(ctx context.Context, site, mac string) error {
	reqBody := struct {
		Cmd string `json:"cmd"`
		MAC string `json:"mac"`
	}{
		Cmd: "adopt",
		MAC: types.NormalizeMAC(mac),
	}

	var respBody struct {
		Meta meta `json:"meta"`
	}

	err := c.siteCommand(ctx, site, "devmgr", reqBody, &respBody)
	if err != nil {
		return err
	}

	return nil
}

func (c *ApiClient) ForgetDevice(ctx context.Context, site, mac string) error {
	reqBody := struct {
		Cmd  string   `json:"cmd"`
		MACs []string `json:"macs"`
	}{
		Cmd:  "delete-device",
		MACs: []string{types.NormalizeMAC(mac)},
	}

	var respBody struct {
		Meta meta     `json:"meta"`
		Data []Device `json:"data"`
	}

	err := c.siteCommand(ctx, site, "sitemgr", reqBody, &respBody)
	if err != nil {
		return err
	}

	return nil
}
