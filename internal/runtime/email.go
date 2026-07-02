package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	mailparse "net/mail"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	DefaultTempMailAPIBaseURL = "http://82.158.91.228:8081"
	DefaultGmailAPIBaseURL    = "http://147.224.217.123:8083"
)

type EmailInfo struct {
	Email     string
	ServiceID string
}

type EmailService interface {
	Type() string
	CreateEmail(ctx context.Context) (EmailInfo, error)
	GetVerificationCode(ctx context.Context, email, emailID string, timeout time.Duration, pattern string) (string, error)
}

type tempMailService struct {
	baseURL  string
	proxyURL string
}

func newTempMailService(proxyURL string) EmailService {
	baseURL := strings.TrimSpace(os.Getenv("TEMP_MAIL_API_BASE_URL"))
	if baseURL == "" {
		baseURL = DefaultTempMailAPIBaseURL
	}
	return &tempMailService{
		baseURL:  strings.TrimRight(baseURL, "/"),
		proxyURL: strings.TrimSpace(proxyURL),
	}
}

func TestTempMail(ctx context.Context, proxyURL string) error {
	service := newTempMailService(proxyURL).(*tempMailService)
	emailInfo, err := service.CreateEmail(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(emailInfo.Email) == "" {
		return fmt.Errorf("temp mail api returned empty email")
	}
	return nil
}

func (s *tempMailService) Type() string { return "tempmail" }

func (s *tempMailService) CreateEmail(ctx context.Context) (EmailInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/temp-email/random", nil)
	if err != nil {
		return EmailInfo{}, err
	}

	var payload struct {
		Email  string `json:"email"`
		Domain string `json:"domain"`
		Prefix string `json:"prefix"`
	}
	if err := s.doJSON(req, &payload); err != nil {
		return EmailInfo{}, err
	}

	email := strings.TrimSpace(payload.Email)
	if email == "" {
		return EmailInfo{}, fmt.Errorf("temp mail api returned empty email")
	}
	return EmailInfo{
		Email:     email,
		ServiceID: email,
	}, nil
}

func (s *tempMailService) GetVerificationCode(ctx context.Context, email, emailID string, timeout time.Duration, pattern string) (string, error) {
	target := strings.TrimSpace(emailID)
	if target == "" {
		target = strings.TrimSpace(email)
	}
	if target == "" {
		return "", fmt.Errorf("temp email is required")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid verification code pattern: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		code, err := s.fetchVerificationCode(ctx, target, re)
		if err == nil && code != "" {
			return code, nil
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	return "", fmt.Errorf("verification code timeout for %s", target)
}

func (s *tempMailService) fetchVerificationCode(ctx context.Context, email string, re *regexp.Regexp) (string, error) {
	rawURL := s.baseURL + "/mail/temp/" + url.PathEscape(email) + "/code"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}

	var payload struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
		Message string `json:"message"`
		Subject string `json:"subject"`
	}
	if err := s.doJSON(req, &payload); err != nil {
		return "", err
	}

	if code := strings.TrimSpace(payload.Code); code != "" {
		match := re.FindStringSubmatch(code)
		if len(match) > 1 {
			return match[1], nil
		}
		match = re.FindStringSubmatch(payload.Subject)
		if len(match) > 1 {
			return match[1], nil
		}
		return code, nil
	}

	if strings.TrimSpace(payload.Message) != "" {
		return "", fmt.Errorf("%s", payload.Message)
	}
	return "", fmt.Errorf("verification code unavailable")
}

func (s *tempMailService) doJSON(req *http.Request, out any) error {
	return doEmailServiceJSON(req, out, "temp mail api")
}

type gmailMail struct {
	Subject string `json:"subject"`
	From    string `json:"from"`
	To      string `json:"to"`
	Date    string `json:"date"`
	Text    string `json:"text"`
	HTML    string `json:"html"`
	Body    string `json:"body"`
}

type gmailAliasService struct {
	baseURL   string
	accountID string
	baseEmail string
	aliasMode string
}

func newGmailAliasService(config map[string]any) EmailService {
	baseURL := strings.TrimSpace(os.Getenv("GMAIL_API_BASE_URL"))
	if baseURL == "" {
		baseURL = DefaultGmailAPIBaseURL
	}
	if value := strings.TrimSpace(asString(config["api_base_url"])); value != "" {
		baseURL = value
	}

	return &gmailAliasService{
		baseURL: strings.TrimRight(baseURL, "/"),
		accountID: firstNonEmpty(
			asString(config["account_id"]),
			os.Getenv("GMAIL_ACCOUNT_ID"),
		),
		baseEmail: firstNonEmpty(
			asString(config["email"]),
			asString(config["base_email"]),
			os.Getenv("GMAIL_BASE_EMAIL"),
		),
		aliasMode: firstNonEmpty(
			asString(config["alias_mode"]),
			os.Getenv("GMAIL_ALIAS_MODE"),
			"plus",
		),
	}
}

func TestGmailAlias(ctx context.Context) error {
	service := newGmailAliasService(nil).(*gmailAliasService)
	emailInfo, err := service.CreateEmail(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(emailInfo.Email) == "" {
		return fmt.Errorf("gmail alias api returned empty email")
	}
	return nil
}

func (s *gmailAliasService) Type() string { return "gmail_alias" }

func (s *gmailAliasService) CreateEmail(ctx context.Context) (EmailInfo, error) {
	baseEmail := strings.TrimSpace(s.baseEmail)
	if baseEmail == "" {
		account, err := s.defaultAccount(ctx)
		if err != nil {
			return EmailInfo{}, err
		}
		baseEmail = account.Email
		if strings.TrimSpace(s.accountID) == "" {
			s.accountID = account.ID
		}
	}
	if baseEmail == "" {
		return EmailInfo{}, fmt.Errorf("gmail base email is required")
	}

	rawURL := s.baseURL + "/gmail/alias/generate?" + url.Values{
		"email": {baseEmail},
		"count": {"1"},
		"mode":  {firstNonEmpty(s.aliasMode, "mixed")},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return EmailInfo{}, err
	}

	var payload struct {
		Aliases []string `json:"aliases"`
	}
	if err := s.doJSON(req, &payload); err != nil {
		return EmailInfo{}, err
	}
	if len(payload.Aliases) == 0 || strings.TrimSpace(payload.Aliases[0]) == "" {
		return EmailInfo{}, fmt.Errorf("gmail alias api returned empty aliases")
	}

	return EmailInfo{Email: strings.TrimSpace(payload.Aliases[0]), ServiceID: strings.TrimSpace(s.accountID)}, nil
}

func (s *gmailAliasService) GetVerificationCode(ctx context.Context, email, emailID string, timeout time.Duration, pattern string) (string, error) {
	accountID := firstNonEmpty(emailID, s.accountID)
	if accountID == "" {
		account, err := s.defaultAccount(ctx)
		if err != nil {
			return "", err
		}
		accountID = account.ID
	}
	if accountID == "" {
		return "", fmt.Errorf("gmail account id is required")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid verification code pattern: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		code, err := s.fetchVerificationCode(ctx, accountID, email, re)
		if err == nil && code != "" {
			return code, nil
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	return "", fmt.Errorf("verification code timeout for %s", strings.TrimSpace(email))
}

func (s *gmailAliasService) fetchVerificationCode(ctx context.Context, accountID, alias string, re *regexp.Regexp) (string, error) {
	values := url.Values{"limit": {"10"}}
	if strings.TrimSpace(alias) != "" {
		values.Set("alias", strings.TrimSpace(alias))
	}
	rawURL := s.baseURL + "/gmail/mails/" + url.PathEscape(accountID)
	if encoded := values.Encode(); encoded != "" {
		rawURL += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}

	var payload struct {
		Success bool        `json:"success"`
		Message string      `json:"message"`
		Alias   string      `json:"alias"`
		Mails   []gmailMail `json:"mails"`
	}
	if err := s.doJSON(req, &payload); err != nil {
		return "", err
	}

	for _, mail := range newestGmailMailsFirst(payload.Mails) {
		if !gmailMailMatchesAlias(mail.To, alias) {
			continue
		}
		body := firstNonEmpty(mail.Text, mail.HTML, mail.Body)
		if code := extractChatGPTVerificationCode(body, mail.Subject, re); code != "" {
			return code, nil
		}
	}

	if strings.TrimSpace(payload.Message) != "" {
		return "", fmt.Errorf("%s", payload.Message)
	}
	return "", fmt.Errorf("verification code unavailable")
}

func newestGmailMailsFirst(mails []gmailMail) []gmailMail {
	result := append([]gmailMail(nil), mails...)
	sort.SliceStable(result, func(i, j int) bool {
		return parseMailDate(result[i].Date).After(parseMailDate(result[j].Date))
	})
	return result
}

func parseMailDate(value string) time.Time {
	if parsed, err := mailparse.ParseDate(strings.TrimSpace(value)); err == nil {
		return parsed
	}
	return time.Time{}
}

func gmailMailMatchesAlias(to, alias string) bool {
	alias = strings.ToLower(strings.TrimSpace(alias))
	if alias == "" {
		return true
	}
	to = strings.ToLower(strings.TrimSpace(to))
	return to == alias || strings.Contains(to, "<"+alias+">") || strings.Contains(to, alias)
}

func extractChatGPTVerificationCode(body, subject string, re *regexp.Regexp) string {
	combined := firstNonEmpty(body, subject)
	if combined == "" {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?is)Enter\s+this\s+temporary\s+verification\s+code\s+to\s+continue:\s*.*?\b(\d{6})\b`),
		regexp.MustCompile(`(?is)temporary\s+ChatGPT\s+verification\s+code.*?\b(\d{6})\b`),
	}
	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(combined); len(match) > 1 {
			return strings.TrimSpace(match[1])
		}
	}
	if match := re.FindStringSubmatch(combined); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func (s *gmailAliasService) defaultAccount(ctx context.Context) (struct{ ID, Email string }, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/gmail/accounts", nil)
	if err != nil {
		return struct{ ID, Email string }{}, err
	}
	var payload struct {
		Accounts []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"accounts"`
	}
	if err := s.doJSON(req, &payload); err != nil {
		return struct{ ID, Email string }{}, err
	}
	for _, account := range payload.Accounts {
		if strings.TrimSpace(account.ID) != "" && strings.TrimSpace(account.Email) != "" {
			return struct{ ID, Email string }{ID: strings.TrimSpace(account.ID), Email: strings.TrimSpace(account.Email)}, nil
		}
	}
	return struct{ ID, Email string }{}, fmt.Errorf("gmail api has no configured accounts")
}

func (s *gmailAliasService) doJSON(req *http.Request, out any) error {
	return doEmailServiceJSON(req, out, "gmail api")
}

func doEmailServiceJSON(req *http.Request, out any, serviceName string) error {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("User-Agent", userAgent())

	client := newHTTPClient("", 30*time.Second, true)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		var errBody map[string]any
		if json.Unmarshal(body, &errBody) == nil {
			if detail := strings.TrimSpace(asString(errBody["detail"])); detail != "" {
				return fmt.Errorf("%s", detail)
			}
			if message := strings.TrimSpace(asString(errBody["message"])); message != "" {
				return fmt.Errorf("%s", message)
			}
		}
		return fmt.Errorf("%s request failed: %d", serviceName, resp.StatusCode)
	}

	return jsonUnmarshalResponse(body, out)
}

func randomMailboxPrefix(length int) string {
	if length < 6 {
		length = 8
	}

	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	builder := strings.Builder{}
	for i := 0; i < length; i++ {
		builder.WriteByte(charset[rand.IntN(len(charset))])
	}
	return builder.String()
}

func randomBirthdate() string {
	year := time.Now().Year() - (18 + rand.IntN(28))
	month := 1 + rand.IntN(12)
	day := 1 + rand.IntN(28)
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}
