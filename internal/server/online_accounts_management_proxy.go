package server

import (
	"io"
	"net/http"
	"strings"

	"codex-register/internal/runtime"
)

func (a *apiServer) handleCPAManagementAuthFilesStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPatch {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	cfg := runtime.ResolveCPAConfig(req.Context(), a.store)
	if strings.TrimSpace(cfg.APIURL) == "" || strings.TrimSpace(cfg.APIToken) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "CPA API 未配置"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.writeCPAManagementProxy(w, req, cfg, http.MethodPatch, "/status", "", body, "application/json")
}

func (a *apiServer) handleCPAManagementAuthFiles(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet, http.MethodDelete:
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	cfg := runtime.ResolveCPAConfig(req.Context(), a.store)
	if strings.TrimSpace(cfg.APIURL) == "" || strings.TrimSpace(cfg.APIToken) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "CPA API 未配置"})
		return
	}
	a.writeCPAManagementProxy(w, req, cfg, req.Method, "", req.URL.RawQuery, nil, "")
}

func (a *apiServer) writeCPAManagementProxy(w http.ResponseWriter, req *http.Request, cfg runtime.CPAConfig, method, pathSuffix, rawQuery string, body []byte, contentType string) {
	respBody, status, err := runtime.ForwardCPAManagementRequest(req.Context(), cfg, method, pathSuffix, rawQuery, body, contentType)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(respBody)
}
