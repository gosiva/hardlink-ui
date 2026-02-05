package api

import "fmt"

// humanSize formats a byte size in human-readable format
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

	// Use French units to match the UI language
	units := []string{"Ko", "Mo", "Go", "To"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}
