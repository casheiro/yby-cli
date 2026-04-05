package indexer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSplitMarkdown(t *testing.T) {
	content := `# Header 1
Content 1

## Subheader A
Content A

# Header 2
Content 2
`
	chunks := splitMarkdown(content)

	if len(chunks) != 3 {
		t.Errorf("Expected 3 chunks, got %d", len(chunks))
	}

	expectedFirst := "# Header 1\nContent 1"
	if len(chunks) > 0 && chunks[0] != expectedFirst {
		t.Errorf("First chunk mismatch.\nExpected:\n%s\nGot:\n%s", expectedFirst, chunks[0])
	}
}

func TestSplitMarkdown_NoHeaders(t *testing.T) {
	content := "Just some plain text without headers."
	chunks := splitMarkdown(content)

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(chunks))
	}

	if chunks[0] != content {
		t.Errorf("Content mismatch")
	}
}

func TestSplitMarkdown_SmallChunksIgnored(t *testing.T) {
	// The splitter ignores chunks < 50 chars
	content := `# Big Header
This content is long enough to be preserved by the splitter logic I hope.

# Small
Short.`

	chunks := splitMarkdown(content)

	// "Short." section is very small, might be ignored if splitter logic < 50
	// Let's verify behavior. "Short." + "# Small\n" is around 14 chars.
	// First chunk is > 50.

	if len(chunks) != 2 {
		t.Errorf("Expected 2 valid chunks (small one accepted > 10 chars), got %d", len(chunks))
	}
}

// TestIndexManifest_SaveAndLoad verifica que o manifest pode ser salvo e carregado corretamente.
func TestIndexManifest_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	idx := &Indexer{RootDir: tmpDir}

	manifest := &IndexManifest{
		Files: map[string]IndexedFile{
			"test.md": {SHA256: "abc123", IndexedAt: time.Now()},
		},
	}

	err := idx.saveManifest(manifest)
	if err != nil {
		t.Fatalf("falha ao salvar manifest: %v", err)
	}

	loaded := idx.loadManifest()
	if len(loaded.Files) != 1 {
		t.Fatalf("esperado 1 arquivo no manifest, obtido %d", len(loaded.Files))
	}
	if loaded.Files["test.md"].SHA256 != "abc123" {
		t.Errorf("hash esperado 'abc123', obtido %q", loaded.Files["test.md"].SHA256)
	}
}

// TestFileHash_Deterministico verifica que o hash é determinístico para o mesmo conteúdo.
func TestFileHash_Deterministico(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(path, []byte("conteúdo fixo"), 0644)

	h1, err := fileHash(path)
	if err != nil {
		t.Fatalf("fileHash falhou: %v", err)
	}
	h2, _ := fileHash(path)
	if h1 != h2 {
		t.Error("hash deveria ser determinístico")
	}
}

// TestFileHash_DiferenteParaConteudoDiferente verifica que conteúdos diferentes geram hashes diferentes.
func TestFileHash_DiferenteParaConteudoDiferente(t *testing.T) {
	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "a.txt")
	path2 := filepath.Join(tmpDir, "b.txt")
	os.WriteFile(path1, []byte("conteúdo A"), 0644)
	os.WriteFile(path2, []byte("conteúdo B"), 0644)

	h1, _ := fileHash(path1)
	h2, _ := fileHash(path2)
	if h1 == h2 {
		t.Error("hashes deveriam ser diferentes para conteúdos diferentes")
	}
}

// TestIndexReport_ZeroValues verifica que um IndexReport recém-criado tem valores zerados.
func TestIndexReport_ZeroValues(t *testing.T) {
	report := &IndexReport{}
	if report.FilesScanned != 0 || report.FilesSkipped != 0 ||
		report.ChunksGenerated != 0 || report.EmbeddingsCreated != 0 {
		t.Error("esperado todos os campos do IndexReport zerados")
	}
	if report.Duration != 0 {
		t.Error("esperado Duration zerado")
	}
}

// TestRun_SemArquivos verifica que Run retorna report com zero arquivos e sem erro.
func TestRun_SemArquivos(t *testing.T) {
	tmpDir := t.TempDir()
	idx := &Indexer{RootDir: tmpDir}

	report, err := idx.Run(nil)
	if err != nil {
		t.Fatalf("Run falhou: %v", err)
	}
	if report == nil {
		t.Fatal("report não deveria ser nil")
	}
	if report.FilesScanned != 0 {
		t.Errorf("esperado 0 arquivos escaneados, obtido %d", report.FilesScanned)
	}
	if report.Duration <= 0 {
		t.Error("esperado Duration positivo")
	}
}

// TestLoadManifest_ArquivoInexistente verifica que carregar de um arquivo inexistente retorna manifest vazio.
func TestLoadManifest_ArquivoInexistente(t *testing.T) {
	tmpDir := t.TempDir()
	idx := &Indexer{RootDir: tmpDir}

	loaded := idx.loadManifest()
	if loaded == nil {
		t.Fatal("manifest não deveria ser nil")
	}
	if len(loaded.Files) != 0 {
		t.Errorf("esperado 0 arquivos no manifest, obtido %d", len(loaded.Files))
	}
}
