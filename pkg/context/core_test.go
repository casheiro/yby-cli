package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- GetCoreContext Tests ----

func setupContextDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	ybyDir := filepath.Join(tmpDir, ".yby")
	if err := os.MkdirAll(ybyDir, 0755); err != nil {
		t.Fatal(err)
	}
	return tmpDir
}

func writeEnvYAML(t *testing.T, tmpDir, content string) {
	t.Helper()
	p := filepath.Join(tmpDir, ".yby", "environments.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGetCoreContext_NoSynapstor(t *testing.T) {
	tmpDir := setupContextDir(t)
	// No synapstor, no README — should return "unknown" env + no overview body
	ctx, err := GetCoreContext(tmpDir)
	if err != nil {
		t.Fatalf("GetCoreContext failed: %v", err)
	}
	if ctx == nil {
		t.Fatal("expected non-nil CoreContext")
	}
	if ctx.Environment != "unknown" {
		t.Errorf("expected 'unknown' environment, got '%s'", ctx.Environment)
	}
	if ctx.Overview == "" {
		t.Error("expected non-empty Overview even as fallback")
	}
}

func TestGetCoreContext_WithSynapstor(t *testing.T) {
	tmpDir := setupContextDir(t)

	// Create synapstor structure
	synapstorDir := filepath.Join(tmpDir, ".synapstor")
	ukiDir := filepath.Join(synapstorDir, ".uki")
	os.MkdirAll(ukiDir, 0755)

	os.WriteFile(filepath.Join(synapstorDir, "00_PROJECT_OVERVIEW.md"), []byte("# My Project Overview"), 0644)
	os.WriteFile(filepath.Join(synapstorDir, "02_BACKLOG_AND_DEBT.md"), []byte("# Backlog"), 0644)
	os.WriteFile(filepath.Join(ukiDir, "UKI_AUTH.md"), []byte("# Auth"), 0644)
	os.WriteFile(filepath.Join(ukiDir, "UKI_PAYMENTS.md"), []byte("# Payments"), 0644)

	ctx, err := GetCoreContext(tmpDir)
	if err != nil {
		t.Fatalf("GetCoreContext failed: %v", err)
	}

	if !strings.Contains(ctx.Overview, "My Project Overview") {
		t.Errorf("expected Overview to contain synapstor content, got: %s", ctx.Overview)
	}
	if !strings.Contains(ctx.Backlog, "Backlog") {
		t.Errorf("expected Backlog to contain content, got: %s", ctx.Backlog)
	}
	if len(ctx.UKIIndex) != 2 {
		t.Errorf("expected 2 UKIs indexed, got %d", len(ctx.UKIIndex))
	}
}

func TestGetCoreContext_FallbackToReadme(t *testing.T) {
	tmpDir := setupContextDir(t)
	// Write a README.md
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# My CLI Tool\nAwesome yby-cli"), 0644)

	ctx, err := GetCoreContext(tmpDir)
	if err != nil {
		t.Fatalf("GetCoreContext failed: %v", err)
	}
	if !strings.Contains(ctx.Overview, "README.md") {
		t.Errorf("expected Overview to mention README.md source, got: %s", ctx.Overview)
	}
}

func TestGetCoreContext_WithEnvironment(t *testing.T) {
	tmpDir := setupContextDir(t)
	writeEnvYAML(t, tmpDir, `current: local
environments:
  local:
    type: local
    description: Desenvolvimento local
    values: config/values-local.yaml
`)

	ctx, err := GetCoreContext(tmpDir)
	if err != nil {
		t.Fatalf("GetCoreContext failed: %v", err)
	}
	if ctx.Environment != "local" {
		t.Errorf("expected environment 'local', got '%s'", ctx.Environment)
	}
}

func TestGetCoreContext_ProjectName(t *testing.T) {
	tmpDir := setupContextDir(t)
	ctx, err := GetCoreContext(tmpDir)
	if err != nil {
		t.Fatalf("GetCoreContext failed: %v", err)
	}
	// Project name should be the base directory name (non-empty)
	if ctx.ProjectName == "" {
		t.Error("expected non-empty ProjectName")
	}
}

// ---- readFileLimited Tests ----

func TestReadFileLimited_ShortFile(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "small.txt")
	os.WriteFile(p, []byte("Hello World"), 0644)

	content, err := readFileLimited(p, 100)
	if err != nil {
		t.Fatalf("readFileLimited failed: %v", err)
	}
	if content != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", content)
	}
}

func TestReadFileLimited_TruncatesLongFile(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "big.txt")
	content := strings.Repeat("A", 200)
	os.WriteFile(p, []byte(content), 0644)

	got, err := readFileLimited(p, 100)
	if err != nil {
		t.Fatalf("readFileLimited failed: %v", err)
	}
	if !strings.Contains(got, "truncated") {
		t.Error("expected truncation marker in output")
	}
	if len(got) <= 100 {
		t.Error("expected output to be slightly longer than limit due to truncation marker")
	}
}

func TestReadFileLimited_Nonexistent(t *testing.T) {
	_, err := readFileLimited("/nonexistent/file.md", 100)
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

// ---- indexUKIs Tests ----

func TestIndexUKIs_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	index := indexUKIs(tmpDir)
	if len(index) != 0 {
		t.Errorf("expected empty index, got %d items", len(index))
	}
}

func TestIndexUKIs_WithMarkdownFiles(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "UKI_AUTH.md"), []byte("# Auth"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "UKI_PAYMENTS.md"), []byte("# Payments"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("ignored"), 0644) // should be skipped

	index := indexUKIs(tmpDir)
	if len(index) != 2 {
		t.Errorf("expected 2 UKIs, got %d", len(index))
	}
	for _, u := range index {
		if !strings.HasSuffix(u.Filename, ".md") {
			t.Errorf("expected .md suffix in Filename, got %s", u.Filename)
		}
		if u.ID == "" {
			t.Error("expected non-empty ID")
		}
	}
}

func TestIndexUKIs_NonexistentDir(t *testing.T) {
	index := indexUKIs("/nonexistent/path/xyz")
	if len(index) != 0 {
		t.Errorf("expected empty index for nonexistent dir, got %d", len(index))
	}
}
