package storage

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	MaxLoginAttempts = 5
	LockoutSeconds   = 600 // 10 minutes
)

// User represents a user account
type User struct {
	Username     string
	PasswordHash string
	TOTPSecret   string
	CreatedAt    int64
}

// CreateUser creates a new user with hashed password
func (db *DB) CreateUser(username, password, totpSecret string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (username, password_hash, totp_secret, created_at)
		VALUES (?, ?, ?, ?)
	`, username, string(hash), totpSecret, time.Now().Unix())

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUser retrieves a user by username
func (db *DB) GetUser(username string) (*User, error) {
	user := &User{}
	err := db.QueryRow(`
		SELECT username, password_hash, totp_secret, created_at
		FROM users
		WHERE username = ?
	`, username).Scan(&user.Username, &user.PasswordHash, &user.TOTPSecret, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// VerifyPassword checks if the password matches the user's hash
func (db *DB) VerifyPassword(username, password string) (bool, error) {
	user, err := db.GetUser(username)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil, nil
}

// IsLoginLocked checks if login is locked for a given IP and username
func (db *DB) IsLoginLocked(ip, username string) (bool, error) {
	key := fmt.Sprintf("%s:%s", ip, username)
	
	var count int
	var lastAttempt int64
	err := db.QueryRow(`
		SELECT count, last_attempt FROM failed_logins WHERE key = ?
	`, key).Scan(&count, &lastAttempt)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if count < MaxLoginAttempts {
		return false, nil
	}

	// Check if lockout period has expired
	return time.Now().Unix()-lastAttempt < LockoutSeconds, nil
}

// RegisterFailedLogin records a failed login attempt
func (db *DB) RegisterFailedLogin(ip, username string) error {
	key := fmt.Sprintf("%s:%s", ip, username)
	now := time.Now().Unix()

	_, err := db.Exec(`
		INSERT INTO failed_logins (key, count, last_attempt)
		VALUES (?, 1, ?)
		ON CONFLICT(key) DO UPDATE SET
			count = count + 1,
			last_attempt = ?
	`, key, now, now)

	return err
}

// ResetFailedLogin clears failed login attempts for a user
func (db *DB) ResetFailedLogin(ip, username string) error {
	key := fmt.Sprintf("%s:%s", ip, username)
	_, err := db.Exec(`DELETE FROM failed_logins WHERE key = ?`, key)
	return err
}

// Is2FALocked checks if 2FA is locked for a given IP
func (db *DB) Is2FALocked(ip string) (bool, error) {
	var count int
	var lastAttempt int64
	err := db.QueryRow(`
		SELECT count, last_attempt FROM failed_2fa WHERE ip = ?
	`, ip).Scan(&count, &lastAttempt)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if count < MaxLoginAttempts {
		return false, nil
	}

	return time.Now().Unix()-lastAttempt < LockoutSeconds, nil
}

// RegisterFailed2FA records a failed 2FA attempt
func (db *DB) RegisterFailed2FA(ip string) error {
	now := time.Now().Unix()
	_, err := db.Exec(`
		INSERT INTO failed_2fa (ip, count, last_attempt)
		VALUES (?, 1, ?)
		ON CONFLICT(ip) DO UPDATE SET
			count = count + 1,
			last_attempt = ?
	`, ip, now, now)

	return err
}

// ResetFailed2FA clears failed 2FA attempts for an IP
func (db *DB) ResetFailed2FA(ip string) error {
	_, err := db.Exec(`DELETE FROM failed_2fa WHERE ip = ?`, ip)
	return err
}

// UserExists checks if a user exists
func (db *DB) UserExists(username string) (bool, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)`, username).Scan(&exists)
	return exists, err
}
