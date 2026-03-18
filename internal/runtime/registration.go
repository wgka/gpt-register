package runtime

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	mrand "math/rand/v2"
	"net/url"
	"regexp"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"

	"codex-register/internal/store"
)

const (
	defaultOpenAIClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	defaultOpenAIAuthURL     = "https://auth.openai.com/oauth/authorize"
	defaultOpenAITokenURL    = "https://auth.openai.com/oauth/token"
	defaultOpenAIRedirectURI = "http://localhost:1455/auth/callback"
	defaultOpenAIScope       = "openid email profile offline_access"
	otpPattern               = `\b(\d{6})\b`
)

var checkoutSessionPattern = regexp.MustCompile(`/checkout/openai_llc/([A-Za-z0-9_-]+)`)

type engineSettings struct {
	OpenAIClientID        string
	OpenAIAuthURL         string
	OpenAITokenURL        string
	OpenAIRedirectURI     string
	OpenAIScope           string
	DefaultPasswordLength int
	EmailCodeTimeout      int
}

type RegistrationResult struct {
	Success      bool           `json:"success"`
	Email        string         `json:"email"`
	Password     string         `json:"password,omitempty"`
	AccountID    string         `json:"account_id,omitempty"`
	WorkspaceID  string         `json:"workspace_id,omitempty"`
	AccessToken  string         `json:"access_token,omitempty"`
	RefreshToken string         `json:"refresh_token,omitempty"`
	IDToken      string         `json:"id_token,omitempty"`
	SessionToken string         `json:"session_token,omitempty"`
	BindCardURL  string         `json:"bind_card_url,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Source       string         `json:"source"`
}

type oauthStart struct {
	AuthURL      string
	State        string
	CodeVerifier string
}

type tokenInfo struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	AccountID    string
}

type registrationEngine struct {
	settings   engineSettings
	service    EmailService
	proxyURL   string
	client     tls_client.HttpClient
	logf       func(string)
	email      string
	password   string
	emailInfo  EmailInfo
	oauthStart oauthStart
	isExisting bool
}

func newRegistrationEngine(settings engineSettings, service EmailService, proxyURL string, logf func(string)) *registrationEngine {
	client, err := newBrowserClient(proxyURL)
	if err != nil {
		client = nil
	}
	return &registrationEngine{
		settings: settings,
		service:  service,
		proxyURL: proxyURL,
		client:   client,
		logf:     logf,
	}
}

func (e *registrationEngine) run(ctx context.Context) RegistrationResult {
	result := RegistrationResult{Success: false, Source: "register"}
	if e.client == nil {
		result.ErrorMessage = "browser session initialization failed"
		return result
	}

	e.logf("检查代理出口地区")
	if ok, location := e.checkIPLocation(ctx); !ok {
		result.ErrorMessage = "unsupported ip location: " + location
		return result
	} else if strings.TrimSpace(location) != "" {
		e.logf("代理地区: " + location)
	}

	e.logf("创建邮箱")
	emailInfo, err := e.service.CreateEmail(ctx)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.emailInfo = emailInfo
	e.email = emailInfo.Email
	result.Email = e.email
	e.logf("邮箱已创建: " + e.email)

	e.oauthStart = startOAuth(e.settings)
	e.logf("OAuth 已初始化")

	did, err := e.getDeviceID(ctx)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("设备标识已获取")

	sentinelToken, _ := e.checkSentinel(ctx, did)
	if strings.TrimSpace(sentinelToken) != "" {
		e.logf("Sentinel 校验通过")
	} else {
		e.logf("Sentinel 未返回 token，继续尝试注册")
	}

	signupPage, err := e.submitSignup(ctx, did, sentinelToken)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("注册入口页面: " + signupPage)

	e.isExisting = signupPage == "email_otp_verification"
	if e.isExisting {
		result.Source = "login"
		e.logf("检测到已注册邮箱，自动走登录流程")
	}

	if !e.isExisting {
		e.logf("提交密码")
		password, err := e.registerPassword(ctx)
		if err != nil {
			result.ErrorMessage = err.Error()
			return result
		}
		e.password = password
		result.Password = password
		e.logf("密码已设置: " + password)

		e.logf("请求发送邮箱验证码")
		if err := e.sendVerificationCode(ctx); err != nil {
			result.ErrorMessage = err.Error()
			return result
		}
		e.logf("验证码已发送")
	}

	e.logf("等待邮箱验证码")
	code, err := e.service.GetVerificationCode(ctx, e.email, e.emailInfo.ServiceID, time.Duration(e.settings.EmailCodeTimeout)*time.Second, otpPattern)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("邮箱验证码: " + code)

	e.logf("校验邮箱验证码")
	if err := e.validateVerificationCode(ctx, code); err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("验证码校验通过")

	if !e.isExisting {
		e.logf("创建 OpenAI 账户资料")
		if err := e.createUserAccount(ctx); err != nil {
			result.ErrorMessage = err.Error()
			return result
		}
		e.logf("账户资料创建完成")
	}

	e.logf("读取 workspace")
	workspaceID, err := e.getWorkspaceID()
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	result.WorkspaceID = workspaceID
	e.logf("Workspace ID: " + workspaceID)

	e.logf("选择 workspace")
	continueURL, err := e.selectWorkspace(ctx, workspaceID)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("Workspace 已选择")

	e.logf("跟随授权跳转")
	callbackURL, err := e.followRedirects(ctx, continueURL)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("收到 OAuth 回调")

	e.logf("交换 access_token / refresh_token")
	tokenInfo, err := e.exchangeCallback(ctx, callbackURL)
	if err != nil {
		e.logf("Token 交换失败: " + err.Error())
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("Token 交换完成")

	result.Success = true
	result.AccountID = tokenInfo.AccountID
	result.AccessToken = tokenInfo.AccessToken
	result.RefreshToken = tokenInfo.RefreshToken
	result.IDToken = tokenInfo.IDToken
	result.Password = e.password
	result.SessionToken = e.getCookieValue("https://chatgpt.com", "__Secure-next-auth.session-token")

	e.logf("生成绑卡链接")
	bindCardURL, err := GenerateBindCardLink(ctx, tokenInfo.AccessToken, e.proxyURL)
	if err != nil {
		e.logf("绑卡链接生成失败: " + err.Error())
	} else {
		result.BindCardURL = bindCardURL
		e.logf("绑卡链接已生成")
	}

	result.Metadata = map[string]any{
		"email_service":       e.service.Type(),
		"proxy_used":          e.proxyURL,
		"registered_at":       time.Now().Format(time.RFC3339),
		"is_existing_account": e.isExisting,
	}
	if result.BindCardURL != "" {
		result.Metadata["bind_card_url"] = result.BindCardURL
	}
	e.logf("注册流程完成")

	return result
}

func (e *registrationEngine) checkIPLocation(ctx context.Context) (bool, string) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://cloudflare.com/cdn-cgi/trace", nil)
	req.Header = browserHeaders(http.Header{
		"accept":             {"*/*"},
		"accept-language":    {"en-US,en;q=0.9"},
		"accept-encoding":    {"gzip, deflate, br"},
		"user-agent":         {userAgent()},
		"sec-ch-ua":          {secCHUA()},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"cross-site"},
	}, "accept", "accept-language", "accept-encoding", "user-agent", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site")
	resp, err := e.client.Do(req)
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	match := regexp.MustCompile(`loc=([A-Z]+)`).FindStringSubmatch(string(body))
	if len(match) < 2 {
		return true, ""
	}
	location := match[1]
	switch location {
	case "CN", "HK", "MO", "TW":
		return false, location
	default:
		return true, location
	}
}

func (e *registrationEngine) getDeviceID(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, e.oauthStart.AuthURL, nil)
	req.Header = authPageHeaders()
	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	parsedURL, _ := url.Parse(e.oauthStart.AuthURL)
	for _, cookie := range e.client.GetCookies(parsedURL) {
		if cookie.Name == "oai-did" {
			return cookie.Value, nil
		}
	}
	return "", fmt.Errorf("missing oai-did cookie")
}

func (e *registrationEngine) checkSentinel(ctx context.Context, did string) (string, error) {
	body := fmt.Sprintf(`{"p":"","id":"%s","flow":"authorize_continue"}`, did)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://sentinel.openai.com/backend-api/sentinel/req", strings.NewReader(body))
	req.Header = browserHeaders(http.Header{
		"origin":             {"https://sentinel.openai.com"},
		"referer":            {"https://sentinel.openai.com/backend-api/sentinel/frame.html?sv=20260219f9f6"},
		"content-type":       {"text/plain;charset=UTF-8"},
		"accept":             {"*/*"},
		"accept-language":    {"en-US,en;q=0.9"},
		"accept-encoding":    {"gzip, deflate, br"},
		"sec-ch-ua":          {secCHUA()},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"same-site"},
		"user-agent":         {userAgent()},
	}, "origin", "referer", "content-type", "accept", "accept-language", "accept-encoding", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "user-agent")
	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", err
	}
	return data.Token, nil
}

func (e *registrationEngine) submitSignup(ctx context.Context, did, sentinelToken string) (string, error) {
	payload := fmt.Sprintf(`{"username":{"value":"%s","kind":"email"},"screen_hint":"signup"}`, e.email)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/api/accounts/authorize/continue", strings.NewReader(payload))
	req.Header = browserHeaders(http.Header{
		"referer":            {"https://auth.openai.com/create-account"},
		"origin":             {"https://auth.openai.com"},
		"accept":             {"application/json"},
		"content-type":       {"application/json"},
		"accept-language":    {"en-US,en;q=0.9"},
		"accept-encoding":    {"gzip, deflate, br"},
		"connection":         {"keep-alive"},
		"priority":           {"u=1, i"},
		"sec-ch-ua":          {secCHUA()},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"same-site"},
		"user-agent":         {userAgent()},
	}, "referer", "origin", "accept", "content-type", "accept-language", "accept-encoding", "connection", "priority", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "user-agent")
	if sentinelToken != "" {
		req.Header.Set("openai-sentinel-token", fmt.Sprintf(`{"p":"","t":"","c":"%s","id":"%s","flow":"authorize_continue"}`, sentinelToken, did))
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("signup failed: %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var response struct {
		Page struct {
			Type string `json:"type"`
		} `json:"page"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", nil
	}
	return response.Page.Type, nil
}

func (e *registrationEngine) registerPassword(ctx context.Context) (string, error) {
	password := randomPassword(e.settings.DefaultPasswordLength)
	payload := map[string]string{"password": password, "username": e.email}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/api/accounts/user/register", strings.NewReader(string(body)))
	req.Header = browserHeaders(http.Header{
		"referer":         {"https://auth.openai.com/create-account/password"},
		"accept":          {"application/json"},
		"content-type":    {"application/json"},
		"accept-language": {"en-US,en;q=0.9"},
		"user-agent":      {userAgent()},
	}, "referer", "accept", "content-type", "accept-language", "user-agent")
	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("register password failed: %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return password, nil
}

func (e *registrationEngine) sendVerificationCode(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://auth.openai.com/api/accounts/email-otp/send", nil)
	req.Header = browserHeaders(http.Header{
		"referer":         {"https://auth.openai.com/create-account/password"},
		"accept":          {"application/json"},
		"accept-language": {"en-US,en;q=0.9"},
		"user-agent":      {userAgent()},
	}, "referer", "accept", "accept-language", "user-agent")
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send otp failed: %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (e *registrationEngine) validateVerificationCode(ctx context.Context, code string) error {
	body := fmt.Sprintf(`{"code":"%s"}`, code)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/api/accounts/email-otp/validate", strings.NewReader(body))
	req.Header = browserHeaders(http.Header{
		"referer":         {"https://auth.openai.com/email-verification"},
		"accept":          {"application/json"},
		"content-type":    {"application/json"},
		"accept-language": {"en-US,en;q=0.9"},
		"user-agent":      {userAgent()},
	}, "referer", "accept", "content-type", "accept-language", "user-agent")
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("validate otp failed: %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (e *registrationEngine) createUserAccount(ctx context.Context) error {
	payload := map[string]string{
		"name":      randomName(),
		"birthdate": randomBirthdate(),
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/api/accounts/create_account", strings.NewReader(string(body)))
	req.Header = browserHeaders(http.Header{
		"referer":         {"https://auth.openai.com/about-you"},
		"accept":          {"application/json"},
		"content-type":    {"application/json"},
		"accept-language": {"en-US,en;q=0.9"},
		"user-agent":      {userAgent()},
	}, "referer", "accept", "content-type", "accept-language", "user-agent")
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create account failed: %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (e *registrationEngine) getWorkspaceID() (string, error) {
	raw := e.getCookieValue("https://auth.openai.com", "oai-client-auth-session")
	if raw == "" {
		raw = e.getCookieValue("https://chatgpt.com", "oai-client-auth-session")
	}
	if raw == "" {
		return "", fmt.Errorf("missing oai-client-auth-session cookie")
	}

	segments := strings.Split(raw, ".")
	if len(segments) == 0 {
		return "", fmt.Errorf("invalid auth session cookie")
	}

	candidates := []string{segments[0]}
	// 兼容 JWT（header.payload.signature）：workspace 通常在 payload（segments[1]）里。
	if len(segments) >= 2 {
		candidates = append(candidates, segments[1])
	}

	var lastDecodeErr error
	for _, candidate := range candidates {
		payload, err := decodeBase64URLJSON(candidate)
		if err != nil {
			lastDecodeErr = err
			continue
		}
		if workspaceID, ok := extractWorkspaceIDFromSessionPayload(payload); ok {
			return workspaceID, nil
		}
	}

	if lastDecodeErr != nil {
		return "", lastDecodeErr
	}
	return "", fmt.Errorf("workspace information not found")
}

func extractWorkspaceIDFromSessionPayload(payload map[string]any) (string, bool) {
	if payload == nil {
		return "", false
	}
	if id := strings.TrimSpace(asString(payload["workspace_id"])); id != "" {
		return id, true
	}
	workspaces, ok := payload["workspaces"].([]any)
	if !ok || len(workspaces) == 0 {
		return "", false
	}
	firstWorkspace, ok := workspaces[0].(map[string]any)
	if !ok {
		return "", false
	}
	workspaceID := strings.TrimSpace(asString(firstWorkspace["id"]))
	if workspaceID == "" {
		return "", false
	}
	return workspaceID, true
}

func (e *registrationEngine) selectWorkspace(ctx context.Context, workspaceID string) (string, error) {
	body := fmt.Sprintf(`{"workspace_id":"%s"}`, workspaceID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/api/accounts/workspace/select", strings.NewReader(body))
	req.Header = browserHeaders(http.Header{
		"referer":         {"https://auth.openai.com/sign-in-with-chatgpt/codex/consent"},
		"content-type":    {"application/json"},
		"accept":          {"application/json"},
		"accept-language": {"en-US,en;q=0.9"},
		"user-agent":      {userAgent()},
	}, "referer", "content-type", "accept", "accept-language", "user-agent")
	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("select workspace failed: %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var payload struct {
		ContinueURL string `json:"continue_url"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.ContinueURL) == "" {
		return "", fmt.Errorf("continue_url missing")
	}
	return payload.ContinueURL, nil
}

func (e *registrationEngine) followRedirects(ctx context.Context, startURL string) (string, error) {
	e.client.SetFollowRedirect(false)
	defer e.client.SetFollowRedirect(true)

	currentURL := startURL
	for redirect := 0; redirect < 6; redirect++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		req.Header = browserHeaders(http.Header{
			"accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"},
			"accept-language": {"en-US,en;q=0.9"},
			"user-agent":      {userAgent()},
		}, "accept", "accept-language", "user-agent")
		resp, err := e.client.Do(req)
		if err != nil {
			return "", err
		}
		resp.Body.Close()
		location := resp.Header.Get("Location")
		if resp.StatusCode < 300 || resp.StatusCode >= 400 || location == "" {
			break
		}
		nextURL, err := url.Parse(currentURL)
		if err != nil {
			return "", err
		}
		resolved, err := nextURL.Parse(location)
		if err != nil {
			return "", err
		}
		if strings.Contains(resolved.String(), "code=") && strings.Contains(resolved.String(), "state=") {
			return resolved.String(), nil
		}
		currentURL = resolved.String()
	}
	return "", fmt.Errorf("callback url not found in redirect chain")
}

func (e *registrationEngine) exchangeCallback(ctx context.Context, callbackURL string) (tokenInfo, error) {
	parsedCallback, err := url.Parse(callbackURL)
	if err != nil {
		return tokenInfo{}, err
	}
	query := parsedCallback.Query()
	code := strings.TrimSpace(query.Get("code"))
	state := strings.TrimSpace(query.Get("state"))
	if code == "" || state == "" {
		return tokenInfo{}, fmt.Errorf("callback missing code/state")
	}
	if state != e.oauthStart.State {
		return tokenInfo{}, fmt.Errorf("oauth state mismatch")
	}

	form := url.Values{
		"grant_type":    []string{"authorization_code"},
		"client_id":     []string{e.settings.OpenAIClientID},
		"code":          []string{code},
		"redirect_uri":  []string{e.settings.OpenAIRedirectURI},
		"code_verifier": []string{e.oauthStart.CodeVerifier},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, e.settings.OpenAITokenURL, strings.NewReader(form.Encode()))
	req.Header = browserHeaders(http.Header{
		"content-type": {"application/x-www-form-urlencoded"},
		"accept":       {"application/json"},
		"user-agent":   {userAgent()},
	}, "content-type", "accept", "user-agent")

	tokenClient, err := newTokenClient(e.proxyURL)
	if err != nil {
		return tokenInfo{}, err
	}

	resp, err := tokenClient.Do(req)
	if err != nil {
		return tokenInfo{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return tokenInfo{}, fmt.Errorf("oauth token exchange failed: %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return tokenInfo{}, err
	}

	claims, err := parseJWTClaims(payload.IDToken)
	if err != nil {
		return tokenInfo{}, nil
	}
	authClaims, _ := claims["https://api.openai.com/auth"].(map[string]any)

	return tokenInfo{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		IDToken:      payload.IDToken,
		AccountID:    asString(authClaims["chatgpt_account_id"]),
	}, nil
}

func (e *registrationEngine) getCookieValue(rawURL, name string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	for _, cookie := range e.client.GetCookies(parsedURL) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func loadEngineSettings(ctx context.Context, db *store.SQLiteStore) (engineSettings, error) {
	clientID, err := db.GetSettingString(ctx, "openai.client_id", defaultOpenAIClientID)
	if err != nil {
		return engineSettings{}, err
	}
	authURL, err := db.GetSettingString(ctx, "openai.auth_url", defaultOpenAIAuthURL)
	if err != nil {
		return engineSettings{}, err
	}
	tokenURL, err := db.GetSettingString(ctx, "openai.token_url", defaultOpenAITokenURL)
	if err != nil {
		return engineSettings{}, err
	}
	redirectURI, err := db.GetSettingString(ctx, "openai.redirect_uri", defaultOpenAIRedirectURI)
	if err != nil {
		return engineSettings{}, err
	}
	scope, err := db.GetSettingString(ctx, "openai.scope", defaultOpenAIScope)
	if err != nil {
		return engineSettings{}, err
	}
	passwordLength, err := db.GetSettingInt(ctx, "registration.default_password_length", 12)
	if err != nil {
		return engineSettings{}, err
	}
	emailCodeTimeout, err := db.GetSettingInt(ctx, "email_code.timeout", 120)
	if err != nil {
		return engineSettings{}, err
	}

	return engineSettings{
		OpenAIClientID:        clientID,
		OpenAIAuthURL:         authURL,
		OpenAITokenURL:        tokenURL,
		OpenAIRedirectURI:     redirectURI,
		OpenAIScope:           scope,
		DefaultPasswordLength: passwordLength,
		EmailCodeTimeout:      emailCodeTimeout,
	}, nil
}

func startOAuth(settings engineSettings) oauthStart {
	state := randomURLSafeToken(16)
	codeVerifier := randomURLSafeToken(64)
	codeChallenge := sha256Base64URL(codeVerifier)
	query := url.Values{
		"client_id":                  []string{settings.OpenAIClientID},
		"response_type":              []string{"code"},
		"redirect_uri":               []string{settings.OpenAIRedirectURI},
		"scope":                      []string{settings.OpenAIScope},
		"state":                      []string{state},
		"code_challenge":             []string{codeChallenge},
		"code_challenge_method":      []string{"S256"},
		"prompt":                     []string{"login"},
		"id_token_add_organizations": []string{"true"},
		"codex_cli_simplified_flow":  []string{"true"},
	}
	return oauthStart{
		AuthURL:      settings.OpenAIAuthURL + "?" + query.Encode(),
		State:        state,
		CodeVerifier: codeVerifier,
	}
}

func randomPassword(length int) string {
	if length < 8 {
		length = 12
	}
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	builder := strings.Builder{}
	for i := 0; i < length; i++ {
		builder.WriteByte(charset[mrand.IntN(len(charset))])
	}
	return builder.String()
}

func randomName() string {
	names := []string{"James", "Emma", "Olivia", "Liam", "Ava", "Noah", "Mia", "Lucas", "Grace", "Nora"}
	return names[mrand.IntN(len(names))]
}

func newBrowserClient(proxyURL string) (tls_client.HttpClient, error) {
	return newTLSClient(proxyURL, true)
}

func newTokenClient(proxyURL string) (tls_client.HttpClient, error) {
	return newTLSClient(proxyURL, false)
}

func newTLSClient(proxyURL string, withJar bool) (tls_client.HttpClient, error) {
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_144),
		tls_client.WithRandomTLSExtensionOrder(),
	}
	if withJar {
		options = append(options, tls_client.WithCookieJar(tls_client.NewCookieJar()))
	}
	if strings.TrimSpace(proxyURL) != "" {
		options = append(options, tls_client.WithProxyUrl(strings.TrimSpace(proxyURL)))
	}
	return tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
}

func browserHeaders(header http.Header, order ...string) http.Header {
	header[http.HeaderOrderKey] = order
	header[http.PHeaderOrderKey] = []string{":method", ":authority", ":scheme", ":path"}
	return header
}

func authPageHeaders() http.Header {
	return browserHeaders(http.Header{
		"accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"},
		"accept-language":           {"en-US,en;q=0.9"},
		"accept-encoding":           {"gzip, deflate, br"},
		"cache-control":             {"max-age=0"},
		"sec-ch-ua":                 {secCHUA()},
		"sec-ch-ua-mobile":          {"?0"},
		"sec-ch-ua-platform":        {`"Windows"`},
		"sec-fetch-dest":            {"document"},
		"sec-fetch-mode":            {"navigate"},
		"sec-fetch-site":            {"none"},
		"sec-fetch-user":            {"?1"},
		"upgrade-insecure-requests": {"1"},
		"user-agent":                {userAgent()},
	}, "accept", "accept-language", "accept-encoding", "cache-control", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "sec-fetch-user", "upgrade-insecure-requests", "user-agent")
}

func secCHUA() string {
	return `"Google Chrome";v="144", "Chromium";v="144", "Not.A/Brand";v="24"`
}

func randomURLSafeToken(nbytes int) string {
	if nbytes <= 0 {
		nbytes = 16
	}
	buf := make([]byte, nbytes)
	if _, err := crand.Read(buf); err != nil {
		return fmt.Sprintf("%d%d", time.Now().UnixNano(), mrand.IntN(100000))
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func parseJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return map[string]any{}, fmt.Errorf("invalid jwt")
	}
	return decodeBase64URLJSON(parts[1])
}

func decodeBase64URLJSON(raw string) (map[string]any, error) {
	padding := strings.Repeat("=", (4-len(raw)%4)%4)
	decoded, err := base64.URLEncoding.DecodeString(raw + padding)
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	if err := json.Unmarshal(decoded, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
