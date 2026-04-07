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
