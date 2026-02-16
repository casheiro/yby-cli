package ai

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Helper: MockProvider for testing
type MockAIProvider struct {
	Response string
}

func (m *MockAIProvider) Name() string                         { return "mock" }
func (m *MockAIProvider) IsAvailable(ctx context.Context) bool { return true }
func (m *MockAIProvider) GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error) {
	return nil, nil
}
func (m *MockAIProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.Response, nil
}
func (m *MockAIProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	return nil
}

// Ensure the MockProvider implements the Provider interface including EmbedDocuments
func (m *MockAIProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	// Simple Deterministic Mock for testing similarity
	// "hello" -> [1.0, 0.0]
	// "world" -> [0.0, 1.0]
	// "mix"   -> [0.5, 0.5]
	var embeddings [][]float32
	for _, t := range texts {
		switch t {
		case "hello":
			embeddings = append(embeddings, []float32{1.0, 0.0})
		case "world":
			embeddings = append(embeddings, []float32{0.0, 1.0})
		default:
			embeddings = append(embeddings, []float32{0.5, 0.5})
		}
	}
	return embeddings, nil
}

// Fix Mock interface signature for StreamCompletion (using io.Writer)
// The previous helper had java.Writer typo
func (m *MockAIProvider) StreamCompletionCorrect(ctx context.Context, systemPrompt, userPrompt string, out interface{}) error {
	return nil // Stub
}

func TestVectorStore_AddAndSearch(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := os.MkdirTemp("", "vector-store-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	provider := &MockAIProvider{}

	vs, err := NewVectorStore(ctx, filepath.Join(tmpDir, "db"), provider)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Add Documents
	docs := []string{"hello", "world"}
	metas := []map[string]string{
		{"id": "1"},
		{"id": "2"},
	}
	ids := []string{"1", "2"}

	if err := vs.AddDocuments(ctx, docs, metas, ids); err != nil {
		t.Fatalf("Failed to add docs: %v", err)
	}

	// Search
	// Searching for "hello" (vector [1,0]) should effectively match "hello" first
	results, err := vs.Search(ctx, "hello", 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].Content != "hello" {
		t.Errorf("Expected top result to be 'hello', got %s", results[0].Content)
	}
}
