package scanner

import (
	"fmt"
	"io"
	"os"

	"github.com/cespare/xxhash/v2"
)

const (
	// SmallFileThreshold - files under this size get full hash
	SmallFileThreshold = 100 * 1024 * 1024 // 100 MB
	// PartialHashSize - for large files, hash this much from start and end
	PartialHashSize = 256 * 1024 // 256 KB
	// ChunkSize for reading files
	ChunkSize = 64 * 1024 // 64 KB
)

// ComputeFileHash computes xxHash for a file
// For files < 100MB: full file hash
// For files >= 100MB: hash first 256KB + last 256KB + file size
func ComputeFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	size := stat.Size()
	h := xxhash.New()

	if size == 0 {
		// Empty file
		return fmt.Sprintf("%016x", h.Sum64()), nil
	}

	if size <= SmallFileThreshold {
		// Small file: hash everything
		if _, err := io.Copy(h, file); err != nil {
			return "", fmt.Errorf("failed to hash file: %w", err)
		}
	} else {
		// Large file: hash start + end
		// Read first PartialHashSize bytes
		buf := make([]byte, PartialHashSize)
		n, err := io.ReadFull(file, buf)
		if err != nil && err != io.ErrUnexpectedEOF {
			return "", fmt.Errorf("failed to read file start: %w", err)
		}
		h.Write(buf[:n])

		// Read last PartialHashSize bytes if file is large enough
		if size > 2*PartialHashSize {
			if _, err := file.Seek(-PartialHashSize, io.SeekEnd); err != nil {
				return "", fmt.Errorf("failed to seek to file end: %w", err)
			}
			n, err := io.ReadFull(file, buf)
			if err != nil && err != io.ErrUnexpectedEOF {
				return "", fmt.Errorf("failed to read file end: %w", err)
			}
			h.Write(buf[:n])
		}

		// Include size in hash to differentiate files with same start/end
		sizeBytes := []byte(fmt.Sprintf("SIZE:%d", size))
		h.Write(sizeBytes)
	}

	return fmt.Sprintf("%016x", h.Sum64()), nil
}

// VerifyFilesIdentical does a full byte-by-byte comparison
// Used before converting duplicates to hardlinks
func VerifyFilesIdentical(path1, path2 string) (bool, error) {
	// First check: file stats
	stat1, err := os.Stat(path1)
	if err != nil {
		return false, err
	}

	stat2, err := os.Stat(path2)
	if err != nil {
		return false, err
	}

	// Size must match
	if stat1.Size() != stat2.Size() {
		return false, nil
	}

	// If they're already the same inode, they're identical
	if os.SameFile(stat1, stat2) {
		return true, nil
	}

	// For files under 100MB, do full byte comparison
	// For larger files, trust the xxHash
	if stat1.Size() <= SmallFileThreshold {
		return compareFilesFull(path1, path2)
	}

	// For large files, just verify the hashes match
	hash1, err := ComputeFileHash(path1)
	if err != nil {
		return false, err
	}

	hash2, err := ComputeFileHash(path2)
	if err != nil {
		return false, err
	}

	return hash1 == hash2, nil
}

// compareFilesFull does a complete byte-by-byte comparison
func compareFilesFull(path1, path2 string) (bool, error) {
	f1, err := os.Open(path1)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(path2)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	buf1 := make([]byte, ChunkSize)
	buf2 := make([]byte, ChunkSize)

	for {
		n1, err1 := f1.Read(buf1)
		n2, err2 := f2.Read(buf2)

		if n1 != n2 {
			return false, nil
		}

		if !bytesEqual(buf1[:n1], buf2[:n2]) {
			return false, nil
		}

		if err1 == io.EOF && err2 == io.EOF {
			return true, nil
		}

		if err1 != nil && err1 != io.EOF {
			return false, err1
		}

		if err2 != nil && err2 != io.EOF {
			return false, err2
		}
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
