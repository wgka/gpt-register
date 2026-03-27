package runtime

import "testing"

func TestNormalizeCPAUploadEndpoint(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "base url appends default management path",
			raw:  "http://127.0.0.1:8317",
			want: "http://127.0.0.1:8317/v0/management/auth-files",
		},
		{
			name: "full endpoint kept as is",
			raw:  "http://127.0.0.1:8317/v0/management/auth-files",
			want: "http://127.0.0.1:8317/v0/management/auth-files",
		},
		{
			name: "trailing slash trimmed",
			raw:  "http://127.0.0.1:8317/v0/management/auth-files/",
			want: "http://127.0.0.1:8317/v0/management/auth-files",
		},
		{
			name: "query string preserved",
			raw:  "http://127.0.0.1:8317/v0/management/auth-files?foo=bar",
			want: "http://127.0.0.1:8317/v0/management/auth-files?foo=bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCPAUploadEndpoint(tt.raw)
			if got != tt.want {
				t.Fatalf("normalizeCPAUploadEndpoint(%q)=%q want %q", tt.raw, got, tt.want)
			}
		})
	}
}
