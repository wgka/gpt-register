package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"codex-register/internal/config"
	"codex-register/internal/runtime"
	"codex-register/internal/store"
)

type dashboardResponse struct {
	App struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"app"`
	Sections []dashboardSection `json:"sections"`
}

type dashboardSection struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type settingsResponse struct {
	App      settingsAppInfo        `json:"app"`
	Runtime  settingsRuntimeInfo    `json:"runtime"`
	Sections []config.PublicSection `json:"sections"`
}

type settingsAppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Debug   bool   `json:"debug"`
}

type settingsRuntimeInfo struct {
	Addr           string `json:"addr"`
	DatabaseURL    string `json:"database_url"`
	DatabaseDriver string `json:"database_driver"`
	LogFile        string `json:"log_file"`
}

type apiServer struct {
	cfg   config.Settings
	store *store.SQLiteStore
	tasks *runtime.TaskManager
}

type App struct {
	Handler http.Handler
	Store   *store.SQLiteStore
	Tasks   *runtime.TaskManager
}

func NewApp(cfg config.Settings) *App {
	dbStore, err := store.NewSQLiteStore(cfg)
	if err != nil {
		log.Printf("database initialization warning: %v", err)
		dbStore = &store.SQLiteStore{}
	}

	api := &apiServer{
		cfg:   cfg,
		store: dbStore,
		tasks: runtime.NewTaskManager(dbStore),
	}

	mux := apiRoutes(cfg, api)
	return &App{
		Handler: mux,
		Store:   dbStore,
		Tasks:   api.tasks,
	}
}

func apiMux(cfg config.Settings) http.Handler {
	app := NewApp(cfg)
	return app.Handler
}

func apiRoutes(cfg config.Settings, api *apiServer) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"host":      cfg.WebUIHost,
			"port":      cfg.WebUIPort,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("/api/dashboard", func(w http.ResponseWriter, req *http.Request) {
		resp := dashboardResponse{
			Sections: []dashboardSection{
				{
					Title:       "Go API",
					Description: "统一承载接口、静态资源和 SPA 路由回退。",
					Status:      "ready",
				},
				{
					Title:       "Vue 3",
					Description: "前端使用 Vite 构建并输出到 web/dist。",
					Status:      "ready",
				},
				{
					Title:       "业务模块",
					Description: "注册链路与邮箱接口已切到 Go，当前统一使用 MeteorMail 单服务模式。",
					Status:      "in_progress",
				},
			},
		}
		resp.App.Name = cfg.AppName
		resp.App.Version = cfg.AppVersion

		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, req *http.Request) {
		resp := settingsResponse{
			App: settingsAppInfo{
				Name:    cfg.AppName,
				Version: cfg.AppVersion,
				Debug:   cfg.Debug,
			},
			Runtime: settingsRuntimeInfo{
				Addr:           cfg.Addr(),
				DatabaseURL:    cfg.NormalizedDatabaseURL(),
				DatabaseDriver: cfg.DatabaseDriver(),
				LogFile:        cfg.LogFile,
			},
			Sections: cfg.PublicSections(),
		}

		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("/api/accounts/stats/summary", api.handleAccountStats)
	mux.HandleFunc("/api/accounts/batch-refresh", api.handleBatchRefresh)
	mux.HandleFunc("/api/accounts/batch-validate", api.handleBatchValidate)
	mux.HandleFunc("/api/accounts/batch-upload-cpa", api.handleBatchCPAUpload)
	mux.HandleFunc("/api/accounts/", api.handleAccountRoute)
	mux.HandleFunc("/api/accounts", api.handleAccounts)
	mux.HandleFunc("/api/email-services/stats", api.handleEmailServiceStats)
	mux.HandleFunc("/api/email-services/types", api.handleEmailServiceTypes)
	mux.HandleFunc("/api/email-services/", api.handleEmailServiceDetail)
	mux.HandleFunc("/api/email-services", api.handleEmailServices)
	mux.HandleFunc("/api/registration/start", api.handleRegistrationStart)
	mux.HandleFunc("/api/registration/batch/", api.handleRegistrationBatchDetail)
	mux.HandleFunc("/api/registration/batch", api.handleRegistrationBatch)
	mux.HandleFunc("/api/registration/tasks/", api.handleRegistrationTaskDetail)
	mux.HandleFunc("/api/registration/tasks", api.handleRegistrationTasks)
	mux.HandleFunc("/api/registration/stats", api.handleRegistrationStats)
	mux.HandleFunc("/api/registration/available-services", api.handleRegistrationAvailableServices)
	mux.HandleFunc("/ws/task/", api.handleTaskWebSocket)
	mux.HandleFunc("/ws/batch/", api.handleBatchWebSocket)

	return mux
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *apiServer) handleAccounts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	params := store.AccountListParams{
		Page:         parsePositiveInt(req.URL.Query().Get("page"), 1, 1, 100000),
		PageSize:     parsePositiveInt(req.URL.Query().Get("page_size"), 20, 1, 100),
		Status:       req.URL.Query().Get("status"),
		EmailService: req.URL.Query().Get("email_service"),
		Search:       req.URL.Query().Get("search"),
	}

	result, err := a.store.ListAccounts(req.Context(), params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *apiServer) handleAccountStats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	stats, err := a.store.GetAccountStats(req.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func parsePositiveInt(raw string, fallback, minValue, maxValue int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
