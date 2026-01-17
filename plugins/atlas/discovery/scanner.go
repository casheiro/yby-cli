package discovery

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// Scan walks the directory and applies rules to identify components.
func Scan(root string, ignores []string) (*Blueprint, error) {
	bp := &Blueprint{
		Components: []Component{},
		Roots:      []string{root},
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check ignores
		for _, ignore := range ignores {
			if strings.Contains(path, ignore) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Apply Rules (defined in rules.go)
		if d.IsDir() {
			return nil
		}

		// Check for markers
		// Current simple logic: check file name matches rule
		compType := Match(d.Name())
		if compType != "" {
			// Found a component marker
			// Deduce component name from parent dir
			dir := filepath.Dir(path)
			name := filepath.Base(dir)

			// Avoid duplicates (naive check)
			exists := false
			for _, c := range bp.Components {
				if c.Path == dir && c.Type == compType {
					exists = true
					break
				}
			}

			if !exists {
				bp.Components = append(bp.Components, Component{
					Name: name,
					Type: compType,
					Path: dir,
				})
			}
		}

		return nil
	})

	return bp, err
}
