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
		{
			name: "leftmost dash-delimited triple (build ID) should be ignored",
			url:  "https://example.com/build-1.0.0-10.4.57-uuid.deb",
			want: "10.4.57",
		},
		{
			name: "version directly before .deb with no trailing dash",
			url:  "https://host/fa30-debian-10.4.57.deb",
			want: "10.4.57",
		},
		{
			name: "canonical dl.ui.com URL carries the version in a directory segment",
			url:  "https://dl.ui.com/unifi/10.4.57/unifi_sysvinit_all.deb",
			want: "10.4.57",
		},
		{
			name: "query string must not pollute the filename heuristic",
			url:  "https://fw-download.ubnt.com/data/unifi-controller/fa30-debian-10.4.57-86432683-a50a-4fd9-8e7b-21180c41611b.deb?token=abc-1.2.3-def",
			want: "10.4.57",
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
