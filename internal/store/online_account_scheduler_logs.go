package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type OnlineAccountSchedulerLog struct {
	ID            int      `json:"id"`
	TriggerType   string   `json:"trigger_type"`
	Status        string   `json:"status"`
	Attempt       int      `json:"attempt"`
	MaxAttempts   int      `json:"max_attempts"`
	ScheduleMode  string   `json:"schedule_mode,omitempty"`
	Actions       []string `json:"actions"`
	InvalidFound  int      `json:"invalid_found"`
	DisabledCount int      `json:"disabled_count"`
	DeletedCount  int      `json:"deleted_count"`
	FailedCount   int      `json:"failed_count"`
	ErrorMessage  string   `json:"error_message,omitempty"`
	Messages      []string `json:"messages,omitempty"`
	StartedAt     string   `json:"started_at,omitempty"`
	FinishedAt    string   `json:"finished_at,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
}

type OnlineAccountSchedulerLogListResult struct {
	Total int                         `json:"total"`
	Logs  []OnlineAccountSchedulerLog `json:"logs"`
}

func (s *SQLiteStore) CreateOnlineAccountSchedulerLog(ctx context.Context, entry OnlineAccountSchedulerLog) (*OnlineAccountSchedulerLog, error) {
	if !s.Available() {
		return nil, nil
	}

	exists, err := s.tableExists(ctx, "online_account_scheduler_logs")
	if err != nil || !exists {
		return nil, err
	}

	actionsJSON, err := json.Marshal(entry.Actions)
	if err != nil {
		return nil, fmt.Errorf("marshal scheduler log actions: %w", err)
	}
	messagesJSON, err := json.Marshal(entry.Messages)
	if err != nil {
		return nil, fmt.Errorf("marshal scheduler log messages: %w", err)
	}

	result, err := s.db.ExecContext(ctx, `
INSERT INTO online_account_scheduler_logs (
	trigger_type, status, attempt, max_attempts, schedule_mode, actions,
	invalid_found, disabled_count, deleted_count, failed_count, error_message, messages,
	started_at, finished_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.TriggerType,
		entry.Status,
		max(entry.Attempt, 1),
		max(entry.MaxAttempts, 1),
		strings.TrimSpace(entry.ScheduleMode),
		string(actionsJSON),
		entry.InvalidFound,
		entry.DisabledCount,
		entry.DeletedCount,
		entry.FailedCount,
		strings.TrimSpace(entry.ErrorMessage),
		string(messagesJSON),
		nullableTrimmedString(entry.StartedAt),
		nullableTrimmedString(entry.FinishedAt),
	)
	if err != nil {
		return nil, fmt.Errorf("insert scheduler log: %w", err)
	}

	insertID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read scheduler log id: %w", err)
	}

	return s.GetOnlineAccountSchedulerLogByID(ctx, int(insertID))
}

func (s *SQLiteStore) GetOnlineAccountSchedulerLogByID(ctx context.Context, id int) (*OnlineAccountSchedulerLog, error) {
	if !s.Available() {
		return nil, nil
	}

	exists, err := s.tableExists(ctx, "online_account_scheduler_logs")
	if err != nil || !exists {
		return nil, err
	}

	const query = `
SELECT
	id, trigger_type, status, attempt, max_attempts, schedule_mode, actions,
	invalid_found, disabled_count, deleted_count, failed_count, error_message, messages,
	started_at, finished_at, created_at
FROM online_account_scheduler_logs
WHERE id = ?`

	logEntry, err := scanOnlineAccountSchedulerLog(s.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &logEntry, nil
}

func (s *SQLiteStore) GetLatestOnlineAccountSchedulerLog(ctx context.Context) (*OnlineAccountSchedulerLog, error) {
	if !s.Available() {
		return nil, nil
	}

	exists, err := s.tableExists(ctx, "online_account_scheduler_logs")
	if err != nil || !exists {
		return nil, err
	}

	const query = `
SELECT
	id, trigger_type, status, attempt, max_attempts, schedule_mode, actions,
	invalid_found, disabled_count, deleted_count, failed_count, error_message, messages,
	started_at, finished_at, created_at
FROM online_account_scheduler_logs
ORDER BY datetime(created_at) DESC, id DESC
LIMIT 1`

	logEntry, err := scanOnlineAccountSchedulerLog(s.db.QueryRowContext(ctx, query))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &logEntry, nil
}

func (s *SQLiteStore) ListOnlineAccountSchedulerLogs(ctx context.Context, page, pageSize int) (OnlineAccountSchedulerLogListResult, error) {
	result := OnlineAccountSchedulerLogListResult{Logs: []OnlineAccountSchedulerLog{}}
	if !s.Available() {
		return result, nil
	}

	exists, err := s.tableExists(ctx, "online_account_scheduler_logs")
	if err != nil || !exists {
		return result, err
	}

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM online_account_scheduler_logs`).Scan(&result.Total); err != nil {
		return result, fmt.Errorf("count scheduler logs: %w", err)
	}

	offset := max(page-1, 0) * pageSize
	const query = `
SELECT
	id, trigger_type, status, attempt, max_attempts, schedule_mode, actions,
	invalid_found, disabled_count, deleted_count, failed_count, error_message, messages,
	started_at, finished_at, created_at
FROM online_account_scheduler_logs
ORDER BY datetime(created_at) DESC, id DESC
LIMIT ? OFFSET ?`

	rows, err := s.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return result, fmt.Errorf("query scheduler logs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		logEntry, err := scanOnlineAccountSchedulerLog(rows)
		if err != nil {
			return result, err
		}
		result.Logs = append(result.Logs, logEntry)
	}

	return result, rows.Err()
}

func scanOnlineAccountSchedulerLog(row scanner) (OnlineAccountSchedulerLog, error) {
	var (
		entry        OnlineAccountSchedulerLog
		scheduleMode sql.NullString
		actionsJSON  sql.NullString
		errorMessage sql.NullString
		messagesJSON sql.NullString
		startedAt    sql.NullString
		finishedAt   sql.NullString
		createdAt    sql.NullString
	)

	err := row.Scan(
		&entry.ID,
		&entry.TriggerType,
		&entry.Status,
		&entry.Attempt,
		&entry.MaxAttempts,
		&scheduleMode,
		&actionsJSON,
		&entry.InvalidFound,
		&entry.DisabledCount,
		&entry.DeletedCount,
		&entry.FailedCount,
		&errorMessage,
		&messagesJSON,
		&startedAt,
		&finishedAt,
		&createdAt,
	)
	if err != nil {
		return entry, err
	}

	entry.ScheduleMode = scheduleMode.String
	entry.ErrorMessage = errorMessage.String
	entry.StartedAt = startedAt.String
	entry.FinishedAt = finishedAt.String
	entry.CreatedAt = createdAt.String

	if actionsJSON.Valid && strings.TrimSpace(actionsJSON.String) != "" {
		_ = json.Unmarshal([]byte(actionsJSON.String), &entry.Actions)
	}
	if messagesJSON.Valid && strings.TrimSpace(messagesJSON.String) != "" {
		_ = json.Unmarshal([]byte(messagesJSON.String), &entry.Messages)
	}
	if entry.Actions == nil {
		entry.Actions = []string{}
	}
	if entry.Messages == nil {
		entry.Messages = []string{}
	}

	return entry, nil
}

func nullableTrimmedString(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
