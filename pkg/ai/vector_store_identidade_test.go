package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// embeddingsFalsos gera vetores de dimensão fixa, para simular a troca de provider ou de
// modelo entre a indexação e a busca.
type embeddingsFalsos struct {
	MockAIProvider
	provider string
	modelo   string
	dimensao int
}

func (e *embeddingsFalsos) Name() string { return e.provider }

func (e *embeddingsFalsos) IdentidadeDosEmbeddings(context.Context) (IdentidadeDoIndice, error) {
	return IdentidadeDoIndice{Provider: e.provider, Modelo: e.modelo}, nil
}

func (e *embeddingsFalsos) EmbedDocuments(_ context.Context, textos []string) ([][]float32, error) {
	vetores := make([][]float32, len(textos))
	for i, texto := range textos {
		v := make([]float32, e.dimensao)
		v[len(texto)%e.dimensao] = 1
		vetores[i] = v
	}
	return vetores, nil
}

// indiceCom grava dois documentos num índice novo com os embeddings de p e devolve a pasta.
func indiceCom(t *testing.T, p Provider) string {
	t.Helper()
	pasta := filepath.Join(t.TempDir(), "indice")
	vs, err := NewVectorStore(context.Background(), pasta, p)
	if err != nil {
		t.Fatal(err)
	}
	err = vs.AddDocuments(context.Background(), []string{"deploy com argo", "backup do banco"},
		[]map[string]string{{"source": "a.md"}, {"source": "b.md"}}, []string{"a.md", "b.md"})
	if err != nil {
		t.Fatal(err)
	}
	return pasta
}

func TestVectorStore_BuscaComOutraDimensaoDizComoRefazer(t *testing.T) {
	pasta := indiceCom(t, &embeddingsFalsos{provider: "a", modelo: "pequeno", dimensao: 128})

	vs, err := NewVectorStore(context.Background(), pasta, &embeddingsFalsos{provider: "b", modelo: "grande", dimensao: 384})
	if err != nil {
		t.Fatal(err)
	}
	_, err = vs.Search(context.Background(), "deploy", 2)
	if err == nil {
		t.Fatal("a busca com embeddings de outra dimensão deve falhar")
	}
	if strings.Contains(err.Error(), "vectors must have the same length") || !strings.Contains(err.Error(), "yby synapstor index --full") {
		t.Errorf("o erro deve dizer como refazer o índice, e não o erro cru do chromem: %v", err)
	}
	var outro *IndiceDeOutroModeloError
	if !errors.As(err, &outro) || !errors.Is(err, ErrIndiceDeOutroModelo) {
		t.Fatalf("esperado *IndiceDeOutroModeloError, obtido %T: %v", err, err)
	}
	if outro.Indice != (IdentidadeDoIndice{Provider: "a", Modelo: "pequeno", Dimensao: 128}) ||
		outro.Atual != (IdentidadeDoIndice{Provider: "b", Modelo: "grande", Dimensao: 384}) {
		t.Errorf("o erro traz com que embeddings o índice foi feito e os de agora: %+v", outro)
	}
	if !strings.Contains(err.Error(), "a/pequeno (128 dimensões)") || !strings.Contains(err.Error(), "b/grande (384 dimensões)") {
		t.Errorf("a mensagem diz os dois modelos: %v", err)
	}
}

func TestVectorStore_IndiceAntigoSemIdentidade(t *testing.T) {
	pasta := indiceCom(t, &embeddingsFalsos{provider: "a", modelo: "pequeno", dimensao: 128})
	// um índice de antes da identidade não tem o arquivo
	if err := os.Remove(filepath.Join(pasta, arquivoDeIdentidade)); err != nil {
		t.Fatal(err)
	}

	vs, err := NewVectorStore(context.Background(), pasta, &embeddingsFalsos{provider: "b", modelo: "grande", dimensao: 384})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := vs.Identidade(); ok {
		t.Error("o índice antigo não tem identidade")
	}
	_, err = vs.Search(context.Background(), "deploy", 2)
	var outro *IndiceDeOutroModeloError
	if !errors.As(err, &outro) || outro.Indice.Provider != "" || !strings.Contains(err.Error(), "não ficou registrado") {
		t.Errorf("a dimensão diferente vira o erro tipado, com o índice sem identidade: %v", err)
	}
	if err := vs.Conferir(context.Background()); !errors.Is(err, ErrIndiceDeOutroModelo) {
		t.Errorf("o índice antigo com documentos não confere com nenhum provider: %v", err)
	}
	if err := vs.AddDocuments(context.Background(), []string{"c"}, []map[string]string{{}}, []string{"c"}); !errors.Is(err, ErrIndiceDeOutroModelo) {
		t.Errorf("não se mistura vetor novo num índice de modelo desconhecido: %v", err)
	}
}

func TestVectorStore_ConferirERecriar(t *testing.T) {
	ctx := context.Background()
	pasta := indiceCom(t, &embeddingsFalsos{provider: "a", modelo: "um", dimensao: 128})

	mesmo, err := NewVectorStore(ctx, pasta, &embeddingsFalsos{provider: "a", modelo: "um", dimensao: 128})
	if err != nil {
		t.Fatal(err)
	}
	if id, ok := mesmo.Identidade(); !ok || id != (IdentidadeDoIndice{Provider: "a", Modelo: "um", Dimensao: 128}) {
		t.Errorf("a identidade gravada volta ao reabrir: %+v %v", id, ok)
	}
	if err := mesmo.Conferir(ctx); err != nil {
		t.Errorf("o mesmo provider e modelo conferem: %v", err)
	}

	novo := &embeddingsFalsos{provider: "a", modelo: "dois", dimensao: 384}
	vs, err := NewVectorStore(ctx, pasta, novo)
	if err != nil {
		t.Fatal(err)
	}
	if err := vs.Conferir(ctx); !errors.Is(err, ErrIndiceDeOutroModelo) {
		t.Fatalf("outro modelo não confere: %v", err)
	}
	if err := vs.AddDocuments(ctx, []string{"c"}, []map[string]string{{}}, []string{"c"}); !errors.Is(err, ErrIndiceDeOutroModelo) {
		t.Errorf("o AddDocuments não mistura vetores de outro modelo: %v", err)
	}
	if err := vs.Recriar(); err != nil {
		t.Fatal(err)
	}
	if vs.Count() != 0 || vs.Conferir(ctx) != nil {
		t.Fatalf("depois de recriado, o índice está vazio e confere com qualquer provider: %d", vs.Count())
	}
	if err := vs.AddDocuments(ctx, []string{"deploy"}, []map[string]string{{}}, []string{"d"}); err != nil {
		t.Fatal(err)
	}

	reaberto, err := NewVectorStore(ctx, pasta, novo)
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := reaberto.Identidade(); id != (IdentidadeDoIndice{Provider: "a", Modelo: "dois", Dimensao: 384}) {
		t.Errorf("o índice refeito grava a identidade nova: %+v", id)
	}
	if docs, err := reaberto.Search(ctx, "deploy", 1); err != nil || len(docs) != 1 {
		t.Errorf("a busca no índice refeito funciona: %d (%v)", len(docs), err)
	}
}

func TestIdentidadeDosEmbeddings_AtravessaOsDecorators(t *testing.T) {
	falso := &embeddingsFalsos{provider: "a", modelo: "um", dimensao: 8}
	id, err := IdentidadeDosEmbeddings(context.Background(), wrapProvider(falso, "um"))
	if err != nil || id != (IdentidadeDoIndice{Provider: "a", Modelo: "um"}) {
		t.Errorf("a cadeia de decorators chega no modelo do provider: %+v (%v)", id, err)
	}

	// quem não se identifica fica pelo Name()
	id, err = IdentidadeDosEmbeddings(context.Background(), wrapProvider(&MockAIProvider{}, ""))
	if err != nil || id != (IdentidadeDoIndice{Provider: "mock"}) {
		t.Errorf("sem identificar o modelo, fica o nome: %+v (%v)", id, err)
	}
}

func TestOllama_IdentidadeDosEmbeddingsSemConfigEOPrimeiroBaixado(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	tags := func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(ollamaTagsResponse{Models: []ollamaModelo{{Name: "nomic-embed-text"}, {Name: "llama3.2"}}})
	}
	server, provider := newOllamaTestServer(t, ollamaRouterHandler(tags, nil, nil))
	defer server.Close()

	id, err := IdentidadeDosEmbeddings(context.Background(), wrapProvider(provider, provider.Model))
	if err != nil || id != (IdentidadeDoIndice{Provider: "ollama", Modelo: "nomic-embed-text"}) {
		t.Errorf("sem config, o modelo dos embeddings é o primeiro baixado: %+v (%v)", id, err)
	}
	if !strings.HasPrefix(provider.Name(), "Ollama") {
		t.Errorf("o nome continua o de sempre: %s", provider.Name())
	}
}

func TestVectorStore_BuscaComOutroModeloDeMesmaDimensaoFalha(t *testing.T) {
	pasta := indiceCom(t, &embeddingsFalsos{provider: "a", modelo: "um", dimensao: 128})

	vs, err := NewVectorStore(context.Background(), pasta, &embeddingsFalsos{provider: "a", modelo: "outro", dimensao: 128})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vs.Search(context.Background(), "deploy", 2); err == nil || !strings.Contains(err.Error(), "yby synapstor index --full") {
		t.Errorf("vetores de outro modelo, mesmo com a mesma dimensão, não se comparam: %v", err)
	}
}

func TestVectorStore_BuscaComOMesmoModeloFunciona(t *testing.T) {
	p := &embeddingsFalsos{provider: "a", modelo: "um", dimensao: 128}
	pasta := indiceCom(t, p)

	vs, err := NewVectorStore(context.Background(), pasta, p)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := vs.Search(context.Background(), "deploy", 2)
	if err != nil || len(docs) != 2 {
		t.Errorf("a busca com os mesmos embeddings acha os 2 documentos, obtido %d (%v)", len(docs), err)
	}
}
