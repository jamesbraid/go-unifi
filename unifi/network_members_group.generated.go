// Code generated from the controller schema in the capture lock
// DO NOT EDIT.

package unifi

import (
	"context"
	"encoding/json"
	"fmt"
)

type NetworkMembersGroup struct {
	ID string `json:"id,omitempty"`

	Members []string `json:"members"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
}

// MarshalJSON fixes up the write shape of this type.
//
// Read-only fields are dropped: the controller reports them and rejects them
// on a write, so without this an update after a read fails on the
// server-assigned fields the read filled in.
//
// Slices marked nil-as-empty are sent as [] rather than null. They serialize
// unconditionally by design -- an empty list has to reach the wire to clear
// the value -- but a caller that never touched the field holds nil, and the
// controller rejects null where it expects an array.
func (src NetworkMembersGroup) MarshalJSON() ([]byte, error) {
	type Alias NetworkMembersGroup
	return json.Marshal(&struct {
		Members []string `json:"members"`
		*Alias
	}{
		Members: emptyIfNil(src.Members),
		Alias:   (*Alias)(&src),
	})
}

func (dst *NetworkMembersGroup) UnmarshalJSON(b []byte) error {
	type Alias NetworkMembersGroup
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

// UpdateNetworkMembersGroupFields writes only the named wire fields and leaves
// the rest of the stored object untouched. Use it when the caller models some
// of the object rather than all of it: an unnamed field keeps its stored
// value, where a full write would assert this struct's zero value for it.
// See maskedBody for how the named fields become the request body.
func (c *ApiClient) UpdateNetworkMembersGroupFields(ctx context.Context, site string, d *NetworkMembersGroup, fields ...string) (*NetworkMembersGroup, error) {
	return bareMasked(ctx, c, fmt.Sprintf("v2/api/site/%s/network-members-group/%s", site, d.ID), d, fields)
}
