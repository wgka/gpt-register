package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"codex-register/internal/store"
)

type CPAUploadDetail struct {
	ID      int    `json:"id"`
	Email   string `json:"email,omitempty"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type CPAUploadBatchResult struct {
	SuccessCount int               `json:"success_count"`
	FailedCount  int               `json:"failed_count"`
	SkippedCount int               `json:"skipped_count"`
	Details      []CPAUploadDetail `json:"details"`
}

type CPAConfig struct {
	Enabled  bool
	APIURL   string
	APIToken string
	ProxyURL string
}

func GenerateCPATokenJSON(account *store.AccountTokens) map[string]any {
	return map[string]any{
		"type":          "codex",
		"email":         account.Email,
		"expired":       formatCPAOptionalTime(account.ExpiresAt),
		"id_token":      derefString(account.IDToken),
		"account_id":    derefString(account.AccountID),
		"access_token":  derefString(account.AccessToken),
		"last_refresh":  formatCPAOptionalTime(account.LastRefresh),
		"refresh_token": derefString(account.RefreshToken),
	}
}

func UploadAccountToCPA(ctx context.Context, db *store.SQLiteStore, accountID int, apiURL, apiToken, proxyURL string) (bool, string) {
	account, err := db.GetAccountTokensByID(ctx, accountID)
	if err != nil {
		return false, err.Error()
	}
	if account == nil {
		return false, "账号不存在"
	}
	if account.AccessToken == nil || strings.TrimSpace(*account.AccessToken) == "" {
		return false, "账号缺少 Token，无法上传"
	}

	success, message := uploadToCPA(ctx, GenerateCPATokenJSON(account), apiURL, apiToken, proxyURL)
	if !success {
		return false, message
	}

	if err := db.UpdateAccount(ctx, accountID, map[string]any{
		"cpa_uploaded":    true,
		"cpa_uploaded_at": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return false, err.Error()
	}
	return true, message
}

func BatchUploadAccountsToCPA(ctx context.Context, db *store.SQLiteStore, accountIDs []int, apiURL, apiToken, proxyURL string) CPAUploadBatchResult {
	result := CPAUploadBatchResult{Details: make([]CPAUploadDetail, 0, len(accountIDs))}

	for _, accountID := range accountIDs {
		account, err := db.GetAccountTokensByID(ctx, accountID)
		if err != nil {
			result.FailedCount++
			result.Details = append(result.Details, CPAUploadDetail{ID: accountID, Success: false, Error: err.Error()})
			continue
		}
		if account == nil {
			result.FailedCount++
			result.Details = append(result.Details, CPAUploadDetail{ID: accountID, Success: false, Error: "账号不存在"})
			continue
		}
		if account.AccessToken == nil || strings.TrimSpace(*account.AccessToken) == "" {
			result.SkippedCount++
			result.Details = append(result.Details, CPAUploadDetail{ID: accountID, Email: account.Email, Success: false, Error: "缺少 Token"})
			continue
		}

		success, message := uploadToCPA(ctx, GenerateCPATokenJSON(account), apiURL, apiToken, proxyURL)
		if !success {
			result.FailedCount++
			result.Details = append(result.Details, CPAUploadDetail{ID: accountID, Email: account.Email, Success: false, Error: message})
			continue
		}

		if err := db.UpdateAccount(ctx, accountID, map[string]any{
			"cpa_uploaded":    true,
			"cpa_uploaded_at": time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			result.FailedCount++
			result.Details = append(result.Details, CPAUploadDetail{ID: accountID, Email: account.Email, Success: false, Error: err.Error()})
			continue
		}

		result.SuccessCount++
		result.Details = append(result.Details, CPAUploadDetail{ID: accountID, Email: account.Email, Success: true, Message: message})
	}

	return result
}

func ResolveCPAConfig(ctx context.Context, db *store.SQLiteStore) CPAConfig {
	envURL := firstEnv("APP_CPA_API_URL", "CPA_API_URL")
	envToken := firstEnv("APP_CPA_API_TOKEN", "CPA_API_TOKEN")
	envEnabledRaw := firstEnv("APP_CPA_ENABLED", "CPA_ENABLED")
	envProxyURL := firstEnv("APP_CPA_PROXY_URL", "CPA_PROXY_URL")

	if envURL != "" || envToken != "" || envEnabledRaw != "" {
		enabled := true
		if envEnabledRaw != "" {
			enabled = parseEnvBool(envEnabledRaw)
		}
		return CPAConfig{
			Enabled:  enabled && envURL != "" && envToken != "",
			APIURL:   envURL,
			APIToken: envToken,
			ProxyURL: envProxyURL,
		}
	}

	apiURL, _ := db.GetSettingString(ctx, "cpa.api_url", "")
	apiToken, _ := db.GetSettingString(ctx, "cpa.api_token", "")
	enabled, _ := db.GetSettingBool(ctx, "cpa.enabled", false)

	return CPAConfig{
		Enabled:  enabled && strings.TrimSpace(apiURL) != "" && strings.TrimSpace(apiToken) != "",
		APIURL:   strings.TrimSpace(apiURL),
		APIToken: strings.TrimSpace(apiToken),
		ProxyURL: envProxyURL,
	}
}

func uploadToCPA(ctx context.Context, tokenData map[string]any, apiURL, apiToken, proxyURL string) (bool, string) {
	apiURL = normalizeCPAUploadEndpoint(apiURL)
	apiToken = strings.TrimSpace(apiToken)
	if apiURL == "" {
		return false, "CPA API URL 未配置"
	}
	if apiToken == "" {
		return false, "CPA API Token 未配置"
	}

	uploadURL := apiURL
	filename := fmt.Sprintf("%v.json", tokenData["email"])
	content, err := json.MarshalIndent(tokenData, "", "  ")
	if err != nil {
		return false, err.Error()
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return false, err.Error()
	}
	if _, err := part.Write(content); err != nil {
		return false, err.Error()
	}
	if err := writer.Close(); err != nil {
		return false, err.Error()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return false, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", userAgent())

	client := newHTTPClient(proxyURL, 30*time.Second, true)
	resp, err := client.Do(req)
	if err != nil {
		return false, "上传异常: " + err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return true, "上传成功"
	}
	return false, fmt.Sprintf("上传失败: HTTP %d", resp.StatusCode)
}

func normalizeCPAUploadEndpoint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
		return ""
	}

	pathname := strings.TrimRight(parsed.EscapedPath(), "/")
	if pathname == "" {
		pathname = "/v0/management/auth-files"
	}

	result := parsed.Scheme + "://" + parsed.Host + pathname
	if strings.TrimSpace(parsed.RawQuery) != "" {
		result += "?" + parsed.RawQuery
	}
	return result
}

func formatCPAOptionalTime(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return trimmed
	}
	return parsed.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02T15:04:05+08:00")
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func parseEnvBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
