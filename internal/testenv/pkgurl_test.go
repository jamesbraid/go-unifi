package testenv

import "testing"

func TestVersionFromPkgURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "real-style deb URL",
			url:  "https://fw-download.ubnt.com/data/unifi-controller/fa30-debian-10.4.57-86432683-a50a-4fd9-8e7b-21180c41611b.deb",
			want: "10.4.57",
		},
		{
			name: "no version in URL",
			url:  "https://fw-download.ubnt.com/data/unifi-controller/unifi.deb",
			want: "",
		},
		{
			name: "empty string",
			url:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := versionFromPkgURL(tt.url); got != tt.want {
				t.Errorf("versionFromPkgURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
