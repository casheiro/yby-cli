package ai

import (
	"context"
	"io"
)

// Provider defines the interface for AI backends (Ollama, OpenAI, etc)
// Provider defines the interface for AI backends (Ollama, OpenAI, Gemini, etc)
type Provider interface {
	// Name returns the provider identifier (e.g., "ollama-local")
	Name() string

	// IsAvailable checks if the backend is reachable
	IsAvailable(ctx context.Context) bool

	// GenerateGovernance creates a governance structure based on project description
	// Deprecated: Use Completion with a specific system prompt instead.
	GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error)

	// Completion sends a prompt to the LLM and returns the generated text.
	Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error)

	// StreamCompletion sends a prompt and writes the generated text chunks to the writer.
	StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error

	// EmbedDocuments generates vector embeddings for a list of texts.
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
}
