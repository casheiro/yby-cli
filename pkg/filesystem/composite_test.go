package filesystem

import (
	"io"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestNewCompositeFS(t *testing.T) {
	layer1 := fstest.MapFS{}
	layer2 := fstest.MapFS{}

	composite := NewCompositeFS(layer1, layer2)

	if composite == nil {
		t.Fatal("NewCompositeFS returned nil")
	}

	if len(composite.layers) != 2 {
		t.Errorf("Expected 2 layers, got %d", len(composite.layers))
	}
}

func TestCompositeFS_Open_SingleLayer(t *testing.T) {
	mockFS := fstest.MapFS{
		"test.txt": &fstest.MapFile{
			Data: []byte("hello world"),
		},
	}

	composite := NewCompositeFS(mockFS)

	file, err := composite.Open("test.txt")
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("ReadAll() failed: %v", err)
	}

	if string(content) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", string(content))
	}
}

func TestCompositeFS_Open_FileNotFound(t *testing.T) {
	mockFS := fstest.MapFS{
		"exists.txt": &fstest.MapFile{
			Data: []byte("data"),
		},
	}

	composite := NewCompositeFS(mockFS)

	_, err := composite.Open("missing.txt")
	if err == nil {
		t.Error("Expected error for missing file")
	}

	if err != fs.ErrNotExist {
		t.Errorf("Expected fs.ErrNotExist, got %v", err)
	}
}

func TestCompositeFS_Open_MultiLayerOverride(t *testing.T) {
	// Bottom layer (base)
	baseFS := fstest.MapFS{
		"config.yaml": &fstest.MapFile{
			Data: []byte("base: value"),
		},
		"base-only.txt": &fstest.MapFile{
			Data: []byte("only in base"),
		},
	}

	// Top layer (override)
	overrideFS := fstest.MapFS{
		"config.yaml": &fstest.MapFile{
			Data: []byte("override: value"),
		},
		"override-only.txt": &fstest.MapFile{
			Data: []byte("only in override"),
		},
	}

	// Layers: [base, override] - override has precedence
	composite := NewCompositeFS(baseFS, overrideFS)

	// Test 1: File exists in both - should get override version
	file, err := composite.Open("config.yaml")
	if err != nil {
		t.Fatalf("Open(config.yaml) failed: %v", err)
	}
	content, _ := io.ReadAll(file)
	file.Close()

	if string(content) != "override: value" {
		t.Errorf("Expected override version, got '%s'", string(content))
	}

	// Test 2: File only in base
	file, err = composite.Open("base-only.txt")
	if err != nil {
		t.Fatalf("Open(base-only.txt) failed: %v", err)
	}
	content, _ = io.ReadAll(file)
	file.Close()

	if string(content) != "only in base" {
		t.Errorf("Expected base file, got '%s'", string(content))
	}

	// Test 3: File only in override
	file, err = composite.Open("override-only.txt")
	if err != nil {
		t.Fatalf("Open(override-only.txt) failed: %v", err)
	}
	content, _ = io.ReadAll(file)
	file.Close()

	if string(content) != "only in override" {
		t.Errorf("Expected override file, got '%s'", string(content))
	}
}

func TestCompositeFS_Open_ThreeLayers(t *testing.T) {
	layer1 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("layer1")},
	}

	layer2 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("layer2")},
	}

	layer3 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("layer3")},
	}

	// Layers: [layer1, layer2, layer3] - layer3 has highest precedence
	composite := NewCompositeFS(layer1, layer2, layer3)

	file, err := composite.Open("file.txt")
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer file.Close()

	content, _ := io.ReadAll(file)

	if string(content) != "layer3" {
		t.Errorf("Expected layer3 (highest precedence), got '%s'", string(content))
	}
}

func TestCompositeFS_ReadDir_SingleLayer(t *testing.T) {
	mockFS := fstest.MapFS{
		"dir/file1.txt": &fstest.MapFile{Data: []byte("1")},
		"dir/file2.txt": &fstest.MapFile{Data: []byte("2")},
		"dir/file3.txt": &fstest.MapFile{Data: []byte("3")},
	}

	composite := NewCompositeFS(mockFS)

	entries, err := composite.ReadDir("dir")
	if err != nil {
		t.Fatalf("ReadDir() failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	// Verify sorted order
	expectedNames := []string{"file1.txt", "file2.txt", "file3.txt"}
	for i, entry := range entries {
		if entry.Name() != expectedNames[i] {
			t.Errorf("Entry %d: expected %s, got %s", i, expectedNames[i], entry.Name())
		}
	}
}

func TestCompositeFS_ReadDir_DirectoryNotFound(t *testing.T) {
	mockFS := fstest.MapFS{
		"dir/file.txt": &fstest.MapFile{Data: []byte("data")},
	}

	composite := NewCompositeFS(mockFS)

	_, err := composite.ReadDir("missing")
	if err == nil {
		t.Error("Expected error for missing directory")
	}

	if err != fs.ErrNotExist {
		t.Errorf("Expected fs.ErrNotExist, got %v", err)
	}
}

func TestCompositeFS_ReadDir_MergedLayers(t *testing.T) {
	baseFS := fstest.MapFS{
		"assets/base1.txt": &fstest.MapFile{Data: []byte("base1")},
		"assets/base2.txt": &fstest.MapFile{Data: []byte("base2")},
	}

	overrideFS := fstest.MapFS{
		"assets/override1.txt": &fstest.MapFile{Data: []byte("override1")},
		"assets/base1.txt":     &fstest.MapFile{Data: []byte("base1-override")}, // Same name as base
	}

	composite := NewCompositeFS(baseFS, overrideFS)

	entries, err := composite.ReadDir("assets")
	if err != nil {
		t.Fatalf("ReadDir() failed: %v", err)
	}

	// Should have 3 unique files: base1.txt (from override), base2.txt, override1.txt
	if len(entries) != 3 {
		t.Errorf("Expected 3 merged entries, got %d", len(entries))
	}

	// Verify names
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}

	expectedNames := []string{"base1.txt", "base2.txt", "override1.txt"}
	for i, expected := range expectedNames {
		if names[i] != expected {
			t.Errorf("Entry %d: expected %s, got %s", i, expected, names[i])
		}
	}
}

func TestCompositeFS_ReadDir_EmptyDirectory(t *testing.T) {
	mockFS := fstest.MapFS{
		"empty/.keep": &fstest.MapFile{Data: []byte("")},
	}

	composite := NewCompositeFS(mockFS)

	entries, err := composite.ReadDir("empty")
	if err != nil {
		t.Fatalf("ReadDir() failed: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Expected 1 entry (.keep), got %d", len(entries))
	}
}

func TestCompositeFS_ReadDir_RootDirectory(t *testing.T) {
	mockFS := fstest.MapFS{
		"file1.txt":     &fstest.MapFile{Data: []byte("1")},
		"file2.txt":     &fstest.MapFile{Data: []byte("2")},
		"dir/file3.txt": &fstest.MapFile{Data: []byte("3")},
	}

	composite := NewCompositeFS(mockFS)

	entries, err := composite.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.) failed: %v", err)
	}

	// Should have 3 entries: file1.txt, file2.txt, dir
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}
}

func TestCompositeFS_ReadDir_NestedDirectories(t *testing.T) {
	mockFS := fstest.MapFS{
		"a/b/c/file.txt": &fstest.MapFile{Data: []byte("nested")},
	}

	composite := NewCompositeFS(mockFS)

	// Test reading nested directory
	entries, err := composite.ReadDir("a/b/c")
	if err != nil {
		t.Fatalf("ReadDir(a/b/c) failed: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Name() != "file.txt" {
		t.Errorf("Expected file.txt, got %s", entries[0].Name())
	}
}

func TestCompositeFS_EmptyLayers(t *testing.T) {
	composite := NewCompositeFS()

	_, err := composite.Open("any.txt")
	if err != fs.ErrNotExist {
		t.Errorf("Expected fs.ErrNotExist for empty composite, got %v", err)
	}

	_, err = composite.ReadDir(".")
	if err != fs.ErrNotExist {
		t.Errorf("Expected fs.ErrNotExist for empty ReadDir, got %v", err)
	}
}

func TestCompositeFS_ReadDir_Sorting(t *testing.T) {
	mockFS := fstest.MapFS{
		"dir/zebra.txt":   &fstest.MapFile{Data: []byte("z")},
		"dir/alpha.txt":   &fstest.MapFile{Data: []byte("a")},
		"dir/charlie.txt": &fstest.MapFile{Data: []byte("c")},
		"dir/bravo.txt":   &fstest.MapFile{Data: []byte("b")},
	}

	composite := NewCompositeFS(mockFS)

	entries, err := composite.ReadDir("dir")
	if err != nil {
		t.Fatalf("ReadDir() failed: %v", err)
	}

	// Verify alphabetical sorting
	expectedOrder := []string{"alpha.txt", "bravo.txt", "charlie.txt", "zebra.txt"}
	for i, entry := range entries {
		if entry.Name() != expectedOrder[i] {
			t.Errorf("Entry %d: expected %s, got %s", i, expectedOrder[i], entry.Name())
		}
	}
}

func TestCompositeFS_IntegrationWithWalkDir(t *testing.T) {
	mockFS := fstest.MapFS{
		"assets/file1.txt":     &fstest.MapFile{Data: []byte("1")},
		"assets/dir/file2.txt": &fstest.MapFile{Data: []byte("2")},
	}

	composite := NewCompositeFS(mockFS)

	// Test that CompositeFS works with fs.WalkDir
	var paths []string
	err := fs.WalkDir(composite, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	})

	if err != nil {
		t.Fatalf("WalkDir() failed: %v", err)
	}

	// Should walk: assets, assets/dir, assets/dir/file2.txt, assets/file1.txt
	if len(paths) < 3 {
		t.Errorf("Expected at least 3 paths, got %d: %v", len(paths), paths)
	}
}
