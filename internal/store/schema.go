package store

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *SQLiteStore) initSchema(ctx context.Context) error {
	if !s.Available() {
		return nil
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			password TEXT,
			access_token TEXT,
			refresh_token TEXT,
			id_token TEXT,
			session_token TEXT,
			client_id TEXT,
			account_id TEXT,
			workspace_id TEXT,
			email_service TEXT NOT NULL,
			email_service_id TEXT,
			proxy_used TEXT,
			registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_refresh DATETIME,
			expires_at DATETIME,
			status TEXT DEFAULT 'active',
			extra_data TEXT,
			cpa_uploaded INTEGER DEFAULT 0,
			cpa_uploaded_at DATETIME,
			source TEXT DEFAULT 'register',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_accounts_email ON accounts(email)`,
		`CREATE TABLE IF NOT EXISTS email_services (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			service_type TEXT NOT NULL,
			name TEXT NOT NULL,
			config TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			priority INTEGER DEFAULT 0,
			last_used DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS registration_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_uuid TEXT NOT NULL UNIQUE,
			status TEXT DEFAULT 'pending',
			email_service_id INTEGER,
			proxy TEXT,
			logs TEXT,
			result TEXT,
			error_message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_registration_tasks_uuid ON registration_tasks(task_uuid)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT,
			description TEXT,
			category TEXT DEFAULT 'general',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS proxies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'http',
			host TEXT NOT NULL,
			port INTEGER NOT NULL,
			username TEXT,
			password TEXT,
			enabled INTEGER DEFAULT 1,
			priority INTEGER DEFAULT 0,
			last_used DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS online_account_scheduler_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trigger_type TEXT NOT NULL,
			status TEXT NOT NULL,
			attempt INTEGER DEFAULT 1,
			max_attempts INTEGER DEFAULT 1,
			schedule_mode TEXT,
			actions TEXT,
			invalid_found INTEGER DEFAULT 0,
			disabled_count INTEGER DEFAULT 0,
			deleted_count INTEGER DEFAULT 0,
			failed_count INTEGER DEFAULT 0,
			error_message TEXT,
			messages TEXT,
			started_at DATETIME,
			finished_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_online_account_scheduler_logs_created_at ON online_account_scheduler_logs(created_at DESC, id DESC)`,
	}

	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}

	migrations := []struct {
		table  string
		column string
		def    string
	}{
		{table: "accounts", column: "cpa_uploaded", def: "INTEGER DEFAULT 0"},
		{table: "accounts", column: "cpa_uploaded_at", def: "DATETIME"},
		{table: "accounts", column: "source", def: "TEXT DEFAULT 'register'"},
	}

	for _, migration := range migrations {
		exists, err := s.columnExists(ctx, migration.table, migration.column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", migration.table, migration.column, migration.def)); err != nil {
			return fmt.Errorf("migrate %s.%s: %w", migration.table, migration.column, err)
		}
	}

	return nil
}

func (s *SQLiteStore) columnExists(ctx context.Context, tableName, columnName string) (bool, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false, fmt.Errorf("pragma table_info %s: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			fieldType string
			notNull   int
			defaultV  sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &fieldType, &notNull, &defaultV, &pk); err != nil {
			return false, fmt.Errorf("scan pragma table_info %s: %w", tableName, err)
		}
		if name == columnName {
			return true, nil
		}
	}

	return false, rows.Err()
}
