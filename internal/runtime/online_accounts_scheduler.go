package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"codex-register/internal/store"
)

var ErrOnlineAccountsSchedulerBusy = errors.New("online accounts scheduler is already running")

const (
	OnlineAccountsScheduleModeInterval   = "interval"
	OnlineAccountsScheduleModeFixedTimes = "fixed_times"

	onlineAccountsTriggerManual    = "manual"
	onlineAccountsTriggerScheduled = "scheduled"
	onlineAccountsTriggerRetry     = "retry"

	onlineAccountsRunStatusSuccess       = "success"
	onlineAccountsRunStatusFailed        = "failed"
	onlineAccountsRunStatusPartialFailed = "partial_failed"
)

var onlineAccountsTokenInvalidMarkers = []string{
	"token_invalidated",
	"deactivated_workspace",
}

var onlineAccountsHTTPClientFactory = func(proxyURL string, timeout time.Duration) *http.Client {
	return newHTTPClient(proxyURL, timeout, true)
}

type OnlineAccountsScheduleConfig struct {
	Enabled        bool     `json:"enabled"`
	Mode           string   `json:"mode"`
	IntervalMins   int      `json:"interval_minutes"`
	FixedTimes     []string `json:"fixed_times"`
	DisableInvalid bool     `json:"disable_invalid"`
	DeleteInvalid  bool     `json:"delete_invalid"`
	RetryCount     int      `json:"retry_count"`
	RetryDelayMins int      `json:"retry_delay_minutes"`
}

type OnlineAccountsRunResult struct {
	StartedAt     string   `json:"started_at"`
	FinishedAt    string   `json:"finished_at"`
	TriggerType   string   `json:"trigger_type,omitempty"`
	Status        string   `json:"status,omitempty"`
	Attempt       int      `json:"attempt"`
	MaxAttempts   int      `json:"max_attempts"`
	Actions       []string `json:"actions"`
	InvalidFound  int      `json:"invalid_found"`
	DisabledCount int      `json:"disabled_count"`
	DeletedCount  int      `json:"deleted_count"`
	FailedCount   int      `json:"failed_count"`
	Messages      []string `json:"messages,omitempty"`
	Error         string   `json:"error,omitempty"`
}

type OnlineAccountsScheduleState struct {
	Config         OnlineAccountsScheduleConfig `json:"config"`
	Running        bool                         `json:"running"`
	LastRunAt      string                       `json:"last_run_at,omitempty"`
	NextRunAt      string                       `json:"next_run_at,omitempty"`
	NextRunReason  string                       `json:"next_run_reason,omitempty"`
	RetryPending   bool                         `json:"retry_pending"`
	RetryRemaining int                          `json:"retry_remaining"`
	LastResult     *OnlineAccountsRunResult     `json:"last_result,omitempty"`
}

type OnlineAccountsScheduler struct {
	store *store.SQLiteStore

	startOnce sync.Once

	mu             sync.RWMutex
	config         OnlineAccountsScheduleConfig
	running        bool
	lastRunAt      time.Time
	nextRunAt      time.Time
	nextRunReason  string
	retryPending   bool
	retryRemaining int
	currentAttempt int
	maxAttempts    int
	lastResult     *OnlineAccountsRunResult
}

type managementAuthFilesResponse struct {
	Files []managementAuthFile `json:"files"`
}

type managementAuthFile struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Account       string          `json:"account"`
	Email         string          `json:"email"`
	Disabled      bool            `json:"disabled"`
	StatusMessage json.RawMessage `json:"status_message"`
}

type onlineAccountsManagementConfig struct {
	Endpoint string
	Token    string
	ProxyURL string
}

func DefaultOnlineAccountsScheduleConfig() OnlineAccountsScheduleConfig {
	return OnlineAccountsScheduleConfig{
		Enabled:        false,
		Mode:           OnlineAccountsScheduleModeInterval,
		IntervalMins:   30,
		FixedTimes:     []string{"09:00"},
		DisableInvalid: false,
		DeleteInvalid:  true,
		RetryCount:     2,
		RetryDelayMins: 5,
	}
}

func LoadOnlineAccountsScheduleConfigFromEnv() OnlineAccountsScheduleConfig {
	cfg := DefaultOnlineAccountsScheduleConfig()

	if raw := firstEnv("APP_ONLINE_ACCOUNTS_SCHEDULE_ENABLED"); raw != "" {
		cfg.Enabled = parseEnvBool(raw)
	}
	if raw := firstEnv("APP_ONLINE_ACCOUNTS_SCHEDULE_MODE"); raw != "" {
		cfg.Mode = strings.TrimSpace(raw)
	}
	if raw := firstEnv("APP_ONLINE_ACCOUNTS_SCHEDULE_INTERVAL_MINUTES"); raw != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			cfg.IntervalMins = parsed
		}
	}
	if raw := firstEnv("APP_ONLINE_ACCOUNTS_SCHEDULE_FIXED_TIMES"); raw != "" {
		cfg.FixedTimes = strings.Split(raw, ",")
	}
	if raw := firstEnv("APP_ONLINE_ACCOUNTS_SCHEDULE_DISABLE_TOKEN_INVALID"); raw != "" {
		cfg.DisableInvalid = parseEnvBool(raw)
	}
	if raw := firstEnv("APP_ONLINE_ACCOUNTS_SCHEDULE_DELETE_TOKEN_INVALID"); raw != "" {
		cfg.DeleteInvalid = parseEnvBool(raw)
	}
	if raw := firstEnv("APP_ONLINE_ACCOUNTS_SCHEDULE_RETRY_COUNT"); raw != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			cfg.RetryCount = parsed
		}
	}
	if raw := firstEnv("APP_ONLINE_ACCOUNTS_SCHEDULE_RETRY_DELAY_MINUTES"); raw != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			cfg.RetryDelayMins = parsed
		}
	}

	return cfg.Normalized()
}

func (c OnlineAccountsScheduleConfig) Normalized() OnlineAccountsScheduleConfig {
	defaults := DefaultOnlineAccountsScheduleConfig()

	switch strings.TrimSpace(c.Mode) {
	case OnlineAccountsScheduleModeFixedTimes:
		c.Mode = OnlineAccountsScheduleModeFixedTimes
	default:
		c.Mode = OnlineAccountsScheduleModeInterval
	}

	if c.IntervalMins <= 0 {
		c.IntervalMins = defaults.IntervalMins
	}
	if c.IntervalMins > 24*60 {
		c.IntervalMins = 24 * 60
	}

	c.DisableInvalid = false
	c.FixedTimes = normalizeFixedTimes(c.FixedTimes)

	if c.RetryCount < 0 {
		c.RetryCount = 0
	}
	if c.RetryCount > 10 {
		c.RetryCount = 10
	}
	if c.RetryDelayMins <= 0 {
		c.RetryDelayMins = defaults.RetryDelayMins
	}
	if c.RetryDelayMins > 24*60 {
		c.RetryDelayMins = 24 * 60
	}

	return c
}

func (c OnlineAccountsScheduleConfig) HasActions() bool {
	return c.DeleteInvalid
}

func (c OnlineAccountsScheduleConfig) Validate() error {
	cfg := c.Normalized()
	if !cfg.HasActions() {
		return errors.New("至少选择一个定时动作")
	}
	if cfg.Mode == OnlineAccountsScheduleModeFixedTimes && len(cfg.FixedTimes) == 0 {
		return errors.New("固定时间模式下至少配置一个执行时间")
	}
	return nil
}

func NewOnlineAccountsScheduler(db *store.SQLiteStore) *OnlineAccountsScheduler {
	s := &OnlineAccountsScheduler{
		store:  db,
		config: LoadOnlineAccountsScheduleConfigFromEnv(),
	}
	if latest, err := s.loadLatestResult(context.Background()); err != nil {
		log.Printf("online-accounts scheduler load latest result failed: %v", err)
	} else if latest != nil {
		s.lastResult = latest
		if parsed, err := parseRFC3339(latest.FinishedAt); err == nil {
			s.lastRunAt = parsed
		}
	}
	s.resetNextRunLocked(time.Now())
	return s
}

func (s *OnlineAccountsScheduler) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		go s.loop(ctx)
	})
}

func (s *OnlineAccountsScheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runScheduled(ctx)
		}
	}
}

func (s *OnlineAccountsScheduler) runScheduled(ctx context.Context) {
	s.mu.RLock()
	due := !s.running && !s.nextRunAt.IsZero() && !time.Now().Before(s.nextRunAt)
	reason := s.nextRunReason
	s.mu.RUnlock()
	if !due {
		return
	}

	triggerType := onlineAccountsTriggerScheduled
	if reason == onlineAccountsTriggerRetry {
		triggerType = onlineAccountsTriggerRetry
	}

	if _, err := s.run(ctx, triggerType); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, ErrOnlineAccountsSchedulerBusy) {
		log.Printf("online-accounts scheduler run failed: %v", err)
	}
}

func (s *OnlineAccountsScheduler) GetState() OnlineAccountsScheduleState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked()
}

func (s *OnlineAccountsScheduler) UpdateConfig(cfg OnlineAccountsScheduleConfig) OnlineAccountsScheduleState {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = cfg.Normalized()
	s.clearRetryLocked()
	s.resetNextRunLocked(time.Now())
	return s.snapshotLocked()
}

func (s *OnlineAccountsScheduler) RunNow(ctx context.Context) (OnlineAccountsScheduleState, error) {
	if _, err := s.run(ctx, onlineAccountsTriggerManual); err != nil {
		return s.GetState(), err
	}
	return s.GetState(), nil
}

func (s *OnlineAccountsScheduler) run(ctx context.Context, triggerType string) (OnlineAccountsRunResult, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return OnlineAccountsRunResult{}, ErrOnlineAccountsSchedulerBusy
	}

	cfg := s.config.Normalized()
	if err := cfg.Validate(); err != nil {
		s.mu.Unlock()
		return OnlineAccountsRunResult{}, err
	}

	attempt, maxAttempts := s.nextAttemptLocked(triggerType, cfg)
	s.running = true
	s.mu.Unlock()

	result, err := ExecuteOnlineAccountsMaintenance(ctx, cfg)
	finishedAt := time.Now().UTC()
	if strings.TrimSpace(result.FinishedAt) == "" {
		result.FinishedAt = finishedAt.Format(time.RFC3339)
	}
	result.TriggerType = triggerType
	result.Attempt = attempt
	result.MaxAttempts = maxAttempts
	if err == nil && result.FailedCount > 0 {
		err = fmt.Errorf("本次执行存在 %d 个失败项", result.FailedCount)
	}
	if err != nil {
		result.Error = err.Error()
	}
	result.Status = deriveRunStatus(result, err)

	if saveErr := s.persistRunLog(ctx, result, cfg); saveErr != nil {
		log.Printf("online-accounts scheduler persist log failed: %v", saveErr)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
	s.lastRunAt = finishedAt
	resultCopy := result
	s.lastResult = &resultCopy

	s.applyNextRunPolicyLocked(triggerType, cfg, finishedAt, result)
	return result, err
}

func (s *OnlineAccountsScheduler) nextAttemptLocked(triggerType string, cfg OnlineAccountsScheduleConfig) (int, int) {
	switch triggerType {
	case onlineAccountsTriggerRetry:
		if s.maxAttempts <= 0 {
			s.maxAttempts = 1 + cfg.RetryCount
		}
		return s.currentAttempt + 1, s.maxAttempts
	case onlineAccountsTriggerScheduled:
		return 1, 1 + cfg.RetryCount
	default:
		return 1, 1
	}
}

func (s *OnlineAccountsScheduler) applyNextRunPolicyLocked(triggerType string, cfg OnlineAccountsScheduleConfig, finishedAt time.Time, result OnlineAccountsRunResult) {
	isSuccess := result.Status == onlineAccountsRunStatusSuccess

	switch triggerType {
	case onlineAccountsTriggerManual:
		if isSuccess {
			s.clearRetryLocked()
			if cfg.Enabled {
				s.resetNextRunLocked(finishedAt)
			} else {
				s.nextRunAt = time.Time{}
				s.nextRunReason = ""
			}
		}
		return
	case onlineAccountsTriggerScheduled, onlineAccountsTriggerRetry:
		if isSuccess {
			s.clearRetryLocked()
			s.resetNextRunLocked(finishedAt)
			return
		}

		attempt := result.Attempt
		maxAttempts := result.MaxAttempts
		if cfg.Enabled && attempt < maxAttempts && cfg.RetryCount > 0 {
			s.retryPending = true
			s.retryRemaining = maxAttempts - attempt
			s.currentAttempt = attempt
			s.maxAttempts = maxAttempts
			s.nextRunAt = finishedAt.Add(time.Duration(cfg.RetryDelayMins) * time.Minute)
			s.nextRunReason = onlineAccountsTriggerRetry
			return
		}

		s.clearRetryLocked()
		s.resetNextRunLocked(finishedAt)
	}
}

func (s *OnlineAccountsScheduler) clearRetryLocked() {
	s.retryPending = false
	s.retryRemaining = 0
	s.currentAttempt = 0
	s.maxAttempts = 0
}

func (s *OnlineAccountsScheduler) resetNextRunLocked(base time.Time) {
	cfg := s.config.Normalized()
	if !cfg.Enabled || !cfg.HasActions() {
		s.nextRunAt = time.Time{}
		s.nextRunReason = ""
		return
	}

	nextRunAt, reason := nextRegularRun(base, cfg)
	s.nextRunAt = nextRunAt
	s.nextRunReason = reason
}

func (s *OnlineAccountsScheduler) snapshotLocked() OnlineAccountsScheduleState {
	state := OnlineAccountsScheduleState{
		Config:         s.config.Normalized(),
		Running:        s.running,
		NextRunReason:  s.nextRunReason,
		RetryPending:   s.retryPending,
		RetryRemaining: s.retryRemaining,
	}
	if !s.lastRunAt.IsZero() {
		state.LastRunAt = s.lastRunAt.UTC().Format(time.RFC3339)
	}
	if !s.nextRunAt.IsZero() {
		state.NextRunAt = s.nextRunAt.UTC().Format(time.RFC3339)
	}
	if s.lastResult != nil {
		resultCopy := *s.lastResult
		state.LastResult = &resultCopy
	}
	return state
}

func (s *OnlineAccountsScheduler) loadLatestResult(ctx context.Context) (*OnlineAccountsRunResult, error) {
	if s.store == nil || !s.store.Available() {
		return nil, nil
	}
	logEntry, err := s.store.GetLatestOnlineAccountSchedulerLog(ctx)
	if err != nil || logEntry == nil {
		return nil, err
	}
	return &OnlineAccountsRunResult{
		StartedAt:     logEntry.StartedAt,
		FinishedAt:    logEntry.FinishedAt,
		TriggerType:   logEntry.TriggerType,
		Status:        logEntry.Status,
		Attempt:       logEntry.Attempt,
		MaxAttempts:   logEntry.MaxAttempts,
		Actions:       append([]string(nil), logEntry.Actions...),
		InvalidFound:  logEntry.InvalidFound,
		DisabledCount: logEntry.DisabledCount,
		DeletedCount:  logEntry.DeletedCount,
		FailedCount:   logEntry.FailedCount,
		Messages:      append([]string(nil), logEntry.Messages...),
		Error:         logEntry.ErrorMessage,
	}, nil
}

func (s *OnlineAccountsScheduler) persistRunLog(ctx context.Context, result OnlineAccountsRunResult, cfg OnlineAccountsScheduleConfig) error {
	if s.store == nil || !s.store.Available() {
		return nil
	}
	_, err := s.store.CreateOnlineAccountSchedulerLog(ctx, store.OnlineAccountSchedulerLog{
		TriggerType:   result.TriggerType,
		Status:        result.Status,
		Attempt:       result.Attempt,
		MaxAttempts:   result.MaxAttempts,
		ScheduleMode:  cfg.Mode,
		Actions:       append([]string(nil), result.Actions...),
		InvalidFound:  result.InvalidFound,
		DisabledCount: result.DisabledCount,
		DeletedCount:  result.DeletedCount,
		FailedCount:   result.FailedCount,
		ErrorMessage:  result.Error,
		Messages:      append([]string(nil), result.Messages...),
		StartedAt:     result.StartedAt,
		FinishedAt:    result.FinishedAt,
	})
	return err
}

func ExecuteOnlineAccountsMaintenance(ctx context.Context, cfg OnlineAccountsScheduleConfig) (OnlineAccountsRunResult, error) {
	cfg = cfg.Normalized()

	result := OnlineAccountsRunResult{
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Actions:   make([]string, 0, 2),
		Messages:  []string{},
	}
	if cfg.DeleteInvalid {
		result.Actions = append(result.Actions, "delete_invalid")
	}
	if !cfg.HasActions() {
		result.Messages = append(result.Messages, "未配置任何可执行动作")
		result.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		return result, errors.New("至少选择一个定时动作")
	}

	managementCfg, err := loadOnlineAccountsManagementConfig()
	if err != nil {
		result.Messages = append(result.Messages, err.Error())
		result.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		return result, err
	}

	files, err := fetchManagementAuthFiles(ctx, managementCfg)
	if err != nil {
		result.Messages = append(result.Messages, err.Error())
		result.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		return result, err
	}

	invalidFiles := make([]managementAuthFile, 0)
	for _, file := range files {
		if isTokenInvalidManagementFile(file) {
			invalidFiles = append(invalidFiles, file)
		}
	}
	result.InvalidFound = len(invalidFiles)

	for _, file := range invalidFiles {
		if cfg.DeleteInvalid {
			if err := deleteManagementAuthFile(ctx, managementCfg, file); err != nil {
				result.FailedCount++
				result.Messages = append(result.Messages, fmt.Sprintf("删除 %s 失败: %v", onlineAccountLabel(file), err))
			} else {
				result.DeletedCount++
			}
		}
	}

	if result.InvalidFound == 0 {
		result.Messages = append(result.Messages, "未发现 Token 失效账号")
	}
	if len(result.Messages) == 0 {
		result.Messages = append(result.Messages, fmt.Sprintf(
			"扫描到 %d 个 Token 失效账号，已禁用 %d 个，已删除 %d 个",
			result.InvalidFound,
			result.DisabledCount,
			result.DeletedCount,
		))
	}

	result.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

func nextRegularRun(base time.Time, cfg OnlineAccountsScheduleConfig) (time.Time, string) {
	cfg = cfg.Normalized()
	if !cfg.Enabled || !cfg.HasActions() {
		return time.Time{}, ""
	}

	if cfg.Mode == OnlineAccountsScheduleModeFixedTimes {
		return nextFixedTimeAfter(base, cfg.FixedTimes), OnlineAccountsScheduleModeFixedTimes
	}
	return base.UTC().Add(time.Duration(cfg.IntervalMins) * time.Minute), OnlineAccountsScheduleModeInterval
}

func nextFixedTimeAfter(base time.Time, fixedTimes []string) time.Time {
	localBase := base.In(time.Local)
	best := time.Time{}
	for _, fixedTime := range normalizeFixedTimes(fixedTimes) {
		hour, minute, ok := parseFixedClock(fixedTime)
		if !ok {
			continue
		}
		candidate := time.Date(localBase.Year(), localBase.Month(), localBase.Day(), hour, minute, 0, 0, time.Local)
		if !candidate.After(localBase) {
			candidate = candidate.Add(24 * time.Hour)
		}
		if best.IsZero() || candidate.Before(best) {
			best = candidate
		}
	}
	return best.UTC()
}

func normalizeFixedTimes(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		hour, minute, ok := parseFixedClock(value)
		if !ok {
			continue
		}
		canonical := fmt.Sprintf("%02d:%02d", hour, minute)
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	sort.Strings(result)
	return result
}

func parseFixedClock(raw string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, false
	}
	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, false
	}
	return hour, minute, true
}

func deriveRunStatus(result OnlineAccountsRunResult, err error) string {
	if err == nil && result.FailedCount == 0 {
		return onlineAccountsRunStatusSuccess
	}
	if result.DisabledCount > 0 || result.DeletedCount > 0 || (result.InvalidFound > 0 && result.FailedCount < result.InvalidFound) {
		return onlineAccountsRunStatusPartialFailed
	}
	return onlineAccountsRunStatusFailed
}

func loadOnlineAccountsManagementConfig() (onlineAccountsManagementConfig, error) {
	rawURL := firstEnv("APP_CPA_API_URL", "CPA_API_URL", "VITE_CPA_API_URL")
	token := firstEnv("APP_CPA_API_TOKEN", "CPA_API_TOKEN", "VITE_CPA_API_TOKEN")
	proxyURL := firstEnv("APP_CPA_PROXY_URL", "CPA_PROXY_URL")

	endpoint := normalizeOnlineAccountsManagementEndpoint(rawURL)
	if endpoint == "" {
		return onlineAccountsManagementConfig{}, errors.New("CPA API URL 未配置，无法执行线上账号定时任务")
	}
	if strings.TrimSpace(token) == "" {
		return onlineAccountsManagementConfig{}, errors.New("CPA API Token 未配置，无法执行线上账号定时任务")
	}

	return onlineAccountsManagementConfig{
		Endpoint: endpoint,
		Token:    strings.TrimSpace(token),
		ProxyURL: strings.TrimSpace(proxyURL),
	}, nil
}

func normalizeOnlineAccountsManagementEndpoint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
		return ""
	}

	pathname := strings.TrimRight(parsed.EscapedPath(), "/")
	if pathname == "" {
		return parsed.Scheme + "://" + parsed.Host + "/v0/management/auth-files"
	}

	if parsed.RawQuery != "" {
		return parsed.Scheme + "://" + parsed.Host + pathname + "?" + parsed.RawQuery
	}
	return parsed.Scheme + "://" + parsed.Host + pathname
}

func fetchManagementAuthFiles(ctx context.Context, cfg onlineAccountsManagementConfig) ([]managementAuthFile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.Endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("User-Agent", userAgent())

	resp, err := onlineAccountsHTTPClientFactory(cfg.ProxyURL, onlineAccountsListTimeout()).Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取线上账号列表失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("读取线上账号列表失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取线上账号列表失败: HTTP %d", resp.StatusCode)
	}

	var payload managementAuthFilesResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析线上账号列表失败: %w", err)
	}
	return payload.Files, nil
}

func deleteManagementAuthFile(ctx context.Context, cfg onlineAccountsManagementConfig, file managementAuthFile) error {
	deleteURL := cfg.Endpoint + "?name=" + url.QueryEscape(file.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("User-Agent", userAgent())

	resp, err := onlineAccountsHTTPClientFactory(cfg.ProxyURL, onlineAccountsMutationTimeout()).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func isTokenInvalidManagementFile(file managementAuthFile) bool {
	raw := strings.ToLower(string(file.StatusMessage))
	if raw == "" {
		return false
	}
	for _, marker := range onlineAccountsTokenInvalidMarkers {
		if strings.Contains(raw, marker) {
			return true
		}
	}
	return false
}

func onlineAccountLabel(file managementAuthFile) string {
	for _, candidate := range []string{file.Account, file.Email, file.Name, file.ID} {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return "unknown"
}

func parseRFC3339(raw string) (time.Time, error) {
	return time.Parse(time.RFC3339, strings.TrimSpace(raw))
}

func onlineAccountsListTimeout() time.Duration {
	return time.Duration(envInt("APP_ONLINE_ACCOUNTS_LIST_TIMEOUT_SECONDS", 120)) * time.Second
}

func onlineAccountsMutationTimeout() time.Duration {
	return time.Duration(envInt("APP_ONLINE_ACCOUNTS_MUTATION_TIMEOUT_SECONDS", 60)) * time.Second
}
