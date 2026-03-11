package shared

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"strings"
)

// RealRunner implementation
type RealRunner struct{}

func (r *RealRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *RealRunner) RunCombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func (r *RealRunner) RunStdin(ctx context.Context, stdin string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *RealRunner) RunStdinOutput(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	return cmd.Output()
}

func (r *RealRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// RealFilesystem implementation
type RealFilesystem struct{}

func (f *RealFilesystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (f *RealFilesystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (f *RealFilesystem) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (f *RealFilesystem) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

func (f *RealFilesystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

func (f *RealFilesystem) WalkDir(root string, fn fs.WalkDirFunc) error {
	return fs.WalkDir(os.DirFS("/"), root, fn) // Simplified
}
