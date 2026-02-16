package scanner

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ScanResult holds the path and content (optional) of a file
type ScanResult struct {
	Path    string
	Content string
}

// Scan walks the directory and returns files matching the criteria.
// If query is provided, it does a simple contains check on filename or content.
func Scan(root string, query string) ([]ScanResult, error) {
	var results []ScanResult
	query = strings.ToLower(query)

	ignores := map[string]bool{
		".git":              true,
		"node_modules":      true,
		"vendor":            true,
		"dist":              true,
		".synapstor":        true, // Don't index the index
		"go.sum":            true,
		"package-lock.json": true,
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if ignores[d.Name()] {
				return filepath.SkipDir
			}
			// Skip hidden dirs
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Simple ignore for hidden files and binaries
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		// Check query relevance
		relPath, _ := filepath.Rel(root, path)
		match := false

		if query == "" {
			match = true
		} else {
			if strings.Contains(strings.ToLower(relPath), query) {
				match = true
			}
		}

		// If matched by filename, or we need to check content
		if !match && query != "" {
			// Read content to check
			// PERF: This is slow for large repos, but fine for MVP
			content, err := os.ReadFile(path)
			if err == nil {
				if strings.Contains(strings.ToLower(string(content)), query) {
					match = true
				}
			}
		}

		if match {
			// Read content if not already read
			content, err := os.ReadFile(path)
			if err == nil {
				// text file heuristic
				if isText(content) {
					results = append(results, ScanResult{
						Path:    relPath,
						Content: string(content),
					})
				}
			}
		}

		return nil
	})

	return results, err
}

func isText(data []byte) bool {
	// Simple check: see if there are null bytes
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}
