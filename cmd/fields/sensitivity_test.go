package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sensitivityMetadata(t *testing.T, db map[string][]string, distinct map[string]string, system []string) []byte {
	t.Helper()
	if db == nil {
		db = map[string][]string{}
	}
	if distinct == nil {
		distinct = map[string]string{}
	}
	if system == nil {
		system = []string{}
	}
	metadata := map[string]any{
		"min_field_size":                             5,
		"default_names":                              []string{"Default"},
		"sensitive_system_properties":                system,
		"sensitive_db_fields_by_collection":          db,
		"sensitive_distinct_db_fields_by_collection": distinct,
	}
	body, err := json.Marshal(metadata)
	require.NoError(t, err)
	return body
}

func approvedPolicy(t *testing.T, metadata []byte, secrets ...string) SensitivityPolicy {
	t.Helper()
	digest, err := CanonicalJSONDigest(metadata)
	require.NoError(t, err)
	return SensitivityPolicy{Version: "1", ApprovedMetadataSHA256: []string{digest}, SecretPaths: secrets, NonGeneratedSecretPaths: []string{}}
}

func resourceFromRaw(t *testing.T, structName, sourceBase string, prefix []string, raw []byte) *ResourceInfo {
	t.Helper()
	r := NewResource(structName, sourceBase)
	r.SourceFileBase = sourceBase
	r.SourcePathPrefix = prefix
	require.NoError(t, r.processJSON(raw))
	return r
}

func rawSchemas(entries map[string]string) RawSchemaIndex {
	raw := make(RawSchemaIndex, len(entries))
	for name, body := range entries {
		raw[name] = []byte(body)
	}
	return raw
}

func TestParseSensitivityMetadata_CompleteShape(t *testing.T) {
	body := sensitivityMetadata(t,
		map[string][]string{"networkconf": {"name", "x_wireguard_private_key"}},
		map[string]string{"radiusprofile": "auth_servers.x_secret"},
		[]string{"mongodb.password"},
	)

	got, err := ParseSensitiveMetadata(body)
	require.NoError(t, err)
	assert.Equal(t, 5, got.MinFieldSize)
	assert.Equal(t, []string{"Default"}, got.DefaultNames)
	assert.Equal(t, []string{"mongodb.password"}, got.SystemProperties)
	assert.Equal(t, []string{"name", "x_wireguard_private_key"}, got.DBFields["networkconf"])
	assert.Equal(t, "auth_servers.x_secret", got.DistinctDBFields["radiusprofile"])
}

func TestParseSensitivityMetadata_RejectsIncompleteOrMalformedShape(t *testing.T) {
	for name, body := range map[string]string{
		"missing section": `{"min_field_size":5,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{}}`,
		"wrong min size":  `{"min_field_size":"5","default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{}}`,
		"unknown field":   `{"min_field_size":5,"default_names":[],"sensitive_system_properties":[],"sensitive_db_fields_by_collection":{},"sensitive_distinct_db_fields_by_collection":{},"extra":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseSensitiveMetadata([]byte(body))
			require.Error(t, err)
		})
	}
}

func TestSensitivityPolicy_LoadAndValidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":"1","approved_metadata_sha256":[],"secret_paths":[],"non_generated_secret_paths":[]}`), 0o600))
	policy, err := LoadSensitivityPolicy(path)
	require.NoError(t, err)
	assert.Equal(t, "1", policy.Version)
	assert.Empty(t, policy.ApprovedMetadataSHA256)
	assert.Empty(t, policy.SecretPaths)
	assert.Empty(t, policy.NonGeneratedSecretPaths)

	require.NoError(t, os.WriteFile(path, []byte(`{"version":"2","approved_metadata_sha256":[],"secret_paths":[],"non_generated_secret_paths":[]}`), 0o600))
	_, err = LoadSensitivityPolicy(path)
	require.ErrorContains(t, err, "version")
}

func TestSensitivityPolicy_BootstrapFile(t *testing.T) {
	policy, err := LoadSensitivityPolicy("sensitive-policy.json")
	require.NoError(t, err)
	assert.Equal(t, "1", policy.Version)
	assert.Empty(t, policy.ApprovedMetadataSHA256)
	assert.Empty(t, policy.SecretPaths)
	assert.Empty(t, policy.NonGeneratedSecretPaths)
}

func TestApplySensitivity_ClassifiesExactLeavesAndCoverage(t *testing.T) {
	networkRaw := []byte(`{"name":".*","x_wireguard_private_key":".*","backup_password_hint":".*"}`)
	radiusRaw := []byte(`{"auth_servers":[{"ip":".*","x_secret":".*"}]}`)
	network := resourceFromRaw(t, "Network", "NetworkConf", nil, networkRaw)
	radius := resourceFromRaw(t, "RADIUSProfile", "RadiusProfile", nil, radiusRaw)
	metadata := sensitivityMetadata(t,
		map[string][]string{
			"networkconf":   {"name", "x_wireguard_private_key", "missing_private_field"},
			"radiusprofile": {"auth_servers.x_secret"},
			"wall":          {"private_value"},
		}, nil, []string{"mongodb.password"})
	policy := approvedPolicy(t, metadata,
		"networkconf.x_wireguard_private_key",
		"radiusprofile.auth_servers.x_secret",
	)

	coverage, err := ApplySensitivity([]*ResourceInfo{network, radius}, rawSchemas(map[string]string{
		"NetworkConf":   string(networkRaw),
		"RadiusProfile": string(radiusRaw),
		"Wall":          `{"private_value":".*"}`,
	}), metadata, policy)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"networkconf.name",
		"networkconf.x_wireguard_private_key",
		"radiusprofile.auth_servers.x_secret",
	}, coverage.Generated)
	assert.Equal(t, []string{
		"networkconf.missing_private_field",
		"systemproperty.mongodb.password",
		"wall.private_value",
	}, coverage.NonGenerated)
	assert.Equal(t, []string{
		"networkconf.x_wireguard_private_key",
		"radiusprofile.auth_servers.x_secret",
	}, coverage.SecretGenerated)
	assert.Empty(t, coverage.SecretNonGenerated)
	assert.Equal(t, []string{"networkconf.name"}, coverage.PrivateGenerated)
	assert.Equal(t, []string{
		"networkconf.missing_private_field",
		"systemproperty.mongodb.password",
		"wall.private_value",
	}, coverage.PrivateNonGenerated)

	base := network.Types[network.StructName]
	assert.True(t, base.Fields["WireguardPrivateKey"].Sensitive)
	assert.False(t, base.Fields["Name"].Sensitive, "private metadata remains visible")
	assert.False(t, base.Fields["BackupPasswordHint"].Sensitive, "field-name substrings never infer secrets")
	radiusSecret := radius.Types[radius.StructName].Fields["AuthServers"].Fields["Secret"]
	assert.True(t, radiusSecret.Sensitive)
}

func TestApplySensitivity_CustomOverlayUsesRawSourceIdentity(t *testing.T) {
	rawBody := `{"x_secret":".*","name":".*"}`
	r := NewResource("FirewallPolicy", "firewall-policies")
	r.SourceFileBase = "FirewallPolicy"
	r.Types[r.StructName].Fields["Secret"] = NewFieldInfo("Secret", "x_secret", "string", "", false, false, false, "")
	metadata := sensitivityMetadata(t, map[string][]string{"firewallpolicy": {"x_secret"}}, nil, nil)

	coverage, err := ApplySensitivity([]*ResourceInfo{r}, rawSchemas(map[string]string{"FirewallPolicy": rawBody}), metadata,
		approvedPolicy(t, metadata, "firewallpolicy.x_secret"))
	require.NoError(t, err)
	assert.Equal(t, []string{"firewallpolicy.x_secret"}, coverage.Generated)
	assert.True(t, r.Types[r.StructName].Fields["Secret"].Sensitive)
}

func TestApplySensitivity_SettingExpansionAndSkippedTerraform(t *testing.T) {
	settingRaw := []byte(`{"mgmt":{"password":".*","name":".*"},"usg":{"password":".*"}}`)
	mgmt := resourceFromRaw(t, "SettingMgmt", "Setting", []string{"mgmt"}, []byte(`{"password":".*","name":".*"}`))
	usg := resourceFromRaw(t, "SettingUsg", "Setting", []string{"usg"}, []byte(`{"password":".*"}`))
	metadata := sensitivityMetadata(t, map[string][]string{"setting": {"password"}}, nil, nil)

	coverage, err := ApplySensitivity([]*ResourceInfo{mgmt, usg}, rawSchemas(map[string]string{"Setting": string(settingRaw)}), metadata,
		approvedPolicy(t, metadata))
	require.NoError(t, err)
	assert.Empty(t, coverage.Generated)
	assert.Equal(t, []string{"setting.mgmt.password", "setting.usg.password"}, coverage.NonGenerated)

	policy := approvedPolicy(t, metadata)
	policy.NonGeneratedSecretPaths = []string{"setting.mgmt.password"}
	coverage, err = ApplySensitivity([]*ResourceInfo{mgmt, usg}, rawSchemas(map[string]string{"Setting": string(settingRaw)}), metadata, policy)
	require.NoError(t, err)
	assert.Equal(t, []string{"setting.mgmt.password"}, coverage.SecretNonGenerated)
	assert.Equal(t, []string{"setting.usg.password"}, coverage.PrivateNonGenerated)
}

func TestApplySensitivity_NonGeneratedSecretPathsEnforceStatus(t *testing.T) {
	metadata := sensitivityMetadata(t, map[string][]string{"wall": {"password"}}, nil, nil)
	policy := approvedPolicy(t, metadata)
	policy.NonGeneratedSecretPaths = []string{"wall.password", "missingcollection.token"}
	coverage, err := ApplySensitivity(nil, rawSchemas(map[string]string{"Wall": `{"password":".*"}`}), metadata, policy)
	require.NoError(t, err)
	assert.Equal(t, []string{"missingcollection.token", "wall.password"}, coverage.SecretNonGenerated)
	assert.Equal(t, []string{"missingcollection.token", "wall.password"}, coverage.NonGenerated)

	generated := resourceFromRaw(t, "Wall", "Wall", nil, []byte(`{"password":".*"}`))
	_, err = ApplySensitivity([]*ResourceInfo{generated}, rawSchemas(map[string]string{"Wall": `{"password":".*"}`}), metadata, policy)
	require.ErrorContains(t, err, "became generated")

	generatedPolicy := approvedPolicy(t, metadata, "wall.password")
	_, err = ApplySensitivity(nil, rawSchemas(map[string]string{"Wall": `{"password":".*"}`}), metadata, generatedPolicy)
	require.ErrorContains(t, err, "generated leaf")

	customOnly := NewResource("MissingCollection", "missingcollection")
	customOnly.SourceFileBase = "MissingCollection"
	customOnly.Types[customOnly.StructName].Fields["Token"] = NewFieldInfo("Token", "token", "string", "", false, false, false, "")
	absentPolicy := approvedPolicy(t, metadata)
	absentPolicy.NonGeneratedSecretPaths = []string{"missingcollection.token"}
	_, err = ApplySensitivity([]*ResourceInfo{customOnly}, rawSchemas(map[string]string{"Wall": `{"password":".*"}`}), metadata, absentPolicy)
	require.ErrorContains(t, err, "became generated")
}

func TestApplySensitivity_ExplicitPolicyPathMayExtendMetadata(t *testing.T) {
	rawBody := []byte(`{"new_secret":".*"}`)
	r := resourceFromRaw(t, "Network", "NetworkConf", nil, rawBody)
	metadata := sensitivityMetadata(t, map[string][]string{}, map[string]string{}, []string{})

	coverage, err := ApplySensitivity([]*ResourceInfo{r}, rawSchemas(map[string]string{"NetworkConf": string(rawBody)}), metadata,
		approvedPolicy(t, metadata, "networkconf.new_secret"))
	require.NoError(t, err)
	assert.Equal(t, []string{"networkconf.new_secret"}, coverage.Generated)
	assert.True(t, r.Types[r.StructName].Fields["NewSecret"].Sensitive)
}

func TestApplySensitivity_FailuresAreTransactional(t *testing.T) {
	validRaw := rawSchemas(map[string]string{"NetworkConf": `{"secret":".*","container":{"child":".*"}}`})
	r := resourceFromRaw(t, "Network", "NetworkConf", nil, []byte(`{"secret":".*","container":{"child":".*"}}`))
	secret := r.Types[r.StructName].Fields["Secret"]
	container := r.Types[r.StructName].Fields["Container"]
	secret.Sensitive = true
	container.Sensitive = true
	metadata := sensitivityMetadata(t, map[string][]string{"networkconf": {"secret"}}, nil, nil)

	tests := []struct {
		name   string
		raw    RawSchemaIndex
		policy SensitivityPolicy
		meta   []byte
	}{
		{"unapproved digest", validRaw, SensitivityPolicy{Version: "1", ApprovedMetadataSHA256: []string{"bad"}}, metadata},
		{"obsolete policy path", validRaw, approvedPolicy(t, metadata, "networkconf.gone"), metadata},
		{"scalar traversal", validRaw, approvedPolicy(t, metadata, "networkconf.secret.child"), metadata},
		{"ambiguous raw collection", rawSchemas(map[string]string{"NetworkConf": `{"secret":".*"}`, "networkconf": `{"secret":".*"}`}), approvedPolicy(t, metadata), metadata},
		{"malformed path", validRaw, approvedPolicy(t, metadata, "networkconf..secret"), metadata},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ApplySensitivity([]*ResourceInfo{r}, tc.raw, tc.meta, tc.policy)
			require.Error(t, err)
			assert.True(t, secret.Sensitive)
			assert.True(t, container.Sensitive)
		})
	}
}

func TestApplySensitivity_RejectsGeneratedIdentityAndJSONNameAmbiguity(t *testing.T) {
	rawBody := `{"secret":".*"}`
	metadata := sensitivityMetadata(t, map[string][]string{"networkconf": {"secret"}}, nil, nil)
	policy := approvedPolicy(t, metadata, "networkconf.secret")

	t.Run("resource identity", func(t *testing.T) {
		a := resourceFromRaw(t, "Network", "NetworkConf", nil, []byte(rawBody))
		b := resourceFromRaw(t, "NetworkCopy", "networkconf", nil, []byte(rawBody))
		_, err := ApplySensitivity([]*ResourceInfo{a, b}, rawSchemas(map[string]string{"NetworkConf": rawBody}), metadata, policy)
		require.ErrorContains(t, err, "ambiguous")
	})

	t.Run("duplicate JSON name", func(t *testing.T) {
		r := resourceFromRaw(t, "Network", "NetworkConf", nil, []byte(rawBody))
		r.Types[r.StructName].Fields["AnotherSecret"] = NewFieldInfo("AnotherSecret", "secret", "string", "", false, false, false, "")
		_, err := ApplySensitivity([]*ResourceInfo{r}, rawSchemas(map[string]string{"NetworkConf": rawBody}), metadata, policy)
		require.ErrorContains(t, err, "duplicate JSONName")
	})
}

func TestApplySensitivity_DeduplicatesAliasesAndClearsStaleFlags(t *testing.T) {
	rawBody := []byte(`{"secret":".*","old_secret":".*"}`)
	r := resourceFromRaw(t, "Network", "NetworkConf", nil, rawBody)
	base := r.Types[r.StructName]
	old := base.Fields["OldSecret"]
	old.Sensitive = true
	r.Types["Alias"] = base
	metadata := sensitivityMetadata(t, map[string][]string{"networkconf": {"secret"}}, nil, nil)

	_, err := ApplySensitivity([]*ResourceInfo{r}, rawSchemas(map[string]string{"NetworkConf": string(rawBody)}), metadata,
		approvedPolicy(t, metadata, "networkconf.secret"))
	require.NoError(t, err)
	assert.True(t, base.Fields["Secret"].Sensitive)
	assert.False(t, old.Sensitive)
}

func TestResourceSourceIdentity_SettingUsesUnsplitRawKey(t *testing.T) {
	r := NewResource("SettingGlobalAp", "settingglobalap")
	err := SetResourceSourceIdentity(r, "SettingGlobalAp.json", []byte(`{"global_ap":{},"mgmt":{}}`))
	require.NoError(t, err)
	assert.Equal(t, "Setting", r.SourceFileBase)
	assert.Equal(t, []string{"global_ap"}, r.SourcePathPrefix)

	for name, raw := range map[string]string{
		"missing":   `{"mgmt":{}}`,
		"ambiguous": `{"global_ap":{},"global__ap":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			r := NewResource("SettingGlobalAp", "settingglobalap")
			require.Error(t, SetResourceSourceIdentity(r, "SettingGlobalAp.json", []byte(raw)))
		})
	}
}
