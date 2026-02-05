package scanner

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"

	"github.com/gosiva/hardlink-ui/internal/storage"
)

// DuplicateGroup represents a group of duplicate files
type DuplicateGroup struct {
	Size       int64    `json:"size"`
	SizeHuman  string   `json:"size_human"`
	Master     string   `json:"master"`
	Others     []string `json:"others"`
	InodeCount int      `json:"inode_count"` // number of unique inodes
}

// Scanner handles duplicate file scanning
type Scanner struct {
	db       *storage.DB
	dataRoot string
	mu       sync.Mutex
	jobs     map[string]*ScanProgress
}

// ScanProgress tracks the progress of a scan job
type ScanProgress struct {
	JobID       string
	TotalFiles  int
	Processed   int
	GroupsFound int
	Status      string
	Error       string
	Results     []DuplicateGroup
	mu          sync.Mutex
}

// NewScanner creates a new scanner instance
func NewScanner(db *storage.DB, dataRoot string) *Scanner {
	return &Scanner{
		db:       db,
		dataRoot: dataRoot,
		jobs:     make(map[string]*ScanProgress),
	}
}

// StartScan starts a background scan job
func (s *Scanner) StartScan(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[jobID]; exists {
		return fmt.Errorf("job already exists")
	}

	progress := &ScanProgress{
		JobID:  jobID,
		Status: "running",
	}
	s.jobs[jobID] = progress

	// Create job in database
	if err := s.db.CreateScanJob(jobID); err != nil {
		return fmt.Errorf("failed to create scan job: %w", err)
	}

	// Start scan in background
	go s.performScan(jobID, progress)

	return nil
}

// GetProgress returns the current progress of a scan job
func (s *Scanner) GetProgress(jobID string) *ScanProgress {
	s.mu.Lock()
	defer s.mu.Unlock()

	if progress, exists := s.jobs[jobID]; exists {
		progress.mu.Lock()
		defer progress.mu.Unlock()

		// Return a copy
		return &ScanProgress{
			JobID:       progress.JobID,
			TotalFiles:  progress.TotalFiles,
			Processed:   progress.Processed,
			GroupsFound: progress.GroupsFound,
			Status:      progress.Status,
			Error:       progress.Error,
			Results:     progress.Results,
		}
	}

	// Check database for completed jobs
	job, err := s.db.GetScanJob(jobID)
	if err == nil && job != nil {
		return &ScanProgress{
			JobID:       job.JobID,
			TotalFiles:  job.TotalFiles,
			Processed:   job.Progress,
			GroupsFound: job.GroupsFound,
			Status:      job.Status,
		}
	}

	return nil
}

func (s *Scanner) performScan(jobID string, progress *ScanProgress) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Scanner panic for job %s: %v", jobID, r)
			progress.mu.Lock()
			progress.Status = "failed"
			progress.Error = fmt.Sprintf("panic: %v", r)
			progress.mu.Unlock()
			s.db.FailScanJob(jobID, progress.Error)
		}
	}()

	log.Printf("Starting duplicate scan job: %s", jobID)

	// Phase 1: Collect all files grouped by size
	sizeMap := make(map[int64][]string)
	totalFiles := 0

	err := filepath.WalkDir(s.dataRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Skip @eaDir directories (Synology)
		if d.IsDir() && d.Name() == "@eaDir" {
			return fs.SkipDir
		}

		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil
			}

			size := info.Size()
			sizeMap[size] = append(sizeMap[size], path)
			totalFiles++

			if totalFiles%1000 == 0 {
				progress.mu.Lock()
				progress.TotalFiles = totalFiles
				progress.mu.Unlock()
				s.db.UpdateScanJobProgress(jobID, 0, totalFiles)
			}
		}

		return nil
	})

	if err != nil {
		progress.mu.Lock()
		progress.Status = "failed"
		progress.Error = fmt.Sprintf("failed to walk directory: %v", err)
		progress.mu.Unlock()
		s.db.FailScanJob(jobID, progress.Error)
		return
	}

	progress.mu.Lock()
	progress.TotalFiles = totalFiles
	progress.mu.Unlock()
	s.db.UpdateScanJobProgress(jobID, 0, totalFiles)

	log.Printf("Job %s: Found %d total files", jobID, totalFiles)

	// Phase 2: Find duplicates by hashing files with same size
	var duplicateGroups []DuplicateGroup
	processed := 0

	for size, paths := range sizeMap {
		if len(paths) < 2 {
			processed += len(paths)
			continue
		}

		// Hash all files with this size
		hashMap := make(map[string][]string)
		for _, path := range paths {
			hash, err := ComputeFileHash(path)
			if err != nil {
				log.Printf("Failed to hash %s: %v", path, err)
				processed++
				continue
			}

			hashMap[hash] = append(hashMap[hash], path)
			processed++

			if processed%100 == 0 {
				progress.mu.Lock()
				progress.Processed = processed
				progress.mu.Unlock()
				s.db.UpdateScanJobProgress(jobID, processed, totalFiles)
			}
		}

		// Group by inode for each hash group
		for _, hashPaths := range hashMap {
			if len(hashPaths) < 2 {
				continue
			}

			// Group by inode
			inodeMap := make(map[uint64][]string)
			for _, path := range hashPaths {
				stat, err := os.Stat(path)
				if err != nil {
					continue
				}

				var inode uint64
				if sys := stat.Sys(); sys != nil {
					if statT, ok := sys.(*syscall.Stat_t); ok {
						inode = statT.Ino
					}
				}

				inodeMap[inode] = append(inodeMap[inode], path)
			}

			// Only create a group if there are multiple inodes
			if len(inodeMap) < 2 {
				continue
			}

			// Sort inodes and select first as master
			var inodes []uint64
			for ino := range inodeMap {
				inodes = append(inodes, ino)
			}
			sort.Slice(inodes, func(i, j int) bool { return inodes[i] < inodes[j] })

			masterInode := inodes[0]
			masterPaths := inodeMap[masterInode]
			sort.Strings(masterPaths)
			master := masterPaths[0]

			// Collect all other paths
			var others []string
			for _, ino := range inodes[1:] {
				others = append(others, inodeMap[ino]...)
			}
			sort.Strings(others)

			if len(others) == 0 {
				continue
			}

			// Convert absolute paths to relative
			relMaster, _ := filepath.Rel(s.dataRoot, master)
			if relMaster != "" {
				master = "/" + relMaster
			}

			var relOthers []string
			for _, o := range others {
				relO, _ := filepath.Rel(s.dataRoot, o)
				if relO != "" {
					relOthers = append(relOthers, "/"+relO)
				} else {
					relOthers = append(relOthers, o)
				}
			}

			duplicateGroups = append(duplicateGroups, DuplicateGroup{
				Size:       size,
				SizeHuman:  humanSize(size),
				Master:     master,
				Others:     relOthers,
				InodeCount: len(inodeMap),
			})

			progress.mu.Lock()
			progress.GroupsFound = len(duplicateGroups)
			progress.mu.Unlock()
		}
	}

	// Sort groups by size descending
	sort.Slice(duplicateGroups, func(i, j int) bool {
		return duplicateGroups[i].Size > duplicateGroups[j].Size
	})

	progress.mu.Lock()
	progress.Status = "completed"
	progress.Processed = totalFiles
	progress.GroupsFound = len(duplicateGroups)
	progress.Results = duplicateGroups
	progress.mu.Unlock()

	s.db.CompleteScanJob(jobID, len(duplicateGroups))
	log.Printf("Job %s: Completed. Found %d duplicate groups", jobID, len(duplicateGroups))
}

func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	
	units := []string{"KB", "Mo", "Go", "To"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}
