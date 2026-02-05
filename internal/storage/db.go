package storage

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection
type DB struct {
	*sql.DB
}

// New creates a new database connection and initializes the schema
func New(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	d := &DB{db}

	// Initialize schema
	if err := d.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Printf("Database initialized: %s", dbPath)
	return d, nil
}

func (db *DB) initSchema() error {
	schema := `
	-- Users table
	CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password_hash TEXT NOT NULL,
		totp_secret TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	-- Sessions table
	CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		authenticated_2fa INTEGER NOT NULL DEFAULT 0,
		last_active INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		FOREIGN KEY (username) REFERENCES users(username) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_username ON sessions(username);
	CREATE INDEX IF NOT EXISTS idx_sessions_last_active ON sessions(last_active);

	-- Inode index table for persistence
	CREATE TABLE IF NOT EXISTS inode_index (
		inode INTEGER NOT NULL,
		path TEXT NOT NULL,
		last_seen INTEGER NOT NULL,
		PRIMARY KEY (inode, path)
	);

	CREATE INDEX IF NOT EXISTS idx_inode ON inode_index(inode);

	-- Duplicate scan jobs
	CREATE TABLE IF NOT EXISTS scan_jobs (
		job_id TEXT PRIMARY KEY,
		status TEXT NOT NULL, -- 'running', 'completed', 'failed'
		progress INTEGER NOT NULL DEFAULT 0,
		total_files INTEGER NOT NULL DEFAULT 0,
		groups_found INTEGER NOT NULL DEFAULT 0,
		started_at INTEGER NOT NULL,
		completed_at INTEGER,
		error TEXT
	);

	-- Duplicate groups results
	CREATE TABLE IF NOT EXISTS duplicate_groups (
		job_id TEXT NOT NULL,
		group_id INTEGER NOT NULL,
		size INTEGER NOT NULL,
		master_path TEXT NOT NULL,
		FOREIGN KEY (job_id) REFERENCES scan_jobs(job_id) ON DELETE CASCADE,
		PRIMARY KEY (job_id, group_id)
	);

	CREATE INDEX IF NOT EXISTS idx_dup_groups_job ON duplicate_groups(job_id);

	-- Duplicate group members
	CREATE TABLE IF NOT EXISTS duplicate_members (
		job_id TEXT NOT NULL,
		group_id INTEGER NOT NULL,
		path TEXT NOT NULL,
		FOREIGN KEY (job_id, group_id) REFERENCES duplicate_groups(job_id, group_id) ON DELETE CASCADE,
		PRIMARY KEY (job_id, group_id, path)
	);

	-- Failed login attempts (anti-brute force)
	CREATE TABLE IF NOT EXISTS failed_logins (
		key TEXT PRIMARY KEY, -- ip:username
		count INTEGER NOT NULL DEFAULT 0,
		last_attempt INTEGER NOT NULL
	);

	-- Failed 2FA attempts
	CREATE TABLE IF NOT EXISTS failed_2fa (
		ip TEXT PRIMARY KEY,
		count INTEGER NOT NULL DEFAULT 0,
		last_attempt INTEGER NOT NULL
	);
	`

	_, err := db.Exec(schema)
	return err
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
