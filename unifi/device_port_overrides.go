package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
)

// portOverridesMember is the wire name of the array this writes, and
// portOverridesKey the field that identifies an entry within it.
const (
	portOverridesMember = "port_overrides"
	portOverridesKey    = "port_idx"
)

// UpdateDevicePortOverrides writes only the named members of the declared
// port overrides, and leaves every other member, every other port, and the
// rest of the device alone.
//
// This is the masked write for a member that is an array of objects. The
// ordinary masked update cannot express it: naming port_overrides in a mask
// sends the whole array as this client models it, and measured on 10.6.101
// the controller replaces at both levels -- an entry the payload omits is
// dropped, and a member an entry omits is dropped from that entry. Since
// every generated member is omitempty, re-encoding the array through the Go
// struct silently discards both the members this client does not model and
// the ones sitting at their zero value.
//
// So the merge happens on the stored JSON. Each declared entry contributes
// only the members fields names, matched to the stored entry with the same
// port_idx; a declared port that has no stored entry is added, which is how
// a caller puts an override on a port that had none.
//
// The returned device is re-read from the controller, by the same route
// GetDevice takes, so a caller does not have to know that devices are read
// from stat/device and written to rest/device.
func (c *ApiClient) UpdateDevicePortOverrides(
	ctx context.Context,
	site string,
	d *Device,
	declared []DevicePortOverrides,
	fields ...string,
) (*Device, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf(
			"a masked write needs at least one member; to replace the port overrides " +
				"wholesale, set Device.PortOverrides and use UpdateDevice")
	}
	if d == nil || d.ID == "" {
		return nil, fmt.Errorf("a port-override write needs the device's id to address it")
	}
	if len(declared) == 0 {
		return nil, fmt.Errorf(
			"no port overrides were declared.\n\n" +
				"This writes the ports it is given and leaves the rest alone, so an empty " +
				"list would be a no-op rather than a way to clear them")
	}

	// The key addresses the entry, so it travels whether or not the caller
	// named it -- the same reason a masked object write carries _id.
	masked := make([]json.RawMessage, 0, len(declared))
	for i := range declared {
		body, err := maskedBody(&declared[i], append(slices.Clone(fields), portOverridesKey))
		if err != nil {
			return nil, fmt.Errorf("port override %d: %w", i, err)
		}
		masked = append(masked, body)
	}

	stored, err := c.rawDevicePortOverrides(ctx, site, d)
	if err != nil {
		return nil, err
	}
	merged, err := overlayKeyedEntries(stored, portOverridesKey, masked)
	if err != nil {
		return nil, err
	}

	// json.RawMessage, not []byte: the request body is marshalled again on
	// the way out, and marshalling a []byte yields a base64 string rather
	// than the object it holds.
	encoded, err := json.Marshal(map[string]any{
		"_id":               d.ID,
		portOverridesMember: merged,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to encode the merged port overrides: %w", err)
	}
	body := json.RawMessage(encoded)

	var respBody struct {
		Meta meta              `json:"meta"`
		Data []json.RawMessage `json:"data"`
	}
	if err := c.do(
		ctx,
		http.MethodPut,
		fmt.Sprintf("api/s/%s/rest/device/%s", site, d.ID),
		body,
		&respBody,
	); err != nil {
		return nil, err
	}

	return c.rereadDevice(ctx, site, d)
}

// rawDevicePortOverrides reads the device's port overrides exactly as the
// controller stores them, including members this client does not model.
//
// It reads the way GetDevice does rather than by id, because the per-device
// read endpoint is keyed by MAC -- see rereadDevice, which documents what
// passing an id there does instead.
func (c *ApiClient) rawDevicePortOverrides(ctx context.Context, site string, d *Device) ([]json.RawMessage, error) {
	var respBody struct {
		Meta meta              `json:"meta"`
		Data []json.RawMessage `json:"data"`
	}

	path := fmt.Sprintf("api/s/%s/stat/device", site)
	if d.MAC != "" {
		path = fmt.Sprintf("api/s/%s/stat/device/%s", site, d.MAC)
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &respBody); err != nil {
		return nil, err
	}

	for _, raw := range respBody.Data {
		var device struct {
			ID            string            `json:"_id"`
			PortOverrides []json.RawMessage `json:"port_overrides"`
		}
		if err := json.Unmarshal(raw, &device); err != nil {
			return nil, fmt.Errorf("unable to read a stored device: %w", err)
		}
		if device.ID == d.ID {
			return device.PortOverrides, nil
		}
	}

	return nil, &NotFoundError{Type: "Device", Attr: "_id", Value: d.ID}
}
