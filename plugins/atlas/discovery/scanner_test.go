package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// ---- Blueprint and Component types ----

func TestBlueprint_Empty(t *testing.T) {
	bp := &Blueprint{
		Components: []Component{},
		Roots:      []string{"/project"},
	}
	if len(bp.Components) != 0 {
		t.Errorf("expected no components, got %d", len(bp.Components))
	}
	if len(bp.Roots) != 1 || bp.Roots[0] != "/project" {
		t.Errorf("expected roots [/project], got %v", bp.Roots)
	}
}

func TestComponent_Fields(t *testing.T) {
	c := Component{
		Name: "backend",
		Type: "app",
		Path: "/project/backend",
		Tags: []string{"golang", "api"},
	}
	if c.Name != "backend" || c.Type != "app" {
		t.Errorf("Component fields mismatch: %+v", c)
	}
	if len(c.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(c.Tags))
	}
}

// ---- Scan Tests ----

func TestScan_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if bp == nil {
		t.Fatal("expected non-nil Blueprint")
	}
	if len(bp.Components) != 0 {
		t.Errorf("expected no components in empty dir, got %d", len(bp.Components))
	}
}

func TestScan_WithGoModule(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "myservice")
	os.MkdirAll(subDir, 0755)
	// Create go.mod to trigger "app" detection
	os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module myservice\n\ngo 1.21\n"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(bp.Components) == 0 {
		t.Error("expected at least 1 component (Go module), got 0")
	}
}

func TestScan_WithPackageJson(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "frontend")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "package.json"), []byte(`{"name":"frontend"}`), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(bp.Components) == 0 {
		t.Error("expected at least 1 component (Node.js app), got 0")
	}
}

func TestScan_WithDockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "myservice")
	os.MkdirAll(subDir, 0755)
	// Dockerfile triggers "infra" detection
	os.WriteFile(filepath.Join(subDir, "Dockerfile"), []byte("FROM alpine\n"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(bp.Components) == 0 {
		t.Error("expected at least 1 component (Dockerfile/infra), got 0")
	}
}

func TestScan_IgnoredDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a go.mod inside node_modules (should be ignored)
	ignoredDir := filepath.Join(tmpDir, "node_modules", "some-pkg")
	os.MkdirAll(ignoredDir, 0755)
	os.WriteFile(filepath.Join(ignoredDir, "go.mod"), []byte("module ignored"), 0644)

	bp, err := Scan(tmpDir, []string{"node_modules"})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	// Should have NO components because node_modules is ignored
	if len(bp.Components) != 0 {
		t.Errorf("expected 0 components (ignored dir), got %d", len(bp.Components))
	}
}

func TestScan_NoDuplicates(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "myapi")
	os.MkdirAll(subDir, 0755)
	// Two files that match the same rule — should not create duplicates
	os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module myapi"), 0644)

	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	// Should have exactly 1 component
	if len(bp.Components) != 1 {
		t.Errorf("expected 1 component (no duplicates), got %d", len(bp.Components))
	}
}

func TestScan_RootsField(t *testing.T) {
	tmpDir := t.TempDir()
	bp, err := Scan(tmpDir, nil)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(bp.Roots) != 1 || bp.Roots[0] != tmpDir {
		t.Errorf("expected Roots=[%s], got %v", tmpDir, bp.Roots)
	}
}

func TestScan_InvalidPath(t *testing.T) {
	_, err := Scan("/nonexistent/path/xyz123", nil)
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}
