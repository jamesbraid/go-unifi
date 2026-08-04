package main

import "testing"

func TestSelectManifestChoosesLockedArchitecture(t *testing.T) {
	digest, err := selectManifest([]byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:amd64","size":1,"platform":{"architecture":"amd64","os":"linux"}},{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:arm64","size":1,"platform":{"architecture":"arm64","os":"linux"}}]}`), "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if digest != "sha256:amd64" {
		t.Fatalf("manifest digest = %q", digest)
	}
}
