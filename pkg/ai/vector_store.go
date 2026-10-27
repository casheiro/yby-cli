package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/philippgille/chromem-go"
)

// nomeDaColecao é a coleção do chromem com os UKIs.
const nomeDaColecao = "synapstor-knowledge"

// arquivoDeIdentidade guarda, ao lado das coleções, com que embeddings o índice foi feito
// (o chromem ignora arquivos na raiz da pasta).
const arquivoDeIdentidade = "identidade.json"

// VectorStore manages the persistence and retrieval of embeddings.
type VectorStore struct {
	db         *chromem.DB
	collection *chromem.Collection
	provider   Provider
	pasta      string
	// identidade é a dos vetores gravados; nil num índice novo ou num índice antigo, de
	// antes de o yby registrar o modelo.
	identidade *IdentidadeDoIndice
	// AoAvancar, se definida, recebe quantos documentos de AddDocuments já têm embedding;
	// o VectorStore não escreve na tela.
	AoAvancar func(feitos, total int)
}

// NewVectorStore creates or opens a vector store at the specified path.
func NewVectorStore(ctx context.Context, storagePath string, provider Provider) (*VectorStore, error) {
	// Ensure directory exists
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar diretorio do vector store: %w", err)
	}

	// Initialize DB with persistence
	db, err := chromem.NewPersistentDB(storagePath, false)
	if err != nil {
		return nil, fmt.Errorf("falha ao inicializar chromem db: %w", err)
	}

	// Create or Get collection "synapstor-v1"
	// We pass nil as embedding function because we will generate embeddings manually
	// using our generic Provider interface before adding to the store.
	// Chromem supports custom embedding functions, but our Provider architecture
	// abstracts limits and selection logic better.
	collection, err := db.GetOrCreateCollection(nomeDaColecao, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar coleção: %w", err)
	}

	return &VectorStore{
		db:         db,
		collection: collection,
		provider:   provider,
		pasta:      storagePath,
		identidade: lerIdentidade(storagePath),
	}, nil
}

// lerIdentidade lê a identidade gravada na pasta do índice; nil se não houver.
func lerIdentidade(pasta string) *IdentidadeDoIndice {
	dados, err := os.ReadFile(filepath.Join(pasta, arquivoDeIdentidade))
	if err != nil {
		return nil
	}
	var id IdentidadeDoIndice
	if err := json.Unmarshal(dados, &id); err != nil || id.Provider == "" {
		slog.Debug("identidade do índice ilegível", "pasta", pasta, "erro", err)
		return nil
	}
	return &id
}

// gravarIdentidade grava com que embeddings o índice foi feito.
func (vs *VectorStore) gravarIdentidade(id IdentidadeDoIndice) error {
	dados, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(vs.pasta, arquivoDeIdentidade), dados, 0o644); err != nil {
		return fmt.Errorf("falha ao gravar a identidade do índice: %w", err)
	}
	vs.identidade = &id
	return nil
}

// Identidade devolve com que embeddings o índice foi feito; ok é false num índice novo ou
// num índice antigo, de antes de o yby registrar o modelo.
func (vs *VectorStore) Identidade() (IdentidadeDoIndice, bool) {
	if vs.identidade == nil {
		return IdentidadeDoIndice{}, false
	}
	return *vs.identidade, true
}

// Conferir diz se o índice foi feito com os embeddings do provider de agora, sem gerar
// nenhum: devolve *IndiceDeOutroModeloError quando o provider ou o modelo mudou, ou
// quando o índice tem documentos e não registrou com o que foi feito. Índice vazio
// combina com qualquer provider.
func (vs *VectorStore) Conferir(ctx context.Context) error {
	if vs.collection.Count() == 0 {
		return nil
	}
	atual, err := IdentidadeDosEmbeddings(ctx, vs.provider)
	if err != nil {
		return fmt.Errorf("não deu para saber o modelo dos embeddings: %w", err)
	}
	if vs.identidade == nil || !vs.identidade.combina(atual) {
		indice, _ := vs.Identidade()
		return &IndiceDeOutroModeloError{Indice: indice, Atual: atual}
	}
	return nil
}

// Recriar apaga todos os documentos e a identidade do índice, para refazê-lo com outros
// embeddings.
func (vs *VectorStore) Recriar() error {
	if err := vs.db.DeleteCollection(nomeDaColecao); err != nil {
		return fmt.Errorf("falha ao apagar a coleção: %w", err)
	}
	if err := os.Remove(filepath.Join(vs.pasta, arquivoDeIdentidade)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("falha ao apagar a identidade do índice: %w", err)
	}
	vs.identidade = nil
	collection, err := vs.db.GetOrCreateCollection(nomeDaColecao, nil, nil)
	if err != nil {
		return fmt.Errorf("falha ao criar coleção: %w", err)
	}
	vs.collection = collection
	return nil
}

// identidadeAtual é a identidade dos embeddings de agora, com a dimensão dos vetores.
func (vs *VectorStore) identidadeAtual(ctx context.Context, dimensao int) (IdentidadeDoIndice, error) {
	atual, err := IdentidadeDosEmbeddings(ctx, vs.provider)
	if err != nil {
		return IdentidadeDoIndice{}, fmt.Errorf("não deu para saber o modelo dos embeddings: %w", err)
	}
	atual.Dimensao = dimensao
	return atual, nil
}

// AddDocuments embeds the texts and saves them to the store.
// Metadatas can include "filename", "hash", "title".
func (vs *VectorStore) AddDocuments(ctx context.Context, documents []string, metadatas []map[string]string, ids []string) error {
	if len(documents) == 0 {
		return nil
	}

	total := len(documents)
	slog.Debug("gerando embeddings", "documentos", total, "provider", vs.provider.Name())

	// Gerar embeddings um por um com progresso
	const embBatchSize = 1
	var vectors [][]float32
	var atual IdentidadeDoIndice

	for i := 0; i < total; i += embBatchSize {
		end := i + embBatchSize
		if end > total {
			end = total
		}
		batch := documents[i:end]

		batchVectors, err := vs.provider.EmbedDocuments(ctx, batch)
		if err != nil {
			return fmt.Errorf("falha ao gerar embeddings (doc %d-%d): %w", i+1, end, err)
		}
		// Com o primeiro vetor, confere se os embeddings de agora se misturam com os do
		// índice, antes de gerar o resto.
		if i == 0 && len(batchVectors) > 0 {
			if atual, err = vs.identidadeAtual(ctx, len(batchVectors[0])); err != nil {
				return err
			}
			if vs.collection.Count() > 0 && (vs.identidade == nil || !vs.identidade.combina(atual)) {
				indice, _ := vs.Identidade()
				return &IndiceDeOutroModeloError{Indice: indice, Atual: atual}
			}
		}
		vectors = append(vectors, batchVectors...)
		if vs.AoAvancar != nil {
			vs.AoAvancar(end, total)
		}
	}

	if len(vectors) != len(documents) {
		return fmt.Errorf("mismatch: %d documentos mas %d vetores gerados", len(documents), len(vectors))
	}

	// Add to Chromem
	// Chromem expects creating Docs first
	docs := make([]chromem.Document, len(documents))
	for i := range documents {
		docs[i] = chromem.Document{
			ID:        ids[i],
			Metadata:  metadatas[i],
			Embedding: vectors[i],
			Content:   documents[i],
		}
	}

	if err := vs.collection.AddDocuments(ctx, docs, runtime.NumCPU()); err != nil {
		return fmt.Errorf("falha ao salvar documentos no chromem: %w", err)
	}

	if vs.identidade == nil || *vs.identidade != atual {
		return vs.gravarIdentidade(atual)
	}
	return nil
}

// Search returns the most similar documents to the query.
func (vs *VectorStore) Search(ctx context.Context, query string, topK int) ([]UnknownDocument, error) {
	// Embed the query
	queryVectors, err := vs.provider.EmbedDocuments(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("falha ao gerar embedding da query: %w", err)
	}
	if len(queryVectors) == 0 {
		return nil, fmt.Errorf("nenhum vetor gerado para a query")
	}

	queryVector := queryVectors[0]

	// Vetores de outro provider, modelo ou dimensão não se comparam com os do índice.
	atual, err := vs.identidadeAtual(ctx, len(queryVector))
	if err != nil {
		return nil, err
	}
	if vs.identidade != nil && !vs.identidade.combina(atual) {
		return nil, &IndiceDeOutroModeloError{Indice: *vs.identidade, Atual: atual}
	}

	// O chromem recusa nResults maior que o número de documentos (inclusive com a coleção vazia).
	if total := vs.collection.Count(); topK > total {
		topK = total
	}
	if topK <= 0 {
		return nil, nil
	}

	// Search
	results, err := vs.collection.QueryEmbedding(ctx, queryVector, topK, nil, nil)
	if err != nil {
		// Num índice antigo, sem identidade, a troca só aparece na dimensão, no erro do chromem.
		if vs.identidade == nil && strings.Contains(err.Error(), "vectors must have the same length") {
			return nil, &IndiceDeOutroModeloError{Atual: atual}
		}
		return nil, fmt.Errorf("falha na busca vetorial: %w", err)
	}

	// Map results
	var docs []UnknownDocument
	for _, res := range results {
		docs = append(docs, UnknownDocument{
			ID:       res.ID,
			Content:  res.Content,
			Metadata: res.Metadata,
			Score:    res.Similarity,
		})
	}

	return docs, nil
}

// DeleteDocuments remove documentos do store pelos IDs informados.
func (vs *VectorStore) DeleteDocuments(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return vs.collection.Delete(ctx, nil, nil, ids...)
}

// DeleteByMetadata remove documentos que correspondem aos metadados informados.
func (vs *VectorStore) DeleteByMetadata(ctx context.Context, where map[string]string) error {
	if len(where) == 0 {
		return nil
	}
	return vs.collection.Delete(ctx, where, nil)
}

// Count retorna o número de documentos na coleção.
func (vs *VectorStore) Count() int {
	return vs.collection.Count()
}

// UnknownDocument is a generic structure for retrieved data.
// We avoid importing plugin-specific types here to keep pkg/ai independent.
type UnknownDocument struct {
	ID       string
	Content  string
	Metadata map[string]string
	Score    float32
}
