package engine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/plugins/forge/internal/engine"
	"github.com/casheiro/yby-cli/plugins/forge/internal/mods"
)

func TestForgeEngine_Run(t *testing.T) {
	// Cria diretório temporário
	tmpDir, err := os.MkdirTemp("", "forge-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Cria arquivo main.go dummy
	mainContent := `package main

func main() {
	println("Hello")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Inicializa Engine e roda Mod
	eng := engine.NewEngine()
	eng.Register(&mods.DummyMod{})

	if err := eng.Run(tmpDir); err != nil {
		t.Fatalf("Engine run failed: %v", err)
	}

	// Verifica se arquivo foi modificado
	newContent, err := os.ReadFile(filepath.Join(tmpDir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}

	expected := "// Refactored by Forge\npackage main"
	if string(newContent)[:len(expected)] != expected {
		t.Errorf("Expected content to start with '%s', got:\n%s", expected, string(newContent))
	}

	// Executa novamente para verificar idempotencia (não deve duplicar)
	if err := eng.Run(tmpDir); err != nil {
		t.Fatalf("Second run failed: %v", err)
	}

	newContent2, _ := os.ReadFile(filepath.Join(tmpDir, "main.go"))
	if string(newContent) != string(newContent2) {
		t.Error("Idempotency check failed: content changed on second run")
	}
}
