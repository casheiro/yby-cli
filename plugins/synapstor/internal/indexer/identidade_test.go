package indexer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai"
)

// embeddingsFalsos gera vetores de dimensão fixa: trocar de provedor falso simula trocar
// de provider ou de modelo de embeddings entre duas indexações.
type embeddingsFalsos struct {
	ai.Provider
	nome     string
	dimensao int
}

func (e *embeddingsFalsos) Name() string { return e.nome }

func (e *embeddingsFalsos) EmbedDocuments(_ context.Context, textos []string) ([][]float32, error) {
	vetores := make([][]float32, len(textos))
	for i, texto := range textos {
		v := make([]float32, e.dimensao)
		v[len(texto)%e.dimensao] = 1
		vetores[i] = v
	}
	return vetores, nil
}

// projetoComUKIs cria um projeto com dois UKIs e devolve a raiz.
func projetoComUKIs(t *testing.T) string {
	t.Helper()
	raiz := t.TempDir()
	dir := filepath.Join(raiz, ".synapstor", ".uki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for nome, conteudo := range map[string]string{
		"UKI-1-deploy.md": "# Deploy\n\nO deploy passa pelo Argo CD.\n",
		"UKI-2-backup.md": "# Backup\n\nO backup do banco roda toda noite.\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, nome), []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return raiz
}

func TestRun_TrocarDeEmbeddingsRefazOIndice(t *testing.T) {
	ctx := context.Background()
	raiz := projetoComUKIs(t)

	if _, err := NewIndexer(&embeddingsFalsos{nome: "a", dimensao: 128}, raiz).Run(ctx); err != nil {
		t.Fatal(err)
	}

	novo := &embeddingsFalsos{nome: "b", dimensao: 384}
	report, err := NewIndexer(novo, raiz).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.EmbeddingsCreated != 2 || report.FilesSkipped != 0 {
		t.Errorf("com outros embeddings, o index refaz tudo: %d indexados, %d pulados", report.EmbeddingsCreated, report.FilesSkipped)
	}

	vs, err := ai.NewVectorStore(ctx, filepath.Join(raiz, ".synapstor", ".index"), novo)
	if err != nil {
		t.Fatal(err)
	}
	if docs, err := vs.Search(ctx, "deploy", 2); err != nil || len(docs) != 2 {
		t.Errorf("depois de refeito, a busca com os embeddings novos funciona: %d (%v)", len(docs), err)
	}
}

func TestRun_MesmosEmbeddingsContinuaIncremental(t *testing.T) {
	ctx := context.Background()
	raiz := projetoComUKIs(t)
	p := &embeddingsFalsos{nome: "a", dimensao: 128}

	if _, err := NewIndexer(p, raiz).Run(ctx); err != nil {
		t.Fatal(err)
	}
	report, err := NewIndexer(p, raiz).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.EmbeddingsCreated != 0 || report.FilesSkipped != 2 {
		t.Errorf("com os mesmos embeddings, nada muda: %d indexados, %d pulados", report.EmbeddingsCreated, report.FilesSkipped)
	}
}

// comOrfao grava no índice um documento que o manifest não conhece.
func comOrfao(t *testing.T, raiz string, p ai.Provider) *ai.VectorStore {
	t.Helper()
	ctx := context.Background()
	vs, err := ai.NewVectorStore(ctx, filepath.Join(raiz, ".synapstor", ".index"), p)
	if err != nil {
		t.Fatal(err)
	}
	err = vs.AddDocuments(ctx, []string{"# Velho\n\ndoc que não existe mais\n"},
		[]map[string]string{{"source": "docs/velho.md"}}, []string{"docs/velho.md"})
	if err != nil {
		t.Fatal(err)
	}
	return vs
}

func TestRun_FullApagaOrfaos(t *testing.T) {
	ctx := context.Background()
	raiz := projetoComUKIs(t)
	p := &embeddingsFalsos{nome: "a", dimensao: 128}
	if _, err := NewIndexer(p, raiz).Run(ctx); err != nil {
		t.Fatal(err)
	}
	vs := comOrfao(t, raiz, p)
	if vs.Count() != 3 {
		t.Fatalf("preparo: esperava 3 documentos, obtive %d", vs.Count())
	}

	idx := NewIndexer(p, raiz)
	idx.FullReindex = true
	report, err := idx.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.EmbeddingsCreated != 2 || report.FilesSkipped != 0 {
		t.Errorf("--full refaz tudo: %d indexados, %d pulados", report.EmbeddingsCreated, report.FilesSkipped)
	}
	vs2, err := ai.NewVectorStore(ctx, filepath.Join(raiz, ".synapstor", ".index"), p)
	if err != nil {
		t.Fatal(err)
	}
	if vs2.Count() != 2 {
		t.Errorf("depois do --full só ficam os arquivos de agora: %d documentos", vs2.Count())
	}
}

func TestRun_IncrementalConciliaOrfaos(t *testing.T) {
	ctx := context.Background()
	raiz := projetoComUKIs(t)
	p := &embeddingsFalsos{nome: "a", dimensao: 128}
	if _, err := NewIndexer(p, raiz).Run(ctx); err != nil {
		t.Fatal(err)
	}
	comOrfao(t, raiz, p)

	if _, err := NewIndexer(p, raiz).Run(ctx); err != nil {
		t.Fatal(err)
	}
	vs, err := ai.NewVectorStore(ctx, filepath.Join(raiz, ".synapstor", ".index"), p)
	if err != nil {
		t.Fatal(err)
	}
	if vs.Count() != 2 {
		t.Errorf("o órfão sai mesmo sem --full: %d documentos", vs.Count())
	}
}

func TestRun_IDEFonteUsamBarra(t *testing.T) {
	ctx := context.Background()
	raiz := projetoComUKIs(t)
	sub := filepath.Join(raiz, ".synapstor", ".uki", "infra")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "UKI-3-rede.md"), []byte("# Rede\n\nA rede usa Cilium.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &embeddingsFalsos{nome: "a", dimensao: 128}
	idx := NewIndexer(p, raiz)
	if _, err := idx.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := idx.loadManifest().Files[".synapstor/.uki/infra/UKI-3-rede.md"]; !ok {
		t.Errorf("a chave do manifest usa /: %v", idx.loadManifest().Files)
	}
	vs, err := ai.NewVectorStore(ctx, filepath.Join(raiz, ".synapstor", ".index"), p)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := vs.Search(ctx, "rede", 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if strings.Contains(d.Metadata["source"], `\`) {
			t.Errorf("source com barra invertida: %q", d.Metadata["source"])
		}
	}
}

func TestRun_FullRefazIndiceQueNaoAbre(t *testing.T) {
	ctx := context.Background()
	raiz := projetoComUKIs(t)
	p := &embeddingsFalsos{nome: "a", dimensao: 128}
	if _, err := NewIndexer(p, raiz).Run(ctx); err != nil {
		t.Fatal(err)
	}
	pasta := filepath.Join(raiz, ".synapstor", ".index")
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		t.Fatal(err)
	}
	estragou := false
	for _, e := range entradas {
		if e.IsDir() {
			if err := os.WriteFile(filepath.Join(pasta, e.Name(), "lixo.gob"), []byte("lixo"), 0644); err != nil {
				t.Fatal(err)
			}
			estragou = true
		}
	}
	if !estragou {
		t.Fatal("preparo: não achei a pasta da coleção")
	}
	if _, err := ai.NewVectorStore(ctx, pasta, p); err == nil {
		t.Fatal("preparo: o índice estragado deveria não abrir")
	}

	idx := NewIndexer(p, raiz)
	idx.FullReindex = true
	if _, err := idx.Run(ctx); err != nil {
		t.Fatalf("--full tem de consertar o índice que não abre: %v", err)
	}
	vs, err := ai.NewVectorStore(ctx, pasta, p)
	if err != nil {
		t.Fatal(err)
	}
	if vs.Count() != 2 {
		t.Errorf("esperava 2 documentos, obtive %d", vs.Count())
	}
}
