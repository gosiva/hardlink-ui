package storage

import (
	"database/sql"
	"time"
)

// AddInodePath adds or updates an inode-path mapping
func (db *DB) AddInodePath(inode uint64, path string) error {
	now := time.Now().Unix()
	_, err := db.Exec(`
		INSERT INTO inode_index (inode, path, last_seen)
		VALUES (?, ?, ?)
		ON CONFLICT(inode, path) DO UPDATE SET last_seen = ?
	`, inode, path, now, now)

	return err
}

// GetInodePaths retrieves all paths for a given inode
func (db *DB) GetInodePaths(inode uint64) ([]string, error) {
	rows, err := db.Query(`
		SELECT path FROM inode_index WHERE inode = ? ORDER BY path
	`, inode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}

	return paths, rows.Err()
}

// RemoveInodePath removes a specific inode-path mapping
func (db *DB) RemoveInodePath(inode uint64, path string) error {
	_, err := db.Exec(`DELETE FROM inode_index WHERE inode = ? AND path = ?`, inode, path)
	return err
}

// CleanupOldInodeEntries removes entries not seen for the given number of days
func (db *DB) CleanupOldInodeEntries(daysOld int) error {
	cutoff := time.Now().Unix() - int64(daysOld*86400)
	_, err := db.Exec(`DELETE FROM inode_index WHERE last_seen < ?`, cutoff)
	return err
}

// FindPathsByInode finds all paths with the same inode (used for details view)
func (db *DB) FindPathsByInode(inode uint64) ([]string, error) {
	return db.GetInodePaths(inode)
}

// ClearInodeIndex clears the entire inode index (use before rebuild)
func (db *DB) ClearInodeIndex() error {
	_, err := db.Exec(`DELETE FROM inode_index`)
	return err
}

// GetInodeCount returns the total number of unique inodes in the index
func (db *DB) GetInodeCount() (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(DISTINCT inode) FROM inode_index`).Scan(&count)
	return count, err
}

// GetPathCount returns the total number of paths in the index
func (db *DB) GetPathCount() (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM inode_index`).Scan(&count)
	return count, err
}

// BatchAddInodePaths adds multiple inode-path mappings in a transaction
func (db *DB) BatchAddInodePaths(entries []struct {
	Inode uint64
	Path  string
}) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO inode_index (inode, path, last_seen)
		VALUES (?, ?, ?)
		ON CONFLICT(inode, path) DO UPDATE SET last_seen = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, entry := range entries {
		if _, err := stmt.Exec(entry.Inode, entry.Path, now, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ScanJob represents a duplicate scan job
type ScanJob struct {
	JobID       string
	Status      string
	Progress    int
	TotalFiles  int
	GroupsFound int
	StartedAt   int64
	CompletedAt sql.NullInt64
	Error       sql.NullString
}

// CreateScanJob creates a new scan job
func (db *DB) CreateScanJob(jobID string) error {
	now := time.Now().Unix()
	_, err := db.Exec(`
		INSERT INTO scan_jobs (job_id, status, progress, total_files, groups_found, started_at)
		VALUES (?, 'running', 0, 0, 0, ?)
	`, jobID, now)
	return err
}

// UpdateScanJobProgress updates the progress of a scan job
func (db *DB) UpdateScanJobProgress(jobID string, progress, totalFiles int) error {
	_, err := db.Exec(`
		UPDATE scan_jobs SET progress = ?, total_files = ? WHERE job_id = ?
	`, progress, totalFiles, jobID)
	return err
}

// CompleteScanJob marks a scan job as completed
func (db *DB) CompleteScanJob(jobID string, groupsFound int) error {
	now := time.Now().Unix()
	_, err := db.Exec(`
		UPDATE scan_jobs
		SET status = 'completed', groups_found = ?, completed_at = ?
		WHERE job_id = ?
	`, groupsFound, now, jobID)
	return err
}

// FailScanJob marks a scan job as failed
func (db *DB) FailScanJob(jobID string, errorMsg string) error {
	now := time.Now().Unix()
	_, err := db.Exec(`
		UPDATE scan_jobs
		SET status = 'failed', error = ?, completed_at = ?
		WHERE job_id = ?
	`, errorMsg, now, jobID)
	return err
}

// GetScanJob retrieves a scan job by ID
func (db *DB) GetScanJob(jobID string) (*ScanJob, error) {
	job := &ScanJob{}
	err := db.QueryRow(`
		SELECT job_id, status, progress, total_files, groups_found, started_at, completed_at, error
		FROM scan_jobs WHERE job_id = ?
	`, jobID).Scan(&job.JobID, &job.Status, &job.Progress, &job.TotalFiles,
		&job.GroupsFound, &job.StartedAt, &job.CompletedAt, &job.Error)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return job, nil
}
