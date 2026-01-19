package mods

import (
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/plugins/forge/internal/engine"
)

func TestLogMod(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "forge-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dummy main.go
	mainContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello World")
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Init Context
	ctx := &engine.Context{
		RootDir: tmpDir,
		Fset:    token.NewFileSet(),
	}

	mod := &LogMod{}

	// 1. First Check should return true (need to apply)
	shouldRun, err := mod.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if !shouldRun {
		t.Error("Expected Check to return true, got false")
	}

	// 2. Apply
	if err := mod.Apply(ctx); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// 3. Verify Content
	bytes, _ := os.ReadFile(filepath.Join(tmpDir, "main.go"))
	content := string(bytes)

	if !strings.Contains(content, "\"Forge Active 🔨\"") {
		t.Errorf("Content does not contain expected log. Got:\n%s", content)
	}

	// 4. Second Check should return false (already applied)
	shouldRunAgain, err := mod.Check(ctx)
	if err != nil {
		t.Fatalf("Second Check failed: %v", err)
	}
	if shouldRunAgain {
		t.Error("Expected Check to return false after apply, got true")
	}
}
