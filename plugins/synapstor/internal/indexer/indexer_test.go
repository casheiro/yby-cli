package indexer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/gofrs/flock"
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

// TestHashDoConteudo_Deterministico verifica que o hash é determinístico para o mesmo conteúdo.
func TestHashDoConteudo_Deterministico(t *testing.T) {
	h1 := hashDoConteudo([]byte("conteúdo fixo"))
	h2 := hashDoConteudo([]byte("conteúdo fixo"))
	if h1 != h2 {
		t.Error("hash deveria ser determinístico")
	}
}

// TestHashDoConteudo_DiferenteParaConteudoDiferente verifica que conteúdos diferentes geram hashes diferentes.
func TestHashDoConteudo_DiferenteParaConteudoDiferente(t *testing.T) {
	h1 := hashDoConteudo([]byte("conteúdo A"))
	h2 := hashDoConteudo([]byte("conteúdo B"))
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
	// sem arquivos, a volta é rápida o bastante para o relógio do Windows medir 0
	if report.Duration < 0 {
		t.Error("esperado Duration não negativo")
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

// etapasComAvisos guarda os avisos da indexação.
type etapasComAvisos struct {
	shared.EtapasMudas
	avisos []string
}

func (e *etapasComAvisos) Avisar(texto string) { e.avisos = append(e.avisos, texto) }

// TestRun_ArquivoIlegivelFicaNoIndice verifica que um arquivo que não deu para ler (sem
// permissão, preso por outro programa) não é tratado como apagado: continua no índice e no
// manifest, com aviso.
func TestRun_ArquivoIlegivelFicaNoIndice(t *testing.T) {
	ctx := context.Background()
	raiz := projetoComUKIs(t)
	p := &embeddingsFalsos{nome: "a", dimensao: 128}
	if _, err := NewIndexer(p, raiz).Run(ctx); err != nil {
		t.Fatal(err)
	}
	// muda o conteúdo do outro UKI, para a segunda indexação ter o que gravar
	outro := filepath.Join(raiz, ".synapstor", ".uki", "UKI-2-backup.md")
	if err := os.WriteFile(outro, []byte("# Backup\n\nO backup do banco roda toda noite, às 2h.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	idx := NewIndexer(p, raiz)
	et := &etapasComAvisos{}
	idx.Etapas = et
	idx.lerArquivo = func(caminho string) ([]byte, error) {
		if strings.HasSuffix(filepath.ToSlash(caminho), "UKI-1-deploy.md") {
			return nil, errors.New("acesso negado")
		}
		return os.ReadFile(caminho)
	}
	report, err := idx.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.FilesRemoved != 0 || report.FilesUnreadable != 1 {
		t.Errorf("o ilegível não é removido: %d removidos, %d ilegíveis", report.FilesRemoved, report.FilesUnreadable)
	}
	vs, err := ai.NewVectorStore(ctx, filepath.Join(raiz, ".synapstor", ".index"), p)
	if err != nil {
		t.Fatal(err)
	}
	if vs.Count() != 2 {
		t.Errorf("o ilegível continua no índice: esperava 2 documentos, obtive %d", vs.Count())
	}
	if _, ok := idx.loadManifest().Files[".synapstor/.uki/UKI-1-deploy.md"]; !ok {
		t.Error("o ilegível continua no manifest")
	}
	if len(et.avisos) != 1 || !strings.Contains(et.avisos[0], "não deu para ler .synapstor/.uki/UKI-1-deploy.md") {
		t.Errorf("esperava um aviso com o arquivo ilegível, obtive %q", et.avisos)
	}

	// Na próxima, já legível e sem mudança, o arquivo é pulado por hash, sem reindexar.
	report, err = NewIndexer(p, raiz).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.FilesSkipped != 2 || report.EmbeddingsCreated != 0 {
		t.Errorf("de volta legível, nada muda: %d pulados, %d indexados", report.FilesSkipped, report.EmbeddingsCreated)
	}
}

// TestRun_LeCadaArquivoUmaVez verifica que o hash sai do conteúdo já lido.
func TestRun_LeCadaArquivoUmaVez(t *testing.T) {
	raiz := projetoComUKIs(t)
	idx := NewIndexer(&embeddingsFalsos{nome: "a", dimensao: 128}, raiz)
	leituras := map[string]int{}
	idx.lerArquivo = func(caminho string) ([]byte, error) {
		leituras[filepath.Base(caminho)]++
		return os.ReadFile(caminho)
	}
	if _, err := idx.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for nome, n := range leituras {
		if n != 1 {
			t.Errorf("%s lido %d vezes", nome, n)
		}
	}
	if len(leituras) != 2 {
		t.Errorf("esperava 2 arquivos lidos, obtive %v", leituras)
	}
}

// TestRun_OutroIndexEmAndamento verifica que dois index no mesmo projeto não gravam o
// índice ao mesmo tempo: o segundo para com erro claro, e a trava sai com o primeiro.
func TestRun_OutroIndexEmAndamento(t *testing.T) {
	ctx := context.Background()
	raiz := projetoComUKIs(t)
	p := &embeddingsFalsos{nome: "a", dimensao: 128}
	pasta := filepath.Join(raiz, ".synapstor", ".index")
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		t.Fatal(err)
	}
	outro := flock.New(filepath.Join(pasta, arquivoDaTrava))
	ok, err := outro.TryLock()
	if err != nil || !ok {
		t.Fatalf("preparo: não deu para travar (%v)", err)
	}
	// a trava presa impede o TempDir de apagar a pasta no Windows
	t.Cleanup(func() { _ = outro.Unlock() })

	_, err = NewIndexer(p, raiz).Run(ctx)
	if !errors.Is(err, ErrIndexEmAndamento) {
		t.Fatalf("esperava ErrIndexEmAndamento, obtive %v", err)
	}
	if !strings.Contains(err.Error(), "outro index") {
		t.Errorf("a mensagem diz o que houve: %v", err)
	}

	if err := outro.Unlock(); err != nil {
		t.Fatal(err)
	}
	report, err := NewIndexer(p, raiz).Run(ctx)
	if err != nil {
		t.Fatalf("sem o outro index, a indexação roda: %v", err)
	}
	if report.EmbeddingsCreated != 2 {
		t.Errorf("esperava 2 indexados, obtive %d", report.EmbeddingsCreated)
	}
	// a trava é liberada no fim: um terceiro index consegue travar
	depois := flock.New(filepath.Join(pasta, arquivoDaTrava))
	if ok, err := depois.TryLock(); err != nil || !ok {
		t.Errorf("a trava continua presa depois do index (%v)", err)
	}
	_ = depois.Unlock()
}
