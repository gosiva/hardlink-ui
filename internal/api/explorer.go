package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/gosiva/hardlink-ui/internal/config"
	"github.com/gosiva/hardlink-ui/internal/storage"
)

// ExplorerHandler handles file explorer requests
type ExplorerHandler struct {
	db        *storage.DB
	cfg       *config.Config
	templates *template.Template
}

// NewExplorerHandler creates a new explorer handler
func NewExplorerHandler(db *storage.DB, cfg *config.Config, templatesPath string) (*ExplorerHandler, error) {
	// Parse only the templates needed for the explorer
	// This prevents conflicts with login.html and 2fa.html which also define "content" block
	tmpl, err := template.ParseFiles(
		filepath.Join(templatesPath, "base.html"),
		filepath.Join(templatesPath, "explorer.html"),
	)
	if err != nil {
		return nil, err
	}

	return &ExplorerHandler{
		db:        db,
		cfg:       cfg,
		templates: tmpl,
	}, nil
}

// ShowExplorer shows the main explorer page
func (h *ExplorerHandler) ShowExplorer(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "explorer.html", nil)
}

// FileEntry represents a file or directory entry
type FileEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"is_dir"`
	Inode     uint64 `json:"inode"`
	Nlink     uint64 `json:"nlink"`
	Size      int64  `json:"size"`
	SizeHuman string `json:"size_human"`
}

// ListDirectory lists files in a directory
func (h *ExplorerHandler) ListDirectory(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		relPath = "/"
	}

	// Resolve path safely
	targetPath := filepath.Join(h.cfg.DataRoot, strings.TrimPrefix(relPath, "/"))
	targetPath = filepath.Clean(targetPath)

	// Ensure path is under DataRoot
	if !strings.HasPrefix(targetPath, h.cfg.DataRoot) {
		JSONError(w, http.StatusBadRequest, "Path outside root")
		return
	}

	// Read directory
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read directory: %v", err))
		return
	}

	// Initialize as empty slice to avoid null in JSON for empty directories
	result := make([]FileEntry, 0)
	for _, entry := range entries {
		// Skip @eaDir
		if entry.Name() == "@eaDir" {
			continue
		}

		fullPath := filepath.Join(targetPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			log.Printf("Failed to stat %s: %v", fullPath, err)
			continue
		}

		// Get inode and nlink
		var inode, nlink uint64
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			inode = stat.Ino
			nlink = stat.Nlink
		}

		// Build relative path
		entryRelPath := filepath.Join(relPath, entry.Name())
		if !strings.HasPrefix(entryRelPath, "/") {
			entryRelPath = "/" + entryRelPath
		}

		sizeHuman := ""
		size := info.Size()
		if !entry.IsDir() {
			sizeHuman = humanSize(size)
		}

		result = append(result, FileEntry{
			Name:      entry.Name(),
			Path:      entryRelPath,
			IsDir:     entry.IsDir(),
			Inode:     inode,
			Nlink:     nlink,
			Size:      size,
			SizeHuman: sizeHuman,
		})
	}

	// Sort: directories first, then by name
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	JSONResponse(w, http.StatusOK, map[string]interface{}{
		"entries": result,
	})
}

// FileDetails represents detailed file information
type FileDetails struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Inode     uint64   `json:"inode"`
	Nlink     uint64   `json:"nlink"`
	Size      int64    `json:"size"`
	SizeHuman string   `json:"size_human"`
	AllPaths  []string `json:"all_paths"`
}

// GetDetails returns detailed information about a file
func (h *ExplorerHandler) GetDetails(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		JSONError(w, http.StatusBadRequest, "Missing path parameter")
		return
	}

	targetPath := filepath.Join(h.cfg.DataRoot, strings.TrimPrefix(relPath, "/"))
	targetPath = filepath.Clean(targetPath)

	if !strings.HasPrefix(targetPath, h.cfg.DataRoot) {
		JSONError(w, http.StatusBadRequest, "Path outside root")
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		JSONError(w, http.StatusNotFound, fmt.Sprintf("File not found: %v", err))
		return
	}

	var inode, nlink uint64
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		inode = stat.Ino
		nlink = stat.Nlink
	}

	// Find all paths with the same inode
	allPaths, err := h.findAllPathsByInode(inode)
	if err != nil {
		log.Printf("Error finding paths by inode: %v", err)
		allPaths = []string{relPath}
	}

	if len(allPaths) == 0 {
		allPaths = []string{relPath}
	}

	details := FileDetails{
		Name:      filepath.Base(targetPath),
		Path:      relPath,
		Inode:     inode,
		Nlink:     nlink,
		Size:      info.Size(),
		SizeHuman: humanSize(info.Size()),
		AllPaths:  allPaths,
	}

	JSONResponse(w, http.StatusOK, details)
}

// findAllPathsByInode finds all paths with the same inode
func (h *ExplorerHandler) findAllPathsByInode(inode uint64) ([]string, error) {
	var paths []string

	err := filepath.Walk(h.cfg.DataRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Skip @eaDir directories
		if info.IsDir() && info.Name() == "@eaDir" {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				if stat.Ino == inode {
					relPath, _ := filepath.Rel(h.cfg.DataRoot, path)
					if relPath != "" {
						paths = append(paths, "/"+relPath)
					}
				}
			}
		}

		return nil
	})

	return paths, err
}

// CreateFolderRequest represents a folder creation request
type CreateFolderRequest struct {
	Parent string `json:"parent"`
	Name   string `json:"name"`
}

// CreateFolder creates a new folder
func (h *ExplorerHandler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	var req CreateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Name == "" {
		JSONError(w, http.StatusBadRequest, "Folder name is required")
		return
	}

	// Validate folder name
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Name == "." || req.Name == ".." {
		JSONError(w, http.StatusBadRequest, "Invalid folder name")
		return
	}

	// Check for forbidden characters
	if strings.ContainsAny(req.Name, `/\:*?"<>|`) {
		JSONError(w, http.StatusBadRequest, "Folder name contains forbidden characters")
		return
	}

	parentPath := filepath.Join(h.cfg.DataRoot, strings.TrimPrefix(req.Parent, "/"))
	parentPath = filepath.Clean(parentPath)

	if !strings.HasPrefix(parentPath, h.cfg.DataRoot) {
		JSONError(w, http.StatusBadRequest, "Path outside root")
		return
	}

	targetPath := filepath.Join(parentPath, req.Name)

	if err := os.Mkdir(targetPath, 0755); err != nil {
		if os.IsExist(err) {
			JSONError(w, http.StatusConflict, "Folder already exists")
			return
		}
		JSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create folder: %v", err))
		return
	}

	log.Printf("FOLDER CREATE %s by %s", targetPath, GetUsername(r))
	JSONResponse(w, http.StatusOK, map[string]bool{"ok": true})
}
