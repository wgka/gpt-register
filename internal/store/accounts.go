package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codex-register/internal/config"

	_ "modernc.org/sqlite"
)

type Account struct {
	ID            int     `json:"id"`
	Email         string  `json:"email"`
	Password      *string `json:"password,omitempty"`
	ClientID      *string `json:"client_id,omitempty"`
	EmailService  string  `json:"email_service"`
	AccountID     *string `json:"account_id,omitempty"`
	WorkspaceID   *string `json:"workspace_id,omitempty"`
	RegisteredAt  string  `json:"registered_at,omitempty"`
	LastRefresh   string  `json:"last_refresh,omitempty"`
	ExpiresAt     string  `json:"expires_at,omitempty"`
	Status        string  `json:"status"`
	ProxyUsed     *string `json:"proxy_used,omitempty"`
	CPAUploaded   bool    `json:"cpa_uploaded"`
	CPAUploadedAt string  `json:"cpa_uploaded_at,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
	UpdatedAt     string  `json:"updated_at,omitempty"`
}

type AccountTokens struct {
	ID           int     `json:"id"`
	Email        string  `json:"email"`
	ClientID     *string `json:"client_id,omitempty"`
	AccessToken  *string `json:"access_token,omitempty"`
	RefreshToken *string `json:"refresh_token,omitempty"`
	IDToken      *string `json:"id_token,omitempty"`
	SessionToken *string `json:"session_token,omitempty"`
	LastRefresh  string  `json:"last_refresh,omitempty"`
	ExpiresAt    string  `json:"expires_at,omitempty"`
	AccountID    *string `json:"account_id,omitempty"`
}

type AccountCreate struct {
	Email          string
	Password       string
	ClientID       string
	SessionToken   string
	EmailService   string
	EmailServiceID string
	AccountID      string
	WorkspaceID    string
	AccessToken    string
	RefreshToken   string
	IDToken        string
	ProxyUsed      string
	ExtraData      map[string]any
	Status         string
	Source         string
}

type AccountListParams struct {
	Page         int
	PageSize     int
	Status       string
	EmailService string
	Search       string
}

type AccountListResult struct {
	Total    int       `json:"total"`
	Accounts []Account `json:"accounts"`
}

type AccountStats struct {
	Total          int            `json:"total"`
	ByStatus       map[string]int `json:"by_status"`
	ByEmailService map[string]int `json:"by_email_service"`
}

type SQLiteStore struct {
	db        *sql.DB
	available bool
}

func NewSQLiteStore(cfg config.Settings) (*SQLiteStore, error) {
	path, ok := cfg.SQLitePath()
	if !ok {
		return &SQLiteStore{}, nil
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	// 允许并发读写时更稳：WAL + busy_timeout 避免短暂写锁导致的 "database is locked"。
	_, _ = db.ExecContext(context.Background(), "PRAGMA journal_mode=WAL;")
	_, _ = db.ExecContext(context.Background(), "PRAGMA busy_timeout=5000;")

	store := &SQLiteStore{
		db:        db,
		available: true,
	}
	if err := store.initSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Available() bool {
	return s.available && s.db != nil
}

func (s *SQLiteStore) ListAccounts(ctx context.Context, params AccountListParams) (AccountListResult, error) {
	result := AccountListResult{Accounts: []Account{}}
	if !s.Available() {
		return result, nil
	}

	exists, err := s.tableExists(ctx, "accounts")
	if err != nil || !exists {
		return result, err
	}

	query, args := buildAccountFilter(params)

	countQuery := "SELECT COUNT(*) FROM accounts " + query
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&result.Total); err != nil {
		return result, fmt.Errorf("count accounts: %w", err)
	}

	offset := max(params.Page-1, 0) * params.PageSize
	listQuery := `
SELECT
	id,
	email,
	password,
	client_id,
	email_service,
	account_id,
	workspace_id,
	registered_at,
	last_refresh,
	expires_at,
	status,
	proxy_used,
	COALESCE(cpa_uploaded, 0),
	cpa_uploaded_at,
	created_at,
	updated_at
FROM accounts ` + query + `
ORDER BY datetime(created_at) DESC, id DESC
LIMIT ? OFFSET ?`

	rows, err := s.db.QueryContext(ctx, listQuery, append(args, params.PageSize, offset)...)
	if err != nil {
		return result, fmt.Errorf("query accounts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return result, err
		}
		result.Accounts = append(result.Accounts, account)
	}

	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate accounts: %w", err)
	}

	return result, nil
}

func (s *SQLiteStore) GetAccountByID(ctx context.Context, accountID int) (*Account, error) {
	if !s.Available() {
		return nil, nil
	}

	exists, err := s.tableExists(ctx, "accounts")
	if err != nil || !exists {
		return nil, err
	}

	const query = `
SELECT
	id,
	email,
	password,
	client_id,
	email_service,
	account_id,
	workspace_id,
	registered_at,
	last_refresh,
	expires_at,
	status,
	proxy_used,
	COALESCE(cpa_uploaded, 0),
	cpa_uploaded_at,
	created_at,
	updated_at
FROM accounts
WHERE id = ?`

	account, err := scanAccount(s.db.QueryRowContext(ctx, query, accountID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &account, nil
}

func (s *SQLiteStore) GetAccountTokensByID(ctx context.Context, accountID int) (*AccountTokens, error) {
	if !s.Available() {
		return nil, nil
	}

	exists, err := s.tableExists(ctx, "accounts")
	if err != nil || !exists {
		return nil, err
	}

	const query = `
SELECT
	id,
	email,
	client_id,
	access_token,
	refresh_token,
	id_token,
	session_token,
	last_refresh,
	expires_at,
	account_id
FROM accounts
WHERE id = ?`

	account, err := scanAccountTokens(s.db.QueryRowContext(ctx, query, accountID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &account, nil
}

func (s *SQLiteStore) GetAccountStats(ctx context.Context) (AccountStats, error) {
	stats := AccountStats{
		ByStatus:       map[string]int{},
		ByEmailService: map[string]int{},
	}
	if !s.Available() {
		return stats, nil
	}

	exists, err := s.tableExists(ctx, "accounts")
	if err != nil || !exists {
		return stats, err
	}

	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounts").Scan(&stats.Total); err != nil {
		return stats, fmt.Errorf("count account stats: %w", err)
	}

	statusRows, err := s.db.QueryContext(ctx, "SELECT status, COUNT(*) FROM accounts GROUP BY status")
	if err != nil {
		return stats, fmt.Errorf("query account status stats: %w", err)
	}
	defer statusRows.Close()

	for statusRows.Next() {
		var status sql.NullString
		var count int
		if err := statusRows.Scan(&status, &count); err != nil {
			return stats, fmt.Errorf("scan account status stat: %w", err)
		}
		key := strings.TrimSpace(status.String)
		if key == "" {
			key = "unknown"
		}
		stats.ByStatus[key] = count
	}

	serviceRows, err := s.db.QueryContext(ctx, "SELECT email_service, COUNT(*) FROM accounts GROUP BY email_service")
	if err != nil {
		return stats, fmt.Errorf("query account service stats: %w", err)
	}
	defer serviceRows.Close()

	for serviceRows.Next() {
		var service sql.NullString
		var count int
		if err := serviceRows.Scan(&service, &count); err != nil {
			return stats, fmt.Errorf("scan account service stat: %w", err)
		}
		key := strings.TrimSpace(service.String)
		if key == "" {
			key = "unknown"
		}
		stats.ByEmailService[key] = count
	}

	return stats, nil
}

func (s *SQLiteStore) CreateAccount(ctx context.Context, account AccountCreate) (int, error) {
	if !s.Available() {
		return 0, errors.New("database unavailable")
	}

	exists, err := s.tableExists(ctx, "accounts")
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, errors.New("accounts table missing")
	}

	extraJSON := "{}"
	if len(account.ExtraData) > 0 {
		data, err := json.Marshal(account.ExtraData)
		if err != nil {
			return 0, fmt.Errorf("marshal account extra_data: %w", err)
		}
		extraJSON = string(data)
	}

	status := account.Status
	if strings.TrimSpace(status) == "" {
		status = "active"
	}

	source := account.Source
	if strings.TrimSpace(source) == "" {
		source = "register"
	}

	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO accounts (
			email, password, access_token, refresh_token, id_token, session_token,
			client_id, account_id, workspace_id, email_service, email_service_id, proxy_used,
			registered_at, status, extra_data, source, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		account.Email,
		account.Password,
		account.AccessToken,
		account.RefreshToken,
		account.IDToken,
		account.SessionToken,
		account.ClientID,
		account.AccountID,
		account.WorkspaceID,
		account.EmailService,
		account.EmailServiceID,
		account.ProxyUsed,
		status,
		extraJSON,
		source,
	)
	if err != nil {
		return 0, fmt.Errorf("insert account: %w", err)
	}

	insertID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted account id: %w", err)
	}

	return int(insertID), nil
}

func (s *SQLiteStore) UpdateAccount(ctx context.Context, accountID int, updates map[string]any) error {
	if !s.Available() {
		return errors.New("database unavailable")
	}
	if len(updates) == 0 {
		return nil
	}

	assignments := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)

	for key, value := range updates {
		switch key {
		case "status", "access_token", "refresh_token", "id_token", "session_token", "last_refresh",
			"expires_at", "account_id", "workspace_id", "proxy_used", "source":
			assignments = append(assignments, key+" = ?")
			args = append(args, value)
		case "cpa_uploaded":
			assignments = append(assignments, key+" = ?")
			args = append(args, boolToInt(value.(bool)))
		case "cpa_uploaded_at":
			assignments = append(assignments, key+" = ?")
			args = append(args, value)
		case "extra_data":
			data, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("marshal account extra_data: %w", err)
			}
			assignments = append(assignments, key+" = ?")
			args = append(args, string(data))
		}
	}

	if len(assignments) == 0 {
		return nil
	}

	args = append(args, accountID)
	_, err := s.db.ExecContext(ctx, `UPDATE accounts SET `+strings.Join(assignments, ", ")+`, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	return nil
}

func (s *SQLiteStore) tableExists(ctx context.Context, name string) (bool, error) {
	if !s.Available() {
		return false, nil
	}

	var exists int
	err := s.db.QueryRowContext(
		ctx,
		"SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ? LIMIT 1",
		name,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check table %s: %w", name, err)
	}
	return true, nil
}

func buildAccountFilter(params AccountListParams) (string, []any) {
	clauses := []string{"WHERE 1 = 1"}
	args := make([]any, 0, 5)

	if value := strings.TrimSpace(params.Status); value != "" {
		clauses = append(clauses, "AND status = ?")
		args = append(args, value)
	}

	if value := strings.TrimSpace(params.EmailService); value != "" {
		clauses = append(clauses, "AND email_service = ?")
		args = append(args, value)
	}

	if value := strings.TrimSpace(params.Search); value != "" {
		pattern := "%" + value + "%"
		clauses = append(
			clauses,
			"AND (email LIKE ? COLLATE NOCASE OR account_id LIKE ? COLLATE NOCASE OR workspace_id LIKE ? COLLATE NOCASE)",
		)
		args = append(args, pattern, pattern, pattern)
	}

	return strings.Join(clauses, " "), args
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAccount(row scanner) (Account, error) {
	var (
		account       Account
		password      sql.NullString
		clientID      sql.NullString
		accountID     sql.NullString
		workspaceID   sql.NullString
		registeredAt  sql.NullString
		lastRefresh   sql.NullString
		expiresAt     sql.NullString
		proxyUsed     sql.NullString
		cpaUploaded   sql.NullInt64
		cpaUploadedAt sql.NullString
		createdAt     sql.NullString
		updatedAt     sql.NullString
	)

	err := row.Scan(
		&account.ID,
		&account.Email,
		&password,
		&clientID,
		&account.EmailService,
		&accountID,
		&workspaceID,
		&registeredAt,
		&lastRefresh,
		&expiresAt,
		&account.Status,
		&proxyUsed,
		&cpaUploaded,
		&cpaUploadedAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return Account{}, err
	}

	account.Password = nullableString(password)
	account.ClientID = nullableString(clientID)
	account.AccountID = nullableString(accountID)
	account.WorkspaceID = nullableString(workspaceID)
	account.RegisteredAt = normalizeTimestamp(registeredAt)
	account.LastRefresh = normalizeTimestamp(lastRefresh)
	account.ExpiresAt = normalizeTimestamp(expiresAt)
	account.ProxyUsed = nullableString(proxyUsed)
	account.CPAUploaded = cpaUploaded.Valid && cpaUploaded.Int64 != 0
	account.CPAUploadedAt = normalizeTimestamp(cpaUploadedAt)
	account.CreatedAt = normalizeTimestamp(createdAt)
	account.UpdatedAt = normalizeTimestamp(updatedAt)

	return account, nil
}

func scanAccountTokens(row scanner) (AccountTokens, error) {
	var (
		account      AccountTokens
		clientID     sql.NullString
		accessToken  sql.NullString
		refreshToken sql.NullString
		idToken      sql.NullString
		sessionToken sql.NullString
		lastRefresh  sql.NullString
		expiresAt    sql.NullString
		accountID    sql.NullString
	)

	err := row.Scan(
		&account.ID,
		&account.Email,
		&clientID,
		&accessToken,
		&refreshToken,
		&idToken,
		&sessionToken,
		&lastRefresh,
		&expiresAt,
		&accountID,
	)
	if err != nil {
		return AccountTokens{}, err
	}

	account.ClientID = nullableString(clientID)
	account.AccessToken = nullableString(accessToken)
	account.RefreshToken = nullableString(refreshToken)
	account.IDToken = nullableString(idToken)
	account.SessionToken = nullableString(sessionToken)
	account.LastRefresh = normalizeTimestamp(lastRefresh)
	account.ExpiresAt = normalizeTimestamp(expiresAt)
	account.AccountID = nullableString(accountID)
	return account, nil
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeTimestamp(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	raw := strings.TrimSpace(value.String)
	if raw == "" {
		return ""
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.Format(time.RFC3339)
		}
	}

	return raw
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
