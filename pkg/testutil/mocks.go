package testutil

import (
	"context"
	"io/fs"
	"os"
)

// MockRunner implements the shared.Runner interface for testing
type MockRunner struct {
	RunFunc               func(ctx context.Context, name string, args ...string) error
	RunCombinedOutputFunc func(ctx context.Context, name string, args ...string) ([]byte, error)
	RunStdinFunc          func(ctx context.Context, stdin string, name string, args ...string) error
	RunStdinOutputFunc    func(ctx context.Context, stdin string, name string, args ...string) ([]byte, error)
	LookPathFunc          func(file string) (string, error)
}

func (m *MockRunner) Run(ctx context.Context, name string, args ...string) error {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, name, args...)
	}
	return nil
}

func (m *MockRunner) RunCombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	if m.RunCombinedOutputFunc != nil {
		return m.RunCombinedOutputFunc(ctx, name, args...)
	}
	return []byte{}, nil
}

func (m *MockRunner) RunStdin(ctx context.Context, stdin string, name string, args ...string) error {
	if m.RunStdinFunc != nil {
		return m.RunStdinFunc(ctx, stdin, name, args...)
	}
	return nil
}

func (m *MockRunner) RunStdinOutput(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
	if m.RunStdinOutputFunc != nil {
		return m.RunStdinOutputFunc(ctx, stdin, name, args...)
	}
	return []byte{}, nil
}

func (m *MockRunner) LookPath(file string) (string, error) {
	if m.LookPathFunc != nil {
		return m.LookPathFunc(file)
	}
	return file, nil
}

// MockFilesystem implements the shared.Filesystem interface for testing
type MockFilesystem struct {
	ReadFileFunc    func(name string) ([]byte, error)
	WriteFileFunc   func(name string, data []byte, perm fs.FileMode) error
	MkdirAllFunc    func(path string, perm fs.FileMode) error
	StatFunc        func(name string) (fs.FileInfo, error)
	UserHomeDirFunc func() (string, error)
	WalkDirFunc     func(root string, fn fs.WalkDirFunc) error
}

func (m *MockFilesystem) ReadFile(name string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(name)
	}
	return nil, os.ErrNotExist
}

func (m *MockFilesystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(name, data, perm)
	}
	return nil
}

func (m *MockFilesystem) MkdirAll(path string, perm fs.FileMode) error {
	if m.MkdirAllFunc != nil {
		return m.MkdirAllFunc(path, perm)
	}
	return nil
}

func (m *MockFilesystem) Stat(name string) (fs.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(name)
	}
	return nil, os.ErrNotExist
}

func (m *MockFilesystem) UserHomeDir() (string, error) {
	if m.UserHomeDirFunc != nil {
		return m.UserHomeDirFunc()
	}
	return "/mock/home", nil
}

func (m *MockFilesystem) WalkDir(root string, fn fs.WalkDirFunc) error {
	if m.WalkDirFunc != nil {
		return m.WalkDirFunc(root, fn)
	}
	return nil
}
