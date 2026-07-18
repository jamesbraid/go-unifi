package main

import (
	"os"
	"path/filepath"
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

func TestCollectionForResource(t *testing.T) {
	assert.Equal(t, "networkconf", collectionForResource("NetworkConf.json", "Network"))
	assert.Equal(t, "wlanconf", collectionForResource("WlanConf.json", "WLAN"))
	assert.Equal(t, "setting", collectionForResource("SettingUsg.json", "SettingUsg"))
	assert.Equal(t, "user", collectionForResource("User.json", "Client"))
	assert.Equal(t, "usergroup", collectionForResource("UserGroup.json", "ClientGroup"))
	assert.Equal(t, "site", collectionForResource("Site.json", "Site"))
}

func TestMarkResource(t *testing.T) {
	r := NewResource("RadiusProfile", "radiusprofile")
	base := r.Types["RadiusProfile"]

	auth := NewFieldInfo("AuthServers", "auth_servers", "RadiusProfileAuthServers", "", true, false, false, "")
	auth.Fields = map[string]*FieldInfo{
		"IP":      NewFieldInfo("IP", "ip", fields.String, "", true, false, false, ""),
		"XSecret": NewFieldInfo("XSecret", "x_secret", fields.String, "", true, false, false, ""),
	}
	base.Fields["AuthServers"] = auth
	base.Fields["XPassphrase"] = NewFieldInfo("XPassphrase", "x_passphrase", fields.String, "", true, false, false, "")
	base.Fields["Name"] = NewFieldInfo("Name", "name", fields.String, "", true, false, false, "")

	meta := &sensitiveMetadata{ByCollection: map[string][]string{
		"radiusprofile": {"name", "auth_servers.x_secret", "x_passphrase", "bogus.path"},
	}}

	meta.markResource(r, "radiusprofile")

	assert.False(t, base.Fields["Name"].Sensitive, "name is allowlisted")
	assert.True(t, base.Fields["XPassphrase"].Sensitive)
	assert.True(t, auth.Fields["XSecret"].Sensitive, "nested leaf")
	assert.False(t, auth.Fields["IP"].Sensitive)
	// bogus.path: logged and skipped, no panic
}

func TestMarkResourceUnknownCollection(t *testing.T) {
	r := NewResource("DnsRecord", "static-dns")
	meta := &sensitiveMetadata{ByCollection: map[string][]string{}}
	meta.markResource(r, "dnsrecord") // must not panic
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
