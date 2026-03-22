package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"codex-register/internal/store"
)

const (
	sessionRefreshURL = "https://chatgpt.com/api/auth/session"
	tokenRefreshURL   = "https://auth.openai.com/oauth/token"
	tokenValidateURL  = "https://chatgpt.com/backend-api/me"
)

type TokenRefreshResult struct {
	Success      bool      `json:"success"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    string    `json:"expires_at,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	UpdatedAt    time.Time `json:"-"`
}

func RefreshAccountToken(ctx context.Context, db *store.SQLiteStore, accountID int, proxyURL string) TokenRefreshResult {
	account, err := db.GetAccountTokensByID(ctx, accountID)
	if err != nil {
		return TokenRefreshResult{ErrorMessage: err.Error()}
	}
	if account == nil {
		return TokenRefreshResult{ErrorMessage: "账号不存在"}
	}

	manager := tokenRefreshManager{proxyURL: strings.TrimSpace(proxyURL)}
	result := manager.refreshAccount(ctx, account)
	if !result.Success {
		return result
	}

	updates := map[string]any{
		"access_token": result.AccessToken,
		"last_refresh": result.UpdatedAt.Format(time.RFC3339),
	}
	if result.RefreshToken != "" {
		updates["refresh_token"] = result.RefreshToken
	}
	if result.ExpiresAt != "" {
		updates["expires_at"] = result.ExpiresAt
	}
	if err := db.UpdateAccount(ctx, accountID, updates); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
	}
	return result
}

func ValidateAccountToken(ctx context.Context, db *store.SQLiteStore, accountID int, proxyURL string) (bool, string) {
	account, err := db.GetAccountTokensByID(ctx, accountID)
	if err != nil {
		return false, err.Error()
	}
	if account == nil {
		return false, "账号不存在"
	}
	if account.AccessToken == nil || strings.TrimSpace(*account.AccessToken) == "" {
		return false, "账号没有 access_token"
	}

	manager := tokenRefreshManager{proxyURL: strings.TrimSpace(proxyURL)}
	return manager.validateToken(ctx, *account.AccessToken)
}

type tokenRefreshManager struct {
	proxyURL string
}

func (m tokenRefreshManager) refreshAccount(ctx context.Context, account *store.AccountTokens) TokenRefreshResult {
	if account.SessionToken != nil && strings.TrimSpace(*account.SessionToken) != "" {
		if result := m.refreshBySessionToken(ctx, *account.SessionToken); result.Success {
			return result
		}
	}
	if account.RefreshToken != nil && strings.TrimSpace(*account.RefreshToken) != "" {
		clientID := defaultOpenAIClientID
		if account.ClientID != nil && strings.TrimSpace(*account.ClientID) != "" {
			clientID = strings.TrimSpace(*account.ClientID)
		}
		return m.refreshByOAuthToken(ctx, *account.RefreshToken, clientID)
	}
	return TokenRefreshResult{ErrorMessage: "账号没有可用的刷新方式（缺少 session_token 和 refresh_token）"}
}

func (m tokenRefreshManager) refreshBySessionToken(ctx context.Context, sessionToken string) TokenRefreshResult {
	client := newHTTPClient(m.proxyURL, 30*time.Second, true)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sessionRefreshURL, nil)
	if err != nil {
		return TokenRefreshResult{ErrorMessage: err.Error()}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent())
	req.AddCookie(&http.Cookie{
		Name:   "__Secure-next-auth.session-token",
		Value:  sessionToken,
		Domain: ".chatgpt.com",
		Path:   "/",
	})

	resp, err := client.Do(req)
	if err != nil {
		return TokenRefreshResult{ErrorMessage: "Session token 刷新异常: " + err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return TokenRefreshResult{ErrorMessage: fmt.Sprintf("Session token 刷新失败: HTTP %d", resp.StatusCode)}
	}

	var payload struct {
		AccessToken string `json:"accessToken"`
		Expires     string `json:"expires"`
	}
	if err := jsonUnmarshalResponse(body, &payload); err != nil {
		return TokenRefreshResult{ErrorMessage: "Session token 刷新异常: " + err.Error()}
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return TokenRefreshResult{ErrorMessage: "Session token 刷新失败: 未找到 accessToken"}
	}

	result := TokenRefreshResult{
		Success:     true,
		AccessToken: payload.AccessToken,
		UpdatedAt:   time.Now().UTC(),
	}
	if parsed, err := time.Parse(time.RFC3339, strings.ReplaceAll(payload.Expires, "Z", "+00:00")); err == nil {
		result.ExpiresAt = parsed.UTC().Format(time.RFC3339)
	}
	return result
}

func (m tokenRefreshManager) refreshByOAuthToken(ctx context.Context, refreshToken, clientID string) TokenRefreshResult {
	form := url.Values{
		"client_id":     []string{clientID},
		"grant_type":    []string{"refresh_token"},
		"refresh_token": []string{refreshToken},
		"redirect_uri":  []string{defaultOpenAIRedirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenRefreshURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenRefreshResult{ErrorMessage: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent())

	client := newHTTPClient(m.proxyURL, 30*time.Second, true)
	resp, err := client.Do(req)
	if err != nil {
		return TokenRefreshResult{ErrorMessage: "OAuth token 刷新异常: " + err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return TokenRefreshResult{ErrorMessage: fmt.Sprintf("OAuth token 刷新失败: HTTP %d", resp.StatusCode)}
	}

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := jsonUnmarshalResponse(body, &payload); err != nil {
		return TokenRefreshResult{ErrorMessage: "OAuth token 刷新异常: " + err.Error()}
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return TokenRefreshResult{ErrorMessage: "OAuth token 刷新失败: 未找到 access_token"}
	}

	if payload.ExpiresIn <= 0 {
		payload.ExpiresIn = 3600
	}
	expiresAt := time.Now().UTC().Add(time.Duration(payload.ExpiresIn) * time.Second)

	if strings.TrimSpace(payload.RefreshToken) == "" {
		payload.RefreshToken = refreshToken
	}

	return TokenRefreshResult{
		Success:      true,
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		ExpiresAt:    expiresAt.Format(time.RFC3339),
		UpdatedAt:    time.Now().UTC(),
	}
}

func (m tokenRefreshManager) validateToken(ctx context.Context, accessToken string) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenValidateURL, nil)
	if err != nil {
		return false, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent())

	client := newHTTPClient(m.proxyURL, 30*time.Second, true)
	resp, err := client.Do(req)
	if err != nil {
		return false, "验证异常: " + err.Error()
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, ""
	case http.StatusUnauthorized:
		return false, "Token 无效或已过期"
	case http.StatusForbidden:
		return false, "账号可能被封禁"
	default:
		return false, fmt.Sprintf("验证失败: HTTP %d", resp.StatusCode)
	}
}
