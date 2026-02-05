package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/pquerna/otp/totp"

	"github.com/gosiva/hardlink-ui/internal/config"
	"github.com/gosiva/hardlink-ui/internal/storage"
)

const (
	SessionCookieName = "hardlink_session"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	db            *storage.DB
	cfg           *config.Config
	loginTemplate *template.Template
	tfaTemplate   *template.Template
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(db *storage.DB, cfg *config.Config, templatesPath string) (*AuthHandler, error) {
	loginTmpl, err := template.ParseFiles(
		filepath.Join(templatesPath, "base.html"),
		filepath.Join(templatesPath, "login.html"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse login template: %w", err)
	}

	tfaTmpl, err := template.ParseFiles(
		filepath.Join(templatesPath, "base.html"),
		filepath.Join(templatesPath, "2fa.html"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse 2fa template: %w", err)
	}

	return &AuthHandler{
		db:            db,
		cfg:           cfg,
		loginTemplate: loginTmpl,
		tfaTemplate:   tfaTmpl,
	}, nil
}

// ShowLogin shows the login page
func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Check if user is already authenticated - redirect to app if so
		cookie, err := r.Cookie(SessionCookieName)
		if err == nil {
			session, err := h.db.GetSession(cookie.Value)
			if err == nil && session != nil && session.Authenticated2FA {
				log.Printf("DEBUG ShowLogin: user already authenticated, redirecting to / (sessionID=%s, user=%s)", session.SessionID, session.Username)
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}
		
		data := map[string]interface{}{
			"error":      "",
			"isAuthPage": true,
		}
		h.loginTemplate.ExecuteTemplate(w, "base.html", data)
		return
	}

	// POST request
	username := r.FormValue("username")
	password := r.FormValue("password")
	ip := getIP(r)

	// Check if locked
	locked, err := h.db.IsLoginLocked(ip, username)
	if err != nil {
		log.Printf("Error checking login lock: %v", err)
		h.showLoginError(w, "Internal error")
		return
	}

	if locked {
		log.Printf("LOGIN LOCKOUT user=%s ip=%s", username, ip)
		h.showLoginError(w, "Trop de tentatives. Réessaie plus tard.")
		return
	}

	// Verify password
	valid, err := h.db.VerifyPassword(username, password)
	if err != nil {
		log.Printf("Error verifying password: %v", err)
		h.showLoginError(w, "Internal error")
		return
	}

	if !valid {
		h.db.RegisterFailedLogin(ip, username)
		log.Printf("LOGIN FAILED user=%s ip=%s", username, ip)
		h.showLoginError(w, "Identifiants invalides")
		return
	}

	// Password valid - reset failed attempts and redirect to 2FA
	h.db.ResetFailedLogin(ip, username)
	log.Printf("LOGIN SUCCESS user=%s ip=%s", username, ip)

	// Create temporary session for 2FA
	sessionID, err := h.db.CreateSession(username, false)
	if err != nil {
		log.Printf("Error creating session: %v", err)
		h.showLoginError(w, "Internal error")
		return
	}

	// Set cookie with proper SameSite and Secure attributes for cross-browser compatibility
	// Use SameSiteNoneMode with Secure for better cross-site behavior, especially on Safari
	sameSite := http.SameSiteLaxMode
	secure := r.TLS != nil
	
	// For production environments or when using HTTPS, use SameSiteNone for better compatibility
	// This prevents cookie issues during redirect chains on Safari and other browsers
	// IMPORTANT: SameSiteNone requires Secure flag, so we only use it when TLS is active
	if secure {
		sameSite = http.SameSiteNoneMode
		log.Printf("DEBUG ShowLogin: using SameSiteNone for TLS connection (sessionID=%s)", sessionID)
	}
	
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   h.cfg.SessionTimeout,
	})
	
	log.Printf("DEBUG ShowLogin: session cookie set (sessionID=%s, secure=%v, sameSite=%v)", 
		sessionID, secure, sameSite)

	// Redirect to 2FA
	next := r.URL.Query().Get("next")
	if next == "" {
		next = "/"
	}
	log.Printf("DEBUG ShowLogin: redirecting to /2fa (sessionID=%s, user=%s, next=%s)", sessionID, username, next)
	
	// Set proper Content-Type header to prevent Safari download dialog issues
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.Redirect(w, r, "/2fa?next="+next, http.StatusFound)
}

// Show2FA shows the 2FA verification page
func (h *AuthHandler) Show2FA(w http.ResponseWriter, r *http.Request) {
	// Get session
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	session, err := h.db.GetSession(cookie.Value)
	if err != nil || session == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if r.Method == "GET" {
		// Check if 2FA already completed - redirect to app if so
		if session.Authenticated2FA {
			log.Printf("DEBUG Show2FA: user already authenticated, redirecting to / (sessionID=%s, user=%s)", session.SessionID, session.Username)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		
		data := map[string]interface{}{
			"error":      "",
			"isAuthPage": true,
		}
		h.tfaTemplate.ExecuteTemplate(w, "base.html", data)
		return
	}

	// POST request
	code := r.FormValue("code")
	ip := getIP(r)

	// Check if 2FA locked
	locked, err := h.db.Is2FALocked(ip)
	if err != nil {
		log.Printf("Error checking 2FA lock: %v", err)
		h.show2FAError(w, "Internal error")
		return
	}

	if locked {
		log.Printf("2FA LOCKOUT user=%s ip=%s", session.Username, ip)
		h.show2FAError(w, "Trop de tentatives 2FA. Réessaie plus tard.")
		return
	}

	// Get user to retrieve TOTP secret
	user, err := h.db.GetUser(session.Username)
	if err != nil || user == nil {
		log.Printf("Error getting user: %v", err)
		h.show2FAError(w, "Internal error")
		return
	}

	// Verify TOTP
	valid := totp.Validate(code, user.TOTPSecret)
	if !valid {
		h.db.RegisterFailed2FA(ip)
		log.Printf("2FA FAILED user=%s ip=%s", session.Username, ip)
		h.show2FAError(w, "Code 2FA invalide")
		return
	}

	// 2FA valid
	h.db.ResetFailed2FA(ip)
	log.Printf("2FA SUCCESS user=%s ip=%s", session.Username, ip)

	// Update session to mark 2FA complete
	if err := h.db.UpdateSession(session.SessionID, true); err != nil {
		log.Printf("Error updating session: %v", err)
		// Force re-login if session is invalid
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	log.Printf("DEBUG Show2FA: session updated in DB, verifying update (sessionID=%s)", session.SessionID)

	// Re-verify that the session was actually updated in the database
	// This ensures the write is committed before we redirect
	updatedSession, err := h.db.GetSession(session.SessionID)
	if err != nil || updatedSession == nil {
		log.Printf("Error verifying updated session: %v", err)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if !updatedSession.Authenticated2FA {
		log.Printf("ERROR: Session 2FA verification pending - database update not yet visible (sessionID=%s)", session.SessionID)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	log.Printf("DEBUG Show2FA: session update verified, authenticated_2fa=%v (sessionID=%s)", 
		updatedSession.Authenticated2FA, session.SessionID)

	// Set cookie with proper SameSite and Secure attributes for cross-browser compatibility
	// This is critical to prevent login loops and download dialogs on Safari
	sameSite := http.SameSiteLaxMode
	secure := r.TLS != nil
	
	// For production environments or when using HTTPS, use SameSiteNone for better compatibility
	// This prevents cookie issues during redirect chains on Safari and other browsers
	// IMPORTANT: SameSiteNone requires Secure flag, so we only use it when TLS is active
	if secure {
		sameSite = http.SameSiteNoneMode
		log.Printf("DEBUG Show2FA: using SameSiteNone for TLS connection (sessionID=%s)", session.SessionID)
	}
	
	// Refresh cookie to ensure browser has updated session reference
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    session.SessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   h.cfg.SessionTimeout,
	})
	
	log.Printf("DEBUG Show2FA: session cookie refreshed (sessionID=%s, secure=%v, sameSite=%v)", 
		session.SessionID, secure, sameSite)

	// Set proper Content-Type header to prevent Safari from triggering download dialog
	// Safari can misinterpret redirects without proper headers as file downloads
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	
	// Redirect to original destination
	next := r.URL.Query().Get("next")
	if next == "" {
		next = "/"
	}
	log.Printf("DEBUG Show2FA: redirecting to %s (sessionID=%s, user=%s)", next, session.SessionID, session.Username)
	http.Redirect(w, r, next, http.StatusFound)
}

// Logout logs out the user
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(SessionCookieName)
	if err == nil {
		h.db.DeleteSession(cookie.Value)
		log.Printf("LOGOUT sessionID=%s", cookie.Value)
	}

	// Determine SameSite and Secure settings
	sameSite := http.SameSiteLaxMode
	secure := r.TLS != nil
	// IMPORTANT: SameSiteNone requires Secure flag, so we only use it when TLS is active
	if secure {
		sameSite = http.SameSiteNoneMode
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   -1,
	})

	log.Printf("DEBUG Logout: session cookie cleared (secure=%v, sameSite=%v)", secure, sameSite)
	
	// Set proper Content-Type header
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *AuthHandler) showLoginError(w http.ResponseWriter, errMsg string) {
	data := map[string]interface{}{
		"error":      errMsg,
		"isAuthPage": true,
	}
	w.WriteHeader(http.StatusUnauthorized)
	h.loginTemplate.ExecuteTemplate(w, "base.html", data)
}

func (h *AuthHandler) show2FAError(w http.ResponseWriter, errMsg string) {
	data := map[string]interface{}{
		"error":      errMsg,
		"isAuthPage": true,
	}
	w.WriteHeader(http.StatusUnauthorized)
	h.tfaTemplate.ExecuteTemplate(w, "base.html", data)
}

func getIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

// JSONResponse sends a JSON response
func JSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// JSONError sends a JSON error response
func JSONError(w http.ResponseWriter, status int, message string) {
	JSONResponse(w, status, map[string]string{"error": message})
}
