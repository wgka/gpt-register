package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"codex-register/internal/runtime"
	"codex-register/internal/store"
)

func (a *apiServer) handleEmailServiceStats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"meteormail_available": true,
		"enabled_count":        1,
	})
}

func (a *apiServer) handleEmailServiceTypes(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"types": []map[string]any{
			{
				"value":       "meteormail",
				"label":       "MeteorMail",
				"description": "随机前缀拼接 @meteormail.me，直接轮询 meteormail 接口",
			},
		},
	})
}

func (a *apiServer) handleEmailServices(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"total": 1,
			"services": []map[string]any{
				virtualMeteorMailService(false),
			},
		})
	case http.MethodPost:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "meteormail does not require configuration"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (a *apiServer) handleEmailServiceDetail(w http.ResponseWriter, req *http.Request) {
	path := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/email-services/"), "/")
	switch path {
	case "1", "meteormail":
		switch req.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, virtualMeteorMailService(false))
		case http.MethodPatch, http.MethodDelete:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "meteormail does not require configuration"})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	case "1/full", "meteormail/full":
		if req.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, virtualMeteorMailService(true))
	case "1/test", "meteormail/test":
		if req.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if err := runtime.TestMeteormail(req.Context(), ""); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"success": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "MeteorMail 接口可用"})
	case "1/enable", "meteormail/enable", "1/disable", "meteormail/disable":
		if req.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "meteormail is always enabled"})
	default:
		http.NotFound(w, req)
	}
}

func virtualMeteorMailService(includeConfig bool) map[string]any {
	service := map[string]any{
		"id":           1,
		"service_type": "meteormail",
		"name":         "MeteorMail",
		"enabled":      true,
		"priority":     0,
		"domain":       "meteormail.me",
	}
	if includeConfig {
		service["config"] = map[string]any{
			"api_url": "http://meteormail.me/api/mails/{email}",
			"domain":  "meteormail.me",
			"mode":    "random_prefix",
		}
	}
	return service
}

func (a *apiServer) handleRegistrationStart(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var payload runtime.StartRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(payload.EmailServiceType) == "" {
		payload.EmailServiceType = "meteormail"
	}

	task, err := a.tasks.StartRegistration(req.Context(), payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (a *apiServer) handleRegistrationBatch(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var payload runtime.BatchRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(payload.EmailServiceType) == "" {
		payload.EmailServiceType = "meteormail"
	}

	batchID, tasks, err := a.tasks.StartBatch(req.Context(), payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"batch_id": batchID,
		"count":    len(tasks),
		"tasks":    tasks,
	})
}

func (a *apiServer) handleRegistrationBatchDetail(w http.ResponseWriter, req *http.Request) {
	batchID := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/registration/batch/"), "/")
	if batchID == "" {
		http.NotFound(w, req)
		return
	}

	switch req.Method {
	case http.MethodGet:
		batch := a.tasks.GetBatch(batchID)
		if batch == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "批量任务不存在"})
			return
		}
		writeJSON(w, http.StatusOK, batch)
	case http.MethodPost:
		if !strings.HasSuffix(req.URL.Path, "/cancel") {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		batchID = strings.Trim(strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/api/registration/batch/"), "/cancel"), "/")
		if !a.tasks.CancelBatch(batchID) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "批量任务不存在或已结束"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "批量任务取消请求已提交"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (a *apiServer) handleRegistrationTasks(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	result, err := a.store.ListRegistrationTasks(req.Context(), store.RegistrationTaskListParams{
		Page:     parsePositiveInt(req.URL.Query().Get("page"), 1, 1, 100000),
		PageSize: parsePositiveInt(req.URL.Query().Get("page_size"), 20, 1, 100),
		Status:   req.URL.Query().Get("status"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *apiServer) handleRegistrationTaskDetail(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/api/registration/tasks/")
	if path == "" {
		http.NotFound(w, req)
		return
	}

	switch {
	case strings.HasSuffix(path, "/logs"):
		taskUUID := strings.TrimSuffix(strings.TrimSuffix(path, "/logs"), "/")
		task, err := a.store.GetRegistrationTaskByUUID(req.Context(), taskUUID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if task == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "任务不存在"})
			return
		}
		logs := []string{}
		if strings.TrimSpace(task.Logs) != "" {
			logs = strings.Split(task.Logs, "\n")
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"task_uuid": task.TaskUUID,
			"status":    task.Status,
			"logs":      logs,
		})
	case strings.HasSuffix(path, "/cancel"):
		if req.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		taskUUID := strings.TrimSuffix(strings.TrimSuffix(path, "/cancel"), "/")
		a.tasks.CancelTask(taskUUID)
		_, err := a.store.UpdateRegistrationTask(req.Context(), taskUUID, map[string]any{
			"status":        "cancelled",
			"completed_at":  timeNowRFC3339(),
			"error_message": "任务已取消",
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
	default:
		taskUUID := strings.Trim(path, "/")
		switch req.Method {
		case http.MethodGet:
			task, err := a.store.GetRegistrationTaskByUUID(req.Context(), taskUUID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if task == nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "任务不存在"})
				return
			}
			writeJSON(w, http.StatusOK, task)
		case http.MethodDelete:
			if err := a.store.DeleteRegistrationTask(req.Context(), taskUUID); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"success": true})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	}
}

func (a *apiServer) handleRegistrationStats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	stats, err := a.store.GetRegistrationStats(req.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (a *apiServer) handleRegistrationAvailableServices(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"meteormail": map[string]any{
			"available": true,
			"count":     1,
			"services": []map[string]any{
				{
					"id":          1,
					"name":        "MeteorMail",
					"type":        "meteormail",
					"domain":      "meteormail.me",
					"description": "随机前缀邮箱，直接轮询 meteormail 接口",
				},
			},
		},
	})
}

func timeNowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}
