package ai

import "context"

// Provider defines the interface for AI backends (Ollama, OpenAI, etc)
type Provider interface {
	// Name returns the provider identifier (e.g., "ollama-local")
	Name() string

	// IsAvailable checks if the backend is reachable
	IsAvailable(ctx context.Context) bool

	// GenerateGovernance creates a governance structure based on project description
	GenerateGovernance(ctx context.Context, description string) (*GovernanceBlueprint, error)
}
