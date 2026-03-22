package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	xproxy "golang.org/x/net/proxy"

	"codex-register/internal/store"
)

type StartRequest struct {
	EmailServiceType   string         `json:"email_service_type"`
	Proxy              string         `json:"proxy,omitempty"`
	EmailServiceConfig map[string]any `json:"email_service_config,omitempty"`
	EmailServiceID     *int           `json:"email_service_id,omitempty"`
}

type BatchRequest struct {
	Count              int            `json:"count"`
	EmailServiceType   string         `json:"email_service_type"`
	Proxy              string         `json:"proxy,omitempty"`
	EmailServiceConfig map[string]any `json:"email_service_config,omitempty"`
	EmailServiceID     *int           `json:"email_service_id,omitempty"`
	IntervalMin        int            `json:"interval_min"`
	IntervalMax        int            `json:"interval_max"`
	Concurrency        int            `json:"concurrency"`
}

type BatchStatus struct {
	BatchID      string   `json:"batch_id"`
	Total        int      `json:"total"`
	Completed    int      `json:"completed"`
	Success      int      `json:"success"`
	Failed       int      `json:"failed"`
	CurrentIndex int      `json:"current_index"`
	Cancelled    bool     `json:"cancelled"`
	Finished     bool     `json:"finished"`
	Status       string   `json:"status"`
	Logs         []string `json:"logs,omitempty"`
}

type TaskEvent struct {
	Type      string         `json:"type"`
	TaskUUID  string         `json:"task_uuid,omitempty"`
	BatchID   string         `json:"batch_id,omitempty"`
	Status    string         `json:"status,omitempty"`
	Message   string         `json:"message,omitempty"`
	Timestamp string         `json:"timestamp"`
	Extra     map[string]any `json:"extra,omitempty"`
}

type TaskManager struct {
	db               *store.SQLiteStore
	mu               sync.Mutex
	cancels          map[string]context.CancelFunc
	taskSubscribers  map[string]map[chan TaskEvent]struct{}
	batchSubscribers map[string]map[chan TaskEvent]struct{}
	batches          map[string]*BatchStatus
}

const batchPhoneVerificationRetryLimit = 4

func NewTaskManager(db *store.SQLiteStore) *TaskManager {
	return &TaskManager{
		db:               db,
		cancels:          map[string]context.CancelFunc{},
		taskSubscribers:  map[string]map[chan TaskEvent]struct{}{},
		batchSubscribers: map[string]map[chan TaskEvent]struct{}{},
		batches:          map[string]*BatchStatus{},
	}
}

func (m *TaskManager) StartRegistration(ctx context.Context, request StartRequest) (*store.RegistrationTask, error) {
	taskUUID := uuid.NewString()
	proxy := nullableTrimmed(request.Proxy)
	task, err := m.db.CreateRegistrationTask(ctx, taskUUID, proxy, request.EmailServiceID)
	if err != nil {
		return nil, err
	}

	taskCtx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancels[taskUUID] = cancel
	m.mu.Unlock()

	go m.runTask(taskCtx, "", taskUUID, request)
	return task, nil
}

func (m *TaskManager) StartBatch(ctx context.Context, request BatchRequest) (string, []*store.RegistrationTask, error) {
	if request.Count < 1 {
		request.Count = 1
	}
	if request.IntervalMax < request.IntervalMin {
		request.IntervalMax = request.IntervalMin
	}
	if request.Concurrency < 1 {
		request.Concurrency = 1
	}
	if request.Concurrency > request.Count {
		request.Concurrency = request.Count
	}

	batchID := uuid.NewString()
	tasks := make([]*store.RegistrationTask, 0, request.Count)
	for i := 0; i < request.Count; i++ {
		taskUUID := uuid.NewString()
		proxy := nullableTrimmed(request.Proxy)
		task, err := m.db.CreateRegistrationTask(ctx, taskUUID, proxy, request.EmailServiceID)
		if err != nil {
			return "", nil, err
		}
		tasks = append(tasks, task)
	}

	m.mu.Lock()
	m.batches[batchID] = &BatchStatus{
		BatchID: batchID,
		Total:   len(tasks),
		Status:  "running",
		Logs:    []string{},
	}
	m.mu.Unlock()
	m.publishBatchEvent(batchID, TaskEvent{
		Type:      "status",
		BatchID:   batchID,
		Status:    "running",
		Timestamp: time.Now().Format(time.RFC3339),
		Extra: map[string]any{
			"total":         len(tasks),
			"completed":     0,
			"success":       0,
			"failed":        0,
			"current_index": 0,
			"finished":      false,
			"cancelled":     false,
			"concurrency":   request.Concurrency,
		},
	})

	go func() {
		jobs := make(chan *store.RegistrationTask)
		var wg sync.WaitGroup

		worker := func() {
			defer wg.Done()
			for task := range jobs {
				taskCtx, cancel := context.WithCancel(context.Background())
				m.mu.Lock()
				m.cancels[task.TaskUUID] = cancel
				m.mu.Unlock()

				m.runTask(taskCtx, batchID, task.TaskUUID, StartRequest{
					EmailServiceType:   request.EmailServiceType,
					Proxy:              request.Proxy,
					EmailServiceConfig: request.EmailServiceConfig,
					EmailServiceID:     request.EmailServiceID,
				})

				taskState, _ := m.db.GetRegistrationTaskByUUID(context.Background(), task.TaskUUID)
				m.updateBatch(batchID, func(status *BatchStatus) {
					status.Completed++
					if taskState != nil && taskState.Status == "completed" {
						status.Success++
					} else {
						status.Failed++
					}
				})
				if taskState != nil && taskState.Status == "completed" {
					m.AppendBatchLog(batchID, fmt.Sprintf("[成功] %s", task.TaskUUID))
				} else if taskState != nil && taskState.ErrorMessage != nil {
					m.AppendBatchLog(batchID, fmt.Sprintf("[失败] %s: %s", task.TaskUUID, *taskState.ErrorMessage))
				}
			}
		}

		for i := 0; i < request.Concurrency; i++ {
			wg.Add(1)
			go worker()
		}

		dispatched := 0
		for index, task := range tasks {
			if m.IsBatchCancelled(batchID) {
				m.AppendBatchLog(batchID, "[取消] 批量任务已取消：停止派发新任务，等待在跑任务结束")
				break
			}

			m.updateBatch(batchID, func(status *BatchStatus) {
				status.CurrentIndex = index
			})
			m.AppendBatchLog(batchID, fmt.Sprintf("[派发] 任务 %d/%d: %s", index+1, len(tasks), task.TaskUUID))

			jobs <- task
			dispatched++

			if index < len(tasks)-1 {
				sleepFor := request.IntervalMin
				if request.IntervalMax > request.IntervalMin {
					sleepFor += randInt(request.IntervalMax - request.IntervalMin + 1)
				}
				if sleepFor > 0 {
					m.AppendBatchLog(batchID, fmt.Sprintf("[等待] %d 秒后继续派发", sleepFor))
					time.Sleep(time.Duration(sleepFor) * time.Second)
				}
			}
		}

		close(jobs)
		wg.Wait()

		if m.IsBatchCancelled(batchID) {
			m.AppendBatchLog(batchID, fmt.Sprintf("[取消] 批量任务已取消（已派发 %d/%d）", dispatched, len(tasks)))
			m.finishBatch(batchID, "cancelled", true)
			return
		}

		m.AppendBatchLog(batchID, "[完成] 批量任务完成")
		m.finishBatch(batchID, "completed", false)
	}()

	return batchID, tasks, nil
}

func (m *TaskManager) CancelTask(taskUUID string) bool {
	m.mu.Lock()
	cancel := m.cancels[taskUUID]
	m.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (m *TaskManager) CancelBatch(batchID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	batch := m.batches[batchID]
	if batch == nil || batch.Cancelled || batch.Finished {
		return false
	}
	batch.Cancelled = true
	batch.Status = "cancelling"
	m.publishBatchEventLocked(batchID, TaskEvent{
		Type:      "status",
		BatchID:   batchID,
		Status:    "cancelling",
		Timestamp: time.Now().Format(time.RFC3339),
		Extra:     m.batchExtra(batch),
	})
	return true
}

func (m *TaskManager) IsBatchCancelled(batchID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	batch := m.batches[batchID]
	return batch != nil && batch.Cancelled
}

func (m *TaskManager) GetBatch(batchID string) *BatchStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	batch := m.batches[batchID]
	if batch == nil {
		return nil
	}
	copyBatch := *batch
	copyBatch.Logs = append([]string{}, batch.Logs...)
	return &copyBatch
}

func (m *TaskManager) runTask(ctx context.Context, batchID, taskUUID string, request StartRequest) {
	defer func() {
		m.mu.Lock()
		delete(m.cancels, taskUUID)
		m.mu.Unlock()
	}()

	shortID := taskUUID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	logf := func(message string) {
		timestamped := "[" + time.Now().Format("15:04:05") + "] [" + shortID + "] " + message
		_ = m.db.AppendRegistrationTaskLog(context.Background(), taskUUID, timestamped)
		m.publishTaskEvent(taskUUID, TaskEvent{
			Type:      "log",
			TaskUUID:  taskUUID,
			Message:   timestamped,
			Timestamp: time.Now().Format(time.RFC3339),
		})
		if strings.TrimSpace(batchID) != "" {
			m.AppendBatchLog(batchID, timestamped)
		}
	}

	_, _ = m.db.UpdateRegistrationTask(context.Background(), taskUUID, map[string]any{
		"status":     "running",
		"started_at": time.Now().Format(time.RFC3339),
	})
	m.publishTaskEvent(taskUUID, TaskEvent{
		Type:      "status",
		TaskUUID:  taskUUID,
		Status:    "running",
		Timestamp: time.Now().Format(time.RFC3339),
	})

	settings, err := loadEngineSettings(context.Background(), m.db)
	if err != nil {
		_, _ = m.db.UpdateRegistrationTask(context.Background(), taskUUID, map[string]any{
			"status":        "failed",
			"completed_at":  time.Now().Format(time.RFC3339),
			"error_message": err.Error(),
		})
		m.publishTaskEvent(taskUUID, TaskEvent{
			Type:      "status",
			TaskUUID:  taskUUID,
			Status:    "failed",
			Message:   err.Error(),
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	service, selectedServiceID, err := m.selectEmailService(context.Background(), request)
	if err != nil {
		_, _ = m.db.UpdateRegistrationTask(context.Background(), taskUUID, map[string]any{
			"status":        "failed",
			"completed_at":  time.Now().Format(time.RFC3339),
			"error_message": err.Error(),
		})
		m.publishTaskEvent(taskUUID, TaskEvent{
			Type:      "status",
			TaskUUID:  taskUUID,
			Status:    "failed",
			Message:   err.Error(),
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}
	if selectedServiceID != nil {
		_, _ = m.db.UpdateRegistrationTask(context.Background(), taskUUID, map[string]any{
			"email_service_id": selectedServiceID,
		})
	}

	resolvedProxy := ResolveRegistrationProxy(ctx, request.Proxy, settings, logf)
	if resolvedProxy != "" {
		logf("使用代理: " + maskProxyForLog(resolvedProxy))
		_, _ = m.db.UpdateRegistrationTask(context.Background(), taskUUID, map[string]any{
			"proxy": resolvedProxy,
		})
	}

	maxPhoneRetries := 0
	if strings.TrimSpace(batchID) != "" {
		maxPhoneRetries = batchPhoneVerificationRetryLimit
	}

	var result RegistrationResult
	for attempt := 1; attempt <= maxPhoneRetries+1; attempt++ {
		if attempt > 1 {
			logf(fmt.Sprintf("检测到 add-phone，自动更换邮箱重试 (%d/%d)", attempt-1, maxPhoneRetries))
		}

		engine := newRegistrationEngine(settings, service, resolvedProxy, logf)
		result = engine.run(ctx)
		if result.Success {
			if attempt > 1 {
				if result.Metadata == nil {
					result.Metadata = map[string]any{}
				}
				result.Metadata["phone_retry_count"] = attempt - 1
			}
			break
		}
		if !shouldRetryPhoneVerification(result, attempt, maxPhoneRetries) {
			break
		}
		logf(fmt.Sprintf("当前邮箱触发 add-phone：%s，准备丢弃并换号", result.Email))
	}

	if !result.Success {
		_, _ = m.db.UpdateRegistrationTask(context.Background(), taskUUID, map[string]any{
			"status":        "failed",
			"completed_at":  time.Now().Format(time.RFC3339),
			"error_message": result.ErrorMessage,
			"result":        result,
		})
		m.publishTaskEvent(taskUUID, TaskEvent{
			Type:      "status",
			TaskUUID:  taskUUID,
			Status:    "failed",
			Message:   result.ErrorMessage,
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	accountID, err := m.db.CreateAccount(context.Background(), store.AccountCreate{
		Email:        result.Email,
		Password:     result.Password,
		ClientID:     settings.OpenAIClientID,
		SessionToken: result.SessionToken,
		EmailService: service.Type(),
		AccountID:    result.AccountID,
		WorkspaceID:  result.WorkspaceID,
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		IDToken:      result.IDToken,
		ProxyUsed:    resolvedProxy,
		ExtraData:    result.Metadata,
		Source:       result.Source,
	})
	if err != nil {
		logf("账号入库失败: " + err.Error())
	} else {
		logf(fmt.Sprintf("账号已入库: %s (ID %d)", result.Email, accountID))
		cpaConfig := ResolveCPAConfig(context.Background(), m.db)
		if cpaConfig.Enabled {
			logf("开始自动上传 CPA")
			success, message := UploadAccountToCPA(context.Background(), m.db, accountID, cpaConfig.APIURL, cpaConfig.APIToken, cpaConfig.ProxyURL)
			if success {
				logf("CPA 已上传")
			} else {
				logf("CPA 自动上传失败: " + message)
			}
		} else {
			logf("CPA 未启用，跳过上传")
		}
	}

	resultExtra := buildRegistrationResultEvent(taskUUID, accountID, result)
	m.publishTaskEvent(taskUUID, TaskEvent{
		Type:      "result",
		TaskUUID:  taskUUID,
		BatchID:   batchID,
		Timestamp: time.Now().Format(time.RFC3339),
		Extra:     resultExtra,
	})
	if strings.TrimSpace(batchID) != "" {
		m.publishBatchEvent(batchID, TaskEvent{
			Type:      "result",
			TaskUUID:  taskUUID,
			BatchID:   batchID,
			Timestamp: time.Now().Format(time.RFC3339),
			Extra:     resultExtra,
		})
	}

	_, _ = m.db.UpdateRegistrationTask(context.Background(), taskUUID, map[string]any{
		"status":       "completed",
		"completed_at": time.Now().Format(time.RFC3339),
		"result":       result,
	})
	m.publishTaskEvent(taskUUID, TaskEvent{
		Type:      "status",
		TaskUUID:  taskUUID,
		Status:    "completed",
		Timestamp: time.Now().Format(time.RFC3339),
		Extra:     map[string]any{"email": result.Email},
	})
}

func shouldRetryPhoneVerification(result RegistrationResult, attempt, maxRetries int) bool {
	if attempt > maxRetries || maxRetries <= 0 {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(result.ErrorMessage))
	if message == "" {
		return false
	}
	return strings.Contains(message, "phone verification required") || strings.Contains(message, "/add-phone")
}

func (m *TaskManager) selectEmailService(ctx context.Context, request StartRequest) (EmailService, *int, error) {
	_ = ctx

	switch strings.TrimSpace(request.EmailServiceType) {
	case "", "tempmail", "temp-email", "meteormail":
		return newTempMailService(strings.TrimSpace(request.Proxy)), nil, nil
	default:
		return nil, nil, fmt.Errorf("unsupported email service type: %s", request.EmailServiceType)
	}
}

func newHTTPClient(proxyURL string, timeout time.Duration, followRedirects bool) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if strings.TrimSpace(proxyURL) != "" {
		if parsedProxy, err := url.Parse(proxyURL); err == nil {
			switch strings.ToLower(parsedProxy.Scheme) {
			case "socks5", "socks5h":
				auth := &xproxy.Auth{}
				if parsedProxy.User != nil {
					auth.User = parsedProxy.User.Username()
					auth.Password, _ = parsedProxy.User.Password()
				}
				dialer, dialErr := xproxy.SOCKS5("tcp", parsedProxy.Host, auth, &net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				})
				if dialErr == nil {
					transport.Proxy = nil
					transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return dialer.Dial(network, addr)
					}
				}
			default:
				transport.Proxy = http.ProxyURL(parsedProxy)
			}
		}
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
	if !followRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client
}

func newSessionClient(proxyURL string, jar http.CookieJar, stopOnRedirect bool) *http.Client {
	client := newHTTPClient(proxyURL, 30*time.Second, !stopOnRedirect)
	client.Jar = jar
	return client
}

func sha256Base64URL(value string) string {
	sum := sha256.Sum256([]byte(value))
	return strings.TrimRight(base64.URLEncoding.EncodeToString(sum[:]), "=")
}

func userAgent() string {
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
}

func nullableTrimmed(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	return int(time.Now().UnixNano() % int64(n))
}

func buildRegistrationResultEvent(taskUUID string, accountDBID int, result RegistrationResult) map[string]any {
	extra := map[string]any{
		"task_uuid":    taskUUID,
		"email":        result.Email,
		"account_id":   result.AccountID,
		"workspace_id": result.WorkspaceID,
		"source":       result.Source,
	}
	if accountDBID > 0 {
		extra["account_db_id"] = accountDBID
	}
	if strings.TrimSpace(result.BindCardURL) != "" {
		extra["bind_card_url"] = result.BindCardURL
		extra["bind_card_url_summary"] = summarizeValue(result.BindCardURL, 88)
	}
	return extra
}

func summarizeValue(value string, keep int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if keep < 16 {
		keep = 16
	}
	if len(trimmed) <= keep {
		return trimmed
	}
	return trimmed[:keep] + "..."
}

func maskProxyForLog(proxyURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil || parsed.User == nil {
		return proxyURL
	}

	username := parsed.User.Username()
	if username == "" {
		return parsed.Scheme + "://" + parsed.Host
	}

	parsed.User = url.UserPassword(username, "****")
	return parsed.String()
}

func newCookieJar() http.CookieJar {
	jar, _ := cookiejar.New(nil)
	return jar
}

func (m *TaskManager) SubscribeTask(taskUUID string) chan TaskEvent {
	ch := make(chan TaskEvent, 64)
	m.mu.Lock()
	if m.taskSubscribers[taskUUID] == nil {
		m.taskSubscribers[taskUUID] = map[chan TaskEvent]struct{}{}
	}
	m.taskSubscribers[taskUUID][ch] = struct{}{}
	m.mu.Unlock()
	return ch
}

func (m *TaskManager) UnsubscribeTask(taskUUID string, ch chan TaskEvent) {
	m.mu.Lock()
	if subscribers := m.taskSubscribers[taskUUID]; subscribers != nil {
		delete(subscribers, ch)
		if len(subscribers) == 0 {
			delete(m.taskSubscribers, taskUUID)
		}
	}
	m.mu.Unlock()
	close(ch)
}

func (m *TaskManager) SubscribeBatch(batchID string) chan TaskEvent {
	ch := make(chan TaskEvent, 64)
	m.mu.Lock()
	if m.batchSubscribers[batchID] == nil {
		m.batchSubscribers[batchID] = map[chan TaskEvent]struct{}{}
	}
	m.batchSubscribers[batchID][ch] = struct{}{}
	m.mu.Unlock()
	return ch
}

func (m *TaskManager) UnsubscribeBatch(batchID string, ch chan TaskEvent) {
	m.mu.Lock()
	if subscribers := m.batchSubscribers[batchID]; subscribers != nil {
		delete(subscribers, ch)
		if len(subscribers) == 0 {
			delete(m.batchSubscribers, batchID)
		}
	}
	m.mu.Unlock()
	close(ch)
}

func (m *TaskManager) publishTaskEvent(taskUUID string, event TaskEvent) {
	m.mu.Lock()
	subscribers := m.taskSubscribers[taskUUID]
	channels := make([]chan TaskEvent, 0, len(subscribers))
	for ch := range subscribers {
		channels = append(channels, ch)
	}
	m.mu.Unlock()
	for _, ch := range channels {
		select {
		case ch <- event:
		default:
		}
	}
}

func (m *TaskManager) publishBatchEvent(batchID string, event TaskEvent) {
	m.mu.Lock()
	m.publishBatchEventLocked(batchID, event)
	m.mu.Unlock()
}

func (m *TaskManager) publishBatchEventLocked(batchID string, event TaskEvent) {
	subscribers := m.batchSubscribers[batchID]
	for ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (m *TaskManager) updateBatch(batchID string, update func(*BatchStatus)) {
	m.mu.Lock()
	batch := m.batches[batchID]
	if batch == nil {
		m.mu.Unlock()
		return
	}
	update(batch)
	extra := m.batchExtra(batch)
	m.publishBatchEventLocked(batchID, TaskEvent{
		Type:      "status",
		BatchID:   batchID,
		Status:    batch.Status,
		Timestamp: time.Now().Format(time.RFC3339),
		Extra:     extra,
	})
	m.mu.Unlock()
}

func (m *TaskManager) finishBatch(batchID, status string, cancelled bool) {
	m.mu.Lock()
	batch := m.batches[batchID]
	if batch == nil {
		m.mu.Unlock()
		return
	}
	batch.Status = status
	batch.Cancelled = cancelled
	batch.Finished = true
	extra := m.batchExtra(batch)
	m.publishBatchEventLocked(batchID, TaskEvent{
		Type:      "status",
		BatchID:   batchID,
		Status:    status,
		Timestamp: time.Now().Format(time.RFC3339),
		Extra:     extra,
	})
	m.mu.Unlock()
}

func (m *TaskManager) AppendBatchLog(batchID, message string) {
	m.mu.Lock()
	batch := m.batches[batchID]
	if batch == nil {
		m.mu.Unlock()
		return
	}
	batch.Logs = append(batch.Logs, message)
	m.publishBatchEventLocked(batchID, TaskEvent{
		Type:      "log",
		BatchID:   batchID,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	})
	m.mu.Unlock()
}

func (m *TaskManager) batchExtra(batch *BatchStatus) map[string]any {
	return map[string]any{
		"total":         batch.Total,
		"completed":     batch.Completed,
		"success":       batch.Success,
		"failed":        batch.Failed,
		"current_index": batch.CurrentIndex,
		"finished":      batch.Finished,
		"cancelled":     batch.Cancelled,
	}
}
