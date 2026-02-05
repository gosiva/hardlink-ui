package api

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gosiva/hardlink-ui/internal/config"
	"github.com/gosiva/hardlink-ui/internal/storage"
)

type contextKey string

const (
	userContextKey contextKey = "user"
)

// Middleware holds middleware dependencies
type Middleware struct {
	db  *storage.DB
	cfg *config.Config
}

// NewMiddleware creates a new middleware instance
func NewMiddleware(db *storage.DB, cfg *config.Config) *Middleware {
	return &Middleware{
		db:  db,
		cfg: cfg,
	}
}

// RequireAuth ensures the user is authenticated
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			log.Printf("DEBUG RequireAuth: no session cookie found, redirecting to /login (path=%s, error=%v)", r.URL.Path, err)
			
			// Set proper Content-Type header before redirect
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusFound)
			return
		}

		log.Printf("DEBUG RequireAuth: session cookie found (sessionID=%s, path=%s)", cookie.Value, r.URL.Path)

		session, err := m.db.GetSession(cookie.Value)
		if err != nil {
			log.Printf("DEBUG RequireAuth: error getting session from DB: %v (sessionID=%s, path=%s)", 
				err, cookie.Value, r.URL.Path)
			
			// Set proper Content-Type header before redirect
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusFound)
			return
		}

		if session == nil {
			log.Printf("DEBUG RequireAuth: session not found in DB, redirecting to /login (sessionID=%s, path=%s)", 
				cookie.Value, r.URL.Path)
			
			// Set proper Content-Type header before redirect
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusFound)
			return
		}

		log.Printf("DEBUG RequireAuth: session found in DB (sessionID=%s, user=%s, authenticated_2fa=%v, last_active=%d, path=%s)",
			session.SessionID, session.Username, session.Authenticated2FA, session.LastActive, r.URL.Path)

		// Check if 2FA is complete
		if !session.Authenticated2FA {
			log.Printf("DEBUG RequireAuth: session 2FA not authenticated, redirecting to /2fa (sessionID=%s, user=%s, authenticated_2fa=%v, path=%s)", 
				session.SessionID, session.Username, session.Authenticated2FA, r.URL.Path)
			
			// Set proper Content-Type header before redirect
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.Redirect(w, r, "/2fa?next="+r.URL.Path, http.StatusFound)
			return
		}

		log.Printf("DEBUG RequireAuth: session validated successfully (sessionID=%s, user=%s, authenticated_2fa=%v, path=%s)", 
			session.SessionID, session.Username, session.Authenticated2FA, r.URL.Path)

		// Check if session is expired
		if time.Now().Unix()-session.LastActive > int64(m.cfg.SessionTimeout) {
			log.Printf("DEBUG RequireAuth: session expired, deleting and redirecting to /login (sessionID=%s, user=%s, last_active=%d, timeout=%d)", 
				session.SessionID, session.Username, session.LastActive, m.cfg.SessionTimeout)
			m.db.DeleteSession(session.SessionID)
			
			// Set proper Content-Type header before redirect
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusFound)
			return
		}

		// Update last active time
		if err := m.db.UpdateSessionActivity(session.SessionID); err != nil {
			log.Printf("DEBUG RequireAuth: error updating session activity: %v (sessionID=%s)", err, session.SessionID)
		} else {
			log.Printf("DEBUG RequireAuth: session activity updated (sessionID=%s)", session.SessionID)
		}

		// Add username to context
		ctx := context.WithValue(r.Context(), userContextKey, session.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logging middleware logs all requests
func (m *Middleware) Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Wrap response writer to capture status
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapped, r)
		
		duration := time.Since(start)
		log.Printf("%s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher if the underlying ResponseWriter supports it
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker if the underlying ResponseWriter supports it
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push implements http.Pusher if the underlying ResponseWriter supports it
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := rw.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

// ReadFrom implements io.ReaderFrom if the underlying ResponseWriter supports it
func (rw *responseWriter) ReadFrom(src io.Reader) (n int64, err error) {
	if readerFrom, ok := rw.ResponseWriter.(io.ReaderFrom); ok {
		return readerFrom.ReadFrom(src)
	}
	return io.Copy(rw.ResponseWriter, src)
}

// CORS middleware
func (m *Middleware) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}

// GetUsername retrieves the username from request context
func GetUsername(r *http.Request) string {
	if username, ok := r.Context().Value(userContextKey).(string); ok {
		return username
	}
	return ""
}
