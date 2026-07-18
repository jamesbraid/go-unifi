package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

func TestLoadSensitiveMetadataAbsent(t *testing.T) {
	meta, err := loadSensitiveMetadata(t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, meta)
}

func TestLoadSensitiveMetadata(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "metadata"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "metadata", "sensitive_metadata.json"),
		[]byte(`{"sensitive_db_fields_by_collection":{"wlanconf":["x_passphrase"]}}`),
		0o644,
	))

	meta, err := loadSensitiveMetadata(dir)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, []string{"x_passphrase"}, meta.ByCollection["wlanconf"])
}

func TestLoadSensitiveMetadataInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "metadata"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "metadata", "sensitive_metadata.json"),
		[]byte(`{not json`),
		0o644,
	))

	meta, err := loadSensitiveMetadata(dir)
	require.Error(t, err)
	assert.Nil(t, meta)
	assert.Contains(t, err.Error(), "unable to parse")
}

func TestCollectionForResource(t *testing.T) {
	assert.Equal(t, "networkconf", collectionForResource("NetworkConf.json", "Network"))
	assert.Equal(t, "wlanconf", collectionForResource("WlanConf.json", "WLAN"))
	assert.Equal(t, "setting", collectionForResource("SettingUsg.json", "SettingUsg"))
	assert.Equal(t, "user", collectionForResource("User.json", "Client"))
	assert.Equal(t, "usergroup", collectionForResource("UserGroup.json", "ClientGroup"))
	assert.Equal(t, "site", collectionForResource("Site.json", "Site"))
}

func radiusProfileResource() *ResourceInfo {
	r := NewResource("RadiusProfile", "radiusprofile")
	base := r.Types["RadiusProfile"]

	auth := NewFieldInfo("AuthServers", "auth_servers", "RadiusProfileAuthServers", "", true, true, false, "")
	auth.Fields = map[string]*FieldInfo{
		"IP":      NewFieldInfo("IP", "ip", fields.String, "", true, false, false, ""),
		"XSecret": NewFieldInfo("XSecret", "x_secret", fields.String, "", true, false, false, ""),
	}
	base.Fields["AuthServers"] = auth
	base.Fields["XPassphrase"] = NewFieldInfo("XPassphrase", "x_passphrase", fields.String, "", true, false, false, "")
	base.Fields["Name"] = NewFieldInfo("Name", "name", fields.String, "", true, false, false, "")
	return r
}

func TestMarkResource(t *testing.T) {
	r := radiusProfileResource()
	base := r.Types["RadiusProfile"]
	auth := base.Fields["AuthServers"]

	meta := &sensitiveMetadata{ByCollection: map[string][]string{
		"radiusprofile": {"name", "auth_servers.x_secret", "x_passphrase", "bogus.path"},
	}}

	marked, missed := meta.markResource(r, "radiusprofile")

	assert.Equal(t, 2, marked, "x_passphrase + auth_servers.x_secret")
	assert.Equal(t, []string{"bogus.path"}, missed)
	assert.False(t, base.Fields["Name"].Sensitive, "name is allowlisted")
	assert.True(t, base.Fields["XPassphrase"].Sensitive)
	assert.True(t, auth.Fields["XSecret"].Sensitive, "nested leaf")
	assert.False(t, auth.Fields["IP"].Sensitive)
}

func TestMarkResourceUnknownCollection(t *testing.T) {
	r := NewResource("DnsRecord", "static-dns")
	meta := &sensitiveMetadata{ByCollection: map[string][]string{}}

	marked, missed := meta.markResource(r, "dnsrecord") // must not panic
	assert.Equal(t, 0, marked)
	assert.Nil(t, missed)
}

func TestMarkResourceMissingLeafUnderExistingParent(t *testing.T) {
	r := radiusProfileResource()

	meta := &sensitiveMetadata{ByCollection: map[string][]string{
		"radiusprofile": {"auth_servers.bogus"},
	}}

	marked, missed := meta.markResource(r, "radiusprofile")
	assert.Equal(t, 0, marked)
	assert.Equal(t, []string{"auth_servers.bogus"}, missed)
}

func TestMarkResourceAllowlistedNestedLeaf(t *testing.T) {
	r := NewResource("Foo", "foo")
	base := r.Types["Foo"]

	nested := NewFieldInfo("Nested", "nested", "FooNested", "", true, false, false, "")
	nested.Fields = map[string]*FieldInfo{
		"Name": NewFieldInfo("Name", "name", fields.String, "", true, false, false, ""),
	}
	base.Fields["Nested"] = nested

	meta := &sensitiveMetadata{ByCollection: map[string][]string{
		"foo": {"nested.name"},
	}}

	marked, missed := meta.markResource(r, "foo")
	assert.Equal(t, 0, marked, "name is allowlisted")
	assert.Empty(t, missed)
	assert.False(t, nested.Fields["Name"].Sensitive)
}

func TestSensitiveAudit(t *testing.T) {
	resourceWithFields := func(structName string, jsonNames ...string) *ResourceInfo {
		r := NewResource(structName, "setting_"+strings.ToLower(structName))
		base := r.Types[structName]
		for _, j := range jsonNames {
			base.Fields[j] = NewFieldInfo(j, j, fields.String, "", true, false, false, "")
		}
		return r
	}

	a := resourceWithFields("SettingAlpha", "common", "alpha_only")
	b := resourceWithFields("SettingBeta", "common", "beta_only")

	meta := &sensitiveMetadata{ByCollection: map[string][]string{
		"setting":  {"common", "beta_only", "missed_by_all"},
		"wlanconf": {"x_passphrase"},
	}}

	audit := newSensitiveAudit()
	audit.record(meta, a, "setting")
	audit.record(meta, b, "setting")

	lines := audit.lines(meta)

	// missed by every resource in the collection: reported once
	assert.Contains(t, lines, "sensitive metadata: setting.missed_by_all not found in any schema")
	// reverse audit: upstream collection no resource consumed
	assert.Contains(t, lines, `sensitive metadata: collection "wlanconf" has no generated resource`)
	// summary: common marked twice + beta_only once
	assert.Contains(t, lines, "sensitive metadata: marked 3 fields across 1 collections")
	// partial misses are noise and must not appear
	for _, l := range lines {
		assert.NotContains(t, l, "beta_only")
		assert.NotContains(t, l, "alpha_only")
	}
	assert.Len(t, lines, 3)
}
