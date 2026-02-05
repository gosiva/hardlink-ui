package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gosiva/hardlink-ui/internal/config"
	"github.com/gosiva/hardlink-ui/internal/scanner"
	"github.com/gosiva/hardlink-ui/internal/storage"
)

// TestDuplicatesProgressSSE verifies that SSE endpoint works correctly
func TestDuplicatesProgressSSE(t *testing.T) {
	// Create temporary database and data directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dataDir := filepath.Join(tmpDir, "data")
	
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create scanner
	scan := scanner.NewScanner(db, dataDir)
	
	// Create config
	cfg := &config.Config{
		DataRoot: dataDir,
	}

	// Create handler
	handler := NewDuplicatesHandler(db, cfg, scan)

	// Start a scan job to test against
	err = scan.StartScan("test-job-123")
	if err != nil {
		t.Fatalf("Failed to start scan: %v", err)
	}

	// Give the scan a moment to initialize
	time.Sleep(100 * time.Millisecond)

	// Create test request
	req := httptest.NewRequest("GET", "/api/duplicates/progress?job_id=test-job-123", nil)
	req.Header.Set("Origin", "http://localhost:8000")
	
	// Create response recorder
	w := httptest.NewRecorder()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Call handler in a goroutine since it's a long-running SSE stream
	done := make(chan bool)
	go func() {
		handler.GetProgress(w, req)
		done <- true
	}()

	// Wait a bit for initial data
	time.Sleep(200 * time.Millisecond)

	// Get the response
	resp := w.Result()
	defer resp.Body.Close()

	// Test 1: Verify SSE headers
	t.Run("SSE Headers", func(t *testing.T) {
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/event-stream") {
			t.Errorf("Expected Content-Type to contain 'text/event-stream', got: %s", contentType)
		}
		
		if !strings.Contains(contentType, "charset=utf-8") {
			t.Errorf("Expected Content-Type to contain 'charset=utf-8', got: %s", contentType)
		}

		cacheControl := resp.Header.Get("Cache-Control")
		if !strings.Contains(cacheControl, "no-cache") {
			t.Errorf("Expected Cache-Control to contain 'no-cache', got: %s", cacheControl)
		}
		
		if !strings.Contains(cacheControl, "no-transform") {
			t.Errorf("Expected Cache-Control to contain 'no-transform', got: %s", cacheControl)
		}

		connection := resp.Header.Get("Connection")
		if connection != "keep-alive" {
			t.Errorf("Expected Connection: keep-alive, got: %s", connection)
		}

		xAccelBuffering := resp.Header.Get("X-Accel-Buffering")
		if xAccelBuffering != "no" {
			t.Errorf("Expected X-Accel-Buffering: no, got: %s", xAccelBuffering)
		}

		contentEncoding := resp.Header.Get("Content-Encoding")
		if contentEncoding != "identity" {
			t.Errorf("Expected Content-Encoding: identity, got: %s", contentEncoding)
		}
	})

	// Test 2: Verify CORS headers for reverse proxy support
	t.Run("CORS Headers", func(t *testing.T) {
		allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
		// Origin should be allowed because it's localhost
		if allowOrigin == "" {
			t.Errorf("Expected Access-Control-Allow-Origin to be set for localhost origin, got empty")
		}
		if allowOrigin != "" && allowOrigin != "http://localhost:8000" {
			t.Errorf("Expected Access-Control-Allow-Origin to match localhost:8000, got: %s", allowOrigin)
		}

		allowCredentials := resp.Header.Get("Access-Control-Allow-Credentials")
		if allowOrigin != "" && allowCredentials != "true" {
			t.Errorf("Expected Access-Control-Allow-Credentials: true when Origin is allowed, got: %s", allowCredentials)
		}
	})

	// Test 3: Verify initial ping and connected event are sent
	t.Run("Initial Ping and Connected Event", func(t *testing.T) {
		body := w.Body.String()
		
		// Verify initial ping comment frame
		if !strings.Contains(body, ": ping") {
			t.Errorf("Expected initial ping comment (': ping') in SSE stream, got: %s", body)
		}

		// Verify ping appears before connected event for early bytes
		pingIndex := strings.Index(body, ": ping")
		connectedIndex := strings.Index(body, "event: connected")
		if pingIndex == -1 || connectedIndex == -1 || pingIndex >= connectedIndex {
			t.Errorf("Expected ping comment to appear before connected event")
		}
		
		if !strings.Contains(body, "event: connected") {
			t.Errorf("Expected initial 'connected' event in SSE stream, got: %s", body)
		}
		
		if !strings.Contains(body, "test-job-123") {
			t.Errorf("Expected job_id in connected event, got: %s", body)
		}
	})

	// Test 4: Verify SSE data format
	t.Run("SSE Data Format", func(t *testing.T) {
		body := w.Body.String()
		scanner := bufio.NewScanner(strings.NewReader(body))
		
		foundDataLine := false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				foundDataLine = true
				// Verify it looks like JSON
				data := strings.TrimPrefix(line, "data: ")
				if !strings.HasPrefix(data, "{") {
					t.Errorf("Expected JSON data, got: %s", data)
				}
			}
		}
		
		if !foundDataLine {
			t.Errorf("Expected at least one 'data:' line in SSE stream")
		}
	})

	// Wait for handler to finish or timeout
	select {
	case <-done:
		t.Log("Handler completed normally")
	case <-time.After(3 * time.Second):
		t.Log("Handler timeout (expected for SSE)")
	}
}

// TestDuplicatesProgressSSE_MissingJobID verifies error handling for missing job_id
func TestDuplicatesProgressSSE_MissingJobID(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dataDir := filepath.Join(tmpDir, "data")
	
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	scan := scanner.NewScanner(db, dataDir)
	cfg := &config.Config{DataRoot: dataDir}
	handler := NewDuplicatesHandler(db, cfg, scan)

	// Request without job_id
	req := httptest.NewRequest("GET", "/api/duplicates/progress", nil)
	w := httptest.NewRecorder()

	handler.GetProgress(w, req)

	resp := w.Result()
	// When job_id is empty from both query param and path, we expect 400
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got: %d", resp.StatusCode)
	}
}

// TestDuplicatesProgressSSE_InvalidJobID verifies error handling for non-existent job
func TestDuplicatesProgressSSE_InvalidJobID(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dataDir := filepath.Join(tmpDir, "data")
	
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	scan := scanner.NewScanner(db, dataDir)
	cfg := &config.Config{DataRoot: dataDir}
	handler := NewDuplicatesHandler(db, cfg, scan)

	// Request with non-existent job_id
	req := httptest.NewRequest("GET", "/api/duplicates/progress?job_id=nonexistent", nil)
	req.Header.Set("Origin", "http://localhost:8000")
	
	w := httptest.NewRecorder()

	handler.GetProgress(w, req)

	resp := w.Result()
	
	// Should get 404 since job doesn't exist (verified before SSE stream starts)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent job, got: %d", resp.StatusCode)
	}
}

// TestCORSOriginValidation tests the CORS origin validation logic
func TestCORSOriginValidation(t *testing.T) {
tmpDir := t.TempDir()
dbPath := filepath.Join(tmpDir, "test.db")
dataDir := filepath.Join(tmpDir, "data")

db, err := storage.New(dbPath)
if err != nil {
t.Fatalf("Failed to create database: %v", err)
}
defer db.Close()

scan := scanner.NewScanner(db, dataDir)
cfg := &config.Config{DataRoot: dataDir}
handler := NewDuplicatesHandler(db, cfg, scan)

// Start a scan job
err = scan.StartScan("test-cors-job")
if err != nil {
t.Fatalf("Failed to start scan: %v", err)
}
time.Sleep(100 * time.Millisecond)

tests := []struct {
name           string
origin         string
host           string
shouldAllow    bool
}{
{
name:        "Same origin with http",
origin:      "http://example.com:8000",
host:        "example.com:8000",
shouldAllow: true,
},
{
name:        "Same origin with https",
origin:      "https://example.com:8000",
host:        "example.com:8000",
shouldAllow: true,
},
{
name:        "Localhost with port",
origin:      "http://localhost:3000",
host:        "example.com:8000",
shouldAllow: true,
},
{
name:        "127.0.0.1",
origin:      "http://127.0.0.1:8000",
host:        "example.com:8000",
shouldAllow: true,
},
{
name:        "Different origin should be blocked",
origin:      "http://evil.com",
host:        "example.com:8000",
shouldAllow: false,
},
{
name:        "Empty origin",
origin:      "",
host:        "example.com:8000",
shouldAllow: false,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
req := httptest.NewRequest("GET", "/api/duplicates/progress?job_id=test-cors-job", nil)
if tt.origin != "" {
req.Header.Set("Origin", tt.origin)
}
req.Host = tt.host

w := httptest.NewRecorder()

// Use context with timeout
ctx, cancel := context.WithTimeout(req.Context(), 1*time.Second)
defer cancel()
req = req.WithContext(ctx)

done := make(chan bool)
go func() {
handler.GetProgress(w, req)
done <- true
}()

time.Sleep(100 * time.Millisecond)

resp := w.Result()
allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")

if tt.shouldAllow {
if allowOrigin == "" {
t.Errorf("Expected CORS headers to be set, but Access-Control-Allow-Origin is empty")
}
if allowOrigin != "" && resp.Header.Get("Access-Control-Allow-Credentials") != "true" {
t.Errorf("Expected Access-Control-Allow-Credentials: true when origin is allowed")
}
} else {
if allowOrigin != "" {
t.Errorf("Expected no CORS headers for disallowed origin, but got Access-Control-Allow-Origin: %s", allowOrigin)
}
}

<-done
})
}
}
