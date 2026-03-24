package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"codex-register/internal/config"
	"codex-register/internal/runtime"
)

type updateOnlineAccountsSchedulerRequest struct {
	Config runtime.OnlineAccountsScheduleConfig `json:"config"`
}

func (a *apiServer) handleOnlineAccountsScheduler(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.onlineAccountsSchedulerPayload(""))
	case http.MethodPut:
		var payload updateOnlineAccountsSchedulerRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		cfg := payload.Config.Normalized()
		if cfg.Enabled {
			if err := cfg.Validate(); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
		}

		editable := currentEditableSettings()
		if cfg.Enabled && (strings.TrimSpace(editable.CPA.APIURL) == "" || strings.TrimSpace(editable.CPA.APIToken) == "") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请先在设置页配置 CPA API URL 和 Token"})
			return
		}

		updates := map[string]string{
			"APP_ONLINE_ACCOUNTS_SCHEDULE_ENABLED":               strconv.FormatBool(cfg.Enabled),
			"APP_ONLINE_ACCOUNTS_SCHEDULE_MODE":                  cfg.Mode,
			"APP_ONLINE_ACCOUNTS_SCHEDULE_INTERVAL_MINUTES":      strconv.Itoa(cfg.IntervalMins),
			"APP_ONLINE_ACCOUNTS_SCHEDULE_FIXED_TIMES":           strings.Join(cfg.FixedTimes, ","),
			"APP_ONLINE_ACCOUNTS_SCHEDULE_DISABLE_TOKEN_INVALID": strconv.FormatBool(cfg.DisableInvalid),
			"APP_ONLINE_ACCOUNTS_SCHEDULE_DELETE_TOKEN_INVALID":  strconv.FormatBool(cfg.DeleteInvalid),
			"APP_ONLINE_ACCOUNTS_SCHEDULE_RETRY_COUNT":           strconv.Itoa(cfg.RetryCount),
			"APP_ONLINE_ACCOUNTS_SCHEDULE_RETRY_DELAY_MINUTES":   strconv.Itoa(cfg.RetryDelayMins),
		}

		if err := config.UpsertEnvFile(".env", updates); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		for key, value := range updates {
			_ = os.Setenv(key, value)
		}

		a.scheduler.UpdateConfig(cfg)
		writeJSON(w, http.StatusOK, a.onlineAccountsSchedulerPayload("定时任务配置已保存"))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (a *apiServer) handleOnlineAccountsSchedulerLogs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	result, err := a.store.ListOnlineAccountSchedulerLogs(
		req.Context(),
		parsePositiveInt(req.URL.Query().Get("page"), 1, 1, 100000),
		parsePositiveInt(req.URL.Query().Get("page_size"), 10, 1, 100),
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *apiServer) handleOnlineAccountsSchedulerRun(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	state, err := a.scheduler.RunNow(req.Context())
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"state":   state,
			"message": "定时任务执行完成",
		})
		return
	}

	if errors.Is(err, runtime.ErrOnlineAccountsSchedulerBusy) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"success": false,
			"state":   state,
			"error":   "定时任务正在执行中，请稍后再试",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": false,
		"state":   state,
		"error":   err.Error(),
	})
}

func (a *apiServer) onlineAccountsSchedulerPayload(message string) map[string]any {
	payload := map[string]any{
		"state": a.scheduler.GetState(),
	}
	if strings.TrimSpace(message) != "" {
		payload["message"] = message
	}
	return payload
}
