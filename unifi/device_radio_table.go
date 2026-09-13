package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
)

// radioTableMember is the wire name of the array this writes, and
// radioTableKey the member that identifies an entry within it.
const (
	radioTableMember = "radio_table"
	radioTableKey    = "name"
)

// UpdateDeviceRadioTable writes only the named members of the declared
// radios, and leaves every other member, every other radio, and the rest of
// the device alone.
//
// This is the masked write for the device's other array of objects, the
// sibling of UpdateDevicePortOverrides. The ordinary masked update cannot
// express it: naming radio_table in a mask sends the whole array as this
// client models it, and since every generated member is omitempty, that
// silently drops the members sitting at their zero value and the ones this
// client has no field for at all -- nss, max_txpower, radio_caps and the
// rest of what the AP reports about its own hardware.
//
// Where it parts company with the port-override writer is the merge, and
// that is measured, not assumed. On 10.6.101, against herded U7PRO and U6M
// access points, radio_table MERGES on both levels: a member an entry omits
// keeps its stored value, and a radio the array omits keeps its whole stored
// entry. port_overrides, in the same document and the same PUT, replaces on
// both levels. So this sends the declared entries alone, with only the
// members the caller named, and does not read the stored array back to
// resend it -- there is nothing for a resend to protect.
// TestIntegrationUpdateDeviceRadioTable and the artifact's Empty section
// pin that; a controller that stops merging fails them both.
//
// The key is name, and it is the controller's own: an entry carrying no name
// is refused (api.err.MissingValue), a name the device does not report is
// refused (InvalidValue on name), and a radio member that disagrees with the
// name is refused too (InvalidValue on radio). A mis-keyed write therefore
// cannot land quietly on the wrong radio, and unlike a port override, a
// radio the device does not have cannot be added -- there is no hardware for
// it, and the controller says so.
//
// The returned device is re-read from the controller, by the same route
// GetDevice takes, so a caller does not have to know that devices are read
// from stat/device and written to rest/device.
func (c *ApiClient) UpdateDeviceRadioTable(
	ctx context.Context,
	site string,
	d *Device,
	declared []DeviceRadioTable,
	fields ...string,
) (*Device, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf(
			"a masked write needs at least one member; to replace the radio table " +
				"wholesale, set Device.RadioTable and use UpdateDevice")
	}
	if d == nil || d.ID == "" {
		return nil, fmt.Errorf("a radio-table write needs the device's id to address it")
	}
	if len(declared) == 0 {
		return nil, fmt.Errorf(
			"no radios were declared.\n\n" +
				"This writes the radios it is given and leaves the rest alone, so an empty " +
				"list would be a no-op rather than a way to clear them")
	}

	// The key addresses the entry, so it travels whether or not the caller
	// named it -- the same reason a masked object write carries _id.
	masked := make([]json.RawMessage, 0, len(declared))
	for i := range declared {
		if declared[i].Name == "" {
			return nil, fmt.Errorf(
				"radio %d carries no name.\n\n"+
					"Radios are addressed by the name the device reports for them (wifi-ng, "+
					"wifi-na, wifi-6e and their kin), so one without it cannot be matched to "+
					"the radio it means to change", i)
		}
		body, err := maskedBody(&declared[i], append(slices.Clone(fields), radioTableKey))
		if err != nil {
			return nil, fmt.Errorf("radio %s: %w", declared[i].Name, err)
		}
		masked = append(masked, body)
	}

	// json.RawMessage, not []byte: the request body is marshalled again on
	// the way out, and marshalling a []byte yields a base64 string rather
	// than the object it holds.
	encoded, err := json.Marshal(map[string]any{
		"_id":            d.ID,
		radioTableMember: masked,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to encode the radio table: %w", err)
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
