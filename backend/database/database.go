package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func Init(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE,
		password_hash TEXT,
		display_name TEXT NOT NULL DEFAULT '',
		avatar_url TEXT NOT NULL DEFAULT '',
		role TEXT NOT NULL DEFAULT 'user',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS oauth_providers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL,
		client_id TEXT NOT NULL,
		client_secret TEXT NOT NULL,
		auth_url TEXT NOT NULL,
		token_url TEXT NOT NULL,
		userinfo_url TEXT NOT NULL,
		scopes TEXT NOT NULL DEFAULT '',
		icon TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		extra_params TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS linked_accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		provider_id INTEGER NOT NULL,
		provider_user_id TEXT NOT NULL,
		provider_email TEXT NOT NULL DEFAULT '',
		provider_name TEXT NOT NULL DEFAULT '',
		provider_avatar TEXT NOT NULL DEFAULT '',
		access_token TEXT NOT NULL DEFAULT '',
		refresh_token TEXT NOT NULL DEFAULT '',
		token_expiry DATETIME,
		refresh_token_expiry DATETIME,
		scopes_granted TEXT NOT NULL DEFAULT '',
		raw_token_response TEXT NOT NULL DEFAULT '{}',
		raw_userinfo TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (provider_id) REFERENCES oauth_providers(id) ON DELETE CASCADE,
		UNIQUE(provider_id, provider_user_id)
	);

	CREATE TABLE IF NOT EXISTS oauth_states (
		state TEXT PRIMARY KEY,
		provider_id INTEGER NOT NULL,
		user_id INTEGER,
		action TEXT NOT NULL DEFAULT 'login',
		redirect_url TEXT NOT NULL DEFAULT '/',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (provider_id) REFERENCES oauth_providers(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_linked_accounts_user ON linked_accounts(user_id);
	CREATE INDEX IF NOT EXISTS idx_linked_accounts_provider ON linked_accounts(provider_id, provider_user_id);
	`
	_, err := db.Exec(schema)
	return err
}

func Seed(db *sql.DB) error {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.Exec(
		"INSERT OR IGNORE INTO users (email, password_hash, display_name, role, created_at) VALUES (?, ?, ?, ?, ?)",
		"admin@example.com", string(hash), "Admin", "admin", time.Now(),
	)
	return err
}
