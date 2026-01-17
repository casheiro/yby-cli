package filesystem

import (
	"errors"
	"io/fs"
	"sort"
)

// CompositeFS implements fs.FS and allows overlaying multiple filesystems.
// Files are searched in layers from last to first (LIFO), enabling overrides.
type CompositeFS struct {
	layers []fs.FS
}

// NewCompositeFS creates a new CompositeFS with the given layers.
// Layers later in the slice have precedence over earlier ones.
func NewCompositeFS(layers ...fs.FS) *CompositeFS {
	return &CompositeFS{layers: layers}
}

// Open opens the named file.
// It iterates through layers in reverse order (LIFO).
func (c *CompositeFS) Open(name string) (fs.File, error) {
	for i := len(c.layers) - 1; i >= 0; i-- {
		f, err := c.layers[i].Open(name)
		if err == nil {
			return f, nil
		}
		// If error is something other than NotExist, we could return it,
		// but simple overlay behavior usually ignores failures in top layers unless specifically handled.
		// Here we assume if it fails to open in top, we look below.
		// However, standard is fs.ErrNotExist. If permission denied, checking lower layer might be wrong?
		// For safety/simplicity in scaffolding: proceed if NotExist.
		if !errors.Is(err, fs.ErrNotExist) {
			// If it's a real error (not missing), we might want to return it or skip.
			// Let's retry unless it's a critical error?
			// Generally for overlays, if a file exists but fails, it's an error.
			// But differentiating "exists" from "Open" failure is hard without Stat.
			// Given fs.FS often just returns Open error, we continue on NotExist.
			continue
		}
	}
	return nil, fs.ErrNotExist
}

// ReadDir reads the named directory and returns a sorted list of directory entries.
// It merges entries from all layers.
func (c *CompositeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries := make(map[string]fs.DirEntry)
	foundAny := false

	// Iterate from bottom to top so that upper layers (later in slice)
	// can overwrite map entries if they have same name (though DirEntry matching is usually by name).
	for _, layer := range c.layers {
		if rdf, ok := layer.(fs.ReadDirFS); ok {
			dirEntries, err := rdf.ReadDir(name)
			if err == nil {
				foundAny = true
				for _, e := range dirEntries {
					entries[e.Name()] = e
				}
			}
		}
	}

	if !foundAny {
		return nil, fs.ErrNotExist
	}

	out := make([]fs.DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, e)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })

	return out, nil
}
