package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gosiva/hardlink-ui/internal/config"
	"github.com/gosiva/hardlink-ui/internal/storage"
)

// TestSessionUpdateFlow verifies that the session is properly updated after 2FA
// and that subsequent requests see the updated session state
func TestSessionUpdateFlow(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test user with TOTP secret
	totpSecret := "JBSWY3DPEHPK3PXP" // This generates predictable codes for testing
	if err := db.CreateUser("testuser", "testpass", totpSecret); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create config
	cfg := &config.Config{
		SessionTimeout: 3600,
	}

	// Create handler (we need templates for this, so we'll skip the actual HTTP test)
	// Instead, we'll test the database session flow directly
	
	// Step 1: Create initial session (as if login succeeded)
	sessionID, err := db.CreateSession("testuser", false)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	t.Logf("Created session: %s", sessionID)

	// Step 2: Verify initial session state
	session, err := db.GetSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if session == nil {
		t.Fatal("Session is nil")
	}
	if session.Authenticated2FA {
		t.Error("Expected Authenticated2FA to be false initially")
	}
	t.Logf("Initial session state: authenticated_2fa=%v", session.Authenticated2FA)

	// Step 3: Simulate 2FA success - update session
	if err := db.UpdateSession(sessionID, true); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}
	t.Logf("Updated session to mark 2FA complete")

	// Step 4: THIS IS THE KEY TEST - Re-query the session to verify the update
	// This simulates what happens when the middleware reads the session on the next request
	updatedSession, err := db.GetSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to get updated session: %v", err)
	}
	if updatedSession == nil {
		t.Fatal("Updated session is nil")
	}
	
	// The critical assertion: The session should now show authenticated_2fa=true
	if !updatedSession.Authenticated2FA {
		t.Errorf("FAIL: Session update not visible! authenticated_2fa is still false after UpdateSession")
		t.Logf("This indicates the redirect loop bug is still present")
	} else {
		t.Logf("SUCCESS: Session update is visible, authenticated_2fa=%v", updatedSession.Authenticated2FA)
	}

	// Step 5: Verify the middleware would allow access
	middleware := NewMiddleware(db, cfg)
	
	// Create a test request with the session cookie
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: sessionID,
	})

	// Create a simple handler that tracks if it was called
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	wrappedHandler := middleware.RequireAuth(handler)

	// Execute the request
	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	// Check that we were NOT redirected (which would be a 302)
	if rr.Code == http.StatusFound {
		location := rr.Header().Get("Location")
		t.Errorf("FAIL: Middleware redirected to %s (status %d) - this indicates the redirect loop bug", 
			location, rr.Code)
		t.Logf("This means the middleware is not seeing the updated session state")
	} else if !handlerCalled {
		t.Error("FAIL: Handler was not called, but no redirect occurred either")
	} else {
		t.Logf("SUCCESS: Middleware allowed access without redirect (status %d)", rr.Code)
	}
}

// TestSessionUpdateVerification verifies the new verification step in Show2FA
func TestSessionUpdateVerification(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create session
	sessionID, err := db.CreateSession("testuser", false)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Update session
	if err := db.UpdateSession(sessionID, true); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	// Immediately re-verify (this is what the fix does in auth.go)
	verifiedSession, err := db.GetSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to verify session: %v", err)
	}

	if !verifiedSession.Authenticated2FA {
		t.Errorf("Session verification failed - authenticated_2fa=%v, expected true", 
			verifiedSession.Authenticated2FA)
		t.Logf("This would cause a redirect to /login in the actual code")
	} else {
		t.Logf("Session verification succeeded - authenticated_2fa=%v", 
			verifiedSession.Authenticated2FA)
	}
}

// TestAuthenticatedUserRedirect verifies that authenticated users are redirected away from login/2FA pages
func TestAuthenticatedUserRedirect(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test user
	totpSecret := "JBSWY3DPEHPK3PXP"
	if err := db.CreateUser("testuser", "testpass", totpSecret); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create fully authenticated session (2FA complete)
	sessionID, err := db.CreateSession("testuser", true)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create config
	cfg := &config.Config{
		SessionTimeout: 3600,
	}

	// Create auth handler
	templatesPath := filepath.Join("..", "..", "web", "templates")
	authHandler, err := NewAuthHandler(db, cfg, templatesPath)
	if err != nil {
		t.Fatalf("Failed to create auth handler: %v", err)
	}

	// Test 1: Authenticated user tries to access /login GET
	t.Run("Login page redirects authenticated user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/login", nil)
		req.AddCookie(&http.Cookie{
			Name:  SessionCookieName,
			Value: sessionID,
		})
		
		rr := httptest.NewRecorder()
		authHandler.ShowLogin(rr, req)

		// Should redirect to /
		if rr.Code != http.StatusFound {
			t.Errorf("Expected redirect (302), got status %d", rr.Code)
		}
		
		location := rr.Header().Get("Location")
		if location != "/" {
			t.Errorf("Expected redirect to /, got %s", location)
		}
		
		t.Logf("SUCCESS: Authenticated user redirected from /login to %s", location)
	})

	// Test 2: Authenticated user tries to access /2fa GET
	t.Run("2FA page redirects authenticated user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/2fa", nil)
		req.AddCookie(&http.Cookie{
			Name:  SessionCookieName,
			Value: sessionID,
		})
		
		rr := httptest.NewRecorder()
		authHandler.Show2FA(rr, req)

		// Should redirect to /
		if rr.Code != http.StatusFound {
			t.Errorf("Expected redirect (302), got status %d", rr.Code)
		}
		
		location := rr.Header().Get("Location")
		if location != "/" {
			t.Errorf("Expected redirect to /, got %s", location)
		}
		
		t.Logf("SUCCESS: Authenticated user redirected from /2fa to %s", location)
	})

	// Test 3: Unauthenticated user can still access /login
	t.Run("Login page shows for unauthenticated user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/login", nil)
		// No cookie
		
		rr := httptest.NewRecorder()
		authHandler.ShowLogin(rr, req)

		// Should show the page (200)
		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got status %d", rr.Code)
		}
		
		t.Logf("SUCCESS: Unauthenticated user can access login page")
	})
}
