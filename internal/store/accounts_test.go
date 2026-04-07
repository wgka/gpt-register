package store

import (
	"context"
	"path/filepath"
	"testing"

	"codex-register/internal/config"
)

func TestDeleteAccount(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "accounts.db")
	sqliteStore, err := NewSQLiteStore(config.Settings{DatabaseURL: dbPath})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}

	accountID, err := sqliteStore.CreateAccount(context.Background(), AccountCreate{
		Email:        "delete-me@example.com",
		EmailService: "tempmail",
		AccessToken:  "access-token",
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	if err := sqliteStore.DeleteAccount(context.Background(), accountID); err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}

	account, err := sqliteStore.GetAccountByID(context.Background(), accountID)
	if err != nil {
		t.Fatalf("GetAccountByID() error = %v", err)
	}
	if account != nil {
		t.Fatalf("expected deleted account to be nil, got %#v", account)
	}

	tokens, err := sqliteStore.GetAccountTokensByID(context.Background(), accountID)
	if err != nil {
		t.Fatalf("GetAccountTokensByID() error = %v", err)
	}
	if tokens != nil {
		t.Fatalf("expected deleted account tokens to be nil, got %#v", tokens)
	}
}

func TestUpdateAccountAllowsClientID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "accounts.db")
	sqliteStore, err := NewSQLiteStore(config.Settings{DatabaseURL: dbPath})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}

	accountID, err := sqliteStore.CreateAccount(context.Background(), AccountCreate{
		Email:        "client-id@example.com",
		EmailService: "tempmail",
		AccessToken:  "access-token",
		ClientID:     "old-client-id",
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	if err := sqliteStore.UpdateAccount(context.Background(), accountID, map[string]any{
		"client_id": "new-client-id",
	}); err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}

	account, err := sqliteStore.GetAccountByID(context.Background(), accountID)
	if err != nil {
		t.Fatalf("GetAccountByID() error = %v", err)
	}
	if account == nil || account.ClientID == nil || *account.ClientID != "new-client-id" {
		t.Fatalf("expected updated client_id, got %#v", account)
	}
}

func TestListAccountsFilterByRefreshToken(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "accounts.db")
	sqliteStore, err := NewSQLiteStore(config.Settings{DatabaseURL: dbPath})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}

	_, err = sqliteStore.CreateAccount(context.Background(), AccountCreate{
		Email:        "with-refresh@example.com",
		EmailService: "tempmail",
		AccessToken:  "access-token-1",
		RefreshToken: "refresh-token-1",
	})
	if err != nil {
		t.Fatalf("CreateAccount(with refresh) error = %v", err)
	}

	_, err = sqliteStore.CreateAccount(context.Background(), AccountCreate{
		Email:        "without-refresh@example.com",
		EmailService: "tempmail",
		AccessToken:  "access-token-2",
	})
	if err != nil {
		t.Fatalf("CreateAccount(without refresh) error = %v", err)
	}

	withRefresh, err := sqliteStore.ListAccounts(context.Background(), AccountListParams{
		Page:               1,
		PageSize:           20,
		RefreshTokenStatus: "has",
	})
	if err != nil {
		t.Fatalf("ListAccounts(has) error = %v", err)
	}
	if len(withRefresh.Accounts) != 1 || withRefresh.Accounts[0].Email != "with-refresh@example.com" {
		t.Fatalf("unexpected has filter result: %#v", withRefresh.Accounts)
	}

	withoutRefresh, err := sqliteStore.ListAccounts(context.Background(), AccountListParams{
		Page:               1,
		PageSize:           20,
		RefreshTokenStatus: "none",
	})
	if err != nil {
		t.Fatalf("ListAccounts(none) error = %v", err)
	}
	if len(withoutRefresh.Accounts) != 1 || withoutRefresh.Accounts[0].Email != "without-refresh@example.com" {
		t.Fatalf("unexpected none filter result: %#v", withoutRefresh.Accounts)
	}
}
