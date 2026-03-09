package ai

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// ---- MockAIProvider Extended Tests ----

func TestMockAIProvider_Name(t *testing.T) {
	m := &MockAIProvider{Response: "test"}
	if m.Name() != "mock" {
		t.Errorf("expected 'mock', got %s", m.Name())
	}
}

func TestMockAIProvider_IsAvailable(t *testing.T) {
	m := &MockAIProvider{}
	if !m.IsAvailable(context.Background()) {
		t.Error("MockAIProvider should always be available")
	}
}

func TestMockAIProvider_GenerateGovernance(t *testing.T) {
	m := &MockAIProvider{}
	result, err := m.GenerateGovernance(context.Background(), "some description")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result from mock")
	}
}

func TestMockAIProvider_Completion(t *testing.T) {
	m := &MockAIProvider{Response: "Minha resposta gerada"}
	result, err := m.Completion(context.Background(), "system", "user")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != "Minha resposta gerada" {
		t.Errorf("expected 'Minha resposta gerada', got %s", result)
	}
}

func TestMockAIProvider_StreamCompletion(t *testing.T) {
	m := &MockAIProvider{}
	var buf bytes.Buffer
	err := m.StreamCompletion(context.Background(), "sys", "user", &buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMockAIProvider_EmbedDocuments_MultipleTexts(t *testing.T) {
	m := &MockAIProvider{}
	texts := []string{"hello", "world", "mix", "unknown text"}

	embeddings, err := m.EmbedDocuments(context.Background(), texts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embeddings) != 4 {
		t.Errorf("expected 4 embeddings, got %d", len(embeddings))
	}
	// Verify specific known vectors
	if embeddings[0][0] != 1.0 || embeddings[0][1] != 0.0 {
		t.Errorf("expected hello=[1.0, 0.0], got %v", embeddings[0])
	}
	if embeddings[1][0] != 0.0 || embeddings[1][1] != 1.0 {
		t.Errorf("expected world=[0.0, 1.0], got %v", embeddings[1])
	}
	if embeddings[2][0] != 0.5 || embeddings[2][1] != 0.5 {
		t.Errorf("expected mix=[0.5, 0.5], got %v", embeddings[2])
	}
}

// ---- VectorStore Extended Tests ----

func newTestVectorStore(t *testing.T) *VectorStore {
	t.Helper()
	tmpDir := t.TempDir()
	vs, err := NewVectorStore(context.Background(), filepath.Join(tmpDir, "db"), &MockAIProvider{})
	if err != nil {
		t.Fatalf("failed to create VectorStore: %v", err)
	}
	return vs
}

func TestVectorStore_AddDocuments_Empty(t *testing.T) {
	vs := newTestVectorStore(t)
	// Adding empty docs should be a no-op
	err := vs.AddDocuments(context.Background(), nil, nil, nil)
	if err != nil {
		t.Errorf("expected no error for empty docs, got: %v", err)
	}
}

func TestVectorStore_AddDocuments_WithData(t *testing.T) {
	vs := newTestVectorStore(t)
	docs := []string{"hello", "world"}
	metas := []map[string]string{{"id": "1"}, {"id": "2"}}
	ids := []string{"doc1", "doc2"}

	err := vs.AddDocuments(context.Background(), docs, metas, ids)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVectorStore_Search_ReturnsTopK(t *testing.T) {
	vs := newTestVectorStore(t)
	docs := []string{"hello", "world"}
	metas := []map[string]string{{"id": "a"}, {"id": "b"}}
	ids := []string{"a", "b"}

	if err := vs.AddDocuments(context.Background(), docs, metas, ids); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	results, err := vs.Search(context.Background(), "hello", 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// ---- ErrorEmbedProvider to test error branches ----

type errorEmbedProvider struct{}

func (e *errorEmbedProvider) Name() string                       { return "error-provider" }
func (e *errorEmbedProvider) IsAvailable(_ context.Context) bool { return true }
func (e *errorEmbedProvider) GenerateGovernance(_ context.Context, _ string) (*GovernanceBlueprint, error) {
	return nil, errors.New("not implemented")
}
func (e *errorEmbedProvider) Completion(_ context.Context, _, _ string) (string, error) {
	return "", errors.New("not implemented")
}
func (e *errorEmbedProvider) StreamCompletion(_ context.Context, _, _ string, _ io.Writer) error {
	return errors.New("not implemented")
}
func (e *errorEmbedProvider) EmbedDocuments(_ context.Context, texts []string) ([][]float32, error) {
	return nil, errors.New("embed error")
}

func TestVectorStore_AddDocuments_EmbedError(t *testing.T) {
	tmpDir := t.TempDir()
	vs, err := NewVectorStore(context.Background(), filepath.Join(tmpDir, "db"), &errorEmbedProvider{})
	if err != nil {
		t.Fatalf("failed to create VectorStore: %v", err)
	}

	err = vs.AddDocuments(context.Background(), []string{"doc1"}, []map[string]string{{"id": "1"}}, []string{"1"})
	if err == nil {
		t.Error("expected error from embed failure, got nil")
	}
}

func TestVectorStore_Search_EmbedError(t *testing.T) {
	tmpDir := t.TempDir()
	vs, err := NewVectorStore(context.Background(), filepath.Join(tmpDir, "db"), &errorEmbedProvider{})
	if err != nil {
		t.Fatalf("failed to create VectorStore: %v", err)
	}

	_, err = vs.Search(context.Background(), "query", 1)
	if err == nil {
		t.Error("expected error from embed failure in search, got nil")
	}
}

func TestNewVectorStore_InvalidPath(t *testing.T) {
	// Try to create a store in a path that cannot be created (root-owned)
	_, err := NewVectorStore(context.Background(), "/root/cannot_write_here/db", &MockAIProvider{})
	if err == nil {
		// If running as root this will succeed; skip
		if os.Getuid() == 0 {
			t.Skip("running as root, skipping permission test")
		}
		t.Error("expected error for unwritable path, got nil")
	}
}
