package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSimpleIndexer(t *testing.T) {
	// Setup temporário
	tmpDir, err := os.MkdirTemp("", "rag-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	docContent := "# Hello World\nThis is a test document about integration."
	if err := os.WriteFile(filepath.Join(tmpDir, "doc1.md"), []byte(docContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Teste Index
	indexer := NewSimpleIndexer()
	if err := indexer.Index(context.Background(), tmpDir); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	if len(indexer.docs) != 1 {
		t.Errorf("Expected 1 document, got %d", len(indexer.docs))
	}

	// Teste Search
	results, err := indexer.Search(context.Background(), "integration")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'integration', got %d", len(results))
	}

	results2, _ := indexer.Search(context.Background(), "banana")
	if len(results2) != 0 {
		t.Errorf("Expected 0 results for 'banana', got %d", len(results2))
	}
}
