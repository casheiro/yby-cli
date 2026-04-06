package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCostTrackingProvider_Completion(t *testing.T) {
	inner := &mockProvider{
		name: "test",
		completionFunc: func(ctx context.Context, _, _ string) (string, error) {
			SetUsage(ctx, &UsageMetadata{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				Provider:         "openai",
				Model:            "gpt-4o-mini",
				Operation:        "completion",
			})
			return "resposta", nil
		},
	}
	ct := NewCostTrackingProvider(inner, "gpt-4o-mini")

	// Usar contexto mutável para SetUsage funcionar
	ctx := context.Background()
	result, err := ct.Completion(ctx, "sys", "usr")
	require.NoError(t, err)
	assert.Equal(t, "resposta", result)
}

func TestCostTrackingProvider_Name(t *testing.T) {
	inner := &mockProvider{name: "meu-provider"}
	ct := NewCostTrackingProvider(inner, "gpt-4o")
	assert.Equal(t, "meu-provider", ct.Name())
}

func TestCostTrackingProvider_IsAvailable(t *testing.T) {
	inner := &mockProvider{name: "test", available: true}
	ct := NewCostTrackingProvider(inner, "gpt-4o")
	assert.True(t, ct.IsAvailable(context.Background()))
}

func TestCostTrackingProvider_EstimateCost(t *testing.T) {
	ct := &CostTrackingProvider{model: "gpt-4o-mini"}

	usage := &UsageMetadata{
		PromptTokens:     1_000_000, // 1M tokens
		CompletionTokens: 1_000_000,
	}
	cost := ct.estimateCost(usage)
	// gpt-4o-mini: $0.15/1M input + $0.60/1M output = $0.75
	assert.InDelta(t, 0.75, cost, 0.001)
}

func TestCostTrackingProvider_EstimateCost_ModeloDesconhecido(t *testing.T) {
	ct := &CostTrackingProvider{model: "modelo-inexistente"}

	usage := &UsageMetadata{
		PromptTokens:     1000,
		CompletionTokens: 500,
	}
	cost := ct.estimateCost(usage)
	assert.Equal(t, 0.0, cost)
}

func TestCostTrackingProvider_EstimateCost_OllamaGratuito(t *testing.T) {
	ct := &CostTrackingProvider{model: "llama3"}

	usage := &UsageMetadata{
		PromptTokens:     1_000_000,
		CompletionTokens: 1_000_000,
	}
	cost := ct.estimateCost(usage)
	assert.Equal(t, 0.0, cost)
}

func TestUsageMetadata_ContextRoundTrip(t *testing.T) {
	ctx := context.Background()

	// Sem usage
	assert.Nil(t, GetUsage(ctx))

	// Com usage
	usage := &UsageMetadata{
		PromptTokens: 42,
		Provider:     "test",
	}
	ctx = SetUsage(ctx, usage)
	got := GetUsage(ctx)
	require.NotNil(t, got)
	assert.Equal(t, 42, got.PromptTokens)
	assert.Equal(t, "test", got.Provider)
}
