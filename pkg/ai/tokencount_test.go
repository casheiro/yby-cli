package ai

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"vazio", "", 0},
		{"4 chars = 1 token", "abcd", 1},
		{"5 chars = 2 tokens", "abcde", 2},
		{"8 chars = 2 tokens", "abcdefgh", 2},
		{"1 char = 1 token", "a", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, EstimateTokens(tt.input))
		})
	}
}

func TestGetModelMetadata_KnownModel(t *testing.T) {
	meta := GetModelMetadata("gpt-4o-mini")
	assert.Equal(t, "gpt-4o-mini", meta.Name)
	assert.Equal(t, 128_000, meta.ContextWindow)
}

func TestGetModelMetadata_GeminiModel(t *testing.T) {
	meta := GetModelMetadata("gemini-2.5-flash")
	assert.Equal(t, 1_000_000, meta.ContextWindow)
}

func TestGetModelMetadata_UnknownModel(t *testing.T) {
	meta := GetModelMetadata("modelo-desconhecido")
	assert.Equal(t, "modelo-desconhecido", meta.Name)
	assert.Equal(t, defaultContextWindow, meta.ContextWindow)
}

// mockProvider implementa Provider para testes do TokenAwareProvider.
type mockProviderForToken struct {
	name string
}

func (m *mockProviderForToken) Name() string                       { return m.name }
func (m *mockProviderForToken) IsAvailable(_ context.Context) bool { return true }
func (m *mockProviderForToken) Completion(_ context.Context, _, _ string) (string, error) {
	return "ok", nil
}
func (m *mockProviderForToken) StreamCompletion(_ context.Context, _, _ string, _ io.Writer) error {
	return nil
}
func (m *mockProviderForToken) GenerateGovernance(_ context.Context, _ string) (*GovernanceBlueprint, error) {
	return &GovernanceBlueprint{}, nil
}
func (m *mockProviderForToken) EmbedDocuments(_ context.Context, _ []string) ([][]float32, error) {
	return nil, nil
}

func TestTokenAwareProvider_Completion_UnderLimit(t *testing.T) {
	inner := &mockProviderForToken{name: "test"}
	tap := NewTokenAwareProvider(inner, "gpt-4o-mini") // 128k

	// Texto pequeno — deve passar
	result, err := tap.Completion(context.Background(), "system", "user prompt")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestTokenAwareProvider_Completion_OverLimit(t *testing.T) {
	inner := &mockProviderForToken{name: "test"}
	// gpt-4 tem 8192 tokens. 90% = ~7372 tokens = ~29489 chars
	tap := NewTokenAwareProvider(inner, "gpt-4")

	// Gerar texto que excede 90% do limite (> 29489 chars)
	bigText := strings.Repeat("x", 40000)
	_, err := tap.Completion(context.Background(), bigText, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ERR_TOKEN_LIMIT")
}

func TestTokenAwareProvider_GenerateGovernance_OverLimit(t *testing.T) {
	inner := &mockProviderForToken{name: "test"}
	tap := NewTokenAwareProvider(inner, "gpt-4") // 8192

	bigText := strings.Repeat("x", 40000)
	_, err := tap.GenerateGovernance(context.Background(), bigText)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ERR_TOKEN_LIMIT")
}

func TestTokenAwareProvider_EmbedDocuments_NoTokenCheck(t *testing.T) {
	inner := &mockProviderForToken{name: "test"}
	tap := NewTokenAwareProvider(inner, "gpt-4")

	// EmbedDocuments não faz verificação de tokens
	result, err := tap.EmbedDocuments(context.Background(), []string{"texto"})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestTokenAwareProvider_DelegatesNameAndIsAvailable(t *testing.T) {
	inner := &mockProviderForToken{name: "meu-provider"}
	tap := NewTokenAwareProvider(inner, "gpt-4")

	assert.Equal(t, "meu-provider", tap.Name())
	assert.True(t, tap.IsAvailable(context.Background()))
}
