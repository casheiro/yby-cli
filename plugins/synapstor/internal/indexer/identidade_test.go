package indexer

import (
	"context"
	"os"
	"path/filepath"
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
