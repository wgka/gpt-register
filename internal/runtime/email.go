package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	defaultMeteorMailAPIURL = "http://meteormail.me/api/mails"
	defaultMeteorMailDomain = "meteormail.me"
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

type meteormailService struct {
	baseURL  string
	domain   string
	proxyURL string
}

func newMeteormailService(proxyURL string) EmailService {
	return &meteormailService{
		baseURL:  defaultMeteorMailAPIURL,
		domain:   defaultMeteorMailDomain,
		proxyURL: proxyURL,
	}
}

func TestMeteormail(ctx context.Context, proxyURL string) error {
	service := newMeteormailService(proxyURL).(*meteormailService)
	emailInfo, err := service.CreateEmail(ctx)
	if err != nil {
		return err
	}

	_, _, statusCode, err := service.fetchMailbox(ctx, emailInfo.Email, "")
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("meteormail returned status %d", statusCode)
	}
	return nil
}

func (s *meteormailService) Type() string { return "meteormail" }

func (s *meteormailService) CreateEmail(ctx context.Context) (EmailInfo, error) {
	_ = ctx

	email := randomMailboxPrefix(8) + "@" + s.domain
	return EmailInfo{
		Email:     email,
		ServiceID: email,
	}, nil
}

func (s *meteormailService) GetVerificationCode(ctx context.Context, email, emailID string, timeout time.Duration, pattern string) (string, error) {
	target := strings.TrimSpace(emailID)
	if target == "" {
		target = strings.TrimSpace(email)
	}
	if target == "" {
		return "", fmt.Errorf("meteormail email is required")
	}

	deadline := time.Now().Add(timeout)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid verification code pattern: %w", err)
	}
	seen := map[string]struct{}{}
	etag := ""

	for time.Now().Before(deadline) {
		payload, nextETag, statusCode, err := s.fetchMailbox(ctx, target, etag)
		if err == nil {
			etag = nextETag
			if statusCode != http.StatusNotModified {
				for _, content := range meteormailContents(payload) {
					normalized := strings.ToLower(strings.TrimSpace(content))
					if normalized == "" {
						continue
					}
					if _, ok := seen[normalized]; ok {
						continue
					}
					seen[normalized] = struct{}{}
					if !strings.Contains(normalized, "openai") {
						continue
					}

					match := re.FindStringSubmatch(normalized)
					if len(match) > 1 {
						return match[1], nil
					}
				}
			}
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	return "", fmt.Errorf("verification code timeout for %s", target)
}

func (s *meteormailService) fetchMailbox(ctx context.Context, mailbox, etag string) ([]byte, string, int, error) {
	rawURL := strings.TrimRight(s.baseURL, "/") + "/" + url.PathEscape(mailbox)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", 0, err
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36")
	if strings.TrimSpace(etag) != "" {
		req.Header.Set("If-None-Match", etag)
	}

	client := newHTTPClient(s.proxyURL, 30*time.Second, true)
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()

	nextETag := strings.TrimSpace(resp.Header.Get("ETag"))
	if resp.StatusCode == http.StatusNotModified {
		return nil, nextETag, resp.StatusCode, nil
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, nextETag, resp.StatusCode, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nextETag, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return body, nextETag, resp.StatusCode, fmt.Errorf("meteormail request failed: %d", resp.StatusCode)
	}
	return body, nextETag, resp.StatusCode, nil
}

func meteormailContents(raw []byte) []string {
	var payload struct {
		Mails []any `json:"mails"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}

	contents := make([]string, 0, len(payload.Mails))
	for _, mail := range payload.Mails {
		fragments := make([]string, 0, 8)
		collectStringFragments(mail, &fragments)
		if len(fragments) == 0 {
			continue
		}
		contents = append(contents, strings.Join(fragments, "\n"))
	}
	return contents
}

func collectStringFragments(value any, fragments *[]string) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed != "" {
			*fragments = append(*fragments, trimmed)
		}
	case []any:
		for _, item := range typed {
			collectStringFragments(item, fragments)
		}
	case map[string]any:
		for _, item := range typed {
			collectStringFragments(item, fragments)
		}
	}
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
