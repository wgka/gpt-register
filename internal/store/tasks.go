package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type RegistrationTask struct {
	ID             int            `json:"id"`
	TaskUUID       string         `json:"task_uuid"`
	Status         string         `json:"status"`
	EmailServiceID *int           `json:"email_service_id,omitempty"`
	Proxy          *string        `json:"proxy,omitempty"`
	Logs           string         `json:"logs,omitempty"`
	Result         map[string]any `json:"result,omitempty"`
	ErrorMessage   *string        `json:"error_message,omitempty"`
	CreatedAt      string         `json:"created_at,omitempty"`
	StartedAt      string         `json:"started_at,omitempty"`
	CompletedAt    string         `json:"completed_at,omitempty"`
}

type RegistrationTaskListParams struct {
	Page     int
	PageSize int
	Status   string
}

type RegistrationTaskListResult struct {
	Total int                `json:"total"`
	Tasks []RegistrationTask `json:"tasks"`
}

type RegistrationStats struct {
	ByStatus   map[string]int `json:"by_status"`
	TodayCount int            `json:"today_count"`
}

func (s *SQLiteStore) CreateRegistrationTask(ctx context.Context, taskUUID string, proxy *string, emailServiceID *int) (*RegistrationTask, error) {
	if !s.Available() {
		return nil, errors.New("database unavailable")
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO registration_tasks (task_uuid, status, email_service_id, proxy, created_at) VALUES (?, 'pending', ?, ?, CURRENT_TIMESTAMP)`,
		taskUUID,
		emailServiceID,
		proxy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert registration task: %w", err)
	}

	return s.GetRegistrationTaskByUUID(ctx, taskUUID)
}

func (s *SQLiteStore) GetRegistrationTaskByUUID(ctx context.Context, taskUUID string) (*RegistrationTask, error) {
	if !s.Available() {
		return nil, nil
	}

	exists, err := s.tableExists(ctx, "registration_tasks")
	if err != nil || !exists {
		return nil, err
	}

	const query = `
SELECT id, task_uuid, status, email_service_id, proxy, logs, result, error_message, created_at, started_at, completed_at
FROM registration_tasks
WHERE task_uuid = ?`

	task, err := scanRegistrationTask(s.db.QueryRowContext(ctx, query, taskUUID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &task, nil
}

func (s *SQLiteStore) ListRegistrationTasks(ctx context.Context, params RegistrationTaskListParams) (RegistrationTaskListResult, error) {
	result := RegistrationTaskListResult{Tasks: []RegistrationTask{}}
	if !s.Available() {
		return result, nil
	}

	exists, err := s.tableExists(ctx, "registration_tasks")
	if err != nil || !exists {
		return result, err
	}

	clauses := []string{"WHERE 1 = 1"}
	args := make([]any, 0, 1)
	if value := strings.TrimSpace(params.Status); value != "" {
		clauses = append(clauses, "AND status = ?")
		args = append(args, value)
	}

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registration_tasks `+strings.Join(clauses, " "), args...).Scan(&result.Total); err != nil {
		return result, fmt.Errorf("count registration tasks: %w", err)
	}

	offset := max(params.Page-1, 0) * params.PageSize
	query := `
SELECT id, task_uuid, status, email_service_id, proxy, logs, result, error_message, created_at, started_at, completed_at
FROM registration_tasks ` + strings.Join(clauses, " ") + `
ORDER BY datetime(created_at) DESC, id DESC
LIMIT ? OFFSET ?`

	rows, err := s.db.QueryContext(ctx, query, append(args, params.PageSize, offset)...)
	if err != nil {
		return result, fmt.Errorf("query registration tasks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		task, err := scanRegistrationTask(rows)
		if err != nil {
			return result, err
		}
		result.Tasks = append(result.Tasks, task)
	}

	return result, rows.Err()
}

func (s *SQLiteStore) UpdateRegistrationTask(ctx context.Context, taskUUID string, updates map[string]any) (*RegistrationTask, error) {
	if !s.Available() {
		return nil, errors.New("database unavailable")
	}
	if len(updates) == 0 {
		return s.GetRegistrationTaskByUUID(ctx, taskUUID)
	}

	assignments := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)

	for key, value := range updates {
		switch key {
		case "status", "proxy", "logs", "error_message":
			assignments = append(assignments, key+" = ?")
			args = append(args, value)
		case "email_service_id":
			assignments = append(assignments, key+" = ?")
			args = append(args, value)
		case "started_at", "completed_at":
			assignments = append(assignments, key+" = ?")
			args = append(args, value)
		case "result":
			resultJSON, err := json.Marshal(value)
			if err != nil {
				return nil, fmt.Errorf("marshal task result: %w", err)
			}
			assignments = append(assignments, key+" = ?")
			args = append(args, string(resultJSON))
		}
	}

	if len(assignments) == 0 {
		return s.GetRegistrationTaskByUUID(ctx, taskUUID)
	}

	args = append(args, taskUUID)
	_, err := s.db.ExecContext(ctx, `UPDATE registration_tasks SET `+strings.Join(assignments, ", ")+` WHERE task_uuid = ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("update registration task: %w", err)
	}

	return s.GetRegistrationTaskByUUID(ctx, taskUUID)
}

func (s *SQLiteStore) AppendRegistrationTaskLog(ctx context.Context, taskUUID string, message string) error {
	task, err := s.GetRegistrationTaskByUUID(ctx, taskUUID)
	if err != nil {
		return err
	}
	if task == nil {
		return nil
	}

	logs := strings.TrimRight(task.Logs, "\n")
	if logs == "" {
		logs = message
	} else {
		logs += "\n" + message
	}

	_, err = s.db.ExecContext(ctx, `UPDATE registration_tasks SET logs = ? WHERE task_uuid = ?`, logs, taskUUID)
	if err != nil {
		return fmt.Errorf("append registration task log: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteRegistrationTask(ctx context.Context, taskUUID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM registration_tasks WHERE task_uuid = ?`, taskUUID)
	if err != nil {
		return fmt.Errorf("delete registration task: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetRegistrationStats(ctx context.Context) (RegistrationStats, error) {
	stats := RegistrationStats{ByStatus: map[string]int{}}
	if !s.Available() {
		return stats, nil
	}

	exists, err := s.tableExists(ctx, "registration_tasks")
	if err != nil || !exists {
		return stats, err
	}

	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM registration_tasks GROUP BY status`)
	if err != nil {
		return stats, fmt.Errorf("query registration stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status sql.NullString
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return stats, err
		}
		key := strings.TrimSpace(status.String)
		if key == "" {
			key = "unknown"
		}
		stats.ByStatus[key] = count
	}

	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registration_tasks WHERE date(created_at) = date('now')`).Scan(&stats.TodayCount); err != nil {
		return stats, fmt.Errorf("count today's registration tasks: %w", err)
	}

	return stats, nil
}

func scanRegistrationTask(row scanner) (RegistrationTask, error) {
	var (
		task           RegistrationTask
		emailServiceID sql.NullInt64
		proxy          sql.NullString
		logs           sql.NullString
		resultJSON     sql.NullString
		errorMessage   sql.NullString
		createdAt      sql.NullString
		startedAt      sql.NullString
		completedAt    sql.NullString
	)

	err := row.Scan(
		&task.ID,
		&task.TaskUUID,
		&task.Status,
		&emailServiceID,
		&proxy,
		&logs,
		&resultJSON,
		&errorMessage,
		&createdAt,
		&startedAt,
		&completedAt,
	)
	if err != nil {
		return RegistrationTask{}, err
	}

	if emailServiceID.Valid {
		id := int(emailServiceID.Int64)
		task.EmailServiceID = &id
	}
	task.Proxy = nullableString(proxy)
	task.Logs = strings.TrimSpace(logs.String)
	task.ErrorMessage = nullableString(errorMessage)
	task.CreatedAt = normalizeTimestamp(createdAt)
	task.StartedAt = normalizeTimestamp(startedAt)
	task.CompletedAt = normalizeTimestamp(completedAt)
	task.Result = map[string]any{}

	if resultJSON.Valid && strings.TrimSpace(resultJSON.String) != "" {
		_ = json.Unmarshal([]byte(resultJSON.String), &task.Result)
	}

	return task, nil
}
