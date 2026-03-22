package runtime

import "testing"

func TestParseCSRFCookieValue(t *testing.T) {
	got := parseCSRFCookieValue("csrf-token-123%7Chash-value")
	want := "csrf-token-123"
	if got != want {
		t.Fatalf("csrf token mismatch: got %q want %q", got, want)
	}
}

func TestExtractContinueURLFromBody(t *testing.T) {
	body := []byte(`{"page":{"type":"email_otp_verification"},"continue_url":"/sign-in-with-chatgpt/codex/consent?foo=bar"}`)
	got := extractContinueURLFromBody(body)
	want := "/sign-in-with-chatgpt/codex/consent?foo=bar"
	if got != want {
		t.Fatalf("continue_url mismatch: got %q want %q", got, want)
	}
}

func TestExtractWorkspaceFromAuthHTML(t *testing.T) {
	html := `<html><head></head><body><script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"workspaces":[{"id":"123e4567-e89b-12d3-a456-426614174000"}]}}}</script></body></html>`
	got, ok := extractWorkspaceFromAuthHTML(html)
	if !ok {
		t.Fatal("expected workspace_id from auth HTML")
	}
	want := "123e4567-e89b-12d3-a456-426614174000"
	if got != want {
		t.Fatalf("workspace_id mismatch: got %q want %q", got, want)
	}
}

func TestExtractWorkspaceFromAccountsCheckFallbackFields(t *testing.T) {
	body := []byte(`{
		"accounts": {
			"bucket-1": {
				"account": {
					"account_id": null,
					"workspace_id": "123e4567-e89b-12d3-a456-426614174001"
				}
			}
		}
	}`)
	got, ok := extractWorkspaceFromAccountsCheck(body)
	if !ok {
		t.Fatal("expected workspace_id from accounts/check fallback fields")
	}
	want := "123e4567-e89b-12d3-a456-426614174001"
	if got != want {
		t.Fatalf("workspace_id mismatch: got %q want %q", got, want)
	}
}

func TestResolveAbsoluteURLAndCallbackDetection(t *testing.T) {
	got := resolveAbsoluteURL("/auth/callback?code=abc&state=xyz", "http://localhost:1455")
	want := "http://localhost:1455/auth/callback?code=abc&state=xyz"
	if got != want {
		t.Fatalf("absolute URL mismatch: got %q want %q", got, want)
	}
	if !isOAuthCallbackURL(got, "http://localhost:1455/auth/callback") {
		t.Fatal("expected callback URL to be recognized")
	}
	webCallback := "https://chatgpt.com/api/auth/callback/openai?code=abc&state=xyz"
	if !isOAuthCallbackURL(webCallback, chatGPTWebRedirectURI) {
		t.Fatal("expected chatgpt web callback URL to be recognized")
	}
}

func TestExtractTokenInfoFromChatGPTSession(t *testing.T) {
	body := []byte(`{
		"accessToken":"eyJhbGciOiJub25lIn0.eyJodHRwczovL2FwaS5vcGVuYWkuY29tL2F1dGgiOnsiY2hhdGdwdF9hY2NvdW50X2lkIjoiMTIzZTQ1NjctZTg5Yi0xMmQzLWE0NTYtNDI2NjE0MTc0MDAyIn19.",
		"user":{"id":"user_123"},
		"account_id":"123e4567-e89b-12d3-a456-426614174003"
	}`)
	info, workspaceID, err := extractTokenInfoFromChatGPTSession(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AccessToken == "" {
		t.Fatal("expected access token")
	}
	wantAccountID := "123e4567-e89b-12d3-a456-426614174002"
	if info.AccountID != wantAccountID {
		t.Fatalf("account id mismatch: got %q want %q", info.AccountID, wantAccountID)
	}
	if workspaceID != "123e4567-e89b-12d3-a456-426614174002" {
		t.Fatalf("workspace id mismatch: got %q", workspaceID)
	}
}
