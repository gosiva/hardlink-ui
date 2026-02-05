package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gosiva/hardlink-ui/internal/config"
	"github.com/gosiva/hardlink-ui/internal/storage"
)

// HardlinkHandler handles hardlink operations
type HardlinkHandler struct {
	db  *storage.DB
	cfg *config.Config
}

// NewHardlinkHandler creates a new hardlink handler
func NewHardlinkHandler(db *storage.DB, cfg *config.Config) *HardlinkHandler {
	return &HardlinkHandler{
		db:  db,
		cfg: cfg,
	}
}

// CreateHardlinkRequest represents a hardlink creation request
type CreateHardlinkRequest struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}

// CreateHardlink creates a single hardlink
func (h *HardlinkHandler) CreateHardlink(w http.ResponseWriter, r *http.Request) {
	var req CreateHardlinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Source == "" || req.Dest == "" {
		JSONError(w, http.StatusBadRequest, "Source and destination are required")
		return
	}

	srcPath := filepath.Join(h.cfg.DataRoot, strings.TrimPrefix(req.Source, "/"))
	srcPath = filepath.Clean(srcPath)

	destPath := filepath.Join(h.cfg.DataRoot, strings.TrimPrefix(req.Dest, "/"))
	destPath = filepath.Clean(destPath)

	if !strings.HasPrefix(srcPath, h.cfg.DataRoot) || !strings.HasPrefix(destPath, h.cfg.DataRoot) {
		JSONError(w, http.StatusBadRequest, "Path outside root")
		return
	}

	// Verify source exists and is a file
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		JSONError(w, http.StatusNotFound, fmt.Sprintf("Source not found: %v", err))
		return
	}

	if srcInfo.IsDir() {
		JSONError(w, http.StatusBadRequest, "Source must be a file, not a directory")
		return
	}

	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		JSONError(w, http.StatusConflict, "Destination already exists")
		return
	}

	// Create parent directories if needed
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		JSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create parent directory: %v", err))
		return
	}

	// Create hardlink
	if err := os.Link(srcPath, destPath); err != nil {
		JSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create hardlink: %v", err))
		return
	}

	// Update inode index
	srcStat := srcInfo.Sys()
	if stat, ok := srcStat.(*syscall.Stat_t); ok {
		h.db.AddInodePath(stat.Ino, destPath)
	}

	log.Printf("HARDLINK CREATE %s -> %s by %s", srcPath, destPath, GetUsername(r))
	JSONResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

// CreateHardlinksFolderRequest represents a bulk folder hardlink request
type CreateHardlinksFolderRequest struct {
	Source   string `json:"source"`
	DestRoot string `json:"dest_root"`
}

// CreateHardlinksFolder creates hardlinks for an entire folder
func (h *HardlinkHandler) CreateHardlinksFolder(w http.ResponseWriter, r *http.Request) {
	var req CreateHardlinksFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Source == "" || req.DestRoot == "" {
		JSONError(w, http.StatusBadRequest, "Source and destination root are required")
		return
	}

	srcPath := filepath.Join(h.cfg.DataRoot, strings.TrimPrefix(req.Source, "/"))
	srcPath = filepath.Clean(srcPath)

	destRootPath := filepath.Join(h.cfg.DataRoot, strings.TrimPrefix(req.DestRoot, "/"))
	destRootPath = filepath.Clean(destRootPath)

	if !strings.HasPrefix(srcPath, h.cfg.DataRoot) || !strings.HasPrefix(destRootPath, h.cfg.DataRoot) {
		JSONError(w, http.StatusBadRequest, "Path outside root")
		return
	}

	// Verify source is a directory
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		JSONError(w, http.StatusNotFound, fmt.Sprintf("Source not found: %v", err))
		return
	}

	if !srcInfo.IsDir() {
		JSONError(w, http.StatusBadRequest, "Source must be a directory")
		return
	}

	created := 0
	var errors []string

	log.Printf("HARDLINK FOLDER START src=%s dest=%s by %s", srcPath, destRootPath, GetUsername(r))

	// Walk source directory
	err = filepath.WalkDir(srcPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Skip @eaDir
		if d.IsDir() && d.Name() == "@eaDir" {
			return fs.SkipDir
		}

		// Skip directories, only process files
		if d.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", d.Name(), err))
			return nil
		}

		// Destination path
		destPath := filepath.Join(destRootPath, relPath)

		// Skip if destination already exists
		if _, err := os.Stat(destPath); err == nil {
			return nil
		}

		// Create parent directory
		destDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			errors = append(errors, fmt.Sprintf("%s: failed to create directory: %v", d.Name(), err))
			return nil
		}

		// Create hardlink
		if err := os.Link(path, destPath); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", d.Name(), err))
			return nil
		}

		// Update inode index
		info, err := d.Info()
		if err == nil {
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				h.db.AddInodePath(stat.Ino, destPath)
			}
		}

		created++
		return nil
	})

	if err != nil {
		JSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to walk directory: %v", err))
		return
	}

	log.Printf("HARDLINK FOLDER END src=%s dest=%s created=%d errors=%d", srcPath, destRootPath, created, len(errors))

	response := map[string]interface{}{
		"ok":      true,
		"created": created,
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	JSONResponse(w, http.StatusOK, response)
}

// DeleteHardlinkRequest represents a hardlink deletion request
type DeleteHardlinkRequest struct {
	Path string `json:"path"`
}

// DeleteHardlink deletes a hardlink (with protection)
func (h *HardlinkHandler) DeleteHardlink(w http.ResponseWriter, r *http.Request) {
	var req DeleteHardlinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Path == "" {
		JSONError(w, http.StatusBadRequest, "Path is required")
		return
	}

	targetPath := filepath.Join(h.cfg.DataRoot, strings.TrimPrefix(req.Path, "/"))
	targetPath = filepath.Clean(targetPath)

	if !strings.HasPrefix(targetPath, h.cfg.DataRoot) {
		JSONError(w, http.StatusBadRequest, "Path outside root")
		return
	}

	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			JSONError(w, http.StatusNotFound, "File or directory not found")
			return
		}
		JSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stat: %v", err))
		return
	}

	// Handle directory deletion
	if info.IsDir() {
		// Check if empty
		entries, err := os.ReadDir(targetPath)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read directory: %v", err))
			return
		}

		if len(entries) > 0 {
			JSONError(w, http.StatusBadRequest, "Directory is not empty")
			return
		}

		if err := os.Remove(targetPath); err != nil {
			JSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete directory: %v", err))
			return
		}

		log.Printf("DELETE DIR %s by %s", targetPath, GetUsername(r))
		JSONResponse(w, http.StatusOK, map[string]interface{}{
			"ok":     true,
			"is_dir": true,
		})
		return
	}

	// Handle file deletion with hardlink protection
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		JSONError(w, http.StatusInternalServerError, "Failed to get file stats")
		return
	}

	nlink := stat.Nlink

	// Protect last link
	if nlink <= 1 {
		JSONError(w, http.StatusForbidden, "Cannot delete the last link to this file")
		return
	}

	// Delete the hardlink
	if err := os.Remove(targetPath); err != nil {
		JSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete: %v", err))
		return
	}

	// Remove from inode index
	h.db.RemoveInodePath(stat.Ino, targetPath)

	log.Printf("DELETE HARDLINK %s remaining_links=%d by %s", targetPath, nlink-1, GetUsername(r))
	JSONResponse(w, http.StatusOK, map[string]interface{}{
		"ok":              true,
		"remaining_links": nlink - 1,
		"is_dir":          false,
	})
}
