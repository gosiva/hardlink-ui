package api

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gosiva/hardlink-ui/internal/config"
	"github.com/gosiva/hardlink-ui/internal/scanner"
	"github.com/gosiva/hardlink-ui/internal/storage"
)

// TestSSEIntegrationWithMiddleware is an end-to-end test that verifies SSE works through the full middleware stack
func TestSSEIntegrationWithMiddleware(t *testing.T) {
	// Create temporary database and data directory
	tmpDir := t.TempDir()
	db, err := storage.New(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create config
	cfg := &config.Config{
		DataRoot:       tmpDir + "/data",
		SessionTimeout: 3600,
	}

	// Create scanner
	scan := scanner.NewScanner(db, cfg.DataRoot)

	// Create handlers
	dupHandler := NewDuplicatesHandler(db, cfg, scan)
	middleware := NewMiddleware(db, cfg)

	// Start a scan job
	jobID := "integration-test-job"
	if err := scan.StartScan(jobID); err != nil {
		t.Fatalf("Failed to start scan: %v", err)
	}

	// Give the scan a moment to initialize
	time.Sleep(100 * time.Millisecond)

	// Create a test server with the actual middleware stack
	handler := http.HandlerFunc(dupHandler.GetProgress)
	wrappedHandler := middleware.Logging(handler)

	// Create test request
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/duplicates/progress?job_id=%s", jobID), nil)
	req.Header.Set("Origin", "http://localhost:8000")

	// Use a custom ResponseRecorder that supports Flusher
	rec := &flushableRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		flushed:          false,
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(req.Context(), 1*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Call handler in goroutine
	done := make(chan bool)
	go func() {
		wrappedHandler.ServeHTTP(rec, req)
		done <- true
	}()

	// Wait for some data
	time.Sleep(300 * time.Millisecond)

	// Get response
	resp := rec.Result()
	body := rec.Body.String()

	// Verify SSE headers are set correctly
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Errorf("Expected SSE Content-Type, got: %s", resp.Header.Get("Content-Type"))
	}

	if resp.Header.Get("Cache-Control") != "no-cache, no-transform" {
		t.Errorf("Expected Cache-Control: no-cache, no-transform, got: %s", resp.Header.Get("Cache-Control"))
	}

	// Verify we got SSE data
	if !strings.Contains(body, "event: connected") {
		t.Errorf("Expected connected event, got: %s", body)
	}

	if !strings.Contains(body, jobID) {
		t.Errorf("Expected job_id in response, got: %s", body)
	}

	// Verify Flush was called (proves Flusher interface works through middleware)
	if !rec.flushed {
		t.Error("Expected Flush to be called through middleware, but it wasn't")
	}

	// Parse SSE data format
	scanner := bufio.NewScanner(strings.NewReader(body))
	foundDataLine := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			foundDataLine = true
			data := strings.TrimPrefix(line, "data: ")
			if !strings.HasPrefix(data, "{") {
				t.Errorf("Expected JSON data, got: %s", data)
			}
		}
	}

	if !foundDataLine {
		t.Error("Expected at least one data line in SSE stream")
	}

	// Wait for handler to complete
	select {
	case <-done:
		t.Log("Handler completed normally")
	case <-time.After(2 * time.Second):
		t.Log("Handler timeout (expected for SSE)")
	}
}

// flushableRecorder is a ResponseRecorder that tracks Flush calls
type flushableRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *flushableRecorder) Flush() {
	f.flushed = true
	// httptest.ResponseRecorder doesn't actually implement Flusher,
	// but our test verifies the Flush method is called
}

// TestMiddlewarePreservesSSEFunctionality verifies middleware doesn't break SSE
func TestMiddlewarePreservesSSEFunctionality(t *testing.T) {
	// This test verifies that wrapping a handler with middleware doesn't break SSE

	tmpDir := t.TempDir()
	db, err := storage.New(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	middleware := NewMiddleware(db, cfg)

	// Create a simple SSE handler
	sseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This is the pattern used in GetProgress
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: test\n\n")
		flusher.Flush()
	})

	// Wrap with all middleware layers
	wrappedHandler := middleware.CORS(middleware.Logging(sseHandler))

	// Test the wrapped handler
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Verify no error (status should be 200, not 500)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got: %d - Body: %s", rec.Code, rec.Body.String())
	}

	// Verify SSE data was sent
	body := rec.Body.String()
	if body != "data: test\n\n" {
		t.Errorf("Expected SSE data, got: %s", body)
	}
}
