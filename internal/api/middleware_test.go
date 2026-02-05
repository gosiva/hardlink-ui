package api

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gosiva/hardlink-ui/internal/config"
	"github.com/gosiva/hardlink-ui/internal/storage"
)

// TestResponseWriterFlusher verifies that the responseWriter wrapper properly forwards the Flusher interface
func TestResponseWriterFlusher(t *testing.T) {
	// Create a test handler that requires Flusher
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to get Flusher interface
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// If we got here, Flusher is supported
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: test\n\n"))
		flusher.Flush()
	})

	// Wrap with logging middleware
	tmpDir := t.TempDir()
	db, err := storage.New(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	middleware := NewMiddleware(db, cfg)
	wrappedHandler := middleware.Logging(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Call the wrapped handler
	wrappedHandler.ServeHTTP(w, req)

	// Verify response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body := w.Body.String()
	if body != "data: test\n\n" {
		t.Errorf("Expected SSE data, got: %s", body)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type: text/event-stream, got: %s", contentType)
	}
}

// TestResponseWriterHijacker verifies that Hijacker interface is properly forwarded
func TestResponseWriterHijacker(t *testing.T) {
	rw := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     http.StatusOK,
	}

	// httptest.ResponseRecorder doesn't implement Hijacker, so this should return an error
	_, _, err := rw.Hijack()
	if err != http.ErrNotSupported {
		t.Errorf("Expected ErrNotSupported, got: %v", err)
	}
}

// TestResponseWriterPush verifies that Pusher interface is properly forwarded
func TestResponseWriterPush(t *testing.T) {
	rw := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     http.StatusOK,
	}

	// httptest.ResponseRecorder doesn't implement Pusher, so this should return an error
	err := rw.Push("/test", nil)
	if err != http.ErrNotSupported {
		t.Errorf("Expected ErrNotSupported, got: %v", err)
	}
}

// mockFlusher is a mock ResponseWriter that implements http.Flusher
type mockFlusher struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (m *mockFlusher) Flush() {
	m.flushed = true
}

// TestResponseWriterFlushCalled verifies that Flush is actually called on the underlying writer
func TestResponseWriterFlushCalled(t *testing.T) {
	mock := &mockFlusher{
		ResponseRecorder: httptest.NewRecorder(),
		flushed:          false,
	}

	rw := &responseWriter{
		ResponseWriter: mock,
		statusCode:     http.StatusOK,
	}

	// Call Flush
	rw.Flush()

	// Verify it was called on the underlying writer
	if !mock.flushed {
		t.Error("Expected Flush to be called on underlying writer")
	}
}

// mockReaderFrom is a mock ResponseWriter that implements io.ReaderFrom
type mockReaderFrom struct {
	*httptest.ResponseRecorder
	readFromCalled bool
	bytesRead      int64
}

func (m *mockReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	m.readFromCalled = true
	n, err := io.Copy(m.ResponseRecorder, r)
	m.bytesRead = n
	return n, err
}

// TestResponseWriterReadFrom verifies that ReadFrom is properly forwarded
func TestResponseWriterReadFrom(t *testing.T) {
	// Verify that responseWriter can be used as io.ReaderFrom when underlying supports it
	mock := &mockReaderFrom{
		ResponseRecorder: httptest.NewRecorder(),
	}

	rw := &responseWriter{
		ResponseWriter: mock,
		statusCode:     http.StatusOK,
	}

	// Test that interface assertion works
	if _, ok := interface{}(rw).(io.ReaderFrom); !ok {
		t.Error("Expected responseWriter to implement io.ReaderFrom when underlying writer does")
	}
}

// mockHijacker is a mock ResponseWriter that implements http.Hijacker
type mockHijacker struct {
	*httptest.ResponseRecorder
	hijackCalled bool
}

func (m *mockHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijackCalled = true
	return nil, nil, nil
}

// TestResponseWriterHijackCalled verifies that Hijack is called on the underlying writer
func TestResponseWriterHijackCalled(t *testing.T) {
	mock := &mockHijacker{
		ResponseRecorder: httptest.NewRecorder(),
	}

	rw := &responseWriter{
		ResponseWriter: mock,
		statusCode:     http.StatusOK,
	}

	// Call Hijack
	_, _, _ = rw.Hijack()

	// Verify it was called on the underlying writer
	if !mock.hijackCalled {
		t.Error("Expected Hijack to be called on underlying writer")
	}
}
