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
    "strconv"
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
        parts := strings.Split(r.URL.Path, "/")
        if len(parts) > 0 {
            lastPart := parts[len(parts)-1]
            if lastPart != "" && lastPart != "progress" {
                jobID = lastPart
            }
        }
    }

    if jobID == "" {
        JSONError(w, http.StatusBadRequest, "Job ID is required")
        return
    }

    username := GetUsername(r)
    remoteAddr := r.RemoteAddr

    progress := h.scanner.GetProgress(jobID)
    if progress == nil {
        JSONError(w, http.StatusNotFound, "Job not found")
        return
    }

    w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
    w.Header().Set("Cache-Control", "no-cache, no-transform")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")
    w.Header().Set("Content-Encoding", "identity")

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

    fmt.Fprintf(w, ": ping\n\n")
    flusher.Flush()

    connectedEvent := fmt.Sprintf("event: connected\ndata: {\"job_id\": \"%s\"}\n\n", jobID)
    fmt.Fprint(w, connectedEvent)
    flusher.Flush()

    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()

    heartbeatTicker := time.NewTicker(15 * time.Second)
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
                return
            }

            data, _ := json.Marshal(progress)
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
            eventCount++

            if progress.Status != lastProgressStatus {
                log.Printf("SSE SENT job=%s event=progress status=%s processed=%d total=%d",
                    jobID, progress.Status, progress.Processed, progress.TotalFiles)
                lastProgressStatus = progress.Status
            }

            if progress.Status == "completed" || progress.Status == "failed" {
                log.Printf("SSE DISCONNECT job=%s status=%s", jobID, progress.Status)
                return
            }

        case <-heartbeatTicker.C:
            fmt.Fprintf(w, ": heartbeat\n\n")
            flusher.Flush()

        case <-timeout:
            fmt.Fprintf(w, "event: timeout\ndata: {\"error\": \"Timeout\"}\n\n")
            flusher.Flush()
            return

        case <-r.Context().Done():
            return
        }
    }
}

func isAllowedOrigin(origin, host string) bool {
    if strings.HasPrefix(origin, "http://"+host) || strings.HasPrefix(origin, "https://"+host) {
        return true
    }
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
        parts := strings.Split(r.URL.Path, "/")
        if len(parts) > 0 {
            lastPart := parts[len(parts)-1]
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

    // Read PUID/PGID from environment
    uidStr := os.Getenv("PUID")
    gidStr := os.Getenv("PGID")
    uid, _ := strconv.Atoi(uidStr)
    gid, _ := strconv.Atoi(gidStr)

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

        // Normalize master permissions
        if uid > 0 && gid > 0 {
            _ = os.Chown(masterPath, uid, gid)
            _ = os.Chmod(masterPath, 0o644)
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

            // Normalize duplicate permissions
            if uid > 0 && gid > 0 {
                _ = os.Chown(otherPath, uid, gid)
                _ = os.Chmod(otherPath, 0o644)
            }

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

            // Create a temporary hardlink path
            tmpPath := otherPath + ".tmp-hardlink"

            // 1. Create temporary hardlink
            if err := os.Link(masterPath, tmpPath); err != nil {
                errors = append(errors, fmt.Sprintf("%s: failed to create temporary hardlink: %v", otherRel, err))
                continue
            }

            // ❌ DO NOT CHOWN THE HARDLINK — impossible and useless
            // It shares the inode with master, so permissions come from master.

            // 2. Remove original duplicate
            if err := os.Remove(otherPath); err != nil {
                _ = os.Remove(tmpPath)
                errors = append(errors, fmt.Sprintf("%s: failed to remove original file: %v", otherRel, err))
                continue
            }

            // 3. Rename temporary hardlink to original filename
            if err := os.Rename(tmpPath, otherPath); err != nil {
                _ = os.Remove(tmpPath)
                errors = append(errors, fmt.Sprintf("%s: failed to rename temporary hardlink: %v", otherRel, err))
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
