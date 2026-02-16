package ai

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/philippgille/chromem-go"
)

// VectorStore manages the persistence and retrieval of embeddings.
type VectorStore struct {
	db         *chromem.DB
	collection *chromem.Collection
	provider   Provider
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
	collection, err := db.GetOrCreateCollection("synapstor-knowledge", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar coleção: %w", err)
	}

	return &VectorStore{
		db:         db,
		collection: collection,
		provider:   provider,
	}, nil
}

// AddDocuments embeds the texts and saves them to the store.
// Metadatas can include "filename", "hash", "title".
func (vs *VectorStore) AddDocuments(ctx context.Context, documents []string, metadatas []map[string]string, ids []string) error {
	if len(documents) == 0 {
		return nil
	}

	fmt.Printf("🧠 Gerando embeddings para %d documentos usando %s...\n", len(documents), vs.provider.Name())

	// Generate Embeddings via Provider
	vectors, err := vs.provider.EmbedDocuments(ctx, documents)
	if err != nil {
		return fmt.Errorf("falha ao gerar embeddings: %w", err)
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

	// Search
	results, err := vs.collection.QueryEmbedding(ctx, queryVector, topK, nil, nil)
	if err != nil {
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

// UnknownDocument is a generic structure for retrieved data.
// We avoid importing plugin-specific types here to keep pkg/ai independent.
type UnknownDocument struct {
	ID       string
	Content  string
	Metadata map[string]string
	Score    float32
}
