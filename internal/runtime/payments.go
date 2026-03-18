package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	http "github.com/bogdanfinn/fhttp"
)

const (
	bindCardCheckoutEndpoint = "https://chatgpt.com/backend-api/payments/checkout"
	bindCardCheckoutBaseURL  = "https://chatgpt.com/checkout/openai_llc/"
)

func GenerateBindCardLink(ctx context.Context, accessToken, proxyURL string) (string, error) {
	trimmedToken := strings.TrimSpace(accessToken)
	if trimmedToken == "" {
		return "", fmt.Errorf("账号没有 access_token")
	}

	client, err := newTokenClient(proxyURL)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"plan_name": "chatgptteamplan",
		"team_plan_data": map[string]any{
			"workspace_name": "MyTeam",
			"price_interval": "month",
			"seat_quantity":  5,
		},
		"promo_campaign": map[string]any{
			"promo_campaign_id":          "team-1-month-free",
			"is_coupon_from_query_param": true,
		},
		"checkout_ui_mode": "custom",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, bindCardCheckoutEndpoint, strings.NewReader(string(body)))
	req.Header = browserHeaders(http.Header{
		"accept":             {"application/json, text/plain, */*"},
		"accept-language":    {"en-US,en;q=0.9"},
		"authorization":      {"Bearer " + trimmedToken},
		"content-type":       {"application/json"},
		"origin":             {"https://chatgpt.com"},
		"referer":            {"https://chatgpt.com/"},
		"sec-ch-ua":          {secCHUA()},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Windows"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"same-origin"},
		"user-agent":         {userAgent()},
	}, "accept", "accept-language", "authorization", "content-type", "origin", "referer", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "user-agent")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var data map[string]any
	_ = json.Unmarshal(respBody, &data)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%s", resolveCheckoutError(data, resp.StatusCode))
	}

	checkoutURL := resolveCheckoutURL(data)
	if checkoutURL == "" {
		return "", fmt.Errorf("未返回绑卡链接")
	}
	return checkoutURL, nil
}

func resolveCheckoutURL(data map[string]any) string {
	if value := strings.TrimSpace(asString(data["checkout_session_id"])); value != "" {
		return buildCheckoutURL(value)
	}

	urlValue := strings.TrimSpace(asString(data["url"]))
	if urlValue == "" {
		return ""
	}

	if match := checkoutSessionPattern.FindStringSubmatch(urlValue); len(match) >= 2 {
		return buildCheckoutURL(match[1])
	}

	return urlValue
}

func buildCheckoutURL(checkoutSessionID string) string {
	return bindCardCheckoutBaseURL + strings.TrimSpace(checkoutSessionID)
}

func resolveCheckoutError(data map[string]any, status int) string {
	for _, key := range []string{"detail", "message", "error"} {
		if value := strings.TrimSpace(asString(data[key])); value != "" {
			return value
		}
	}
	return fmt.Sprintf("HTTP %d", status)
}
