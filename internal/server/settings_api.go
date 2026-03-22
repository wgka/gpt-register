package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"codex-register/internal/config"
)

type settingsEditableInfo struct {
	Proxy    settingsProxyInfo    `json:"proxy"`
	CPA      settingsCPAInfo      `json:"cpa"`
	Telegram settingsTelegramInfo `json:"telegram"`
}

type settingsProxyInfo struct {
	URL              string `json:"url"`
	APIURL           string `json:"api_url"`
	Attempts         int    `json:"attempts"`
	PreflightTimeout int    `json:"preflight_timeout"`
}

type settingsCPAInfo struct {
	Enabled  bool   `json:"enabled"`
	APIURL   string `json:"api_url"`
	APIToken string `json:"api_token"`
	ProxyURL string `json:"proxy_url"`
}

type settingsTelegramInfo struct {
	BotToken       string `json:"bot_token"`
	AllowedChatIDs string `json:"allowed_chat_ids"`
	Debug          bool   `json:"debug"`
	RestartHint    string `json:"restart_hint"`
}

type updateSettingsRequest struct {
	Editable settingsEditableInfo `json:"editable"`
}

func (a *apiServer) handleSettings(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		resp := settingsResponse{
			App: settingsAppInfo{
				Name:    a.cfg.AppName,
				Version: a.cfg.AppVersion,
				Debug:   a.cfg.Debug,
			},
			Runtime: settingsRuntimeInfo{
				Addr:           a.cfg.Addr(),
				DatabaseURL:    a.cfg.NormalizedDatabaseURL(),
				DatabaseDriver: a.cfg.DatabaseDriver(),
				LogFile:        a.cfg.LogFile,
			},
			Sections: a.cfg.PublicSections(),
			Editable: currentEditableSettings(),
		}

		writeJSON(w, http.StatusOK, resp)
	case http.MethodPut:
		var payload updateSettingsRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		updates := map[string]string{
			"APP_PROXY_URL":               strings.TrimSpace(payload.Editable.Proxy.URL),
			"APP_PROXY_API_URL":           strings.TrimSpace(payload.Editable.Proxy.APIURL),
			"APP_PROXY_ATTEMPTS":          strconv.Itoa(clampInt(payload.Editable.Proxy.Attempts, 1, 20, 4)),
			"APP_PROXY_PREFLIGHT_TIMEOUT": strconv.Itoa(clampInt(payload.Editable.Proxy.PreflightTimeout, 3, 60, 12)),
			"APP_CPA_ENABLED":             strconv.FormatBool(payload.Editable.CPA.Enabled),
			"APP_CPA_API_URL":             strings.TrimSpace(payload.Editable.CPA.APIURL),
			"APP_CPA_API_TOKEN":           strings.TrimSpace(payload.Editable.CPA.APIToken),
			"APP_CPA_PROXY_URL":           strings.TrimSpace(payload.Editable.CPA.ProxyURL),
			"VITE_CPA_API_URL":            strings.TrimSpace(payload.Editable.CPA.APIURL),
			"VITE_CPA_API_TOKEN":          strings.TrimSpace(payload.Editable.CPA.APIToken),
			"TELEGRAM_BOT_TOKEN":          strings.TrimSpace(payload.Editable.Telegram.BotToken),
			"TELEGRAM_ALLOWED_CHAT_IDS":   strings.TrimSpace(payload.Editable.Telegram.AllowedChatIDs),
			"TELEGRAM_DEBUG":              strconv.FormatBool(payload.Editable.Telegram.Debug),
		}

		if err := config.UpsertEnvFile(".env", updates); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		for key, value := range updates {
			_ = os.Setenv(key, value)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"success":  true,
			"editable": currentEditableSettings(),
			"message":  "设置已保存；Telegram Bot Token 变更需重启应用后生效",
		})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func currentEditableSettings() settingsEditableInfo {
	return settingsEditableInfo{
		Proxy: settingsProxyInfo{
			URL:              strings.TrimSpace(os.Getenv("APP_PROXY_URL")),
			APIURL:           strings.TrimSpace(os.Getenv("APP_PROXY_API_URL")),
			Attempts:         envIntWithFallback("APP_PROXY_ATTEMPTS", 4),
			PreflightTimeout: envIntWithFallback("APP_PROXY_PREFLIGHT_TIMEOUT", 12),
		},
		CPA: settingsCPAInfo{
			Enabled:  envBoolWithFallback("APP_CPA_ENABLED", false),
			APIURL:   strings.TrimSpace(os.Getenv("APP_CPA_API_URL")),
			APIToken: strings.TrimSpace(os.Getenv("APP_CPA_API_TOKEN")),
			ProxyURL: strings.TrimSpace(os.Getenv("APP_CPA_PROXY_URL")),
		},
		Telegram: settingsTelegramInfo{
			BotToken:       strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
			AllowedChatIDs: strings.TrimSpace(os.Getenv("TELEGRAM_ALLOWED_CHAT_IDS")),
			Debug:          envBoolWithFallback("TELEGRAM_DEBUG", false),
			RestartHint:    "Bot Token 变更后需重启应用，其他配置保存后会同步到当前进程环境",
		},
	}
}

func envBoolWithFallback(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envIntWithFallback(key string, fallback int) int {
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

func clampInt(value, minValue, maxValue, fallback int) int {
	if value == 0 {
		value = fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
