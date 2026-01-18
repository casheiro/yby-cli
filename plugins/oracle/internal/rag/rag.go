package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Document representa um fragmento de conhecimento
type Document struct {
	ID      string
	Content string
	Source  string
}

// Indexer define a interface para indexar documentos
type Indexer interface {
	Index(ctx context.Context, dir string) error
	Search(ctx context.Context, query string) ([]Document, error)
}

// SimpleIndexer é uma implementação em memória (MVP)
type SimpleIndexer struct {
	docs []Document
}

func NewSimpleIndexer() *SimpleIndexer {
	return &SimpleIndexer{
		docs: []Document{},
	}
}

func (i *SimpleIndexer) Index(ctx context.Context, dir string) error {
	fmt.Printf("📂 Escaneando documentação em %s...\n", dir)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			doc := Document{
				ID:      path,
				Source:  path,
				Content: string(content),
			}
			i.docs = append(i.docs, doc)
			fmt.Printf("📄 Indexed: %s\n", info.Name())
		}
		return nil
	})

	if err != nil {
		return err
	}

	fmt.Printf("✅ Indexing complete. Total documents: %d\n", len(i.docs))
	return nil
}

func (i *SimpleIndexer) Search(ctx context.Context, query string) ([]Document, error) {
	var results []Document
	// Busca ingênua por substring (contains)
	query = strings.ToLower(query)
	for _, doc := range i.docs {
		if strings.Contains(strings.ToLower(doc.Content), query) {
			results = append(results, doc)
		}
	}
	return results, nil
}
