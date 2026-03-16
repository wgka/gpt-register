package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func (s *SQLiteStore) GetSettingString(ctx context.Context, key string, fallback string) (string, error) {
	if !s.Available() {
		return fallback, nil
	}

	exists, err := s.tableExists(ctx, "settings")
	if err != nil || !exists {
		return fallback, err
	}

	var value sql.NullString
	err = s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return fallback, nil
	}
	if err != nil {
		return fallback, fmt.Errorf("get setting %s: %w", key, err)
	}
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return fallback, nil
	}
	return value.String, nil
}

func (s *SQLiteStore) GetSettingBool(ctx context.Context, key string, fallback bool) (bool, error) {
	value, err := s.GetSettingString(ctx, key, "")
	if err != nil || value == "" {
		return fallback, err
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return fallback, nil
	}
}

func (s *SQLiteStore) GetSettingInt(ctx context.Context, key string, fallback int) (int, error) {
	value, err := s.GetSettingString(ctx, key, "")
	if err != nil || value == "" {
		return fallback, err
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback, nil
	}
	return parsed, nil
}

func (s *SQLiteStore) GetSettingJSONMap(ctx context.Context, key string) (map[string]any, error) {
	value, err := s.GetSettingString(ctx, key, "")
	if err != nil || strings.TrimSpace(value) == "" {
		return map[string]any{}, err
	}

	result := map[string]any{}
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return map[string]any{}, nil
	}
	return result, nil
}
