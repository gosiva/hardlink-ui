package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"
)

// Session represents a user session
type Session struct {
	SessionID        string
	Username         string
	Authenticated2FA bool
	LastActive       int64
	CreatedAt        int64
}

// GenerateSessionID creates a cryptographically secure session ID
func GenerateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateSession creates a new session for a user
func (db *DB) CreateSession(username string, authenticated2FA bool) (string, error) {
	sessionID, err := GenerateSessionID()
	if err != nil {
		return "", fmt.Errorf("failed to generate session ID: %w", err)
	}

	now := time.Now().Unix()
	_, err = db.Exec(`
		INSERT INTO sessions (session_id, username, authenticated_2fa, last_active, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, sessionID, username, boolToInt(authenticated2FA), now, now)

	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return sessionID, nil
}

// GetSession retrieves a session by ID
func (db *DB) GetSession(sessionID string) (*Session, error) {
	log.Printf("DEBUG GetSession: querying DB for sessionID=%s", sessionID)
	session := &Session{}
	var authenticated2FAInt int
	err := db.QueryRow(`
		SELECT session_id, username, authenticated_2fa, last_active, created_at
		FROM sessions
		WHERE session_id = ?
	`, sessionID).Scan(&session.SessionID, &session.Username, &authenticated2FAInt, &session.LastActive, &session.CreatedAt)

	if err == sql.ErrNoRows {
		log.Printf("DEBUG GetSession: session not found in DB (sessionID=%s)", sessionID)
		return nil, nil
	}
	if err != nil {
		log.Printf("DEBUG GetSession: DB error: %v (sessionID=%s)", err, sessionID)
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	session.Authenticated2FA = authenticated2FAInt == 1
	log.Printf("DEBUG GetSession: session retrieved from DB (sessionID=%s, username=%s, authenticated_2fa_int=%d, authenticated_2fa_bool=%v)", 
		sessionID, session.Username, authenticated2FAInt, session.Authenticated2FA)
	return session, nil
}

// UpdateSession updates the last active timestamp and 2FA status
func (db *DB) UpdateSession(sessionID string, authenticated2FA bool) error {
	log.Printf("DEBUG UpdateSession: updating session in DB (sessionID=%s, authenticated_2fa=%v)", 
		sessionID, authenticated2FA)

	now := time.Now().Unix()
	result, err := db.Exec(`
		UPDATE sessions
		SET authenticated_2fa = ?, last_active = ?
		WHERE session_id = ?
	`, boolToInt(authenticated2FA), now, sessionID)

	if err != nil {
		log.Printf("DEBUG UpdateSession: DB update error: %v (sessionID=%s)", err, sessionID)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		log.Printf("DEBUG UpdateSession: error getting rows affected: %v (sessionID=%s)", err, sessionID)
		rows = 0
	}

	log.Printf("DEBUG UpdateSession: DB update completed (sessionID=%s, authenticated_2fa=%v, rows_affected=%d)", 
		sessionID, authenticated2FA, rows)

	if rows == 0 {
		log.Printf("DEBUG UpdateSession: no rows affected, session not found (sessionID=%s)", sessionID)
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

// UpdateSessionActivity updates only the last active timestamp
func (db *DB) UpdateSessionActivity(sessionID string) error {
	log.Printf("DEBUG UpdateSessionActivity: updating last_active for sessionID=%s", sessionID)
	
	now := time.Now().Unix()
	result, err := db.Exec(`
		UPDATE sessions
		SET last_active = ?
		WHERE session_id = ?
	`, now, sessionID)

	if err != nil {
		log.Printf("DEBUG UpdateSessionActivity: DB error: %v (sessionID=%s)", err, sessionID)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		log.Printf("DEBUG UpdateSessionActivity: error getting rows affected: %v (sessionID=%s)", err, sessionID)
		// Use -1 to indicate unknown row count (error occurred)
		rows = -1
	}

	log.Printf("DEBUG UpdateSessionActivity: completed (sessionID=%s, rows_affected=%d, last_active=%d)", 
		sessionID, rows, now)

	return nil
}

// DeleteSession removes a session
func (db *DB) DeleteSession(sessionID string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE session_id = ?`, sessionID)
	return err
}

// CleanupExpiredSessions removes sessions that haven't been active for the given timeout
func (db *DB) CleanupExpiredSessions(timeoutSeconds int) error {
	cutoff := time.Now().Unix() - int64(timeoutSeconds)
	_, err := db.Exec(`DELETE FROM sessions WHERE last_active < ?`, cutoff)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
