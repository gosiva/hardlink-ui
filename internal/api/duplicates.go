package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gosiva/hardlink-ui/internal/config"
	"github.com/gosiva/hardlink-ui/internal/scanner"
	"github.com/gosiva/hardlink-ui/internal/storage"
)

// DuplicatesHandler handles duplicate scanning and conversion
type DuplicatesHandler struct {
	db      *storage.DB
	cfg     *config.Config
	scanner *scanner.Scanner
}

// NewDuplicatesHandler creates a new duplicates handler
func NewDuplicatesHandler(db *storage.DB, cfg *config.Config, scan *scanner.Scanner) *DuplicatesHandler {
	return &DuplicatesHandler{
		db:      db,
		cfg:     cfg,
		scanner: scan,
	}
}

// StartScan starts a duplicate scan job
func (h *DuplicatesHandler) StartScan(w http.ResponseWriter, r *http.Request) {
	// Generate job ID
	jobID, err := generateJobID()
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "Failed to generate job ID")
		return
	}

	// Start scan in background
	if err := h.scanner.StartScan(jobID); err != nil {
		JSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start scan: %v", err))
		return
	}

	log.Printf("DUPSCAN START job=%s by %s", jobID, GetUsername(r))

	JSONResponse(w, http.StatusOK, map[string]string{
		"job_id": jobID,
	})
}

// GetProgress streams scan progress via SSE
func (h *DuplicatesHandler) GetProgress(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		// Try path parameter
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			// Only use path parameter if it's not "progress" (the endpoint name)
			if lastPart != "" && lastPart != "progress" {
				jobID = lastPart
			}
		}
	}

	if jobID == "" {
		JSONError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	// Get username and remote address for logging
	username := GetUsername(r)
	remoteAddr := r.RemoteAddr

	// Verify job exists before starting SSE stream
	progress := h.scanner.GetProgress(jobID)
	if progress == nil {
		JSONError(w, http.StatusNotFound, "Job not found")
		return
	}

	// Set SSE headers - critical for reverse proxy compatibility
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")         // Disable nginx buffering
	w.Header().Set("Content-Encoding", "identity")    // Avoid proxy compression

	// For SSE to work behind reverse proxies (e.g., Synology), we need proper CORS headers
	// but only for same-origin or localhost requests (security)
	origin := r.Header.Get("Origin")
	if origin != "" && isAllowedOrigin(origin, r.Host) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("SSE CONNECT job=%s user=%s remote=%s flusher=NO", jobID, username, remoteAddr)
		JSONError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	log.Printf("SSE CONNECT job=%s user=%s remote=%s flusher=YES", jobID, username, remoteAddr)

	// Send immediate initial ping comment frame to ensure early bytes for reverse proxy
	fmt.Fprintf(w, ": ping\n\n")
	flusher.Flush()

	// Send initial connected event immediately to confirm SSE connection
	connectedEvent := fmt.Sprintf("event: connected\ndata: {\"job_id\": \"%s\"}\n\n", jobID)
	fmt.Fprint(w, connectedEvent)
	flusher.Flush()
	log.Printf("SSE SENT job=%s event=connected bytes=%d", jobID, len(connectedEvent))

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	heartbeatTicker := time.NewTicker(15 * time.Second) // Heartbeat every 15 seconds
	defer heartbeatTicker.Stop()

	timeout := time.After(10 * time.Minute)

	lastProgressStatus := ""
	eventCount := 0

	for {
		select {
		case <-ticker.C:
			progress := h.scanner.GetProgress(jobID)
			if progress == nil {
				fmt.Fprintf(w, "event: error\ndata: {\"error\": \"Job not found\"}\n\n")
				flusher.Flush()
				log.Printf("SSE ERROR job=%s error=job_not_found", jobID)
				return
			}

			data, _ := json.Marshal(progress)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			eventCount++

			// Log only when status changes to reduce log noise
			if progress.Status != lastProgressStatus {
				log.Printf("SSE SENT job=%s event=progress status=%s processed=%d total=%d bytes=%d events=%d",
					jobID, progress.Status, progress.Processed, progress.TotalFiles, len(data), eventCount)
				lastProgressStatus = progress.Status
			}

			// If completed or failed, close the stream
			if progress.Status == "completed" || progress.Status == "failed" {
				log.Printf("SSE DISCONNECT job=%s status=%s events_sent=%d", jobID, progress.Status, eventCount)
				return
			}

		case <-heartbeatTicker.C:
			// Send periodic heartbeat comment to keep connection alive
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
			log.Printf("SSE HEARTBEAT job=%s events=%d", jobID, eventCount)

		case <-timeout:
			fmt.Fprintf(w, "event: timeout\ndata: {\"error\": \"Timeout\"}\n\n")
			flusher.Flush()
			log.Printf("SSE TIMEOUT job=%s events_sent=%d", jobID, eventCount)
			return

		case <-r.Context().Done():
			log.Printf("SSE DISCONNECT job=%s reason=client_disconnect events_sent=%d", jobID, eventCount)
			return
		}
	}
}

// isAllowedOrigin checks if the origin is allowed for CORS
// This allows same-origin requests and localhost for development
func isAllowedOrigin(origin, host string) bool {
	// Parse origin to get host
	// Origin format: scheme://host:port
	if strings.HasPrefix(origin, "http://"+host) || strings.HasPrefix(origin, "https://"+host) {
		return true
	}

	// Allow localhost for development (any port)
	if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "https://localhost") {
		return true
	}
	if strings.HasPrefix(origin, "http://127.0.0.1") || strings.HasPrefix(origin, "https://127.0.0.1") {
		return true
	}

	return false
}

// GetResults returns the results of a completed scan
func (h *DuplicatesHandler) GetResults(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		// Try path parameter
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			// Only use path parameter if it's not "results" (the endpoint name)
			if lastPart != "" && lastPart != "results" {
				jobID = lastPart
			}
		}
	}

	if jobID == "" {
		JSONError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	progress := h.scanner.GetProgress(jobID)
	if progress == nil {
		JSONError(w, http.StatusNotFound, "Job not found")
		return
	}

	if progress.Status != "completed" {
		JSONError(w, http.StatusBadRequest, fmt.Sprintf("Job not completed, status: %s", progress.Status))
		return
	}

	JSONResponse(w, http.StatusOK, map[string]interface{}{
		"items": progress.Results,
	})
}

// ConvertDuplicatesRequest represents a conversion request
type ConvertDuplicatesRequest struct {
	Groups []struct {
		Master string   `json:"master"`
		Others []string `json:"others"`
	} `json:"groups"`
}

// ConvertDuplicates converts duplicate files to hardlinks
func (h *DuplicatesHandler) ConvertDuplicates(w http.ResponseWriter, r *http.Request) {
	var req ConvertDuplicatesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	totalCreated := 0
	totalBytesSaved := int64(0)
	var errors []string

	log.Printf("DUPCONVERT START groups=%d by %s", len(req.Groups), GetUsername(r))

	for _, group := range req.Groups {
		if group.Master == "" || len(group.Others) == 0 {
			continue
		}

		masterPath := filepath.Join(h.cfg.DataRoot, strings.TrimPrefix(group.Master, "/"))
		masterPath = filepath.Clean(masterPath)

		if !strings.HasPrefix(masterPath, h.cfg.DataRoot) {
			errors = append(errors, fmt.Sprintf("Master path outside root: %s", group.Master))
			continue
		}

		masterInfo, err := os.Stat(masterPath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to stat master %s: %v", group.Master, err))
			continue
		}

		if !masterInfo.Mode().IsRegular() {
			errors = append(errors, fmt.Sprintf("Master is not a regular file: %s", group.Master))
			continue
		}

		for _, otherRel := range group.Others {
			otherPath := filepath.Join(h.cfg.DataRoot, strings.TrimPrefix(otherRel, "/"))
			otherPath = filepath.Clean(otherPath)

			if !strings.HasPrefix(otherPath, h.cfg.DataRoot) {
				errors = append(errors, fmt.Sprintf("Path outside root: %s", otherRel))
				continue
			}

			otherInfo, err := os.Stat(otherPath)
			if err != nil {
				if os.IsNotExist(err) {
					errors = append(errors, fmt.Sprintf("File not found: %s", otherRel))
				} else {
					errors = append(errors, fmt.Sprintf("Failed to stat %s: %v", otherRel, err))
				}
				continue
			}

			size := otherInfo.Size()

			// Verify files are identical
			identical, err := scanner.VerifyFilesIdentical(masterPath, otherPath)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: verification error: %v", otherRel, err))
				continue
			}

			if !identical {
				errors = append(errors, fmt.Sprintf("%s: files are not identical", otherRel))
				continue
			}

			// Delete the duplicate
			if err := os.Remove(otherPath); err != nil {
				errors = append(errors, fmt.Sprintf("%s: failed to remove: %v", otherRel, err))
				continue
			}

			// Create hardlink
			if err := os.Link(masterPath, otherPath); err != nil {
				errors = append(errors, fmt.Sprintf("%s: failed to create hardlink: %v", otherRel, err))
				// Try to restore by recreating the file would be complex, so we just report error
				continue
			}

			totalCreated++
			totalBytesSaved += size
		}
	}

	log.Printf("DUPCONVERT END created=%d bytes_saved=%d errors=%d", totalCreated, totalBytesSaved, len(errors))

	response := map[string]interface{}{
		"ok":                true,
		"created":           totalCreated,
		"bytes_saved":       totalBytesSaved,
		"bytes_saved_human": humanSize(totalBytesSaved),
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	JSONResponse(w, http.StatusOK, response)
}

func generateJobID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
