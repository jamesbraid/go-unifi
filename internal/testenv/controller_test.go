package testenv

import "testing"

func TestControllerConfigFromEnv(t *testing.T) {
	t.Setenv("UNIFI_TEST_IMAGE", "jacobalberty/unifi:v99")
	t.Setenv("UNIFI_TEST_PKGURL", "https://example.invalid/unifi.deb")

	cfg := configFromEnv()
	if cfg.Image != "jacobalberty/unifi:v99" {
		t.Errorf("Image = %q", cfg.Image)
	}
	if cfg.PkgURL != "https://example.invalid/unifi.deb" {
		t.Errorf("PkgURL = %q", cfg.PkgURL)
	}
}

func TestControllerConfigDefaults(t *testing.T) {
	t.Setenv("UNIFI_TEST_IMAGE", "")
	t.Setenv("UNIFI_TEST_PKGURL", "")

	cfg := configFromEnv()
	if cfg.Image != defaultImage {
		t.Errorf("Image = %q, want %q", cfg.Image, defaultImage)
	}
	if cfg.PkgURL != "" {
		t.Errorf("PkgURL = %q, want empty", cfg.PkgURL)
	}
}
