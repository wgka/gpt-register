package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	httpf "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
)

type dynamicProxyResponse struct {
	Final []struct {
		IP       string `json:"ip"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"final"`
}

type dynamicProxyError struct {
	Msg     string `json:"msg"`
	Message string `json:"message"`
	Error   string `json:"error"`
	Status  string `json:"status"`
	Code    int    `json:"code"`
}

type proxyCandidate struct {
	URL      string
	Scheme   string
	HostPort string
}

func ResolveProxy(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}

	if apiURL := firstEnvValue("APP_PROXY_API_URL", "PROXY_API_URL"); apiURL != "" {
		if directProxy := directProxyURL(apiURL); directProxy != "" {
			return directProxy
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		if candidate, err := fetchDynamicProxy(ctx, apiURL); err == nil {
			return candidate.URL
		}
	}

	return firstEnvValue("APP_PROXY_URL", "PROXY_URL")
}

func ResolveRegistrationProxy(ctx context.Context, value string, settings engineSettings, logf func(string)) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}

	apiURL := firstEnvValue("APP_PROXY_API_URL", "PROXY_API_URL")
	if directProxy := directProxyURL(apiURL); directProxy != "" {
		return directProxy
	}
	if apiURL == "" {
		return firstEnvValue("APP_PROXY_URL", "PROXY_URL")
	}

	attempts := envInt("APP_PROXY_ATTEMPTS", 4)
	if attempts < 1 {
		attempts = 1
	}
	preflightTimeoutSeconds := envInt("APP_PROXY_PREFLIGHT_TIMEOUT", 12)
	if preflightTimeoutSeconds < 3 {
		preflightTimeoutSeconds = 3
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		candidate, err := fetchDynamicProxy(attemptCtx, apiURL)
		cancel()
		if err != nil {
			if logf != nil {
				logf(fmt.Sprintf("获取动态代理失败 (%d/%d): %v", attempt, attempts, err))
			}
			continue
		}
		if logf != nil {
			logf(fmt.Sprintf("动态代理候选 (%d/%d): %s %s", attempt, attempts, candidate.Scheme, candidate.HostPort))
		}

		checkCtx, cancel := context.WithTimeout(ctx, time.Duration(preflightTimeoutSeconds)*time.Second)
		err = validateRegistrationProxy(checkCtx, candidate.URL, settings)
		cancel()
		if err == nil {
			if logf != nil && attempt > 1 {
				logf(fmt.Sprintf("动态代理预检通过 (%d/%d)", attempt, attempts))
			}
			return candidate.URL
		}

		if logf != nil {
			logf(fmt.Sprintf("动态代理预检失败 (%d/%d): %s %s -> %v", attempt, attempts, candidate.Scheme, candidate.HostPort, err))
		}
	}

	fallback := firstEnvValue("APP_PROXY_URL", "PROXY_URL")
	if fallback != "" && logf != nil {
		logf("动态代理预检全部失败，回退到固定代理")
	}
	return fallback
}

func fetchDynamicProxy(ctx context.Context, apiURL string) (proxyCandidate, error) {
	client, err := newProxyAPIClient()
	if err != nil {
		return proxyCandidate{}, err
	}

	req, err := httpf.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(apiURL), nil)
	if err != nil {
		return proxyCandidate{}, err
	}
	req.Header = browserHeaders(httpf.Header{
		"accept":             {"application/json, text/plain, */*"},
		"accept-language":    {"zh-CN,zh;q=0.9,en;q=0.8"},
		"cache-control":      {"no-cache"},
		"pragma":             {"no-cache"},
		"sec-ch-ua":          {secCHUA()},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"cross-site"},
		"user-agent":         {userAgent()},
	}, "accept", "accept-language", "cache-control", "pragma", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "user-agent")

	resp, err := client.Do(req)
	if err != nil {
		return proxyCandidate{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return proxyCandidate{}, fmt.Errorf("dynamic proxy api http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return proxyCandidate{}, err
	}

	var payload dynamicProxyResponse
	if err := jsonUnmarshalResponse(body, &payload); err != nil {
		return proxyCandidate{}, err
	}
	if len(payload.Final) == 0 {
		var details dynamicProxyError
		if err := json.Unmarshal(body, &details); err == nil {
			message := firstNonEmpty(details.Message, details.Msg, details.Error, details.Status)
			if message != "" {
				if details.Code != 0 {
					return proxyCandidate{}, fmt.Errorf("dynamic proxy api returned empty result: code=%d msg=%s", details.Code, message)
				}
				return proxyCandidate{}, fmt.Errorf("dynamic proxy api returned empty result: %s", message)
			}
		}
		return proxyCandidate{}, fmt.Errorf("dynamic proxy api returned empty result: %s", strings.TrimSpace(string(body)))
	}

	item := payload.Final[0]
	if strings.TrimSpace(item.IP) == "" || item.Port <= 0 {
		return proxyCandidate{}, fmt.Errorf("dynamic proxy api returned invalid proxy")
	}

	scheme := detectProxyScheme(apiURL)
	auth := ""
	if strings.TrimSpace(item.Username) != "" || strings.TrimSpace(item.Password) != "" {
		auth = item.Username + ":" + item.Password + "@"
	}

	hostPort := item.IP + ":" + strconv.Itoa(item.Port)
	return proxyCandidate{
		URL:      scheme + "://" + auth + hostPort,
		Scheme:   scheme,
		HostPort: hostPort,
	}, nil
}

func newProxyAPIClient() (tls_client.HttpClient, error) {
	return newTLSClient("", false, nil)
}

func validateRegistrationProxy(ctx context.Context, proxyURL string, settings engineSettings) error {
	client, err := newBrowserClient(proxyURL, nil)
	if err != nil {
		return err
	}

	authURL := startOAuth(settings).AuthURL
	req, err := httpf.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		return err
	}
	req.Header = authPageHeaders()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth preflight http %d", resp.StatusCode)
	}

	if strings.Contains(strings.ToLower(string(body)), "just a moment") {
		return fmt.Errorf("cloudflare challenge")
	}

	parsedURL, _ := url.Parse("https://auth.openai.com")
	for _, cookie := range client.GetCookies(parsedURL) {
		if cookie.Name == "oai-did" && strings.TrimSpace(cookie.Value) != "" {
			return nil
		}
	}

	return fmt.Errorf("missing oai-did cookie")
}

func firstEnvValue(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func detectProxyScheme(apiURL string) string {
	parsedURL, err := url.Parse(strings.TrimSpace(apiURL))
	if err != nil {
		return "socks5"
	}

	proxyType := strings.ToLower(strings.TrimSpace(parsedURL.Query().Get("proxyType")))
	switch proxyType {
	case "http", "https":
		return proxyType
	case "socks5", "socks5h", "socks":
		return "socks5"
	default:
		return "socks5"
	}
}

func directProxyURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parsedURL, err := url.Parse(trimmed)
	if err != nil || strings.TrimSpace(parsedURL.Host) == "" {
		return ""
	}

	switch strings.ToLower(strings.TrimSpace(parsedURL.Scheme)) {
	case "http", "https", "socks5", "socks5h", "socks":
	default:
		return ""
	}

	if parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return ""
	}
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		return ""
	}

	if parsedURL.Scheme == "socks" {
		parsedURL.Scheme = "socks5"
	}
	parsedURL.Path = ""

	return parsedURL.String()
}
