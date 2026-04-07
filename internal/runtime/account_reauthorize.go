package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codex-register/internal/store"
)

type AccountReauthorizeResult struct {
	Success      bool   `json:"success"`
	AuthUpdated  bool   `json:"auth_updated"`
	CPAUploaded  bool   `json:"cpa_uploaded"`
	Message      string `json:"message,omitempty"`
	ErrorMessage string `json:"error,omitempty"`
}

func ReauthorizeAccountWithCodexCLI(ctx context.Context, db *store.SQLiteStore, accountID int, proxyURL string, uploadCPA bool) AccountReauthorizeResult {
	account, err := db.GetAccountByID(ctx, accountID)
	if err != nil {
		return AccountReauthorizeResult{ErrorMessage: err.Error()}
	}
	if account == nil {
		return AccountReauthorizeResult{ErrorMessage: "账号不存在"}
	}

	password := ""
	if account.Password != nil {
		password = strings.TrimSpace(*account.Password)
	}
	if password == "" {
		return AccountReauthorizeResult{ErrorMessage: "账号缺少已保存密码，无法执行 Codex/CLI 授权"}
	}

	settings, err := loadEngineSettings(ctx, db)
	if err != nil {
		return AccountReauthorizeResult{ErrorMessage: err.Error()}
	}
	settings.AuthMode = authModeCodexCLI

	resolvedProxy := strings.TrimSpace(proxyURL)
	if resolvedProxy == "" && account.ProxyUsed != nil {
		resolvedProxy = strings.TrimSpace(*account.ProxyUsed)
	}
	resolvedProxy = ResolveProxy(resolvedProxy)

	service, emailInfo, err := buildExistingAccountEmailService(account.EmailService, resolvedProxy, account.Email)
	if err != nil {
		return AccountReauthorizeResult{ErrorMessage: err.Error()}
	}

	engine := newRegistrationEngine(settings, service, resolvedProxy, func(string) {})
	engine.email = account.Email
	engine.emailInfo = emailInfo
	engine.password = password

	prepared := RegistrationResult{
		Email:    account.Email,
		Password: password,
		Source:   "login",
	}
	if err := engine.prepareRegistration(ctx, &prepared); err != nil {
		return AccountReauthorizeResult{ErrorMessage: err.Error()}
	}

	result := engine.runPrepared(ctx, authModeCodexCLI)
	if !result.Success {
		return AccountReauthorizeResult{ErrorMessage: result.ErrorMessage}
	}

	tokens, err := db.GetAccountTokensByID(ctx, accountID)
	if err != nil {
		return AccountReauthorizeResult{ErrorMessage: err.Error()}
	}

	mergedExtra := map[string]any{}
	if tokens != nil {
		for key, value := range tokens.ExtraData {
			mergedExtra[key] = value
		}
	}
	for key, value := range result.Metadata {
		mergedExtra[key] = value
	}
	mergedExtra["reauthorized_at"] = time.Now().UTC().Format(time.RFC3339)
	mergedExtra["reauthorize_mode"] = authModeCodexCLI

	updates := map[string]any{
		"status":        "active",
		"client_id":     settings.OpenAIClientID,
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"id_token":      result.IDToken,
		"session_token": result.SessionToken,
		"last_refresh":  time.Now().UTC().Format(time.RFC3339),
		"account_id":    result.AccountID,
		"workspace_id":  result.WorkspaceID,
		"proxy_used":    resolvedProxy,
		"source":        "codex_cli_reauthorize",
		"extra_data":    mergedExtra,
	}
	if expiresAt := accessTokenExpiresAt(result.AccessToken); expiresAt != "" {
		updates["expires_at"] = expiresAt
	}
	if err := db.UpdateAccount(ctx, accountID, updates); err != nil {
		return AccountReauthorizeResult{ErrorMessage: err.Error()}
	}

	response := AccountReauthorizeResult{
		Success:     true,
		AuthUpdated: true,
		Message:     "Codex/CLI 授权已更新",
	}
	if !uploadCPA {
		return response
	}

	cpaConfig := ResolveCPAConfig(ctx, db)
	if strings.TrimSpace(cpaConfig.APIURL) == "" || strings.TrimSpace(cpaConfig.APIToken) == "" {
		response.Success = false
		response.ErrorMessage = "Codex/CLI 授权成功，但 CPA API URL 或 Token 未配置"
		return response
	}
	cpaProxy := resolvedProxy
	if strings.TrimSpace(cpaProxy) == "" {
		cpaProxy = strings.TrimSpace(cpaConfig.ProxyURL)
	}
	success, message := UploadAccountToCPA(ctx, db, accountID, cpaConfig.APIURL, cpaConfig.APIToken, cpaProxy)
	if !success {
		response.Success = false
		response.ErrorMessage = "Codex/CLI 授权成功，但上传 CPA 失败: " + message
		return response
	}
	response.CPAUploaded = true
	response.Message = "Codex/CLI 授权已更新，并已上报 CPA"
	return response
}

func buildExistingAccountEmailService(serviceType, proxyURL, email string) (EmailService, EmailInfo, error) {
	switch strings.TrimSpace(serviceType) {
	case "", "tempmail", "temp-email", "meteormail":
		return newTempMailService(proxyURL), EmailInfo{
			Email:     strings.TrimSpace(email),
			ServiceID: strings.TrimSpace(email),
		}, nil
	default:
		return nil, EmailInfo{}, fmt.Errorf("unsupported email service type: %s", serviceType)
	}
}

func accessTokenExpiresAt(accessToken string) string {
	claims, err := parseJWTClaims(accessToken)
	if err != nil {
		return ""
	}
	expRaw, ok := claims["exp"]
	if !ok {
		return ""
	}
	switch exp := expRaw.(type) {
	case float64:
		return time.Unix(int64(exp), 0).UTC().Format(time.RFC3339)
	case int64:
		return time.Unix(exp, 0).UTC().Format(time.RFC3339)
	case int:
		return time.Unix(int64(exp), 0).UTC().Format(time.RFC3339)
	default:
		return ""
	}
}
