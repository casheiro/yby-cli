package executor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLocalExecutor(t *testing.T) {
	e := NewLocalExecutor()
	if e == nil {
		t.Fatal("NewLocalExecutor returned nil")
	}
}

func TestLocalExecutor_Close(t *testing.T) {
	e := NewLocalExecutor()
	err := e.Close()
	if err != nil {
		t.Errorf("Close should return nil, got: %v", err)
	}
}

func TestLocalExecutor_Run_Success(t *testing.T) {
	e := NewLocalExecutor()
	// Simple echo command should always succeed
	err := e.Run("test echo", "echo hello")
	if err != nil {
		t.Errorf("Run(echo) unexpected error: %v", err)
	}
}

func TestLocalExecutor_Run_Failure(t *testing.T) {
	e := NewLocalExecutor()
	// Nonexistent command should fail
	err := e.Run("test fail", "exit 1")
	if err == nil {
		t.Error("expected error for 'exit 1', got nil")
	}
}

func TestLocalExecutor_FetchFile_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(path, []byte("hello executor"), 0644)

	e := NewLocalExecutor()
	data, err := e.FetchFile(path)
	if err != nil {
		t.Fatalf("FetchFile failed: %v", err)
	}
	if string(data) != "hello executor" {
		t.Errorf("expected 'hello executor', got %q", data)
	}
}

func TestLocalExecutor_FetchFile_NotFound(t *testing.T) {
	e := NewLocalExecutor()
	_, err := e.FetchFile("/nonexistent/path/xyz.txt")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestLocalExecutor_Run_WithEnvVar(t *testing.T) {
	e := NewLocalExecutor()
	// Script that uses environment variable
	t.Setenv("TEST_VAR", "hello_from_test")
	err := e.Run("env test", "test \"$TEST_VAR\" = \"hello_from_test\"")
	if err != nil {
		t.Errorf("Run with env var unexpected error: %v", err)
	}
}
