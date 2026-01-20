package agent

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai"
)

// MockProvider implements ai.Provider for testing
type MockProvider struct {
	Response string
	Err      error
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) IsAvailable(ctx context.Context) bool {
	return true
}

func (m *MockProvider) GenerateGovernance(ctx context.Context, description string) (*ai.GovernanceBlueprint, error) {
	return nil, nil
}

func (m *MockProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.Response, m.Err
}

func (m *MockProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	return nil
}

func (m *MockProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	// Mock implementation: return empty vectors or random
	return make([][]float32, len(texts)), nil
}

func TestCapture(t *testing.T) {
	// Create temp dir for test
	tmpDir, err := os.MkdirTemp("", "synapstor-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Mock AI Response
	mockJson := `{
		"title": "Test UKI",
		"filename": "UKI-123-test.md",
		"content": "**ID:** UKI-MOCK-TEST\n# Test UKI\nContent here.",
		"summary": "Summary"
	}`

	mockProvider := &MockProvider{
		Response: mockJson,
	}

	agent := NewAgent(mockProvider, tmpDir)

	err = agent.Capture("Test input")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	// Verify file creation
	expectedPath := filepath.Join(tmpDir, ".synapstor", ".uki", "UKI-123-test.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected UKI file was not created at %s", expectedPath)
	}

	content, _ := os.ReadFile(expectedPath)
	// Check for semantic ID format
	if !strings.Contains(string(content), "**ID:** UKI-") {
		t.Errorf("Expected content to contain semantic ID format, but got: %s", string(content))
	}
	// Original content check (assuming it should still be there or modified)
	if !strings.Contains(string(content), "# Test UKI\nContent here.") {
		t.Errorf("Unexpected content: %s", string(content))
	}
}

func TestStudy_NoFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "synapstor-test-study")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mockProvider := &MockProvider{Response: "{}"}
	agent := NewAgent(mockProvider, tmpDir)

	// Should not fail, just find nothing
	err = agent.Study("nonexistent")
	if err != nil {
		t.Fatalf("Study failed: %v", err)
	}
}
