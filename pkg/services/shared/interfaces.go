package shared

import (
	"context"
	"io/fs"
)

// Runner abstracts command execution (sh, helm, kubectl)
type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
	RunCombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error)
	RunStdin(ctx context.Context, stdin string, name string, args ...string) error
	LookPath(file string) (string, error)
}

// Filesystem abstracts file operations
type Filesystem interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Stat(name string) (fs.FileInfo, error)
	UserHomeDir() (string, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
}
