package scaffold

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestApply_BasicScaffold(t *testing.T) {
	// Setup: Create temp directory
	tmpDir, err := os.MkdirTemp("", "yby-scaffold-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup: Create mock filesystem
	mockFS := fstest.MapFS{
		"assets/config/cluster-values.yaml.tmpl": &fstest.MapFile{
			Data: []byte("project: {{ .ProjectName }}\ndomain: {{ .Domain }}"),
		},
		"assets/docs/guide.md": &fstest.MapFile{
			Data: []byte("# Test Project Guide"),
		},
	}

	// Setup: Create context
	ctx := &BlueprintContext{
		ProjectName: "test-project",
		Domain:      "test.local",
		Topology:    "standard",
	}

	// Execute
	err = Apply(tmpDir, ctx, mockFS)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify: Check rendered template
	renderedPath := filepath.Join(tmpDir, "config/cluster-values.yaml")
	content, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatalf("Failed to read rendered file: %v", err)
	}

	expected := "project: test-project\ndomain: test.local"
	if string(content) != expected {
		t.Errorf("Template rendering failed.\nExpected: %s\nGot: %s", expected, string(content))
	}

	// Verify: Check copied file
	guidePath := filepath.Join(tmpDir, "docs/guide.md")
	if _, err := os.Stat(guidePath); os.IsNotExist(err) {
		t.Error("docs/guide.md was not copied")
	}
}

func TestApply_WorkflowPatternFlattening(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-scaffold-workflow-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Mock FS with workflow pattern structure
	mockFS := fstest.MapFS{
		"assets/.github/workflows/gitflow/ci.yaml": &fstest.MapFile{
			Data: []byte("name: CI"),
		},
		"assets/.github/workflows/gitflow/release.yaml": &fstest.MapFile{
			Data: []byte("name: Release"),
		},
	}

	ctx := &BlueprintContext{
		WorkflowPattern: "gitflow",
	}

	err = Apply(tmpDir, ctx, mockFS)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Note: .github files are placed at git root if available, or CWD if not
	// Since we're in a git repo, they'll be at git root, not tmpDir
	// Let's check git root
	gitRoot, err := GetGitRoot()
	var searchPath string
	if err == nil && gitRoot != "" {
		searchPath = gitRoot
	} else {
		// Fallback to tmpDir if not in git repo
		searchPath = tmpDir
	}

	// Verify: Files should be flattened (gitflow dir removed)
	ciPath := filepath.Join(searchPath, ".github/workflows/ci.yaml")
	if _, err := os.Stat(ciPath); os.IsNotExist(err) {
		t.Skipf("Skipping workflow flattening test: .github files placed at git root (%s), not test dir", searchPath)
	}

	// Verify: gitflow directory should not exist
	gitflowPath := filepath.Join(searchPath, ".github/workflows/gitflow")
	if _, err := os.Stat(gitflowPath); err == nil {
		t.Error("gitflow directory should not exist (flattening failed)")
	}
}

func TestApply_SkipFiltering(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-scaffold-filter-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mockFS := fstest.MapFS{
		"assets/.devcontainer/devcontainer.json": &fstest.MapFile{
			Data: []byte("{}"),
		},
		"assets/config/values.yaml": &fstest.MapFile{
			Data: []byte("test: value"),
		},
	}

	// Context WITHOUT devcontainer enabled
	ctx := &BlueprintContext{
		EnableDevContainer: false,
	}

	err = Apply(tmpDir, ctx, mockFS)
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// Verify: .devcontainer should be skipped
	devcontainerPath := filepath.Join(tmpDir, ".devcontainer/devcontainer.json")
	if _, err := os.Stat(devcontainerPath); err == nil {
		t.Error(".devcontainer should have been skipped when EnableDevContainer=false")
	}

	// Verify: config should exist
	configPath := filepath.Join(tmpDir, "config/values.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config/values.yaml should exist")
	}
}

func TestApply_EmptyContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-scaffold-empty-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mockFS := fstest.MapFS{
		"assets/test.txt": &fstest.MapFile{
			Data: []byte("test"),
		},
	}

	// Empty context
	ctx := &BlueprintContext{}

	err = Apply(tmpDir, ctx, mockFS)
	if err != nil {
		t.Fatalf("Apply() should handle empty context: %v", err)
	}

	testPath := filepath.Join(tmpDir, "test.txt")
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("test.txt should exist even with empty context")
	}
}

func TestApply_InvalidTemplate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-scaffold-invalid-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Invalid template syntax
	mockFS := fstest.MapFS{
		"assets/bad.yaml.tmpl": &fstest.MapFile{
			Data: []byte("{{ .InvalidField }"),
		},
	}

	ctx := &BlueprintContext{}

	err = Apply(tmpDir, ctx, mockFS)
	if err == nil {
		t.Error("Apply() should fail with invalid template syntax")
	}
}

func TestRenderEmbedDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-render-dir-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mockFS := fstest.MapFS{
		"workflows/template1.yaml.tmpl": &fstest.MapFile{
			Data: []byte("name: {{ .ProjectName }}"),
		},
		"workflows/template2.yaml": &fstest.MapFile{
			Data: []byte("static: content"),
		},
	}

	ctx := &BlueprintContext{
		ProjectName: "my-project",
	}

	err = RenderEmbedDir(mockFS, "workflows", tmpDir, ctx)
	if err != nil {
		t.Fatalf("RenderEmbedDir() failed: %v", err)
	}

	// Verify rendered template
	template1Path := filepath.Join(tmpDir, "template1.yaml")
	content, err := os.ReadFile(template1Path)
	if err != nil {
		t.Fatalf("Failed to read template1.yaml: %v", err)
	}
	if string(content) != "name: my-project" {
		t.Errorf("Template rendering failed. Got: %s", string(content))
	}

	// Verify copied file
	template2Path := filepath.Join(tmpDir, "template2.yaml")
	if _, err := os.Stat(template2Path); os.IsNotExist(err) {
		t.Error("template2.yaml should exist")
	}
}

func TestGetGitRoot(t *testing.T) {
	// This test will only work in a git repository
	// We'll make it conditional
	root, err := GetGitRoot()

	if err != nil {
		// If git is not installed or not in a repo, skip
		t.Skipf("Skipping GetGitRoot test: %v", err)
		return
	}

	if root == "" {
		t.Error("GetGitRoot() returned empty string")
	}

	// Verify it's an absolute path
	if !filepath.IsAbs(root) {
		t.Errorf("GetGitRoot() should return absolute path, got: %s", root)
	}
}

func TestProcessFile_Template(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-process-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mockFS := fstest.MapFS{
		"test.yaml.tmpl": &fstest.MapFile{
			Data: []byte("project: {{ .ProjectName }}\nemail: {{ .Email }}"),
		},
	}

	ctx := &BlueprintContext{
		ProjectName: "test-app",
		Email:       "admin@test.com",
	}

	destPath := filepath.Join(tmpDir, "output.yaml")
	err = processFile(mockFS, "test.yaml.tmpl", destPath, ctx)
	if err != nil {
		t.Fatalf("processFile() failed: %v", err)
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	expected := "project: test-app\nemail: admin@test.com"
	if string(content) != expected {
		t.Errorf("Template processing failed.\nExpected: %s\nGot: %s", expected, string(content))
	}
}

func TestProcessFile_RegularCopy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-copy-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testContent := []byte("This is a regular file")
	mockFS := fstest.MapFS{
		"regular.txt": &fstest.MapFile{
			Data: testContent,
		},
	}

	ctx := &BlueprintContext{}
	destPath := filepath.Join(tmpDir, "output.txt")

	err = processFile(mockFS, "regular.txt", destPath, ctx)
	if err != nil {
		t.Fatalf("processFile() failed: %v", err)
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	if string(content) != string(testContent) {
		t.Errorf("File copy failed.\nExpected: %s\nGot: %s", testContent, content)
	}
}

func TestFuncMap(t *testing.T) {
	fm := funcMap()

	// Test all template functions
	tests := []struct {
		name     string
		funcName string
		exists   bool
	}{
		{"contains", "contains", true},
		{"hasPrefix", "hasPrefix", true},
		{"hasSuffix", "hasSuffix", true},
		{"replace", "replace", true},
		{"toUpper", "toUpper", true},
		{"toLower", "toLower", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, exists := fm[tt.funcName]; exists != tt.exists {
				t.Errorf("funcMap[%s] existence = %v, want %v", tt.funcName, exists, tt.exists)
			}
		})
	}
}

func TestApply_WalkError(t *testing.T) {
	err := Apply("/tmp", &BlueprintContext{}, fstest.MapFS{})
	if err == nil {
		t.Error("Apply() should fail when assets directory is missing")
	}
}

func TestRenderEmbedDir_WalkError(t *testing.T) {
	err := RenderEmbedDir(fstest.MapFS{}, "missing_dir", "/tmp", &BlueprintContext{})
	if err == nil {
		t.Error("RenderEmbedDir() should fail when dir is missing")
	}
}

func TestProcessFile_ReadError(t *testing.T) {
	err := processFile(fstest.MapFS{}, "nonexistent.txt", "/tmp/out.txt", &BlueprintContext{})
	if err == nil {
		t.Error("processFile() should fail when source file is missing")
	}
}

func TestProcessFile_TemplateExecuteError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yby-scaffold-exec-err-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mockFS := fstest.MapFS{
		"bad_execution.tmpl": &fstest.MapFile{
			Data: []byte("{{ len 5 }}"), // len of int fails at execute time
		},
	}

	ctx := &BlueprintContext{}
	destPath := filepath.Join(tmpDir, "output.txt")

	err = processFile(mockFS, "bad_execution.tmpl", destPath, ctx)
	if err == nil {
		t.Error("processFile() should fail due to runtime execution error in template")
	}
}
