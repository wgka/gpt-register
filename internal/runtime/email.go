package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultTempMailAPIBaseURL = "http://82.158.91.228:8081"
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
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("User-Agent", userAgent())

	client := newHTTPClient("", 30*time.Second, true)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var body map[string]any
		if json.NewDecoder(resp.Body).Decode(&body) == nil {
			if detail := strings.TrimSpace(asString(body["detail"])); detail != "" {
				return fmt.Errorf("%s", detail)
			}
			if message := strings.TrimSpace(asString(body["message"])); message != "" {
				return fmt.Errorf("%s", message)
			}
		}
		return fmt.Errorf("temp mail api request failed: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
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
