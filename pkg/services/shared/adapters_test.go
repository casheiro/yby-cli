package shared

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// ---- RealRunner Tests ----

func TestRealRunner_LookPath_Found(t *testing.T) {
	r := &RealRunner{}
	path, err := r.LookPath("sh")
	if err != nil {
		t.Errorf("LookPath(sh) unexpected error: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path for sh")
	}
}

func TestRealRunner_LookPath_NotFound(t *testing.T) {
	r := &RealRunner{}
	_, err := r.LookPath("nonexistent_binary_xyz_abc")
	if err == nil {
		t.Error("expected error for nonexistent binary, got nil")
	}
}

func TestRealRunner_Run_Echo(t *testing.T) {
	r := &RealRunner{}
	err := r.Run(context.Background(), "echo", "hello")
	if err != nil {
		t.Errorf("Run(echo hello) unexpected error: %v", err)
	}
}

func TestRealRunner_Run_InvalidCommand(t *testing.T) {
	r := &RealRunner{}
	err := r.Run(context.Background(), "nonexistent_cmd_xyz")
	if err == nil {
		t.Error("expected error for invalid command, got nil")
	}
}

func TestRealRunner_RunCombinedOutput(t *testing.T) {
	r := &RealRunner{}
	out, err := r.RunCombinedOutput(context.Background(), "echo", "hello")
	if err != nil {
		t.Errorf("RunCombinedOutput unexpected error: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output from echo")
	}
}

func TestRealRunner_RunStdin(t *testing.T) {
	r := &RealRunner{}
	err := r.RunStdin(context.Background(), "hello", "cat")
	if err != nil {
		t.Errorf("RunStdin unexpected error: %v", err)
	}
}

func TestRealRunner_RunStdinOutput(t *testing.T) {
	r := &RealRunner{}
	out, err := r.RunStdinOutput(context.Background(), "hello world", "cat")
	if err != nil {
		t.Errorf("RunStdinOutput unexpected error: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output from cat+stdin")
	}
}

// ---- RealFilesystem Tests ----

func TestRealFilesystem_WriteAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	data := []byte("conteúdo de teste")

	fs := &RealFilesystem{}

	if err := fs.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	got, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("expected %q, got %q", data, got)
	}
}

func TestRealFilesystem_ReadFile_NotFound(t *testing.T) {
	f := &RealFilesystem{}
	_, err := f.ReadFile("/nonexistent/path/xyz.txt")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestRealFilesystem_MkdirAll(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "a", "b", "c")
	f := &RealFilesystem{}
	if err := f.MkdirAll(newDir, 0o755); err != nil {
		t.Errorf("MkdirAll failed: %v", err)
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Errorf("directory was not created: %v", err)
	}
}

func TestRealFilesystem_Stat_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	f := &RealFilesystem{}
	info, err := f.Stat(tmpDir)
	if err != nil {
		t.Errorf("Stat on existing dir failed: %v", err)
	}
	if info == nil {
		t.Error("expected non-nil FileInfo")
	}
}

func TestRealFilesystem_Stat_NotExists(t *testing.T) {
	f := &RealFilesystem{}
	_, err := f.Stat("/nonexistent/path/xyz")
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

func TestRealFilesystem_UserHomeDir(t *testing.T) {
	f := &RealFilesystem{}
	home, err := f.UserHomeDir()
	if err != nil {
		t.Errorf("UserHomeDir failed: %v", err)
	}
	if home == "" {
		t.Error("expected non-empty home dir")
	}
}

func TestRealFilesystem_WalkDir(t *testing.T) {
	f := &RealFilesystem{}
	// Walking a valid path — we just verify it doesn't panic/error in an unexpected way
	visited := 0
	_ = f.WalkDir("tmp", func(path string, d fs.DirEntry, err error) error {
		visited++
		return nil
	})
	// visited count doesn't matter; just verifying no panic
}
