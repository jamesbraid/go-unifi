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
