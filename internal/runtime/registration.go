package runtime

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	mrand "math/rand/v2"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	chatGPTWebClientID       = "app_X8zY6vW2pQ9tR3dE7nK1jL5gH"
	chatGPTWebRedirectURI    = "https://chatgpt.com/api/auth/callback/openai"
	chatGPTWebAudience       = "https://api.openai.com/v1"
	chatGPTWebScope          = "openid email profile offline_access model.request model.read organization.read organization.write"
	otpPattern               = `\b(\d{6})\b`
)

var (
	checkoutSessionPattern = regexp.MustCompile(`/checkout/openai_llc/([A-Za-z0-9_-]+)`)
	authHTMLWorkspaceKeyRe = regexp.MustCompile(`"(?:workspace_id|organization_id|account_id|chatgpt_account_id|default_workspace_id)"\s*:\s*"([^"\\]+)"`)
	nextDataScriptRe       = regexp.MustCompile(`(?s)<script[^>]*\bid=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`)
)

var errPhoneVerificationRequired = errors.New("phone verification required")

// workspaceUUIDRe matches a standard UUID (8-4-4-4-12 hex) which is the
// format OpenAI uses for selectable workspace / organization IDs.
var workspaceUUIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isSelectableWorkspaceID returns true only when id looks like a real UUID
// that the /workspace/select endpoint will accept.
func isSelectableWorkspaceID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	lower := strings.ToLower(id)
	switch lower {
	case "default", "personal", "none", "null":
		return false
	}
	if strings.HasPrefix(lower, "ua-") {
		return false
	}
	return workspaceUUIDRe.MatchString(id)
}

func pickSelectableWorkspace(id string) (string, bool) {
	id = strings.TrimSpace(id)
	if !isSelectableWorkspaceID(id) {
		return "", false
	}
	return id, true
}

// isHexHyphensUUID returns true if s is exactly 36 characters composed only of
// hex digits and hyphens – a relaxed UUID check that ignores group lengths.
func isHexHyphensUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-') {
			return false
		}
	}
	return true
}

// pickIDFromAccountField is a relaxed version of pickSelectableWorkspace used
// exclusively when reading account_id from the accounts/check response.
// It accepts standard UUIDs as well as 36-char hex-and-hyphen strings that
// may not match the strict 8-4-4-4-12 pattern.
func pickIDFromAccountField(id string) (string, bool) {
	id = strings.TrimSpace(id)
	if s, ok := pickSelectableWorkspace(id); ok {
		return s, true
	}
	if id == "" {
		return "", false
	}
	lower := strings.ToLower(id)
	switch lower {
	case "default", "personal", "none", "null":
		return "", false
	}
	if strings.HasPrefix(lower, "ua-") {
		return "", false
	}
	if workspaceUUIDRe.MatchString(id) || isHexHyphensUUID(id) {
		return id, true
	}
	return "", false
}

// pickRelaxedAccountSelectID accepts anything that pickSelectableWorkspace
// accepts plus OpenAI org-ID format ("org-…") which personal accounts may
// use as their workspace / account identifier.
func pickRelaxedAccountSelectID(id string) (string, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", false
	}
	if s, ok := pickSelectableWorkspace(id); ok {
		return s, true
	}
	if isHexHyphensUUID(id) {
		return id, true
	}
	lower := strings.ToLower(id)
	if strings.HasPrefix(lower, "org-") && len(id) > 4 {
		return id, true
	}
	return "", false
}

// acceptExtractedWorkspaceID normalizes IDs returned from extractors; strict UUID first, then relaxed (hex36, org-).
func acceptExtractedWorkspaceID(id string) (string, bool) {
	if sid, ok := pickSelectableWorkspace(id); ok {
		return sid, true
	}
	if sid, ok := pickRelaxedAccountSelectID(id); ok {
		return sid, true
	}
	if sid, ok := pickIDFromAccountField(id); ok {
		return sid, true
	}
	return "", false
}

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
	settings          engineSettings
	service           EmailService
	proxyURL          string
	client            tls_client.HttpClient
	logf              func(string)
	email             string
	password          string
	emailInfo         EmailInfo
	oauthStart        oauthStart
	isExisting        bool
	cachedWorkspaceID string
	deviceID          string
	csrfToken         string
	authSessionID     string
	authorizeURL      string
	oauthState        string
	redirectMu        sync.Mutex
	redirectChain     []string
	lastRedirectURL   string
}

func newRegistrationEngine(settings engineSettings, service EmailService, proxyURL string, logf func(string)) *registrationEngine {
	engine := &registrationEngine{
		settings: settings,
		service:  service,
		proxyURL: proxyURL,
		logf:     logf,
		deviceID: generateDeviceIDUUID(),
	}
	client, err := newBrowserClient(proxyURL, engine.recordRedirect)
	if err != nil {
		return engine
	}
	engine.client = client
	return engine
}

func (e *registrationEngine) run(ctx context.Context) RegistrationResult {
	result := RegistrationResult{Success: false, Source: "register"}
	if e.client == nil {
		result.ErrorMessage = "browser session initialization failed"
		return result
	}

	e.logf("检查代理出口地区")
	if ok, location := e.checkIPLocation(ctx); !ok {
		result.ErrorMessage = location
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

	e.logf("预热 ChatGPT 首页会话")
	if err := e.bootstrapChatGPT(ctx); err != nil {
		e.logf("首页预热失败，继续当前授权链路: " + err.Error())
	} else {
		e.logf("首页预热完成")
	}

	e.authSessionID = generateDeviceIDUUID()
	e.logf("ChatGPT signin 已初始化")

	if err := e.initiateChatGPTSignin(ctx); err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("已获取 authorize_url")

	if err := e.followChatGPTAuthorize(ctx); err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("Auth 会话已建立")

	e.logf("提交密码")
	password, pageType, err := e.registerPassword(ctx)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.password = password
	result.Password = password
	if strings.TrimSpace(pageType) == "email_otp_verification" {
		e.isExisting = true
		result.Source = "login"
		e.logf("注册接口返回 email_otp_verification，按已存在账号流程继续")
	}
	e.logf("密码步骤完成")

	e.logf("请求发送邮箱验证码")
	if err := e.sendVerificationCode(ctx); err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("验证码已发送")

	e.logf("等待邮箱验证码")
	code, err := e.service.GetVerificationCode(ctx, e.email, e.emailInfo.ServiceID, time.Duration(e.settings.EmailCodeTimeout)*time.Second, otpPattern)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("邮箱验证码: " + code)

	e.logf("校验邮箱验证码")
	continueURL, err := e.validateVerificationCode(ctx, code)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	e.logf("验证码校验通过")
	if trimmed := strings.TrimSpace(continueURL); trimmed != "" {
		e.logf("OTP 已返回授权继续地址")
	}

	if !e.isExisting {
		e.logf("创建 OpenAI 账户资料")
		accountContinueURL, err := e.createUserAccount(ctx)
		if err != nil {
			result.ErrorMessage = err.Error()
			return result
		}
		if trimmed := strings.TrimSpace(accountContinueURL); trimmed != "" {
			continueURL = trimmed
			e.logf("账户资料步骤已返回授权继续地址")
		}
		e.logf("账户资料创建完成")
	}

	var (
		callbackURL string
		workspaceID string
	)

	if strings.TrimSpace(continueURL) != "" {
		e.logf("沿 continue_url 完成授权")
		callbackURL, workspaceID, err = e.completeAuthorization(ctx, continueURL)
		if err != nil {
			if errors.Is(err, errPhoneVerificationRequired) {
				result.ErrorMessage = err.Error()
				return result
			}
			e.logf("continue_url 授权链路失败，尝试兼容回退: " + err.Error())
		}
	}

	if strings.TrimSpace(callbackURL) == "" {
		e.logf("continue_url 未直接产出回调，尝试 ChatGPT oauth2/auth 回退")
		callbackURL, err = e.followRedirects(ctx, e.buildChatGPTWebOAuthURL())
		if err != nil {
			e.logf("ChatGPT oauth2/auth 回退失败: " + err.Error())
		}
	}
	if strings.TrimSpace(callbackURL) != "" {
		e.logf("收到 OAuth 回调")
	} else {
		e.logf("未捕获 OAuth 回调，准备尝试 ChatGPT Session 回退")
	}
	result.WorkspaceID = workspaceID

	var (
		finalTokenInfo      tokenInfo
		sessionTokenInfo    tokenInfo
		sessionWorkspaceID  string
		usedSessionFallback bool
		sessionSyncErr      error
	)

	isChatGPTWebCallback := strings.TrimSpace(callbackURL) != "" && isOAuthCallbackURL(callbackURL, chatGPTWebRedirectURI)
	if isChatGPTWebCallback && !e.hasChatGPTSessionToken() {
		e.logf("访问 ChatGPT callback 以建立 session")
		if err := e.visitChatGPTWebCallback(ctx, callbackURL); err != nil {
			e.logf("访问 ChatGPT callback 失败: " + err.Error())
		}
	}

	if e.hasChatGPTSessionToken() || strings.TrimSpace(callbackURL) == "" || isChatGPTWebCallback {
		e.logf("同步 ChatGPT Session")
		if isChatGPTWebCallback {
			sessionTokenInfo, sessionWorkspaceID, sessionSyncErr = e.fetchChatGPTSessionTokenInfoWithRetry(ctx, 3, 400*time.Millisecond)
		} else {
			sessionTokenInfo, sessionWorkspaceID, sessionSyncErr = e.fetchChatGPTSessionTokenInfo(ctx)
		}
		if sessionSyncErr != nil {
			e.logf("ChatGPT Session 同步失败: " + sessionSyncErr.Error())
		} else {
			if result.WorkspaceID == "" && sessionWorkspaceID != "" {
				result.WorkspaceID = sessionWorkspaceID
			}
			e.logf("ChatGPT Session 已同步")
		}
	}

	if strings.TrimSpace(callbackURL) != "" && !isChatGPTWebCallback {
		e.logf("交换 access_token / refresh_token")
		finalTokenInfo, err = e.exchangeCallback(ctx, callbackURL)
		if err != nil {
			e.logf("Token 交换失败，尝试使用 ChatGPT Session 回退: " + err.Error())
			if strings.TrimSpace(sessionTokenInfo.AccessToken) == "" {
				sessionTokenInfo, sessionWorkspaceID, _ = e.fetchChatGPTSessionTokenInfo(ctx)
				if result.WorkspaceID == "" && sessionWorkspaceID != "" {
					result.WorkspaceID = sessionWorkspaceID
				}
			}
			if strings.TrimSpace(sessionTokenInfo.AccessToken) == "" {
				result.ErrorMessage = err.Error()
				return result
			}
			finalTokenInfo = sessionTokenInfo
			usedSessionFallback = true
			e.logf("已切换为 ChatGPT Session access_token 回退")
		} else {
			e.logf("Token 交换完成")
		}
	} else {
		if strings.TrimSpace(sessionTokenInfo.AccessToken) == "" {
			if isChatGPTWebCallback && sessionSyncErr != nil {
				result.ErrorMessage = "chatgpt web callback reached, but session sync failed: " + sessionSyncErr.Error()
			} else {
				result.ErrorMessage = "chatgpt web callback reached, but accessToken is still unavailable"
			}
			return result
		}
		finalTokenInfo = sessionTokenInfo
		usedSessionFallback = true
		e.logf("使用 ChatGPT Session access_token 完成注册")
	}
	if strings.TrimSpace(finalTokenInfo.AccountID) == "" && strings.TrimSpace(sessionTokenInfo.AccountID) != "" {
		finalTokenInfo.AccountID = sessionTokenInfo.AccountID
	}
	if result.WorkspaceID == "" && sessionWorkspaceID != "" {
		result.WorkspaceID = sessionWorkspaceID
	}

	result.Success = true
	result.AccountID = finalTokenInfo.AccountID
	result.AccessToken = finalTokenInfo.AccessToken
	result.RefreshToken = finalTokenInfo.RefreshToken
	result.IDToken = finalTokenInfo.IDToken
	result.Password = e.password
	result.SessionToken = e.getCookieValue("https://chatgpt.com", "__Secure-next-auth.session-token")

	e.logf("生成绑卡链接")
	bindCardURL, err := GenerateBindCardLink(ctx, finalTokenInfo.AccessToken, e.proxyURL)
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
		"session_fallback":    usedSessionFallback,
	}
	if result.BindCardURL != "" {
		result.Metadata["bind_card_url"] = result.BindCardURL
	}
	e.logf("注册流程完成")

	return result
}

func (e *registrationEngine) checkIPLocation(ctx context.Context) (bool, string) {
	if parseEnvBool(firstEnvValue("APP_SKIP_IP_CHECK", "SKIP_IP_CHECK")) {
		e.logf("已启用跳过 IP 地区校验")
		return true, "SKIPPED"
	}

	const maxRetries = 3
	var body []byte
	var statusCode int
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return false, "ip check cancelled: " + ctx.Err().Error()
		}

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
			lastErr = err
			errMsg := err.Error()
			isTransient := strings.Contains(errMsg, "EOF") ||
				strings.Contains(errMsg, "connection reset") ||
				strings.Contains(errMsg, "timeout")
			if isTransient {
				e.logf(fmt.Sprintf("IP 检测重试 (%d/%d): %s", attempt, maxRetries, errMsg))
				if attempt < maxRetries {
					select {
					case <-ctx.Done():
						return false, "ip check cancelled: " + ctx.Err().Error()
					case <-time.After(400 * time.Millisecond):
					}
				}
				continue
			}
			e.logf("IP 检测请求失败: " + errMsg)
			return false, "ip check failed: " + errMsg
		}

		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		statusCode = resp.StatusCode
		lastErr = nil

		if statusCode == 502 || statusCode == 503 {
			e.logf(fmt.Sprintf("IP 检测重试 (%d/%d): HTTP %d", attempt, maxRetries, statusCode))
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return false, "ip check cancelled: " + ctx.Err().Error()
				case <-time.After(400 * time.Millisecond):
				}
			}
			continue
		}

		break
	}

	if lastErr != nil {
		e.logf("IP 检测请求失败: " + lastErr.Error())
		return false, "ip check failed: " + lastErr.Error()
	}
	if statusCode != http.StatusOK {
		snippet := bodySnippet(body, 200)
		e.logf(fmt.Sprintf("IP 检测返回异常状态码 %d: %s", statusCode, snippet))
		return false, fmt.Sprintf("ip check failed: HTTP %d", statusCode)
	}
	match := regexp.MustCompile(`loc=([A-Z]+)`).FindStringSubmatch(string(body))
	if len(match) < 2 {
		snippet := bodySnippet(body, 200)
		e.logf("IP 检测响应中未找到 loc= 字段: " + snippet)
		return true, ""
	}
	location := match[1]
	switch location {
	case "CN", "HK", "MO", "TW":
		e.logf("代理出口地区受限: " + location)
		return false, "unsupported ip location: " + location
	default:
		return true, location
	}
}

func (e *registrationEngine) getDeviceID(ctx context.Context) (string, error) {
	const maxRetries = 3
	var lastErr error
	e.syncDeviceCookies()
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}

		e.resetRedirectTracking()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, e.oauthStart.AuthURL, nil)
		req.Header = authPageHeaders()

		resp, err := e.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("初始化授权失败: %w", err)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		finalURL := ""
		if resp.Request != nil && resp.Request.URL != nil {
			finalURL = resp.Request.URL.String()
		}
		if strings.TrimSpace(finalURL) == "" {
			_, finalURL = e.redirectSnapshot()
		}
		if strings.TrimSpace(finalURL) == "" {
			lastErr = fmt.Errorf("初始化授权失败: 未捕获授权跳转")
			continue
		}
		if resp.StatusCode == http.StatusForbidden {
			lastErr = fmt.Errorf("初始化授权失败: 被限流 (403)")
			continue
		}
		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("初始化授权失败: HTTP %d", resp.StatusCode)
			continue
		}
		e.captureDeviceIDFromCookies()
		e.syncDeviceCookies()
		return e.deviceID, nil
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("初始化授权失败")
}

func (e *registrationEngine) checkSentinel(ctx context.Context) (string, error) {
	return e.getSentinelToken(ctx, "authorize_continue")
}

func (e *registrationEngine) getSentinelToken(ctx context.Context, flow string) (string, error) {
	proof := generateSentinelProofP(e.deviceID, userAgent())

	requestBody := map[string]any{
		"p":    proof,
		"id":   e.deviceID,
		"flow": flow,
	}

	respData, err := e.requestSentinelToken(ctx, requestBody)
	if err != nil {
		return "", err
	}

	var token string
	if pow, ok := respData["proofofwork"].(map[string]any); ok {
		required, _ := pow["required"].(bool)
		seed := strings.TrimSpace(asString(pow["seed"]))
		difficulty := strings.TrimSpace(asString(pow["difficulty"]))
		if required && seed != "" && difficulty != "" {
			proof = generateProofPWithSeedPoW(seed, difficulty, e.deviceID, userAgent())
			requestBody["p"] = proof
			respData, err = e.requestSentinelToken(ctx, requestBody)
			if err != nil {
				return "", err
			}
		}
	}
	if rawToken, ok := respData["token"].(string); ok {
		token = rawToken
	}

	tokenData := map[string]any{
		"p":    proof,
		"t":    nil,
		"c":    token,
		"id":   e.deviceID,
		"flow": flow,
	}
	tokenBytes, _ := json.Marshal(tokenData)
	return string(tokenBytes), nil
}

func (e *registrationEngine) requestSentinelToken(ctx context.Context, requestBody map[string]any) (map[string]any, error) {
	bodyBytes, _ := json.Marshal(requestBody)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://sentinel.openai.com/backend-api/sentinel/req", strings.NewReader(string(bodyBytes)))
	req.Header = browserHeaders(http.Header{
		"origin":             {"https://sentinel.openai.com"},
		"referer":            {"https://sentinel.openai.com/backend-api/sentinel/frame.html"},
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
		return nil, err
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return map[string]any{}, nil
	}

	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (e *registrationEngine) submitSignup(ctx context.Context, sentinelToken string) (string, error) {
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
		req.Header.Set("openai-sentinel-token", sentinelToken)
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
	if err := jsonUnmarshalResponse(body, &response); err != nil {
		return "", nil
	}
	return response.Page.Type, nil
}

func (e *registrationEngine) registerPassword(ctx context.Context) (string, string, error) {
	password := randomPassword(e.settings.DefaultPasswordLength)
	payload := map[string]string{"password": password, "username": e.email}
	body, _ := json.Marshal(payload)
	sentinelToken, err := e.getSentinelToken(ctx, "oauth_create_account")
	if err != nil {
		return "", "", fmt.Errorf("get register sentinel token failed: %w", err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/api/accounts/user/register", strings.NewReader(string(body)))
	req.Header = browserHeaders(http.Header{
		"referer":            {"https://auth.openai.com/create-account/password"},
		"origin":             {"https://auth.openai.com"},
		"accept":             {"application/json"},
		"content-type":       {"application/json"},
		"accept-language":    {"en-US,en;q=0.9"},
		"accept-encoding":    {"gzip, deflate, br"},
		"priority":           {"u=1, i"},
		"sec-ch-ua":          {secCHUA()},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"same-origin"},
		"user-agent":         {userAgent()},
	}, "referer", "origin", "accept", "content-type", "accept-language", "accept-encoding", "priority", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "user-agent")
	req.Header.Set("openai-sentinel-token", sentinelToken)
	resp, err := e.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("register password failed: %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var response struct {
		Page struct {
			Type string `json:"type"`
		} `json:"page"`
	}
	if err := jsonUnmarshalResponse(respBody, &response); err != nil {
		return password, "", nil
	}
	return password, strings.TrimSpace(response.Page.Type), nil
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
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send otp failed: %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (e *registrationEngine) validateVerificationCode(ctx context.Context, code string) (string, error) {
	body := fmt.Sprintf(`{"code":"%s"}`, code)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/api/accounts/email-otp/validate", strings.NewReader(body))
	req.Header = browserHeaders(http.Header{
		"referer":            {"https://auth.openai.com/email-verification"},
		"origin":             {"https://auth.openai.com"},
		"accept":             {"application/json"},
		"content-type":       {"application/json"},
		"accept-language":    {"en-US,en;q=0.9"},
		"accept-encoding":    {"gzip, deflate, br"},
		"priority":           {"u=1, i"},
		"sec-ch-ua":          {secCHUA()},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"same-origin"},
		"user-agent":         {userAgent()},
	}, "referer", "origin", "accept", "content-type", "accept-language", "accept-encoding", "priority", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "user-agent")
	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("validate otp failed: %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	e.tryExtractWorkspaceFromBody(respBody)
	return extractContinueURLFromBody(respBody), nil
}

func (e *registrationEngine) createUserAccount(ctx context.Context) (string, error) {
	payload := map[string]string{
		"name":      randomName(),
		"birthdate": randomBirthdate(),
	}
	body, _ := json.Marshal(payload)
	sentinelToken, err := e.getSentinelToken(ctx, "oauth_create_account")
	if err != nil {
		return "", fmt.Errorf("get create_account sentinel token failed: %w", err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/api/accounts/create_account", strings.NewReader(string(body)))
	req.Header = browserHeaders(http.Header{
		"referer":            {"https://auth.openai.com/about-you"},
		"origin":             {"https://auth.openai.com"},
		"accept":             {"application/json"},
		"content-type":       {"application/json"},
		"accept-language":    {"en-US,en;q=0.9"},
		"accept-encoding":    {"gzip, deflate, br"},
		"priority":           {"u=1, i"},
		"sec-ch-ua":          {secCHUA()},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"same-origin"},
		"user-agent":         {userAgent()},
	}, "referer", "origin", "accept", "content-type", "accept-language", "accept-encoding", "priority", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "user-agent")
	req.Header.Set("openai-sentinel-token", sentinelToken)
	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create account failed: %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	e.tryExtractWorkspaceFromBody(respBody)
	return extractContinueURLFromBody(respBody), nil
}

func (e *registrationEngine) tryExtractWorkspaceFromBody(body []byte) {
	if e.cachedWorkspaceID != "" {
		return
	}
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return
	}
	if id, ok := extractWorkspaceIDFromSessionPayload(m); ok {
		e.cacheWorkspaceID(id)
	}
}

func (e *registrationEngine) cacheWorkspaceID(id string) {
	if sid, ok := acceptExtractedWorkspaceID(id); ok {
		e.cachedWorkspaceID = sid
	}
}

func (e *registrationEngine) recordRedirect(rawURL string) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return
	}
	e.redirectMu.Lock()
	e.lastRedirectURL = trimmed
	e.redirectChain = append(e.redirectChain, trimmed)
	e.redirectMu.Unlock()
}

func (e *registrationEngine) resetRedirectTracking() {
	e.redirectMu.Lock()
	e.redirectChain = nil
	e.lastRedirectURL = ""
	e.redirectMu.Unlock()
}

func (e *registrationEngine) redirectSnapshot() ([]string, string) {
	e.redirectMu.Lock()
	defer e.redirectMu.Unlock()
	chain := append([]string(nil), e.redirectChain...)
	return chain, e.lastRedirectURL
}

func (e *registrationEngine) findCallbackFromRedirectTracking() (string, bool) {
	chain, last := e.redirectSnapshot()
	for _, rawURL := range chain {
		if e.isKnownOAuthCallbackURL(rawURL) {
			return rawURL, true
		}
	}
	if e.isKnownOAuthCallbackURL(last) {
		return last, true
	}
	return "", false
}

func extractContinueURLFromBody(body []byte) string {
	var raw any
	if json.Unmarshal(body, &raw) == nil {
		if next := findStringFieldDeep(raw, 0, "continue_url", "continueUrl"); strings.TrimSpace(next) != "" {
			return strings.TrimSpace(next)
		}
	}
	return ""
}

func findStringFieldDeep(v any, depth int, keys ...string) string {
	if depth > 6 {
		return ""
	}

	switch typed := v.(type) {
	case map[string]any:
		for _, wanted := range keys {
			for actual, child := range typed {
				if strings.EqualFold(actual, wanted) {
					if s := strings.TrimSpace(asString(child)); s != "" {
						return s
					}
				}
			}
		}
		for _, child := range typed {
			if s := findStringFieldDeep(child, depth+1, keys...); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range typed {
			if s := findStringFieldDeep(child, depth+1, keys...); s != "" {
				return s
			}
		}
	}

	return ""
}

func resolveAbsoluteURL(rawURL, baseURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.IsAbs() {
		return parsed.String()
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://auth.openai.com"
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return trimmed
	}
	resolved, err := base.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	return resolved.String()
}

func isOAuthCallbackURL(rawURL, redirectURI string) bool {
	if strings.TrimSpace(rawURL) == "" {
		return false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	query := parsed.Query()
	if strings.TrimSpace(query.Get("code")) == "" || strings.TrimSpace(query.Get("state")) == "" {
		return false
	}
	redirectURI = strings.TrimSpace(redirectURI)
	if redirectURI == "" {
		return true
	}
	expected, err := url.Parse(redirectURI)
	if err != nil {
		return true
	}
	return parsed.Scheme == expected.Scheme && parsed.Host == expected.Host && parsed.Path == expected.Path
}

func (e *registrationEngine) isKnownOAuthCallbackURL(rawURL string) bool {
	return isOAuthCallbackURL(rawURL, e.settings.OpenAIRedirectURI) || isOAuthCallbackURL(rawURL, chatGPTWebRedirectURI)
}

func isConsentContinueURL(rawURL string) bool {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	return strings.Contains(lower, "/consent") || strings.Contains(lower, "sign-in-with-chatgpt")
}

func isAddPhoneContinueURL(rawURL string) bool {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	return strings.Contains(lower, "/add-phone")
}

func (e *registrationEngine) buildConsentOAuthURL() string {
	codeChallenge := sha256Base64URL(e.oauthStart.CodeVerifier)
	query := url.Values{
		"client_id":                  []string{e.settings.OpenAIClientID},
		"code_challenge":             []string{codeChallenge},
		"code_challenge_method":      []string{"S256"},
		"redirect_uri":               []string{e.settings.OpenAIRedirectURI},
		"response_type":              []string{"code"},
		"scope":                      []string{e.settings.OpenAIScope},
		"state":                      []string{e.oauthStart.State},
		"id_token_add_organizations": []string{"true"},
		"codex_cli_simplified_flow":  []string{"true"},
		"originator":                 []string{"codex_cli_rs"},
		"prompt":                     []string{"login"},
	}
	return "https://auth.openai.com/api/oauth/oauth2/auth?" + query.Encode()
}

func extractWorkspaceFromAuthHTML(html string) (string, bool) {
	if m := nextDataScriptRe.FindStringSubmatch(html); len(m) > 1 {
		inner := strings.TrimSpace(m[1])
		var nd map[string]any
		if json.Unmarshal([]byte(inner), &nd) == nil {
			if id := findWorkspaceUUIDDeep(nd, 0); id != "" {
				if sid, ok := acceptExtractedWorkspaceID(id); ok {
					return sid, true
				}
			}
		}
		var raw any
		if json.Unmarshal([]byte(inner), &raw) == nil {
			if id := extractWorkspaceIDFromJSONValue(raw, 0); id != "" {
				if sid, ok := acceptExtractedWorkspaceID(id); ok {
					return sid, true
				}
			}
		}
	}

	for _, sub := range authHTMLWorkspaceKeyRe.FindAllStringSubmatch(html, -1) {
		if len(sub) > 1 {
			if sid, ok := acceptExtractedWorkspaceID(strings.TrimSpace(sub[1])); ok {
				return sid, true
			}
		}
	}

	return "", false
}

func (e *registrationEngine) loadAuthHTML(ctx context.Context, pageURL string) (string, error) {
	resolved := resolveAbsoluteURL(pageURL, "https://auth.openai.com")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved, nil)
	if err != nil {
		return "", err
	}
	req.Header = authPageHeaders()
	req.Header.Set("referer", "https://auth.openai.com/")
	req.Header.Set("sec-fetch-site", "same-origin")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth HTML %s -> HTTP %d: %s", resolved, resp.StatusCode, bodySnippet(body, 160))
	}
	return string(body), nil
}

func (e *registrationEngine) completeAuthorization(ctx context.Context, continueURL string) (string, string, error) {
	resolved := resolveAbsoluteURL(continueURL, "https://auth.openai.com")
	if resolved == "" {
		return "", "", fmt.Errorf("continue_url missing")
	}
	e.logf("授权继续地址: " + resolved)
	if e.isKnownOAuthCallbackURL(resolved) {
		return resolved, "", nil
	}
	if isAddPhoneContinueURL(resolved) {
		return "", "", fmt.Errorf("%w: %s", errPhoneVerificationRequired, resolved)
	}
	if isConsentContinueURL(resolved) {
		return e.completeConsentAuthorization(ctx, resolved)
	}
	callbackURL, err := e.followRedirects(ctx, resolved)
	return callbackURL, "", err
}

func (e *registrationEngine) completeConsentAuthorization(ctx context.Context, consentURL string) (string, string, error) {
	html, err := e.loadAuthHTML(ctx, consentURL)
	if err != nil {
		return "", "", fmt.Errorf("访问 consent 页面失败: %w", err)
	}

	var workspaceID string
	if sid, ok := extractWorkspaceFromAuthHTML(html); ok {
		workspaceID = sid
		e.cacheWorkspaceID(sid)
		e.logf("从 consent 页面解析到 workspace_id")
	}

	if workspaceID == "" {
		workspaceID, err = e.getWorkspaceID()
		if err != nil {
			e.logf("Cookie/缓存未找到 workspace，尝试 API 回退: " + err.Error())
			workspaceID, err = e.fetchWorkspaceIDFromAPI(ctx)
			if err != nil {
				return "", "", err
			}
		}
	}

	nextURL, err := e.selectWorkspace(ctx, workspaceID, consentURL)
	if err != nil {
		return "", workspaceID, err
	}
	if strings.Contains(nextURL, "login_verifier") {
		e.logf("workspace/select 返回 login_verifier")
		callbackURL, err := e.followRedirects(ctx, resolveAbsoluteURL(nextURL, consentURL))
		return callbackURL, workspaceID, err
	}
	if strings.TrimSpace(nextURL) == "" {
		e.logf("workspace/select 未返回 continue_url，改走 oauth2/auth")
	} else {
		e.logf("workspace/select 返回非 login_verifier continue_url，改走 oauth2/auth")
	}
	nextURL = e.buildConsentOAuthURL()
	callbackURL, err := e.followRedirects(ctx, resolveAbsoluteURL(nextURL, consentURL))
	return callbackURL, workspaceID, err
}

func (e *registrationEngine) getWorkspaceID() (string, error) {
	raw := e.getCookieValue("https://auth.openai.com", "oai-client-auth-session")
	if raw == "" {
		raw = e.getCookieValue("https://chatgpt.com", "oai-client-auth-session")
	}
	if raw == "" {
		if sid, ok := acceptExtractedWorkspaceID(e.cachedWorkspaceID); ok {
			e.logf("使用账号流程 API 缓存的 workspace_id")
			return sid, nil
		}
		return "", fmt.Errorf("missing oai-client-auth-session cookie")
	}

	segments := strings.Split(raw, ".")
	if len(segments) == 0 {
		return "", fmt.Errorf("invalid auth session cookie")
	}

	candidates := []string{segments[0]}
	if len(segments) >= 2 {
		candidates = append(candidates, segments[1])
	}

	var lastDecodeErr error
	parsedJSON := false
	for _, candidate := range candidates {
		payload, err := decodeBase64URLJSON(candidate)
		if err != nil {
			lastDecodeErr = err
			continue
		}
		parsedJSON = true
		if workspaceID, ok := extractWorkspaceIDFromSessionPayload(payload); ok {
			if sid, ok2 := acceptExtractedWorkspaceID(workspaceID); ok2 {
				return sid, nil
			}
		}
	}

	if sid, ok := acceptExtractedWorkspaceID(e.cachedWorkspaceID); ok {
		e.logf("使用账号流程 API 缓存的 workspace_id")
		return sid, nil
	}

	if parsedJSON {
		if len(segments) >= 5 {
			return "", fmt.Errorf("session cookie appears to be JWE-encrypted (segments=%d); workspace_id not extractable from encrypted token", len(segments))
		}
		return "", fmt.Errorf("session cookie parsed as JWT (segments=%d) but no workspace_id found in known fields", len(segments))
	}
	if lastDecodeErr != nil {
		return "", lastDecodeErr
	}
	return "", fmt.Errorf("workspace information not found")
}

func (e *registrationEngine) workspaceAPIGET(ctx context.Context, endpoint, referer, origin, secFetchSite string) ([]byte, int, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header = browserHeaders(http.Header{
		"referer":            {referer},
		"origin":             {origin},
		"accept":             {"application/json"},
		"oai-language":       {"zh-CN"},
		"accept-language":    {"en-US,en;q=0.9"},
		"accept-encoding":    {"gzip, deflate, br"},
		"user-agent":         {userAgent()},
		"sec-ch-ua":          {secCHUA()},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {secFetchSite},
	}, "referer", "origin", "accept", "oai-language", "accept-language", "accept-encoding", "user-agent", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return body, resp.StatusCode, nil
}

// refreshChatGPTSession re-fetches NextAuth session so cookie jar picks up server-set updates before workspace resolution.
func (e *registrationEngine) refreshChatGPTSession(ctx context.Context) error {
	_, code, err := e.workspaceAPIGET(ctx, "https://chatgpt.com/api/auth/session", "https://chatgpt.com/", "https://chatgpt.com", "same-origin")
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("session refresh HTTP %d", code)
	}
	return nil
}

// tryExtractWorkspaceFromAuthHTML loads auth.openai.com HTML pages (consent / OAuth
// authorize) where Next.js embeds workspace/org IDs in __NEXT_DATA__ or inline JSON
// when JSON APIs omit account_id (accounts/check returns null).
func (e *registrationEngine) tryExtractWorkspaceFromAuthHTML(ctx context.Context) (string, bool) {
	candidates := []string{
		"https://auth.openai.com/sign-in-with-chatgpt/codex/consent",
	}
	if strings.TrimSpace(e.oauthStart.AuthURL) != "" {
		candidates = append(candidates, e.oauthStart.AuthURL)
	}

	for _, pageURL := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if err != nil {
			continue
		}
		req.Header = authPageHeaders()
		req.Header.Set("referer", "https://auth.openai.com/")
		req.Header.Set("sec-fetch-site", "same-origin")

		resp, err := e.client.Do(req)
		if err != nil {
			e.logf("auth HTML GET 失败 " + pageURL + ": " + err.Error())
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			e.logf(fmt.Sprintf("auth HTML %s -> HTTP %d", pageURL, resp.StatusCode))
			continue
		}

		if sid, ok := extractWorkspaceFromAuthHTML(string(body)); ok {
			return sid, true
		}
	}
	return "", false
}

// workspaceAPIEndpoint describes a single API endpoint used to retrieve workspace IDs.
type workspaceAPIEndpoint struct {
	url, referer, origin, secFetchSite string
	extractor                          func([]byte) (string, bool)
}

// tryExtractWorkspaceFromEndpoint runs the dedicated extractor and generic
// JSON fallbacks on a successful API response body, using acceptExtractedWorkspaceID
// for final ID validation.
func (e *registrationEngine) tryExtractWorkspaceFromEndpoint(ep workspaceAPIEndpoint, body []byte) (string, bool) {
	if ep.extractor != nil {
		if id, ok := ep.extractor(body); ok {
			if sid, ok2 := acceptExtractedWorkspaceID(id); ok2 {
				return sid, true
			}
		}
	}

	var arr []any
	if json.Unmarshal(body, &arr) == nil && len(arr) > 0 {
		if first, ok := arr[0].(map[string]any); ok {
			for _, key := range []string{"id", "workspace_id", "workspaceId"} {
				if id := strings.TrimSpace(asString(first[key])); id != "" {
					if sid, ok2 := acceptExtractedWorkspaceID(id); ok2 {
						return sid, true
					}
				}
			}
			if id, ok := extractWorkspaceIDFromSessionPayload(first); ok {
				if sid, ok2 := acceptExtractedWorkspaceID(id); ok2 {
					return sid, true
				}
			}
		}
	}

	var obj map[string]any
	if json.Unmarshal(body, &obj) == nil {
		if id, ok := extractWorkspaceIDFromSessionPayload(obj); ok {
			if sid, ok2 := acceptExtractedWorkspaceID(id); ok2 {
				return sid, true
			}
		}
	}

	var raw any
	if json.Unmarshal(body, &raw) == nil {
		if id := extractWorkspaceIDFromJSONValue(raw, 0); id != "" {
			if sid, ok := acceptExtractedWorkspaceID(id); ok {
				return sid, true
			}
		}
	}

	return "", false
}

// fetchWorkspaceIDFromAPI tries to obtain the workspace ID by calling
// multiple API endpoints with the current browser session cookies.
func (e *registrationEngine) fetchWorkspaceIDFromAPI(ctx context.Context) (string, error) {
	if sid, ok := e.tryExtractWorkspaceFromAuthHTML(ctx); ok {
		e.cacheWorkspaceID(sid)
		e.logf("从 auth.openai.com 网页解析到 workspace_id")
		return sid, nil
	}

	endpoints := []workspaceAPIEndpoint{
		{
			"https://chatgpt.com/api/auth/session",
			"https://chatgpt.com/", "https://chatgpt.com", "same-origin",
			extractWorkspaceFromChatGPTSession,
		},
		{
			"https://chatgpt.com/backend-api/me",
			"https://chatgpt.com/", "https://chatgpt.com", "same-origin",
			extractWorkspaceFromBackendMe,
		},
		{
			buildAccountsCheckURL(),
			"https://chatgpt.com/", "https://chatgpt.com", "same-origin",
			extractWorkspaceFromAccountsCheck,
		},
	}

	for _, ep := range endpoints {
		isAccountsCheck := strings.Contains(ep.url, "accounts/check")
		maxTries := 1
		if isAccountsCheck {
			maxTries = 15
		}

		var lastBody []byte
		for try := 1; try <= maxTries; try++ {
			if try > 1 {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(800 * time.Millisecond):
				}
			}

			body, code, err := e.workspaceAPIGET(ctx, ep.url, ep.referer, ep.origin, ep.secFetchSite)
			if err != nil {
				e.logf("workspace API " + ep.url + " 请求失败: " + err.Error())
				break
			}
			if code != http.StatusOK {
				e.logf(fmt.Sprintf("workspace API %s -> HTTP %d: %s", ep.url, code, bodySnippet(body, 120)))
				break
			}

			lastBody = body

			if sid, ok := e.tryExtractWorkspaceFromEndpoint(ep, body); ok {
				e.cacheWorkspaceID(sid)
				return sid, nil
			}

			if isAccountsCheck && try < maxTries {
				e.logf(fmt.Sprintf("accounts/check 尝试 %d/%d 未解析到 workspace_id，800ms 后重试", try, maxTries))
			}
		}

		if lastBody == nil {
			continue
		}

		if isAccountsCheck {
			var top map[string]any
			if json.Unmarshal(lastBody, &top) == nil {
				if accounts, ok := top["accounts"].(map[string]any); ok {
					for bucket, v := range accounts {
						bkt, ok := v.(map[string]any)
						if !ok {
							continue
						}
						inner, ok := bkt["account"].(map[string]any)
						if !ok {
							e.logf(fmt.Sprintf("accounts/check bucket %s: 无 account 子对象", bucket))
							continue
						}
						keys := make([]string, 0, len(inner))
						for k := range inner {
							keys = append(keys, k)
						}
						sort.Strings(keys)
						line := strings.Join(keys, ", ")
						if len(line) > 500 {
							line = line[:500] + "..."
						}
						e.logf(fmt.Sprintf("accounts/check bucket %s account 字段(无值): %s", bucket, line))
						e.logf(fmt.Sprintf("accounts/check bucket %s account_id 诊断: 类型=%T", bucket, inner["account_id"]))
					}
				}
			}
		}

		e.logf(fmt.Sprintf("workspace API %s -> HTTP 200 但未提取到 workspace_id: %s", ep.url, bodySnippet(lastBody, 120)))
	}
	return "", fmt.Errorf("workspace API fallback: no workspace found (accounts/check 的 account_id 常为 null；已尝试 HTML/JSON API，详见任务日志)")
}

// workspaceDirectKeys lists all top-level key variants that may hold a workspace ID.
var workspaceDirectKeys = []string{
	"workspace_id",
	"workspaceId",
	"default_workspace_id",
	"defaultWorkspaceId",
	"organization_id",
	"organizationId",
	"chatgpt_account_id",
	"account_id",
	"default_account_id",
}

// extractWorkspaceFromChatGPTSession parses the chatgpt.com/api/auth/session
// response, which may contain a WARNING_BANNER string and an embedded JWT
// accessToken whose claims hold chatgpt_account_id.
func extractWorkspaceFromChatGPTSession(body []byte) (string, bool) {
	var raw map[string]any
	if json.Unmarshal(body, &raw) != nil {
		return "", false
	}

	// Try JWT tokens embedded in the session response (accessToken, then idToken).
	for _, tokenKey := range []string{"accessToken", "idToken", "id_token"} {
		tok, ok := raw[tokenKey].(string)
		if !ok || !strings.Contains(tok, ".") {
			continue
		}
		claims, err := parseJWTClaims(tok)
		if err != nil {
			continue
		}
		if authClaims, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
			for _, key := range []string{"chatgpt_account_id", "workspace_id", "organization_id", "account_id"} {
				if id := strings.TrimSpace(asString(authClaims[key])); id != "" {
					if sid, ok2 := acceptExtractedWorkspaceID(id); ok2 {
						return sid, true
					}
				}
			}
		}
		if id, ok := extractWorkspaceIDFromSessionPayload(claims); ok {
			if sid, ok2 := acceptExtractedWorkspaceID(id); ok2 {
				return sid, true
			}
		}
	}

	// Walk nested objects (skip scalar WARNING_BANNER / expires).
	for key, val := range raw {
		if key == "WARNING_BANNER" || key == "accessToken" || key == "idToken" || key == "id_token" || key == "expires" {
			continue
		}
		if sub, ok := val.(map[string]any); ok {
			if id, ok := extractWorkspaceIDFromSessionPayload(sub); ok {
				if sid, ok2 := acceptExtractedWorkspaceID(id); ok2 {
					return sid, true
				}
			}
			for _, inner := range []string{"account", "workspace", "organization", "user"} {
				if nested, ok := sub[inner].(map[string]any); ok {
					for _, k := range []string{"id", "workspace_id", "account_id", "organization_id"} {
						if id := strings.TrimSpace(asString(nested[k])); id != "" {
							if sid, ok2 := acceptExtractedWorkspaceID(id); ok2 {
								return sid, true
							}
						}
					}
				}
			}
		}
	}

	if id, ok := extractWorkspaceIDFromSessionPayload(raw); ok {
		if sid, ok2 := acceptExtractedWorkspaceID(id); ok2 {
			return sid, true
		}
	}

	if id := findWorkspaceUUIDDeep(raw, 0); id != "" {
		return id, true
	}

	return "", false
}

// extractWorkspaceFromBackendMe parses chatgpt.com/backend-api/me which
// returns user info with an orgs.data[] array of organizations.
func extractWorkspaceFromBackendMe(body []byte) (string, bool) {
	var raw map[string]any
	if json.Unmarshal(body, &raw) != nil {
		return "", false
	}

	// Primary: orgs.data[].{id, organization_id, workspace_id}
	if orgs, ok := raw["orgs"].(map[string]any); ok {
		if data, ok := orgs["data"].([]any); ok {
			for _, item := range data {
				if org, ok := item.(map[string]any); ok {
					for _, key := range []string{"id", "organization_id", "workspace_id"} {
						if id := strings.TrimSpace(asString(org[key])); isSelectableWorkspaceID(id) {
							return id, true
						}
					}
				}
			}
		}
	}

	// Secondary: scan known top-level keys that might hold a workspace UUID.
	for _, key := range []string{"accounts", "workspace", "organization"} {
		switch v := raw[key].(type) {
		case map[string]any:
			for _, k := range []string{"id", "workspace_id", "organization_id", "account_id"} {
				if id := strings.TrimSpace(asString(v[k])); isSelectableWorkspaceID(id) {
					return id, true
				}
			}
		case []any:
			for _, item := range v {
				if obj, ok := item.(map[string]any); ok {
					for _, k := range []string{"id", "workspace_id", "organization_id"} {
						if id := strings.TrimSpace(asString(obj[k])); isSelectableWorkspaceID(id) {
							return id, true
						}
					}
				}
			}
		}
	}

	if id := findWorkspaceUUIDDeep(raw, 0); id != "" {
		return id, true
	}

	return "", false
}

func extractAccountIDFromJSONValue(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return pickIDFromAccountField(strings.TrimSpace(t))
	case map[string]any:
		for _, nk := range []string{"id", "workspace_id", "account_id", "organization_id", "value"} {
			if s, ok := pickIDFromAccountField(asString(t[nk])); ok {
				return s, true
			}
			if s, ok := pickRelaxedAccountSelectID(asString(t[nk])); ok {
				return s, true
			}
		}
	}
	return "", false
}

// extractWorkspaceFromAccountsCheck parses the
// chatgpt.com/backend-api/accounts/check/v4-2023-04-27 response whose
// shape is {"accounts":{"<account_id>":{"account":{...},...},...}}.
func extractWorkspaceFromAccountsCheck(body []byte) (string, bool) {
	var raw map[string]any
	if json.Unmarshal(body, &raw) != nil {
		return "", false
	}

	if accounts, ok := raw["accounts"].(map[string]any); ok {
		for _, accountData := range accounts {
			ad, ok := accountData.(map[string]any)
			if !ok {
				continue
			}
			if account, ok := ad["account"].(map[string]any); ok {
				if id, ok := extractAccountIDFromJSONValue(account["account_id"]); ok {
					return id, true
				}
				if id, ok := pickIDFromAccountField(asString(account["account_id"])); ok {
					return id, true
				}
				for _, key := range []string{"organization_id", "workspace_id"} {
					if id := strings.TrimSpace(asString(account[key])); id != "" {
						if sid, ok := acceptExtractedWorkspaceID(id); ok {
							return sid, true
						}
					}
				}
			}
			if id, ok := extractAccountIDFromJSONValue(ad["account_id"]); ok {
				return id, true
			}
			if id, ok := pickIDFromAccountField(asString(ad["account_id"])); ok {
				return id, true
			}
			for _, key := range []string{"organization_id", "workspace_id"} {
				if id := strings.TrimSpace(asString(ad[key])); id != "" {
					if sid, ok := acceptExtractedWorkspaceID(id); ok {
						return sid, true
					}
				}
			}
			for _, sibKey := range []string{"workspace", "organization", "selected_workspace", "default_workspace"} {
				if sub, ok := ad[sibKey].(map[string]any); ok {
					if id := findWorkspaceUUIDDeep(sub, 0); id != "" {
						return id, true
					}
					if sid, ok2 := acceptExtractedWorkspaceID(strings.TrimSpace(asString(sub["id"]))); ok2 {
						return sid, true
					}
				}
			}
			for _, scalarKey := range []string{"default_workspace_id", "selected_workspace_id", "primary_workspace_id", "chatgpt_account_id", "struct_id"} {
				if sid, ok := acceptExtractedWorkspaceID(strings.TrimSpace(asString(ad[scalarKey]))); ok {
					return sid, true
				}
			}
			// Deep scan account sub-map then the wrapper ad map.
			if account, ok := ad["account"].(map[string]any); ok {
				if id := findWorkspaceUUIDDeep(account, 0); id != "" {
					return id, true
				}
			}
			if id := findWorkspaceUUIDDeep(ad, 0); id != "" {
				return id, true
			}
		}
		// Map keys themselves may be UUIDs — only use if selectable.
		for accountID := range accounts {
			if isSelectableWorkspaceID(accountID) {
				return accountID, true
			}
		}
	}

	if id, ok := extractWorkspaceIDFromSessionPayload(raw); ok {
		if sid, ok2 := acceptExtractedWorkspaceID(id); ok2 {
			return sid, true
		}
	}
	return "", false
}

func extractWorkspaceIDFromSessionPayload(payload map[string]any) (string, bool) {
	if payload == nil {
		return "", false
	}

	for _, key := range workspaceDirectKeys {
		if id := strings.TrimSpace(asString(payload[key])); id != "" {
			return id, true
		}
	}

	if id := extractWorkspaceIDFromSlice(payload["workspaces"]); id != "" {
		return id, true
	}

	for _, parent := range []string{"user", "session", "account", "auth", "data", "profile"} {
		if sub, ok := payload[parent].(map[string]any); ok {
			for _, key := range workspaceDirectKeys {
				if id := strings.TrimSpace(asString(sub[key])); id != "" {
					return id, true
				}
			}
			if id := extractWorkspaceIDFromSlice(sub["workspaces"]); id != "" {
				return id, true
			}
		}
	}

	if authClaims, ok := payload["https://api.openai.com/auth"].(map[string]any); ok {
		for _, key := range workspaceDirectKeys {
			if id := strings.TrimSpace(asString(authClaims[key])); id != "" {
				return id, true
			}
		}
	}

	if id := findWorkspaceIDRecursive(payload, 0); id != "" {
		return id, true
	}

	return "", false
}

func extractWorkspaceIDFromSlice(v any) string {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"id", "workspace_id", "workspaceId", "organization_id", "account_id"} {
			if id, ok := acceptExtractedWorkspaceID(strings.TrimSpace(asString(obj[key]))); ok {
				return id
			}
		}
	}
	return ""
}

// extractWorkspaceIDFromJSONValue recursively walks an arbitrary JSON value
// (maps and slices, depth-limited to 6) calling extractWorkspaceIDFromSessionPayload
// on every map it encounters.
func extractWorkspaceIDFromJSONValue(v any, depth int) string {
	if depth > 6 {
		return ""
	}
	switch val := v.(type) {
	case map[string]any:
		if id, ok := extractWorkspaceIDFromSessionPayload(val); ok {
			return id
		}
		for _, child := range val {
			if id := extractWorkspaceIDFromJSONValue(child, depth+1); id != "" {
				return id
			}
		}
	case []any:
		for _, item := range val {
			if id := extractWorkspaceIDFromJSONValue(item, depth+1); id != "" {
				return id
			}
		}
	}
	return ""
}

// findWorkspaceIDRecursive performs a depth-limited scan (max 2 levels) for
// string values under "id" when the parent map key contains "workspace" or
// "organization", covering JWT shapes we haven't explicitly enumerated.
func findWorkspaceIDRecursive(m map[string]any, depth int) string {
	if depth > 2 {
		return ""
	}
	for key, val := range m {
		keyLower := strings.ToLower(key)
		isWorkspaceKey := strings.Contains(keyLower, "workspace") || strings.Contains(keyLower, "organization")
		switch v := val.(type) {
		case map[string]any:
			if isWorkspaceKey {
				for _, idKey := range []string{"id", "workspace_id", "workspaceId"} {
					if id := strings.TrimSpace(asString(v[idKey])); id != "" {
						return id
					}
				}
			}
			if found := findWorkspaceIDRecursive(v, depth+1); found != "" {
				return found
			}
		case []any:
			if isWorkspaceKey {
				for _, item := range v {
					if obj, ok := item.(map[string]any); ok {
						if id := strings.TrimSpace(asString(obj["id"])); id != "" {
							return id
						}
					}
				}
			}
		}
	}
	return ""
}

// findWorkspaceUUIDDeep performs a two-phase deep scan of an arbitrary nested
// JSON map to locate a selectable workspace UUID.
//
// Phase A checks a prioritised list of well-known keys at the current level.
// Phase B iterates all keys in sorted order looking for any string value whose
// key ends with "_id" (or equals "id") that passes isSelectableWorkspaceID,
// then recurses into nested maps and slices.
func findWorkspaceUUIDDeep(m map[string]any, depth int) string {
	if m == nil || depth > 8 {
		return ""
	}

	// Phase A: priority keys.
	priorityKeys := []string{
		"workspace_id",
		"organization_id",
		"account_id",
		"primary_organization_id",
		"default_organization_id",
		"personal_workspace_id",
		"owner_organization_id",
		"org_id",
		"group_id",
		"struct_id",
		"chatgpt_account_id",
	}
	for _, pk := range priorityKeys {
		id := strings.TrimSpace(asString(m[pk]))
		if pk == "account_id" {
			if s, ok := pickRelaxedAccountSelectID(id); ok {
				return s
			}
		} else if isSelectableWorkspaceID(id) {
			return id
		}
	}

	// Phase B: sorted iteration for deterministic order.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		kLower := strings.ToLower(k)
		val := m[k]

		s := strings.TrimSpace(asString(val))
		if s != "" {
			if kLower == "account_id" {
				if id, ok := pickRelaxedAccountSelectID(s); ok {
					return id
				}
			} else if strings.HasSuffix(kLower, "_id") || kLower == "id" {
				if id, ok := acceptExtractedWorkspaceID(s); ok {
					return id
				}
			}
		}
		if sub, ok := val.(map[string]any); ok {
			if id := findWorkspaceUUIDDeep(sub, depth+1); id != "" {
				return id
			}
		}
		if arr, ok := val.([]any); ok {
			for _, elem := range arr {
				if sub, ok := elem.(map[string]any); ok {
					if id := findWorkspaceUUIDDeep(sub, depth+1); id != "" {
						return id
					}
				}
			}
		}
	}

	return ""
}

func (e *registrationEngine) selectWorkspace(ctx context.Context, workspaceID, referer string) (string, error) {
	if trimmed := strings.TrimSpace(referer); trimmed != "" {
		referer = trimmed
	} else {
		referer = "https://auth.openai.com/sign-in-with-chatgpt/codex/consent"
	}
	body := fmt.Sprintf(`{"workspace_id":"%s"}`, workspaceID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.openai.com/api/accounts/workspace/select", strings.NewReader(body))
	req.Header = browserHeaders(http.Header{
		"referer":         {referer},
		"origin":          {"https://auth.openai.com"},
		"content-type":    {"application/json"},
		"accept":          {"application/json"},
		"accept-language": {"en-US,en;q=0.9"},
		"user-agent":      {userAgent()},
	}, "referer", "origin", "content-type", "accept", "accept-language", "user-agent")
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
	if err := jsonUnmarshalResponse(respBody, &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.ContinueURL), nil
}

func (e *registrationEngine) followRedirects(ctx context.Context, startURL string) (string, error) {
	currentURL := resolveAbsoluteURL(startURL, "https://auth.openai.com")
	if e.isKnownOAuthCallbackURL(currentURL) {
		return currentURL, nil
	}
	e.resetRedirectTracking()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
	req.Header = browserHeaders(http.Header{
		"accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"},
		"accept-language": {"en-US,en;q=0.9"},
		"user-agent":      {userAgent()},
	}, "accept", "accept-language", "user-agent")
	resp, err := e.client.Do(req)
	if err != nil {
		if callbackURL, ok := e.findCallbackFromRedirectTracking(); ok {
			return callbackURL, nil
		}
		return "", err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if callbackURL, ok := e.findCallbackFromRedirectTracking(); ok {
		return callbackURL, nil
	}
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL := resp.Request.URL.String()
		if e.isKnownOAuthCallbackURL(finalURL) {
			return finalURL, nil
		}
	}
	chain, last := e.redirectSnapshot()
	if len(chain) > 0 {
		e.logf(fmt.Sprintf("授权重定向链: %s", strings.Join(chain, " -> ")))
	}
	if strings.TrimSpace(last) != "" {
		e.logf("授权最后跳转 URL: " + last)
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
	if err := jsonUnmarshalResponse(body, &payload); err != nil {
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

func (e *registrationEngine) initiateChatGPTSignin(ctx context.Context) error {
	if strings.TrimSpace(e.csrfToken) == "" {
		return fmt.Errorf("missing chatgpt csrf token")
	}
	e.syncDeviceCookies()

	query := url.Values{
		"prompt":                  []string{"login"},
		"ext-oai-did":             []string{e.deviceID},
		"auth_session_logging_id": []string{e.authSessionID},
		"screen_hint":             []string{"login_or_signup"},
		"login_hint":              []string{e.email},
	}
	form := url.Values{
		"callbackUrl": []string{"https://chatgpt.com/"},
		"csrfToken":   []string{e.csrfToken},
		"json":        []string{"true"},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://chatgpt.com/api/auth/signin/openai?"+query.Encode(), strings.NewReader(form.Encode()))
	req.Header = browserHeaders(http.Header{
		"accept":             {"*/*"},
		"accept-language":    {"en-US,en;q=0.9"},
		"accept-encoding":    {"gzip, deflate, br"},
		"origin":             {"https://chatgpt.com"},
		"referer":            {"https://chatgpt.com/"},
		"content-type":       {"application/x-www-form-urlencoded"},
		"sec-ch-ua":          {secCHUA()},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"same-origin"},
		"user-agent":         {userAgent()},
	}, "accept", "accept-language", "accept-encoding", "origin", "referer", "content-type", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "user-agent")

	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("chatgpt signin failed: %d: %s", resp.StatusCode, bodySnippet(body, 200))
	}
	var payload struct {
		URL string `json:"url"`
	}
	if err := jsonUnmarshalResponse(body, &payload); err != nil {
		return err
	}
	e.authorizeURL = strings.TrimSpace(payload.URL)
	if e.authorizeURL == "" {
		return fmt.Errorf("chatgpt signin response missing authorize url")
	}
	if parsed, err := url.Parse(e.authorizeURL); err == nil {
		e.oauthState = strings.TrimSpace(parsed.Query().Get("state"))
	}
	return nil
}

func (e *registrationEngine) followChatGPTAuthorize(ctx context.Context) error {
	if strings.TrimSpace(e.authorizeURL) == "" {
		return fmt.Errorf("missing authorize_url")
	}
	e.syncDeviceCookies()

	const maxRetries = 3
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, e.authorizeURL, nil)
		req.Header = browserHeaders(http.Header{
			"accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
			"accept-language": {"en-US,en;q=0.9"},
			"referer":         {"https://chatgpt.com/"},
			"user-agent":      {userAgent()},
		}, "accept", "accept-language", "referer", "user-agent")

		resp, err := e.client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusFound {
				if strings.TrimSpace(e.getCookieValue("https://auth.openai.com", "oai-client-auth-session")) != "" {
					e.captureDeviceIDFromCookies()
					e.syncDeviceCookies()
					return nil
				}
				lastErr = fmt.Errorf("authorize completed without oai-client-auth-session")
			} else {
				lastErr = fmt.Errorf("authorize failed: http %d", resp.StatusCode)
			}
		}
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("authorize failed")
}

func (e *registrationEngine) buildChatGPTWebOAuthURL() string {
	query := url.Values{
		"audience":                []string{chatGPTWebAudience},
		"auth_session_logging_id": []string{e.authSessionID},
		"client_id":               []string{chatGPTWebClientID},
		"device_id":               []string{e.deviceID},
		"ext-oai-did":             []string{e.deviceID},
		"prompt":                  []string{"login"},
		"redirect_uri":            []string{chatGPTWebRedirectURI},
		"response_type":           []string{"code"},
		"scope":                   []string{chatGPTWebScope},
	}
	if strings.TrimSpace(e.oauthState) != "" {
		query.Set("state", e.oauthState)
	}
	return "https://auth.openai.com/api/oauth/oauth2/auth?" + query.Encode()
}

func (e *registrationEngine) visitChatGPTWebCallback(ctx context.Context, callbackURL string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, callbackURL, nil)
	req.Header = browserHeaders(http.Header{
		"accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
		"accept-language": {"en-US,en;q=0.9"},
		"referer":         {"https://auth.openai.com/"},
		"user-agent":      {userAgent()},
	}, "accept", "accept-language", "referer", "user-agent")
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

func (e *registrationEngine) bootstrapChatGPT(ctx context.Context) error {
	const maxRetries = 3
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://chatgpt.com/", nil)
		if err != nil {
			return err
		}
		req.Header = chatGPTPageHeaders()

		resp, err := e.client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if token := parseCSRFCookieValue(e.getCookieValue("https://chatgpt.com", "__Host-next-auth.csrf-token")); token != "" {
					e.csrfToken = token
				}
				e.captureDeviceIDFromCookies()
				e.syncDeviceCookies()
				return nil
			}
			lastErr = fmt.Errorf("chatgpt homepage http %d", resp.StatusCode)
		}

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 400 * time.Millisecond):
			}
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("chatgpt homepage bootstrap failed")
}

func (e *registrationEngine) syncDeviceCookies() {
	if e.client == nil {
		return
	}
	deviceID := strings.TrimSpace(e.deviceID)
	if deviceID == "" {
		return
	}
	for _, rawURL := range []string{"https://chatgpt.com", "https://auth.openai.com"} {
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		e.client.SetCookies(parsedURL, []*http.Cookie{{
			Name:   "oai-did",
			Value:  deviceID,
			Path:   "/",
			Secure: true,
		}})
	}
}

func (e *registrationEngine) captureDeviceIDFromCookies() bool {
	for _, rawURL := range []string{"https://chatgpt.com", "https://auth.openai.com"} {
		if deviceID := strings.TrimSpace(e.getCookieValue(rawURL, "oai-did")); deviceID != "" {
			e.deviceID = deviceID
			return true
		}
	}
	return false
}

func (e *registrationEngine) hasChatGPTSessionToken() bool {
	return strings.TrimSpace(e.getCookieValue("https://chatgpt.com", "__Secure-next-auth.session-token")) != ""
}

func (e *registrationEngine) fetchChatGPTSessionTokenInfo(ctx context.Context) (tokenInfo, string, error) {
	body, code, err := e.workspaceAPIGET(ctx, "https://chatgpt.com/api/auth/session", "https://chatgpt.com/", "https://chatgpt.com", "same-origin")
	if err != nil {
		return tokenInfo{}, "", err
	}
	if code != http.StatusOK {
		return tokenInfo{}, "", fmt.Errorf("chatgpt session http %d: %s", code, bodySnippet(body, 160))
	}
	return extractTokenInfoFromChatGPTSession(body)
}

func (e *registrationEngine) fetchChatGPTSessionTokenInfoWithRetry(ctx context.Context, attempts int, delay time.Duration) (tokenInfo, string, error) {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	var lastWorkspaceID string
	for i := 0; i < attempts; i++ {
		info, workspaceID, err := e.fetchChatGPTSessionTokenInfo(ctx)
		if err == nil {
			return info, workspaceID, nil
		}
		lastErr = err
		lastWorkspaceID = workspaceID
		if i < attempts-1 {
			select {
			case <-ctx.Done():
				return tokenInfo{}, lastWorkspaceID, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return tokenInfo{}, lastWorkspaceID, lastErr
}

func extractTokenInfoFromChatGPTSession(body []byte) (tokenInfo, string, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return tokenInfo{}, "", err
	}

	info := tokenInfo{
		AccessToken:  firstNonEmpty(asString(raw["accessToken"]), asString(raw["access_token"])),
		RefreshToken: firstNonEmpty(asString(raw["refreshToken"]), asString(raw["refresh_token"])),
		IDToken:      firstNonEmpty(asString(raw["idToken"]), asString(raw["id_token"])),
	}
	if strings.TrimSpace(info.AccessToken) == "" {
		return tokenInfo{}, "", fmt.Errorf("session payload missing accessToken")
	}

	if claims, err := parseJWTClaims(info.AccessToken); err == nil {
		if authClaims, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
			info.AccountID = firstNonEmpty(
				asString(authClaims["chatgpt_account_id"]),
				asString(authClaims["account_id"]),
				asString(authClaims["workspace_id"]),
				asString(authClaims["organization_id"]),
			)
		}
		if info.AccountID == "" {
			info.AccountID = firstNonEmpty(
				asString(claims["chatgpt_account_id"]),
				asString(claims["account_id"]),
				asString(claims["workspace_id"]),
				asString(claims["organization_id"]),
			)
		}
	}

	workspaceID, _ := extractWorkspaceFromChatGPTSession(body)
	return info, workspaceID, nil
}

func (e *registrationEngine) getChatRequirements(ctx context.Context) map[string]any {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://chatgpt.com/backend-anon/sentinel/chat-requirements", strings.NewReader(`{}`))
	req.Header = browserHeaders(http.Header{
		"accept":          {"*/*"},
		"accept-language": {"en-US,en;q=0.9"},
		"accept-encoding": {"gzip, deflate, br"},
		"origin":          {"https://chatgpt.com"},
		"referer":         {"https://chatgpt.com/"},
		"content-type":    {"application/json"},
		"oai-device-id":   {e.deviceID},
		"openai-sentinel-chat-requirements-token": {generateRequirementsToken(e.deviceID, userAgent())},
		"sec-ch-ua":          {secCHUA()},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"same-origin"},
		"user-agent":         {userAgent()},
	}, "accept", "accept-language", "accept-encoding", "origin", "referer", "content-type", "oai-device-id", "openai-sentinel-chat-requirements-token", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "user-agent")

	resp, err := e.client.Do(req)
	if err != nil {
		return map[string]any{}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

func (e *registrationEngine) buildRegisterSentinelToken(ctx context.Context) string {
	requirements := e.getChatRequirements(ctx)
	if pow, ok := requirements["proofofwork"].(map[string]any); ok {
		required, _ := pow["required"].(bool)
		seed := strings.TrimSpace(asString(pow["seed"]))
		difficulty := strings.TrimSpace(asString(pow["difficulty"]))
		if required && seed != "" && difficulty != "" {
			payload := map[string]any{
				"p": generateProofPWithSeedPoW(seed, difficulty, e.deviceID, userAgent()),
			}
			body, _ := json.Marshal(payload)
			return string(body)
		}
	}
	return e.createLightweightSentinelToken()
}

func (e *registrationEngine) createLightweightSentinelToken() string {
	payload := map[string]any{
		"p": generateSentinelProofP(e.deviceID, userAgent()),
	}
	body, _ := json.Marshal(payload)
	return string(body)
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
		"originator":                 []string{"codex_cli_rs"},
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

func newBrowserClient(proxyURL string, redirectRecorder func(string)) (tls_client.HttpClient, error) {
	return newTLSClient(proxyURL, true, redirectRecorder)
}

func newTokenClient(proxyURL string) (tls_client.HttpClient, error) {
	return newTLSClient(proxyURL, false, nil)
}

func newTLSClient(proxyURL string, withJar bool, redirectRecorder func(string)) (tls_client.HttpClient, error) {
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_124),
		tls_client.WithRandomTLSExtensionOrder(),
	}
	if redirectRecorder != nil {
		options = append(options, tls_client.WithCustomRedirectFunc(func(req *http.Request, via []*http.Request) error {
			if req != nil && req.URL != nil {
				redirectRecorder(req.URL.String())
			}
			return nil
		}))
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

func chatGPTPageHeaders() http.Header {
	return browserHeaders(http.Header{
		"accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
		"accept-language":           {"en-US,en;q=0.9"},
		"accept-encoding":           {"gzip, deflate, br"},
		"sec-ch-ua":                 {secCHUA()},
		"sec-ch-ua-mobile":          {"?0"},
		"sec-ch-ua-platform":        {`"Windows"`},
		"sec-fetch-dest":            {"document"},
		"sec-fetch-mode":            {"navigate"},
		"sec-fetch-site":            {"none"},
		"sec-fetch-user":            {"?1"},
		"upgrade-insecure-requests": {"1"},
		"user-agent":                {userAgent()},
	}, "accept", "accept-language", "accept-encoding", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "sec-fetch-user", "upgrade-insecure-requests", "user-agent")
}

func secCHUA() string {
	return `"Google Chrome";v="124", "Chromium";v="124", "Not.A/Brand";v="24"`
}

func generateSentinelProofP(deviceID, ua string) string {
	config := []any{
		"1920x1080",
		time.Now().Format(time.RFC1123),
		4294967296,
		0,
		ua,
		"",
		"",
		"en-US",
		"en-US,en",
		0,
		"userAgent-Mozilla/5.0",
		"addEventListener",
		"addEventListener",
		float64(time.Now().UnixMilli() % 1000000),
		deviceID,
		"",
		8,
		float64(time.Now().UnixMilli()),
	}
	jsonData, _ := json.Marshal(config)
	encoded := base64.StdEncoding.EncodeToString(jsonData)
	return "gAAAAAC" + encoded + "~S"
}

func generateRequirementsToken(deviceID, ua string) string {
	config := []any{
		"1920x1080",
		time.Now().Format(time.RFC1123),
		4294967296,
		0,
		ua,
		"",
		"",
		"en-US",
		"en-US,en",
		0,
		"userAgent-Mozilla/5.0",
		"addEventListener",
		"addEventListener",
		float64(time.Now().UnixMilli() % 1000000),
		deviceID,
		"",
		8,
		float64(time.Now().UnixMilli()),
	}
	jsonData, _ := json.Marshal(config)
	return "gAAAAAC" + base64.StdEncoding.EncodeToString(jsonData)
}

func generateProofPWithSeedPoW(seed, difficulty, deviceID, ua string) string {
	startTime := time.Now()
	config := []any{
		"1920x1080",
		time.Now().Format(time.RFC1123),
		4294967296,
		0,
		ua,
		"",
		"",
		"en-US",
		"en-US,en",
		0,
		"userAgent-Mozilla/5.0",
		"addEventListener",
		"addEventListener",
		float64(time.Now().UnixMilli() % 1000000),
		deviceID,
		"",
		8,
		float64(time.Now().UnixMilli()),
	}

	for attempt := 0; attempt < 500000; attempt++ {
		config[3] = attempt
		config[9] = int(time.Since(startTime).Milliseconds())
		jsonData, _ := json.Marshal(config)
		encoded := base64.StdEncoding.EncodeToString(jsonData)
		hashHex := fmt.Sprintf("%08x", fnv1aHashCalc(seed+encoded))
		if len(hashHex) >= len(difficulty) && hashHex[:len(difficulty)] <= difficulty {
			return "gAAAAAB" + encoded + "~S"
		}
	}

	jsonData, _ := json.Marshal(config)
	encoded := base64.StdEncoding.EncodeToString(jsonData)
	return "gAAAAAB" + encoded + "~F"
}

func fnv1aHashCalc(s string) uint32 {
	var hash uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		hash ^= uint32(s[i])
		hash *= 16777619
	}
	hash ^= hash >> 16
	hash *= 2246822507
	hash ^= hash >> 13
	hash *= 3266489909
	hash ^= hash >> 16
	return hash
}

func generateDeviceIDUUID() string {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		return randomURLSafeToken(16)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func buildAccountsCheckURL() string {
	query := url.Values{
		"timezone_offset_min": {strconv.Itoa(currentTimezoneOffsetMinutes())},
	}
	return "https://chatgpt.com/backend-api/accounts/check/v4-2023-04-27?" + query.Encode()
}

// currentTimezoneOffsetMinutes mirrors JavaScript Date#getTimezoneOffset():
// UTC - local time, in minutes. Asia/Shanghai therefore becomes -480.
func currentTimezoneOffsetMinutes() int {
	_, offsetSeconds := time.Now().Zone()
	return -(offsetSeconds / 60)
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
		snippet := string(decoded)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return nil, fmt.Errorf("%w; decoded payload: %s", err, snippet)
	}
	return result, nil
}

func bodySnippet(body []byte, maxLen int) string {
	s := strings.TrimSpace(string(body))
	if len(s) > maxLen {
		s = s[:maxLen] + "..."
	}
	return s
}

func jsonUnmarshalResponse(body []byte, v any) error {
	if err := json.Unmarshal(body, v); err != nil {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return fmt.Errorf("%w; response body: %s", err, snippet)
	}
	return nil
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return fmt.Sprint(typed)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case nil:
		return ""
	default:
		return ""
	}
}

func parseCSRFCookieValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	decoded, err := url.QueryUnescape(trimmed)
	if err != nil {
		decoded = trimmed
	}
	if token, _, ok := strings.Cut(decoded, "|"); ok {
		return strings.TrimSpace(token)
	}
	return strings.TrimSpace(decoded)
}
