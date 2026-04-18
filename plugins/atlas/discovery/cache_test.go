package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewScanManifest(t *testing.T) {
	m := NewScanManifest()
	if m.Files == nil {
		t.Error("Files não deve ser nil")
	}
	if len(m.Files) != 0 {
		t.Errorf("esperado 0 arquivos, obtido %d", len(m.Files))
	}
}

func TestLoadManifest_ArquivoInexistente(t *testing.T) {
	m, err := LoadManifest("/caminho/inexistente/manifest.json")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(m.Files) != 0 {
		t.Errorf("esperado manifesto vazio, obtido %d arquivos", len(m.Files))
	}
}

func TestSaveAndLoadManifest(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, ".yby", "atlas-manifest.json")

	// Salvar manifesto
	m := NewScanManifest()
	m.Files["test.go"] = FileEntry{
		SHA256: "abc123",
	}

	if err := SaveManifest(manifestPath, m); err != nil {
		t.Fatalf("falha ao salvar manifesto: %v", err)
	}

	// Carregar manifesto
	loaded, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("falha ao carregar manifesto: %v", err)
	}

	if len(loaded.Files) != 1 {
		t.Fatalf("esperado 1 arquivo, obtido %d", len(loaded.Files))
	}

	entry, ok := loaded.Files["test.go"]
	if !ok {
		t.Fatal("entrada 'test.go' não encontrada")
	}
	if entry.SHA256 != "abc123" {
		t.Errorf("hash esperado 'abc123', obtido %q", entry.SHA256)
	}
}

func TestHashFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(filePath, []byte("conteúdo de teste"), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo: %v", err)
	}

	hash1, err := HashFile(filePath)
	if err != nil {
		t.Fatalf("falha ao calcular hash: %v", err)
	}

	if hash1 == "" {
		t.Error("hash não deve estar vazio")
	}

	// Mesmo conteúdo = mesmo hash
	hash2, err := HashFile(filePath)
	if err != nil {
		t.Fatalf("falha ao calcular hash: %v", err)
	}
	if hash1 != hash2 {
		t.Error("hashes devem ser iguais para mesmo conteúdo")
	}

	// Conteúdo diferente = hash diferente
	if err := os.WriteFile(filePath, []byte("conteúdo diferente"), 0644); err != nil {
		t.Fatalf("falha ao reescrever arquivo: %v", err)
	}
	hash3, err := HashFile(filePath)
	if err != nil {
		t.Fatalf("falha ao calcular hash: %v", err)
	}
	if hash1 == hash3 {
		t.Error("hashes devem ser diferentes para conteúdos diferentes")
	}
}

func TestHashFile_ArquivoInexistente(t *testing.T) {
	_, err := HashFile("/caminho/inexistente")
	if err == nil {
		t.Error("esperado erro para arquivo inexistente")
	}
}

func TestNeedsRescan_ArquivoNovo(t *testing.T) {
	m := NewScanManifest()
	if !m.NeedsRescan("arquivo_novo.go") {
		t.Error("arquivo novo deve precisar de rescan")
	}
}

func TestNeedsRescan_ArquivoNaoModificado(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo: %v", err)
	}

	m := NewScanManifest()
	if err := m.Update(filePath); err != nil {
		t.Fatalf("falha ao atualizar manifesto: %v", err)
	}

	if m.NeedsRescan(filePath) {
		t.Error("arquivo não modificado não deve precisar de rescan")
	}
}

func TestNeedsRescan_ArquivoModificado(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo: %v", err)
	}

	m := NewScanManifest()
	if err := m.Update(filePath); err != nil {
		t.Fatalf("falha ao atualizar manifesto: %v", err)
	}

	// Modificar o arquivo
	if err := os.WriteFile(filePath, []byte("package main\n// modificado"), 0644); err != nil {
		t.Fatalf("falha ao modificar arquivo: %v", err)
	}

	if !m.NeedsRescan(filePath) {
		t.Error("arquivo modificado deve precisar de rescan")
	}
}

func TestUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo: %v", err)
	}

	m := NewScanManifest()
	if err := m.Update(filePath); err != nil {
		t.Fatalf("falha ao atualizar: %v", err)
	}

	entry, ok := m.Files[filePath]
	if !ok {
		t.Fatal("entrada não encontrada após update")
	}
	if entry.SHA256 == "" {
		t.Error("SHA256 não deve estar vazio após update")
	}
	if entry.ScannedAt.IsZero() {
		t.Error("ScannedAt não deve ser zero após update")
	}
}

func TestRemoveStale(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.go")
	if err := os.WriteFile(existingFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo: %v", err)
	}

	m := NewScanManifest()
	m.Files[existingFile] = FileEntry{SHA256: "abc"}
	m.Files[filepath.Join(tmpDir, "removed.go")] = FileEntry{SHA256: "def"}

	m.RemoveStale()

	if len(m.Files) != 1 {
		t.Errorf("esperado 1 arquivo após remover stale, obtido %d", len(m.Files))
	}
	if _, ok := m.Files[existingFile]; !ok {
		t.Error("arquivo existente não deveria ter sido removido")
	}
}

func TestSaveManifest_CriaDiretorio(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "subdir", "nested", "manifest.json")

	m := NewScanManifest()
	if err := SaveManifest(manifestPath, m); err != nil {
		t.Fatalf("falha ao salvar manifesto em diretório inexistente: %v", err)
	}

	// Verificar que o arquivo foi criado
	if _, err := os.Stat(manifestPath); err != nil {
		t.Errorf("arquivo não foi criado: %v", err)
	}
}
