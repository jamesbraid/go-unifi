package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDriftCompare(t *testing.T) {
	schema := map[string]any{
		"name":        "",
		"enabled":     "true|false",
		"network_ids": []any{""},
	}
	observed := []map[string]any{
		{"_id": "a", "name": "x", "enabled": true, "origin_type": "zbf"},
		{"_id": "b", "name": "y", "site_id": "s", "sorting_weight": 1},
	}

	r := driftCompare(observed, schema)

	// _id/site_id are controller envelope, ignored; origin_type and
	// sorting_weight are genuine drift.
	require.Equal(t, []string{"origin_type", "sorting_weight"}, r.LiveOnly)
	// network_ids never appeared in live output: informational.
	require.Equal(t, []string{"network_ids"}, r.SchemaOnly)
}

func TestDriftCompareEmptyObserved(t *testing.T) {
	r := driftCompare(nil, map[string]any{"name": ""})
	require.Empty(t, r.LiveOnly)
	require.Equal(t, []string{"name"}, r.SchemaOnly)
}

func TestObservedObjects(t *testing.T) {
	tests := []struct {
		name string
		body any
		want []map[string]any
	}{
		{
			name: "collection",
			body: []any{map[string]any{"_id": "a"}, map[string]any{"_id": "b"}},
			want: []map[string]any{{"_id": "a"}, {"_id": "b"}},
		},
		{
			// ospf/router and bgp/config answer with the document itself
			// rather than a one-element list.
			name: "singleton object",
			body: map[string]any{"router_id": "0.0.0.1"},
			want: []map[string]any{{"router_id": "0.0.0.1"}},
		},
		{
			name: "empty collection",
			body: []any{},
			want: nil,
		},
		{
			// Some controllers serve an empty v2 collection as JSON null.
			name: "null body",
			body: nil,
			want: nil,
		},
		{
			name: "non-object members are dropped",
			body: []any{"scalar", map[string]any{"_id": "a"}},
			want: []map[string]any{{"_id": "a"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, observedObjects(tt.body))
		})
	}
}
