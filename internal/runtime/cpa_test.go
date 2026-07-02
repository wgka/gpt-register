package runtime

import (
	"testing"

	"codex-register/internal/store"
)

func TestGenerateCPATokenJSONShape(t *testing.T) {
	access := "access-token"
	idToken := "id-token"
	refresh := "refresh-token"
	accountID := "account-id"
	account := &store.AccountTokens{
		Email:        "user@example.com",
		AccessToken:  &access,
		IDToken:      &idToken,
		RefreshToken: &refresh,
		AccountID:    &accountID,
		ExtraData: map[string]any{
			"session_expires": "2026-07-12T02:39:22Z",
		},
	}

	got := GenerateCPATokenJSON(account)
	if got["type"] != "codex" || got["email"] != "user@example.com" || got["disabled"] != false {
		t.Fatalf("unexpected CPA json shape: %#v", got)
	}
	if got["access_token"] != access || got["id_token"] != idToken || got["refresh_token"] != refresh || got["account_id"] != accountID {
		t.Fatalf("unexpected token fields: %#v", got)
	}
	if got["expired"] == "" || got["last_refresh"] == "" {
		t.Fatalf("expected expired and last_refresh fields: %#v", got)
	}
}

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
